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
	"strconv"
	"strings"

	"github.com/inryeol/blog/internal/markdown"
)

//go:embed templates/*.html
var templateFS embed.FS

// staticFS는 브라우저에서 도는 스크립트다. 빌드 스텝 없이 손으로 쓴 것을
// 그대로 바이너리에 박아 서빙한다.
//
//go:embed static/*.js
var staticFS embed.FS

// Server는 라우팅과 렌더링을 들고 있다.
type Server struct {
	store *store
	md    *markdown.Renderer
	// pages는 페이지 이름 → 그 페이지만 담은 템플릿 집합이다.
	pages map[string]*template.Template
}

// pageTemplates는 layout과 함께 묶을 페이지 템플릿 목록이다.
var pageTemplates = []string{"home.html", "index.html", "category.html", "post.html"}

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
	mux.HandleFunc("GET /static/{file}", s.handleStatic)
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
	// Outline은 본문에서 뽑은 목차다. 짧은 글에는 안 붙인다.
	Outline []markdown.Heading
	// Nav는 사이드바에 그릴 카테고리 트리다. render가 채운다.
	Nav []NavCategory
	// openCats는 사이드바에서 펼쳐둘 카테고리다(지금 보고 있는 곳의 조상).
	// activeCat은 현재 위치다. 둘 다 핸들러가 채우고 render가 트리에 반영한다.
	openCats  map[int64]bool
	activeCat int64
	// HomeActive는 사이드바의 "홈"에 현재 위치 표시를 할지다.
	HomeActive bool
	// TotalPosts는 사이드바 머리에 찍는 전체 글 수다. render가 채운다.
	TotalPosts int
	// Assets는 이 페이지가 CDN에서 받아야 할 것이다. render가 Body를 보고 채운다
	// (assets.go).
	Assets assetNeeds
}

// TotalPostsText는 천 단위를 끊은 글 수다. 네 자리라 끊는 편이 읽기 쉽다.
func (d pageData) TotalPostsText() string { return comma(d.TotalPosts) }

// comma는 정수에 천 단위 구분을 넣는다. 표준 라이브러리에 없어서 직접 쓴다
// (이것 하나 때문에 의존성을 늘리지 않는다).
func comma(n int) string {
	s := strconv.Itoa(n)
	if n < 0 {
		return "-" + comma(-n)
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
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

// render는 페이지 하나를 그린다. 사이드바는 모든 페이지에 나오므로 여기서
// 한 번에 채운다 — 핸들러마다 잊지 않고 넣게 하는 것보다 안전하다.
func (s *Server) render(w http.ResponseWriter, name string, data pageData) {
	t, ok := s.pages[name]
	if !ok {
		s.fail(w, fmt.Errorf("템플릿이 없다: %s", name))
		return
	}
	nav, err := s.store.NavTree()
	if err != nil {
		s.fail(w, err)
		return
	}
	markNav(nav, data.openCats, data.activeCat)
	data.Nav = nav
	// 최상위 분류의 글 수 합이 곧 전체다. 카테고리 없는 글은 현재 0건이라
	// 따로 세지 않는다 — 생기면 여기에 안 잡힌다.
	for _, c := range nav {
		data.TotalPosts += c.PostCount
	}
	// 이 페이지가 CDN에서 받을 것을 본문을 보고 정한다. 핸들러마다 챙기지
	// 않도록 여기 한 곳에서 한다 — 사이드바를 여기서 채우는 것과 같은 이유다.
	data.Assets = needsFor(data.Body)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		// 헤더를 이미 보냈을 수 있어서 상태 코드를 바꿀 수 없다. 로그만 남긴다.
		fmt.Printf("템플릿 실행 실패(%s): %v\n", name, err)
	}
}

// outlineMinHeadings는 목차를 붙이기 시작하는 제목 개수다. 하나짜리 목차는
// 자리만 차지하고 알려주는 게 없다.
const outlineMinHeadings = 3

// renderedBody는 본문 한 편을 그린 결과다.
type renderedBody struct {
	HTML    template.HTML
	Outline []markdown.Heading
	fix     bodyFix
}

// renderPostBody는 본문을 HTML로 바꾸고 목차를 함께 뽑는다.
//
// 렌더링 직전에 죽은 링크 두 종류를 손본다 — inline.go 참고. DB의 body는
// 건드리지 않는다. 목차는 **손본 뒤의 문자열**에서 뽑아야 앵커가 본문과 맞는다.
func (s *Server) renderPostBody(post *Post) (renderedBody, error) {
	resolved, fix, err := s.resolveBody(post.Body, post.OriginalPath.String)
	if err != nil {
		return renderedBody{}, err
	}
	html, err := s.md.Render(resolved)
	if err != nil {
		return renderedBody{}, err
	}
	out := renderedBody{HTML: html, fix: fix}
	if heads := s.md.Outline(resolved); len(heads) >= outlineMinHeadings {
		out.Outline = heads
	}
	return out, nil
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	fmt.Printf("요청 처리 실패: %v\n", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

// homeCategorySlug는 홈에 펼칠 표지 글을 가진 카테고리다.
// curation.Covers가 이 분류에 자기소개 글을 표지로 붙여둔다.
const homeCategorySlug = "intro"

// handleIndex는 홈이다. **카테고리 목록이 아니라 자기소개를 편다.**
//
// 분류로 들어가는 길은 이제 사이드바가 늘 열어두고 있어서, 첫 화면까지
// 목록이면 같은 것을 두 번 보여주는 셈이다. 소개 카테고리의 표지 글이 곧
// 그 자리에 놓을 글이다.
//
// 표지가 없으면(이관 상태에 따라 비어 있을 수 있다) 예전처럼 최상위 분류
// 목록을 보여준다. 홈이 빈 화면이 되는 것보다 낫다.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	slug, err := s.store.CoverPostSlugOf(homeCategorySlug)
	if err != nil {
		s.fail(w, err)
		return
	}
	if slug != "" {
		post, err := s.store.PostBySlug(slug)
		if err != nil {
			s.fail(w, err)
			return
		}
		if post != nil {
			rendered, err := s.renderPostBody(post)
			if err != nil {
				s.fail(w, err)
				return
			}
			s.render(w, "home.html", pageData{
				Title:      post.Title,
				Post:       post,
				Body:       rendered.HTML,
				HomeActive: true,
			})
			return
		}
	}

	cats, err := s.store.TopCategories()
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, "index.html", pageData{Title: "카테고리", Categories: cats, HomeActive: true})
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
	var cover renderedBody
	var coverPost *Post
	if current.CoverPostSlug != "" {
		coverPost, err = s.store.PostBySlug(current.CoverPostSlug)
		if err != nil {
			s.fail(w, err)
			return
		}
		if coverPost != nil {
			if cover, err = s.renderPostBody(coverPost); err != nil {
				s.fail(w, err)
				return
			}
			if children, err = s.dropCoveredChildren(children, cover.fix.Shown); err != nil {
				s.fail(w, err)
				return
			}
		}
	}

	crumbList, basePath := crumbs(trail)
	open, active := openTrail(trail)
	s.render(w, "category.html", pageData{
		Title:      current.Name,
		Trail:      crumbList,
		BasePath:   basePath,
		Categories: children,
		Posts:      posts,
		Post:       coverPost,
		Body:       cover.HTML,
		openCats:   open,
		activeCat:  active,
	})
}

// dropCoveredChildren은 표지 글 본문이 이미 통째로 펼쳐 보여준 하위 분류를
// 목록에서 뺀다.
//
// 노션에서 한 절의 머리에 두던 인라인 데이터베이스가 두 벌로 나타나기 때문이다:
// categorize가 경로에서 그 이름을 카테고리로 뽑아 "하위 분류"에 넣고, 같은
// 데이터베이스가 표지 글 본문에도 링크로 남아 이제 목록으로 펼쳐진다. 그러면
// 한 화면에 같은 것이 두 번 나온다.
//
// **이름이 아니라 글로 따진다.** 이름이 같아도 분류 쪽에 더 깊은 글이 달려 있으면
// 본문이 다 보여준 게 아니다. 실제로 `Language > 프로그래밍 언어`가 그렇다 —
// 이름은 같은데 분류에는 191건이 있고 본문에 펼쳐진 건 7건뿐이라, 이름만 보고
// 빼면 184건으로 가는 길이 사라진다.
//
// 섹션 자체를 없애지 않는 이유도 같다. `머신러닝 & 딥러닝`처럼 본문이 다루지 않는
// 분류가 하나 더 있는 경우가 있다.
func (s *Server) dropCoveredChildren(children []Category, shown map[string]bool) ([]Category, error) {
	if len(shown) == 0 || len(children) == 0 {
		return children, nil
	}
	out := make([]Category, 0, len(children))
	for _, c := range children {
		slugs, err := s.store.CategorySubtreePostSlugs(c.ID)
		if err != nil {
			return nil, err
		}
		covered := len(slugs) > 0
		for slug := range slugs {
			if !shown[slug] {
				covered = false
				break
			}
		}
		if !covered {
			out = append(out, c)
		}
	}
	return out, nil
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

	rendered, err := s.renderPostBody(post)
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

	// 글을 열면 사이드바에서 그 글이 속한 분류가 펼쳐져 있어야 한다.
	open, active := openTrail(post.Trail)
	s.render(w, "post.html", pageData{
		Title:     post.Title,
		Trail:     crumbList,
		Post:      post,
		Body:      rendered.HTML,
		Outline:   rendered.Outline,
		openCats:  open,
		activeCat: active,
	})
}

// staticName은 정적 파일 이름이 될 수 있는 형태인지 본다.
// {file}은 한 칸짜리 경로라 /가 안 들어오지만, 이름을 좁혀 두면 embed에
// 없는 것을 찾느라 헛일하지 않는다.
var staticName = regexp.MustCompile(`^[a-z0-9_-]+\.(js|css)$`)

// handleStatic은 바이너리에 박힌 스크립트를 내보낸다.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	if !staticName.MatchString(name) {
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
	}
	// 경로에 내용 해시가 없어서 immutable을 걸 수 없다. 바이너리를 새로 올리면
	// 곧 반영되도록 짧게 잡는다.
	w.Header().Set("Cache-Control", "public, max-age=300")
	if _, err := w.Write(data); err != nil {
		fmt.Printf("정적 파일 쓰기 실패(%s): %v\n", name, err)
	}
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
