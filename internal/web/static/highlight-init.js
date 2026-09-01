// 본문의 코드 블록을 highlight.js로 색칠한다.
//
// hljs.highlightAll()을 쓰지 않는다. 그건 언어 클래스가 없거나 등록되지 않은
// 블록에 **자동 추측**을 돌리는데, 이 블로그에는 그러면 안 되는 것이 두 종류 있다:
//
//   text     265건 — 언어가 아니라 "색칠하지 말라"는 뜻이다
//   mermaid   30건 — hljs에 없는 언어다. 추측을 돌리면 엉뚱한 색이 칠해진다
//
// 그래서 등록된 언어가 붙어 있는 블록만 골라서 칠한다.
(function () {
  "use strict";

  function highlightAll() {
    if (typeof hljs === "undefined") {
      // CDN이 막혔거나 무결성 검사에 걸렸다. 색만 없을 뿐 코드는 그대로 읽힌다.
      return;
    }

    // 본문은 우리가 만든 것이고 goldmark가 이미 이스케이프해서 넣었다.
    // 그 경고는 꺼둔다.
    hljs.configure({ ignoreUnescapedHTML: true });

    var blocks = document.querySelectorAll("article pre > code[class*='language-']");
    for (var i = 0; i < blocks.length; i++) {
      var el = blocks[i];
      var m = /(?:^|\s)language-([\w+#-]+)/.exec(el.className);
      if (!m) continue;

      var lang = m[1].toLowerCase();
      // text/plaintext는 "칠하지 말라"는 뜻이라 등록돼 있어도 건너뛴다.
      if (lang === "text" || lang === "plaintext" || lang === "plain") continue;
      // mermaid는 다이어그램으로 그려진다(static/mermaid.js). assets.go의
      // skipLangs와 이 목록이 어긋나면, 스크립트를 안 받았는데 칠할 것이
      // 있거나 받았는데 칠할 것이 없다.
      if (lang === "mermaid") continue;
      // 모르는 언어는 그대로 둔다. hljs에 넘기면 콘솔에 경고만 쌓인다.
      if (!hljs.getLanguage(lang)) continue;

      try {
        hljs.highlightElement(el);
      } catch (e) {
        // 한 블록이 실패해도 나머지는 칠한다.
      }
    }
  }

  // math.js와 같은 이유로 연다 — admin 미리보기가 본문을 다시 그릴 때마다
  // 부른다. 이미 칠한 블록에는 hljs가 표시를 남기므로 다시 칠해도 같다.
  window.blogHighlight = highlightAll;

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", highlightAll);
  } else {
    highlightAll();
  }
})();
