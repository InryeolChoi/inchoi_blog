package admin

// 여기서 지키는 것은 하나다: **허용한 계정 말고는 아무도 admin에 못 들어온다.**
//
// 가짜 GitHub을 httptest로 세워서 **네트워크 없이** 전체 흐름을 돈다. 진짜
// GitHub을 부르면 테스트가 남의 서비스와 인터넷 사정에 매달리게 되고, 무엇보다
// "거절당하는 계정" 같은 경우를 만들 수가 없다.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testLogin = "InryeolChoi"

// fakeGitHub은 토큰 교환과 사용자 조회에 답한다. login을 바꿔가며 "남의 계정"을
// 흉내 낼 수 있다.
func fakeGitHub(t *testing.T, login string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.Form.Get("code") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"bearer"}`))
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"login":` + quote(login) + `}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func quote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"` }

// authHandler는 인증을 켠 admin 핸들러를 만든다. gh는 가짜 GitHub이다.
func authHandler(t *testing.T, gh *httptest.Server, allowed ...string) (http.Handler, *Server) {
	t.Helper()
	if len(allowed) == 0 {
		allowed = []string{testLogin}
	}
	s, err := New(testDB(t), &AuthConfig{
		ClientID:      "id",
		ClientSecret:  "secret",
		AllowedLogins: allowed,
		SessionKey:    []byte("0123456789abcdef0123456789abcdef"),
		AuthorizeURL:  gh.URL + "/authorize",
		TokenURL:      gh.URL + "/token",
		UserURL:       gh.URL + "/user",
	})
	if err != nil {
		t.Fatal(err)
	}
	return s.Handler(), s
}

// login은 OAuth 흐름을 끝까지 돌고 세션 쿠키를 돌려준다. 못 받으면 nil이다.
func login(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()

	start := httptest.NewRecorder()
	h.ServeHTTP(start, httptest.NewRequest(http.MethodGet, "/admin/auth/start", nil))
	if start.Code != http.StatusSeeOther {
		t.Fatalf("auth/start가 %d를 줬다 (303이어야 한다)", start.Code)
	}
	state := cookieByName(start.Result().Cookies(), stateCookie)
	if state == nil {
		t.Fatal("auth/start가 state 쿠키를 안 줬다")
	}

	// GitHub이 돌아온 척한다.
	r := httptest.NewRequest(http.MethodGet, "/admin/auth/callback?code=ok&state="+state.Value, nil)
	r.AddCookie(state)
	cb := httptest.NewRecorder()
	h.ServeHTTP(cb, r)
	return cookieByName(cb.Result().Cookies(), sessionCookie)
}

func cookieByName(cs []*http.Cookie, name string) *http.Cookie {
	for _, c := range cs {
		if c.Name == name && c.MaxAge >= 0 && c.Value != "" {
			return c
		}
	}
	return nil
}

// get은 (있으면) 세션 쿠키를 달고 한 번 찌른다.
func get(t *testing.T, h http.Handler, path string, session *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	if session != nil {
		r.AddCookie(session)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// TestLockedOutWithoutSession은 이 파일에서 제일 중요한 테스트다. 로그인하지
// 않은 사람에게는 화면도 API도 열리지 않아야 한다.
func TestLockedOutWithoutSession(t *testing.T) {
	h, _ := authHandler(t, fakeGitHub(t, testLogin))

	for _, path := range []string{"/admin", "/admin/new", "/admin/edit/live-post", "/admin/static/admin.js"} {
		w := get(t, h, path, nil)
		if w.Code != http.StatusSeeOther {
			t.Errorf("%s가 %d를 줬다. 로그인 화면으로 보내야 한다", path, w.Code)
		}
		if loc := w.Header().Get("Location"); loc != "/admin/login" {
			t.Errorf("%s가 %q로 보냈다. /admin/login이어야 한다", path, loc)
		}
	}

	// **API는 HTML이 아니라 JSON 401을 준다.** fetch()가 받는 자리라
	// 로그인 화면이 오면 파싱에서 터지고 진짜 이유가 가려진다.
	for _, path := range []string{"/api/admin/posts", "/api/admin/posts/live-post"} {
		w := get(t, h, path, nil)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s가 %d를 줬다. 401이어야 한다", path, w.Code)
		}
		if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("%s의 Content-Type이 %q다. JSON이어야 한다", path, ct)
		}
	}

	// 본문을 바꾸는 쪽도 마찬가지다.
	for _, m := range []struct{ method, path string }{
		{http.MethodPost, "/api/admin/preview"},
		{http.MethodPost, "/api/admin/posts"},
		{http.MethodPut, "/api/admin/posts/live-post"},
		{http.MethodPost, "/api/admin/images"},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(m.method, m.path, strings.NewReader("{}")))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s가 %d를 줬다. 401이어야 한다", m.method, m.path, w.Code)
		}
	}
}

func TestLoginPageIsOpen(t *testing.T) {
	h, _ := authHandler(t, fakeGitHub(t, testLogin))
	w := get(t, h, "/admin/login", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("로그인 화면이 %d다", w.Code)
	}
	if !strings.Contains(w.Body.String(), "/admin/auth/start") {
		t.Error("로그인 화면에 GitHub으로 가는 길이 없다")
	}
	if w.Header().Get("X-Robots-Tag") == "" {
		t.Error("로그인 화면에 X-Robots-Tag가 없다")
	}
}

func TestAllowedLoginGetsIn(t *testing.T) {
	h, _ := authHandler(t, fakeGitHub(t, testLogin))

	session := login(t, h)
	if session == nil {
		t.Fatal("허용한 계정인데 세션을 못 받았다")
	}
	if !session.HttpOnly {
		t.Error("세션 쿠키에 HttpOnly가 없다")
	}
	if session.SameSite != http.SameSiteLaxMode {
		t.Error("세션 쿠키의 SameSite가 Lax가 아니다")
	}

	w := get(t, h, "/admin", session)
	if w.Code != http.StatusOK {
		t.Fatalf("로그인했는데 /admin이 %d다", w.Code)
	}
	// 화면이 누가 들어와 있는지 보여준다.
	if !strings.Contains(w.Body.String(), testLogin) {
		t.Error("화면에 로그인한 계정이 안 보인다")
	}

	if w := get(t, h, "/api/admin/posts", session); w.Code != http.StatusOK {
		t.Errorf("로그인했는데 API가 %d다", w.Code)
	}
}

// TestOtherAccountsAreRefused는 화이트리스트가 실제로 관문인지 본다.
//
// **GitHub 로그인 자체는 성공한 상황이다.** 계정은 누구나 몇 초면 만들 수
// 있으므로, 여기서 통과시키면 인증이 없는 것과 같다.
func TestOtherAccountsAreRefused(t *testing.T) {
	h, _ := authHandler(t, fakeGitHub(t, "someone-else"), testLogin)

	start := httptest.NewRecorder()
	h.ServeHTTP(start, httptest.NewRequest(http.MethodGet, "/admin/auth/start", nil))
	state := cookieByName(start.Result().Cookies(), stateCookie)
	if state == nil {
		t.Fatal("state 쿠키가 없다")
	}

	r := httptest.NewRequest(http.MethodGet, "/admin/auth/callback?code=ok&state="+state.Value, nil)
	r.AddCookie(state)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("남의 계정에 %d를 줬다. 403이어야 한다", w.Code)
	}
	if c := cookieByName(w.Result().Cookies(), sessionCookie); c != nil {
		t.Fatal("**남의 계정에 세션을 줬다.** 화이트리스트가 안 걸렸다")
	}
	// 화면에 계정 이름을 도로 찍지 않는다.
	if strings.Contains(w.Body.String(), "someone-else") {
		t.Error("거절 화면에 남이 지은 계정 이름이 그대로 나온다")
	}
}

// TestStateMismatchIsRefused — state가 없거나 다르면 로그인이 안 된다.
// 이게 없으면 남이 자기 code로 우리 callback을 부르게 해서, 브라우저 주인이
// 모르는 사이에 남의 계정으로 로그인시킬 수 있다.
func TestStateMismatchIsRefused(t *testing.T) {
	h, _ := authHandler(t, fakeGitHub(t, testLogin))

	start := httptest.NewRecorder()
	h.ServeHTTP(start, httptest.NewRequest(http.MethodGet, "/admin/auth/start", nil))
	state := cookieByName(start.Result().Cookies(), stateCookie)

	cases := []struct {
		name   string
		query  string
		cookie *http.Cookie
	}{
		{"state 쿠키가 없다", "?code=ok&state=" + state.Value, nil},
		{"state가 다르다", "?code=ok&state=deadbeef", state},
		{"state가 비었다", "?code=ok", state},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/admin/auth/callback"+c.query, nil)
			if c.cookie != nil {
				r.AddCookie(c.cookie)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest {
				t.Errorf("%d를 줬다. 400이어야 한다", w.Code)
			}
			if got := cookieByName(w.Result().Cookies(), sessionCookie); got != nil {
				t.Fatal("state가 안 맞는데 세션을 줬다")
			}
		})
	}
}

// TestForgedSessionIsRefused — 서명을 못 맞추면 쿠키를 지어낼 수 없다.
func TestForgedSessionIsRefused(t *testing.T) {
	h, s := authHandler(t, fakeGitHub(t, testLogin))
	good := login(t, h)
	if good == nil {
		t.Fatal("세션을 못 받았다")
	}

	other := &authenticator{cfg: s.auth.cfg, key: []byte("전혀 다른 키다 전혀 다른 키다 전혀")}

	cases := map[string]string{
		"서명이 없다":     "aW5yeWVvbA",
		"서명이 딴 키다":   other.sign(testLogin, time.Now().Add(time.Hour)),
		"본문을 고쳤다":    "YWRtaW58OTk5OTk5OTk5OQ." + strings.SplitN(good.Value, ".", 2)[1],
		"만료가 지났다":    s.auth.sign(testLogin, time.Now().Add(-time.Minute)),
		"허용 목록 밖 계정": s.auth.sign("someone-else", time.Now().Add(time.Hour)),
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			w := get(t, h, "/admin", &http.Cookie{Name: sessionCookie, Value: value})
			if w.Code != http.StatusSeeOther {
				t.Fatalf("%d를 줬다. 로그인 화면으로 보내야 한다", w.Code)
			}
		})
	}
}

// TestLogoutClearsTheSession — 나가면 정말 나가진다.
func TestLogoutClearsTheSession(t *testing.T) {
	h, _ := authHandler(t, fakeGitHub(t, testLogin))
	session := login(t, h)
	if session == nil {
		t.Fatal("세션을 못 받았다")
	}

	r := httptest.NewRequest(http.MethodPost, "/admin/logout", nil)
	r.AddCookie(session)
	// 로그아웃도 POST라 sameOrigin을 지난다. 브라우저는 폼 전송에도 Origin을 붙인다.
	r.Header.Set("Origin", "http://"+r.Host)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("로그아웃이 %d다", w.Code)
	}

	var cleared bool
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("로그아웃이 세션 쿠키를 안 지웠다")
	}
}

// TestLogoutIsNotAGet — GET이면 남의 페이지에 박아둔 <img> 하나로 사람을
// 로그아웃시킬 수 있다.
func TestLogoutIsNotAGet(t *testing.T) {
	h, _ := authHandler(t, fakeGitHub(t, testLogin))
	session := login(t, h)
	w := get(t, h, "/admin/logout", session)
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie && c.MaxAge < 0 {
			t.Fatal("GET /admin/logout이 세션을 지웠다")
		}
	}
}

// TestBrokenAuthConfigRefusesToStart — 반쯤 설정된 채로 뜨면 그게 곧
// "인증이 있는 줄 알았는데 없는" 상태다.
func TestBrokenAuthConfigRefusesToStart(t *testing.T) {
	cases := map[string]AuthConfig{
		"client id가 없다": {ClientSecret: "s", AllowedLogins: []string{"a"}},
		"secret이 없다":    {ClientID: "i", AllowedLogins: []string{"a"}},
		"허용 계정이 없다":     {ClientID: "i", ClientSecret: "s"},
		"허용 계정이 공백뿐이다":  {ClientID: "i", ClientSecret: "s", AllowedLogins: []string{"  ", ""}},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := New(testDB(t), &cfg); err == nil {
				t.Fatal("모자란 설정으로 서버가 만들어졌다")
			}
		})
	}
}

// TestLoginIsCaseInsensitive — GitHub의 login은 대소문자를 구별하지 않는다.
func TestLoginIsCaseInsensitive(t *testing.T) {
	h, _ := authHandler(t, fakeGitHub(t, "inryeolchoi"), "InryeolChoi")
	if login(t, h) == nil {
		t.Fatal("대소문자만 다른데 거절했다")
	}
}
