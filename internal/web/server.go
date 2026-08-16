// Package web은 읽기 전용 공개 페이지를 서빙한다.
//
// 지금은 인증도 접근 제어도 없고 status(draft/unlisted/published)를 가리지 않는다.
// 로컬에서 이관 결과를 눈으로 확인하려고 만든 것이다. 글쓰기/수정(admin)은 별도다.
package web

import (
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/inryeol/blog/internal/markdown"
)

//go:embed templates/*.html
var templateFS embed.FS

// Server는 라우팅과 렌더링을 들고 있다.
type Server struct {
	store *store
	md    *markdown.Renderer
	// pages는 페이지 이름 → 그 페이지만 담은 템플릿 집합이다.
	pages map[string]*template.Template
}

// pageTemplates는 layout과 함께 묶을 페이지 템플릿 목록이다.
var pageTemplates = []string{"index.html", "category.html", "post.html"}

// New는 서버를 만든다. 템플릿은 바이너리에 박혀 있으므로 여기서 한 번만 파싱한다.
//
// 페이지마다 따로 파싱한다. Go 템플릿은 이름 공간이 하나라, 여러 파일을 한 번에
// 파싱하면 각 파일의 {{define "content"}}가 서로를 덮어써서 마지막 것만 남는다.
// 그러면 모든 페이지가 같은 내용을 그린다.
func New(db *sql.DB) (*Server, error) {
	pages := make(map[string]*template.Template, len(pageTemplates))
	for _, name := range pageTemplates {
		t, err := template.ParseFS(templateFS, "templates/layout.html", "templates/"+name)
		if err != nil {
			return nil, fmt.Errorf("템플릿 파싱(%s): %w", name, err)
		}
		pages[name] = t
	}
	return &Server{
		store: &store{db: db},
		md:    markdown.New(),
		pages: pages,
	}, nil
}

// Handler는 라우트를 붙인 http.Handler를 돌려준다.
//
// 카테고리는 3단계라 경로도 세 단계까지 받는다. 글 1408건 중 1279건이 3단계
// 카테고리에 있어서, 두 단계까지만 열면 대부분의 글에 탐색으로 닿지 못한다.
//
// "/{slug}"는 무엇이든 받는 패턴이라 "/p/..."나 "/img/..."와 겹칠 것 같지만,
// Go의 ServeMux는 더 구체적인 패턴을 먼저 고르므로 리터럴이 있는 쪽이 이긴다.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /p/{slug}", s.handlePost)
	mux.HandleFunc("GET /img/{sha256}", s.handleImage)
	mux.HandleFunc("GET /{l1}", s.handleCategory)
	mux.HandleFunc("GET /{l1}/{l2}", s.handleCategory)
	mux.HandleFunc("GET /{l1}/{l2}/{l3}", s.handleCategory)
	return mux
}

// Crumb는 상단 경로에 찍을 링크 한 칸이다.
type Crumb struct {
	Name string
	URL  string
}

// pageData는 모든 템플릿이 공통으로 받는 것이다.
type pageData struct {
	Title string
	Trail []Crumb
	// BasePath는 현재 카테고리의 경로다. 하위 링크를 이 뒤에 붙인다.
	BasePath   string
	Categories []Category
	Posts      []PostSummary
	Post       *Post
	Body       template.HTML
}

// crumbs는 카테고리 경로를 링크로 바꾼다.
// slug에 한글이 들어 있어서 경로 조각마다 URL 인코딩을 한다.
func crumbs(trail []Category) ([]Crumb, string) {
	out := make([]Crumb, 0, len(trail))
	path := ""
	for _, c := range trail {
		path += "/" + url.PathEscape(c.Slug)
		out = append(out, Crumb{Name: c.Name, URL: path})
	}
	return out, path
}

func (s *Server) render(w http.ResponseWriter, name string, data pageData) {
	t, ok := s.pages[name]
	if !ok {
		s.fail(w, fmt.Errorf("템플릿이 없다: %s", name))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		// 헤더를 이미 보냈을 수 있어서 상태 코드를 바꿀 수 없다. 로그만 남긴다.
		fmt.Printf("템플릿 실행 실패(%s): %v\n", name, err)
	}
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	fmt.Printf("요청 처리 실패: %v\n", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	cats, err := s.store.TopCategories()
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, "index.html", pageData{Title: "카테고리", Categories: cats})
}

// handleCategory는 1~3단계 카테고리를 모두 처리한다.
// 경로를 위에서부터 따라 내려가면서 부모가 맞는지 확인한다.
func (s *Server) handleCategory(w http.ResponseWriter, r *http.Request) {
	var slugs []string
	for _, name := range []string{"l1", "l2", "l3"} {
		if v := r.PathValue(name); v != "" {
			slugs = append(slugs, v)
		}
	}

	var trail []Category
	var parentID sql.NullInt64
	for _, slug := range slugs {
		cat, err := s.store.CategoryBySlug(slug, parentID)
		if err != nil {
			s.fail(w, err)
			return
		}
		if cat == nil {
			http.NotFound(w, r)
			return
		}
		trail = append(trail, *cat)
		parentID = sql.NullInt64{Int64: cat.ID, Valid: true}
	}

	current := trail[len(trail)-1]
	children, err := s.store.ChildCategories(current.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	posts, err := s.store.PostsInCategory(current.ID)
	if err != nil {
		s.fail(w, err)
		return
	}

	// 표지 글이 있으면 본문을 목록 위에 펼친다. 소개처럼 목록이 아니라 글 자체가
	// 그 분류의 알맹이인 경우가 있다. 한 번 더 눌러 들어가게 하지 않는다.
	var coverBody template.HTML
	var coverPost *Post
	if current.CoverPostSlug != "" {
		coverPost, err = s.store.PostBySlug(current.CoverPostSlug)
		if err != nil {
			s.fail(w, err)
			return
		}
		if coverPost != nil {
			if coverBody, err = s.md.Render(coverPost.Body); err != nil {
				s.fail(w, err)
				return
			}
		}
	}

	crumbList, basePath := crumbs(trail)
	s.render(w, "category.html", pageData{
		Title:      current.Name,
		Trail:      crumbList,
		BasePath:   basePath,
		Categories: children,
		Posts:      posts,
		Post:       coverPost,
		Body:       coverBody,
	})
}

func (s *Server) handlePost(w http.ResponseWriter, r *http.Request) {
	post, err := s.store.PostBySlug(r.PathValue("slug"))
	if err != nil {
		s.fail(w, err)
		return
	}
	if post == nil {
		http.NotFound(w, r)
		return
	}

	body, err := s.md.Render(post.Body)
	if err != nil {
		s.fail(w, err)
		return
	}

	// 카테고리 경로 뒤에 상위 글 사슬을 이어붙인다. 노션 계층이 카테고리 3단계보다
	// 깊었던 글은 카테고리만으로는 어디쯤인지 알 수 없다.
	crumbList, _ := crumbs(post.Trail)
	for _, a := range post.Ancestors {
		crumbList = append(crumbList, Crumb{Name: a.Title, URL: "/p/" + url.PathEscape(a.Slug)})
	}

	s.render(w, "post.html", pageData{
		Title: post.Title,
		Trail: crumbList,
		Post:  post,
		Body:  body,
	})
}

// sha256Pattern은 이미지 경로가 해시 형태인지 본다.
// 형태가 아니면 DB를 찌르지 않고 바로 404를 준다.
var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	sha := r.PathValue("sha256")
	if !sha256Pattern.MatchString(sha) {
		http.NotFound(w, r)
		return
	}

	img, err := s.store.ImageBySHA256(sha)
	if err != nil {
		s.fail(w, err)
		return
	}
	if img == nil {
		http.NotFound(w, r)
		return
	}

	mime := img.MIME
	if mime == "" || !strings.HasPrefix(mime, "image/") {
		// mime 컬럼이 비었거나 이상하면 브라우저가 알아서 하게 둔다.
		mime = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mime)
	// 내용 주소 지정이라(경로가 곧 sha256) 내용이 바뀌면 경로도 바뀐다.
	// 그래서 오래 캐시해도 안전하다.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	if _, err := w.Write(img.Data); err != nil {
		fmt.Printf("이미지 쓰기 실패(%s): %v\n", sha, err)
	}
}
