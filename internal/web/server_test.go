package web

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/inryeol/blog"
	"github.com/inryeol/blog/internal/db"
)

// testDB는 마이그레이션을 건 빈 DB를 만든다.
//
// :memory:를 쓰지 않는다. 커넥션 풀이 커넥션마다 별개의 인메모리 DB를 만들어서
// 마이그레이션을 건 DB와 조회하는 DB가 달라진다.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if _, err := db.Migrate(sqlDB, blog.MigrationsFS()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return sqlDB
}

// execer는 실패하면 바로 멈추는 Exec을 만든다.
func execer(t *testing.T, sqlDB *sql.DB) func(string, ...any) sql.Result {
	t.Helper()
	return func(q string, args ...any) sql.Result {
		t.Helper()
		res, err := sqlDB.Exec(q, args...)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return res
	}
}

// testServer는 작은 트리를 넣은 서버를 만든다.
//
//	개발(dev) > 언어(language) > 파이썬(python) > 글 "리스트"
//	                                            표지 글 "언어"는 language에 붙는다
func testServer(t *testing.T) http.Handler {
	t.Helper()

	sqlDB := testDB(t)
	exec := execer(t, sqlDB)

	exec(`INSERT INTO categories (id, name, slug, sort_order) VALUES (1, '개발', 'dev', 0)`)
	exec(`INSERT INTO categories (id, name, slug, parent_id, sort_order) VALUES (2, '언어', 'language', 1, 0)`)
	exec(`INSERT INTO categories (id, name, slug, parent_id, sort_order) VALUES (3, '파이썬', 'python', 2, 0)`)
	// dev 바로 아래에도 하나 둔다. 경로 검사가 부모까지 보는지 확인하는 데 쓴다.
	// categories.slug는 전역 UNIQUE라 같은 slug를 두 부모 밑에 둘 수는 없다.
	exec(`INSERT INTO categories (id, name, slug, parent_id, sort_order) VALUES (4, '도구', 'tools', 1, 1)`)

	now := time.Now().UTC()
	post := func(id int, slug, title, body string, catID int) {
		exec(`INSERT INTO posts (id, slug, title, body, status, source, category_id, sort_order, created_at, updated_at)
		      VALUES (?, ?, ?, ?, 'unlisted', 'notion', ?, 0, ?, ?)`,
			id, slug, title, body, catID, now, now)
	}
	post(1, "cover-language", "언어", "표지 본문\n\n## 목차\n\n[목차 글](/p/toc-post)\n\n## 참고 동영상\n\n[영상](https://www.youtube.com/watch?v=fNk_zzaMoSs)", 2)
	post(5, "category-post", "언어 글", "언어 글 본문", 2)
	post(6, "toc-post", "목차 글", "목차 글 본문", 2)
	post(7, "toc-child", "목차 글의 하위 글", "하위 글 본문", 2)
	exec(`UPDATE posts SET parent_id = 6 WHERE id = 7`)
	post(2, "list-post", "리스트", "# 리스트\n\n식은 $x_1 + y_2$ 이다.\n\n$$\n\\sum_{i=1}^{n} i\n$$\n", 3)
	exec(`UPDATE categories SET cover_post_id = 1 WHERE id = 2`)

	// 노션 계층이 카테고리 3단계보다 깊었던 모습을 재현한다.
	// "묶음"은 python 카테고리 안에 있지만 부모는 한 단계 위 카테고리의 표지 글이고,
	// "리스트"는 그 묶음 밑에 들어간다. 실제 수리통계1이 이 모양이다.
	post(3, "section-post", "묶음", "묶음 본문", 3)
	exec(`UPDATE posts SET parent_id = 1 WHERE id = 3`)
	exec(`UPDATE posts SET parent_id = 3 WHERE id = 2`)

	// status 뱃지는 draft만 찍는다. 양쪽을 다 보려면 draft 글이 하나 있어야 한다.
	post(4, "draft-post", "초안", "초안 본문", 3)
	exec(`UPDATE posts SET status = 'draft' WHERE id = 4`)

	exec(`INSERT INTO images (sha256, data, mime, created_at) VALUES (?, ?, 'image/png', ?)`,
		strings.Repeat("ab", 32), []byte{0x89, 0x50, 0x4e, 0x47}, now)

	srv, err := New(sqlDB)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv.Handler()
}

// mainOf는 본문 영역만 잘라낸다.
//
// 사이드바가 모든 페이지에 분류 트리를 통째로 그리기 때문에 필요하다.
// 그냥 페이지 전체에서 href를 찾으면 "본문에 있다"가 아니라 "사이드바에
// 있다"를 확인하게 되어 테스트가 아무것도 지키지 못한다.
func mainOf(t *testing.T, body string) string {
	t.Helper()
	i := strings.Index(body, `<main`)
	if i < 0 {
		t.Fatalf("<main>이 없다:\n%s", body)
	}
	return body[i:]
}

// sideOf는 사이드바 영역만 잘라낸다.
func sideOf(t *testing.T, body string) string {
	t.Helper()
	i := strings.Index(body, `<aside`)
	j := strings.Index(body, `<main`)
	if i < 0 || j < i {
		t.Fatalf("<aside>가 없다:\n%s", body)
	}
	return body[i:j]
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestIndexListsTopCategories(t *testing.T) {
	rec := get(t, testServer(t), "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("상태 코드 %d", rec.Code)
	}
	body := mainOf(t, rec.Body.String())
	if !strings.Contains(body, `href="/dev"`) {
		t.Errorf("최상위 카테고리 링크가 없다:\n%s", body)
	}
	// 하위 글까지 세야 한다 (dev 아래 글 7건)
	if !strings.Contains(body, `class="home-field-meta">7 notes`) {
		t.Errorf("하위 글 수가 홈 카드에 안 세졌다:\n%s", body)
	}
}

func TestCategoryPageShowsChildrenAndPosts(t *testing.T) {
	h := testServer(t)

	rec := get(t, h, "/dev")
	if rec.Code != http.StatusOK {
		t.Fatalf("/dev 상태 코드 %d", rec.Code)
	}
	if !strings.Contains(mainOf(t, rec.Body.String()), `href="/dev/language"`) {
		t.Errorf("하위 분류 링크가 없다:\n%s", rec.Body.String())
	}

	rec = get(t, h, "/dev/language")
	if rec.Code != http.StatusOK {
		t.Fatalf("/dev/language 상태 코드 %d", rec.Code)
	}
	body := mainOf(t, rec.Body.String())
	if !strings.Contains(body, `href="/dev/language/python"`) {
		t.Errorf("3단계 링크가 없다:\n%s", body)
	}
	if !strings.Contains(body, `href="/p/category-post"`) {
		t.Errorf("직속 글 링크가 없다:\n%s", body)
	}
	// 표지 글은 위에서 이미 본문으로 펼쳤으므로 목록과 "따로 보기" 링크로
	// 한 번 더 내보내지 않는다.
	if got := strings.Count(body, `href="/p/cover-language"`); got != 0 {
		t.Errorf("표지 글 링크가 %d개 남았다:\n%s", got, body)
	}
	if strings.Contains(body, `<span class="cover">`) {
		t.Errorf("표지 뱃지가 남아 있다:\n%s", body)
	}
}

// TestThirdLevelCategoryReachable은 3단계 카테고리가 열리는지 본다.
// 실제 데이터에서 글의 91%가 3단계에 있다.
func TestThirdLevelCategoryReachable(t *testing.T) {
	rec := get(t, testServer(t), "/dev/language/python")
	if rec.Code != http.StatusOK {
		t.Fatalf("상태 코드 %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `href="/p/list-post"`) {
		t.Errorf("3단계 카테고리의 글이 안 보인다:\n%s", rec.Body.String())
	}
}

// TestCategoryPathChecksParent는 URL 경로가 실제 트리와 맞는지 보는지 확인한다.
// 존재하는 카테고리라도 엉뚱한 부모 밑에 붙여 부르면 404여야 한다.
func TestCategoryPathChecksParent(t *testing.T) {
	h := testServer(t)

	// 제자리 경로는 열린다.
	for _, path := range []string{"/dev", "/dev/tools", "/dev/language", "/dev/language/python"} {
		if rec := get(t, h, path); rec.Code != http.StatusOK {
			t.Errorf("%s 가 열려야 하는데 %d", path, rec.Code)
		}
	}

	// python은 language의 자식이지 tools의 자식이 아니다.
	if rec := get(t, h, "/dev/tools/python"); rec.Code != http.StatusNotFound {
		t.Errorf("/dev/tools/python 은 404여야 하는데 %d", rec.Code)
	}
	// language는 최상위가 아니다.
	if rec := get(t, h, "/language"); rec.Code != http.StatusNotFound {
		t.Errorf("/language 는 404여야 하는데 %d (2단계를 최상위로 열면 안 된다)", rec.Code)
	}
}

func TestUnknownCategoryIs404(t *testing.T) {
	h := testServer(t)
	for _, path := range []string{"/nope", "/dev/nope", "/dev/language/nope"} {
		if rec := get(t, h, path); rec.Code != http.StatusNotFound {
			t.Errorf("%s 는 404여야 하는데 %d", path, rec.Code)
		}
	}
}

func TestPostRendersMarkdown(t *testing.T) {
	rec := get(t, testServer(t), "/p/list-post")
	if rec.Code != http.StatusOK {
		t.Fatalf("상태 코드 %d", rec.Code)
	}
	body := rec.Body.String()

	if !strings.Contains(body, "<h1") {
		t.Errorf("마크다운 제목이 렌더링되지 않았다:\n%s", body)
	}
	// 수식은 원문 그대로 남아야 한다 (KaTeX가 나중에 그린다).
	if !strings.Contains(body, `class="math math-inline"`) {
		t.Errorf("인라인 수식 노드가 없다:\n%s", body)
	}
	if !strings.Contains(body, `class="math math-display"`) {
		t.Errorf("블록 수식 노드가 없다:\n%s", body)
	}
	if !strings.Contains(body, `\sum_{i=1}^{n} i`) {
		t.Errorf("LaTeX가 변형됐다:\n%s", body)
	}
	if strings.Contains(body, "<em>") {
		t.Errorf("수식 안의 밑줄이 기울임이 됐다:\n%s", body)
	}
}

// TestPostShowsCategoryTrail은 글 위에 카테고리 경로가 뜨는지 본다.
func TestPostShowsCategoryTrail(t *testing.T) {
	rec := get(t, testServer(t), "/p/list-post")
	body := rec.Body.String()
	for _, want := range []string{`href="/dev"`, `href="/dev/language"`, `href="/dev/language/python"`} {
		if !strings.Contains(body, want) {
			t.Errorf("경로 링크 %s 가 없다:\n%s", want, body)
		}
	}
}

func TestUnknownPostIs404(t *testing.T) {
	if rec := get(t, testServer(t), "/p/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("404여야 하는데 %d", rec.Code)
	}
}

func TestImageServesBlobWithMIME(t *testing.T) {
	sha := strings.Repeat("ab", 32)
	rec := get(t, testServer(t), "/img/"+sha)
	if rec.Code != http.StatusOK {
		t.Fatalf("상태 코드 %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
	if got := rec.Body.Bytes(); string(got) != string([]byte{0x89, 0x50, 0x4e, 0x47}) {
		t.Errorf("BLOB이 그대로 안 나왔다: %v", got)
	}
}

func TestImageRejectsBadHash(t *testing.T) {
	h := testServer(t)
	for _, path := range []string{"/img/short", "/img/" + strings.Repeat("zz", 32)} {
		if rec := get(t, h, path); rec.Code != http.StatusNotFound {
			t.Errorf("%s 는 404여야 하는데 %d", path, rec.Code)
		}
	}
}

// TestRoutePrecedence는 /p/ 와 /img/ 가 카테고리 패턴에 먹히지 않는지 본다.
func TestRoutePrecedence(t *testing.T) {
	h := testServer(t)

	// /p/{slug}가 /{l1}보다 먼저 잡혀야 한다. 카테고리로 갔으면 404 대신
	// 카테고리 404가 났을 것이므로, 글이 제대로 나오는지로 확인한다.
	rec := get(t, h, "/p/cover-language")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "표지 본문") {
		t.Errorf("/p/{slug} 라우트가 안 잡혔다: %d\n%s", rec.Code, rec.Body.String())
	}
}

// TestKoreanSlugURL은 한글 slug가 URL 인코딩돼 나가고 다시 찾히는지 본다.
func TestKoreanSlugURL(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sqlDB.Close()
	if _, err := db.Migrate(sqlDB, blog.MigrationsFS()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := sqlDB.Exec(
		`INSERT INTO categories (id, name, slug, sort_order) VALUES (1, '운영체제', '운영체제', 0)`); err != nil {
		t.Fatalf("INSERT: %v", err)
	}
	srv, err := New(sqlDB)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	h := srv.Handler()

	// href는 퍼센트 인코딩돼 나간다. html/template은 소문자 16진수를 쓰고
	// url.PathEscape는 대문자를 쓴다. 둘 다 유효하므로 대소문자를 무시하고 본다.
	rec := get(t, h, "/")
	want := `href="/` + url.PathEscape("운영체제") + `"`
	if !strings.Contains(strings.ToLower(rec.Body.String()), strings.ToLower(want)) {
		t.Errorf("한글 slug 링크(%s)가 없다:\n%s", want, rec.Body.String())
	}

	// 브라우저가 보내는 형태(퍼센트 인코딩)로도, 날것으로도 열려야 한다.
	for _, path := range []string{"/" + url.PathEscape("운영체제"), "/운영체제"} {
		if rec := get(t, h, path); rec.Code != http.StatusOK {
			t.Errorf("%s 가 안 열린다: %d", path, rec.Code)
		}
	}
}

// TestTrailLinksAreNotDoubleEncoded는 경로 링크가 두 번 인코딩되지 않는지 본다.
//
// crumbs()가 Go에서 url.PathEscape로 만든 경로를 href에 넣으면, html/template이
// href 문맥에서 한 번 더 처리한다. 여기서 %를 %25로 바꿔버리면 링크가 깨진다.
// 실제 카테고리 slug는 대부분 한글이라 전부 이 경로를 탄다.
func TestTrailLinksAreNotDoubleEncoded(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sqlDB.Close()
	if _, err := db.Migrate(sqlDB, blog.MigrationsFS()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := sqlDB.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO categories (id,name,slug,sort_order) VALUES (1,'CS 이론','cs-theory',0)`)
	exec(`INSERT INTO categories (id,name,slug,parent_id,sort_order) VALUES (2,'운영체제','운영체제',1,0)`)
	exec(`INSERT INTO categories (id,name,slug,parent_id,sort_order) VALUES (3,'Part 2','part-2',2,0)`)
	now := time.Now().UTC()
	exec(`INSERT INTO posts (id,slug,title,body,status,source,category_id,sort_order,created_at,updated_at)
	      VALUES (1,'x','글','본문','unlisted','notion',3,0,?,?)`, now, now)

	srv, err := New(sqlDB)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	body := get(t, srv.Handler(), "/p/x").Body.String()

	if strings.Contains(body, "%25") {
		t.Errorf("경로가 두 번 인코딩됐다:\n%s", body)
	}
	// 3단계 경로 링크가 온전히 나와야 한다.
	deep := "/cs-theory/" + url.PathEscape("운영체제") + "/part-2"
	if !strings.Contains(strings.ToLower(body), strings.ToLower(deep)) {
		t.Errorf("3단계 경로 링크(%s)가 없다:\n%s", deep, body)
	}
}

// TestCategoryPageNestsPostsByParent는 카테고리 목록이 parent_id 계층대로
// 중첩되는지 본다. 평평하게 나열되면 원래 몇 단계였는지 알 수 없다.
func TestCategoryPageNestsPostsByParent(t *testing.T) {
	rec := get(t, testServer(t), "/dev/language/python")
	if rec.Code != http.StatusOK {
		t.Fatalf("상태 코드 %d", rec.Code)
	}
	body := rec.Body.String()

	// 부모가 이 카테고리 밖(언어 표지 글)이라 "묶음"이 뿌리로 올라오고,
	// "리스트"는 그 밑에 중첩된 목록으로 들어가야 한다.
	sec := strings.Index(body, `href="/p/section-post"`)
	list := strings.Index(body, `href="/p/list-post"`)
	if sec < 0 || list < 0 {
		t.Fatalf("두 글이 다 보여야 한다:\n%s", body)
	}
	if sec > list {
		t.Errorf("묶음이 리스트보다 먼저 나와야 한다:\n%s", body)
	}

	// 두 링크 사이에 안쪽 <ul>이 열려야 중첩이다.
	between := body[sec:list]
	if !strings.Contains(between, "<ul") {
		t.Errorf("리스트가 묶음 안에 중첩되지 않았다:\n%s", between)
	}
}

// TestPostShowsAncestorsAndChildren은 글 상세가 상위 글 사슬과 하위 글을
// 보여주는지 본다.
func TestPostShowsAncestorsAndChildren(t *testing.T) {
	h := testServer(t)

	// 잎: 상위 사슬이 빵부스러기에 이어져야 한다.
	body := get(t, h, "/p/list-post").Body.String()
	for _, want := range []string{`href="/p/cover-language"`, `href="/p/section-post"`} {
		if !strings.Contains(body, want) {
			t.Errorf("상위 글 링크 %s 가 없다:\n%s", want, body)
		}
	}

	// 중간 노드: 하위 글이 보여야 한다. 자기 본문이 없어도 사슬에서 빠지지 않는다.
	body = get(t, h, "/p/section-post").Body.String()
	if !strings.Contains(body, "하위 글") || !strings.Contains(body, `href="/p/list-post"`) {
		t.Errorf("하위 글 목록이 없다:\n%s", body)
	}
}

// TestNestPostsKeepsEveryPost는 parent_id가 서로를 가리키며 도는 경우에도
// 글이 목록에서 사라지지 않는지 본다. posts.parent_id에는 categories와 달리
// 깊이/순환을 막는 트리거가 없다.
func TestNestPostsKeepsEveryPost(t *testing.T) {
	ref := func(id int64) sql.NullInt64 { return sql.NullInt64{Int64: id, Valid: true} }
	flat := []PostSummary{
		{ID: 1, Title: "뿌리"},
		{ID: 2, Title: "자식", ParentID: ref(1)},
		{ID: 3, Title: "바깥 부모를 가리킴", ParentID: ref(99)},
		{ID: 4, Title: "순환 A", ParentID: ref(5)},
		{ID: 5, Title: "순환 B", ParentID: ref(4)},
	}

	seen := map[int64]bool{}
	var walk func(ps []PostSummary)
	walk = func(ps []PostSummary) {
		for _, p := range ps {
			if seen[p.ID] {
				t.Fatalf("글 %d가 두 번 나온다", p.ID)
			}
			seen[p.ID] = true
			walk(p.Children)
		}
	}
	walk(nestPosts(flat))

	if len(seen) != len(flat) {
		t.Errorf("글 %d건 중 %d건만 나왔다: %v", len(flat), len(seen), seen)
	}
}

// TestStatusBadgeOnlyForDraft는 목록과 상세에서 unlisted 뱃지를 감추는지 본다.
// 1408건 중 1003건이 unlisted라 전부 찍으면 목록이 그 글자로 덮인다.
func TestStatusBadgeOnlyForDraft(t *testing.T) {
	h := testServer(t)

	// 픽스처의 글은 전부 unlisted다. 어디에도 그 글자가 보이면 안 된다.
	for _, path := range []string{"/dev/language/python", "/p/list-post"} {
		if body := get(t, h, path).Body.String(); strings.Contains(body, "unlisted") {
			t.Errorf("%s 에 unlisted 뱃지가 남아 있다:\n%s", path, body)
		}
	}

	// draft는 손봐야 할 신호라 계속 보여야 한다.
	for _, path := range []string{"/dev/language/python", "/p/draft-post"} {
		if body := get(t, h, path).Body.String(); !strings.Contains(body, "draft") {
			t.Errorf("%s 에 draft 뱃지가 없다:\n%s", path, body)
		}
	}
}

// TestCategoryShowsCoverBody는 표지 글이 있는 카테고리를 열면 그 본문이 바로
// 나오는지 본다. 소개처럼 목록이 아니라 글 자체가 알맹이인 분류가 있다.
func TestCategoryShowsCoverBody(t *testing.T) {
	h := testServer(t)

	// language에는 표지 글 "언어"가 붙어 있다.
	body := get(t, h, "/dev/language").Body.String()
	if !strings.Contains(body, "표지 본문") {
		t.Errorf("표지 글 본문이 카테고리 페이지에 안 나온다:\n%s", body)
	}
	if strings.Contains(mainOf(t, body), `href="/p/cover-language"`) {
		t.Errorf("없애기로 한 표지 글 따로 보기 링크가 남았다:\n%s", body)
	}

	// 표지가 없는 카테고리는 <article>을 그리지 않는다.
	if body := get(t, h, "/dev/tools").Body.String(); strings.Contains(body, "<article>") {
		t.Errorf("표지가 없는데 본문 영역이 그려졌다:\n%s", body)
	}
}

// TestCategoryDoesNotRepeatCoverTableOfContents는 표지 본문의 목차가 가리키는
// 글 트리를 아래 "글" 목록에서 한 번 더 펼치지 않는지 본다.
func TestCategoryDoesNotRepeatCoverTableOfContents(t *testing.T) {
	body := mainOf(t, get(t, testServer(t), "/dev/language").Body.String())

	// 목차 링크는 상단 본문에 한 번만 남는다.
	if got := strings.Count(body, `href="/p/toc-post"`); got != 1 {
		t.Errorf("목차 글 링크가 %d개다, 상단 목차 하나만 있어야 한다:\n%s", got, body)
	}
	// 목차가 가리킨 부모 아래의 하위 글도 하단 트리와 함께 빠진다.
	if strings.Contains(body, `href="/p/toc-child"`) {
		t.Errorf("목차 글의 하위 트리가 아래 목록에 다시 나왔다:\n%s", body)
	}
	// 표지에서 안내하지 않은 글은 길을 잃지 않도록 아래 목록에 남는다.
	if !strings.Contains(body, `href="/p/category-post"`) {
		t.Errorf("목차에 없는 글까지 목록에서 사라졌다:\n%s", body)
	}
}

// TestCategoryMovesReferenceVideoBelowPosts는 카테고리에 펼친 표지 글의 참고
// 동영상 절만 글 목록 뒤로 보내는지 본다. 상세 글의 본문 순서는 바꾸지 않는다.
func TestCategoryMovesReferenceVideoBelowPosts(t *testing.T) {
	h := testServer(t)

	page := get(t, h, "/dev/language").Body.String()
	body := mainOf(t, page)
	intro := strings.Index(body, "표지 본문")
	posts := strings.Index(body, `class="section-title">글</h2>`)
	reference := strings.Index(body, "참고 동영상")
	if intro < 0 || posts < 0 || reference < 0 {
		t.Fatalf("표지 본문, 글 목록, 참고 동영상 절이 모두 있어야 한다:\n%s", body)
	}
	if !(intro < posts && posts < reference) {
		t.Errorf("카테고리에서는 참고 동영상이 글 목록 뒤에 있어야 한다:\n%s", body)
	}
	if !strings.Contains(page, `src="/static/youtube.js"`) {
		t.Errorf("아래로 옮긴 동영상에 필요한 스크립트가 빠졌다:\n%s", page)
	}

	detail := mainOf(t, get(t, h, "/p/cover-language").Body.String())
	if strings.Index(detail, "표지 본문") > strings.Index(detail, "참고 동영상") {
		t.Errorf("상세 글의 원래 본문 순서가 바뀌었다:\n%s", detail)
	}
}

// TestMachineLearningCategoryIsListOnly는 머신러닝 첫 화면이 표지 본문을 펼치지
// 않고 `수리/통계: 이론`처럼 하위 분류 목록만 보여주는지 본다.
func TestMachineLearningCategoryIsListOnly(t *testing.T) {
	sqlDB := testDB(t)
	exec := execer(t, sqlDB)
	exec(`INSERT INTO categories (id, name, slug, sort_order) VALUES (1, '데이터 & 수리', 'data-math', 0)`)
	exec(`INSERT INTO categories (id, name, slug, parent_id, sort_order) VALUES
		(2, '머신러닝', '머신러닝', 1, 0),
		(3, '머신러닝: 기초이론', '머신러닝-기초이론', 2, 0),
		(4, '자연어처리', '자연어처리', 2, 1)`)
	now := time.Now().UTC()
	exec(`INSERT INTO posts (id, slug, title, body, status, source, category_id, sort_order, created_at, updated_at)
		VALUES (1, 'machine-cover', '머신러닝 & 딥러닝', '펼치지 않을 표지 본문', 'unlisted', 'notion', 2, 0, ?, ?)`, now, now)
	exec(`UPDATE categories SET cover_post_id = 1 WHERE id = 2`)

	srv, err := New(sqlDB)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	body := mainOf(t, get(t, srv.Handler(), "/data-math/머신러닝").Body.String())
	for _, want := range []string{
		`class="section-title">하위 분류</h2>`,
		`href="/data-math/` + url.PathEscape("머신러닝") + `/` + url.PathEscape("머신러닝-기초이론") + `"`,
		`href="/data-math/` + url.PathEscape("머신러닝") + `/` + url.PathEscape("자연어처리") + `"`,
	} {
		// html/template는 URL 문맥에서 새로 이스케이프한 조각의 16진수를
		// 소문자로 낼 수 있다. 퍼센트 인코딩은 대소문자를 구분하지 않는다.
		if !strings.Contains(strings.ToLower(body), strings.ToLower(want)) {
			t.Errorf("머신러닝 목록형 첫 화면에 %q가 없다:\n%s", want, body)
		}
	}
	for _, unwanted := range []string{"펼치지 않을 표지 본문", "이 글만 따로 보기", `class="section-title">글</h2>`} {
		if strings.Contains(body, unwanted) {
			t.Errorf("머신러닝 목록형 첫 화면에 %q가 남았다:\n%s", unwanted, body)
		}
	}
}
