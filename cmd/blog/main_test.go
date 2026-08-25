package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/inryeol/blog"
	"github.com/inryeol/blog/internal/db"
	"github.com/inryeol/blog/internal/web"
)

// 타임아웃이 0이면 "무제한"이다. 공개 주소에서 무제한은 느린 연결 몇 개로
// 서버가 멎는다는 뜻이라, 실수로 지워지지 않게 여기서 지킨다.
func TestHTTPServerHasEveryTimeout(t *testing.T) {
	srv := httpServer("127.0.0.1:0", http.NotFoundHandler())

	checks := []struct {
		name string
		zero bool
	}{
		{"ReadHeaderTimeout", srv.ReadHeaderTimeout == 0},
		{"ReadTimeout", srv.ReadTimeout == 0},
		{"WriteTimeout", srv.WriteTimeout == 0},
		{"IdleTimeout", srv.IdleTimeout == 0},
	}
	for _, c := range checks {
		if c.zero {
			t.Errorf("%s가 0이다 — 무제한이라는 뜻이다", c.name)
		}
	}
	if srv.MaxHeaderBytes == 0 {
		t.Error("MaxHeaderBytes가 기본값이다")
	}
	// 가장 큰 응답이 3.4MB짜리 이미지 BLOB이다. 쓰기 타임아웃을 읽기보다
	// 짧게 잡으면 느린 회선에서 그림이 잘린다.
	if srv.WriteTimeout <= srv.ReadTimeout {
		t.Errorf("WriteTimeout(%v)이 ReadTimeout(%v)보다 짧다", srv.WriteTimeout, srv.ReadTimeout)
	}
	if srv.ReadHeaderTimeout > srv.ReadTimeout {
		t.Errorf("ReadHeaderTimeout(%v)이 ReadTimeout(%v)보다 길다", srv.ReadHeaderTimeout, srv.ReadTimeout)
	}
}

// **admin은 기본으로 꺼져 있어야 한다.** 인증이 아직 없어서(로드맵 2단계),
// 켜진 채로 배포되면 아무나 글을 고칠 수 있는 화면이 공개 주소에 열린다.
//
// -drafts와 같은 원칙이다: 켜고 끄는 것을 실수해도 새는 방향이 아니라 막는
// 방향으로 틀리게 둔다. 그래서 여기서는 **안 붙였을 때 정말 없는지**를 본다.
func TestAdminIsOffUnlessAskedFor(t *testing.T) {
	sqlDB := testDB(t)

	srv, err := web.New(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	public := srv.Handler()

	for _, path := range []string{"/admin", "/admin/new", "/api/admin/posts"} {
		rec := httptest.NewRecorder()
		public.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: 상태 코드 %d — admin을 안 켰는데 응답이 있다", path, rec.Code)
		}
		if strings.Contains(rec.Body.String(), `id="ad-root"`) {
			t.Errorf("%s: admin 화면이 나왔다", path)
		}
	}

	// -admin을 줬을 때는 반대로 실제로 붙어야 한다. 안 그러면 위 검사는
	// 아무것도 확인하지 않는 셈이 된다.
	withAdm, err := withAdmin(public, sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	withAdm.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `id="ad-root"`) {
		t.Errorf("-admin을 줬는데 화면이 없다: %d", rec.Code)
	}
}

// admin을 붙여도 공개 라우트는 그대로여야 한다. 바깥 mux가 "/"로 받은 것을
// 공개 핸들러에 그대로 넘기는지 본다.
func TestPublicRoutesSurviveTheAdminMux(t *testing.T) {
	sqlDB := testDB(t)
	srv, err := web.New(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	h, err := withAdmin(srv.Handler(), sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]int{
		"/":                http.StatusOK,
		"/p/live-post":     http.StatusOK,
		"/없는-분류":           http.StatusNotFound,
		"/admin":           http.StatusOK,
		"/api/admin/그런거없음": http.StatusNotFound,
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want {
			t.Errorf("%s: 상태 코드 %d, %d여야 한다", path, rec.Code, want)
		}
	}
}

// testDB는 마이그레이션을 건 빈 DB에 글 하나를 넣어 돌려준다.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if _, err := db.Migrate(sqlDB, blog.MigrationsFS()); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := sqlDB.Exec(`INSERT INTO posts (id, slug, title, body, status, source, sort_order, created_at, updated_at)
	      VALUES (1, 'live-post', '보이는 글', '본문', 'unlisted', 'notion', 0, ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}
	return sqlDB
}
