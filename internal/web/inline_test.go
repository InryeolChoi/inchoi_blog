package web

import (
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/inryeol/blog"
	"github.com/inryeol/blog/internal/db"
)

// inlineFixture는 "탐색적 자료분석" 글의 모양을 그대로 흉내낸 서버를 만든다.
//
//	수학 & 통계 > 탐색적 자료분석                     ← 주인 글
//	  ├ R언어 팁 정리                                ← 진짜 하위 글(child_page)
//	  └ 탐색적 자료분석 : 목차                        ← 인라인 데이터베이스(글이 아니다)
//	      ├ 1. 파일 다루기
//	      └ 2. 데이터프레임 다루기
//
// 데이터베이스는 posts에 행이 없다. 그래서 본문의 링크가 404가 되고, 그 밑의
// 글들은 경로로만 찾을 수 있다.
func inlineFixture(t *testing.T) (*Server, http.Handler) {
	t.Helper()

	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if _, err := db.Migrate(sqlDB, blog.MigrationsFS()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := sqlDB.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO categories (id, name, slug, sort_order) VALUES (1, '수학 & 통계', 'math-stat', 0)`)

	now := time.Now().UTC()
	post := func(slug, title, body, path string, sortOrder int) {
		t.Helper()
		exec(`INSERT INTO posts (slug, title, body, status, source, category_id, sort_order,
		      original_path, created_at, updated_at)
		      VALUES (?, ?, ?, 'unlisted', 'notion', 1, ?, ?, ?, ?)`,
			slug, title, body, sortOrder, path, now, now)
	}

	const base = "수학 & 통계 > 탐색적 자료분석"
	ownerBody := strings.Join([]string{
		"## 기초지식",
		"",
		"[페이지 링크](/p/r-tips)",
		"",
		"[탐색적 자료분석 : 목차](/p/db-0001)",
		"",
		"[R언어 팁 정리](/p/r-tips)",
		"",
	}, "\n")
	post("owner", "탐색적 자료분석", ownerBody, base, 0)
	post("r-tips", "R언어 팁 정리", "팁", base+" > R언어 팁 정리", 0)
	post("row-1", "1. 파일 다루기", "본문1", base+" > 탐색적 자료분석 : 목차 > 1. 파일 다루기", 0)
	post("row-2", "2. 데이터프레임 다루기", "본문2", base+" > 탐색적 자료분석 : 목차 > 2. 데이터프레임 다루기", 1)

	srv, err := New(sqlDB)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv, srv.Handler()
}

// TestResolveBodyFillsPlaceholder는 link_to_page 자리표시자가 대상 글의 제목으로
// 바뀌는지 본다. 링크 주소는 그대로여야 한다.
func TestResolveBodyFillsPlaceholder(t *testing.T) {
	srv, _ := inlineFixture(t)

	out, fix, err := srv.resolveBody("[페이지 링크](/p/r-tips)", "수학 & 통계 > 탐색적 자료분석")
	if err != nil {
		t.Fatal(err)
	}
	if want := "[R언어 팁 정리](/p/r-tips)"; out != want {
		t.Errorf("got  %q\nwant %q", out, want)
	}
	if fix.Titled != 1 {
		t.Errorf("Titled = %d, want 1", fix.Titled)
	}
}

// TestResolveBodyKeepsPlaceholderWhenTargetMissing은 대상 글이 없으면 그대로
// 두는지 본다. 바꿔 넣을 제목 자체가 없다.
func TestResolveBodyKeepsPlaceholderWhenTargetMissing(t *testing.T) {
	srv, _ := inlineFixture(t)

	body := "[페이지 링크](/p/없는글)"
	out, fix, err := srv.resolveBody(body, "수학 & 통계 > 탐색적 자료분석")
	if err != nil {
		t.Fatal(err)
	}
	if out != body {
		t.Errorf("건드렸다: %q", out)
	}
	if fix.Titled != 0 {
		t.Errorf("Titled = %d, want 0", fix.Titled)
	}
}

// TestResolveBodyLeavesPlainTextAlone은 본문에 "페이지 링크"라는 말이 그냥
// 글자로 들어 있을 때 건드리지 않는지 본다. 실제로 그런 글이 1건 있다.
func TestResolveBodyLeavesPlainTextAlone(t *testing.T) {
	srv, _ := inlineFixture(t)

	body := "‘정보활용 거부 페이지 링크’ 를 제공하여 [R](/p/r-tips) 처럼 쓴다"
	out, fix, err := srv.resolveBody(body, "수학 & 통계 > 탐색적 자료분석")
	if err != nil {
		t.Fatal(err)
	}
	if out != body {
		t.Errorf("건드렸다:\ngot  %q\nwant %q", out, body)
	}
	if fix.Titled != 0 {
		t.Errorf("Titled = %d, want 0", fix.Titled)
	}
}

// TestEscapeLinkTextGuardsBrackets는 제목의 대괄호가 링크를 깨지 않는지 본다.
// 제목에 대괄호나 백슬래시가 든 글이 8건 있다.
func TestEscapeLinkTextGuardsBrackets(t *testing.T) {
	if got, want := escapeLinkText(`[Ch1] 빅데이터`), `\[Ch1\] 빅데이터`; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := escapeLinkText(`a\b`), `a\\b`; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestResolveBodyExpandsInlineDB는 인라인 데이터베이스 링크가 그 밑 글 목록으로
// 펼쳐지는지 본다. 이게 이 파일의 본론이다.
func TestResolveBodyExpandsInlineDB(t *testing.T) {
	srv, _ := inlineFixture(t)

	out, fix, err := srv.resolveBody("[탐색적 자료분석 : 목차](/p/db-0001)", "수학 & 통계 > 탐색적 자료분석")
	if err != nil {
		t.Fatal(err)
	}
	if fix.Expanded != 1 || fix.Rows != 2 {
		t.Fatalf("Expanded=%d Rows=%d, want 1/2 (%q)", fix.Expanded, fix.Rows, out)
	}
	for _, want := range []string{
		`class="inline-db"`,
		`탐색적 자료분석 : 목차`,
		`<a href="/p/row-1">1. 파일 다루기</a>`,
		`<a href="/p/row-2">2. 데이터프레임 다루기</a>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("%q가 없다:\n%s", want, out)
		}
	}
	if strings.Contains(out, "/p/db-0001") {
		t.Errorf("죽은 링크가 남았다:\n%s", out)
	}
}

// TestInlineDBHTMLHasNoNewline은 펼친 HTML에 줄바꿈이 없는지 본다.
// CommonMark의 HTML 블록은 빈 줄에서 끝나므로, 줄이 나뉘면 뒷부분이 마크다운으로
// 다시 해석돼 태그가 글자로 드러난다.
func TestInlineDBHTMLHasNoNewline(t *testing.T) {
	rows := []PostSummary{{Slug: "a", Title: "가", Status: "draft",
		Children: []PostSummary{{Slug: "b", Title: "나", Status: "unlisted"}}}}
	got, err := inlineDBHTML("목차", rows)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(got, "\n\r") {
		t.Errorf("줄바꿈이 있다: %q", got)
	}
	if !strings.Contains(got, `<span class="status">draft</span>`) {
		t.Errorf("draft 뱃지가 없다: %q", got)
	}
	if !strings.Contains(got, `<a href="/p/b">나</a>`) {
		t.Errorf("하위 글이 없다: %q", got)
	}
}

// TestInlineDBGroupsSkipsRealChildPages는 바로 아래 한 칸인 이름을 데이터베이스로
// 오해하지 않는지 본다. 그건 실제로 존재하는 글이라, 그 밑의 손자 글을 데이터베이스
// 목록으로 끌어오면 남의 글이 섞인다.
func TestInlineDBGroupsSkipsRealChildPages(t *testing.T) {
	srv, _ := inlineFixture(t)

	groups, err := srv.store.InlineDBGroups("수학 & 통계 > 탐색적 자료분석")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := groups["R언어 팁 정리"]; ok {
		t.Error("진짜 하위 글을 데이터베이스로 묶었다")
	}
	if got := len(groups["탐색적 자료분석 : 목차"]); got != 2 {
		t.Errorf("행 수 = %d, want 2 (%v)", got, groups)
	}
}

// TestResolveBodyExpandsSameDBOnce는 같은 이름의 데이터베이스 링크가 두 번
// 나와도 한 번만 펼치는지 본다. 두 번 펼치면 같은 글이 두 벌로 보인다.
func TestResolveBodyExpandsSameDBOnce(t *testing.T) {
	srv, _ := inlineFixture(t)

	body := "[탐색적 자료분석 : 목차](/p/db-0001)\n\n[탐색적 자료분석 : 목차](/p/db-0002)"
	out, fix, err := srv.resolveBody(body, "수학 & 통계 > 탐색적 자료분석")
	if err != nil {
		t.Fatal(err)
	}
	if fix.Expanded != 1 || fix.Left != 1 {
		t.Errorf("Expanded=%d Left=%d, want 1/1", fix.Expanded, fix.Left)
	}
	if !strings.Contains(out, "/p/db-0002") {
		t.Errorf("짝 못 찾은 링크를 지웠다:\n%s", out)
	}
}

// TestResolveBodyKeepsUnmatchedDeadLink는 짝을 못 찾은 죽은 링크를 그대로 두는지
// 본다. 억지로 아무 목록이나 붙이면 남의 글이 섞여 들어간다. 지금 19개가 여기 해당한다.
func TestResolveBodyKeepsUnmatchedDeadLink(t *testing.T) {
	srv, _ := inlineFixture(t)

	body := "[Untitled](/p/db-9999)"
	out, fix, err := srv.resolveBody(body, "수학 & 통계 > 탐색적 자료분석")
	if err != nil {
		t.Fatal(err)
	}
	if out != body {
		t.Errorf("건드렸다: %q", out)
	}
	if fix.Left != 1 || fix.Expanded != 0 {
		t.Errorf("Left=%d Expanded=%d, want 1/0", fix.Left, fix.Expanded)
	}
}

// TestPostPageExpandsInlineDB는 글 상세 페이지의 HTML에 목록이 실제로 나오는지
// 본다. 마크다운 렌더러가 raw HTML을 글자로 만들어버리면 여기서 잡힌다.
func TestPostPageExpandsInlineDB(t *testing.T) {
	_, h := inlineFixture(t)

	rec := get(t, h, "/p/owner")
	if rec.Code != http.StatusOK {
		t.Fatalf("상태 코드 %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`<a href="/p/row-1">1. 파일 다루기</a>`,
		`<a href="/p/row-2">2. 데이터프레임 다루기</a>`,
		`<a href="/p/r-tips">R언어 팁 정리</a>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("%q가 없다", want)
		}
	}
	if strings.Contains(body, "페이지 링크") {
		t.Error("자리표시자가 그대로 나왔다")
	}
	if strings.Contains(body, "/p/db-0001") {
		t.Error("죽은 데이터베이스 링크가 그대로 나왔다")
	}
	// 태그가 글자로 새어나오면 안 된다.
	if strings.Contains(body, "&lt;ul") || strings.Contains(body, "&lt;div") {
		t.Error("HTML이 글자로 드러났다")
	}
}

// TestCoverBodyExpandsInlineDB는 카테고리 페이지에 펼쳐지는 표지 글 본문에도
// 같은 처리가 걸리는지 본다. 두 군데서 본문을 그리므로 한쪽만 고치면 어긋난다.
func TestCoverBodyExpandsInlineDB(t *testing.T) {
	srv, h := inlineFixture(t)

	var id int64
	if err := srv.store.db.QueryRow(`SELECT id FROM posts WHERE slug = 'owner'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.store.db.Exec(`UPDATE categories SET cover_post_id = ? WHERE id = 1`, id); err != nil {
		t.Fatal(err)
	}

	rec := get(t, h, "/math-stat")
	if rec.Code != http.StatusOK {
		t.Fatalf("상태 코드 %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `<a href="/p/row-1">1. 파일 다루기</a>`) {
		t.Error("표지 글 본문에서 인라인 데이터베이스가 안 펼쳐졌다")
	}
}

// TestResolveBodyWithoutOriginalPath는 경로가 없는 글에서 조용히 넘어가는지 본다.
// 경로가 유일한 단서라 없으면 펼칠 수 없다. 에러가 나면 안 된다.
func TestResolveBodyWithoutOriginalPath(t *testing.T) {
	srv, _ := inlineFixture(t)

	body := "[목차](/p/db-0001)"
	out, fix, err := srv.resolveBody(body, "")
	if err != nil {
		t.Fatal(err)
	}
	if out != body || fix.Left != 1 {
		t.Errorf("got %q fix=%+v", out, fix)
	}
}

// TestDropCoveredChildrenRemovesFullyShown은 표지 글 본문이 통째로 펼쳐 보여준
// 하위 분류를 목록에서 빼는지 본다. 한 화면에 같은 것이 두 번 나오지 않게 한다.
func TestDropCoveredChildrenRemovesFullyShown(t *testing.T) {
	srv, _ := inlineFixture(t)

	// 인라인 데이터베이스의 두 글을 하위 분류 하나에 몰아넣는다.
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := srv.store.db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO categories (id, name, slug, parent_id, sort_order) VALUES (2, '목차', 'toc', 1, 0)`)
	exec(`UPDATE posts SET category_id = 2 WHERE slug IN ('row-1', 'row-2')`)

	children, err := srv.store.ChildCategories(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 1 {
		t.Fatalf("하위 분류가 %d개다", len(children))
	}

	got, err := srv.dropCoveredChildren(children, map[string]bool{"row-1": true, "row-2": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("본문이 다 보여준 분류가 남았다: %+v", got)
	}
}

// TestDropCoveredChildrenKeepsPartiallyShown은 분류에 본문이 안 보여준 글이
// 하나라도 있으면 남기는지 본다. 이름만 보고 빼면 그 글로 가는 길이 사라진다.
// 실제로 `Language > 프로그래밍 언어`가 그렇다 — 이름은 같은데 분류에 191건,
// 본문에 펼쳐진 건 7건이다.
func TestDropCoveredChildrenKeepsPartiallyShown(t *testing.T) {
	srv, _ := inlineFixture(t)

	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := srv.store.db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO categories (id, name, slug, parent_id, sort_order) VALUES (2, '목차', 'toc', 1, 0)`)
	exec(`UPDATE posts SET category_id = 2 WHERE slug IN ('row-1', 'row-2')`)

	children, _ := srv.store.ChildCategories(1)
	got, err := srv.dropCoveredChildren(children, map[string]bool{"row-1": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("일부만 보여줬는데 뺐다: %+v", got)
	}
}

// TestDropCoveredChildrenLeavesEmptyCategories는 글이 없는 분류를 건드리지
// 않는지 본다. "본문이 다 보여줬다"고 말할 근거가 없다.
func TestDropCoveredChildrenLeavesEmptyCategories(t *testing.T) {
	srv, _ := inlineFixture(t)

	if _, err := srv.store.db.Exec(
		`INSERT INTO categories (id, name, slug, parent_id, sort_order) VALUES (2, '빈 분류', 'empty', 1, 0)`); err != nil {
		t.Fatal(err)
	}
	children, _ := srv.store.ChildCategories(1)
	got, err := srv.dropCoveredChildren(children, map[string]bool{"row-1": true, "row-2": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("빈 분류를 뺐다: %+v", got)
	}
}

// TestPostPageHasOutline은 제목이 충분히 많은 글에 목차가 붙고 앵커가 본문의
// id와 맞는지 본다.
func TestPostPageHasOutline(t *testing.T) {
	srv, h := inlineFixture(t)

	body := "## 하나\n\n가\n\n## 둘\n\n나\n\n### 셋\n\n다\n"
	if _, err := srv.store.db.Exec(`UPDATE posts SET body = ? WHERE slug = 'r-tips'`, body); err != nil {
		t.Fatal(err)
	}

	page := get(t, h, "/p/r-tips").Body.String()
	if !strings.Contains(page, `<nav class="toc"`) {
		t.Fatal("목차가 없다")
	}
	// href의 조각은 URL 인코딩된다. 디코딩해서 본문 id와 맞춰본다.
	for _, want := range []string{"하나", "둘", "셋"} {
		frag := regexp.MustCompile(`href="#([^"]+)"`)
		found := false
		for _, m := range frag.FindAllStringSubmatch(page, -1) {
			if dec, err := url.QueryUnescape(m[1]); err == nil && dec == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%q로 가는 목차 링크가 없다", want)
		}
	}
	for _, want := range []string{`<h2 id="하나">`, `<h2 id="둘">`, `<h3 id="셋">`} {
		if !strings.Contains(page, want) {
			t.Errorf("본문 앵커 %q가 없다", want)
		}
	}
}

// TestPostPageSkipsShortOutline은 제목이 몇 개 안 되는 글에는 목차를 안 붙이는지
// 본다. 한두 줄짜리 목차는 자리만 차지한다.
func TestPostPageSkipsShortOutline(t *testing.T) {
	srv, h := inlineFixture(t)

	if _, err := srv.store.db.Exec(
		`UPDATE posts SET body = '## 하나' || char(10) || char(10) || '가' WHERE slug = 'r-tips'`); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(get(t, h, "/p/r-tips").Body.String(), `<nav class="toc"`) {
		t.Error("제목 1개짜리에 목차가 붙었다")
	}
}

// TestOutlineUsesResolvedBody는 목차를 "손본 뒤의 본문"에서 뽑는지 본다.
// 인라인 데이터베이스를 펼치면 raw HTML이 끼어드는데, 그걸 넣기 전 원문으로
// 목차를 뽑으면 앵커 번호가 어긋날 수 있다.
func TestOutlineUsesResolvedBody(t *testing.T) {
	srv, _ := inlineFixture(t)

	post, err := srv.store.PostBySlug("owner")
	if err != nil || post == nil {
		t.Fatalf("PostBySlug: %v", err)
	}
	rendered, err := srv.renderPostBody(post)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rendered.HTML), `<a href="/p/row-1">1. 파일 다루기</a>`) {
		t.Error("본문이 안 펼쳐졌다")
	}
	for _, h := range rendered.Outline {
		if !strings.Contains(string(rendered.HTML), `id="`+h.ID+`"`) {
			t.Errorf("본문에 없는 앵커: %q", h.ID)
		}
	}
}
