package web

import (
	"database/sql"
	"net/http"
	"strings"
	"testing"
)

// 이 파일이 지키는 것은 하나다: **비공개 글은 허용된 계정에게만 보인다.**
//
// draft를 가리는 것과 축이 다르다. draft는 "아직 안 쓴 자리"라 서버 전체
// 스위치(`-drafts`)로 갈리지만, 비공개는 **요청마다** 갈린다. 그래서
// 새는 자리도 다르다 — 목록·카운트·본문 링크·사이트맵이 각각 구멍이 될 수 있다.

// privateSeed는 기본 트리에 비공개 글 한 편을 더한다.
//
// **`section-post` 본문이 그 글을 가리키게 한다.** 링크를 푸는 자리
// (`unlinkHidden`)까지 함께 확인해야 하기 때문이다.
func privateSeed(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB := seedTestDB(t)
	exec := execer(t, sqlDB)
	exec(`INSERT INTO posts (id, slug, title, body, status, visibility, source,
	                         category_id, sort_order, created_at, updated_at)
	      VALUES (20, 'secret-post', '숨긴 글', '비밀 본문', 'unlisted', 'private', 'notion', 3, 5,
	              datetime('now'), datetime('now'))`)
	exec(`UPDATE posts SET body = '묶음 본문' || char(10) || char(10) || '[숨긴 글](/p/secret-post)' WHERE id = 3`)
	return sqlDB
}

func member(t *testing.T) http.Handler {
	t.Helper()
	return handlerFor(t, privateSeed(t), WithEditor(func(*http.Request) string { return "InryeolChoi" }))
}

func stranger(t *testing.T) http.Handler {
	t.Helper()
	return handlerFor(t, privateSeed(t), WithEditor(func(*http.Request) string { return "" }))
}

// **주소를 알아도 못 본다.** 여기가 뚫리면 나머지 검사는 뜻이 없다.
func TestPrivatePostIs404ForStrangers(t *testing.T) {
	if got := get(t, stranger(t), "/p/secret-post").Code; got != http.StatusNotFound {
		t.Errorf("남에게 %d다, 404여야 한다", got)
	}
	res := get(t, member(t), "/p/secret-post")
	if res.Code != http.StatusOK {
		t.Fatalf("들어와 있는 사람에게 %d다, 200이어야 한다", res.Code)
	}
	if !strings.Contains(res.Body.String(), "비밀 본문") {
		t.Error("허용된 계정인데 본문이 안 나온다")
	}
	// 평소와 똑같이 보이면 이 글이 비공개라는 것을 알 길이 없다.
	if !strings.Contains(res.Body.String(), `data-i18n="private"`) {
		t.Error("비공개 표시가 없다")
	}
}

// **옵션이 없으면 아무에게도 안 보인다.** `-admin` 없이 뜬 배포에서 실수로
// 열리는 방향이 아니라 막히는 방향으로 틀린다.
func TestPrivatePostNeedsTheEditorOption(t *testing.T) {
	h := handlerFor(t, privateSeed(t))
	if got := get(t, h, "/p/secret-post").Code; got != http.StatusNotFound {
		t.Errorf("옵션 없는 서버가 %d를 줬다, 404여야 한다", got)
	}
}

// 목록·카운트·본문 링크 어디에도 안 나온다. draft를 가릴 때 함께 처리했던
// 자리들이고, 축이 하나 늘었다고 그중 하나가 빠지면 안 된다.
func TestPrivatePostLeaksNowhereForStrangers(t *testing.T) {
	h := stranger(t)
	for _, path := range []string{"/", "/dev", "/dev/language", "/dev/language/python", "/p/section-post"} {
		body := get(t, h, path).Body.String()
		if strings.Contains(body, "/p/secret-post") {
			t.Errorf("%s에 비공개 글로 가는 링크가 있다", path)
		}
	}
	// 링크는 풀되 **글자는 남긴다.** 줄째 지우면 문장이 끊긴다 — unlinkHidden과 같다.
	body := mainOf(t, get(t, h, "/p/section-post").Body.String())
	if !strings.Contains(body, "숨긴 글") {
		t.Error("링크를 풀면서 글자까지 지웠다")
	}
}

// 사이트맵은 **크롤러에게 주소를 알려주는 것이 곧 공개**다. 여기서 새면
// 목록에서 가린 것이 통째로 무의미해진다.
func TestSitemapNeverLeaksPrivatePosts(t *testing.T) {
	for _, h := range []http.Handler{stranger(t), member(t)} {
		body := get(t, h, "/sitemap.xml").Body.String()
		if strings.Contains(body, "secret-post") {
			t.Error("사이트맵에 비공개 글이 있다")
		}
	}
}

// **`-drafts`가 비공개까지 열어주지 않는다.** 두 축이 서로를 덮으면
// 로컬 확인용 스위치 하나가 공개 배포의 규칙을 바꾸게 된다.
func TestDraftsSwitchDoesNotOpenPrivatePosts(t *testing.T) {
	h := handlerFor(t, privateSeed(t), WithDrafts())
	if got := get(t, h, "/p/secret-post").Code; got != http.StatusNotFound {
		t.Errorf("-drafts만 켠 서버가 %d를 줬다, 404여야 한다", got)
	}
}
