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

  // **이름표는 여기 하나뿐이다.** 본문의 `:::anim 이름`이 이 표를 찾는다.
  var COMPONENTS = {
    "sort-bubble": sortBubble,
    "binary-search": binarySearch,
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
