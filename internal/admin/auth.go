package admin

// 로드맵 2단계: GitHub OAuth. 허용한 GitHub 계정 하나(또는 몇 개)만 admin에
// 들어올 수 있다.
//
// # 왜 GitHub OAuth인가
//
// 비밀번호를 우리가 들고 있지 않아도 된다. 저장할 것은 "누가 들어왔나"뿐이고,
// 그 판정은 GitHub이 한다. 이 프로젝트에는 사용자 테이블도 비밀번호 해시도
// 없고, 앞으로도 만들 이유가 없다.
//
// # 화이트리스트가 진짜 관문이다
//
// **OAuth는 "누구인지"만 알려준다. "들어와도 되는지"는 우리가 정한다.**
// GitHub 계정은 누구나 몇 초면 만들 수 있으므로, 로그인에 성공했다는 것만으로
// 통과시키면 사실상 인증이 없는 것과 같다. allowed에 적힌 login과 정확히
// 맞을 때만 세션을 준다. 못 맞으면 그 자리에서 끝이고 세션 쿠키는 안 나간다.
//
// # 세션은 서명한 쿠키 하나다
//
// 세션 테이블을 만들지 않는다. 마이그레이션이 하나 늘고, 지우는 마이그레이션을
// 안 만든다는 원칙 때문에 나중에 물리기도 어렵다. 대신 `login|만료`를 HMAC으로
// 서명해서 쿠키에 담는다. 서버가 키를 들고 있으므로 위조할 수 없다.
//
//   - **키가 바뀌면 세션이 전부 끊긴다.** 그래서 BLOG_SESSION_KEY를 안 주면
//     시작할 때 무작위로 만든다 — 재시작하면 다시 로그인해야 하지만, 키를
//     저장소나 유닛 파일에 적어두는 것보다 낫다.
//   - 취소(로그아웃)는 쿠키를 지우는 것이다. 훔쳐간 쿠키를 서버에서 끊을 수는
//     없으므로 만료를 짧게 둔다.

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// 진짜 GitHub 주소. AuthConfig에서 비워두면 이걸 쓴다. 테스트는 여기에
// httptest 서버 주소를 넣어서 **네트워크 없이** 전체 흐름을 돈다.
const (
	githubAuthorizeURL = "https://github.com/login/oauth/authorize"
	githubTokenURL     = "https://github.com/login/oauth/access_token"
	githubUserURL      = "https://api.github.com/user"
)

const (
	sessionCookie = "blog_admin_session"
	stateCookie   = "blog_admin_state"
	// state 쿠키는 /admin 안에서만 쓴다. 지울 때도 이 path를 그대로 줘야 한다.
	stateCookiePath = "/admin"

	// 세션 수명. 훔쳐간 쿠키를 서버에서 끊을 방법이 없으므로(세션 테이블이
	// 없다) 길게 두지 않는다. 글을 쓰다 말고 쫓겨나지 않을 만큼은 된다.
	sessionTTL = 12 * time.Hour

	// state는 GitHub에 갔다 오는 동안만 산다.
	stateTTL = 10 * time.Minute
)

// AuthConfig는 사람이 밖에서 넣어주는 값이다. cmd/blog가 환경변수에서 읽는다.
type AuthConfig struct {
	ClientID     string
	ClientSecret string

	// AllowedLogins는 들어올 수 있는 GitHub login이다. **비어 있으면 아무도
	// 못 들어온다** — 설정을 빠뜨렸을 때 모두 허용으로 열리는 것이 이 코드에서
	// 제일 위험한 실수라, 빈 목록은 "전부 막는다"로 읽는다.
	AllowedLogins []string

	// SessionKey는 세션 서명 키다. 비면 시작할 때 무작위로 만든다.
	SessionKey []byte

	// 아래 셋은 테스트에서 가짜 GitHub을 세우려고 열어둔 것이다. 비면 진짜 주소.
	AuthorizeURL string
	TokenURL     string
	UserURL      string
}

type authenticator struct {
	cfg    AuthConfig
	key    []byte
	client *http.Client
	login  *template.Template
	css    template.CSS
}

// newAuthenticator는 설정을 확인하고 인증기를 만든다.
//
// **모자란 설정으로는 만들어지지 않는다.** 반쯤 설정된 채로 뜨면 그게 곧
// "인증이 있는 줄 알았는데 없는" 상태다.
func newAuthenticator(cfg AuthConfig, css template.CSS) (*authenticator, error) {
	if strings.TrimSpace(cfg.ClientID) == "" {
		return nil, errors.New("GitHub client id가 비어 있다")
	}
	if strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, errors.New("GitHub client secret이 비어 있다")
	}
	allowed := make([]string, 0, len(cfg.AllowedLogins))
	for _, l := range cfg.AllowedLogins {
		if l = strings.TrimSpace(l); l != "" {
			allowed = append(allowed, l)
		}
	}
	if len(allowed) == 0 {
		return nil, errors.New("허용할 GitHub 계정이 하나도 없다 (BLOG_ADMIN_LOGINS)")
	}
	cfg.AllowedLogins = allowed

	key := cfg.SessionKey
	if len(key) == 0 {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("세션 키 생성: %w", err)
		}
		log.Print("admin: 세션 키를 무작위로 만들었다 — 서버를 재시작하면 다시 로그인해야 한다 (BLOG_SESSION_KEY로 고정할 수 있다)")
	}

	if cfg.AuthorizeURL == "" {
		cfg.AuthorizeURL = githubAuthorizeURL
	}
	if cfg.TokenURL == "" {
		cfg.TokenURL = githubTokenURL
	}
	if cfg.UserURL == "" {
		cfg.UserURL = githubUserURL
	}

	tpl, err := template.ParseFS(templateFS, "templates/login.html")
	if err != nil {
		return nil, fmt.Errorf("로그인 템플릿 파싱: %w", err)
	}

	return &authenticator{
		cfg: cfg,
		key: key,
		// **반드시 시간 제한을 둔다.** 기본 http.Client는 무제한이라 GitHub이
		// 응답을 안 주면 그 요청이 영원히 고루틴을 물고 있는다.
		client: &http.Client{Timeout: 10 * time.Second},
		login:  tpl,
		css:    css,
	}, nil
}

// allows는 GitHub이 알려준 login이 화이트리스트에 있는지 본다.
//
// GitHub의 login은 대소문자를 구별하지 않으므로 접어서 견준다. 사람이 설정에
// `InryeolChoi`라고 적고 GitHub이 `inryeolchoi`를 돌려줘도 같은 사람이다.
func (a *authenticator) allows(login string) bool {
	if login == "" {
		return false
	}
	for _, l := range a.cfg.AllowedLogins {
		if strings.EqualFold(l, login) {
			return true
		}
	}
	return false
}

// --- 세션 쿠키 ---------------------------------------------------------

// sign은 `login|만료`를 서명해서 쿠키에 담을 문자열로 만든다.
func (a *authenticator) sign(login string, exp time.Time) string {
	payload := login + "|" + strconv.FormatInt(exp.Unix(), 10)
	mac := hmac.New(sha256.New, a.key)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." +
		base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// verify는 쿠키 값에서 login을 꺼낸다. 서명이 틀렸거나 만료됐으면 빈 문자열이다.
func (a *authenticator) verify(value string) string {
	body, sig, ok := strings.Cut(value, ".")
	if !ok {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return ""
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return ""
	}
	mac := hmac.New(sha256.New, a.key)
	mac.Write(payload)
	// **hmac.Equal을 쓴다.** bytes.Equal은 다른 바이트를 만나면 바로 끝나서
	// 걸린 시간이 "몇 글자까지 맞았나"를 알려준다.
	if !hmac.Equal(got, mac.Sum(nil)) {
		return ""
	}

	login, expStr, ok := strings.Cut(string(payload), "|")
	if !ok {
		return ""
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().After(time.Unix(exp, 0)) {
		return ""
	}

	// **서명이 맞아도 화이트리스트를 다시 본다.** 설정에서 계정을 빼면 아직
	// 살아 있는 쿠키도 그 순간부터 안 통해야 한다. 세션 테이블이 없어서
	// 취소할 방법이 이것뿐이다.
	if !a.allows(login) {
		return ""
	}
	return login
}

// currentLogin은 요청에 붙은 세션에서 login을 꺼낸다. 없으면 빈 문자열.
func (a *authenticator) currentLogin(r *http.Request) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return a.verify(c.Value)
}

// secure는 이 요청이 HTTPS로 왔는지 본다.
//
// **무조건 Secure를 붙이면 안 된다.** 로컬(http://127.0.0.1:8080)에서 붙이는
// 순간 브라우저가 쿠키를 아예 안 보낸다 — 로그인이 되는데 로그인이 안 되는
// 상태가 된다. 대신 HTTPS면 반드시 붙인다.
//
// 배포에서는 Caddy가 앞에 서서 X-Forwarded-Proto: https를 붙여준다
// (deploy/Caddyfile). blog 자신은 127.0.0.1:8080에서 평문을 듣고 있으므로
// r.TLS는 언제나 nil이다 — **이 헤더가 유일한 근거다.** 프록시를 바꿀 일이
// 생기면 그것도 이 헤더를 붙이는지 먼저 본다.
func secure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (a *authenticator) setSession(w http.ResponseWriter, r *http.Request, login string) {
	exp := time.Now().Add(sessionTTL)
	http.SetCookie(w, &http.Cookie{
		Name:  sessionCookie,
		Value: a.sign(login, exp),
		Path:  "/",
		// JS가 못 읽게 한다. 본문 렌더러가 html.WithUnsafe()라 본문에 들어온
		// 스크립트가 실행될 수 있는데, 그때 세션까지 가져가면 안 된다.
		HttpOnly: true,
		Secure:   secure(r),
		// Lax면 남의 사이트에서 온 POST에는 안 실린다. 우리 쪽 로그인
		// 리다이렉트(GET)에는 실리므로 흐름이 안 깨진다.
		SameSite: http.SameSiteLaxMode,
		Expires:  exp,
	})
}

// clearCookie는 쿠키를 지운다.
//
// **path를 심을 때와 똑같이 줘야 한다.** 브라우저는 (이름, 도메인, path)로
// 쿠키를 가리므로, path가 다르면 지워지지 않고 같은 이름이 두 개가 된다.
func clearCookie(w http.ResponseWriter, r *http.Request, name, path string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     path,
		HttpOnly: true,
		Secure:   secure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// --- 관문 -------------------------------------------------------------

// openPaths는 로그인하지 않아도 되는 곳이다. **여기 없는 /admin·/api/admin은
// 전부 막힌다** — 새 라우트를 더할 때 따로 챙기지 않아도 기본이 "막는다"이다.
var openPaths = map[string]bool{
	"/admin/login":         true,
	"/admin/auth/start":    true,
	"/admin/auth/callback": true,
}

// guard는 세션이 없는 요청을 돌려보낸다.
//
// **mux보다 바깥에 둔다.** 핸들러마다 챙기게 하면 새 API를 더할 때마다 구멍이
// 하나씩 생긴다 — web이 draft 판정을 store 한 곳에 모은 것과 같은 이유다.
func (a *authenticator) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if openPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		if a.currentLogin(r) != "" {
			next.ServeHTTP(w, r)
			return
		}
		// **API에는 HTML을 주지 않는다.** fetch()가 받는 자리라 로그인 화면이
		// 오면 JSON 파싱에서 터지고 "로그인이 풀렸다"는 진짜 이유가 가려진다.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeErr(w, http.StatusUnauthorized, "로그인이 필요하다")
			return
		}
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
	})
}

// --- OAuth 흐름 -------------------------------------------------------

type loginData struct {
	SiteCSS template.CSS
	Error   string
}

func (a *authenticator) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	// 이미 들어와 있으면 로그인 화면을 다시 보여줄 이유가 없다.
	if a.currentLogin(r) != "" {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	a.renderLogin(w, http.StatusOK, "")
}

func (a *authenticator) renderLogin(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.WriteHeader(code)
	if err := a.login.ExecuteTemplate(w, "login", loginData{SiteCSS: a.css, Error: msg}); err != nil {
		log.Printf("admin 로그인 화면 렌더 실패: %v", err)
	}
}

// handleStart는 state를 만들어 쿠키에 심고 GitHub으로 보낸다.
//
// state는 CSRF 방지다. 이게 없으면 남이 자기 code로 우리 callback을 부르게
// 만들어서, 브라우저 주인이 모르는 사이에 **남의 계정으로 로그인시킬** 수 있다.
func (a *authenticator) handleStart(w http.ResponseWriter, r *http.Request) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		log.Printf("admin state 생성 실패: %v", err)
		a.renderLogin(w, http.StatusInternalServerError, "로그인을 시작하지 못했다")
		return
	}
	state := hex.EncodeToString(raw)

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookie,
		Value:    state,
		Path:     stateCookiePath,
		HttpOnly: true,
		Secure:   secure(r),
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(stateTTL),
	})

	q := url.Values{}
	q.Set("client_id", a.cfg.ClientID)
	q.Set("state", state)
	// **scope를 비워둔다.** 우리가 알아야 할 것은 login 하나뿐이다. 저장소나
	// 이메일 권한을 달라고 할 이유가 없다.
	q.Set("scope", "")
	// redirect_uri를 안 보낸다 — GitHub이 OAuth 앱에 등록된 주소를 쓴다.
	// 여기서 만들어 보내면 등록된 것과 어긋나 흐름이 깨지기 쉽고, 우리가
	// 리다이렉트 주소를 조립하는 자리를 만드는 것 자체가 위험하다.
	http.Redirect(w, r, a.cfg.AuthorizeURL+"?"+q.Encode(), http.StatusSeeOther)
}

func (a *authenticator) handleCallback(w http.ResponseWriter, r *http.Request) {
	// GitHub이 거절당했다고 알려주는 경우(사용자가 취소 등).
	if e := r.URL.Query().Get("error"); e != "" {
		a.renderLogin(w, http.StatusForbidden, "GitHub이 거절했다: "+e)
		return
	}

	want, err := r.Cookie(stateCookie)
	got := r.URL.Query().Get("state")
	// 한 번 쓰면 버린다.
	clearCookie(w, r, stateCookie, stateCookiePath)
	if err != nil || want.Value == "" || got == "" ||
		subtle.ConstantTimeCompare([]byte(want.Value), []byte(got)) != 1 {
		a.renderLogin(w, http.StatusBadRequest, "로그인 흐름이 어긋났다. 다시 시도해라.")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		a.renderLogin(w, http.StatusBadRequest, "GitHub이 코드를 주지 않았다")
		return
	}

	token, err := a.exchange(code)
	if err != nil {
		log.Printf("admin 토큰 교환 실패: %v", err)
		a.renderLogin(w, http.StatusBadGateway, "GitHub과 이야기하지 못했다")
		return
	}
	login, err := a.fetchLogin(token)
	if err != nil {
		log.Printf("admin 사용자 조회 실패: %v", err)
		a.renderLogin(w, http.StatusBadGateway, "GitHub에서 계정을 확인하지 못했다")
		return
	}

	if !a.allows(login) {
		// **누가 두드렸는지는 남긴다.** 화면에는 계정 이름을 도로 찍지 않는다 —
		// 그대로 그리면 남이 지은 이름이 우리 화면에 나오는 자리가 된다.
		log.Printf("admin 로그인 거부: %q는 허용 목록에 없다", login)
		a.renderLogin(w, http.StatusForbidden, "이 계정으로는 들어올 수 없다.")
		return
	}

	a.setSession(w, r, login)
	log.Printf("admin 로그인: %s", login)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// handleLogout은 세션 쿠키를 지운다.
//
// **authenticator가 아니라 Server의 메서드다.** 하는 일이 쿠키 하나를 지우는
// 것뿐이라 인증 설정이 필요 없고, 그래야 인증이 꺼진 채로 뜬 서버에서도
// 등록해 둘 수 있다. 읽는 화면(사이드바)에서 부르는 자리라 언제나 있어야 한다.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	clearCookie(w, r, sessionCookie, "/")
	// 인증이 꺼져 있으면 돌아갈 로그인 화면이 없다.
	if s.auth == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// exchange는 code를 access token으로 바꾼다.
func (a *authenticator) exchange(code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", a.cfg.ClientID)
	form.Set("client_secret", a.cfg.ClientSecret)
	form.Set("code", code)

	req, err := http.NewRequest(http.MethodPost, a.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// 이걸 안 주면 GitHub이 폼 인코딩으로 답한다.
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("토큰 응답이 %d", resp.StatusCode)
	}

	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	// 응답 크기를 제한한다. 상대가 끝없이 보내면 메모리가 그만큼 간다.
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return "", err
	}
	if out.Error != "" {
		return "", fmt.Errorf("GitHub이 거절: %s (%s)", out.Error, out.Description)
	}
	if out.AccessToken == "" {
		return "", errors.New("토큰이 비어 있다")
	}
	return out.AccessToken, nil
}

// fetchLogin은 토큰으로 "누구인지"를 묻는다.
func (a *authenticator) fetchLogin(token string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, a.cfg.UserURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("사용자 응답이 %d", resp.StatusCode)
	}

	var out struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return "", err
	}
	if out.Login == "" {
		return "", errors.New("login이 비어 있다")
	}
	return out.Login, nil
}
