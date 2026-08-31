// 본문 코드 블록의 오른쪽 위 모서리에 복사 버튼을 단다.
//
// **서버가 HTML로 미리 찍지 않고 여기서 만든다.** 클립보드는 스크립트 없이
// 쓸 수 없으므로, 템플릿이 버튼을 찍어두면 스크립트가 꺼진 브라우저에는
// 눌러도 아무 일이 없는 죽은 버튼이 남는다. youtube.js가 누르기 전까지 그냥
// 링크로 있는 것과 같은 점진적 향상이다.
//
// **navigator.clipboard는 보안 컨텍스트(HTTPS/localhost)에만 있다.** 없으면
// 버튼을 아예 만들지 않는다 — 같은 이유로, 못 하는 일에 버튼을 두지 않는다.
//
// 버튼은 `<pre>`가 아니라 껍데기 `.codeblock`에 붙인다. **pre는 가로로
// 스크롤하므로**(article pre { overflow-x: auto }) 그 안에 absolute로 두면
// 긴 줄을 밀 때 버튼이 코드와 함께 흘러간다. 껍데기는 스크롤하지 않는다.
(function () {
  "use strict";

  // 아이콘 세 개를 한꺼번에 넣어두고 **어느 것을 보일지는 CSS가 정한다**
  // (.copy[data-state="…"]). JS는 data-state 한 글자만 바꾼다.
  var ICONS =
    '<svg class="copy-ico copy-ico-copy" viewBox="0 0 20 20" aria-hidden="true">' +
      '<path d="M13 7.25V5.5A2.25 2.25 0 0 0 10.75 3.25h-5.5A2.25 2.25 0 0 0 3 5.5v5.5a2.25 2.25 0 0 0 2.25 2.25H7"/>' +
      '<rect x="7" y="7" width="10" height="10" rx="2.25"/>' +
    '</svg>' +
    '<svg class="copy-ico copy-ico-done" viewBox="0 0 20 20" aria-hidden="true">' +
      '<path d="M4 10.6l4 4 8-9.2"/>' +
    '</svg>' +
    '<svg class="copy-ico copy-ico-fail" viewBox="0 0 20 20" aria-hidden="true">' +
      '<path d="M5.5 5.5l9 9M14.5 5.5l-9 9"/>' +
    '</svg>';

  // 서버가 내보내는 글자는 한국어이고 사전이 세 언어로 바꾼다. 템플릿이
  // `aria-label="분류" data-i18n-aria-label="categories"`로 쓰는 것과 같은 꼴이라,
  // preferences.js가 못 떠도 한국어 라벨은 남는다.
  var LABELS = {
    idle: { key: "copyCode", ko: "코드 복사" },
    done: { key: "copyCodeDone", ko: "복사했습니다" },
    fail: { key: "copyCodeFailed", ko: "복사하지 못했습니다" }
  };

  function setState(btn, state) {
    var label = LABELS[state] || LABELS.idle;
    btn.dataset.state = state;
    btn.dataset.i18nAriaLabel = label.key;
    btn.setAttribute("aria-label", label.ko);
    // 지금 고른 언어로 바꾼다. preferences.js가 열어둔 것이고, 없으면 한국어로
    // 남는다 — 사전이 없을 때 조용히 원문으로 두는 다른 자리와 같다.
    if (window.blogLabel) window.blogLabel(btn);
  }

  function rest(btn) {
    if (btn.blogCopyTimer) clearTimeout(btn.blogCopyTimer);
    btn.blogCopyTimer = setTimeout(function () { setState(btn, "idle"); }, 1600);
  }

  function copy(btn, pre) {
    // 라벨(.lang)은 껍데기에 있고 pre 밖이라 애초에 섞이지 않는다.
    // code가 있으면 그쪽이 정확하다 — highlight.js가 span을 쌓아도
    // textContent는 원래 코드 그대로다.
    var code = pre.querySelector("code");
    var text = (code || pre).textContent;

    var done = function () { setState(btn, "done"); rest(btn); };
    var failed = function () { setState(btn, "fail"); rest(btn); };

    try {
      var p = navigator.clipboard.writeText(text);
      // 옛 구현은 Promise를 안 주기도 한다. 그때는 성공으로 본다.
      if (p && typeof p.then === "function") p.then(done, failed);
      else done();
    } catch (e) {
      // **조용히 실패하지 않는다.** 눌렀는데 아무 표시가 없으면 복사된 줄 안다.
      failed();
    }
  }

  // 껍데기가 없는 <pre>도 있다. codeblock.go는 **펜스** 코드 블록만 감싸므로,
  // 들여쓰기(4칸) 코드 블록이 다시 생기면 <pre>만 홀로 나온다(현재 0개).
  // 그런 것은 여기서 껍데기를 씌우고 나서 버튼을 단다 — 안 그러면 붙일 기준이
  // 없어서 버튼이 스크롤하는 pre 안으로 들어간다.
  function shellFor(pre) {
    var parent = pre.parentNode;
    if (parent && parent.classList && parent.classList.contains("codeblock")) return parent;
    var shell = document.createElement("div");
    shell.className = "codeblock";
    parent.insertBefore(shell, pre);
    shell.appendChild(pre);
    return shell;
  }

  function addButtons() {
    if (!navigator.clipboard || typeof navigator.clipboard.writeText !== "function") return;

    var pres = document.querySelectorAll("article pre");
    for (var i = 0; i < pres.length; i++) {
      (function (pre) {
        var shell = shellFor(pre);
        if (shell.dataset.copy === "on") return; // 이미 달았다

        var btn = document.createElement("button");
        btn.type = "button";
        btn.className = "copy";
        btn.innerHTML = ICONS; // 우리가 쓴 고정 문자열이다. 바깥 값이 섞이지 않는다.
        setState(btn, "idle");
        btn.addEventListener("click", function () { copy(btn, pre); });

        shell.appendChild(btn);
        // 언어 라벨을 버튼만큼 왼쪽으로 민다. 둘 다 오른쪽 위 모서리를 쓴다.
        shell.dataset.copy = "on";
      })(pres[i]);
    }
  }

  // math.js / highlight-init.js와 같은 이유로 연다 — admin 미리보기가 본문을
  // 다시 그릴 때마다 불러야 새로 들어온 코드 블록에도 버튼이 달린다.
  // 이미 달린 것은 위에서 건너뛰므로 몇 번을 불러도 같다.
  window.blogCopyButtons = addButtons;

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", addButtons);
  } else {
    addButtons();
  }
})();
