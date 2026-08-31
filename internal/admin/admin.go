// Package admin은 글을 쓰고 고치는 화면이다. 공개 페이지(internal/web)와
// **일부러 갈라 둔 패키지**다.
//
// # 지금 어디까지 와 있나
//
// 1단계(화면)·2단계(인증)·3단계(저장)가 끝났다. 글은 실제로 DB에 들어가고
// 이미지도 BLOB으로 저장된다(save.go, upload.go). 남은 것은 4단계(AI 삽입)다.
//
// 인증은 auth.go에 있다. 허용 목록(AuthConfig.AllowedLogins)에 적은 GitHub
// 계정만 들어올 수 있고, 관문은 Handler()가 mux 바깥에 두른다.
//
// **쓰기에는 관문이 하나 더 있다**(sameOrigin). 남의 사이트에서 온 POST/PUT을
// 막는다 — 세션 쿠키가 SameSite=Lax라 브라우저가 이미 한 겹 막아주지만,
// 그건 브라우저에 기대는 것이라 서버도 자기 눈으로 본다.
//
// 순서는 CLAUDE.md의 "Admin 화면" 로드맵에 적혀 있다. **인증이 붙었다고 이걸
// 배포에서 함부로 켜지 않는다** — 켜는 스위치는 인스턴스의
// /etc/blog/admin.env 하나뿐이고(deploy/blog.service의 $BLOG_ADMIN_FLAG),
// 저장소에는 없다.
//
// # 왜 web과 따로인가
//
// web은 "읽기 전용 공개 페이지"라고 못박아 둔 패키지다. draft를 어디에서도 안
// 보여주는 것이 그쪽 규칙인데(store.notHidden), admin은 반대로 draft가 주인공이다.
// 한 패키지에서 플래그로 가르면 언젠가 공개 쪽에 샌다.
//
// 다만 **스타일시트와 마크다운 렌더러는 나눠 쓴다.** 미리보기가 실제 글 화면과
// 다르게 보이면 그건 미리보기가 아니다 — web.SiteCSS()와 markdown.New()를 그대로 쓴다.
package admin

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/inryeol/blog/internal/markdown"
	"github.com/inryeol/blog/internal/web"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Server는 admin 화면과 그 API를 들고 있다.
type Server struct {
	store *store
	md    *markdown.Renderer
	shell *template.Template
	css   template.CSS
	tags  template.HTML
	// auth가 nil이면 **관문이 없다.** cmd/blog가 이 상태를 loopback에서만
	// 허용한다(-admin-no-auth). auth.go의 guard 참고.
	auth *authenticator
}

// New는 admin 서버를 만든다.
//
// **auth를 반드시 적어야 한다.** nil이면 인증이 없는 화면이 된다. 기본값으로
// 슬쩍 얻어지지 않게 인자로 뒀다 — 부르는 쪽이 `nil`이라고 쓰게 만드는 것이
// 요점이다.
func New(db *sql.DB, auth *AuthConfig) (*Server, error) {
	shell, err := template.ParseFS(templateFS, "templates/admin.html")
	if err != nil {
		return nil, fmt.Errorf("admin 템플릿 파싱: %w", err)
	}
	css, err := web.SiteCSS()
	if err != nil {
		return nil, fmt.Errorf("사이트 CSS: %w", err)
	}
	tags, err := web.PreviewAssetTags()
	if err != nil {
		return nil, fmt.Errorf("CDN 태그: %w", err)
	}
	s := &Server{
		store: &store{db: db},
		// **공개 페이지와 같은 렌더러다.** 확장(수식·코드 라벨·외부 링크 카드)이
		// 전부 여기 들어 있어서, 미리보기와 실제 글이 같은 결과를 낸다.
		md:    markdown.New(),
		shell: shell,
		css:   css,
		tags:  tags,
	}
	if auth != nil {
		if s.auth, err = newAuthenticator(*auth, css); err != nil {
			return nil, fmt.Errorf("admin 인증 설정: %w", err)
		}
	}
	return s, nil
}

// Handler는 admin 라우트를 붙인 http.Handler를 돌려준다.
//
// cmd/blog가 이걸 "/admin/"과 "/api/admin/" 앞에 물린다. 공개 mux의 catch-all
// (GET /)보다 구체적이라 ServeMux가 이쪽을 먼저 고른다.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// 화면. CSR이라 어느 경로로 들어와도 같은 껍데기를 주고, 그 안에서
	// JS가 목록/편집을 갈아 끼운다. 새로고침해도 화면이 유지된다.
	mux.HandleFunc("GET /admin", s.handleShell)
	mux.HandleFunc("GET /admin/{rest...}", s.handleShell)
	mux.HandleFunc("GET /admin/static/{file}", s.handleStatic)

	// API. 전부 JSON이고, 실패해도 HTML을 돌려주지 않는다 — fetch()가 받는
	// 자리라 오류 화면이 오면 파싱에서 터지고 진짜 원인이 가려진다.
	mux.HandleFunc("GET /api/admin/posts", s.handleList)
	mux.HandleFunc("GET /api/admin/posts/{slug}", s.handleGet)
	mux.HandleFunc("GET /api/admin/categories", s.handleCategories)
	// 데이터 보기. 이 아카이브가 지금 어떤 상태인지 한 화면에서 본다(stats.go).
	mux.HandleFunc("GET /api/admin/stats", s.handleStats)
	mux.HandleFunc("POST /api/admin/preview", s.handlePreview)
	mux.HandleFunc("POST /api/admin/posts", s.handleSave)
	mux.HandleFunc("PUT /api/admin/posts/{slug}", s.handleSave)
	mux.HandleFunc("POST /api/admin/images", s.handleUpload)
	// 지우기 전에 무엇이 걸리는지 먼저 묻는 자리다. 무엇을 잃는지 모른 채
	// 확인 창의 "예"를 누르게 하지 않는다.
	mux.HandleFunc("GET /api/admin/posts/{slug}/refs", s.handleRefs)
	mux.HandleFunc("DELETE /api/admin/posts/{slug}", s.handleDelete)

	// 로그아웃은 인증이 꺼져 있어도 등록해 둔다. 읽는 화면의 사이드바가
	// 부르는 자리라 언제나 있어야 하고, 하는 일은 쿠키 하나를 지우는 것뿐이다.
	//
	// **POST다.** GET이면 남의 페이지에 박아둔 <img> 하나로도 사람을
	// 로그아웃시킬 수 있다.
	mux.HandleFunc("POST /admin/logout", s.handleLogout)

	// 어디에도 안 걸린 /api/admin/* 은 JSON 404를 준다.
	mux.HandleFunc("/api/admin/", func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, http.StatusNotFound, "그런 API가 없다: "+r.Method+" "+r.URL.Path)
	})

	if s.auth != nil {
		// 로그인 길. ServeMux가 더 구체적인 패턴을 먼저 고르므로 이 셋이
		// 위의 `GET /admin/{rest...}` 껍데기를 이긴다.
		mux.HandleFunc("GET /admin/login", s.auth.handleLoginPage)
		mux.HandleFunc("GET /admin/auth/start", s.auth.handleStart)
		mux.HandleFunc("GET /admin/auth/callback", s.auth.handleCallback)

		// **관문은 mux 바깥이다.** 안쪽에 두면 새 라우트를 더할 때마다 챙겨야
		// 하고, 한 번 빠뜨리면 그게 곧 구멍이다. CSRF 검사도 같은 이유로
		// mux 바깥이지만 **관문보다는 안쪽**이다 — 로그인하지 않은 요청의
		// 진짜 이유는 "출처가 다르다"가 아니라 "로그인이 필요하다"이고,
		// CSRF는 살아 있는 세션을 지키는 장치라 세션이 있을 때 의미가 있다.
		return recovering(s.auth.guard(sameOrigin(mux)))
	}
	// 인증이 없어도 CSRF 검사는 남긴다. 이 모드는 loopback 전용이지만,
	// 그렇다고 남의 페이지가 이 서버에 글을 쓰게 둘 이유는 없다.
	return recovering(sameOrigin(mux))
}

// recovering은 admin에서 난 panic이 연결만 끊고 끝나지 않게 한다.
//
// web.recovering과 하는 일은 같지만 **돌려주는 것이 다르다.** 여기는 대부분
// JSON API라 HTML 오류 화면을 주면 fetch()가 파싱에서 터진다.
func recovering(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			// "조용히 끊어라"라는 약속된 신호다. 삼키면 뜻이 사라진다.
			if v == http.ErrAbortHandler {
				panic(v)
			}
			log.Printf("admin panic: %s %s: %v\n%s", r.Method, r.URL.Path, v, debug.Stack())
			// 헤더를 이미 보냈으면 상태 코드를 못 바꾼다. 로그만 남기고 끊는다.
			writeErr(w, http.StatusInternalServerError, "서버에서 터졌다")
		}()
		next.ServeHTTP(w, r)
	})
}

// shellData는 껍데기 템플릿이 받는 것이다.
type shellData struct {
	// SiteCSS는 공개 페이지와 **같은** 스타일시트다. 미리보기가 실제 글과
	// 같아 보이는 근거가 이것 하나다.
	SiteCSS template.CSS
	// AssetTags는 KaTeX·highlight.js를 받아오는 태그다. 주소와 SRI 해시의
	// 정본은 layout.html이고 web.PreviewAssetTags()가 뽑아준다.
	AssetTags template.HTML
	// Statuses는 폼의 status 선택지다. 코드(Statuses)를 그대로 내려보내
	// 템플릿에 값을 또 적지 않는다.
	Statuses []string
	// Login은 지금 들어와 있는 GitHub 계정이다. 인증이 꺼져 있으면 빈 값이고,
	// 화면이 그 자리에 "인증 없음" 경고를 대신 그린다.
	Login string
}

func (s *Server) handleShell(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// 편집 화면이 중간 캐시에 남으면 안 된다. 아직 인증이 없어서 더 그렇다.
	w.Header().Set("Cache-Control", "no-store")
	// 검색엔진이 이 화면을 주워가지 않게 한다. robots.txt가 아직 없기도 하고,
	// 있어도 헤더가 더 확실하다.
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	login := ""
	if s.auth != nil {
		login = s.auth.currentLogin(r)
	}
	if err := s.shell.ExecuteTemplate(w, "admin", shellData{
		SiteCSS:   s.css,
		AssetTags: s.tags,
		Statuses:  Statuses,
		Login:     login,
	}); err != nil {
		log.Printf("admin 껍데기 렌더 실패: %v", err)
	}
}

// staticName은 정적 파일 이름이 될 수 있는 형태인지 본다. 이름이 그대로
// embed.FS 경로가 되므로 형태를 좁혀둔다.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	if strings.ContainsAny(name, "/\\.") && !strings.HasSuffix(name, ".js") && !strings.HasSuffix(name, ".css") {
		http.NotFound(w, r)
		return
	}
	data, err := staticFS.ReadFile("static/" + name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case strings.HasSuffix(name, ".js"):
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	case strings.HasSuffix(name, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	default:
		http.NotFound(w, r)
		return
	}
	// 만들면서 계속 고치는 파일이다. 캐시가 남으면 고친 게 안 보인다.
	w.Header().Set("Cache-Control", "no-store")
	if _, err := w.Write(data); err != nil {
		log.Printf("admin 정적 파일 쓰기 실패(%s): %v", name, err)
	}
}

// writeJSON은 JSON 하나를 내보낸다.
//
// **먼저 통째로 만들고 성공했을 때만 쓴다.** 인코딩 도중에 실패하면 이미 나간
// 절반짜리 JSON 뒤에 오류를 붙일 수 없다 — web의 오류 화면이 "본문을 다 그린
// 뒤에 한 번에 내보낸다"와 같은 이유다.
func writeJSON(w http.ResponseWriter, code int, v any) {
	buf, err := json.Marshal(v)
	if err != nil {
		log.Printf("JSON 인코딩 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "응답을 만들지 못했다")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	if _, err := w.Write(buf); err != nil {
		log.Printf("JSON 쓰기 실패: %v", err)
	}
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	buf, _ := json.Marshal(map[string]string{"error": msg})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_, _ = w.Write(buf)
}

// listLimit은 목록에 한 번에 실을 글 수다. 검색과 페이지 나누기는 다음 단계다.
const listLimit = 2000

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	posts, err := s.store.listPosts(listLimit)
	if err != nil {
		log.Printf("admin 목록 조회 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "목록을 가져오지 못했다")
		return
	}
	counts, err := s.store.counts()
	if err != nil {
		log.Printf("admin 카운트 조회 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "글 수를 세지 못했다")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"posts":  posts,
		"counts": counts,
		"limit":  listLimit,
	})
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	post, err := s.store.postBySlug(r.PathValue("slug"))
	if err != nil {
		log.Printf("admin 글 조회 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "글을 가져오지 못했다")
		return
	}
	if post == nil {
		writeErr(w, http.StatusNotFound, "그런 글이 없다")
		return
	}
	writeJSON(w, http.StatusOK, post)
}

// LoginFor는 이 요청에 들어와 있는 계정 이름을 준다. 없으면 빈 문자열이다.
//
// **공개 화면(internal/web)이 "고치기"를 보여줄지 정하는 데 쓴다.** web은
// 세션이 무엇인지 몰라야 하므로 쿠키도 서명 키도 넘기지 않고 이 질문 하나만
// 함수로 건넨다(web.WithEditor).
//
// 인증이 꺼진 채로 뜬 서버(-admin-no-auth)에서는 **누구나 들어와 있는 것과
// 같다.** 그 모드는 loopback에서만 뜨고 화면 위에 그렇다고 띠가 붙어 있다.
// 여기서 빈 문자열을 주면 로컬에서 이 기능만 확인할 수 없게 된다.
func (s *Server) LoginFor(r *http.Request) string {
	if s.auth == nil {
		return "(인증 없음)"
	}
	return s.auth.currentLogin(r)
}
