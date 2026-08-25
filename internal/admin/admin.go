// Package admin은 글을 쓰고 고치는 화면이다. 공개 페이지(internal/web)와
// **일부러 갈라 둔 패키지**다.
//
// # 지금 어디까지 와 있나
//
// 이번 단계는 **뼈대뿐이다.** 화면 모양을 먼저 보고 다듬으려고 만든 것이라
// 다음 셋이 없다:
//
//	인증  — 아무나 들어올 수 있다. 그래서 기본값이 "안 띄운다"이다(cmd/blog의 -admin).
//	저장  — "저장"은 서버 로그만 남기고 DB를 건드리지 않는다 (handleSave).
//	업로드 — 이미지 고르는 UI만 있고 받은 파일을 버린다 (handleUpload).
//
// 순서는 CLAUDE.md의 "Admin 화면" 로드맵에 적혀 있다. **1단계가 끝났다고 이걸
// 배포에 올리면 안 된다** — 2단계(인증)가 끝나기 전까지 이 화면은 로컬 전용이다.
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
}

// New는 admin 서버를 만든다.
func New(db *sql.DB) (*Server, error) {
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
	return &Server{
		store: &store{db: db},
		// **공개 페이지와 같은 렌더러다.** 확장(수식·코드 라벨·외부 링크 카드)이
		// 전부 여기 들어 있어서, 미리보기와 실제 글이 같은 결과를 낸다.
		md:    markdown.New(),
		shell: shell,
		css:   css,
		tags:  tags,
	}, nil
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
	mux.HandleFunc("POST /api/admin/preview", s.handlePreview)
	mux.HandleFunc("POST /api/admin/posts", s.handleSave)
	mux.HandleFunc("PUT /api/admin/posts/{slug}", s.handleSave)
	mux.HandleFunc("POST /api/admin/images", s.handleUpload)

	// 어디에도 안 걸린 /api/admin/* 은 JSON 404를 준다.
	mux.HandleFunc("/api/admin/", func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, http.StatusNotFound, "그런 API가 없다: "+r.Method+" "+r.URL.Path)
	})

	return recovering(mux)
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
}

func (s *Server) handleShell(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// 편집 화면이 중간 캐시에 남으면 안 된다. 아직 인증이 없어서 더 그렇다.
	w.Header().Set("Cache-Control", "no-store")
	// 검색엔진이 이 화면을 주워가지 않게 한다. robots.txt가 아직 없기도 하고,
	// 있어도 헤더가 더 확실하다.
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	if err := s.shell.ExecuteTemplate(w, "admin", shellData{
		SiteCSS:   s.css,
		AssetTags: s.tags,
		Statuses:  Statuses,
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
