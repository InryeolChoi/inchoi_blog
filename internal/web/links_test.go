package web

import (
	"net/http"
	"strings"
	"testing"
)

// **소개 화면에는 이제 바깥 링크가 없다**(2026-09-08). 개인 페이지
// (inryeolchoi.github.io) 카드를 본문 아래에 두고 있었는데, 사이드바 바닥의
// Pages 링크가 이미 같은 곳으로 가고 있어서 같은 길이 두 벌이었다.
//
// **장치 자체는 남는다.** 그래서 여기서 보는 것은 "링크가 없다"와 "표에 적으면
// 붙는다" 둘이다 — 표만 비운 것이지 기능을 걷어낸 것이 아니라는 뜻이고,
// 다음에 링크를 붙일 때 이 테스트가 그 길이 살아 있음을 보증한다.
func TestIntroHasNoOutboundCard(t *testing.T) {
	sqlDB := testDB(t)
	exec := execer(t, sqlDB)

	exec(`INSERT INTO categories (id, name, slug, sort_order) VALUES (10, '소개', 'intro', 0)`)
	exec(`INSERT INTO posts (id, slug, title, body, status, source, category_id, sort_order, created_at, updated_at)
	      VALUES (10, 'about-me', '최인렬', '늘 우직하게 도전하는 개발자입니다.', 'unlisted', 'notion', 10, 0,
	              datetime('now'), datetime('now'))`)
	exec(`UPDATE categories SET cover_post_id = 10 WHERE id = 10`)

	srv, err := New(sqlDB)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rec := get(t, srv.Handler(), "/intro")
	if rec.Code != http.StatusOK {
		t.Fatalf("상태 코드 %d", rec.Code)
	}
	body := mainOf(t, rec.Body.String())
	if strings.Contains(body, "inryeolchoi.github.io") {
		t.Errorf("없앤 개인 페이지 카드가 아직 그려진다:\n%s", body)
	}
	// 하위 분류가 없으니 그 섹션도 없어야 한다.
	if strings.Contains(body, "하위 분류") {
		t.Errorf("하위 분류가 없는데 섹션이 그려졌다:\n%s", body)
	}
}

// 표에 적으면 그 분류에만 붙는다. 장치가 살아 있는지 보는 자리다.
func TestSiteLinksAppearOnlyWhereTheTableSaysSo(t *testing.T) {
	sqlDB := testDB(t)
	exec := execer(t, sqlDB)

	exec(`INSERT INTO categories (id, name, slug, sort_order) VALUES (10, '소개', 'intro', 0)`)
	exec(`INSERT INTO categories (id, name, slug, sort_order) VALUES (11, '개발', 'dev', 1)`)
	exec(`INSERT INTO posts (id, slug, title, body, status, source, category_id, sort_order, created_at, updated_at)
	      VALUES (10, 'about-me', '최인렬', '소개 본문이다.', 'unlisted', 'notion', 10, 0,
	              datetime('now'), datetime('now'))`)
	exec(`UPDATE categories SET cover_post_id = 10 WHERE id = 10`)

	categoryLinks["intro"] = []SiteLink{{
		Title: "보기용 링크", Host: "example.com", URL: "https://example.com",
		I18n: "personalSite", Icon: globeIcon,
	}}
	t.Cleanup(func() { delete(categoryLinks, "intro") })

	srv, err := New(sqlDB)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	h := srv.Handler()

	body := mainOf(t, get(t, h, "/intro").Body.String())
	if !strings.Contains(body, `href="https://example.com"`) {
		t.Errorf("표에 적은 바깥 링크가 안 그려졌다:\n%s", body)
	}
	// 아카이브 밖으로 나가는 링크라 우리 주소를 상대 쪽 로그에 남기지 않는다.
	if !strings.Contains(body, `rel="noreferrer"`) {
		t.Errorf("바깥 링크에 rel=noreferrer가 없다:\n%s", body)
	}
	// 표에 없는 분류에는 안 붙는다.
	if other := mainOf(t, get(t, h, "/dev").Body.String()); strings.Contains(other, "example.com") {
		t.Errorf("표에 없는 분류에도 바깥 링크가 붙었다:\n%s", other)
	}
}

// 링크 글자는 preferences.js의 고정 사전이 세 언어로 바꾼다. 사전에 없는 키를
// 적으면 언어를 바꿔도 한국어 그대로 남는데, 조용히 그렇게 되는 것을 막는다.
func TestSiteLinkI18nKeysExistInDictionary(t *testing.T) {
	dict, err := staticFS.ReadFile("static/preferences.js")
	if err != nil {
		t.Fatalf("preferences.js 읽기: %v", err)
	}
	for slug, links := range categoryLinks {
		for _, link := range links {
			if link.I18n == "" {
				t.Errorf("%s의 바깥 링크에 사전 키가 없다: %s", slug, link.Title)
				continue
			}
			if strings.Count(string(dict), link.I18n+":") < 3 {
				t.Errorf("사전에 %q가 세 언어로 다 있지 않다", link.I18n)
			}
		}
	}
}
