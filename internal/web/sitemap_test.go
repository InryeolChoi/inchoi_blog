package web

import (
	"encoding/xml"
	"net/http/httptest"
	"strings"
	"testing"
)

// **여기서 지키는 것은 하나다: 사이트맵이 draft를 흘리면 안 된다.**
//
// 사이트맵은 크롤러에게 주소를 알려주는 것이라 **그 자체가 공개다.** 숨긴 글의
// slug가 나가면 목록·카운트·본문 링크에서 draft를 가려온 것이 통째로 무의미해진다.
func TestSitemapNeverLeaksDrafts(t *testing.T) {
	// seedTestDB에는 draft 글이 하나 들어 있다(draft-post).
	h := handlerFor(t, seedTestDB(t))
	rec := httptest.NewRecorder()
	rec.Body.Reset()
	req := httptest.NewRequest("GET", "/sitemap.xml", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("sitemap.xml이 %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "draft-post") {
		t.Errorf("숨긴 글이 사이트맵에 샜다:\n%s", body)
	}
	if !strings.Contains(body, "/p/category-post") {
		t.Errorf("보이는 글이 사이트맵에 없다:\n%s", body)
	}
}

// TestSitemapIsWellFormed는 크롤러가 읽을 수 있는 XML인지 본다.
// 반쯤 그린 XML은 아무것도 안 준 것만 못하다.
func TestSitemapIsWellFormed(t *testing.T) {
	h := handlerFor(t, seedTestDB(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/sitemap.xml", nil))

	var set urlSet
	if err := xml.Unmarshal(rec.Body.Bytes(), &set); err != nil {
		t.Fatalf("XML을 못 읽는다: %v\n%s", err, rec.Body.String())
	}
	if set.NS != "http://www.sitemaps.org/schemas/sitemap/0.9" {
		t.Errorf("네임스페이스가 %q다", set.NS)
	}
	if len(set.URLs) == 0 {
		t.Fatal("주소가 하나도 없다")
	}
	// 홈은 늘 들어간다.
	var home bool
	for _, u := range set.URLs {
		if strings.HasSuffix(u.Loc, "/") && strings.Count(u.Loc, "/") == 3 {
			home = true
		}
		if !strings.HasPrefix(u.Loc, "http") {
			t.Errorf("절대 주소가 아니다: %s", u.Loc)
		}
	}
	if !home {
		t.Error("홈이 사이트맵에 없다")
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/xml") {
		t.Errorf("Content-Type이 %q다", got)
	}
}

// TestSitemapEscapesKoreanSlugs는 한글 slug가 URL 인코딩되는지 본다.
// 카테고리 주소에 한글이 그대로 들어 있어서(`/data-math/수리통계-이론`)
// 인코딩하지 않으면 규격에 안 맞는다.
func TestSitemapEscapesKoreanSlugs(t *testing.T) {
	if got, want := escapePath("/data-math/수리통계-이론"), "/data-math/%EC%88%98%EB%A6%AC%ED%86%B5%EA%B3%84-%EC%9D%B4%EB%A1%A0"; got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
	if got := escapePath("/"); got != "/" {
		t.Errorf("홈이 %q가 됐다", got)
	}
}

// TestRobotsPointsAtSitemap은 robots.txt가 사이트맵을 가리키고 admin을
// 막는지 본다. **이건 차단 장치가 아니라 요청이다** — 진짜 관문은 허용
// 목록과 세션이고, 여기서 얻는 것은 크롤 예산을 글 쪽에 쓰게 하는 것뿐이다.
func TestRobotsPointsAtSitemap(t *testing.T) {
	h := handlerFor(t, seedTestDB(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/robots.txt", nil))

	body := rec.Body.String()
	for _, want := range []string{"User-agent: *", "Disallow: /admin", "Disallow: /api/", "/sitemap.xml"} {
		if !strings.Contains(body, want) {
			t.Errorf("robots.txt에 %q가 없다:\n%s", want, body)
		}
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Errorf("Content-Type이 %q다", got)
	}
}

// TestSiteOriginFollowsForwardedProto는 프록시 뒤에서 https를 알아보는지 본다.
// blog 자신은 평문 8080을 들으므로 r.TLS가 언제나 nil이다 — 그대로 두면
// 사이트맵이 http 주소를 뿌린다.
func TestSiteOriginFollowsForwardedProto(t *testing.T) {
	r := httptest.NewRequest("GET", "/sitemap.xml", nil)
	r.Host = "inquieto.dev"
	if got := siteOrigin(r); got != "http://inquieto.dev" {
		t.Errorf("헤더 없이 %q", got)
	}
	r.Header.Set("X-Forwarded-Proto", "https")
	if got := siteOrigin(r); got != "https://inquieto.dev" {
		t.Errorf("X-Forwarded-Proto를 안 봤다: %q", got)
	}
}
