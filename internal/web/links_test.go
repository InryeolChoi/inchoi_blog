package web

import (
	"net/http"
	"strings"
	"testing"
)

// 소개 화면은 자기소개 본문 아래에 개인 페이지로 나가는 길을 둔다.
//
// 예전에는 이 자리에 "하위 분류 > 최인렬 (Inryeol Choi) 글 1건"이 있었다. 그
// 갈래에는 지금 펼쳐놓은 바로 그 글 한 편뿐이라 눌러도 제자리로 돌아왔다.
// 자기소개를 읽은 사람이 다음으로 갈 곳은 아카이브의 다른 분류가 아니다.
func TestIntroLinksToPersonalSite(t *testing.T) {
	sqlDB := testDB(t)
	exec := execer(t, sqlDB)

	exec(`INSERT INTO categories (id, name, slug, sort_order) VALUES (10, '소개', 'intro', 0)`)
	exec(`INSERT INTO categories (id, name, slug, sort_order) VALUES (11, '개발', 'dev', 1)`)
	exec(`INSERT INTO posts (id, slug, title, body, status, source, category_id, sort_order, created_at, updated_at)
	      VALUES (10, 'about-me', '최인렬', '늘 우직하게 도전하는 개발자입니다.', 'unlisted', 'notion', 10, 0,
	              datetime('now'), datetime('now'))`)
	exec(`UPDATE categories SET cover_post_id = 10 WHERE id = 10`)

	srv, err := New(sqlDB)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	h := srv.Handler()

	rec := get(t, h, "/intro")
	if rec.Code != http.StatusOK {
		t.Fatalf("상태 코드 %d", rec.Code)
	}
	body := mainOf(t, rec.Body.String())
	if !strings.Contains(body, `href="https://inryeolchoi.github.io"`) {
		t.Errorf("소개 화면에 개인 페이지 링크가 없다:\n%s", body)
	}
	// 아카이브 밖으로 나가는 링크라 우리 주소를 상대 쪽 로그에 남기지 않는다.
	if !strings.Contains(body, `rel="noreferrer"`) {
		t.Errorf("바깥 링크에 rel=noreferrer가 없다:\n%s", body)
	}
	// 하위 분류가 없으니 그 섹션도 없어야 한다.
	if strings.Contains(body, "하위 분류") {
		t.Errorf("하위 분류가 없는데 섹션이 그려졌다:\n%s", body)
	}

	// 다른 분류에는 붙지 않는다. 이건 소개 화면 하나를 위한 장치다.
	if other := mainOf(t, get(t, h, "/dev").Body.String()); strings.Contains(other, "inryeolchoi.github.io") {
		t.Errorf("소개가 아닌 분류에도 개인 페이지 링크가 붙었다:\n%s", other)
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
