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

    var bar = el("div", { class: "edit-bar" }, [
      titleInput, statusSel, visSel, note,
      el("span", { class: "edit-spacer" }),
      el("a", { class: "edit-more", href: "/admin/edit/" + encodeURIComponent(slug),
        text: "자세히 ↗" }),
      el("button", { type: "button", class: "edit-btn", text: "취소", onclick: cancel }),
      el("button", { type: "button", class: "edit-btn primary", text: "저장", onclick: save }),
    ]);

    var preview = el("div", { class: "edit-preview" });

    article.textContent = "";
    article.appendChild(bar);
    article.appendChild(el("div", { class: "edit-split" }, [area, preview]));

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
      if (area.value !== (post.body || "") && !confirm("고친 것을 버릴까?")) return;
      article.replaceWith(saved);
      article = saved;
      saved = null;
      showMenu(true);
    }

    function save() {
      note.className = "edit-note";
      note.textContent = "저장하는 중…";
      api("PUT", "/api/admin/posts/" + encodeURIComponent(slug), {
        slug: post.slug, title: titleInput.value.trim(), body: area.value,
        status: statusSel.value, visibility: visSel.value, rev: post.rev || "",
        // **여기서는 메타를 건드리지 않는다.** PUT은 통째로 바꾸기라 안 보낸
        // 칸은 비워지므로, 지금 값을 그대로 되돌려 보낸다. 분류나 계층을
        // 옮기는 것은 "자세히"로 가서 할 일이다.
        categoryId: post.categoryId, parentSlug: post.parentSlug || "",
        sortOrder: post.sortOrder || 0,
        originalCreatedAt: (post.createdAt || "").slice(0, 10),
      }).then(function (r) {
        if (!r.ok) {
          note.className = "edit-note edit-error";
          note.textContent = r.data.error || ("저장 실패 (HTTP " + r.status + ")");
          return;
        }
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
