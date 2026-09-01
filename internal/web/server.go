// Package web은 읽기 전용 공개 페이지를 서빙한다.
//
// 인증과 접근 제어는 아직 없다. 다만 **draft는 어디에도 안 보이고 /p/{slug}도
// 404다** — 목록·카운트·본문 링크 전부에서 빠진다. 로컬에서 확인할 때만
// `-drafts`로 켠다. 글쓰기/수정(admin)은 별도로 만든다.
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
	// editorFor는 "지금 누가 들어와 있나"를 묻는다. nil이면 **글 화면에서
	// 고치는 길이 아예 없다** — 버튼도 스크립트도 나가지 않는다.
	editorFor func(*http.Request) string
}

// pageTemplates는 layout과 함께 묶을 페이지 템플릿 목록이다.
var pageTemplates = []string{"home.html", "index.html", "category.html", "post.html", "error.html"}

// New는 서버를 만든다. 템플릿은 바이너리에 박혀 있으므로 여기서 한 번만 파싱한다.
//
// 페이지마다 따로 파싱한다. Go 템플릿은 이름 공간이 하나라, 여러 파일을 한 번에
// 파싱하면 각 파일의 {{define "content"}}가 서로를 덮어써서 마지막 것만 남는다.
// 그러면 모든 페이지가 같은 내용을 그린다.
// options는 서버를 만들 때 정하는 것들이다.
type options struct {
	showDrafts bool
	editorFor  func(*http.Request) string
}

// Option은 서버를 만들 때 주는 선택지다.
type Option func(*options)

// WithDrafts는 draft 글까지 보여준다. **로컬에서 눈으로 확인할 때만 쓴다.**
//
// 기본값이 "가린다"인 이유: 공개 배포가 기본이고, 안 가리는 쪽이 사고다.
// 켜고 끄는 것을 실수해도 새는 방향이 아니라 막는 방향으로 틀리게 둔다.
func WithDrafts() Option { return func(o *options) { o.showDrafts = true } }

// WithEditor는 **글 화면에서 바로 고치는 길**을 연다. fn은 요청을 보고
// 들어와 있는 계정 이름을 주거나, 없으면 빈 문자열을 준다.
//
// # 왜 함수로 받나
//
// web은 "읽기 전용 공개 페이지"고 admin은 따로 있다. 세션이 무엇인지 web이
// 알게 되면 그 경계가 무너지므로, **"지금 누가 들어와 있나"라는 질문 하나만**
// 함수로 받는다. 쿠키도 서명 키도 web은 모른다.
//
// # 기본값은 "닫힘"이다
//
// 이 옵션을 안 주면 글 화면은 예전 그대로다 — 고치기 버튼도, 편집기
// 스크립트도 나가지 않는다. `-admin`이 없는 배포에서 실수로 열리는 일이
// 없어야 한다. cmd/blog가 admin을 켤 때만 이걸 건넨다.
func WithEditor(fn func(*http.Request) string) Option {
	return func(o *options) { o.editorFor = fn }
}

func New(db *sql.DB, opts ...Option) (*Server, error) {
	pages := make(map[string]*template.Template, len(pageTemplates))
	for _, name := range pageTemplates {
		t, err := template.ParseFS(templateFS, "templates/layout.html", "templates/"+name)
		if err != nil {
			return nil, fmt.Errorf("템플릿 파싱(%s): %w", name, err)
		}
		pages[name] = t
	}
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	return &Server{
		store:     &store{db: db, showDrafts: o.showDrafts},
		md:        markdown.New(),
		pages:     pages,
		editorFor: o.editorFor,
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
	// 네 단계 이상처럼 어느 패턴에도 안 걸리는 경로를 받는다. 이게 없으면
	// ServeMux가 기본 404(글자 한 줄)를 돌려줘서, 같은 사이트인데 없는 길을
	// 알려주는 화면이 두 가지가 된다.
	mux.HandleFunc("GET /", s.handleNotFound)
	// panic이 프로세스를 죽이거나 연결만 끊고 끝나지 않게 감싼다 (recover.go).
	return s.recovering(mux)
}

// handleNotFound는 어느 라우트에도 안 걸린 경로를 받는다.
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	s.notFound(w, r)
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
	// PostTree는 노션 경로에 남아 있던 층으로 다시 묶은 글 목록이다
	// (pathtree.go). 비어 있으면 Posts를 평소대로 그린다.
	PostTree []PostNode
	Post     *Post
	Body     template.HTML
	// AfterPosts는 카테고리 표지 글에서 글 목록 뒤로 보낼 참고 절이다.
	// 상세 글에서는 원래 본문 순서를 그대로 쓰므로 비어 있다.
	AfterPosts template.HTML
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
	// Deck은 갈래를 아이콘 카드로 펼칠 때 그릴 것이다 (deck.go).
	// 비어 있으면 평소대로 목록을 그린다.
	Deck []DeckCard
	// Links는 이 화면에서 아카이브 바깥으로 나가는 링크다 (links.go).
	Links []SiteLink
	// Err은 404·500 화면에만 채워진다 (errorpage.go).
	Err *errorInfo
	// Editor는 지금 들어와 있는 계정 이름이다. 비어 있으면 **고치는 길이
	// 화면에 아예 없다.** WithEditor를 안 준 서버에서는 언제나 비어 있다.
	Editor string
	// AdminOn은 이 서버에 글쓰기 화면이 있는지다. Editor와 다른 질문이다 —
	// "글쓰기 화면이 있다"와 "지금 들어와 있다"를 구별해야 **로그인 링크를
	// 보여줄지** 정할 수 있다. 둘을 하나로 묶으면 로그인하기 전에는 로그인
	// 링크가 안 나오는 우스운 상태가 된다.
	AdminOn bool
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
func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data pageData) {
	t, ok := s.pages[name]
	if !ok {
		s.fail(w, r, fmt.Errorf("템플릿이 없다: %s", name))
		return
	}
	nav, err := s.store.NavTree()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	markNav(nav, data.openCats, data.activeCat)
	// **여기 한 곳에서 채운다.** 핸들러마다 챙기게 하면 새 화면을 더할 때마다
	// 빠뜨리고, 그 빠뜨림은 "고칠 수 있는데 버튼이 없다"로 조용히 나타난다.
	// 사이드바와 자산 판정을 render가 채우는 것과 같은 이유다.
	if s.editorFor != nil {
		data.AdminOn = true
		data.Editor = s.editorFor(r)
	}
	data.Nav = nav
	// 최상위 분류의 글 수 합이 곧 전체다. 카테고리 없는 글은 현재 0건이라
	// 따로 세지 않는다 — 생기면 여기에 안 잡힌다.
	for _, c := range nav {
		data.TotalPosts += c.PostCount
	}
	// 이 페이지가 CDN에서 받을 것을 본문을 보고 정한다. 핸들러마다 챙기지
	// 않도록 여기 한 곳에서 한다 — 사이드바를 여기서 채우는 것과 같은 이유다.
	// 카테고리 표지의 참고 절을 목록 뒤로 떼어놓아도 그 안의 수식·코드·유튜브에
	// 필요한 자산을 빠뜨리면 안 된다.
	assetBody := template.HTML(string(data.Body) + string(data.AfterPosts))
	data.Assets = needsFor(assetBody)
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
	HTML       template.HTML
	AfterPosts template.HTML
	Outline    []markdown.Heading
	fix        bodyFix
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

// referenceVideoHeading은 카테고리 표지 글에서 글 목록 뒤로 보낼 절의 시작이다.
// 현재 이 제목을 쓰는 표지 글은 선형대수 하나뿐이다. 글 상세는 원문 순서를
// 그대로 두고, 카테고리에서 펼칠 때만 이 절을 떼어낸다.
var referenceVideoHeading = regexp.MustCompile(`(?m)^#{1,6}[ \t]+참고 동영상[ \t]*\r?$`)

func splitReferenceVideoSection(source string) (before, after string) {
	loc := referenceVideoHeading.FindStringIndex(source)
	if loc == nil {
		return source, ""
	}
	return strings.TrimRight(source[:loc[0]], "\r\n"), source[loc[0]:]
}

// renderCategoryCoverBody는 표지 글을 카테고리 페이지용으로 그린다.
// `참고 동영상` 절은 글 목록을 먼저 훑은 뒤 볼 수 있도록 별도로 렌더링한다.
func (s *Server) renderCategoryCoverBody(post *Post) (renderedBody, error) {
	resolved, fix, err := s.resolveBody(post.Body, post.OriginalPath.String)
	if err != nil {
		return renderedBody{}, err
	}
	before, after := splitReferenceVideoSection(resolved)
	beforeHTML, err := s.md.Render(before)
	if err != nil {
		return renderedBody{}, err
	}
	var afterHTML template.HTML
	if after != "" {
		afterHTML, err = s.md.Render(after)
		if err != nil {
			return renderedBody{}, err
		}
	}
	return renderedBody{HTML: beforeHTML, AfterPosts: afterHTML, fix: fix}, nil
}

// handleIndex는 아카이브의 탐색 허브다.
//
// 자기소개는 /intro가 전담한다. 홈까지 같은 글을 펼치면 사이드바의 "홈"과
// "소개"가 이름만 다른 중복 페이지가 된다. 홈은 최상위 분류와 전체 글 수만
// 받아 HTML 자체의 링크와 details로 시작점을 만든다.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	categories, err := s.store.TopCategories()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	recent, err := s.store.RecentPosts(recentLimit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, "home.html", pageData{
		Title:      "열렬히.뛰기",
		Categories: categories,
		Posts:      recent,
		HomeActive: true,
	})
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
			s.fail(w, r, err)
			return
		}
		if cat == nil {
			s.notFound(w, r)
			return
		}
		trail = append(trail, *cat)
		parentID = sql.NullInt64{Int64: cat.ID, Valid: true}
	}

	current := trail[len(trail)-1]
	children, err := s.store.ChildCategories(current.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	posts, err := s.store.PostsInCategory(current.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	crumbList, basePath := crumbs(trail)

	// 표지 글이 있으면 본문을 목록 위에 펼친다. 소개처럼 목록이 아니라 글 자체가
	// 그 분류의 알맹이인 경우가 있다. 한 번 더 눌러 들어가게 하지 않는다.
	var cover renderedBody
	var coverPost *Post
	if current.CoverPostSlug != "" && !listOnlyCategory(current.Slug) {
		coverPost, err = s.store.PostBySlug(current.CoverPostSlug)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if coverPost != nil {
			if cover, err = s.renderCategoryCoverBody(coverPost); err != nil {
				s.fail(w, r, err)
				return
			}
			// 표지 본문의 글 링크가 같은 이름의 하위 분류 표지라면 분류 링크로
			// 바꾼다. 그러면 소개 본문과 그 아래 모든 글을 한 번에 볼 수 있고,
			// 부모 화면의 별도 하위 분류 목록은 중복이라 뺄 수 있다.
			cover.HTML, children = linkCoveredChildCategories(cover.HTML, children, basePath)
			// 표지 본문의 목차가 이미 가리키는 글 트리는 아래 "글" 목록에서
			// 다시 보여주지 않는다. 상단 목차가 그 갈래의 입구다.
			posts = dropShownPostTrees(posts, cover.fix.Shown)
			if children, err = s.dropCoveredChildren(children, cover.fix.Shown); err != nil {
				s.fail(w, r, err)
				return
			}
		}
	}

	open, active := openTrail(trail)
	deck, err := s.deckFor(current, basePath, children)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	// 평평하게 쏟아지는 목록은 노션 경로에 남은 층으로 다시 묶는다. 근거가
	// 없으면 nil이라 예전 목록 그대로다 — pathtree.go 참고.
	s.render(w, r, "category.html", pageData{
		Title:      current.Name,
		Deck:       deck,
		Trail:      crumbList,
		BasePath:   basePath,
		Categories: children,
		Posts:      posts,
		PostTree:   pathTree(current, posts),
		Post:       coverPost,
		Body:       cover.HTML,
		AfterPosts: cover.AfterPosts,
		Links:      linksFor(current.Slug),
		openCats:   open,
		activeCat:  active,
	})
}

// linkCoveredChildCategories는 부모 표지 본문이 하위 분류의 표지 글을 직접
// 가리킬 때 그 링크를 하위 분류 URL로 바꾸고, 같은 분류를 아래 목록에서 뺀다.
// 글 상세로 보내면 그 분류의 나머지 글로 이어지지 않지만 분류로 보내면 표지 본문과
// 전체 글 목록을 함께 볼 수 있다. 표지 글 링크가 실제로 없으면 아무것도 빼지 않는다.
func linkCoveredChildCategories(body template.HTML, children []Category, basePath string) (template.HTML, []Category) {
	if body == "" || len(children) == 0 {
		return body, children
	}

	html := string(body)
	remaining := make([]Category, 0, len(children))
	for _, child := range children {
		if child.CoverPostSlug == "" {
			remaining = append(remaining, child)
			continue
		}
		from := `href="/p/` + template.HTMLEscapeString(child.CoverPostSlug) + `"`
		if !strings.Contains(html, from) {
			remaining = append(remaining, child)
			continue
		}
		to := `href="` + template.HTMLEscapeString(basePath+"/"+url.PathEscape(child.Slug)) + `"`
		html = strings.ReplaceAll(html, from, to)
	}
	return template.HTML(html), remaining
}

// listOnlyCategories는 표지 글보다 하위 분류 자체가 첫 화면의 알맹이인 곳이다.
//
// 표지 지정은 DB의 분류 관계로 그대로 보존하되, 여기서는 본문을 펼치지 않는다.
// PostsInCategory도 표지 글을 목록에서 빼므로 결과는 합성 중간층인
// `수리/통계: 이론`처럼 하위 분류 목록만 남는다.
var listOnlyCategories = map[string]bool{
	"머신러닝": true,
	// école 42의 표지 글은 Piscine·inner circle·서클별로 스무 개 넘는 과제를
	// 한 화면에 쭉 늘어놓는다. 그 과제들이 이미 하위 분류로 서 있어서, 펼치면
	// 같은 목록이 본문과 분류 목록에 두 벌로 나온다 — 누르면 그 분류로
	// 넘어가기만 하면 되는 자리다.
	"école-42": true,
}

func listOnlyCategory(slug string) bool {
	return listOnlyCategories[slug]
}

// dropShownPostTrees는 표지 본문이 이미 링크한 글과 그 하위 트리를 목록에서 뺀다.
// 부모 글이 목차에 있으면 그 상세 페이지에서 하위 글로 이어지므로, 카테고리
// 화면에 같은 사슬을 한 벌 더 펼칠 필요가 없다. 상단에서 안내하지 않은 갈래는
// 그대로 남겨 길을 잃지 않게 한다.
func dropShownPostTrees(posts []PostSummary, shown map[string]bool) []PostSummary {
	if len(posts) == 0 || len(shown) == 0 {
		return posts
	}
	out := make([]PostSummary, 0, len(posts))
	for _, post := range posts {
		if shown[post.Slug] {
			continue
		}
		post.Children = dropShownPostTrees(post.Children, shown)
		out = append(out, post)
	}
	return out
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
		s.fail(w, r, err)
		return
	}
	if post == nil {
		s.notFound(w, r)
		return
	}

	rendered, err := s.renderPostBody(post)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	// 카테고리 경로 뒤에 상위 글 사슬을 이어붙인다. 노션 계층이 카테고리 3단계보다
	// 깊었던 글은 카테고리만으로는 어디쯤인지 알 수 없다.
	crumbList, _ := crumbs(post.Trail)
	for _, a := range post.Ancestors {
		crumbList = append(crumbList, Crumb{Name: a.Title, URL: "/p/" + url.PathEscape(a.Slug)})
	}

	// 본문이 이미 가리키고 있는 글은 "하위 글"에서 뺀다. 안 그러면 한 화면에
	// 같은 목록이 두 번 나온다.
	post.Children = dropLinkedChildren(post.Children, rendered.HTML)

	// 글을 열면 사이드바에서 그 글이 속한 분류가 펼쳐져 있어야 한다.
	open, active := openTrail(post.Trail)
	s.render(w, r, "post.html", pageData{
		Title:     post.Title,
		Trail:     crumbList,
		Post:      post,
		Body:      rendered.HTML,
		Outline:   rendered.Outline,
		openCats:  open,
		activeCat: active,
	})
}

// postHref는 본문 HTML에서 글로 가는 링크를 잡는다.
var postHref = regexp.MustCompile(`href="/p/([^"#]+)`)

// dropLinkedChildren은 본문이 이미 링크로 가리키고 있는 하위 글을 "하위 글"
// 목록에서 뺀다.
//
// 노션에서 한 글의 머리에 목록을 두던 습관이 두 벌로 나타나기 때문이다.
// cmd/postparent가 parent_id를 채운 근거가 child_page 블록인데, **그 블록이
// 본문에서는 링크가 된다.** 그래서 본문에 목록이 있고 그 아래 "하위 글"에
// 같은 목록이 또 나온다. 인라인 데이터베이스가 목록으로 펼쳐지는 경우도
// 결국 같은 `<a href="/p/...">`라 여기서 같이 걸린다.
//
// **본문 HTML에서 찾는다.** 원문 마크다운이 아니라 렌더링 결과를 보는 이유는,
// 펼친 인라인 데이터베이스처럼 렌더링 단계에서 생기는 링크까지 세야 하기
// 때문이다(inline.go).
//
// **글 하나씩 따진다.** 본문이 가리키는 글은 눌러서 갈 수 있으니 목록에서
// 빼도 길이 사라지지 않고, 본문이 안 가리키는 형제는 그대로 남는다. 지금은
// 하위 글이 있는 6편 모두 자식 전부가 본문에 링크돼 있어 섹션이 통째로
// 사라지지만, parent_id를 다른 카테고리로 넓히면 일부만 빠지는 경우가 생긴다.
func dropLinkedChildren(children []PostSummary, body template.HTML) []PostSummary {
	if len(children) == 0 {
		return children
	}
	linked := map[string]bool{}
	for _, m := range postHref.FindAllStringSubmatch(string(body), -1) {
		slug, err := url.PathUnescape(m[1])
		if err != nil {
			slug = m[1]
		}
		linked[slug] = true
	}
	if len(linked) == 0 {
		return children
	}
	out := make([]PostSummary, 0, len(children))
	for _, c := range children {
		if linked[c.Slug] {
			continue
		}
		out = append(out, c)
	}
	return out
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
		s.fail(w, r, err)
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
