// 글 화면에서 **그 자리에서** 고친다.
//
// 로그인이 확인되면 글 제목 옆에 "고치기"가 나온다. 누르면 /admin으로 옮겨
// 가는 것이 아니라 본문이 있던 자리가 편집기가 된다 — 읽던 맥락을 잃지 않는다.
//
// # 서버가 이미 판단했다
//
// 이 파일은 **로그인이 확인된 요청에만 실려 나간다**(internal/web/server.go의
// WithEditor). 그러니 여기서 다시 "로그인했나"를 따지지 않는다. 화면이 스스로
// 권한을 판단하기 시작하면 그 판단이 진짜 관문인 줄 알게 되는데, 진짜 관문은
// 언제나 서버다 — 여기 있는 버튼을 지운다고 아무것도 못 하게 되지 않고,
// 억지로 눌러봐야 API가 401을 준다.
//
// # 뒷단은 admin과 같다
//
// /api/admin/posts/{slug}로 읽고 쓴다. 미리보기도 같은 /api/admin/preview다.
// 편집기를 두 벌 만들면 "여기서 본 것과 발행 뒤 화면이 같다"는 보장이 두
// 배로 깨지기 쉬워진다.
(function () {
  "use strict";

  var mount = document.querySelector("[data-inline-edit]");
  if (!mount) return;
  var slug = mount.getAttribute("data-inline-edit");
  var article = document.querySelector("article.post-body") || document.querySelector("article");
  if (!slug || !article) return;

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

  function api(method, path, body) {
    var opts = { method: method, headers: {} };
    if (body !== undefined) {
      opts.headers["Content-Type"] = "application/json";
      opts.body = JSON.stringify(body);
    }
    return fetch(path, opts).then(function (res) {
      return res.json().catch(function () {
        return { error: "응답을 읽지 못했다 (HTTP " + res.status + ")" };
      }).then(function (data) { return { ok: res.ok, status: res.status, data: data }; });
    });
  }

  // svg는 아이콘 하나를 만든다. path 문자열은 layout.html의 것과 같은 모양이다.
  function svg(d) {
    var n = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    n.setAttribute("viewBox", "0 0 16 16");
    n.setAttribute("aria-hidden", "true");
    var p = document.createElementNS("http://www.w3.org/2000/svg", "path");
    p.setAttribute("d", d);
    n.appendChild(p);
    return n;
  }

  // **커튼 안의 두 버튼을 여기서 만든다.** 서버가 미리 찍지 않는 이유는
  // 둘 다 JS 없이는 할 수 없는 일이어서다 — 미리 찍어두면 스크립트가 꺼진
  // 브라우저에 눌러도 아무 일 없는 죽은 버튼이 남는다(복사 버튼과 같은 판단).
  // 라벨을 따로 들고 있는다. **firstChild로 찾으면 안 된다** — 아이콘이
  // 앞에 붙으면서 그게 SVG가 되고, 불러오는 동안 글자를 바꾸는 자리가
  // 조용히 엉뚱한 요소를 가리킨다(실제로 그럴 뻔했다).
  var label = el("span", { text: "고치기" });
  var button = el("button", { type: "button", class: "edit-here", onclick: open }, [
    svg("M12.1 2.3a1.7 1.7 0 0 1 2.4 2.4l-.9.9-2.4-2.4.9-.9zM10.3 4.1l2.4 2.4-6.5 6.5-3.1.7.7-3.1 6.5-6.5z"),
    label,
  ]);
  mount.appendChild(button);
  mount.appendChild(el("button", { type: "button", class: "del-here", onclick: remove }, [
    svg("M6.5 1.5h3v1h3.5v1.5h-11V2.5H6.5v-1zM3.5 5h9l-.6 9.1a1 1 0 0 1-1 .9H5.1a1 1 0 0 1-1-.9L3.5 5z"),
    el("span", { text: "지우기" }),
  ]));

  // closeMenu는 커튼을 닫는다. 무엇을 고르든 커튼은 제 일을 마친 것이라
  // 열린 채로 두면 그 아래 편집기가 가려진다.
  var menu = mount.closest ? mount.closest("details.edit-menu") : null;
  function closeMenu() {
    if (menu) menu.open = false;
  }
  // 편집기가 열려 있는 동안에는 `편집` 자체를 감춘다. 고치는 중에 또
  // 열리면 같은 일을 두 번 시작하게 되고, 지우기는 지금 화면을 통째로
  // 없애는 행동이라 더 그렇다.
  function showMenu(on) {
    if (menu) menu.hidden = !on;
    else button.hidden = !on;
  }

  // ── 지우기 ────────────────────────────────────────────────────
  //
  // **admin의 흐름을 그대로 쓴다**(admin.js의 removeFromList). refs로 무엇을
  // 잃는지 먼저 묻고, 자식이 있으면 아예 막고, 잃을 것이 있으면 확인을 받는다.
  // 규칙을 두 곳에 따로 적으면 한쪽이 느슨해진다 — 되돌릴 수 없는 행동이라
  // 느슨해지는 쪽이 그대로 사고다.
  function remove() {
    closeMenu();
    api("GET", "/api/admin/posts/" + encodeURIComponent(slug) + "/refs").then(function (r) {
      if (!r.ok) return window.alert(r.data.error || "무엇이 걸리는지 알아내지 못했다");
      var refs = r.data;
      // 자식이 있으면 못 지운다. force로도 안 된다 — 되돌릴 수 없이 사슬이 끊긴다.
      if (refs.children && refs.children.length) {
        return window.alert("하위 글 " + refs.children.length + "편이 매달려 있다: " +
          refs.children.slice(0, 3).join(", ") +
          (refs.children.length > 3 ? " 외" : "") +
          "\n\n그것들을 먼저 옮기거나 지워라.");
      }
      var lose = [];
      if (refs.notion) lose.push("노션에서 온 글이라 다음 재이관이 되살린다 (진짜로 빼려면 internal/curation의 DropPosts에 적어야 한다)");
      if (refs.coverOf && refs.coverOf.length) lose.push("분류 " + refs.coverOf.join(", ") + "의 표지가 사라진다");
      if (refs.linkedFrom && refs.linkedFrom.length) lose.push("이 글을 가리키던 " + refs.linkedFrom.length + "편의 링크를 글자로 푼다");

      var msg = "\"" + (document.title || slug) + "\"을(를) 지운다.";
      if (lose.length) msg += "\n\n" + lose.map(function (l, i) { return (i + 1) + ". " + l; }).join("\n");
      msg += "\n\n되돌릴 수 없다. 지울까?";
      if (!window.confirm(msg)) return;

      // 지우기는 저장과 같은 rev 표를 요구한다. 지금 값을 한 번 더 받아온다 —
      // 그 사이에 다른 탭이 고쳤으면 거절된다.
      api("GET", "/api/admin/posts/" + encodeURIComponent(slug)).then(function (g) {
        if (!g.ok) return window.alert(g.data.error || "글을 못 가져왔다");
        api("DELETE", "/api/admin/posts/" + encodeURIComponent(slug),
          { rev: g.data.rev || "", force: lose.length > 0 }).then(function (d) {
          if (!d.ok) return window.alert(d.data.error || ("지우지 못했다 (HTTP " + d.status + ")"));
          // 지운 글의 화면에 머물 수 없다 — 새로고침하면 404다.
          location.href = "/";
        });
      });
    });
  }

  // 원래 화면을 그대로 들고 있다가 취소하면 되돌린다. 다시 그리면
  // 수식·코드 색칠·복사 버튼·애니메이션을 전부 다시 붙여야 하는데,
  // 그 목록은 언젠가 하나 빠진다.
  var saved = null;

  function open() {
    if (saved) return;
    closeMenu();
    button.disabled = true;
    label.textContent = "불러오는 중…";
    api("GET", "/api/admin/posts/" + encodeURIComponent(slug)).then(function (r) {
      button.disabled = false;
      label.textContent = "고치기";
      if (!r.ok) {
        // 401이면 세션이 풀린 것이다. 그 말을 그대로 보여준다 — "안 된다"만
        // 보여주면 다시 로그인하면 된다는 것을 알 수 없다.
        alert(r.status === 401
          ? "로그인이 풀렸다. /admin/login에서 다시 들어와라."
          : (r.data.error || "글을 못 가져왔다"));
        return;
      }
      show(r.data);
    });
  }

  function show(post) {
    saved = article.cloneNode(true);
    showMenu(false);
    // **편집 중에는 목차를 감춘다.** 목차는 저장된 본문에서 뽑은 것이라
    // 고치는 동안에는 이미 낡았고, 좁은 화면에서는 그것 때문에 정작 글
    // 쓰는 칸까지 한참 스크롤해야 한다.
    //
    // **무엇을 감출지는 CSS가 정한다**(`body.editing`). 여기서는 표시만
    // 뒤집는다 — 나중에 감출 것이 늘어도 이 파일은 안 고친다.
    document.body.classList.add("editing");

    var area = el("textarea", { class: "edit-body mono", spellcheck: "false" });
    area.value = post.body || "";
    var note = el("span", { class: "edit-note" });
    var titleInput = el("input", { class: "edit-title", type: "text", value: post.title || "" });

    var statusSel = el("select", { class: "edit-status" },
      ["draft", "unlisted", "published"].map(function (v) {
        var o = el("option", { value: v, text: v });
        if (v === post.status) o.selected = true;
        return o;
      }));

    // 공개 범위도 여기서 바꾼다. 되돌려 보내기만 하면 되지만(PUT은 통째로
    // 바꾸기다) **이 글이 지금 비공개라는 것을 화면이 말해줘야 한다** —
    // 로그인해서 보고 있으면 평소와 똑같이 보이므로 다른 단서가 없다.
    var visSel = el("select", { class: "edit-status" },
      ["public", "private"].map(function (v) {
        var o = el("option", { value: v, text: v === "private" ? "🔒 private" : "public" });
        if (v === (post.visibility || "public")) o.selected = true;
        return o;
      }));

    // ── 분류 · 계층 · 날짜 ────────────────────────────────────────
    //
    // 예전에는 이걸 `자세히 ↗`로 미뤘다. 그런데 **글을 쓰는 동안 알아야 할
    // 것이 본문만은 아니다** — 어느 분류에 들어가는지, 어느 글 아래인지가
    // 안 보이면 저장하고 나서야 엉뚱한 데 있는 걸 안다.
    //
    // **접어둔다.** 평소에는 본문이 주인공이라 네 칸이 늘 펼쳐져 있으면
    // 시끄럽다. <details>라 여는 일은 브라우저가 한다.
    //
    // **분류 목록은 서버에 묻는다**(/api/admin/categories). admin 편집기와
    // 같은 엔드포인트라 선택지가 갈릴 수 없다.
    function metaPanel() {
      var catSel = el("select", { class: "edit-meta-in" }, [
        el("option", { value: "", text: "(분류 없음)" }),
      ]);
      var parentIn = el("input", { class: "edit-meta-in", type: "text",
        value: post.parentSlug || "", placeholder: "부모 글의 slug" });
      var orderIn = el("input", { class: "edit-meta-in", type: "number",
        value: String(post.sortOrder || 0) });
      var dateIn = el("input", { class: "edit-meta-in", type: "date",
        value: (post.createdAt || "").slice(0, 10) });

      // 선택지는 늦게 온다. 그동안에도 지금 값은 잃지 않는다 — 못 가져오면
      // 옛 값 그대로 되돌려 보낸다(아래 read가 그 자리를 지킨다).
      var loaded = false;
      api("GET", "/api/admin/categories").then(function (r) {
        if (!r.ok) return;
        (r.data.categories || r.data || []).forEach(function (c) {
          var o = el("option", { value: String(c.id), text: c.path || c.name });
          if (post.categoryId && c.id === post.categoryId) o.selected = true;
          catSel.appendChild(o);
        });
        loaded = true;
      });

      var box = el("details", { class: "edit-meta" }, [
        el("summary", { text: "분류 · 계층 · 날짜" }),
        el("div", { class: "edit-meta-grid" }, [
          el("label", {}, [el("span", { text: "분류" }), catSel]),
          el("label", {}, [el("span", { text: "부모 글" }), parentIn]),
          el("label", {}, [el("span", { text: "형제 순서" }), orderIn]),
          el("label", {}, [el("span", { text: "작성일" }), dateIn]),
        ]),
      ]);

      return {
        box: box,
        // read는 저장에 실어 보낼 값이다. **PUT은 통째로 바꾸기라** 안 보낸
        // 칸은 비워진다 — 목록을 못 가져왔으면 사람이 고른 적이 없으므로
        // 옛 값을 그대로 돌려보낸다.
        read: function () {
          return {
            categoryId: loaded ? (catSel.value ? Number(catSel.value) : null) : post.categoryId,
            parentSlug: parentIn.value.trim(),
            sortOrder: Number(orderIn.value) || 0,
            originalCreatedAt: dateIn.value,
          };
        },
        inputs: [catSel, parentIn, orderIn, dateIn],
      };
    }
    var meta = metaPanel();

    var bar = el("div", { class: "edit-bar" }, [
      titleInput, statusSel, visSel, note,
      el("span", { class: "edit-spacer" }),
      el("a", { class: "edit-more", href: "/admin/edit/" + encodeURIComponent(slug),
        text: "자세히 ↗" }),
      el("button", { type: "button", class: "edit-btn", text: "취소", onclick: cancel }),
      el("button", { type: "button", class: "edit-btn primary", text: "저장",
        title: "\u2318S / Ctrl+S", onclick: save }),
    ]);

    var preview = el("div", { class: "edit-preview" });

    // 좁은 화면에서는 쓰기와 미리보기를 **탭으로 가른다.** 375px에서 둘을
    // 위아래로 쌓으면 한 화면에 조금씩만 보여 어느 쪽도 못 읽는다.
    //
    // **고르는 일은 라디오와 CSS가 한다**(`:has()`). 여기서 하는 것은 마크업을
    // 놓는 것뿐이라 JS가 상태를 따로 들지 않는다 — 사이드바 아코디언을
    // 서버가 펼쳐 보내는 것과 같은 결이다.
    var tabs = el("div", { class: "edit-tabs" }, [
      el("input", { type: "radio", name: "edit-pane", id: "pane-write", checked: "checked" }),
      el("label", { for: "pane-write", text: "쓰기" }),
      el("input", { type: "radio", name: "edit-pane", id: "pane-preview" }),
      el("label", { for: "pane-preview", text: "미리보기" }),
    ]);

    article.textContent = "";
    article.appendChild(bar);
    article.appendChild(meta.box);
    article.appendChild(el("div", { class: "edit-panes" }, [
      tabs, el("div", { class: "edit-split" }, [area, preview]),
    ]));

    // 노션에서 온 글은 여기서 고쳐도 다음 재이관이 되돌린다. 저장이 되는데
    // 사라지는 것이 가장 나쁜 결과라 미리 말한다.
    if (post.source === "notion") {
      article.insertBefore(el("p", { class: "edit-warn", role: "status",
        text: post.managed
          ? "노션에서 온 글이다. 제목·날짜·순서나 본문을 internal/curation이 관리한다 — 여기서 고친 것은 다음 재이관에 되돌아간다."
          : "노션에서 온 글이다. 본문은 다음 import -db가 변환 결과로 덮는다." }), bar);
    }

    if (window.blogPalette) {
      window.blogPalette.attach(area, null, function () {
        note.textContent = "이미지는 /admin 편집기에서 올린다";
      });
    }

    var timer = null;
    area.addEventListener("input", function () {
      clearTimeout(timer);
      timer = setTimeout(render, 250);
    });
    render();

    // ── 잃지 않게 하는 것들 ───────────────────────────────────────
    //
    // # 나갈 때 경고
    //
    // 고친 것이 있는데 탭을 닫거나 링크를 누르면 **그대로 사라진다.** 저장이
    // 붙은 뒤로는 이게 실제로 일어나는 사고다. 브라우저의 beforeunload는
    // 문구를 우리가 못 정하지만(스팸을 막으려고 그렇게 정해져 있다) 멈춰
    // 세우는 일은 한다.
    //
    // **고친 것이 없으면 안 묻는다.** 늘 물으면 그 물음이 곧 무시된다.
    function dirty() {
      var m = meta.read();
      return area.value !== (post.body || "") ||
        titleInput.value.trim() !== (post.title || "") ||
        statusSel.value !== post.status ||
        visSel.value !== (post.visibility || "public") ||
        m.parentSlug !== (post.parentSlug || "") ||
        m.sortOrder !== (post.sortOrder || 0) ||
        m.originalCreatedAt !== (post.createdAt || "").slice(0, 10) ||
        m.categoryId !== post.categoryId;
    }
    function guard(e) {
      if (!dirty()) return;
      e.preventDefault();
      // 옛 브라우저는 returnValue를 봐야 멈춘다.
      e.returnValue = "";
      return "";
    }
    window.addEventListener("beforeunload", guard);

    // # 단축키
    //
    // Cmd/Ctrl+S. 브라우저의 "페이지 저장"을 가로채는 것이라 preventDefault가
    // 반드시 있어야 한다 — 없으면 HTML 파일을 내려받는 창이 뜬다.
    //
    // **문서 전체에 건다.** 제목 칸이나 메타 칸에 커서가 있어도 저장은 되어야
    // 한다. 편집기를 닫을 때 떼는 것을 잊으면 유령 리스너가 남으므로
    // cancel/save 양쪽에서 unbind를 부른다.
    function keys(e) {
      var mod = e.metaKey || e.ctrlKey;
      if (mod && (e.key === "s" || e.key === "S")) {
        e.preventDefault();
        save();
      }
    }
    document.addEventListener("keydown", keys);

    function unbind() {
      window.removeEventListener("beforeunload", guard);
      document.removeEventListener("keydown", keys);
      document.body.classList.remove("editing");
    }

    function render() {
      api("POST", "/api/admin/preview", { markdown: area.value }).then(function (r) {
        if (!r.ok) {
          preview.innerHTML = "";
          preview.appendChild(el("p", { class: "edit-error", text: r.data.error || "미리보기 실패" }));
          return;
        }
        preview.innerHTML = r.data.html;
        // **공개 화면이 쓰는 것과 같은 함수들이다.** 여기서 다르게 그리면
        // 미리보기가 아니게 된다.
        if (window.blogRenderMath) window.blogRenderMath();
        if (window.blogHighlight) window.blogHighlight();
        if (window.blogCopyButtons) window.blogCopyButtons();
        if (window.blogRenderMermaid) window.blogRenderMermaid();
        if (window.blogMountAnims) window.blogMountAnims();
      });
    }

    function cancel() {
      if (dirty() && !confirm("고친 것을 버릴까?")) return;
      unbind();
      article.replaceWith(saved);
      article = saved;
      saved = null;
      showMenu(true);
    }

    function save() {
      note.className = "edit-note";
      note.textContent = "저장하는 중…";
      var m = meta.read();
      api("PUT", "/api/admin/posts/" + encodeURIComponent(slug), {
        slug: post.slug, title: titleInput.value.trim(), body: area.value,
        status: statusSel.value, visibility: visSel.value, rev: post.rev || "",
        // **PUT은 통째로 바꾸기라 안 보낸 칸은 비워진다.** 그래서 메타도
        // 빠짐없이 싣는다 — 사람이 안 건드렸으면 읽어온 값 그대로다.
        categoryId: m.categoryId, parentSlug: m.parentSlug,
        sortOrder: m.sortOrder, originalCreatedAt: m.originalCreatedAt,
      }).then(function (r) {
        if (!r.ok) {
          note.className = "edit-note edit-error";
          note.textContent = r.data.error || ("저장 실패 (HTTP " + r.status + ")");
          return;
        }
        // **떠나기 전에 뗀다.** 안 떼면 방금 저장했는데도 beforeunload가
        // "정말 나갈까"를 묻는다 — 저장이 됐는지 아닌지 알 수 없게 된다.
        unbind();
        // slug가 바뀌면 이 주소는 더 이상 이 글이 아니다. 그리로 옮긴다.
        if (r.data.slug !== slug) {
          location.href = "/p/" + encodeURIComponent(r.data.slug);
          return;
        }
        // **저장한 것을 그대로 다시 그리지 않고 새로 받아온다.** 실제 글
        // 화면은 렌더링 직전에 죽은 링크를 손보는데(resolveBody) 미리보기는
        // 그걸 안 한다 — 저장 뒤에 그 차이가 남으면 안 된다.
        location.reload();
      });
    }
  }
})();
