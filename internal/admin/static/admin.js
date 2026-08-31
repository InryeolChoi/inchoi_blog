// admin 화면. 목록과 편집 폼을 브라우저에서 그린다(CSR).
//
// **프레임워크도 빌드 스텝도 없다.** 이 저장소의 규칙이고, 이 화면이 하는 일은
// 목록 하나와 폼 하나라 프레임워크가 벌어다 줄 것이 없다.
//
// # 지금 되는 것
//
//   인증   — GitHub 로그인. 허용 목록에 적은 계정만 들어온다.
//   저장   — 실제로 DB에 들어간다. 한 트랜잭션이다.
//   업로드 — 이미지가 BLOB으로 저장되고 본문에 마크다운이 꽂힌다.
//
// **성공한 척하지 않는 것**이 이 화면의 규칙이다. 저장이 실패하면 실패했다고
// 적고, 노션에서 온 글처럼 "저장은 되는데 다음 재이관에 사라지는" 것은
// 미리 경고한다.
(function () {
  "use strict";

  var root = document.getElementById("ad-root");
  var config = JSON.parse(document.getElementById("ad-config").textContent);

  // ---------------------------------------------------------------- 잔손질

  function el(tag, attrs, kids) {
    var node = document.createElement(tag);
    for (var k in attrs || {}) {
      if (k === "class") node.className = attrs[k];
      else if (k === "text") node.textContent = attrs[k];
      else if (k.slice(0, 2) === "on") node.addEventListener(k.slice(2), attrs[k]);
      else if (attrs[k] !== null && attrs[k] !== undefined) node.setAttribute(k, attrs[k]);
    }
    (kids || []).forEach(function (kid) {
      if (kid) node.appendChild(typeof kid === "string" ? document.createTextNode(kid) : kid);
    });
    return node;
  }

  function clear(node) {
    while (node.firstChild) node.removeChild(node.firstChild);
  }

  // 서버가 실패해도 화면이 조용히 멈추면 안 된다. 오류는 늘 글자로 보여준다.
  function api(method, path, body, isForm) {
    var opts = { method: method, headers: {} };
    if (isForm) {
      opts.body = body;
    } else if (body !== undefined) {
      opts.headers["Content-Type"] = "application/json";
      opts.body = JSON.stringify(body);
    }
    return fetch(path, opts).then(function (res) {
      return res.json().catch(function () {
        return { error: "응답을 읽지 못했다 (HTTP " + res.status + ")" };
      }).then(function (data) {
        return { ok: res.ok, status: res.status, data: data };
      });
    });
  }

  function dateText(iso) {
    if (!iso) return "";
    var d = new Date(iso);
    if (isNaN(d)) return "";
    return d.getFullYear() + "-" +
      String(d.getMonth() + 1).padStart(2, "0") + "-" +
      String(d.getDate()).padStart(2, "0");
  }

  // ---------------------------------------------------------------- 라우팅
  //
  // 경로 두 개뿐이다. history API를 쓰므로 새로고침해도 서버가 같은 껍데기를
  // 주고(어느 /admin/* 이든) 여기서 다시 그린다.
  //
  //   /admin                  목록
  //   /admin/edit/{slug}      기존 글 편집
  //   /admin/new              새 글

  function go(path) {
    history.pushState({}, "", path);
    route();
  }

  window.addEventListener("popstate", route);

  function route() {
    var path = location.pathname.replace(/\/+$/, "") || "/admin";
    var m = /^\/admin\/edit\/(.+)$/.exec(path);
    if (m) return showEditor(decodeURIComponent(m[1]));
    if (path === "/admin/new") return showEditor(null);
    return showList();
  }

  // ---------------------------------------------------------------- 목록

  function showList() {
    clear(root);
    root.appendChild(el("p", { class: "ad-empty", text: "불러오는 중…" }));

    api("GET", "/api/admin/posts").then(function (r) {
      clear(root);
      if (!r.ok) {
        root.appendChild(el("p", { class: "ad-error", text: r.data.error || "목록을 못 가져왔다" }));
        return;
      }
      var posts = r.data.posts || [];
      var counts = r.data.counts || {};

      var head = el("div", { class: "ad-listhead" }, [
        el("h1", { text: "글" }),
        el("p", { class: "ad-counts" }, config.statuses.map(function (s) {
          return el("span", { class: "ad-chip st-" + s, text: s + " " + (counts[s] || 0) });
        })),
        el("button", { class: "ad-btn primary", onclick: function () { go("/admin/new"); }, text: "새 글" }),
      ]);
      root.appendChild(head);

      if (posts.length >= r.data.limit) {
        root.appendChild(el("p", {
          class: "ad-note",
          text: "최근 " + r.data.limit + "편만 싣는다. 검색과 페이지 나누기는 다음 단계다.",
        }));
      }

      var table = el("table", { class: "ad-table" }, [
        el("thead", {}, [el("tr", {}, [
          el("th", { text: "제목" }),
          el("th", { text: "분류" }),
          el("th", { text: "상태" }),
          el("th", { class: "num", text: "본문" }),
          el("th", { text: "수정" }),
          el("th", { text: "" }),
        ])]),
      ]);
      var tbody = el("tbody");
      posts.forEach(function (p) {
        tbody.appendChild(el("tr", {}, [
          el("td", {}, [el("a", {
            class: "ad-title", href: "/admin/edit/" + encodeURIComponent(p.slug),
            onclick: function (e) { e.preventDefault(); go("/admin/edit/" + encodeURIComponent(p.slug)); },
            text: p.title || "(제목 없음)",
          })]),
          el("td", { class: "ad-dim", text: p.category || "—" }),
          el("td", {}, [el("span", { class: "ad-chip st-" + p.status, text: p.status })]),
          // 본문이 비어 있는 글은 목록에서 바로 보여야 한다. 공개 화면에서
          // 제목만 뜨는 글이 지금 아홉 편 있다.
          el("td", {
            class: "num " + (p.bodyBytes < 50 ? "ad-warnnum" : "ad-dim"),
            text: p.bodyBytes.toLocaleString(),
          }),
          el("td", { class: "ad-dim", text: dateText(p.updatedAt) }),
          el("td", {}, [el("a", {
            class: "ad-dim", href: "/p/" + encodeURIComponent(p.slug),
            target: "_blank", rel: "noreferrer", text: "보기 ↗",
          })]),
        ]));
      });
      table.appendChild(tbody);
      root.appendChild(table);
    });
  }

  // ---------------------------------------------------------------- 편집

  // cats는 분류 목록이다. 한 번 받아 두고 편집 화면마다 다시 쓴다 —
  // 87개뿐이고 편집 중에 늘어날 것이 아니다.
  var cats = null;

  function loadCategories() {
    if (cats) return Promise.resolve(cats);
    return api("GET", "/api/admin/categories").then(function (r) {
      cats = r.ok ? (r.data.categories || []) : [];
      return cats;
    });
  }

  function showEditor(slug) {
    clear(root);
    root.appendChild(el("p", { class: "ad-empty", text: "불러오는 중…" }));

    if (slug === null) {
      return loadCategories().then(function (cs) {
        renderEditor({ slug: "", title: "", body: "", status: "draft", sortOrder: 0 }, true, cs);
      });
    }
    Promise.all([
      api("GET", "/api/admin/posts/" + encodeURIComponent(slug)),
      loadCategories(),
    ]).then(function (out) {
      var r = out[0];
      if (!r.ok) {
        clear(root);
        root.appendChild(el("p", { class: "ad-error", text: r.data.error || "글을 못 가져왔다" }));
        root.appendChild(el("p", {}, [el("a", {
          href: "/admin", onclick: function (e) { e.preventDefault(); go("/admin"); }, text: "← 목록",
        })]));
        return;
      }
      renderEditor(r.data, false, out[1]);
    });
  }

  // dateValue는 <input type="date">가 받는 꼴로 바꾼다.
  function dateValue(iso) {
    var t = dateText(iso);
    return t || "";
  }

  // note는 화면을 다시 그린 직후에 보여줄 말이다. 새 글을 저장하면 주소가
  // 바뀌면서 폼을 다시 그리는데, 그때 "저장했다"가 같이 지워지면 사람은
  // 저장이 됐는지 알 수 없다. 실패를 숨기지 않는 것과 같은 이유로
  // 성공도 삼키지 않는다.
  function renderEditor(post, isNew, categories, note) {
    clear(root);

    var titleInput = el("input", {
      class: "ad-input", type: "text", id: "ad-title",
      placeholder: "제목", value: post.title || "",
    });
    var slugInput = el("input", {
      class: "ad-input mono", type: "text", id: "ad-slug",
      placeholder: "비워 두면 제목에서 만든다", value: post.slug || "",
    });
    var statusSelect = el("select", { class: "ad-input", id: "ad-status" },
      config.statuses.map(function (s) {
        var o = el("option", { value: s, text: s });
        if (s === post.status) o.selected = true;
        return o;
      }));

    // ── 메타 패널 ────────────────────────────────────────────────
    // 글 하나를 실제로 고치려면 본문만으로 부족하다. 어느 분류에 붙어 있고,
    // 어느 글의 자식이고, 형제 사이 몇 번째인지가 전부 posts의 다른 칸이다.
    var catSelect = el("select", { class: "ad-input", id: "ad-category" },
      [el("option", { value: "", text: "— 분류 없음 —" })].concat(
        (categories || []).map(function (c) {
          var o = el("option", { value: String(c.id), text: c.path });
          if (post.categoryId === c.id) o.selected = true;
          return o;
        })));
    var parentInput = el("input", {
      class: "ad-input mono", type: "text", id: "ad-parent",
      placeholder: "부모 글의 slug (비우면 최상위)", value: post.parentSlug || "",
    });
    var sortInput = el("input", {
      class: "ad-input", type: "number", id: "ad-sort", min: "0",
      value: String(post.sortOrder || 0),
    });
    var dateInput = el("input", {
      class: "ad-input", type: "date", id: "ad-date", value: dateValue(post.createdAt),
    });

    var bodyAreaRef = el("textarea", {
      class: "ad-body mono", id: "ad-body", spellcheck: "false",
      placeholder: "마크다운으로 쓴다. 오른쪽에 그대로 그려진다.",
    });
    bodyAreaRef.value = post.body || "";

    var preview = el("article", { class: "ad-preview-body" });
    var previewNote = el("p", { class: "ad-note" });

    root.appendChild(el("div", { class: "ad-editbar" }, [
      el("a", {
        class: "ad-back", href: "/admin",
        onclick: function (e) { e.preventDefault(); go("/admin"); }, text: "← 목록",
      }),
      el("h1", { text: isNew ? "새 글" : "글 고치기" }),
      el("span", { class: "ad-spacer" }),
      isNew ? null : el("a", {
        class: "ad-dim", href: "/p/" + encodeURIComponent(post.slug),
        target: "_blank", rel: "noreferrer", text: "공개 화면에서 보기 ↗",
      }),
      el("button", { class: "ad-btn primary", id: "ad-save", onclick: save, text: "저장" }),
    ]));

    // **노션에서 온 글에는 경고를 띄운다.** 여기서 고쳐도 다음 `import -db`가
    // 본문을 통째로 덮고, 제목·날짜·순서는 internal/curation의 표가 이긴다.
    // 저장은 되는데 다음 이관에 사라지는 것이 가장 나쁜 결과다.
    if (post.source === "notion") {
      var why = post.managed
        ? "제목·날짜·순서나 본문을 internal/curation이 관리한다. 여기서 고친 것은 다음 재이관에 되돌아간다."
        : "본문은 다음 `go run ./cmd/import -db blog.db`가 변환 결과로 덮는다.";
      root.appendChild(el("p", { class: "ad-warn ad-warn-inline", role: "status" }, [
        el("strong", { text: "노션에서 온 글이다. " }), why,
        " 오래 남길 수정이면 " ,
        el("code", { text: "internal/curation" }), "에 적는 편이 맞다.",
      ]));
    }

    root.appendChild(el("div", { class: "ad-split" }, [
      el("section", { class: "ad-pane" }, [
        el("div", { class: "ad-fields" }, [
          el("label", { class: "ad-field wide" }, [el("span", { text: "제목" }), titleInput]),
          el("label", { class: "ad-field" }, [el("span", { text: "상태" }), statusSelect]),
          el("label", { class: "ad-field wide" }, [el("span", { text: "slug" }), slugInput]),
        ]),
        el("details", { class: "ad-meta" }, [
          el("summary", { text: "분류 · 계층 · 날짜" }),
          el("div", { class: "ad-fields" }, [
            el("label", { class: "ad-field wide" }, [el("span", { text: "분류" }), catSelect]),
            el("label", { class: "ad-field wide" }, [el("span", { text: "부모 글" }), parentInput]),
            el("label", { class: "ad-field" }, [el("span", { text: "형제 순서" }), sortInput]),
            el("label", { class: "ad-field" }, [el("span", { text: "작성일" }), dateInput]),
          ]),
          el("p", { class: "ad-note" }, [
            post.publishedAt
              ? "공개 시각 " + dateText(post.publishedAt) + " (status를 published로 처음 바꿀 때 서버가 찍는다)"
              : "status를 published로 바꾸면 그때 공개 시각이 찍힌다.",
          ]),
        ]),
        imageBox(bodyAreaRef),
        bodyAreaRef,
      ]),
      el("section", { class: "ad-pane" }, [
        el("div", { class: "ad-panehead" }, [
          el("h2", { text: "미리보기" }),
          previewNote,
        ]),
        preview,
      ]),
    ]));

    var status = el("p", {
      class: "ad-status" + (note ? " ok" : ""), id: "ad-savestatus",
      text: note || "",
    });
    root.appendChild(status);

    // 미리보기. 입력이 멈춘 뒤에 한 번만 보낸다 — 키를 칠 때마다 보내면
    // 긴 글에서 요청이 밀린다.
    var timer = null;
    function schedulePreview() {
      clearTimeout(timer);
      timer = setTimeout(renderPreview, 250);
    }
    bodyAreaRef.addEventListener("input", schedulePreview);

    function renderPreview() {
      api("POST", "/api/admin/preview", { markdown: bodyAreaRef.value }).then(function (r) {
        if (!r.ok) {
          previewNote.className = "ad-note ad-error";
          previewNote.textContent = r.data.error || "미리보기 실패";
          return;
        }
        // innerHTML로 넣는 이유: 서버가 goldmark로 그린 HTML이고, 그게 곧
        // 공개 화면에 나갈 것과 같은 문자열이다. 여기서 다르게 다루면
        // 미리보기가 아니게 된다.
        preview.innerHTML = r.data.html;
        // 공개 페이지가 쓰는 것과 **같은 함수**로 수식과 코드를 처리한다.
        if (window.blogRenderMath) window.blogRenderMath();
        if (window.blogHighlight) window.blogHighlight();
        // 복사 버튼도 같은 함수로 단다. innerHTML을 갈아치웠으니 버튼이 통째로
        // 사라졌다 — 다시 부르지 않으면 미리보기에만 버튼이 없다.
        if (window.blogCopyButtons) window.blogCopyButtons();

        var heads = r.data.outline || [];
        previewNote.className = "ad-note";
        previewNote.textContent = bodyAreaRef.value.length.toLocaleString() + "자" +
          (heads.length ? " · 제목 " + heads.length + "개" : "") +
          (heads.length >= 3 ? " (목차가 붙는다)" : "");
      });
    }
    renderPreview();

    function save() {
      var payload = {
        slug: slugInput.value.trim(),
        title: titleInput.value.trim(),
        body: bodyAreaRef.value,
        status: statusSelect.value,
        // **rev를 반드시 같이 보낸다.** 이걸 빼면 서버가 거절한다 — 두 탭에서
        // 연 글이 서로를 조용히 지우는 것을 막는 표다.
        rev: post.rev || "",
        categoryId: catSelect.value ? Number(catSelect.value) : null,
        parentSlug: parentInput.value.trim(),
        sortOrder: Number(sortInput.value) || 0,
        originalCreatedAt: dateInput.value || "",
      };
      status.className = "ad-status";
      status.textContent = "저장하는 중…";
      var isNewPost = isNew || !post.slug;
      var req = isNewPost
        ? api("POST", "/api/admin/posts", payload)
        : api("PUT", "/api/admin/posts/" + encodeURIComponent(post.slug), payload);
      req.then(function (r) {
        if (!r.ok) {
          status.className = "ad-status ad-error";
          status.textContent = r.data.error || ("저장 실패 (HTTP " + r.status + ")");
          return;
        }
        // **새 rev를 받아 둔다.** 안 받으면 이어서 또 저장할 때 서버가
        // "그새 바뀌었다"고 거절한다.
        var saved = r.data;
        var msg = "저장했다 · " + dateText(saved.updatedAt) +
          (saved.status === "draft" ? " (draft라 공개 화면에는 안 보인다)" : "");
        status.className = "ad-status ok";
        status.textContent = msg;
        if (isNewPost || saved.slug !== post.slug) {
          // slug가 정해졌거나 바뀌었으면 주소도 그리로 옮긴다. 새로고침했을 때
          // 없는 글을 열지 않게 하려는 것이다. 다시 그리는 폼에 방금 그 말을
          // 같이 넘긴다.
          post = saved;
          history.replaceState({}, "", "/admin/edit/" + encodeURIComponent(saved.slug));
          renderEditor(saved, false, categories, msg);
          return;
        }
        post = saved;
        slugInput.value = saved.slug;
      });
    }
  }

  // ---------------------------------------------------------------- 이미지
  //
  // 파일이 서버로 가서 sha256으로 저장되고, 응답의 마크다운 한 줄이 본문
  // 커서 자리에 꽂힌다. **마크다운을 만드는 규칙은 서버에 있다** — 화면과
  // 서버 두 곳에 두면 언젠가 갈라진다.

  function imageBox(bodyArea) {
    var input = el("input", { type: "file", accept: "image/*", id: "ad-image", class: "ad-file" });
    var note = el("span", { class: "ad-note" });

    input.addEventListener("change", function () {
      var file = input.files && input.files[0];
      if (!file) return;
      note.className = "ad-note";
      note.textContent = "올리는 중… " + file.name;
      var form = new FormData();
      form.append("image", file);
      api("POST", "/api/admin/images", form, true).then(function (r) {
        if (!r.ok) {
          note.className = "ad-note ad-error";
          note.textContent = r.data.error || "올리지 못했다";
          input.value = "";
          return;
        }
        insertAtCursor(bodyArea, r.data.markdown);
        note.className = "ad-note";
        note.textContent = (r.data.existed ? "이미 있던 그림이다 · " : "올렸다 · ") +
          (r.data.width ? r.data.width + "×" + r.data.height + " · " : "") +
          Math.round(r.data.bytes / 1024) + "KB · 본문에 넣었다";
        input.value = "";
      });
    });

    return el("div", { class: "ad-imagebox" }, [
      el("label", { class: "ad-btn", for: "ad-image", text: "이미지 올리기" }),
      input,
      note,
    ]);
  }

  // insertAtCursor는 커서 자리에 글자를 끼운다. **본문 끝에 붙이지 않는다** —
  // 쓰던 자리에서 그림을 올렸는데 글이 맨 끝에 생기면 다시 옮겨야 한다.
  function insertAtCursor(area, text) {
    var block = "\n\n" + text + "\n\n";
    var at = area.selectionStart;
    if (at === undefined || at === null) {
      area.value += block;
    } else {
      area.value = area.value.slice(0, at) + block + area.value.slice(area.selectionEnd);
      area.selectionStart = area.selectionEnd = at + block.length;
    }
    area.focus();
    // 미리보기를 바로 갱신한다. input 이벤트는 사람이 칠 때만 나므로 직접 쏜다.
    area.dispatchEvent(new Event("input"));
  }

  route();
})();
