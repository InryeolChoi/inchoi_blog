package web

import (
	"database/sql"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// 여기서 지키는 것은 하나다: **본문이 안 가리키는 아래 갈래에도 길이 있어야
// 한다.** `Java` 표지 글이 노션에서 온 목차만 들고 있고 그 아래 `모던 자바`
// 16편은 GitHub에서 나중에 들어온 것이라, 표지가 그 존재를 알 리가 없었다.
// `프로그래밍 언어`는 카드만 보여주므로 주소를 직접 치는 것 말고는 닿는 길이
// 없었다 — 그것이 이 기능이 막는 일이다.
func subBranchDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB := testDB(t)
	exec := execer(t, sqlDB)
	exec(`INSERT INTO categories (id, name, slug, sort_order) VALUES (1, '언어', 'lang', 0)`)

	now := time.Now().UTC()
	post := func(id int, slug, title, body, path string) {
		exec(`INSERT INTO posts (id, slug, title, body, status, source, category_id, sort_order, original_path, created_at, updated_at)
		      VALUES (?, ?, ?, ?, 'unlisted', 'notion', 1, 0, ?, ?, ?)`,
			id, slug, title, body, path, now, now)
	}
	// 표지 본문은 `문자열`만 가리킨다. `모던 자바`는 한 번도 안 나온다.
	post(1, "java", "Java", "목차\n\n[문자열](/p/strings)\n", "Java")
	post(2, "strings", "문자열", "문자열 본문", "Java > 문자열")
	post(3, "strings-a", "charAt", "본문", "Java > 문자열 > charAt")
	post(4, "modern-1", "모던 1", "본문", "Java > 모던 자바 > 모던 1")
	post(5, "modern-2", "모던 2", "본문", "Java > 모던 자바 > 모던 2")
	// 갈래가 아니라 직속 자식 한 편인 것. 층이 아니라 글 하나라 상자가 되면 안 된다.
	post(6, "alone", "혼자", "본문", "Java > 혼자")
	// 이름 없는 인라인 데이터베이스는 상자 제목이 될 수 없다.
	post(7, "nameless-1", "이름없음 1", "본문", "Java > (제목 없음) > 이름없음 1")
	post(8, "nameless-2", "이름없음 2", "본문", "Java > (제목 없음) > 이름없음 2")
	return sqlDB
}

func TestPostShowsBranchesTheBodyNeverLinks(t *testing.T) {
	h := handlerFor(t, subBranchDB(t))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/p/java", nil))
	if rec.Code != 200 {
		t.Fatalf("Java 글이 %d", rec.Code)
	}
	body := mainOf(t, rec.Body.String())
	tail := body[strings.Index(body, "</article>"):]

	if !strings.Contains(tail, `inline-db-title">모던 자바<`) {
		t.Error("본문이 안 가리키는 `모던 자바` 갈래가 상자로 안 나온다")
	}
	for _, slug := range []string{"/p/modern-1", "/p/modern-2"} {
		if !strings.Contains(tail, slug) {
			t.Errorf("%s로 가는 길이 없다", slug)
		}
	}
	// 본문이 이미 안내한 갈래를 또 그리면 한 화면에 같은 목록이 두 번 난다.
	if strings.Contains(tail, `inline-db-title">문자열<`) {
		t.Error("본문이 이미 가리키는 `문자열` 갈래를 또 그렸다")
	}
	// 층이 아니라 글 하나인 것에 이름표를 씌우지 않는다.
	if strings.Contains(tail, `inline-db-title">혼자<`) {
		t.Error("한 편짜리를 갈래 상자로 세웠다")
	}
	// 이름 없는 갈래는 제목으로 쓸 글자가 없다.
	if strings.Contains(tail, "(제목 없음)") {
		t.Error("이름 없는 갈래를 상자로 세웠다")
	}
}
