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

// testServer는 작은 트리를 넣은 서버를 만든다.
//
//	개발(dev) > 언어(language) > 파이썬(python) > 글 "리스트"
//	                                            표지 글 "언어"는 language에 붙는다
func testServer(t *testing.T) http.Handler {
	t.Helper()

	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if _, err := db.Migrate(sqlDB, blog.MigrationsFS()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	exec := func(q string, args ...any) sql.Result {
		t.Helper()
		res, err := sqlDB.Exec(q, args...)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return res
	}

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
	post(1, "cover-language", "언어", "표지 본문", 2)
	post(2, "list-post", "리스트", "# 리스트\n\n식은 $x_1 + y_2$ 이다.\n\n$$\n\\sum_{i=1}^{n} i\n$$\n", 3)
	exec(`UPDATE categories SET cover_post_id = 1 WHERE id = 2`)

	exec(`INSERT INTO images (sha256, data, mime, created_at) VALUES (?, ?, 'image/png', ?)`,
		strings.Repeat("ab", 32), []byte{0x89, 0x50, 0x4e, 0x47}, now)

	srv, err := New(sqlDB)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv.Handler()
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
	body := rec.Body.String()
	if !strings.Contains(body, `href="/dev"`) {
		t.Errorf("최상위 카테고리 링크가 없다:\n%s", body)
	}
	// 하위 글까지 세야 한다 (dev 아래 글 2건)
	if !strings.Contains(body, "글 2건") {
		t.Errorf("하위 글 수가 안 세졌다:\n%s", body)
	}
}

func TestCategoryPageShowsChildrenAndPosts(t *testing.T) {
	h := testServer(t)

	rec := get(t, h, "/dev")
	if rec.Code != http.StatusOK {
		t.Fatalf("/dev 상태 코드 %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `href="/dev/language"`) {
		t.Errorf("하위 분류 링크가 없다:\n%s", rec.Body.String())
	}

	rec = get(t, h, "/dev/language")
	if rec.Code != http.StatusOK {
		t.Fatalf("/dev/language 상태 코드 %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `href="/dev/language/python"`) {
		t.Errorf("3단계 링크가 없다:\n%s", body)
	}
	if !strings.Contains(body, `href="/p/cover-language"`) {
		t.Errorf("직속 글 링크가 없다:\n%s", body)
	}
	if !strings.Contains(body, "표지") {
		t.Errorf("표지 표시가 없다:\n%s", body)
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
