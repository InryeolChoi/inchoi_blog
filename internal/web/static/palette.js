// `/`를 치면 뜨는 조각 팔레트.
//
// # 마크다운 위에 얹는다
//
// 노션처럼 보이지만 **정본은 계속 마크다운이다.** 고르면 마크다운 조각이 커서
// 자리에 꽂힐 뿐이고, 왼쪽 편집·오른쪽 미리보기 구조는 그대로다. 블록 JSON을
// 정본으로 두는 길은 렌더링 파이프라인을 통째로 다시 쓰고 1,357편을 되돌리는
// 일이라 안 간다(CLAUDE.md "정해둔 것 ①·②").
//
// # 빨라야 한다
//
// **서버에 묻지 않는다.** 목록이 아래 ITEMS 하나에 다 있고, 거르는 것은
// 부분 일치 하나다. `/`를 친 순간과 목록이 뜨는 순간 사이에 네트워크가 끼면
// 그 지연은 절대 감출 수 없다 — 미리보기(250ms 디바운스)와 정반대의 요구다.
// 화면도 목록이 열릴 때 한 번만 만들고 그다음부터는 보이기/숨기기만 한다.
//
// # 커서 자리에 뜬다
//
// 텍스트에어리어에는 캐럿 좌표를 알려주는 API가 없어서, 같은 글꼴·같은 폭의
// 거울 div에 커서 앞까지를 복사해 그 끝 위치를 잰다. 흔한 방법이고 이것 말고는
// 길이 없다.
(function () {
  "use strict";

  // $0은 꽂은 뒤 커서가 갈 자리다. 없으면 조각 끝이다.
  var ITEMS = [
    { label: "제목 2", hint: "## ", keys: "heading h2 제목", snip: "## $0" },
    { label: "제목 3", hint: "### ", keys: "heading h3 제목", snip: "### $0" },
    { label: "목록", hint: "- ", keys: "list bullet 목록 리스트", snip: "- $0" },
    { label: "번호 목록", hint: "1. ", keys: "list ordered number 번호 목록", snip: "1. $0" },
    { label: "인용", hint: "> ", keys: "quote blockquote 인용", snip: "> $0" },
    { label: "구분선", hint: "---", keys: "hr divider rule 구분선", snip: "---\n\n$0" },

    { label: "수식 (문장 안)", hint: "$…$", keys: "math katex latex inline 수식 인라인",
      snip: "$$0$" },
    { label: "수식 (블록)", hint: "$$…$$", keys: "math katex latex block display 수식 블록",
      snip: "$$\n$0\n$$" },

    { label: "코드 (javascript)", hint: "```js", keys: "code js javascript 코드 자바스크립트",
      snip: "```javascript\n$0\n```" },
    { label: "코드 (python)", hint: "```py", keys: "code python py 코드 파이썬",
      snip: "```python\n$0\n```" },
    { label: "코드 (go)", hint: "```go", keys: "code go golang 코드", snip: "```go\n$0\n```" },
    { label: "코드 (sql)", hint: "```sql", keys: "code sql 코드", snip: "```sql\n$0\n```" },
    { label: "코드 (bash)", hint: "```sh", keys: "code bash sh shell 코드 셸",
      snip: "```bash\n$0\n```" },
    { label: "코드 (언어 없이)", hint: "```", keys: "code plain text 코드", snip: "```\n$0\n```" },

    { label: "표", hint: "3칸", keys: "table 표 테이블",
      snip: "| 머리 | 머리 | 머리 |\n|---|---|---|\n| $0 |  |  |\n|  |  |  |" },

    { label: "애니메이션 (버블 정렬)", hint: ":::anim", keys: "anim animation 애니메이션 정렬 버블 sort",
      snip: ":::anim sort-bubble\n\n$0" },
    { label: "애니메이션 (이진 탐색)", hint: ":::anim", keys: "anim animation 애니메이션 이진 탐색 binary search",
      snip: ":::anim binary-search\n\n$0" },

    { label: "접기", hint: "<details>", keys: "details toggle fold 접기 토글",
      snip: "<details>\n<summary>$0</summary>\n\n내용\n\n</details>" },
    { label: "링크", hint: "[…](…)", keys: "link 링크", snip: "[$0](https://)" },
    // **이건 조각을 꽂지 않는다.** 파일 고르는 창을 연다 — 올린 뒤에 서버가
    // 준 마크다운이 커서 자리에 들어간다(admin.js의 imageBox).
    { label: "이미지 올리기", hint: "파일 고르기", keys: "image img picture 이미지 그림 사진",
      pick: true },
  ];

  // 거르기는 부분 일치 하나다. 한글은 초성 검색 같은 것을 하지 않는다 —
  // 목록이 스무 줄이라 그럴 값이 없고, 규칙이 늘면 예측이 어려워진다.
  function match(item, q) {
    if (!q) return true;
    return (item.label + " " + item.keys).toLowerCase().indexOf(q.toLowerCase()) >= 0;
  }

  // 커서 앞의 `/질의`를 찾는다. **줄 처음이나 공백 뒤의 `/`만 본다** —
  // 안 그러면 `https://`나 경로를 칠 때마다 팔레트가 튀어나온다.
  var TRIGGER = /(^|[\s(])\/([^\s/]{0,20})$/;

  function queryAt(area) {
    var upto = area.value.slice(0, area.selectionStart);
    var m = TRIGGER.exec(upto);
    if (!m) return null;
    return { q: m[2], from: area.selectionStart - m[2].length - 1 };
  }

  // 캐럿 좌표. 거울 div에 커서 앞까지를 복사해 그 끝을 잰다.
  var MIRROR_PROPS = [
    "boxSizing", "width", "paddingTop", "paddingRight", "paddingBottom", "paddingLeft",
    "borderTopWidth", "borderRightWidth", "borderBottomWidth", "borderLeftWidth",
    "fontFamily", "fontSize", "fontWeight", "lineHeight", "letterSpacing",
    "textIndent", "whiteSpace", "wordWrap", "overflowWrap", "tabSize",
  ];
  var mirror = null;

  function caretXY(area) {
    if (!mirror) {
      mirror = document.createElement("div");
      mirror.setAttribute("aria-hidden", "true");
      mirror.style.position = "absolute";
      mirror.style.visibility = "hidden";
      mirror.style.top = "0";
      mirror.style.left = "-9999px";
      document.body.appendChild(mirror);
    }
    var cs = getComputedStyle(area);
    for (var i = 0; i < MIRROR_PROPS.length; i++) {
      mirror.style[MIRROR_PROPS[i]] = cs[MIRROR_PROPS[i]];
    }
    mirror.style.whiteSpace = "pre-wrap";
    mirror.style.wordWrap = "break-word";
    mirror.textContent = area.value.slice(0, area.selectionStart);
    var end = document.createElement("span");
    // 빈 span은 높이가 0이라 자리를 못 잡는다. 폭 없는 글자 하나를 넣는다.
    end.textContent = "​";
    mirror.appendChild(end);

    var box = area.getBoundingClientRect();
    return {
      x: box.left + end.offsetLeft - area.scrollLeft,
      y: box.top + end.offsetTop - area.scrollTop,
      lineHeight: parseFloat(cs.lineHeight) || 20,
    };
  }

  // attach는 텍스트에어리어 하나에 팔레트를 붙인다.
  //
  // onPick(item)은 조각이 아닌 항목(이미지 올리기)을 부르는 쪽에 넘긴다.
  function attach(area, insert, onPick) {
    var box = document.createElement("div");
    box.className = "pal";
    box.setAttribute("role", "listbox");
    box.hidden = true;
    document.body.appendChild(box);

    var shown = [], at = 0, from = 0;

    function close() {
      if (box.hidden) return;
      box.hidden = true;
      area.removeAttribute("aria-expanded");
    }

    function render(q) {
      shown = ITEMS.filter(function (it) { return match(it, q); });
      if (!shown.length) return close();
      at = 0;
      // **한 번 만들고 다시 쓴다.** 열 때마다 DOM을 새로 만들면 그 값이
      // 그대로 지연으로 보인다.
      box.textContent = "";
      shown.forEach(function (it, i) {
        var row = document.createElement("div");
        row.className = "pal-row" + (i === 0 ? " on" : "");
        row.setAttribute("role", "option");
        var name = document.createElement("span");
        name.className = "pal-name";
        name.textContent = it.label;
        var hint = document.createElement("span");
        hint.className = "pal-hint mono";
        hint.textContent = it.hint;
        row.appendChild(name);
        row.appendChild(hint);
        // mousedown으로 받는다. click이면 그 전에 textarea가 포커스를 잃는다.
        row.addEventListener("mousedown", function (e) { e.preventDefault(); choose(i); });
        row.addEventListener("mousemove", function () { highlight(i); });
        box.appendChild(row);
      });
      var xy = caretXY(area);
      box.hidden = false;
      // 아래로 넘치면 커서 위로 올린다.
      var h = box.offsetHeight;
      var below = xy.y + xy.lineHeight;
      box.style.left = Math.min(xy.x, window.innerWidth - box.offsetWidth - 8) + "px";
      box.style.top = (below + h > window.innerHeight - 8 ? xy.y - h - 2 : below) + "px";
      area.setAttribute("aria-expanded", "true");
    }

    function highlight(i) {
      var rows = box.children;
      if (!rows.length) return;
      at = (i + rows.length) % rows.length;
      for (var k = 0; k < rows.length; k++) rows[k].className = "pal-row" + (k === at ? " on" : "");
      rows[at].scrollIntoView({ block: "nearest" });
    }

    function choose(i) {
      var it = shown[i];
      if (!it) return close();
      // `/질의`를 지우고 그 자리에 꽂는다.
      var head = area.value.slice(0, from);
      var tail = area.value.slice(area.selectionStart);
      close();
      if (it.pick) {
        area.value = head + tail;
        area.selectionStart = area.selectionEnd = head.length;
        area.dispatchEvent(new Event("input"));
        if (onPick) onPick(it);
        return;
      }
      var snip = it.snip;
      var caret = snip.indexOf("$0");
      var text = caret >= 0 ? snip.replace("$0", "") : snip;
      // 블록 조각은 제 줄에서 시작해야 한다. 앞에 글자가 있으면 줄을 바꾼다.
      if (/\n/.test(snip) && head && !/\n$/.test(head)) {
        text = "\n" + text;
        caret = caret >= 0 ? caret + 1 : caret;
      }
      area.value = head + text + tail;
      var pos = head.length + (caret >= 0 ? caret : text.length);
      area.selectionStart = area.selectionEnd = pos;
      area.focus();
      area.dispatchEvent(new Event("input"));
    }

    area.addEventListener("input", function () {
      var t = queryAt(area);
      if (!t) return close();
      from = t.from;
      render(t.q);
    });

    // **키 처리는 keydown에서 가로챈다.** 팔레트가 떠 있을 때 위/아래는
    // 커서를 옮기면 안 되고 Enter는 줄을 바꾸면 안 된다.
    area.addEventListener("keydown", function (e) {
      if (box.hidden) return;
      if (e.key === "ArrowDown") { e.preventDefault(); highlight(at + 1); }
      else if (e.key === "ArrowUp") { e.preventDefault(); highlight(at - 1); }
      else if (e.key === "Enter" || e.key === "Tab") { e.preventDefault(); choose(at); }
      else if (e.key === "Escape") { e.preventDefault(); close(); }
    });
    area.addEventListener("blur", close);
    area.addEventListener("scroll", close);
    window.addEventListener("resize", close);

    return { close: close };
  }

  window.blogPalette = { attach: attach, items: ITEMS };
})();
