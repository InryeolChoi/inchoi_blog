package admin

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/inryeol/blog"
	"github.com/inryeol/blog/internal/db"
	"github.com/inryeol/blog/internal/markdown"
)

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
	now := time.Now().UTC()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := sqlDB.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO categories (id, name, slug, sort_order) VALUES (1, '개발', 'dev', 0)`)
	// **노션에서 온 글에는 notion_page_id가 있다.** 이관은 언제나 그것을 넣는다.
	// 픽스처가 비워두면 "웹에서 쓴 글"(native)과 구별되지 않아서, 그 구별에
	// 기대는 것들(deploy/upload-guard.sql, 데이터 보기)을 확인할 수 없다.
	exec(`INSERT INTO posts (id, slug, title, body, status, source, notion_page_id,
	                         category_id, sort_order, created_at, updated_at)
	      VALUES (1, 'live-post', '보이는 글', '# 제목\n\n본문이다.', 'unlisted', 'notion',
	              'aaaaaaaa-0000-0000-0000-000000000001', 1, 0, ?, ?)`, now, now)
	exec(`INSERT INTO posts (id, slug, title, body, status, source, notion_page_id,
	                         category_id, sort_order, created_at, updated_at)
	      VALUES (2, 'draft-post', '숨긴 글', '아직 안 썼다.', 'draft', 'notion',
	              'aaaaaaaa-0000-0000-0000-000000000002', 1, 0, ?, ?)`, now, now)
	return sqlDB
}

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	s, err := New(testDB(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	return s.Handler()
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	// **브라우저 흉내를 낸다.** 진짜 브라우저는 POST·PUT에 Origin을 반드시
	// 붙이고 그 값은 스크립트가 못 바꾼다(csrf.go의 sameOrigin). 여기서 안
	// 붙이면 테스트가 브라우저가 아닌 것으로 보여 403에 걸린다.
	// httptest.NewRequest의 Host는 example.com이다.
	if !safeMethod(method) {
		r.Header.Set("Origin", "http://"+r.Host)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// admin의 목록에는 draft가 **나와야 한다.** 공개 쪽(web)은 반대로 draft를
// 어디에도 안 보여준다. 두 패키지를 갈라 둔 이유가 이 차이다.
func TestListShowsDrafts(t *testing.T) {
	rec := do(t, testHandler(t), http.MethodGet, "/api/admin/posts", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("상태 코드 %d: %s", rec.Code, rec.Body)
	}
	var got struct {
		Posts  []PostRow      `json:"posts"`
		Counts map[string]int `json:"counts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Posts) != 2 {
		t.Fatalf("글 %d편, 2편이어야 한다", len(got.Posts))
	}
	var sawDraft bool
	for _, p := range got.Posts {
		if p.Status == "draft" {
			sawDraft = true
		}
	}
	if !sawDraft {
		t.Error("admin 목록에 draft가 없다 — admin은 draft를 봐야 한다")
	}
	if got.Counts["draft"] != 1 || got.Counts["unlisted"] != 1 {
		t.Errorf("카운트가 이상하다: %v", got.Counts)
	}
	// status 세 값이 0이더라도 자리가 있어야 화면이 흔들리지 않는다.
	if _, ok := got.Counts["published"]; !ok {
		t.Error("published 자리가 없다")
	}
}

// 목록에 본문을 싣지 않는다. 1,356편의 본문을 다 실으면 목록 한 번이 수십 MB다.
func TestListDoesNotCarryBodies(t *testing.T) {
	rec := do(t, testHandler(t), http.MethodGet, "/api/admin/posts", "")
	if strings.Contains(rec.Body.String(), "본문이다") {
		t.Error("목록에 본문이 실려 있다")
	}
	if !strings.Contains(rec.Body.String(), `"bodyBytes"`) {
		t.Error("본문 길이조차 없다 — 빈 글을 목록에서 알아볼 수 없다")
	}
}

func TestGetPostCarriesBody(t *testing.T) {
	h := testHandler(t)
	rec := do(t, h, http.MethodGet, "/api/admin/posts/draft-post", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("draft를 못 가져왔다: %d %s", rec.Code, rec.Body)
	}
	var got PostDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Body != "아직 안 썼다." {
		t.Errorf("본문이 %q", got.Body)
	}
	if rec := do(t, h, http.MethodGet, "/api/admin/posts/없는-글", ""); rec.Code != http.StatusNotFound {
		t.Errorf("없는 글에 %d", rec.Code)
	}
}

// 미리보기가 공개 화면과 **같은 렌더러**를 써야 미리보기다. 여기서 본 것과
// 발행 뒤 화면이 다르면 이 기능은 없느니만 못하다.
func TestPreviewMatchesTheSiteRenderer(t *testing.T) {
	const src = "# 제목\n\n식은 $x_1$ 이다.\n\n```python\nprint(1)\n```\n"
	rec := do(t, testHandler(t), http.MethodPost, "/api/admin/preview",
		mustJSON(t, previewReq{Markdown: src}))
	if rec.Code != http.StatusOK {
		t.Fatalf("상태 코드 %d: %s", rec.Code, rec.Body)
	}
	var got previewResp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want, err := markdown.New().Render(src)
	if err != nil {
		t.Fatal(err)
	}
	if got.HTML != string(want) {
		t.Errorf("미리보기가 공개 렌더러와 다르다:\n미리보기: %s\n공개:     %s", got.HTML, want)
	}
	// 확장이 실제로 걸렸는지도 본다. 위 비교만으로는 둘 다 맨 goldmark일 수 있다.
	if !strings.Contains(got.HTML, `class="math`) {
		t.Error("수식이 .math로 안 나왔다")
	}
	if !strings.Contains(got.HTML, `class="lang"`) {
		t.Error("코드 블록에 언어 라벨이 없다")
	}
	// 본문 제목은 한 단계 내려간다. 페이지의 <h1>은 템플릿이 그리는 글 제목이다.
	if strings.Contains(got.HTML, "<h1") {
		t.Error("본문에 <h1>이 남아 있다")
	}
	if len(got.Outline) != 1 || got.Outline[0].Text != "제목" {
		t.Errorf("목차가 %v", got.Outline)
	}
}

// 저장·업로드 테스트는 save_test.go와 upload_test.go에 있다.
// (여기 있던 TestSaveIsNotImplementedYet / TestSaveDoesNotTouchTheDatabase는
//  3단계에서 실제 저장이 붙으면서 뒤집혔다.)

// API가 실패할 때 HTML을 돌려주면 fetch()가 파싱에서 터지고 진짜 원인이 가려진다.
func TestAPIErrorsAreJSON(t *testing.T) {
	h := testHandler(t)
	for _, path := range []string{"/api/admin/posts/없는-글", "/api/admin/그런거없음"} {
		rec := do(t, h, http.MethodGet, path, "")
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("%s: Content-Type=%q", path, ct)
		}
		var got map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Errorf("%s: JSON이 아니다: %s", path, rec.Body)
		} else if got["error"] == "" {
			t.Errorf("%s: error 자리가 비었다", path)
		}
	}
}

// 껍데기가 공개 페이지와 같은 스타일시트를 싣고, 검색엔진에 안 잡히게 해둔다.
func TestShellCarriesSiteStyles(t *testing.T) {
	rec := do(t, testHandler(t), http.MethodGet, "/admin", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("상태 코드 %d", rec.Code)
	}
	body := rec.Body.String()
	// sitecss에만 있는 토큰이다. 있으면 layout.html의 그 블록이 실려 나온 것이다.
	if !strings.Contains(body, "--measure") || !strings.Contains(body, "--rail") {
		t.Error("공개 스타일시트가 안 실렸다 — 미리보기가 실제 화면과 달라진다")
	}
	// CDN 태그는 layout.html이 정본이다. 해시가 어긋나면 브라우저가 조용히 거부한다.
	if !strings.Contains(body, "katex") || !strings.Contains(body, "integrity=") {
		t.Error("KaTeX 태그나 SRI 해시가 없다")
	}
	if !strings.Contains(body, "/static/math.js") || !strings.Contains(body, "/static/highlight-init.js") {
		t.Error("공개 페이지와 같은 수식·코드 스크립트를 안 쓴다")
	}
	if got := rec.Header().Get("X-Robots-Tag"); !strings.Contains(got, "noindex") {
		t.Errorf("X-Robots-Tag=%q — admin이 검색에 잡히면 안 된다", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control=%q", got)
	}
	// status 선택지는 코드가 정본이다. 화면에도 그 값이 그대로 가야 한다.
	for _, s := range Statuses {
		if !strings.Contains(body, s) {
			t.Errorf("status %q가 화면에 없다", s)
		}
	}
}

// CSR이라 /admin/edit/... 로 새로고침해도 같은 껍데기가 나와야 한다.
func TestDeepLinksGetTheSameShell(t *testing.T) {
	h := testHandler(t)
	for _, path := range []string{"/admin", "/admin/new", "/admin/edit/live-post"} {
		rec := do(t, h, http.MethodGet, path, "")
		if rec.Code != http.StatusOK {
			t.Errorf("%s: 상태 코드 %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `id="ad-root"`) {
			t.Errorf("%s: admin 껍데기가 아니다", path)
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// 상단 메뉴는 **서버가 그린다.** 스크립트가 못 떠도 세 화면 사이를 오갈 수
// 있어야 하고, admin.js는 지금 어디인지 표시만 덧입힌다. 여기서 JS로 만들면
// 그 점진적 향상이 사라진다.
func TestShellRendersTopMenu(t *testing.T) {
	body := do(t, testHandler(t), "GET", "/admin", "").Body.String()
	for _, want := range []string{
		`class="ad-menu"`,
		`href="/admin" data-menu="/admin"`,
		`href="/admin/data" data-menu="/admin/data"`,
		`href="/admin/settings" data-menu="/admin/settings"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("껍데기에 %q가 없다", want)
		}
	}
}

// 메뉴가 가리키는 세 주소가 전부 껍데기를 준다. 하나라도 404면 스크립트가
// 꺼진 브라우저에서 그 메뉴는 막다른 길이다.
func TestMenuPathsServeTheShell(t *testing.T) {
	h := testHandler(t)
	for _, path := range []string{"/admin", "/admin/data", "/admin/settings"} {
		rec := do(t, h, "GET", path, "")
		if rec.Code != http.StatusOK {
			t.Errorf("%s = %d, 200을 바랐다", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `id="ad-root"`) {
			t.Errorf("%s에 껍데기가 없다", path)
		}
	}
}
