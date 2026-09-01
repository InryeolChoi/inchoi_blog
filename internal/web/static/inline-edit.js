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

  var button = el("button", { type: "button", class: "edit-here", onclick: open }, [
    el("span", { text: "고치기" }),
  ]);
  mount.appendChild(button);

  // 원래 화면을 그대로 들고 있다가 취소하면 되돌린다. 다시 그리면
  // 수식·코드 색칠·복사 버튼·애니메이션을 전부 다시 붙여야 하는데,
  // 그 목록은 언젠가 하나 빠진다.
  var saved = null;

  function open() {
    if (saved) return;
    button.disabled = true;
    button.firstChild.textContent = "불러오는 중…";
    api("GET", "/api/admin/posts/" + encodeURIComponent(slug)).then(function (r) {
      button.disabled = false;
      button.firstChild.textContent = "고치기";
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
    button.hidden = true;

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

    var bar = el("div", { class: "edit-bar" }, [
      titleInput, statusSel, note,
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
      button.hidden = false;
    }

    function save() {
      note.className = "edit-note";
      note.textContent = "저장하는 중…";
      api("PUT", "/api/admin/posts/" + encodeURIComponent(slug), {
        slug: post.slug, title: titleInput.value.trim(), body: area.value,
        status: statusSel.value, rev: post.rev || "",
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
