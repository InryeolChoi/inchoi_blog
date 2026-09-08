// 커서가 수식 안에 들어가면 **그 자리 바로 위에 그린 수식이 뜬다.**
//
// # 왜 이것만 따로 있나
//
// 편집기는 왼쪽에 마크다운, 오른쪽에 미리보기다. 대부분의 글에는 그걸로
// 충분한데 **수식만은 다르다** — `\dfrac{\partial}{\partial x}`는 글자로
// 봐서는 맞게 썼는지 알 수 없고, 오른쪽까지 눈을 옮겨 찾는 사이에 어디를
// 고치고 있었는지 놓친다. 노션이 수식만 따로 상자를 띄우는 이유와 같다.
//
// **서버에 안 묻는다.** KaTeX가 이미 이 페이지에 있으므로 치는 즉시 그린다 —
// 미리보기는 왕복이 있어 못 하는 일이다. `/` 팔레트가 서버에 안 묻는 것과
// 같은 판단이다.
//
// # 실패를 숨기지 않는다
//
// 쓰는 도중에는 수식이 거의 늘 미완성이라 실패가 정상이다. 그렇다고 조용히
// 옛 그림을 남겨두면 **다 고친 줄 알고 넘어간다.** 안 되는 동안에는 안 된다고
// 적고, 되는 순간 그림으로 바뀐다 — 그 전환이 곧 "이제 맞다"는 신호다.
(function () {
  "use strict";

  // 코드 펜스 안은 건드리지 않는다. 셸의 `$PATH`나 R의 `data1$col`이 수식으로
  // 둔갑하면, 고칠 것이 없는 자리에 상자가 뜬다.
  function inFence(text, pos) {
    var fences = 0;
    var lines = text.slice(0, pos).split("\n");
    for (var i = 0; i < lines.length; i++) {
      if (/^\s*(```|~~~)/.test(lines[i])) fences++;
    }
    return fences % 2 === 1;
  }

  // mathAt은 pos가 들어 있는 수식을 찾는다. 없으면 null이다.
  //
  // **`$$`를 먼저 본다.** `$` 규칙으로 먼저 훑으면 `$$x$$`의 여는 기호가
  // 빈 인라인 수식으로 잡힌다.
  //
  // 인라인 수식은 **한 줄 안에서만** 닫힌다. 변환기가 그것을 보장하고
  // (normalizeInlineMath), 줄을 넘겨 찾으면 본문의 진짜 `$` 두 개가 짝을 지어
  // 문단 하나를 통째로 수식으로 만든다.
  function mathAt(text, pos) {
    if (inFence(text, pos)) return null;

    var i = 0;
    while (i < text.length) {
      var d = text.indexOf("$", i);
      if (d < 0) return null;
      var display = text.slice(d, d + 2) === "$$";
      var open = display ? d + 2 : d + 1;
      var close;
      if (display) {
        close = text.indexOf("$$", open);
        if (close < 0) return null;
      } else {
        var nl = text.indexOf("\n", open);
        close = text.indexOf("$", open);
        if (close < 0 || (nl >= 0 && close > nl)) {
          // 짝이 없는 `$`다. 그 하나만 지나치고 다음부터 다시 본다 —
          // 통째로 포기하면 뒤에 있는 멀쩡한 수식까지 못 찾는다.
          i = d + 1;
          continue;
        }
      }
      var end = close + (display ? 2 : 1);
      if (pos >= d && pos <= end) {
        return { latex: text.slice(open, close), display: display };
      }
      i = end;
    }
    return null;
  }

  function box() {
    var node = document.getElementById("blog-math-live");
    if (!node) {
      node = document.createElement("div");
      node.id = "blog-math-live";
      node.className = "math-live";
      node.setAttribute("aria-hidden", "true"); // 읽어줄 것은 원문 쪽에 있다.
      document.body.appendChild(node);
    }
    return node;
  }

  function hide() {
    var node = document.getElementById("blog-math-live");
    if (node) node.hidden = true;
  }

  function show(area) {
    if (!window.katex || !window.blogPalette || !window.blogPalette.caretXY) return;
    if (area.selectionStart !== area.selectionEnd) return hide(); // 고르는 중이다.

    var found = mathAt(area.value, area.selectionStart);
    if (!found || !found.latex.trim()) return hide();

    var node = box();
    node.hidden = false;
    node.className = "math-live";
    try {
      node.innerHTML = window.katex.renderToString(found.latex, {
        displayMode: found.display,
        throwOnError: true,
      });
    } catch (e) {
      // **KaTeX의 빨간 그림을 쓰지 않는다**(throwOnError: false). 여기서
      // 실패하는 것은 대개 아직 다 안 쓴 식이라, 빨간 덩어리가 떴다 사라졌다
      // 하면 화면만 시끄럽고 무엇이 잘못됐는지는 그 글자에 없다.
      node.className = "math-live is-bad";
      node.textContent = String(e.message || e).replace(/^KaTeX parse error:\s*/, "");
    }

    var xy = window.blogPalette.caretXY(area);
    var w = node.offsetWidth, h = node.offsetHeight;
    var left = Math.max(8, Math.min(xy.x, window.innerWidth - w - 8));
    // 기본은 커서 **위**다. 아래에 두면 지금 치고 있는 줄을 가린다.
    var top = xy.y - h - 8;
    if (top < 8) top = xy.y + xy.lineHeight + 8; // 위가 좁으면 아래로 넘긴다.
    node.style.left = left + "px";
    node.style.top = top + "px";
  }

  // attach는 텍스트에어리어 하나에 붙인다. 팔레트와 같은 모양의 API다.
  function attach(area) {
    if (!area || area.dataset.mathLive) return;
    area.dataset.mathLive = "1";
    var tick = function () { show(area); };
    // **selectionchange를 안 쓴다.** 문서 전체에 걸리는 이벤트라 이 칸과
    // 상관없는 움직임에도 돌고, 옛 사파리에는 아예 없다.
    ["input", "keyup", "click", "scroll"].forEach(function (ev) {
      area.addEventListener(ev, tick);
    });
    area.addEventListener("blur", hide);
    // 편집기를 닫으면 칸이 통째로 사라지는데, 상자는 body에 붙어 있어서
    // 그대로 떠 있는다. 화면에서 사라진 칸에 딸린 상자는 유령이다.
    window.addEventListener("scroll", hide, true);
  }

  window.blogMathLive = { attach: attach, hide: hide, mathAt: mathAt };
})();
