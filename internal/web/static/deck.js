// 갈래 카드가 포인터를 따라 기울게 한다.
//
// **없어도 된다.** CSS만으로도 hover하면 고정 각도로 기울고 아이콘이 떠 있다
// (layout.html의 .deck-card). 이 스크립트는 그 각도를 포인터 위치에 맞춰
// 바꿔줄 뿐이라, 못 뜨면 원래의 CSS 동작이 그대로 남는다.
(function () {
  "use strict";

  // 움직임을 줄여달라고 한 사람에게는 아무것도 하지 않는다.
  if (window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;

  var MAX = 9; // 기울기 최대 각도(도). 더 주면 글자가 읽기 힘들어진다.

  function bind(card) {
    function tilt(e) {
      var r = card.getBoundingClientRect();
      // -0.5 ~ 0.5. 카드 한가운데가 0이다.
      var dx = (e.clientX - r.left) / r.width - 0.5;
      var dy = (e.clientY - r.top) / r.height - 0.5;
      card.style.transform =
        "translateY(-5px) rotateX(" + (-dy * 2 * MAX).toFixed(2) + "deg)" +
        " rotateY(" + (dx * 2 * MAX).toFixed(2) + "deg)";
    }
    function reset() { card.style.transform = ""; }

    card.addEventListener("pointermove", tilt);
    card.addEventListener("pointerleave", reset);
    // 키보드로 옮겨온 초점에는 각도를 지어내지 않는다. CSS 기본값을 쓴다.
    card.addEventListener("blur", reset);
  }

  function init() {
    var cards = document.querySelectorAll(".deck-card");
    for (var i = 0; i < cards.length; i++) bind(cards[i]);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
