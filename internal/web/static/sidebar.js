// 사이드바 아코디언과 좁은 화면의 서랍.
//
// 이 스크립트가 못 떠도 길이 막히지는 않는다. 지금 보고 있는 곳까지는 서버가
// 이미 펼쳐서 보내고(markNav), 좁은 화면에서도 사이드바는 DOM에 그대로 있다.
// 여기서 더해주는 것은 "다른 가지를 눌러서 열기"와 "서랍 여닫기"뿐이다.
(function () {
  "use strict";

  var side = document.getElementById("side");
  if (!side) return;

  // ── 아코디언 ───────────────────────────────────────────────────────
  // 최상위 8개는 늘 보이고, 그 아래(19 + 66)는 접혀 있다. 한 번에 다 펼치면
  // 목록이 화면보다 길어져서 지금 어디 있는지가 오히려 안 보인다.
  side.addEventListener("click", function (e) {
    var twist = e.target.closest(".nav-twist");
    if (!twist || !side.contains(twist)) return;
    var item = twist.closest(".nav-item");
    if (!item) return;
    var open = item.classList.toggle("is-open");
    twist.setAttribute("aria-expanded", open ? "true" : "false");
  });

  // 현재 위치가 사이드바 스크롤 밖에 있으면 보이게 올려준다. 93줄짜리
  // 목록이라 깊은 분류를 열면 표시가 접힌 자리 아래로 내려가 있다.
  var current = side.querySelector('.nav-link[aria-current="page"]');
  if (current) {
    var box = side.getBoundingClientRect();
    var mark = current.getBoundingClientRect();
    if (mark.top < box.top || mark.bottom > box.bottom) {
      // 화면 전체가 아니라 사이드바 안에서만 움직인다. block:"nearest"가
      // 그 일을 한다 — "center"로 두면 본문까지 같이 스크롤된다.
      current.scrollIntoView({ block: "nearest" });
    }
  }

  // ── 좁은 화면의 서랍 ───────────────────────────────────────────────
  var burger = document.getElementById("burger");
  var scrim = document.getElementById("scrim");
  if (!burger || !scrim) return;

  function setOpen(open) {
    document.body.classList.toggle("nav-open", open);
    burger.setAttribute("aria-expanded", open ? "true" : "false");
    scrim.hidden = !open;
    // 서랍이 열려 있는 동안 뒤쪽 본문이 같이 스크롤되지 않게 한다.
    document.body.style.overflow = open ? "hidden" : "";
    if (open) {
      var first = side.querySelector("a, button");
      if (first) first.focus();
    } else {
      burger.focus();
    }
  }

  burger.addEventListener("click", function () {
    setOpen(!document.body.classList.contains("nav-open"));
  });
  scrim.addEventListener("click", function () { setOpen(false); });

  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape" && document.body.classList.contains("nav-open")) {
      setOpen(false);
    }
  });

  // 넓은 화면으로 돌아가면 사이드바가 고정 자리로 돌아간다. 열림 상태가
  // 남아 있으면 scrim과 body의 스크롤 잠금만 유령처럼 남는다.
  var wide = window.matchMedia("(min-width: 60rem)");
  var onWide = function (e) { if (e.matches) setOpen(false); };
  if (wide.addEventListener) wide.addEventListener("change", onWide);
  else if (wide.addListener) wide.addListener(onWide);
})();
