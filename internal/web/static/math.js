// 본문의 수식을 KaTeX로 그린다.
//
// **auto-render 확장을 쓰지 않는다.** 그건 문서 전체를 훑으며 $...$를 찾는데,
// 이 블로그에는 본문에 진짜 $ 문자를 쓴 글이 90개 있다(R의 data1$col, Makefile
// 변수, 정규식 등). 그중 20개는 수식도 같이 있어서 auto-render가 본문 $와
// 수식 $의 짝을 잘못 지을 수 있다.
//
// 서버가 이미 수식을 골라 .math 안에 원문 LaTeX만 넣어놨으므로($ 없이),
// 그 요소만 집어서 그린다. 훑을 일도, 잘못 짝지을 일도 없다.
(function () {
  "use strict";

  function renderAll() {
    if (typeof katex === "undefined") {
      // CDN이 막혔거나 무결성 검사에 걸렸다. 원문 LaTeX가 그대로 보이는 편이
      // 빈 자리보다 낫다. 아무것도 하지 않는다.
      return;
    }

    var nodes = document.querySelectorAll(".math:not(.katex-done):not(.katex-failed)");
    for (var i = 0; i < nodes.length; i++) {
      var el = nodes[i];
      // textContent가 곧 원문이다. 서버가 HTML 이스케이프만 해서 내보냈다.
      var src = el.textContent;
      var html;
      try {
        // **먼저 문자열로 뽑고 성공했을 때만 넣는다.** katex.render는 실패하면
        // 요소를 반쯤 고쳐놓을 수 있다. 이렇게 하면 실패한 수식은 원문 그대로
        // 남는다.
        //
        // throwOnError를 켠 이유: 끄면 KaTeX가 못 읽은 원문을 자기가 빨갛게
        // 그려버리는데, 여기서 실패하는 것들은 대개 수식이 아니라 **본문이
        // 통째로 딸려 들어온 것**이라(아래 참고) 빨간 덩어리가 되면 오히려
        // 읽기 힘들다. 실패하면 손대지 않는 편이 낫다.
        html = katex.renderToString(src, {
          displayMode: el.classList.contains("math-display"),
          throwOnError: true,
          // 유니코드 글자(①, 한글 등)에 경고만 내고 넘어간다.
          strict: false,
        });
      } catch (e) {
        // 원문을 그대로 두고 표시만 남긴다. 개발자 도구에서
        // `document.querySelectorAll('.katex-failed')`로 찾을 수 있다.
        el.classList.add("katex-failed");
        el.title = "수식으로 읽지 못했다: " + e.message;
        continue;
      }
      el.innerHTML = html;
      el.classList.add("katex-done");
    }
  }

  // defer 스크립트끼리는 순서가 보장되고 DOM도 다 만들어져 있다.
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", renderAll);
  } else {
    renderAll();
  }
})();
