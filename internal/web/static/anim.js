// 본문의 `:::anim 이름` 자리를 채운다.
//
// **본문에는 이름만 들어 있다.** 실제로 움직이는 것은 여기 이름표를 달고
// 사람이 쓴 함수다(internal/markdown/anim.go 참고). 본문에 <script>를 담기
// 시작하면 그 글 자체가 XSS 벡터가 되고, 나중에 CSP를 걸 길도 막힌다.
//
// 새 애니메이션을 더할 때 손대는 곳은 아래 `COMPONENTS` 한 줄과 그 함수 하나다.
(function () {
  "use strict";

  var reduced = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  // svg는 네임스페이스가 달라서 createElement로는 안 만들어진다.
  function svgEl(tag, attrs) {
    var n = document.createElementNS("http://www.w3.org/2000/svg", tag);
    for (var k in attrs || {}) n.setAttribute(k, attrs[k]);
    return n;
  }

  function el(tag, attrs, kids) {
    var n = document.createElement(tag);
    for (var k in attrs || {}) {
      if (k === "class") n.className = attrs[k];
      else if (k === "text") n.textContent = attrs[k];
      else if (k.slice(0, 2) === "on") n.addEventListener(k.slice(2), attrs[k]);
      else n.setAttribute(k, attrs[k]);
    }
    (kids || []).forEach(function (c) { if (c) n.appendChild(c); });
    return n;
  }

  // 껍데기를 만든다. 어느 애니메이션이든 조작부(다시·한 걸음)와 설명 한 줄은
  // 같은 자리에 있어야 한다 — 글마다 다르게 생기면 읽는 사람이 매번 배운다.
  function shell(host, title) {
    host.textContent = "";
    var stage = el("div", { class: "anim-stage" });
    var note = el("p", { class: "anim-note", role: "status" });
    var bar = el("div", { class: "anim-bar" });
    host.appendChild(el("div", { class: "anim-head" }, [
      el("span", { class: "anim-title", text: title }), bar,
    ]));
    host.appendChild(stage);
    host.appendChild(note);
    return { stage: stage, note: note, bar: bar };
  }

  // 걸음들을 미리 다 만들어 두고 하나씩 보여준다. **계산과 그리기를 나눈다** —
  // 그래야 "한 걸음씩"과 "자동 재생"이 같은 목록을 쓰고, 되감기도 공짜다.
  function player(ui, steps, draw) {
    var at = 0, timer = null;

    function show(i) {
      at = Math.max(0, Math.min(i, steps.length - 1));
      draw(steps[at]);
      ui.note.textContent = (at + 1) + " / " + steps.length + " · " + (steps[at].say || "");
    }
    function stop() { clearTimeout(timer); timer = null; play.textContent = "재생"; }
    function tick() {
      if (at >= steps.length - 1) return stop();
      show(at + 1);
      timer = setTimeout(tick, 420);
    }

    var play = el("button", { type: "button", class: "anim-btn", text: "재생",
      onclick: function () {
        if (timer) return stop();
        if (at >= steps.length - 1) show(0);
        play.textContent = "멈춤";
        timer = setTimeout(tick, 120);
      } });
    ui.bar.appendChild(el("button", { type: "button", class: "anim-btn", text: "◀",
      "aria-label": "한 걸음 뒤로", onclick: function () { stop(); show(at - 1); } }));
    ui.bar.appendChild(play);
    ui.bar.appendChild(el("button", { type: "button", class: "anim-btn", text: "▶",
      "aria-label": "한 걸음 앞으로", onclick: function () { stop(); show(at + 1); } }));
    ui.bar.appendChild(el("button", { type: "button", class: "anim-btn", text: "처음",
      onclick: function () { stop(); show(0); } }));

    show(0);
    // **prefers-reduced-motion이면 저절로 움직이지 않는다.** 버튼은 그대로라
    // 보고 싶은 사람은 한 걸음씩 볼 수 있다.
    if (!reduced) play.click();
  }

  // ── 막대 그리기 (두 정렬 컴포넌트가 같이 쓴다) ──────────────────
  function bars(stage, values, mark) {
    var max = Math.max.apply(null, values) || 1;
    var W = 100 / values.length;
    var svg = svgEl("svg", { viewBox: "0 0 100 46", class: "anim-svg",
      preserveAspectRatio: "none", role: "img" });
    values.forEach(function (v, i) {
      var h = (v / max) * 40;
      var cls = "anim-bar-rect" + (mark && mark[i] ? " is-" + mark[i] : "");
      svg.appendChild(svgEl("rect", {
        x: (i * W + W * 0.14).toFixed(2), y: (44 - h).toFixed(2),
        width: (W * 0.72).toFixed(2), height: h.toFixed(2), class: cls,
      }));
    });
    stage.textContent = "";
    stage.appendChild(svg);
  }

  // ── 컴포넌트 ────────────────────────────────────────────────────

  // 버블 정렬. 비교하는 두 칸과 이미 자리를 잡은 뒤쪽을 표시한다.
  function sortBubble(host) {
    var ui = shell(host, "버블 정렬");
    var a = [5, 3, 8, 1, 9, 2, 7, 4];
    var steps = [{ arr: a.slice(), mark: {}, say: "시작" }];
    var v = a.slice();
    for (var end = v.length - 1; end > 0; end--) {
      for (var i = 0; i < end; i++) {
        var mark = {};
        mark[i] = "look"; mark[i + 1] = "look";
        for (var k = end + 1; k < v.length; k++) mark[k] = "done";
        steps.push({ arr: v.slice(), mark: mark, say: v[i] + " 와 " + v[i + 1] + " 견줌" });
        if (v[i] > v[i + 1]) {
          var t = v[i]; v[i] = v[i + 1]; v[i + 1] = t;
          var sm = {};
          sm[i] = "swap"; sm[i + 1] = "swap";
          for (var k2 = end + 1; k2 < v.length; k2++) sm[k2] = "done";
          steps.push({ arr: v.slice(), mark: sm, say: "자리를 바꿈" });
        }
      }
    }
    var all = {};
    v.forEach(function (_, i) { all[i] = "done"; });
    steps.push({ arr: v.slice(), mark: all, say: "끝" });
    player(ui, steps, function (s) { bars(ui.stage, s.arr, s.mark); });
  }

  // 이진 탐색. 남은 구간과 지금 보는 가운데 칸을 표시한다.
  function binarySearch(host) {
    var ui = shell(host, "이진 탐색");
    var v = [2, 5, 8, 12, 16, 23, 38, 56, 72, 91];
    var target = 23;
    var steps = [];
    var lo = 0, hi = v.length - 1, found = -1;
    function markRange(l, h, mid) {
      var m = {};
      for (var i = 0; i < v.length; i++) if (i < l || i > h) m[i] = "out";
      if (mid !== undefined) m[mid] = "look";
      return m;
    }
    steps.push({ arr: v, mark: markRange(lo, hi), say: target + " 를 찾는다" });
    while (lo <= hi) {
      var mid = (lo + hi) >> 1;
      steps.push({ arr: v, mark: markRange(lo, hi, mid), say: "가운데 " + v[mid] + " 를 본다" });
      if (v[mid] === target) { found = mid; break; }
      if (v[mid] < target) { lo = mid + 1; steps.push({ arr: v, mark: markRange(lo, hi), say: "작다 → 오른쪽 절반" }); }
      else { hi = mid - 1; steps.push({ arr: v, mark: markRange(lo, hi), say: "크다 → 왼쪽 절반" }); }
    }
    var done = {};
    for (var i = 0; i < v.length; i++) if (i !== found) done[i] = "out";
    if (found >= 0) done[found] = "done";
    steps.push({ arr: v, mark: done, say: found >= 0 ? "찾았다 (" + found + "번째)" : "없다" });
    player(ui, steps, function (s) { bars(ui.stage, s.arr, s.mark); });
  }

  // ── 결합확률 / 조건부확률 ─────────────────────────────────────
  //
  // 죽은 apption.co 임베드가 있던 자리를 대신한다. 본문은 "노란색 부분이
  // 결합확률, 초록색 부분이 주변확률"이라고 설명하는데 정작 볼 그림이 없었다.
  //
  // **두 눈높이를 버튼으로 오간다.** 위에서 내려다보면 어느 칸이 큰지 표처럼
  // 한눈에 읽히고, 오른쪽 위에서 비스듬히 보면 높이가 곧 확률이라는 것이
  // 보인다. 기본은 위에서 보는 쪽이다 — 표를 읽는 것이 먼저다.
  //
  // 3D는 **등각투상(isometric)을 직접 계산해서** 그린다. 라이브러리를 쓰지
  // 않는 이유는 이 파일의 다른 것들과 같다(빌드 스텝 없음, CDN 의존 없음).
  // 막대 하나가 윗면·왼면·오른면 세 개의 다각형이라 그게 전부다.
  var JOINT = {
    xName: "날씨", yName: "행동",
    xs: ["맑음", "흐림", "비"],
    ys: ["산책", "집"],
    // p[j][i] = P(X = xs[i], Y = ys[j]). 합이 1이다.
    p: [
      [0.30, 0.10, 0.02],
      [0.12, 0.16, 0.30]
    ]
  };

  function jointProbability(host) {
    var d = JOINT;
    var nx = d.xs.length, ny = d.ys.length;

    var maxP = 0, total = 0;
    for (var j = 0; j < ny; j++) {
      for (var i = 0; i < nx; i++) { maxP = Math.max(maxP, d.p[j][i]); total += d.p[j][i]; }
    }
    // 주변확률. px[i] = P(X = xs[i]), py[j] = P(Y = ys[j]).
    var px = [], py = [];
    for (var a = 0; a < nx; a++) { var s = 0; for (var b = 0; b < ny; b++) s += d.p[b][a]; px.push(s); }
    for (var c = 0; c < ny; c++) { var t = 0; for (var e = 0; e < nx; e++) t += d.p[c][e]; py.push(t); }

    host.textContent = "";
    var wrap = el("div", { class: "joint" });
    var head = el("div", { class: "anim-head" }, [
      el("span", { class: "anim-title", text: "결합확률 P(" + d.xName + ", " + d.yName + ")" })
    ]);
    var barBox = el("div", { class: "anim-bar" });
    head.appendChild(barBox);
    var stage = el("div", { class: "joint-stage" });
    var note = el("p", { class: "anim-note", role: "status" });
    wrap.appendChild(head);
    wrap.appendChild(stage);
    wrap.appendChild(note);
    host.appendChild(wrap);

    var mode = "top";      // 기본은 위에서 바라보기
    var sel = null;        // 고른 칸 {i, j}

    var btn = el("button", {
      type: "button", class: "anim-btn", text: "비스듬히 보기", "aria-pressed": "false",
      onclick: function () {
        mode = mode === "top" ? "iso" : "top";
        btn.textContent = mode === "top" ? "비스듬히 보기" : "위에서 보기";
        btn.setAttribute("aria-pressed", mode === "top" ? "false" : "true");
        draw();
      }
    });
    barBox.appendChild(btn);

    function tell() {
      if (!sel) {
        note.textContent = "칸을 누르면 결합확률과 조건부확률을 함께 보여준다. 전체 합 = " + total.toFixed(2);
        return;
      }
      var joint = d.p[sel.j][sel.i], mx = px[sel.i], my = py[sel.j];
      note.textContent =
        "P(" + d.xs[sel.i] + ", " + d.ys[sel.j] + ") = " + joint.toFixed(2) +
        " · P(" + d.ys[sel.j] + " | " + d.xs[sel.i] + ") = " +
        joint.toFixed(2) + "/" + mx.toFixed(2) + " = " + (joint / mx).toFixed(3) +
        " · P(" + d.xs[sel.i] + " | " + d.ys[sel.j] + ") = " +
        joint.toFixed(2) + "/" + my.toFixed(2) + " = " + (joint / my).toFixed(3);
    }

    function pick(i, j) {
      sel = (sel && sel.i === i && sel.j === j) ? null : { i: i, j: j };
      draw();
    }

    // 고른 칸이 있으면 그 칸의 행과 열을 함께 물들인다 — 조건부확률은
    // "그 줄만 떼어 다시 1로 만든 것"이라, 어느 줄로 나누는지가 보여야 한다.
    function cellClass(i, j) {
      if (!sel) return "joint-cell";
      if (sel.i === i && sel.j === j) return "joint-cell is-sel";
      if (sel.i === i || sel.j === j) return "joint-cell is-line";
      return "joint-cell is-dim";
    }

    // ── 위에서 바라보기 ──────────────────────────────────────
    // 칸 넓이는 그대로 두고 **색 진하기로** 확률을 나타낸다. 넓이까지
    // 바꾸면 모자이크가 되어 행·열을 눈으로 따라가기 어렵다.
    function drawTop() {
      var L = 62, T = 18, CW = 92, CH = 50, MW = 58, MH = 34;
      var W = L + nx * CW + MW + 10, H = T + ny * CH + MH + 10;
      var svg = svgEl("svg", {
        viewBox: "0 0 " + W + " " + H, class: "joint-svg",
        role: "img", "aria-label": "결합확률 표를 위에서 본 그림"
      });

      // 열 이름
      for (var i = 0; i < nx; i++) {
        svg.appendChild(text(L + i * CW + CW / 2, T - 6, d.xs[i], "joint-axis"));
      }

      // 결합확률 칸 — **노란색**. 본문이 "노란색 부분을 결합확률"이라고 부른다.
      for (var j = 0; j < ny; j++) {
        svg.appendChild(text(L - 8, T + j * CH + CH / 2 + 4, d.ys[j], "joint-axis joint-axis-y"));
        for (var i2 = 0; i2 < nx; i2++) {
          var p = d.p[j][i2];
          var g = svgEl("g", { class: cellClass(i2, j), tabindex: "0", role: "button",
            "aria-label": d.xs[i2] + " " + d.ys[j] + " 결합확률 " + p.toFixed(2) });
          // **흰 바탕을 먼저 깐다.** 진하기로 크기를 나타내는데, 그냥 투명하게
          // 두면 다크 테마에서 옅은 칸이 검게 가라앉아 그 위의 검정 숫자가
          // 안 보인다. 노랑·초록은 검정 글자를 얹는 지면이라 어느 테마에서든
          // 밝은 바탕 위에 있어야 한다.
          g.appendChild(svgEl("rect", {
            x: L + i2 * CW + 2, y: T + j * CH + 2, width: CW - 4, height: CH - 4,
            class: "joint-bg"
          }));
          g.appendChild(svgEl("rect", {
            x: L + i2 * CW + 2, y: T + j * CH + 2, width: CW - 4, height: CH - 4,
            class: "joint-fill", "fill-opacity": (0.14 + 0.86 * (p / maxP)).toFixed(3)
          }));
          g.appendChild(text(L + i2 * CW + CW / 2, T + j * CH + CH / 2 + 5, p.toFixed(2), "joint-num"));
          bindPick(g, i2, j);
          svg.appendChild(g);
        }
      }

      // 주변확률 — **초록색 띠**. 본문이 "초록색 부분을 주변확률"이라고 부른다.
      // 색을 나누는 것이 이 그림이 하는 일의 절반이다: 조건부확률은
      // 노란 칸 하나를 그 줄의 초록 띠로 나눈 값이다.
      var maxMy = Math.max.apply(null, py), maxMx = Math.max.apply(null, px);
      for (var j2 = 0; j2 < ny; j2++) {
        var on = sel && sel.j === j2;
        var gm = svgEl("g", { class: "joint-marg-cell" + (sel ? (on ? " is-sel" : " is-dim") : "") });
        gm.appendChild(svgEl("rect", {
          x: L + nx * CW + 6, y: T + j2 * CH + 2, width: MW - 12, height: CH - 4,
          class: "joint-bg"
        }));
        gm.appendChild(svgEl("rect", {
          x: L + nx * CW + 6, y: T + j2 * CH + 2, width: MW - 12, height: CH - 4,
          class: "joint-mfill", "fill-opacity": (0.16 + 0.84 * (py[j2] / maxMy)).toFixed(3)
        }));
        gm.appendChild(text(L + nx * CW + 6 + (MW - 12) / 2, T + j2 * CH + CH / 2 + 5,
          py[j2].toFixed(2), "joint-num"));
        svg.appendChild(gm);
      }
      for (var i3 = 0; i3 < nx; i3++) {
        var on2 = sel && sel.i === i3;
        var gm2 = svgEl("g", { class: "joint-marg-cell" + (sel ? (on2 ? " is-sel" : " is-dim") : "") });
        gm2.appendChild(svgEl("rect", {
          x: L + i3 * CW + 2, y: T + ny * CH + 6, width: CW - 4, height: MH - 12,
          class: "joint-bg"
        }));
        gm2.appendChild(svgEl("rect", {
          x: L + i3 * CW + 2, y: T + ny * CH + 6, width: CW - 4, height: MH - 12,
          class: "joint-mfill", "fill-opacity": (0.16 + 0.84 * (px[i3] / maxMx)).toFixed(3)
        }));
        gm2.appendChild(text(L + i3 * CW + CW / 2, T + ny * CH + 6 + (MH - 12) / 2 + 5,
          px[i3].toFixed(2), "joint-num"));
        svg.appendChild(gm2);
      }
      svg.appendChild(text(L - 8, T + ny * CH + 6 + (MH - 12) / 2 + 5, "주변확률", "joint-axis joint-axis-y"));
      svg.appendChild(text(L + nx * CW + 6 + (MW - 12) / 2, T - 6, "주변확률", "joint-axis"));
      return svg;
    }

    // ── 오른쪽 위에서 비스듬히 보기 (등각투상) ──────────────────
    // 격자 좌표 (i, j)와 높이 h를 화면 좌표로 옮긴다. 등각투상이라 깊이와
    // 관계없이 같은 비율이고, 그래서 뒤쪽 막대가 작아 보이지 않는다.
    function iso(i, j, h) {
      // **같은 대각선(i-j가 같은) 칸들은 화면에서 x가 똑같다.** 그래서 그
      // 칸들의 숫자 라벨은 세로로만 갈리는데, 높이 차이가 깊이 차이를 거의
      // 지우면 두 라벨이 겹친다. 실제로 (흐림, 산책)=0.10과 (비, 집)=0.30이
      // 14px까지 붙었다. 깊이 간격 V를 키우고 높이 배율 HZ를 줄여 그 최소
      // 간격을 32px로 벌렸다.
      var U = 46, V = 32, HZ = 160;   // 가로/세로 한 칸, 확률 1의 높이
      return {
        x: (i - j) * U,
        y: (i + j) * V - h * HZ
      };
    }

    function drawIso() {
      var pts = [];
      // 바닥 네 귀퉁이로 화면 크기를 잡는다. 가장 높은 막대의 꼭대기도 넣는다.
      for (var j = 0; j <= ny; j++) for (var i = 0; i <= nx; i++) pts.push(iso(i, j, 0));
      // 주변확률 막대가 서는 바깥 줄과, 가장 높은 막대의 꼭대기까지 넣는다.
      // 안 넣으면 그것들이 viewBox 밖으로 잘려 나간다.
      for (var jj = 0; jj <= ny; jj++) pts.push(iso(-1.15, jj, 0));
      for (var ii = 0; ii <= nx; ii++) pts.push(iso(ii, -1.15, 0));
      var maxM = Math.max(Math.max.apply(null, px), Math.max.apply(null, py));
      pts.push(iso(0, 0, maxP));
      pts.push(iso(-1.15, 0, maxM));
      pts.push(iso(nx, -1.15, maxM));
      var minX = 1e9, maxX = -1e9, minY = 1e9, maxY = -1e9;
      pts.forEach(function (p) {
        minX = Math.min(minX, p.x); maxX = Math.max(maxX, p.x);
        minY = Math.min(minY, p.y); maxY = Math.max(maxY, p.y);
      });
      var pad = 34;
      var W = (maxX - minX) + pad * 2, H = (maxY - minY) + pad * 2;
      var ox = -minX + pad, oy = -minY + pad;
      var svg = svgEl("svg", {
        viewBox: "0 0 " + W.toFixed(1) + " " + H.toFixed(1), class: "joint-svg",
        role: "img", "aria-label": "결합확률을 오른쪽 위에서 비스듬히 본 3차원 막대그림"
      });
      function P(i, j, h) { var q = iso(i, j, h); return (q.x + ox).toFixed(2) + "," + (q.y + oy).toFixed(2); }

      // 바닥 격자를 먼저 깐다.
      for (var j1 = 0; j1 < ny; j1++) {
        for (var i1 = 0; i1 < nx; i1++) {
          svg.appendChild(svgEl("polygon", {
            points: [P(i1, j1, 0), P(i1 + 1, j1, 0), P(i1 + 1, j1 + 1, 0), P(i1, j1 + 1, 0)].join(" "),
            class: "joint-floor"
          }));
        }
      }

      // **결합 막대와 주변 막대를 한 목록에 담아 함께 정렬한다.** 예전에는
      // 노란 막대를 다 그린 뒤 초록 막대를 그렸는데, 그러면 뒤에 있어야 할
      // 초록 막대가 앞의 노란 막대를 덮었다. 깊이는 종류가 아니라 자리가
      // 정하는 것이라 한 번에 세워야 한다.
      var boxes = [];
      for (var j2 = 0; j2 < ny; j2++) {
        for (var i2 = 0; i2 < nx; i2++) {
          boxes.push({ i0: i2, j0: j2, i1: i2 + 1, j1: j2 + 1, h: d.p[j2][i2],
            kind: "joint", i: i2, j: j2 });
        }
      }
      // 주변확률은 바닥 바깥 줄에 세운다. 결합확률(노랑) 옆에 나란히 서야
      // "노란 칸을 초록 줄로 나눈 것이 조건부확률"이라는 말이 그림에서 읽힌다.
      for (var jm = 0; jm < ny; jm++) {
        boxes.push({ i0: -1.15, j0: jm, i1: -0.15, j1: jm + 1, h: py[jm], kind: "marg", j: jm });
      }
      for (var im = 0; im < nx; im++) {
        boxes.push({ i0: im, j0: -1.15, i1: im + 1, j1: -0.15, h: px[im], kind: "marg", i: im });
      }
      // i+j가 작을수록 뒤다(화가 알고리즘). 바깥 줄은 좌표가 음수라 저절로
      // 맨 뒤로 간다.
      boxes.sort(function (a, b) { return (a.i0 + a.j0) - (b.i0 + b.j0); });

      var labels = [];

      boxes.forEach(function (c) {
        var joint = c.kind === "joint";
        var pre = joint ? "joint" : "joint-m";
        var on = sel && (joint ? (sel.i === c.i && sel.j === c.j)
          : (c.i !== undefined ? sel.i === c.i : sel.j === c.j));
        var attrs = joint
          ? { class: cellClass(c.i, c.j), tabindex: "0", role: "button",
              "aria-label": d.xs[c.i] + " " + d.ys[c.j] + " 결합확률 " + c.h.toFixed(2) }
          : { class: "joint-marg-cell" + (sel ? (on ? " is-sel" : " is-dim") : "") };
        var g = svgEl("g", attrs);
        // 윗면
        g.appendChild(svgEl("polygon", {
          points: [P(c.i0, c.j0, c.h), P(c.i1, c.j0, c.h), P(c.i1, c.j1, c.h), P(c.i0, c.j1, c.h)].join(" "),
          class: pre + "-top"
        }));
        // 왼쪽 면 (j가 커지는 쪽)
        g.appendChild(svgEl("polygon", {
          points: [P(c.i0, c.j1, c.h), P(c.i1, c.j1, c.h), P(c.i1, c.j1, 0), P(c.i0, c.j1, 0)].join(" "),
          class: pre + "-side"
        }));
        // 오른쪽 면 (i가 커지는 쪽)
        g.appendChild(svgEl("polygon", {
          points: [P(c.i1, c.j0, c.h), P(c.i1, c.j1, c.h), P(c.i1, c.j1, 0), P(c.i1, c.j0, 0)].join(" "),
          class: pre + "-face"
        }));
        if (joint) bindPick(g, c.i, c.j);
        svg.appendChild(g);

        // 라벨은 윗면 한가운데 바로 위에 둔다. 글자에 지면색 테두리를 둘러
        // (CSS의 paint-order) 뒤 막대와 겹쳐도 읽힌다.
        var lab = iso((c.i0 + c.i1) / 2, (c.j0 + c.j1) / 2, c.h);
        var lg = svgEl("g", { class: (joint ? cellClass(c.i, c.j) : "") + " joint-lab" });
        lg.appendChild(text(lab.x + ox, lab.y + oy - 7, c.h.toFixed(2), "joint-num3"));
        labels.push(lg);
      });

      // **라벨은 막대를 전부(결합 + 주변) 세운 뒤 한 번에 얹는다.** 중간에
      // 얹으면 나중에 그린 막대가 앞선 라벨을 덮는다.
      labels.forEach(function (lg) { svg.appendChild(lg); });

      // 축 이름은 바닥에서 충분히 떨어뜨린다. 가까이 두면 막대 옆면에 묻힌다.
      var xm = iso(nx / 2, ny + 0.75, 0), ym = iso(nx + 0.75, ny / 2, 0);
      svg.appendChild(text(xm.x + ox, xm.y + oy, d.xName, "joint-axis"));
      svg.appendChild(text(ym.x + ox, ym.y + oy, d.yName, "joint-axis"));
      return svg;
    }

    function text(x, y, s, cls) {
      var t = svgEl("text", { x: x.toFixed(2), y: y.toFixed(2), class: cls });
      t.textContent = s;
      return t;
    }

    // 누르는 길을 마우스와 키보드 양쪽으로 연다. SVG 안이라 <button>을
    // 쓸 수 없어서 role과 tabindex를 손으로 단다.
    function bindPick(g, i, j) {
      g.addEventListener("click", function () { pick(i, j); });
      g.addEventListener("keydown", function (ev) {
        if (ev.key === "Enter" || ev.key === " ") { ev.preventDefault(); pick(i, j); }
      });
    }

    function draw() {
      stage.textContent = "";
      stage.appendChild(mode === "top" ? drawTop() : drawIso());
      tell();
    }

    draw();
  }

  // **이름표는 여기 하나뿐이다.** 본문의 `:::anim 이름`이 이 표를 찾는다.
  var COMPONENTS = {
    "sort-bubble": sortBubble,
    "binary-search": binarySearch,
    "joint-probability": jointProbability,
  };

  function mount(root) {
    var hosts = (root || document).querySelectorAll(".anim[data-anim]");
    for (var i = 0; i < hosts.length; i++) {
      var host = hosts[i];
      if (host.dataset.mounted === "on") continue;
      var make = COMPONENTS[host.dataset.anim];
      if (!make) {
        // **모르는 이름은 조용히 넘기지 않는다.** 서버가 적어둔 대체 글자를
        // 그대로 두고 무엇이 잘못됐는지 덧붙인다.
        var p = host.querySelector(".anim-fallback");
        if (p) p.textContent = "그런 애니메이션이 없다: " + host.dataset.anim;
        host.dataset.mounted = "on";
        continue;
      }
      host.dataset.mounted = "on";
      try {
        make(host);
      } catch (e) {
        host.textContent = "";
        host.appendChild(el("p", { class: "anim-fallback", text: "애니메이션을 그리지 못했다." }));
        if (window.console) console.error("[anim]", host.dataset.anim, e);
      }
    }
  }

  // admin 미리보기가 본문을 다시 그릴 때마다 부른다 — math.js·highlight-init.js와
  // 같은 자리다.
  window.blogMountAnims = mount;

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () { mount(); });
  } else {
    mount();
  }
})();
