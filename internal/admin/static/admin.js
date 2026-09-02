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
  //   /admin/data             데이터 보기

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
    if (path === "/admin/new") return showEditor(null);
    if (path === "/admin/data") return showStats();
    if (path === "/admin/settings") return showSettings();
    return showList();
  }

  // markMenu는 상단 메뉴에서 지금 있는 곳을 표시한다.
  //
  // **메뉴 자체는 서버가 그렸다.** 여기서 하는 일은 표시뿐이라, 스크립트가
  // 못 떠도 세 링크는 그대로 눌린다. 새 글과 편집 화면은 `전체 글`에 딸린
  // 자리라 그쪽을 켠다 — 어디에도 안 걸린 화면을 만들지 않는다.
  function markMenu(path) {
    var here = path === "/admin/data" || path === "/admin/settings" ? path : "/admin";
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

  // ---------------------------------------------------------------- 환경설정
  //
  // **여기 있는 것은 이 브라우저의 설정뿐이다.** 서버에 저장하지 않고
  // localStorage에만 남는다 — 공개 화면의 화면 설정과 같은 자리, 같은 키를
  // 쓰므로 admin에서 다크로 바꾸면 공개 화면도 다크다.
  //
  // **없는 것을 있는 척하지 않는다.** 글쓰기 기본값이나 계정 설정 같은 것은
  // 아직 저장할 자리가 없어서 여기 두지 않는다.

  function showSettings() {
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
          ]);
        })),
      ]));
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
    var mine = drawTicket;
    clear(root);
    root.appendChild(el("p", { class: "ad-empty", text: "불러오는 중…" }));

    if (slug === null) {
      return loadCategories().then(function (cs) {
        if (stale(mine)) return;
        renderEditor({ slug: "", title: "", body: "", status: "draft", visibility: "public", sortOrder: 0 }, true, cs);
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
      isNew ? null : el("button", {
        class: "ad-btn danger", id: "ad-delete", onclick: remove, text: "지우기",
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
          el("label", { class: "ad-field" }, [el("span", { text: "공개 범위" }), visSelect]),
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
