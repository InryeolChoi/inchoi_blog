// 유튜브 재생 자리를 눌렀을 때 그 자리에서 플레이어를 켠다.
//
// **누르기 전에는 유튜브에 아무 요청도 가지 않는다.** 섬네일도 받지 않는다 —
// 그것도 유튜브 서버에서 오는 것이라 글을 열기만 해도 독자 IP가 제3자에게 간다.
// 눌렀을 때 비로소 youtube-nocookie.com 플레이어를 끼운다.
//
// 이 스크립트가 못 뜨면 원래대로 유튜브로 가는 링크다. 빈 자리가 되지 않는다.
(function () {
  "use strict";

  function open(a, e) {
    var id = a.getAttribute("data-yt");
    if (!id) return; // 주소를 못 읽었다. 링크로 두는 편이 낫다.
    e.preventDefault();

    var src = "https://www.youtube-nocookie.com/embed/" + encodeURIComponent(id) +
      "?autoplay=1&rel=0";
    var list = a.getAttribute("data-yt-list");
    if (list) src += "&list=" + encodeURIComponent(list);

    var frame = document.createElement("iframe");
    frame.src = src;
    frame.title = a.querySelector(".ytembed-t") ? a.querySelector(".ytembed-t").textContent : "YouTube";
    frame.allow = "accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture";
    // **referrer를 아주 끊으면 안 된다.** 유튜브 임베드는 출처를 확인해서,
    // no-referrer면 "오류 153 · 동영상 플레이어 구성 오류"로 재생을 거부한다.
    // 여기서는 이미 사람이 눌러서 연결에 동의한 뒤라 출처(도메인)까지만 보낸다.
    frame.referrerPolicy = "strict-origin-when-cross-origin";
    frame.setAttribute("allowfullscreen", "");

    var box = document.createElement("div");
    box.className = "ytembed ytembed-on";
    box.appendChild(frame);
    a.parentNode.replaceChild(box, a);
  }

  function init() {
    var list = document.querySelectorAll("a.ytembed");
    for (var i = 0; i < list.length; i++) {
      (function (a) {
        a.addEventListener("click", function (e) { open(a, e); });
      })(list[i]);
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
