// admin 화면. 목록과 편집 폼을 브라우저에서 그린다(CSR).
//
// **프레임워크도 빌드 스텝도 없다.** 이 저장소의 규칙이고, 이 화면이 하는 일은
// 목록 하나와 폼 하나라 프레임워크가 벌어다 줄 것이 없다.
//
// # 이번 단계에 없는 것
//
//   인증   — 없다. 그래서 서버가 기본적으로 이 화면을 안 띄운다(-admin).
//   저장   — 서버가 501을 준다. 화면은 그 말을 그대로 보여준다.
//   업로드 — 파일이 서버까지 갔다가 버려진다.
//
// 셋 다 "성공한 척"하지 않는 것이 중요하다. 안 들어간 글을 들어갔다고 믿게 하면
// 뼈대가 아니라 함정이다.
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

  function showEditor(slug) {
    clear(root);
    root.appendChild(el("p", { class: "ad-empty", text: "불러오는 중…" }));

    if (slug === null) {
      return renderEditor({ slug: "", title: "", body: "", status: "draft" }, true);
    }
    api("GET", "/api/admin/posts/" + encodeURIComponent(slug)).then(function (r) {
      if (!r.ok) {
        clear(root);
        root.appendChild(el("p", { class: "ad-error", text: r.data.error || "글을 못 가져왔다" }));
        root.appendChild(el("p", {}, [el("a", {
          href: "/admin", onclick: function (e) { e.preventDefault(); go("/admin"); }, text: "← 목록",
        })]));
        return;
      }
      renderEditor(r.data, false);
    });
  }

  function renderEditor(post, isNew) {
    clear(root);

    var titleInput = el("input", {
      class: "ad-input", type: "text", id: "ad-title",
      placeholder: "제목", value: post.title || "",
    });
    var slugInput = el("input", {
      class: "ad-input mono", type: "text", id: "ad-slug",
      placeholder: "slug", value: post.slug || "",
    });
    var statusSelect = el("select", { class: "ad-input", id: "ad-status" },
      config.statuses.map(function (s) {
        var o = el("option", { value: s, text: s });
        if (s === post.status) o.selected = true;
        return o;
      }));
    var bodyArea = el("textarea", {
      class: "ad-body mono", id: "ad-body", spellcheck: "false",
      placeholder: "마크다운으로 쓴다. 오른쪽에 그대로 그려진다.",
    });
    bodyArea.value = post.body || "";

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

    root.appendChild(el("div", { class: "ad-split" }, [
      el("section", { class: "ad-pane" }, [
        el("div", { class: "ad-fields" }, [
          el("label", { class: "ad-field wide" }, [el("span", { text: "제목" }), titleInput]),
          el("label", { class: "ad-field" }, [el("span", { text: "상태" }), statusSelect]),
          el("label", { class: "ad-field wide" }, [el("span", { text: "slug" }), slugInput]),
        ]),
        imageBox(bodyArea),
        bodyArea,
      ]),
      el("section", { class: "ad-pane" }, [
        el("div", { class: "ad-panehead" }, [
          el("h2", { text: "미리보기" }),
          previewNote,
        ]),
        preview,
      ]),
    ]));

    var status = el("p", { class: "ad-status", id: "ad-savestatus" });
    root.appendChild(status);

    // 미리보기. 입력이 멈춘 뒤에 한 번만 보낸다 — 키를 칠 때마다 보내면
    // 긴 글에서 요청이 밀린다.
    var timer = null;
    function schedulePreview() {
      clearTimeout(timer);
      timer = setTimeout(renderPreview, 250);
    }
    bodyArea.addEventListener("input", schedulePreview);

    function renderPreview() {
      api("POST", "/api/admin/preview", { markdown: bodyArea.value }).then(function (r) {
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

        var heads = r.data.outline || [];
        previewNote.className = "ad-note";
        previewNote.textContent = bodyArea.value.length.toLocaleString() + "자" +
          (heads.length ? " · 제목 " + heads.length + "개" : "") +
          (heads.length >= 3 ? " (목차가 붙는다)" : "");
      });
    }
    renderPreview();

    function save() {
      var payload = {
        slug: slugInput.value.trim(),
        title: titleInput.value.trim(),
        body: bodyArea.value,
        status: statusSelect.value,
      };
      // 콘솔에도 남긴다. 서버가 아직 안 받으므로 여기가 유일하게 눈으로
      // 확인할 수 있는 자리다.
      console.log("[admin] 저장 요청", payload);
      status.className = "ad-status";
      status.textContent = "보내는 중…";
      var isNewPost = isNew || !post.slug;
      var req = isNewPost
        ? api("POST", "/api/admin/posts", payload)
        : api("PUT", "/api/admin/posts/" + encodeURIComponent(post.slug), payload);
      req.then(function (r) {
        // 501이 정상이다. 3단계가 오면 여기가 성공으로 바뀐다.
        status.className = r.ok ? "ad-status ok" : "ad-status pending";
        status.textContent = r.data.error || "저장됨";
      });
    }
  }

  // ---------------------------------------------------------------- 이미지
  //
  // 껍데기만이다. 파일이 서버까지 가는 것까지는 확인되고, 서버는 그것을 버린다.
  // 3단계에서 응답에 sha256이 담기면 본문에 `![](/img/{sha})`를 끼우면 된다.

  function imageBox(bodyArea) {
    var input = el("input", { type: "file", accept: "image/*", id: "ad-image", class: "ad-file" });
    var note = el("span", { class: "ad-note" });

    input.addEventListener("change", function () {
      var file = input.files && input.files[0];
      if (!file) return;
      note.className = "ad-note";
      note.textContent = "보내는 중… " + file.name;
      var form = new FormData();
      form.append("image", file);
      api("POST", "/api/admin/images", form, true).then(function (r) {
        note.className = r.ok ? "ad-note" : "ad-note pending";
        note.textContent = r.data.error || "올렸다";
        // 3단계가 오면 여기서 본문에 마크다운을 끼운다. 지금 끼우면 없는
        // 그림을 가리키는 링크가 본문에 남는다.
        input.value = "";
      });
    });

    return el("div", { class: "ad-imagebox" }, [
      el("label", { class: "ad-btn", for: "ad-image", text: "이미지 올리기" }),
      input,
      note,
    ]);
  }

  route();
})();
