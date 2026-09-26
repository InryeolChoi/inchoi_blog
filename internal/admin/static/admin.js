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

  // icon은 아이콘 하나를 만든다. 순서를 옮기는 화살표가 쓴다.
  function icon(d) {
    var n = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    n.setAttribute("viewBox", "0 0 16 16");
    n.setAttribute("aria-hidden", "true");
    var p = document.createElementNS("http://www.w3.org/2000/svg", "path");
    p.setAttribute("d", d);
    n.appendChild(p);
    return n;
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

  // notify는 다 됐다는 것을 화면 가운데서 알리고 확인을 받는다.
  //
  // **왜 줄 끝의 글자가 아니라 창인가.** 저장 버튼 옆에 적으면 긴 폼 아래쪽에
  // 있을 때 눈이 거기 없고, 모바일에서는 키보드에 가려 아예 안 보인다.
  // 한 번 멈춰 세우는 편이 "저장됐나?" 하고 다시 누르는 것보다 낫다.
  //
  // **alert()가 아니라 <dialog>다.** alert은 탭 전체를 멈추고 생김새를 손댈 수
  // 없다. <dialog>.showModal()은 가운데 정렬·Esc로 닫기·바깥 포커스 가두기를
  // 브라우저가 해주고, 닫히면 원래 누르던 버튼으로 포커스가 돌아온다.
  function notify(message, kind, action) {
    // 아주 오래된 브라우저에는 showModal이 없다. 그때는 조용히 지나가지 말고
    // alert이라도 띄운다 — 알림이 통째로 사라지는 쪽이 제일 나쁘다.
    if (!window.HTMLDialogElement || !document.createElement("dialog").showModal) {
      if (action) {
        if (window.confirm(message + "\n\n화면으로 가볼까?")) location.href = action.href;
      } else window.alert(message);
      return;
    }
    var ok = el("button", { type: "button", class: "ad-btn primary", text: "확인" });
    var actionLink = action ? el("a", { class: "ad-btn primary", href: action.href,
      text: action.text }) : null;
    var dialog = el("dialog", { class: "ad-modal" + (kind === "error" ? " danger" : "") }, [
      el("p", { class: "ad-modal-text", text: message }),
      el("div", { class: "ad-modal-acts" }, [ok, actionLink]),
    ]);
    ok.addEventListener("click", function () { dialog.close(); });
    // 닫히면 문서에서 치운다. 남겨두면 저장할 때마다 <dialog>가 쌓인다.
    dialog.addEventListener("close", function () {
      if (dialog.parentNode) dialog.parentNode.removeChild(dialog);
    });
    document.body.appendChild(dialog);
    dialog.showModal();
    ok.focus();
  }

  // ---------------------------------------------------------------- 라우팅
  //
  // 경로 두 개뿐이다. history API를 쓰므로 새로고침해도 서버가 같은 껍데기를
  // 주고(어느 /admin/* 이든) 여기서 다시 그린다.
  //
  //   /admin                  목록
  //   /admin/edit/{slug}      기존 글 편집
  //   /admin/new              새 글
  //   /admin/notes            글감함
  //   /admin/home             홈 화면 편집
  //   /admin/data             데이터 보기
  //   /admin/graph            글 지도

  function go(path) {
    history.pushState({}, "", path);
    route();
  }

  window.addEventListener("popstate", route);

  // drawTicket은 지금 그리고 있는 화면의 표다.
  //
  // **목록과 데이터 보기는 서버에 물어본 뒤에 그린다.** 그 사이에 사람이
  // 메뉴를 한 번 더 누르면 늦게 온 응답이 **새 화면을 덮어쓴다** — 실제로
  // 데이터를 누르고 곧장 환경설정을 눌렀더니 주소는 설정인데 화면은 데이터가
  // 나왔다. 그릴 때 표를 확인해서, 그 사이에 다른 화면으로 갔으면 그린다.
  var drawTicket = 0;

  function ticket() {
    return ++drawTicket;
  }

  function stale(mine) {
    return mine !== drawTicket;
  }

  function route() {
    var path = location.pathname.replace(/\/+$/, "") || "/admin";
    markMenu(path);
    ticket();
    var m = /^\/admin\/edit\/(.+)$/.exec(path);
    if (m) return showEditor(decodeURIComponent(m[1]));
    if (path === "/admin/new") return showEditor(null, prefillCategory());
    if (path === "/admin/notes") return showNotes();
    if (path === "/admin/home") return showHome();
    if (path === "/admin/data") return showStats();
    if (path === "/admin/graph") return showGraph();
    if (path === "/admin/settings") return showSettings();
    return showList();
  }

  // markMenu는 상단 메뉴에서 지금 있는 곳을 표시한다.
  //
  // **메뉴 자체는 서버가 그렸다.** 여기서 하는 일은 표시뿐이라, 스크립트가
  // 못 떠도 세 링크는 그대로 눌린다. 새 글과 편집 화면은 `전체 글`에 딸린
  // 자리라 그쪽을 켠다 — 어디에도 안 걸린 화면을 만들지 않는다.
  function markMenu(path) {
    var here = path === "/admin/notes" || path === "/admin/home" || path === "/admin/data" || path === "/admin/graph" || path === "/admin/settings"
      ? path : "/admin";
    Array.prototype.forEach.call(document.querySelectorAll(".ad-menu a"), function (a) {
      var on = a.dataset.menu === here;
      a.classList.toggle("ad-on", on);
      if (on) a.setAttribute("aria-current", "page");
      else a.removeAttribute("aria-current");
    });
  }

  // 메뉴 링크는 진짜 링크지만, 눌렀을 때 페이지를 통째로 다시 받을 이유는 없다.
  document.addEventListener("click", function (e) {
    var a = e.target.closest ? e.target.closest(".ad-menu a") : null;
    if (!a || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
    e.preventDefault();
    go(a.getAttribute("href"));
  });

  // ---------------------------------------------------------------- 목록

  function showList() {
    var mine = drawTicket;
    clear(root);
    root.appendChild(el("p", { class: "ad-empty", text: "불러오는 중…" }));

    api("GET", "/api/admin/posts").then(function (r) {
      if (stale(mine)) return;
      clear(root);
      if (!r.ok) {
        root.appendChild(el("p", { class: "ad-error", text: r.data.error || "목록을 못 가져왔다" }));
        return;
      }
      var posts = r.data.posts || [];
      var counts = r.data.counts || {};

      var head = el("div", { class: "ad-listhead" }, [
        el("h1", { text: "전체 글" }),
        el("p", { class: "ad-counts" }, config.statuses.map(function (s) {
          return el("span", { class: "ad-chip st-" + s, text: s + " " + (counts[s] || 0) });
        })),
        el("button", { class: "ad-btn primary", onclick: function () { go("/admin/new"); }, text: "새 글" }),
      ]);
      root.appendChild(head);

      // 필터는 메뉴 바로 아래에 깐다. **서버에 다시 묻지 않는다** — 목록이
      // 이미 통째로 와 있어서 거르는 일은 이 자리에서 끝난다. `/` 팔레트가
      // 서버에 안 묻는 것과 같은 판단이다: 친 순간과 걸러진 순간 사이에
      // 네트워크가 끼면 그 지연은 감출 수 없다.
      var filter = { q: "", status: "", category: "", empty: false };
      var tbody = el("tbody");
      root.appendChild(filterBar(posts, filter, function () { fill(tbody, posts, filter); }));

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
          el("th", { class: "ad-acts-head", text: "" }),
        ])]),
      ]);
      fill(tbody, posts, filter);
      table.appendChild(tbody);
      // 표는 제 안에서만 가로로 스크롤한다. 여섯 칸이라 375px에서 35px쯤
      // 넘치는데, 그대로 두면 페이지 전체가 밀린다 — 공개 화면에서 코드와
      // 표를 가두는 것과 같은 규칙이다.
      root.appendChild(el("div", { class: "ad-tablewrap" }, [table]));
    });
  }

  // filterBar는 목록 위의 거르개다. 무엇 하나라도 바꾸면 표를 다시 채운다.
  function filterBar(posts, filter, redraw) {
    var cats = [];
    var seen = {};
    posts.forEach(function (p) {
      if (p.category && !seen[p.category]) { seen[p.category] = true; cats.push(p.category); }
    });
    cats.sort();

    function onChange(key) {
      return function (e) {
        filter[key] = e.target.type === "checkbox" ? e.target.checked : e.target.value;
        redraw();
      };
    }

    return el("div", { class: "ad-filters" }, [
      el("input", {
        class: "ad-search", type: "search", placeholder: "제목이나 slug로 찾기",
        "aria-label": "제목이나 slug로 찾기", oninput: onChange("q"),
      }),
      el("select", { "aria-label": "상태", onchange: onChange("status") },
        [el("option", { value: "", text: "상태 전체" })].concat(config.statuses.map(function (s) {
          return el("option", { value: s, text: s });
        }))),
      el("select", { "aria-label": "분류", onchange: onChange("category") },
        [el("option", { value: "", text: "분류 전체" })].concat(cats.map(function (c) {
          return el("option", { value: c, text: c });
        }))),
      // 본문이 없는 글을 골라내는 것이 이 화면에서 가장 자주 하는 일이다.
      // 공개 화면에서 제목만 뜨는 글이 그것들이다.
      el("label", { class: "ad-check" }, [
        el("input", { type: "checkbox", onchange: onChange("empty") }),
        document.createTextNode(" 본문 없는 것만"),
      ]),
    ]);
  }

  function keep(p, f) {
    if (f.status && p.status !== f.status) return false;
    if (f.category && p.category !== f.category) return false;
    if (f.empty && p.bodyBytes >= 50) return false;
    if (f.q) {
      var q = f.q.toLowerCase();
      if ((p.title || "").toLowerCase().indexOf(q) < 0 &&
          (p.slug || "").toLowerCase().indexOf(q) < 0) return false;
    }
    return true;
  }

  // fill은 거른 결과로 표 몸통을 다시 채운다.
  //
  // **한 건도 안 남으면 그렇다고 적는다.** 빈 표만 남으면 거르개가 걸린 것인지
  // 목록을 못 가져온 것인지 화면만 보고는 알 수 없다.
  function fill(tbody, posts, filter) {
    clear(tbody);
    var shown = 0;
    posts.forEach(function (p) {
      if (!keep(p, filter)) return;
      shown++;
      tbody.appendChild(el("tr", {}, [
          el("td", {}, [el("a", {
            class: "ad-title", href: "/admin/edit/" + encodeURIComponent(p.slug),
            onclick: function (e) { e.preventDefault(); go("/admin/edit/" + encodeURIComponent(p.slug)); },
            text: p.title || "(제목 없음)",
          })]),
          el("td", { class: "ad-dim", text: p.category || "—" }),
          el("td", {}, [
            el("span", { class: "ad-chip st-" + p.status, text: p.status }),
          ].concat(p.visibility === "private"
            ? [el("span", { class: "ad-chip ad-chip-private", text: "private" })]
            : [])),
          // 본문이 비어 있는 글은 목록에서 바로 보여야 한다. 공개 화면에서
          // 제목만 뜨는 글이 지금 아홉 편 있다.
          el("td", {
            class: "num " + (p.bodyBytes < 50 ? "ad-warnnum" : "ad-dim"),
            text: p.bodyBytes.toLocaleString(),
          }),
          el("td", { class: "ad-dim", text: dateText(p.updatedAt) }),
          // **손댈 것을 한 줄에서 끝낸다.** 예전에는 "보기"만 있어서 고치거나
          // 치우려면 제목을 눌러 편집 화면까지 들어가야 했다.
          el("td", { class: "ad-acts" }, [
            el("a", {
              class: "ad-act", href: "/admin/edit/" + encodeURIComponent(p.slug),
              onclick: function (e) { e.preventDefault(); go("/admin/edit/" + encodeURIComponent(p.slug)); },
              text: "고치기",
            }),
            el("a", {
              class: "ad-act", href: "/p/" + encodeURIComponent(p.slug),
              target: "_blank", rel: "noreferrer", text: "보기 ↗",
            }),
            el("button", {
              type: "button", class: "ad-act danger", text: "지우기",
              onclick: function () { removeFromList(p, showList); },
            }),
          ]),
        ]));
    });
    if (shown === 0) {
      tbody.appendChild(el("tr", {}, [
        el("td", { class: "ad-empty", colspan: "6", text: "거르고 나니 남는 글이 없다." }),
      ]));
    }
  }

  // removeFromList는 목록에서 글 하나를 지운다.
  //
  // **편집 화면의 지우기와 같은 흐름이다** — refs로 무엇을 잃는지 먼저 묻고,
  // 자식이 있으면 아예 막고, 잃을 것이 있으면 확인을 받는다. 규칙을 두 곳에
  // 따로 적으면 한쪽이 느슨해진다. 다른 점은 끝난 뒤에 목록을 다시 그리는
  // 것뿐이다.
  function removeFromList(post, done) {
    api("GET", "/api/admin/posts/" + encodeURIComponent(post.slug) + "/refs")
      .then(function (r) {
        if (!r.ok) return alert(r.data.error || "무엇이 걸리는지 알아내지 못했다");
        var refs = r.data;
        if (refs.children && refs.children.length) {
          return alert("하위 글 " + refs.children.length + "편이 매달려 있다: " +
            refs.children.slice(0, 3).join(", ") +
            (refs.children.length > 3 ? " 외" : "") +
            "\n\n그것들을 먼저 옮기거나 지워라.");
        }
        var lose = [];
        if (refs.notion) lose.push("노션에서 온 글이라 다음 재이관이 되살린다 (진짜로 빼려면 internal/curation의 DropPosts에 적어야 한다)");
        if (refs.coverOf && refs.coverOf.length) lose.push("분류 " + refs.coverOf.join(", ") + "의 표지가 사라진다");
        if (refs.linkedFrom && refs.linkedFrom.length) lose.push("이 글을 가리키던 " + refs.linkedFrom.length + "편의 링크를 글자로 푼다");

        var msg = "\"" + (post.title || post.slug) + "\"을(를) 지운다.";
        if (lose.length) msg += "\n\n" + lose.map(function (l, i) { return (i + 1) + ". " + l; }).join("\n");
        msg += "\n\n되돌릴 수 없다. 지울까?";
        if (!window.confirm(msg)) return;

        // **목록에는 rev가 없다.** 지우기는 저장과 같은 rev 표를 요구하므로
        // 지금 값을 한 번 더 받아온다 — 그 사이에 다른 탭이 고쳤으면 거절된다.
        api("GET", "/api/admin/posts/" + encodeURIComponent(post.slug)).then(function (g) {
          if (!g.ok) return alert(g.data.error || "글을 못 가져왔다");
          api("DELETE", "/api/admin/posts/" + encodeURIComponent(post.slug),
            { rev: g.data.rev || "", force: lose.length > 0 }).then(function (d) {
            if (!d.ok) return alert(d.data.error || ("지우지 못했다 (HTTP " + d.status + ")"));
            if (done) done();
          });
        });
      });
  }

  // ---------------------------------------------------------------- 글감함
  //
  // 프로젝트를 하는 동안 남긴 메모를 여기 쌓아둔다. "초안 생성"을 누르면
  // OpenRouter가 그 메모를 정리해 draft 글 하나를 만든다 — 최종 편집은
  // 항상 사람이 admin 편집기에서 한다.

  function noteCard(note, onChange) {
    var busy = false;

    var genBtn = el("button", {
      type: "button", class: "ad-act",
      text: note.status === "generated" ? "다시 생성" : "초안 생성",
    });
    var editSlug = note.generatedSlug
      ? el("a", { class: "ad-act", href: "/admin/edit/" + encodeURIComponent(note.generatedSlug), text: "만든 초안 열기 ↗" })
      : null;
    var status = el("span", {
      class: "ad-dim",
      text: note.status === "generated" ? "초안 생성됨" : "아직 안 만듦",
    });

    genBtn.addEventListener("click", function () {
      if (busy) return;
      busy = true;
      genBtn.textContent = "만드는 중… (최대 5분 걸릴 수 있다)";
      genBtn.disabled = true;
      api("POST", "/api/admin/notes/" + note.id + "/generate").then(function (r) {
        busy = false;
        genBtn.disabled = false;
        if (!r.ok) {
          genBtn.textContent = note.status === "generated" ? "다시 생성" : "초안 생성";
          return alert(r.data.error || ("초안을 만들지 못했다 (HTTP " + r.status + ")"));
        }
        if (onChange) onChange();
        notify("AI 초안 생성에 성공했다. 편집 화면에서 확인하고 저장 상태를 정할 수 있다.",
          "ok", { href: "/admin/edit/" + encodeURIComponent(r.data.slug), text: "글 화면으로 가보기" });
      }).catch(function () {
        busy = false;
        genBtn.disabled = false;
        genBtn.textContent = note.status === "generated" ? "다시 생성" : "초안 생성";
        notify("연결이 끊겨 초안 생성 결과를 받지 못했다. 목록을 새로 불러와 확인해라", "error");
      });
    });

    var delBtn = el("button", {
      type: "button", class: "ad-act danger", text: "글감 지우기",
      onclick: function () {
        if (!window.confirm("\"" + note.title + "\" 글감을 지운다. 이미 만든 초안 글은 남는다. 지울까?")) return;
        api("DELETE", "/api/admin/notes/" + note.id).then(function (r) {
          if (!r.ok) return alert(r.data.error || "지우지 못했다");
          if (onChange) onChange();
        });
      },
    });

    return el("li", { class: "ad-note" }, [
      el("div", { class: "ad-note-head" }, [
        el("strong", { text: note.title }),
        status,
      ]),
      el("p", { class: "ad-dim", text: dateText(note.createdAt) }),
      el("div", { class: "ad-acts ad-note-actions" }, [genBtn, editSlug, delBtn]),
    ]);
  }

  function showNotes() {
    var mine = drawTicket;
    clear(root);
    root.appendChild(el("div", { class: "ad-listhead" }, [
      el("h1", { text: "글감함" }),
    ]));
    root.appendChild(el("p", { class: "ad-dim", text:
      "프로젝트를 하면서 남긴 메모를 여기 적어둔다. \"초안 생성\"을 누르면 AI가 정리해서 " +
      "draft 글을 하나 만든다 — 공개되지 않으며, 최종 편집은 직접 한다." }));

    var titleInput = el("input", { type: "text", placeholder: "제목", maxlength: "300", class: "ad-note-title" });
    var bodyInput = el("textarea", { rows: "6", placeholder: "글감 본문. 생각나는 대로 적어둔다.", class: "ad-note-body" });
    var addErr = el("p", { class: "ad-error" });
    var addBtn = el("button", { type: "button", class: "ad-btn primary", text: "글감 남기기" });

    addBtn.addEventListener("click", function () {
      clear(addErr);
      var title = titleInput.value.trim();
      var body = bodyInput.value.trim();
      if (!title || !body) {
        addErr.textContent = "제목과 본문을 둘 다 적어야 한다";
        return;
      }
      addBtn.disabled = true;
      api("POST", "/api/admin/notes", { title: title, body: body }).then(function (r) {
        addBtn.disabled = false;
        if (!r.ok) {
          addErr.textContent = r.data.error || "글감을 남기지 못했다";
          return;
        }
        titleInput.value = "";
        bodyInput.value = "";
        loadList();
        notify("글감을 남겼다. 목록에서 초안 생성을 시작할 수 있다.");
      });
    });

    root.appendChild(el("section", { class: "ad-card" }, [
      el("h2", { text: "새 글감" }),
      titleInput, bodyInput, addErr, addBtn,
    ]));

    var listBox = el("ul", { class: "ad-note-list" });
    root.appendChild(listBox);

    function loadList() {
      var ticketNow = mine;
      clear(listBox);
      listBox.appendChild(el("li", { class: "ad-empty", text: "불러오는 중…" }));
      api("GET", "/api/admin/notes").then(function (r) {
        if (stale(ticketNow)) return;
        clear(listBox);
        if (!r.ok) {
          listBox.appendChild(el("li", { class: "ad-empty", text: r.data.error || "글감을 못 가져왔다" }));
          return;
        }
        var notes = r.data.notes || [];
        if (!notes.length) {
          listBox.appendChild(el("li", { class: "ad-empty", text: "아직 남긴 글감이 없다." }));
          return;
        }
        notes.forEach(function (note) {
          listBox.appendChild(noteCard(note, loadList));
        });
      });
    }
    loadList();
  }

  // ---------------------------------------------------------------- 홈 화면
  //
  // **홈도 글처럼 쓴다.** 예전에는 문구 네 줄만 고칠 수 있었고 그것도
  // 환경설정 구석에 있었다 — 홈에 무엇을 둘지는 사람이 정할 일인데, 그
  // 밖의 것을 넣으려면 템플릿을 고쳐 배포해야 했다.
  //
  // 이제 표제지 아래에 **마크다운 본문**을 자유롭게 쓴다. 글과 같은 렌더러로
  // 그리므로 수식도 코드도 목록도 글에서 되는 것은 여기서도 된다.

  function showHome() {
    var mine = drawTicket;
    clear(root);
    root.appendChild(el("div", { class: "ad-listhead" }, [el("h1", { text: "홈 화면" })]));
    var wait = el("p", { class: "ad-dim", text: "불러오는 중…" });
    root.appendChild(wait);

    api("GET", "/api/admin/settings").then(function (r) {
      if (stale(mine)) return;
      wait.remove();
      if (!r.ok) {
        root.appendChild(el("p", { class: "ad-error",
          text: (r.data && r.data.error) || "설정을 읽지 못했다" }));
        return;
      }
      var vals = r.data.values || {}, defs = r.data.defaults || {};
      var inputs = {};

      // **비우면 기본 문구로 돌아간다.** 그래서 자리표시자에 그 기본값을
      // 회색으로 깔아 둔다 — 비웠을 때 무엇이 나오는지 보이지 않으면
      // 지우기가 무섭다.
      function box(key, tag, cls, rows) {
        var node = el(tag, { class: cls, placeholder: defs[key] || "" });
        node.value = vals[key] || "";
        if (rows) node.rows = rows;
        inputs[key] = node;
        return node;
      }
      // **`.ad-body`는 글 편집기의 본문 칸이라 26rem으로 서 있다.** 리드
      // 한두 줄에 그 높이를 주면 폼이 통째로 그 칸이 된다 — 짧은 칸은
      // 따로 둔다(`.ad-line`).
      function field(key, label, tag, rows) {
        var cls = tag === "textarea" ? "ad-line" : "ad-input";
        return el("label", { class: "ad-field wide" },
          [el("span", { text: label }), box(key, tag, cls, rows)]);
      }

      var hero = el("section", { class: "ad-card" }, [
        el("h2", { text: "표제지" }),
        el("p", { class: "ad-dim", text: "첫 화면 맨 위의 네 줄이다. 비우면 기본 문구로 돌아간다." }),
        el("div", { class: "ad-fields" }, [
          field("home.kicker", "눈썹줄", "input"),
          field("home.title_top", "제목 첫 줄", "input"),
          field("home.title_mark", "제목 둘째 줄 (형광 블록)", "input"),
          field("home.lead", "리드 문장", "textarea", 2),
        ]),
      ]);

      // 자유 본문. **글과 같은 미리보기다**(POST /api/admin/preview) —
      // 여기서 다르게 그리면 홈에 쓴 것과 글에 쓴 것이 다르게 보인다.
      //
      // **`.ad-field.wide`로 감싸지 않는다.** 그건 `grid-column: 1 / -1`이라
      // 두 칸짜리 격자 안에 두면 원문이 두 칸을 다 먹고 미리보기가 아래로
      // 밀린다 — 실제로 그렇게 나왔다.
      var body = box("home.body", "textarea", "ad-body", 12);
      var preview = el("article", { class: "ad-preview-body" });
      var bodyCard = el("section", { class: "ad-card" }, [
        el("h2", { text: "본문" }),
        el("p", { class: "ad-dim",
          text: "표제지 아래에 자유롭게 쓴다. 글과 같은 마크다운이라 수식·코드·목록이 그대로 된다. 비우면 아무것도 안 나온다." }),
        el("div", { class: "ad-split" }, [body, preview]),
      ]);

      var timer = null;
      function draw() {
        api("POST", "/api/admin/preview", { markdown: inputs["home.body"].value }).then(function (pr) {
          if (!pr.ok) {
            preview.textContent = (pr.data && pr.data.error) || "미리보기 실패";
            return;
          }
          preview.innerHTML = pr.data.html;
          // **공개 화면이 쓰는 것과 같은 함수들이다.** 여기서 다르게 그리면
          // 미리보기가 아니게 된다.
          if (window.blogRenderMath) window.blogRenderMath();
          if (window.blogHighlight) window.blogHighlight();
          if (window.blogCopyButtons) window.blogCopyButtons();
          if (window.blogRenderMermaid) window.blogRenderMermaid();
          if (window.blogMountAnims) window.blogMountAnims();
        });
      }
      inputs["home.body"].addEventListener("input", function () {
        clearTimeout(timer);
        timer = setTimeout(draw, 120);
      });
      if (window.blogPalette) window.blogPalette.attach(inputs["home.body"]);
      if (window.blogMathLive) window.blogMathLive.attach(inputs["home.body"]);
      draw();

      // 최근 글. **0은 "안 보인다"라는 뜻이 있는 값**이라 빈 값(안 정함)과
      // 구별해야 한다 — 위의 문구들과 반대다.
      var recent = el("input", { class: "ad-input", type: "number", min: "0", max: "20" });
      recent.value = vals["home.recent"] !== undefined ? vals["home.recent"] : (defs["home.recent"] || "6");
      inputs["home.recent"] = recent;
      var recentCard = el("section", { class: "ad-card" }, [
        el("h2", { text: "최근에 쓴 글" }),
        el("p", { class: "ad-dim", text: "본문 아래에 최근 글을 몇 편 세울지. 0이면 그 절이 아예 안 나온다." }),
        el("div", { class: "ad-fields" }, [
          el("label", { class: "ad-field" }, [el("span", { text: "개수" }), recent]),
        ]),
      ]);

      var note = el("span", { class: "ad-status" });
      var save = el("button", { type: "button", class: "ad-btn primary", text: "홈 저장",
        onclick: function () {
          var values = {};
          Object.keys(inputs).forEach(function (k) { values[k] = inputs[k].value; });
          note.className = "ad-status";
          note.textContent = "저장하는 중…";
          api("PUT", "/api/admin/settings", { values: values }).then(function (r2) {
            if (!r2.ok) {
              note.className = "ad-status ad-error";
              note.textContent = (r2.data && r2.data.error) || "저장하지 못했다";
              return;
            }
            // **성공도 삼키지 않는다.** 눌렀는데 아무 표시가 없으면 저장이
            // 됐는지 알 수 없다.
            note.textContent = "저장했다.";
          });
        } });

      root.appendChild(hero);
      root.appendChild(bodyCard);
      root.appendChild(recentCard);
      root.appendChild(el("div", { class: "ad-homesave" }, [
        save, note,
        el("a", { class: "ad-act", href: "/", target: "_blank", rel: "noreferrer", text: "홈 보기 ↗" }),
      ]));
    });
  }

  // ---------------------------------------------------------------- 환경설정
  //
  // 두 종류가 섞여 있어서 **절마다 어디에 저장되는지 적는다.** 테마는 이
  // 브라우저에만(localStorage) 남고, AI 설정은 서버의 settings 표에 남는다 —
  // 한 화면에 있다고 같은 자리에 저장되는 줄 알면, 다른 기기에서 열었을 때
  // 왜 하나는 따라오고 하나는 안 따라오는지 알 수 없다.
  //
  // 테마는 공개 화면의 화면 설정과 같은 키라, admin에서 다크로 바꾸면 공개
  // 화면도 다크다.
  //
  // **없는 것을 있는 척하지 않는다.** 글쓰기 기본값 같은 것은 아직 저장할
  // 자리가 없어서 여기 두지 않는다.
  //
  // 홈 문구는 2026-09-09에 제 화면으로 나갔다(`/admin/home`). 서버에 저장하는
  // 것이 이 화면의 성격과 달랐고, 무엇보다 **구석에 있어서 있는 줄을 몰랐다.**

  // ------------------------------------------------------- AI 초안 생성 설정
  //
  // 글감함의 "초안 생성"이 무엇으로 돌아가는지를 한 자리에 모은다 — 어떤
  // 모델을 쓰고, 어떤 지시문을 주고, 돈이 얼마나 남았나. **누르기 전에 보이는
  // 것이 요점이다.** 예전에는 버튼을 눌러 실패해야 키가 없다는 것을 알았다.
  //
  // **키는 여기서 못 고친다.** 서버의 환경변수이고, 배포가 넣는다. 화면은
  // 있는지 없는지만 안다 — DB로 내려오면 백업과 이관 작업본이 전부 secret을
  // 들고 다니게 된다(internal/admin/ai.go).
  //
  // 저장은 **모델과 프롬프트 둘을 한 번에** 보낸다. 서로 맞물리는 값이라
  // (프롬프트를 모델에 맞춰 쓴다) 따로 저장하면 반만 바뀐 상태가 생긴다.

  // money는 OpenRouter가 주는 달러 값을 읽을 수 있게 적는다. 소수점 둘은
  // 잔액으로는 너무 거칠다 — 한 번 부르는 값이 센트 아래라서 0.00으로만 보인다.
  function money(v) {
    if (typeof v !== "number" || isNaN(v)) return "—";
    return "$" + v.toFixed(v !== 0 && Math.abs(v) < 0.01 ? 4 : 2);
  }

  function aiCard() {
    var card = el("section", { class: "ad-card" }, [
      el("h2", { text: "AI 초안 생성" }),
      el("p", { class: "ad-dim", text:
        "글감함에서 \"초안 생성\"을 누르면 OpenRouter로 이 설정이 쓰인다. " +
        "모델과 프롬프트는 서버에 저장되므로 다른 기기에서도 같다." }),
    ]);
    var wait = el("p", { class: "ad-dim", text: "불러오는 중…" });
    card.appendChild(wait);

    api("GET", "/api/admin/ai").then(function (r) {
      card.removeChild(wait);
      if (!r.ok) {
        card.appendChild(el("p", { class: "ad-error", text: r.data.error || "AI 설정을 못 가져왔다" }));
        return;
      }
      var d = r.data;
      var defaults = d.defaults || {};
      var effective = d.effective || {};

      // ── 키. **없으면 제일 먼저, 눈에 띄게 적는다** — 이 값이 없으면 아래
      //    설정을 아무리 손봐도 버튼은 실패한다.
      card.appendChild(d.configured
        ? el("p", { class: "ad-dim", text: "서버에 API 키가 있다. 키 자체는 화면에 나오지 않는다." })
        : el("p", { class: "ad-warn ad-warn-inline", text:
            "서버에 API 키가 없다. 초안 생성은 실패한다 — GitHub Actions의 " +
            "OPENROUTER_API_KEY secret을 넣고 다시 배포하면 들어간다." }));

      // ── 잔액. 키가 없으면 물어볼 것도 없다.
      if (d.configured) card.appendChild(creditsBox());

      // ── 모델
      var list = el("datalist", { id: "ad-ai-models" });
      var modelInput = el("input", {
        type: "text", class: "ad-input ad-ai-model", list: "ad-ai-models",
        maxlength: "200", placeholder: defaults.model || "", value: d.model || "",
        autocapitalize: "off", autocorrect: "off", spellcheck: "false",
      });
      var loadBtn = el("button", { type: "button", class: "ad-act", text: "쓸 수 있는 모델 불러오기" });
      var modelNote = el("p", { class: "ad-dim ad-ai-note", text: modelNoteText(d, effective) });

      // 모델 목록은 **누를 때만 받는다.** 수백 개라 설정 화면을 열 때마다
      // 받아오면 그 값을 쓰지도 않는 사람의 화면이 매번 느려진다.
      loadBtn.addEventListener("click", function () {
        loadBtn.disabled = true;
        loadBtn.textContent = "불러오는 중…";
        api("GET", "/api/admin/ai/models").then(function (m) {
          loadBtn.disabled = false;
          if (!m.ok) {
            loadBtn.textContent = "쓸 수 있는 모델 불러오기";
            modelNote.textContent = m.data.error || "모델 목록을 못 가져왔다";
            modelNote.className = "ad-error ad-ai-note";
            return;
          }
          var models = m.data.models || [];
          clear(list);
          models.forEach(function (mm) {
            // 자동완성 목록은 브라우저가 걸러 준다. 값은 id고, 사람이 읽을
            // 이름과 가격은 곁들이는 글자로 붙인다.
            list.appendChild(el("option", { value: mm.id, label: modelLabel(mm) }));
          });
          loadBtn.textContent = "모델 " + models.length + "개 불러옴";
          modelNote.className = "ad-dim ad-ai-note";
          modelNote.textContent = "칸을 누르고 이름을 치면 자동완성으로 고를 수 있다.";
        });
      });

      card.appendChild(el("div", { class: "ad-field ad-ai-field" }, [
        el("label", { for: "ad-ai-model-input", text: "모델" }),
        modelInput, list,
        el("div", { class: "ad-ai-row" }, [loadBtn]),
        modelNote,
      ]));
      modelInput.id = "ad-ai-model-input";

      // ── 프롬프트
      var promptInput = el("textarea", {
        id: "ad-ai-prompt-input", class: "ad-body ad-ai-prompt", rows: "12",
        placeholder: defaults.prompt || "",
      });
      promptInput.value = d.prompt || "";
      card.appendChild(el("div", { class: "ad-field ad-ai-field" }, [
        el("label", { for: "ad-ai-prompt-input", text: "지시문(시스템 프롬프트)" }),
        promptInput,
        el("p", { class: "ad-dim ad-ai-note", text:
          "비우면 기본 지시문이 쓰인다(회색으로 보이는 문장). 첫 줄을 \"# 제목\"으로 " +
          "내놓으라는 규칙은 지우지 않는 편이 낫다 — 그 줄을 글 제목으로 떼어 쓴다." }),
      ]));

      // ── 저장
      var status = el("p", { class: "ad-status" });
      var save = el("button", { type: "button", class: "ad-btn primary", text: "AI 설정 저장" });
      save.addEventListener("click", function () {
        save.disabled = true;
        status.className = "ad-status pending";
        status.textContent = "저장 중…";
        api("PUT", "/api/admin/ai", { model: modelInput.value, prompt: promptInput.value })
          .then(function (r2) {
            save.disabled = false;
            if (!r2.ok) {
              status.className = "ad-error";
              status.textContent = (r2.data && r2.data.error) || "저장하지 못했다";
              return;
            }
            // **성공도 삼키지 않는다.** 무엇이 실제로 쓰이게 됐는지까지 적는다.
            // 창으로 한 번 멈춰 세우고, 줄 끝의 글자는 지운다 — 같은 말을 두
            // 군데에 두면 창을 닫은 뒤에도 남아서 언제 저장한 것인지 흐려진다.
            status.className = "ad-status";
            status.textContent = "";
            modelNote.className = "ad-dim ad-ai-note";
            modelNote.textContent = modelNoteText(r2.data, r2.data.effective || {});
            notify("AI 설정을 저장했다.\n이제 " + ((r2.data.effective || {}).model || "") + "로 초안을 만든다.");
          });
      });
      card.appendChild(el("div", { class: "ad-homesave" }, [save, status]));
    });

    return card;
  }

  // modelNoteText는 지금 실제로 무엇이 쓰이는지 한 줄로 적는다.
  //
  // **저장된 값이 비어 있을 때가 핵심이다.** 칸이 비어 있으면 "아무것도 안
  // 쓴다"로 읽히지만 실제로는 기본값이나 서버 환경변수가 쓰인다.
  function modelNoteText(d, effective) {
    var used = effective.model || "";
    if (d.model) return "지금 " + used + "로 만든다.";
    if (d.envModel) return "비어 있어서 서버가 정한 " + used + "로 만든다.";
    return "비어 있어서 기본값 " + used + "로 만든다.";
  }

  function modelLabel(m) {
    var label = m.name || m.id;
    // 가격은 토큰 하나당 달러다. 백만 토큰당으로 환산해야 사람이 읽을 수 있다.
    var p = parseFloat(m.prompt);
    var o = parseFloat(m.output);
    if (!isNaN(p) && !isNaN(o)) {
      label += " · 입력 $" + (p * 1e6).toFixed(2) + " / 출력 $" + (o * 1e6).toFixed(2) + " (1M 토큰)";
    }
    return label;
  }

  // creditsBox는 남은 금액을 보여준다. **따로 부른다** — OpenRouter가 느리거나
  // 죽어도 설정 화면의 나머지는 떠야 한다.
  function creditsBox() {
    var box = el("div", { class: "ad-ai-credits" });
    var reload = el("button", { type: "button", class: "ad-act", text: "다시 확인" });

    function load() {
      clear(box);
      box.appendChild(el("p", { class: "ad-dim", text: "잔액을 확인하는 중…" }));
      api("GET", "/api/admin/ai/credits").then(function (r) {
        clear(box);
        if (!r.ok) {
          box.appendChild(el("p", { class: "ad-error", text: r.data.error || "잔액을 못 가져왔다" }));
          box.appendChild(el("div", { class: "ad-ai-row" }, [reload]));
          return;
        }
        box.appendChild(el("div", { class: "ad-stats ad-ai-stats" }, [
          statCard("남은 금액", money(r.data.remaining), "이 키로 더 쓸 수 있는 금액"),
          statCard("충전", money(r.data.total), "지금까지 넣은 금액"),
          statCard("사용", money(r.data.used), "지금까지 쓴 금액"),
        ]));
        box.appendChild(el("div", { class: "ad-ai-row" }, [reload]));
      });
    }
    reload.addEventListener("click", load);
    load();
    return box;
  }

  function showSettings() {
    var mine = drawTicket;
    clear(root);
    root.appendChild(el("div", { class: "ad-listhead" }, [el("h1", { text: "환경설정" })]));

    var choice = "system";
    try { choice = localStorage.getItem("blog-theme") || "system"; } catch (_) {}

    var buttons = [];
    function paint() {
      buttons.forEach(function (b) {
        b.setAttribute("aria-pressed", b.dataset.themeChoice === choice ? "true" : "false");
      });
    }
    function pick(v) {
      choice = v;
      var resolved = v === "system"
        ? (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light")
        : v;
      document.documentElement.dataset.theme = resolved;
      document.documentElement.dataset.themeChoice = v;
      try { localStorage.setItem("blog-theme", v); } catch (_) {}
      paint();
    }

    buttons = [["system", "시스템"], ["light", "화이트"], ["dark", "다크"]].map(function (t) {
      return el("button", {
        type: "button", class: "ad-seg", "data-theme-choice": t[0], text: t[1],
        onclick: function () { pick(t[0]); },
      });
    });
    buttons.forEach(function (b) { b.dataset.themeChoice = b.getAttribute("data-theme-choice"); });
    paint();

    root.appendChild(el("section", { class: "ad-card" }, [
      el("h2", { text: "테마" }),
      el("p", { class: "ad-dim", text: "이 브라우저에만 저장한다. 공개 화면과 같은 설정이다." }),
      el("div", { class: "ad-segs" }, buttons),
    ]));

    root.appendChild(aiCard());

    root.appendChild(el("section", { class: "ad-card" }, [
      el("h2", { text: "계정" }),
      el("p", { class: "ad-dim", text: "로그인한 계정과 로그아웃은 화면 오른쪽 위에 있다. 들어올 수 있는 계정은 서버의 허용 목록이 정하므로 여기서 바꿀 수 없다." }),
      el("p", {}, [el("a", { class: "ad-act", href: "/", target: "_blank", rel: "noreferrer", text: "공개 화면 보기 ↗" })]),
    ]));
  }

  // ---------------------------------------------------------------- 데이터 보기
  //
  // **이 아카이브가 지금 어떤 상태인지 한 화면에서 본다.** 글이 1,356편이라
  // 목록을 넘겨서는 전체 모양이 안 보인다 — 어느 분류에 쏠려 있나, 안 쓰는
  // 이미지가 있나 같은 것은 지금까지 sqlite3을 직접 열어야 알 수 있었고,
  // 그게 "DB를 손으로 열지 마라"와 부딪혔다.
  //
  // **방문자 수는 없다.** 이 서버는 그런 것을 남기지 않는다. 여기서 세는 것은
  // 전부 내가 쓴 것이다.

  function num(n) { return (n || 0).toLocaleString(); }
  function kb(n) {
    if (!n) return "0";
    if (n < 1024) return n + "B";
    if (n < 1024 * 1024) return Math.round(n / 1024) + "KB";
    return (n / 1024 / 1024).toFixed(1) + "MB";
  }

  // 막대 하나. 값이 아니라 **가장 큰 것에 대한 비율**로 그린다.
  function bar(value, max) {
    var w = max > 0 ? Math.max(2, Math.round((value / max) * 100)) : 0;
    return el("span", { class: "ad-bar" }, [
      el("span", { class: "ad-bar-fill", style: "width:" + w + "%" }),
    ]);
  }

  function statCard(label, value, note) {
    return el("div", { class: "ad-stat" }, [
      el("p", { class: "ad-stat-label", text: label }),
      el("p", { class: "ad-stat-value", text: value }),
      note ? el("p", { class: "ad-stat-note", text: note }) : null,
    ]);
  }

  function showStats() {
    var mine = drawTicket;
    clear(root);
    root.appendChild(el("p", { class: "ad-empty", text: "세는 중…" }));

    api("GET", "/api/admin/stats").then(function (r) {
      if (stale(mine)) return;
      clear(root);
      if (!r.ok) {
        root.appendChild(el("p", { class: "ad-error", text: r.data.error || "데이터를 못 가져왔다" }));
        return;
      }
      var d = r.data;

      root.appendChild(el("div", { class: "ad-editbar" }, [
        el("a", {
          class: "ad-back", href: "/admin",
          onclick: function (e) { e.preventDefault(); go("/admin"); }, text: "← 목록",
        }),
        el("h1", { text: "데이터" }),
      ]));

      // ── 한눈에
      root.appendChild(el("div", { class: "ad-stats" }, [
        statCard("전체 글", num(d.posts.total),
          "공개 " + num(d.posts.unlisted + d.posts.published) + " · draft " + num(d.posts.draft)),
        statCard("본문", kb(d.body.bytes),
          "중앙값 " + num(d.body.median) + "자 · 최대 " + num(d.body.max) + "자"),
        statCard("이미지", num(d.images.count) + "장",
          kb(d.images.bytes) + (d.images.unused ? " · 안 쓰는 것 " + d.images.unused + "장" : "")),
        statCard("분류", num(d.categories.length),
          d.orphans.emptyCats ? "글 없는 분류 " + d.orphans.emptyCats + "개" : "전부 글이 있다"),
      ]));

      // ── 손볼 곳. **0이면 줄을 안 그린다** — 할 일 없는 목록에 0을 늘어놓으면
      //    진짜 할 일이 묻힌다.
      var todo = [];
      if (d.posts.draft) todo.push(["draft", d.posts.draft + "편이 아직 공개되지 않았다"]);
      if (d.body.empty) todo.push(["본문이 빈 글", d.body.empty + "편"]);
      if (d.orphans.noCategory) todo.push(["분류 없는 글", d.orphans.noCategory + "편"]);
      if (d.orphans.noDate) todo.push(["작성일 없는 글", d.orphans.noDate + "편 (목록에서 날짜가 빈다)"]);
      if (d.orphans.emptyCats) todo.push(["글 없는 분류", d.orphans.emptyCats + "개"]);
      if (d.images.unused) todo.push(["아무 글도 안 쓰는 이미지", d.images.unused + "장 (지우는 도구가 아직 없다)"]);
      if (d.orphans.native) todo.push(["웹에서 쓴 글", d.orphans.native + "편 — 재이관이 되살리지 않는다"]);
      if (todo.length) {
        root.appendChild(el("section", { class: "ad-panel" }, [
          el("h2", { text: "눈여겨볼 것" }),
          el("ul", { class: "ad-todo" }, todo.map(function (t) {
            return el("li", {}, [el("b", { text: t[0] }), el("span", { text: t[1] })]);
          })),
        ]));
      }

      // ── 해마다 쓴 글
      if (d.years && d.years.length) {
        var ymax = Math.max.apply(null, d.years.map(function (y) { return y.count; }));
        root.appendChild(el("section", { class: "ad-panel" }, [
          el("h2", { text: "해마다 쓴 글" }),
          el("p", { class: "ad-note", text: "원본 작성일 기준이다. 이관 시점이 아니라 실제로 쓴 해다." }),
          el("ul", { class: "ad-rows" }, d.years.map(function (y) {
            return el("li", {}, [
              el("span", { class: "ad-row-name mono", text: y.name }),
              bar(y.count, ymax),
              el("span", { class: "ad-row-num", text: num(y.count) }),
            ]);
          })),
        ]));
      }

      // deleteCategory는 분류 지우기 버튼이 공통으로 쓴다. 서버가 글·하위
      // 분류가 있다고 400으로 거절하면, 그 문구를 그대로 확인창에 띄워
      // "그래도 지울까?"로 되묻는다 — 확인하면 ?force=true로 다시 부른다.
      function deleteCategory(c) {
        api("DELETE", "/api/admin/categories/" + c.id).then(function (r) {
          if (r.ok) { cats = null; showStats(); return; }
          var msg = r.data && r.data.error ? r.data.error : "지우지 못했다";
          if (r.status !== 400 || !confirm(msg)) { if (r.status !== 400) alert(msg); return; }
          api("DELETE", "/api/admin/categories/" + c.id + "?force=true").then(function (r2) {
            if (!r2.ok) { alert(r2.data && r2.data.error ? r2.data.error : "지우지 못했다"); return; }
            cats = null; // 편집기가 다시 불러오게 캐시를 비운다
            showStats();
          });
        });
      }

      function deleteButton(c) {
        return el("button", {
          class: "ad-btn danger", text: "지우기",
          onclick: function () { deleteCategory(c); },
        });
      }

      // ── 글 없는 분류.
      var empties = d.categories.filter(function (c) { return c.posts === 0; });
      if (empties.length) {
        root.appendChild(el("section", { class: "ad-panel" }, [
          el("h2", { text: "글 없는 분류" }),
          el("p", { class: "ad-note", text: "실수로 만들었거나 다 지워서 비어 있는 분류다." }),
          el("ul", { class: "ad-rows" }, empties.map(function (c) {
            return el("li", {}, [
              el("span", { class: "ad-row-name", title: c.path, text: c.path }),
              c.children ? el("span", { class: "ad-dim", text: "하위 분류 " + c.children + "개" }) : null,
              deleteButton(c),
            ]);
          })),
        ]));
      }

      // ── 분류별. 직속 글만 센다.
      var top = d.categories.filter(function (c) { return c.posts > 0; });
      var cmax = top.length ? top[0].posts : 0;
      root.appendChild(el("section", { class: "ad-panel" }, [
        el("h2", { text: "분류별 글" }),
        el("p", { class: "ad-note", text: "직속 글만 센다 — 하위까지 더하면 상위 분류가 전부를 삼켜서 쏠림이 안 보인다." }),
        el("ul", { class: "ad-rows" }, top.map(function (c) {
          return el("li", {}, [
            el("span", { class: "ad-row-name", title: c.path, text: c.path }),
            bar(c.posts, cmax),
            el("span", { class: "ad-row-num", text: num(c.posts) + (c.drafts ? " (draft " + c.drafts + ")" : "") }),
            deleteButton(c),
          ]);
        })),
      ]));
    });
  }

  // ---------------------------------------------------------------- 지도
  //
  // 글 949편을 점으로, 부모-자식과 본문 링크를 선으로 그린다. 힘-기반
  // 배치(Fruchterman-Reingold 계열)를 손으로 굴린다 — D3 같은 라이브러리를
  // CDN에서 받아오면 이 저장소의 "빌드 스텝도 CDN도 없다" 규칙과 부딪힌다.
  //
  // **몇백 프레임만 굴리고 멈춘다.** 계속 움직이는 그래프는 보기 힘들고,
  // 950개 점을 매 프레임 밀어내는 건 배터리에도 나쁘다. 자리가 잡히면
  // 멈추고, 그 다음은 사람이 끌고 돌리고 확대하는 정적인 지도가 된다.
  function showGraph() {
    var mine = drawTicket;
    clear(root);
    root.appendChild(el("div", { class: "ad-editbar" }, [
      el("h1", { text: "글 지도" }),
    ]));
    var note = el("p", { class: "ad-note", text: "불러오는 중…" });
    root.appendChild(note);

    api("GET", "/api/admin/graph").then(function (r) {
      if (stale(mine)) return;
      if (!r.ok) {
        note.textContent = (r.data && r.data.error) || "지도를 못 가져왔다";
        note.className = "ad-error";
        return;
      }
      note.remove();
      renderGraph(r.data.nodes || [], r.data.edges || []);
    });
  }

  function renderGraph(nodes, edges) {
    if (!nodes.length) {
      root.appendChild(el("p", { class: "ad-empty", text: "글이 없다." }));
      return;
    }

    var wrap = el("div", { class: "ad-graph-wrap" });
    var canvas = el("canvas", { class: "ad-graph", "aria-label": "글 지도" });
    var legend = el("p", {
      class: "ad-note", id: "ad-graph-legend",
      text: "굵은 선은 하위 글, 옅은 선은 본문 링크다. 끌어서 옮기고, 휠로 확대하고, 점을 누르면 그 글을 연다.",
    });
    wrap.appendChild(canvas);
    root.appendChild(legend);
    root.appendChild(wrap);

    var ctx = canvas.getContext("2d");
    var reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

    // ── 자료 준비: id → 색인, 최상위 분류 → 뭉치 중심.
    var byID = {};
    nodes.forEach(function (n, i) {
      n.x = 0; n.y = 0; n.vx = 0; n.vy = 0; n.fixed = false;
      byID[n.id] = n;
    });
    var tops = [];
    var topIndex = {};
    nodes.forEach(function (n) {
      var key = n.top || "(분류 없음)";
      if (!(key in topIndex)) { topIndex[key] = tops.length; tops.push(key); }
    });
    // 뭉치 중심을 서로 충분히 떼어놔야 한다 — 안 그러면 뭉치 안 밀어내기가
    // 서로 겹쳐서 지도가 그냥 균일한 그물처럼 보인다(글 수가 늘수록 더 그렇다).
    var anchorR = 260 + Math.sqrt(nodes.length) * 22;
    var anchors = tops.map(function (_, i) {
      var a = (i / tops.length) * Math.PI * 2;
      return { x: Math.cos(a) * anchorR, y: Math.sin(a) * anchorR };
    });
    nodes.forEach(function (n) {
      var key = n.top || "(분류 없음)";
      var anchor = anchors[topIndex[key]];
      var a = Math.random() * Math.PI * 2, r = Math.random() * 60;
      n.ax = anchor.x; n.ay = anchor.y;
      n.x = anchor.x + Math.cos(a) * r;
      n.y = anchor.y + Math.sin(a) * r;
    });

    // 이웃 목록. 스프링 힘과 호버 강조가 같이 쓴다.
    var neighbors = {};
    nodes.forEach(function (n) { neighbors[n.id] = []; });
    var validEdges = edges.filter(function (e) { return byID[e.source] && byID[e.target]; });
    validEdges.forEach(function (e) {
      neighbors[e.source].push(e.target);
      neighbors[e.target].push(e.source);
    });

    // ── 힘 시뮬레이션. 격자로 나눠 가까운 점끼리만 밀어낸다 — 950개를
    // 전부 서로 비교하면(n^2) 프레임마다 90만 번 거리 계산이 나온다.
    var cell = 50;
    function repel() {
      var grid = {};
      nodes.forEach(function (n) {
        var key = (Math.floor(n.x / cell)) + "," + (Math.floor(n.y / cell));
        (grid[key] || (grid[key] = [])).push(n);
      });
      nodes.forEach(function (n) {
        var cx = Math.floor(n.x / cell), cy = Math.floor(n.y / cell);
        for (var dx = -1; dx <= 1; dx++) {
          for (var dy = -1; dy <= 1; dy++) {
            var bucket = grid[(cx + dx) + "," + (cy + dy)];
            if (!bucket) continue;
            for (var i = 0; i < bucket.length; i++) {
              var o = bucket[i];
              if (o === n) continue;
              var ddx = n.x - o.x, ddy = n.y - o.y;
              var d2 = ddx * ddx + ddy * ddy || 0.01;
              if (d2 > 6000) continue;
              var f = 400 / d2;
              n.vx += ddx * f; n.vy += ddy * f;
            }
          }
        }
      });
    }

    var running = true, iter = 0, maxIter = reduceMotion ? 0 : 240;
    function tick(temp) {
      repel();
      validEdges.forEach(function (e) {
        var a = byID[e.source], b = byID[e.target];
        var ddx = b.x - a.x, ddy = b.y - a.y;
        var dist = Math.sqrt(ddx * ddx + ddy * ddy) || 0.01;
        var target = e.kind === "parent" ? 45 : 90;
        var strength = (e.kind === "parent" ? 0.02 : 0.006) * (dist - target);
        var fx = (ddx / dist) * strength, fy = (ddy / dist) * strength;
        a.vx += fx; a.vy += fy; b.vx -= fx; b.vy -= fy;
      });
      nodes.forEach(function (n) {
        if (n.fixed) return;
        // 뭉치 중심으로 끌린다 — 분류별로 은하처럼 갈라져 보이는 이유다.
        n.vx += (n.ax - n.x) * 0.01;
        n.vy += (n.ay - n.y) * 0.01;
        n.vx *= 0.82; n.vy *= 0.82;
        n.x += n.vx * temp; n.y += n.vy * temp;
      });
    }

    // ── 보기(팬·줌). 세계 좌표 ↔ 화면 좌표.
    var view = { scale: 1, x: 0, y: 0 };
    // 캔버스 크기를 바꾸면 브라우저가 내용을 지운다. 자리가 이미 다 잡혀서
    // 프레임 루프가 멈춘 뒤라면(running이 거짓) 아무도 다시 그려주지 않으니
    // 여기서 직접 다시 맞추고 그린다 — 안 그러면 창 크기를 바꾸는 순간
    // 지도가 통째로 비어 보인다.
    function resize() {
      var r = wrap.getBoundingClientRect();
      canvas.width = Math.max(320, r.width) * devicePixelRatio;
      canvas.height = 420 * devicePixelRatio;
      canvas.style.height = "420px";
      if (typeof running !== "undefined" && !running) { fitView(); draw(); }
    }
    resize();
    window.addEventListener("resize", resize);

    function cssVar(name) {
      return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    }

    var hovered = null, dragging = null, panning = null;
    var mine = drawTicket;

    function draw() {
      var w = canvas.width, h = canvas.height;
      ctx.save();
      ctx.clearRect(0, 0, w, h);
      ctx.translate(w / 2 + view.x, h / 2 + view.y);
      ctx.scale(view.scale * devicePixelRatio, view.scale * devicePixelRatio);

      var ink = cssVar("--ink") || "#111";
      var dim = cssVar("--line-mid") || "#999";
      var rail = cssVar("--rail") || "#4aa8ff";
      var hoverSet = hovered ? [hovered.id].concat(neighbors[hovered.id]) : null;

      validEdges.forEach(function (e) {
        var a = byID[e.source], b = byID[e.target];
        var lit = hoverSet && hoverSet.indexOf(e.source) >= 0 && hoverSet.indexOf(e.target) >= 0;
        ctx.beginPath();
        ctx.moveTo(a.x, a.y);
        ctx.lineTo(b.x, b.y);
        ctx.lineWidth = (e.kind === "parent" ? 1.4 : 0.7) / view.scale;
        ctx.strokeStyle = lit ? rail : dim;
        ctx.globalAlpha = lit ? 0.9 : (e.kind === "parent" ? 0.55 : 0.25);
        ctx.stroke();
      });
      ctx.globalAlpha = 1;

      nodes.forEach(function (n) {
        var lit = hoverSet && hoverSet.indexOf(n.id) >= 0;
        var r = (n.id === (hovered && hovered.id) ? 5 : 3) / Math.sqrt(view.scale);
        ctx.beginPath();
        ctx.arc(n.x, n.y, r, 0, Math.PI * 2);
        ctx.fillStyle = lit ? rail : (n.status === "draft" ? dim : ink);
        ctx.globalAlpha = hoverSet && !lit ? 0.35 : 1;
        ctx.fill();
      });
      ctx.globalAlpha = 1;

      if (hovered) {
        ctx.font = (12 / view.scale) + "px sans-serif";
        ctx.fillStyle = ink;
        ctx.fillText(hovered.title, hovered.x + 8 / view.scale, hovered.y - 8 / view.scale);
      }
      ctx.restore();
    }

    // fitView는 자리가 다 잡힌 뒤 한 번, 전체가 다 보이게 확대·이동한다.
    // 사람이 그 전에 손으로 줌·팬을 했으면(userMoved) 건드리지 않는다 —
    // 보던 자리를 갑자기 옮기면 안 된다.
    var userMoved = false;
    function fitView() {
      if (userMoved) return;
      var minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity;
      nodes.forEach(function (n) {
        if (n.x < minX) minX = n.x; if (n.x > maxX) maxX = n.x;
        if (n.y < minY) minY = n.y; if (n.y > maxY) maxY = n.y;
      });
      var spanX = Math.max(1, maxX - minX), spanY = Math.max(1, maxY - minY);
      var s = Math.min(
        (canvas.width * 0.92) / spanX / devicePixelRatio,
        (canvas.height * 0.92) / spanY / devicePixelRatio
      );
      view.scale = Math.min(6, Math.max(0.08, s));
      view.x = -view.scale * devicePixelRatio * (minX + maxX) / 2;
      view.y = -view.scale * devicePixelRatio * (minY + maxY) / 2;
    }

    function frame() {
      if (stale(mine)) { window.removeEventListener("resize", resize); return; }
      if (running) {
        var temp = Math.max(0.05, 1 - iter / maxIter);
        tick(temp);
        iter++;
        if (iter >= maxIter) { running = false; fitView(); }
      }
      draw();
      if (running || dragging || panning) requestAnimationFrame(frame);
    }
    if (reduceMotion) { for (; iter < 60; iter++) tick(0.6); running = false; fitView(); }
    requestAnimationFrame(frame);

    // ── 입력. Pointer Events라 마우스와 터치를 같은 코드로 받는다.
    function toWorld(clientX, clientY) {
      var rect = canvas.getBoundingClientRect();
      var sx = clientX - rect.left, sy = clientY - rect.top;
      return {
        x: (sx - rect.width / 2 - view.x) / view.scale,
        y: (sy - rect.height / 2 - view.y) / view.scale,
      };
    }
    function hitTest(clientX, clientY) {
      var p = toWorld(clientX, clientY);
      var best = null, bestD = 10 / view.scale;
      nodes.forEach(function (n) {
        var d = Math.hypot(n.x - p.x, n.y - p.y);
        if (d < bestD) { bestD = d; best = n; }
      });
      return best;
    }

    canvas.addEventListener("pointerdown", function (e) {
      canvas.setPointerCapture(e.pointerId);
      var hit = hitTest(e.clientX, e.clientY);
      if (hit) { dragging = { node: hit, moved: false }; hit.fixed = true; }
      else panning = { x: e.clientX, y: e.clientY, vx: view.x, vy: view.y };
      if (!running) requestAnimationFrame(frame);
    });
    canvas.addEventListener("pointermove", function (e) {
      if (dragging) {
        dragging.moved = true;
        var p = toWorld(e.clientX, e.clientY);
        dragging.node.x = p.x; dragging.node.y = p.y;
        dragging.node.vx = 0; dragging.node.vy = 0;
        if (!running) draw();
      } else if (panning) {
        userMoved = true;
        view.x = panning.vx + (e.clientX - panning.x);
        view.y = panning.vy + (e.clientY - panning.y);
        draw();
      } else {
        var hit = hitTest(e.clientX, e.clientY);
        if (hit !== hovered) { hovered = hit; canvas.style.cursor = hit ? "pointer" : "grab"; if (!running) draw(); }
      }
    });
    function endPointer() {
      if (dragging && !dragging.moved) go("/admin/edit/" + encodeURIComponent(dragging.node.slug));
      if (dragging) dragging.node.fixed = false;
      dragging = null; panning = null;
    }
    canvas.addEventListener("pointerup", endPointer);
    canvas.addEventListener("pointercancel", endPointer);
    canvas.addEventListener("wheel", function (e) {
      e.preventDefault();
      userMoved = true;
      var factor = Math.exp(-e.deltaY * 0.001);
      view.scale = Math.min(6, Math.max(0.08, view.scale * factor));
      draw();
    }, { passive: false });
    canvas.style.cursor = "grab";
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

  // prefillCategory는 공개 화면에서 넘겨준 분류다(`/admin/new?category=12`).
  //
  // 분류를 보다가 "새 글"을 누른 사람은 그 분류에 쓰려는 것이다. 안 받으면
  // 방금 보던 것을 잊고 88개짜리 선택 상자를 처음부터 다시 고르게 된다.
  //
  // **숫자가 아니면 그냥 무시한다.** 주소는 누구나 손으로 고칠 수 있는데,
  // 여기서 하는 일은 선택 상자의 초깃값을 정하는 것뿐이라 틀린 값은
  // "아무것도 안 고른 상태"가 되면 그만이다. 진짜 검사는 저장할 때
  // 서버가 한다(save.go가 없는 분류면 400을 준다).
  function prefillCategory() {
    var m = /[?&]category=(\d+)(?:&|$)/.exec(location.search);
    return m ? Number(m[1]) : null;
  }

  function showEditor(slug, categoryId) {
    var mine = drawTicket;
    clear(root);
    root.appendChild(el("p", { class: "ad-empty", text: "불러오는 중…" }));

    if (slug === null) {
      return loadCategories().then(function (cs) {
        if (stale(mine)) return;
        renderEditor({
          slug: "", title: "", body: "", status: "draft", visibility: "public",
          sortOrder: 0, categoryId: categoryId || null,
        }, true, cs);
      });
    }
    Promise.all([
      api("GET", "/api/admin/posts/" + encodeURIComponent(slug)),
      loadCategories(),
    ]).then(function (out) {
      if (stale(mine)) return;
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
    // 공개 범위는 status와 **다른 축**이다. status는 "어디까지 썼나"고
    // 이건 "누가 볼 수 있나"라, 한 칸에 밀어 넣으면 "draft이면서 비공개"를
    // 적을 수 없다. 그래서 선택 상자도 나란히 둘이다.
    var visSelect = el("select", { class: "ad-input", id: "ad-visibility" },
      (config.visibilities || ["public", "private"]).map(function (v) {
        var o = el("option", {
          value: v,
          text: v === "private" ? "private (허용된 계정만)" : "public (누구나)",
        });
        if (v === (post.visibility || "public")) o.selected = true;
        return o;
      }));

    // ── 메타 패널 ────────────────────────────────────────────────
    // 글 하나를 실제로 고치려면 본문만으로 부족하다. 어느 분류에 붙어 있고,
    // 어느 글의 자식이고, 형제 사이 몇 번째인지가 전부 posts의 다른 칸이다.
    // cats는 이 폼이 들고 있는 분류 목록이다. 새로 만들면 여기 밀어 넣고
    // 선택 상자를 다시 그린다 — 창을 새로고침해야 보이면 방금 만든 걸
    // 바로 못 쓴다.
    var catList = (categories || []).slice();
    var catSelect = el("select", { class: "ad-input", id: "ad-category" });

    function drawCatOptions() {
      clear(catSelect);
      catSelect.appendChild(el("option", { value: "", text: "— 분류 없음 —" }));
      catList.forEach(function (c) {
        var o = el("option", { value: String(c.id), text: c.path });
        if (post.categoryId === c.id) o.selected = true;
        catSelect.appendChild(o);
      });
    }
    drawCatOptions();

    // ── 그 자리에서 새 분류 만들기 ──────────────────────────────────
    //
    // **분류는 87개뿐이라 미리 다 갖춰두지 않는다.** 새 프로젝트·새 갈래가
    // 생기면 지금까지는 SQL을 직접 만져야 붙일 수 있었다 — 그 자리에서
    // 만들고 바로 이 글에 고르게 한다.
    var newCatName = el("input", {
      class: "ad-input", type: "text", placeholder: "새 분류 이름",
    });
    // 부모는 지금 목록에서 고른다. **깊이 2(0부터 세어 최상위가 0)까지만
    // 보여준다** — 그 밑에 자식을 넣으면 4단계가 되어 트리거가 막는다
    // (migrations/002). 미리 걸러야 헛눌러보고 오류를 받는 일이 없다.
    var newCatParent = el("select", { class: "ad-input" },
      [el("option", { value: "", text: "— 최상위 —" })]);
    function drawParentOptions() {
      var kept = catList.filter(function (c) { return c.depth < 2; });
      clear(newCatParent);
      newCatParent.appendChild(el("option", { value: "", text: "— 최상위 —" }));
      kept.forEach(function (c) {
        newCatParent.appendChild(el("option", { value: String(c.id), text: c.path }));
      });
    }
    drawParentOptions();
    var newCatMsg = el("p", { class: "ad-note" });
    var newCatBox = el("div", { class: "ad-newcat" }, [
      newCatName, newCatParent,
      el("button", { type: "button", class: "ad-btn", text: "만들기", onclick: createCat }),
      newCatMsg,
    ]);
    newCatBox.hidden = true;
    var newCatToggle = el("button", {
      type: "button", class: "ad-btn", text: "+ 새 분류",
      onclick: function () { newCatBox.hidden = !newCatBox.hidden; },
    });

    function createCat() {
      var name = newCatName.value.trim();
      if (!name) { newCatMsg.className = "ad-note ad-error"; newCatMsg.textContent = "이름을 적어라"; return; }
      newCatMsg.className = "ad-note";
      newCatMsg.textContent = "만드는 중…";
      api("POST", "/api/admin/categories", {
        name: name,
        parentId: newCatParent.value ? Number(newCatParent.value) : null,
      }).then(function (r) {
        if (!r.ok) {
          newCatMsg.className = "ad-note ad-error";
          newCatMsg.textContent = r.data.error || "만들지 못했다";
          return;
        }
        catList.push(r.data);
        drawCatOptions();
        drawParentOptions();
        catSelect.value = String(r.data.id);
        newCatName.value = "";
        newCatMsg.textContent = "";
        newCatBox.hidden = true;
      });
    }
    var parentInput = el("input", {
      class: "ad-input mono", type: "text", id: "ad-parent",
      placeholder: "부모 글의 slug (비우면 최상위)", value: post.parentSlug || "",
    });
    var sortInput = el("input", {
      class: "ad-input", type: "number", id: "ad-sort", min: "0",
      value: String(post.sortOrder || 0),
    });
    // **순서를 적는 것만으로는 화면이 안 바뀐다.** 공개 목록은 sort_order를
    // 안 믿는다 — 이관이 채운 값이 분 단위 created_time 순위라 시리즈가
    // 엇갈리기 때문이다(web.sortPosts). 사람이 정한 순서만 예외로 따르므로
    // (migrations/005의 sort_order_manual) 그 표시를 여기서 켠다.
    var manualInput = el("input", {
      class: "ad-check", type: "checkbox", id: "ad-sort-manual",
      title: "켜면 목록이 제목 번호 대신 이 순서를 따른다. 순서를 정한 글은 맨 앞에 선다",
    });
    manualInput.checked = !!post.sortOrderManual;
    var dateInput = el("input", {
      class: "ad-input", type: "date", id: "ad-date", value: dateValue(post.createdAt),
    });

    // ── 순서를 목록에서 옮긴다 ────────────────────────────────────
    //
    // 글 화면의 바로 고치기와 **같은 엔드포인트, 같은 규칙**이다
    // (internal/web/static/inline-edit.js). 목록을 아는 것은 공개 쪽이고
    // (web.SiblingOrder), 편집기가 제 눈으로 다시 세면 두 답이 갈라진다.
    var numberField = el("label", { class: "ad-field" }, [el("span", { text: "형제 순서" }),
      el("div", { class: "ad-order" }, [sortInput,
        el("label", { class: "ad-order-manual" },
          [manualInput, el("span", { text: "화면에 적용" })])])]);
    var orderBox = el("div", { class: "ad-siblings" });
    var order = null, firstOrder = "", siblings = null, askedOrder = false;
    numberField.hidden = true;

    function drawOrder(items) {
      orderBox.textContent = "";
      var list = el("ol", { class: "ad-sibs" });
      items.forEach(function (it, i) {
        var row = el("li", { class: it.current ? "is-me" : "" },
          [el("span", { class: "ad-sib-t", text: it.title })]);
        if (it.current) {
          var up = el("button", { type: "button", class: "ad-sib-mv", title: "위로",
            "aria-label": "위로", onclick: function () { moveOrder(-1); } }, [icon("M4 10l4-4 4 4")]);
          var dn = el("button", { type: "button", class: "ad-sib-mv", title: "아래로",
            "aria-label": "아래로", onclick: function () { moveOrder(1); } }, [icon("M4 6l4 4 4-4")]);
          up.disabled = i === 0;
          dn.disabled = i === items.length - 1;
          row.appendChild(up);
          row.appendChild(dn);
        }
        list.appendChild(row);
      });
      orderBox.appendChild(el("p", { class: "ad-sibs-cap",
        text: "이 분류 화면에 서는 차례다. 저장하면 이대로 보인다." }));
      orderBox.appendChild(list);
      siblings = items;
    }

    function moveOrder(by) {
      var i = -1;
      siblings.forEach(function (it, k) { if (it.current) i = k; });
      var j = i + by;
      if (j < 0 || j >= siblings.length) return;
      var moved = siblings.slice();
      moved.splice(j, 0, moved.splice(i, 1)[0]);
      order = moved.map(function (it) { return it.slug; });
      drawOrder(moved);
    }

    // **한 번만 받는다.** 패널을 여닫을 때마다 부르면 옮겨둔 것이 되돌아간다.
    function askOrder() {
      if (askedOrder || !post.slug) return;
      askedOrder = true;
      orderBox.textContent = "목록을 가져오는 중…";
      api("GET", "/api/admin/posts/" + encodeURIComponent(post.slug) + "/siblings")
        .then(function (r) {
          if (!r.ok) { orderBox.textContent = "목록을 가져오지 못했다."; return; }
          var d = r.data || {};
          if (d.reason || !(d.items || []).length) {
            orderBox.textContent = "";
            orderBox.appendChild(el("p", { class: "ad-sibs-why",
              text: d.reason || "이 글이 서는 목록을 찾지 못했다." }));
            numberField.hidden = false;
            return;
          }
          order = d.items.map(function (it) { return it.slug; });
          firstOrder = order.join("\n");
          drawOrder(d.items);
        });
    }

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
      isNew ? null : el("button", {
        class: "ad-btn danger", id: "ad-delete", onclick: remove, text: "지우기",
      }),
      el("label", { class: "ad-field ad-editbar-status" }, [el("span", { text: "상태" }), statusSelect]),
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

    root.appendChild(el("div", { class: "ad-fields" }, [
      el("label", { class: "ad-field wide" }, [el("span", { text: "제목" }), titleInput]),
      el("label", { class: "ad-field" }, [el("span", { text: "공개 범위" }), visSelect]),
      el("label", { class: "ad-field wide" }, [el("span", { text: "slug" }), slugInput]),
    ]));
    root.appendChild(el("details", { class: "ad-meta", ontoggle: function (e) {
      if (e.target.open) askOrder();
    } }, [
      el("summary", { text: "분류 · 계층 · 순서 · 날짜" }),
      el("div", { class: "ad-fields" }, [
        el("label", { class: "ad-field wide" }, [el("span", { text: "분류" }),
          el("div", { class: "ad-catrow" }, [catSelect, newCatToggle]), newCatBox]),
        el("label", { class: "ad-field wide" }, [el("span", { text: "부모 글" }), parentInput]),
        numberField,
        el("label", { class: "ad-field" }, [el("span", { text: "작성일" }), dateInput]),
        orderBox,
      ]),
      el("p", { class: "ad-note" }, [
        post.publishedAt
          ? "공개 시각 " + dateText(post.publishedAt) + " (status를 published로 처음 바꿀 때 서버가 찍는다)"
          : "status를 published로 바꾸면 그때 공개 시각이 찍힌다.",
      ]),
    ]));

    if (!isNew) {
      var reviseInstruction = el("textarea", { class: "ad-input ad-ai-instruction", rows: "3",
        maxlength: "2000", placeholder: "예: 반복을 줄이고 문장을 간결하게 다듬어줘. 사실과 코드는 유지해줘." });
      var reviseBtn = el("button", { type: "button", class: "ad-btn", text: "AI 수정안 받기",
        onclick: requestRevision });
      var reviseStatus = el("p", { class: "ad-note", role: "status" });
      root.appendChild(el("details", { class: "ad-card ad-ai-edit" }, [
        el("summary", { text: "AI로 기존 글 수정" }),
        el("p", { class: "ad-dim", text:
          "현재 제목·본문과 수정 요청을 OpenRouter에 보낸다. 제안을 편집기에 적용한 뒤 저장해야 글이 바뀐다." }),
        reviseInstruction, reviseBtn, reviseStatus,
      ]));
    }

    root.appendChild(el("div", { class: "ad-split" }, [
      el("section", { class: "ad-pane" }, [
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
    // **120ms다.** 사람이 한 글자를 더 치는 데 걸리는 시간보다 짧아서
    // "쓰자마자 보인다"에 가깝고, 그보다 줄이면 왕복이 겹치기 시작한다.
    var timer = null;
    function schedulePreview() {
      clearTimeout(timer);
      timer = setTimeout(renderPreview, 120);
    }
    bodyAreaRef.addEventListener("input", schedulePreview);

    // 커서가 수식 안에 들어가면 그 자리 위에 그린 수식이 뜬다
    // (internal/web/static/math-live.js). 글 화면의 바로 고치기와 같은 파일이다.
    if (window.blogMathLive) window.blogMathLive.attach(bodyAreaRef);

    // 미리보기가 같은 자리를 보게 따라 스크롤한다. 두 칸의 높이가 달라 줄을
    // 정확히 맞출 수는 없으므로 비율로 맞춘다.
    var syncing = false;
    bodyAreaRef.addEventListener("scroll", function () {
      if (syncing) return;
      var max = bodyAreaRef.scrollHeight - bodyAreaRef.clientHeight;
      if (max <= 0) return;
      syncing = true;
      preview.scrollTop = (bodyAreaRef.scrollTop / max) * (preview.scrollHeight - preview.clientHeight);
      requestAnimationFrame(function () { syncing = false; });
    });

    // 글이 길면 아래로 한참 내려간다. 맨 위(제목·상태·저장 버튼)로 바로
    // 돌아갈 수 있게 스크롤이 어느 정도 내려갔을 때만 뜨는 버튼을 둔다.
    var toTop = el("button", {
      class: "ad-totop", type: "button", title: "맨 위로",
      onclick: function () {
        var reduce = window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
        window.scrollTo({ top: 0, behavior: reduce ? "auto" : "smooth" });
      },
      text: "↑ 맨 위로",
    });
    toTop.hidden = true;
    root.appendChild(toTop);
    var mine = drawTicket;
    function onScroll() {
      if (stale(mine)) { window.removeEventListener("scroll", onScroll); return; }
      toTop.hidden = window.scrollY < 400;
    }
    window.addEventListener("scroll", onScroll);

    // `/`를 치면 조각 팔레트가 뜬다. **서버에 묻지 않으므로 지연이 없다.**
    // "이미지 올리기"만은 조각이 아니라 파일 고르는 창을 여는 항목이라,
    // 팔레트가 그걸 여기로 넘긴다.
    if (window.blogPalette) {
      window.blogPalette.attach(bodyAreaRef, null, function () {
        var picker = document.getElementById("ad-image");
        if (picker) picker.click();
      });
    }

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
        if (window.blogRenderMermaid) window.blogRenderMermaid();
        // 애니메이션도 다시 붙인다. innerHTML을 갈아치웠으니 통째로 사라졌다.
        if (window.blogMountAnims) window.blogMountAnims();

        var heads = r.data.outline || [];
        previewNote.className = "ad-note";
        previewNote.textContent = bodyAreaRef.value.length.toLocaleString() + "자" +
          (heads.length ? " · 제목 " + heads.length + "개" : "") +
          (heads.length >= 3 ? " (목차가 붙는다)" : "");
      });
    }
    renderPreview();

    function requestRevision() {
      var instruction = reviseInstruction.value.trim();
      if (!instruction) {
        reviseStatus.className = "ad-note ad-error";
        reviseStatus.textContent = "어떻게 고칠지 적어라";
        reviseInstruction.focus();
        return;
      }
      var sentTitle = titleInput.value;
      var sentBody = bodyAreaRef.value;
      reviseBtn.disabled = true;
      reviseBtn.textContent = "AI가 수정하는 중… (최대 5분)";
      reviseStatus.className = "ad-note";
      reviseStatus.textContent = "수정안을 기다리는 중…";
      api("POST", "/api/admin/posts/" + encodeURIComponent(post.slug) + "/ai-revise", {
        rev: post.rev, title: sentTitle, body: sentBody, instruction: instruction,
      }).then(function (r) {
        if (stale(mine)) return;
        reviseBtn.disabled = false;
        reviseBtn.textContent = "AI 수정안 받기";
        if (!r.ok) {
          reviseStatus.className = "ad-note ad-error";
          reviseStatus.textContent = r.data.error || "AI 수정안을 받지 못했다";
          return;
        }
        if (titleInput.value !== sentTitle || bodyAreaRef.value !== sentBody) {
          reviseStatus.className = "ad-note ad-error";
          reviseStatus.textContent = "AI가 작업하는 동안 편집 내용이 바뀌었다. 현재 내용을 기준으로 다시 요청해라";
          return;
        }
        showRevision(sentTitle, sentBody, r.data.title, r.data.body);
      }).catch(function () {
        if (stale(mine)) return;
        reviseBtn.disabled = false;
        reviseBtn.textContent = "AI 수정안 받기";
        reviseStatus.className = "ad-note ad-error";
        reviseStatus.textContent = "연결이 끊겨 AI 수정안을 받지 못했다";
      });
    }

    function showRevision(oldTitle, oldBody, newTitle, newBody) {
      var before = el("textarea", { class: "ad-input ad-ai-review-body", readonly: "readonly" });
      before.value = "# " + oldTitle + "\n\n" + oldBody;
      var after = el("textarea", { class: "ad-input ad-ai-review-body", readonly: "readonly" });
      after.value = "# " + newTitle + "\n\n" + newBody;
      var cancel = el("button", { type: "button", class: "ad-btn", text: "취소" });
      var apply = el("button", { type: "button", class: "ad-btn primary", text: "편집기에 적용" });
      var dialog = el("dialog", { class: "ad-modal ad-ai-review" }, [
        el("h2", { text: "AI 수정안 확인" }),
        el("p", { class: "ad-dim", text: "적용해도 저장 전까지 글은 바뀌지 않는다." }),
        el("div", { class: "ad-ai-review-grid" }, [
          el("label", {}, [el("span", { text: "현재 내용" }), before]),
          el("label", {}, [el("span", { text: "AI 제안" }), after]),
        ]),
        el("div", { class: "ad-modal-acts" }, [cancel, apply]),
      ]);
      cancel.addEventListener("click", function () { dialog.close(); });
      apply.addEventListener("click", function () {
        if (titleInput.value !== oldTitle || bodyAreaRef.value !== oldBody) {
          reviseStatus.className = "ad-note ad-error";
          reviseStatus.textContent = "편집 내용이 바뀌었다. 현재 내용을 기준으로 다시 요청해라";
          dialog.close();
          return;
        }
        titleInput.value = newTitle;
        bodyAreaRef.value = newBody;
        bodyAreaRef.dispatchEvent(new Event("input", { bubbles: true }));
        reviseStatus.className = "ad-note";
        reviseStatus.textContent = "AI 제안을 편집기에 적용했다. 확인한 뒤 저장해라";
        dialog.close();
      });
      dialog.addEventListener("close", function () { dialog.remove(); });
      document.body.appendChild(dialog);
      dialog.showModal();
      cancel.focus();
    }

    function save() {
      var payload = {
        slug: slugInput.value.trim(),
        title: titleInput.value.trim(),
        body: bodyAreaRef.value,
        status: statusSelect.value,
        visibility: visSelect.value,
        // **rev를 반드시 같이 보낸다.** 이걸 빼면 서버가 거절한다 — 두 탭에서
        // 연 글이 서로를 조용히 지우는 것을 막는 표다.
        rev: post.rev || "",
        categoryId: catSelect.value ? Number(catSelect.value) : null,
        parentSlug: parentInput.value.trim(),
        sortOrder: Number(sortInput.value) || 0,
        sortOrderManual: manualInput.checked,
        siblingOrder: order || [],
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
          (saved.status === "draft" ? " (draft라 공개 화면에는 안 보인다)" : "") +
          (saved.visibility === "private" ? " (비공개라 허용된 계정만 볼 수 있다)" : "");
        status.className = "ad-status ok";
        status.textContent = msg;

        // 저장 후에는 편집기를 새 rev로 갱신한다. 공개 가능한 글에만
        // 화면 링크를 보여준다. draft와 private는 공개 URL로 확인할 수 없다.
        if (saved.status !== "draft") {
          post = saved;
          history.replaceState({}, "", "/admin/edit/" + encodeURIComponent(saved.slug));
          renderEditor(saved, false, catList, msg);
          if (saved.visibility === "public") {
            notify("글 배포에 성공했다. 공개 화면에서 결과를 확인할 수 있다.", "ok",
              { href: "/p/" + encodeURIComponent(saved.slug), text: "화면으로 가보기" });
          } else {
            notify("비공개 글을 저장했다. 허용된 계정만 볼 수 있다.");
          }
          return;
        }
        if (isNewPost || saved.slug !== post.slug) {
          // slug가 정해졌거나 바뀌었으면 주소도 그리로 옮긴다. 새로고침했을 때
          // 없는 글을 열지 않게 하려는 것이다. 다시 그리는 폼에 방금 그 말을
          // 같이 넘긴다.
          post = saved;
          history.replaceState({}, "", "/admin/edit/" + encodeURIComponent(saved.slug));
          renderEditor(saved, false, catList, msg);
          return;
        }
        post = saved;
        slugInput.value = saved.slug;
      });
    }

    // ── 지우기 ────────────────────────────────────────────────────
    //
    // **무엇을 잃는지 먼저 묻고 보여준다.** 확인 창의 "예"를 무엇인지 모른 채
    // 누르게 하지 않는다. 서버가 refs로 알려주고, 자식이 있으면 아예 못 지운다.
    function remove() {
      api("GET", "/api/admin/posts/" + encodeURIComponent(post.slug) + "/refs")
        .then(function (r) {
          if (!r.ok) {
            status.className = "ad-status ad-error";
            status.textContent = r.data.error || "무엇이 걸리는지 알아내지 못했다";
            return;
          }
          var refs = r.data;
          if (refs.children && refs.children.length) {
            status.className = "ad-status ad-error";
            status.textContent = "하위 글 " + refs.children.length + "편이 매달려 있다: " +
              refs.children.slice(0, 3).join(", ") +
              (refs.children.length > 3 ? " 외" : "") +
              " — 그것들을 먼저 옮기거나 지워라";
            return;
          }
          var lose = [];
          if (refs.notion) lose.push("노션에서 온 글이라 **다음 재이관이 되살린다** (진짜로 빼려면 internal/curation의 DropPosts에 적어야 한다)");
          if (refs.coverOf && refs.coverOf.length) lose.push("분류 " + refs.coverOf.join(", ") + "의 표지가 사라진다");
          if (refs.linkedFrom && refs.linkedFrom.length) lose.push("이 글을 가리키던 " + refs.linkedFrom.length + "편의 링크를 글자로 푼다 (" + refs.linkedFrom.slice(0, 3).join(", ") + ")");

          var msg = "\"" + post.title + "\"을(를) 지운다.";
          if (lose.length) msg += "\n\n" + lose.map(function (l, i) { return (i + 1) + ". " + l; }).join("\n");
          msg += "\n\n되돌릴 수 없다. 지울까?";
          if (!window.confirm(msg)) return;

          status.className = "ad-status";
          status.textContent = "지우는 중…";
          api("DELETE", "/api/admin/posts/" + encodeURIComponent(post.slug),
            { rev: post.rev || "", force: lose.length > 0 }).then(function (d) {
            if (!d.ok) {
              status.className = "ad-status ad-error";
              status.textContent = d.data.error || ("지우지 못했다 (HTTP " + d.status + ")");
              return;
            }
            go("/admin");
          });
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
