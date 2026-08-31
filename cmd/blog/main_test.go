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
	"github.com/inryeol/blog/internal/admin"
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
	adm, err := admin.New(sqlDB, nil)
	if err != nil {
		t.Fatal(err)
	}
	withAdm := withAdmin(public, adm)
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
	adm, err := admin.New(sqlDB, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := withAdmin(srv.Handler(), adm)
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

// TestNoAuthAdminStaysOnLoopback은 이 파일에서 제일 중요한 테스트다.
//
// **인증 없는 글쓰기 화면이 밖에서 닿는 주소에 열리면 안 된다.** -admin-no-auth는
// 로컬에서 화면을 보라고 둔 문이고, loopback이 아니면 서버가 아예 안 떠야 한다.
func TestNoAuthAdminStaysOnLoopback(t *testing.T) {
	// 밖에서 닿는 주소들. **":8080"은 loopback이 아니다** — 모든 인터페이스에
	// 붙는다는 뜻이라 여기서 틀리면 그대로 공개된다.
	for _, addr := range []string{":8080", "0.0.0.0:8080", "35.230.119.252:80", "[::]:8080", "192.168.0.9:8080"} {
		if isLoopback(addr) {
			t.Errorf("isLoopback(%q)가 참이다. 밖에서 닿는 주소다", addr)
		}
		if _, err := adminAuth(addr, true); err == nil {
			t.Errorf("-admin-no-auth가 %q에서 통과했다. 거절해야 한다", addr)
		}
	}

	// 이 기계 밖에서 닿을 수 없는 주소들.
	for _, addr := range []string{"127.0.0.1:8080", "localhost:8080", "[::1]:8080", "127.0.0.2:9999"} {
		if !isLoopback(addr) {
			t.Errorf("isLoopback(%q)가 거짓이다. loopback이다", addr)
		}
		auth, err := adminAuth(addr, true)
		if err != nil {
			t.Errorf("-admin-no-auth가 %q에서 막혔다: %v", addr, err)
		}
		if auth != nil {
			t.Errorf("-admin-no-auth인데 인증 설정이 나왔다 (%q)", addr)
		}
	}
}

// TestAdminNeedsAuthConfig — 설정이 모자라면 뜨지 않는다. 반쯤 설정된 채로
// 뜨면 그게 곧 "인증이 있는 줄 알았는데 없는" 상태다.
func TestAdminNeedsAuthConfig(t *testing.T) {
	full := map[string]string{
		envClientID:     "id",
		envClientSecret: "secret",
		envLogins:       "InryeolChoi",
	}

	// 하나씩 빼면 하나씩 거절당한다.
	for missing := range full {
		t.Run("없음: "+missing, func(t *testing.T) {
			for k, v := range full {
				if k == missing {
					t.Setenv(k, "")
					continue
				}
				t.Setenv(k, v)
			}
			if _, err := adminAuth("0.0.0.0:80", false); err == nil {
				t.Fatalf("%s가 없는데 통과했다", missing)
			}
		})
	}

	t.Run("다 있으면 통과", func(t *testing.T) {
		for k, v := range full {
			t.Setenv(k, v)
		}
		auth, err := adminAuth("0.0.0.0:80", false)
		if err != nil {
			t.Fatalf("설정이 다 있는데 막혔다: %v", err)
		}
		if len(auth.AllowedLogins) != 1 || auth.AllowedLogins[0] != "InryeolChoi" {
			t.Fatalf("허용 목록이 %v다", auth.AllowedLogins)
		}
	})

	// **설정과 -admin-no-auth를 같이 주면 거절한다.** 사람이 무엇을 원하는지
	// 알 수 없는 상태고, 조용히 인증을 끄는 쪽으로 고르면 그게 사고다.
	t.Run("설정과 -admin-no-auth를 같이 줌", func(t *testing.T) {
		for k, v := range full {
			t.Setenv(k, v)
		}
		if _, err := adminAuth("127.0.0.1:8080", true); err == nil {
			t.Fatal("둘 다 줬는데 통과했다")
		}
	})

	// 짧은 세션 키는 HMAC을 무르게 만든다.
	t.Run("세션 키가 짧다", func(t *testing.T) {
		for k, v := range full {
			t.Setenv(k, v)
		}
		t.Setenv(envSessionKey, "짧다")
		if _, err := adminAuth("127.0.0.1:8080", false); err == nil {
			t.Fatal("짧은 세션 키가 통과했다")
		}
	})
}
