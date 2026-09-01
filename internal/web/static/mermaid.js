// 본문의 ```mermaid 코드 블록을 다이어그램으로 그린다.
//
// **원문을 지우지 않는다.** 그린 SVG를 `<pre>` 옆에 끼워 넣고 CSS가 원문을
// 감춘다. 그래야 복사 버튼이 그대로 코드를 집어가고(copy.js는 `<code>`의
// textContent를 읽는다), 실패했을 때 되돌릴 것이 남는다. KaTeX 쪽에서
// **성공했을 때만** 원문을 대신하는 것과 같은 원칙이다.
//
// **CDN이 막히면 아무 일도 안 한다.** 그러면 예전처럼 색 없는 코드가 보인다 —
// 빈 자리가 되지 않는다.
(function () {
  "use strict";

  var started = false;

  // **색은 지면에서 가져온다.** mermaid의 기본 팔레트는 연보라 도형에 노란
  // 묶음 상자라, 잉크 괘선과 포인트 하나로 버티는 이 사이트에서 그 상자만
  // 남의 것으로 보인다. 토큰을 읽어 넘기면 라이트·다크가 저절로 따라오고,
  // 팔레트를 또 갈아엎어도 여기를 다시 고칠 일이 없다.
  function token(name, fallback) {
    var v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    return v || fallback;
  }

  function themeVars() {
    var ink = token("--ink", "#121415");
    var bg = token("--bg", "#ffffff");
    var surface = token("--surface", "#f5f6f5");
    var line = token("--line", "#c6cbc9");
    var dim = token("--ink-dim", "#5b6467");
    return {
      background: bg,
      primaryColor: surface, primaryTextColor: ink, primaryBorderColor: ink,
      secondaryColor: bg, secondaryTextColor: ink, secondaryBorderColor: line,
      tertiaryColor: bg, tertiaryTextColor: ink, tertiaryBorderColor: line,
      // 묶음 상자(subgraph)는 도형보다 한 단계 더 가라앉는다.
      clusterBkg: bg, clusterBorder: line,
      lineColor: ink, textColor: ink,
      edgeLabelBackground: surface,
      nodeBorder: ink, mainBkg: surface, titleColor: ink,
      noteBkgColor: surface, noteTextColor: ink, noteBorderColor: line,
      // 글꼴도 코드 블록과 같은 것을 쓴다. 도형 안의 글자가 본문 활자면
      // 코드에서 온 그림으로 안 읽힌다.
      fontFamily: token("--mono", "monospace"),
      fontSize: "13px",
      labelColor: dim
    };
  }

  function shellOf(pre) {
    var p = pre.parentElement;
    return p && p.classList.contains("codeblock") ? p : pre;
  }

  function renderAll() {
    if (typeof mermaid === "undefined") return;

    var blocks = document.querySelectorAll("pre > code.language-mermaid");
    if (!blocks.length) return;

    if (!started) {
      // startOnLoad를 끄는 이유는 highlightAll()을 안 쓰는 것과 같다. 그건
      // `.mermaid` 요소를 제 마음대로 찾아 그리는데, 우리 마크업은 코드
      // 블록이고 무엇을 그릴지는 우리가 고른다.
      mermaid.initialize({
        startOnLoad: false,
        securityLevel: "strict",
        // base라야 themeVariables가 먹는다. default/dark는 제 팔레트를 고집한다.
        theme: "base",
        themeVariables: themeVars(),
        flowchart: { curve: "linear", useMaxWidth: true }
      });
      started = true;
    }

    for (var i = 0; i < blocks.length; i++) {
      draw(blocks[i], i);
    }
  }

  function draw(code, i) {
    var pre = code.parentElement;
    var shell = shellOf(pre);
    var old = shell.querySelector(".mermaid-figure");
    if (old) old.remove();

    var id = "mermaid-" + Date.now().toString(36) + "-" + i;
    // mermaid 11의 render는 Promise다. 실패하면 원문이 그대로 남는다.
    var out;
    try {
      out = mermaid.render(id, code.textContent);
    } catch (e) {
      return;
    }
    if (!out || typeof out.then !== "function") return;
    out.then(function (res) {
      var fig = document.createElement("div");
      fig.className = "mermaid-figure";
      fig.innerHTML = res.svg;
      pre.insertAdjacentElement("afterend", fig);
      shell.classList.add("has-diagram");
    }).catch(function () {
      // 못 그린 도형은 원문 그대로 남는다. 반쯤 그린 것을 남기지 않는다.
      shell.classList.remove("has-diagram");
      // mermaid가 실패하면서 body 끝에 남기는 임시 요소를 치운다.
      var junk = document.getElementById("d" + id) || document.getElementById(id);
      if (junk && junk.parentElement === document.body) junk.remove();
    });
  }

  // 테마를 바꾸면 다시 그린다. preferences.js가 <html data-theme>을 갈아끼우므로
  // 그것만 보면 된다 — 두 스크립트가 서로를 알 필요가 없다.
  function watchTheme() {
    if (!window.MutationObserver) return;
    new MutationObserver(function () {
      started = false;
      var figs = document.querySelectorAll(".mermaid-figure");
      for (var i = 0; i < figs.length; i++) figs[i].remove();
      var shells = document.querySelectorAll(".codeblock.has-diagram");
      for (var j = 0; j < shells.length; j++) shells[j].classList.remove("has-diagram");
      renderAll();
    }).observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
  }

  // math.js·highlight-init.js와 같은 이유로 연다 — admin 미리보기가 본문을
  // 다시 그릴 때마다 부른다.
  window.blogRenderMermaid = renderAll;

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () { renderAll(); watchTheme(); });
  } else {
    renderAll();
    watchTheme();
  }
})();
