package web

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// 검색엔진에 주는 두 파일.
//
//	robots.txt    어디를 긁어도 되고 어디는 안 되는지
//	sitemap.xml   여기 이런 주소들이 있다
//
// # 왜 파일로 두지 않고 그때그때 그리나
//
// **DB가 정본이라** 글이 늘거나 status가 바뀔 때마다 파일을 다시 만들어야
// 한다. 요청이 올 때 DB에서 그리면 그 일이 아예 없고, 단일 바이너리 배포에
// 파일이 하나도 안 는다. 사이드바 트리를 매 요청 한 번의 조회로 그리는 것과
// 같은 판단이다.
//
// # draft가 새면 안 된다
//
// 사이트맵은 **크롤러에게 주소를 알려주는 것이라 그 자체가 공개다.** 숨긴 글의
// slug를 흘리면 draft 366편을 가려온 것이 무의미해진다. 판정은 새로 쓰지 않고
// `store.SitemapEntries`가 `notHidden`을 그대로 쓴다.

// robotsTxt는 크롤러에게 주는 규칙이다.
//
// **이건 차단 장치가 아니라 요청이다.** 나쁜 봇은 그냥 무시하므로 여기 적는
// 것으로 무엇을 지키지는 못한다 — `/admin`의 진짜 관문은 허용 목록과 세션이고,
// `/api/admin`은 로그인 없이 401 JSON을 준다. 여기서 얻는 것은 **크롤 예산을
// 글 쪽에 쓰게 하는 것**뿐이다.
//
// `/img/`와 `/static/`도 막는다. 그림 3.4MB짜리를 크롤러가 훑을 이유가 없고,
// 어느 글에도 안 쓰이는 이미지가 검색 결과에 뜨는 것도 원하지 않는다.
const robotsTxt = `User-agent: *
Disallow: /admin
Disallow: /api/
Disallow: /img/
Disallow: /static/

Sitemap: %s/sitemap.xml
`

func (s *Server) handleRobots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, robotsTxt, siteOrigin(r))
}

// urlSet은 sitemap.xml의 뿌리다. 이름과 네임스페이스는 규격이 정한 것이라
// 바꿀 수 없다.
type urlSet struct {
	XMLName xml.Name  `xml:"urlset"`
	NS      string    `xml:"xmlns,attr"`
	URLs    []sitemap `xml:"url"`
}

type sitemap struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	entries, err := s.store.SitemapEntries()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	origin := siteOrigin(r)
	set := urlSet{NS: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: make([]sitemap, 0, len(entries))}
	for _, e := range entries {
		u := sitemap{Loc: origin + escapePath(e.Path)}
		if e.Updated.Valid {
			// sitemap 규격은 W3C Datetime을 쓴다. 날짜만으로도 유효하고,
			// 시각까지 주면 초 단위로 바뀔 때마다 크롤러가 다시 받는다.
			u.LastMod = e.Updated.Time.UTC().Format("2006-01-02")
		}
		set.URLs = append(set.URLs, u)
	}

	// **다 그린 뒤에 한 번에 내보낸다.** 도중에 실패하면 반쯤 그린 XML이
	// 나가는데, 그건 크롤러에게 깨진 파일이라 아무것도 안 준 것만 못하다.
	// 오류 화면이 잘린 본문 뒤에 붙지 않게 하는 것과 같은 이유다.
	var b strings.Builder
	b.WriteString(xml.Header)
	enc := xml.NewEncoder(&b)
	enc.Indent("", "  ")
	if err := enc.Encode(set); err != nil {
		s.fail(w, r, fmt.Errorf("사이트맵 XML: %w", err))
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	fmt.Fprint(w, b.String())
}

// siteOrigin은 이 사이트의 주소 앞부분이다.
//
// **호스트를 코드에 박지 않는다.** 로컬에서 열면 127.0.0.1이고 배포에서는
// inquieto.dev인데, 박아두면 로컬 사이트맵이 남의 주소를 가리킨다.
//
// 스킴은 `X-Forwarded-Proto`로 안다. blog 자신은 평문 8080을 듣고 있어서
// `r.TLS`가 언제나 nil이다 — admin 세션 쿠키가 `Secure`를 정하는 근거와
// 같은 자리다(internal/admin/auth.go).
func siteOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
		scheme = p
	}
	return scheme + "://" + r.Host
}

// escapePath는 경로의 각 칸을 URL 인코딩한다. slug에 한글이 그대로 들어
// 있어서(`/data-math/수리통계-이론`) 인코딩하지 않으면 규격에 안 맞는다.
func escapePath(p string) string {
	if p == "/" {
		return "/"
	}
	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	for i, seg := range parts {
		parts[i] = url.PathEscape(seg)
	}
	return "/" + strings.Join(parts, "/")
}
