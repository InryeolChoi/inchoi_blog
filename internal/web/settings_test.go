package web

import (
	"net/http"
	"strings"
	"testing"
)

// 홈 표제지 문구는 DB가 정본이다(migrations/008).
func TestHomeCopyComesFromTheDatabase(t *testing.T) {
	sqlDB := testDB(t)
	exec := execer(t, sqlDB)

	srv, err := New(sqlDB)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	h := srv.Handler()

	// 저장된 값이 없으면 코드의 기본값이다. **DB에 미리 안 넣기 때문에**
	// 처음 뜨는 사이트도 문장이 있다.
	body := mainOf(t, get(t, h, "/").Body.String())
	if !strings.Contains(body, defaultHomeText.TitleTop) {
		t.Errorf("기본 문구가 안 나온다:\n%s", body)
	}
	// 기본값일 때는 사전이 언어를 바꿔준다.
	if !strings.Contains(body, `data-i18n="homeKicker"`) {
		t.Errorf("기본 눈썹줄에 사전 키가 없다:\n%s", body)
	}

	exec(`INSERT INTO settings (key, value) VALUES ('home.title_top', '내가 적은 첫 줄')`)
	exec(`INSERT INTO settings (key, value) VALUES ('home.kicker', '내가 적은 눈썹줄')`)
	body = mainOf(t, get(t, h, "/").Body.String())
	if !strings.Contains(body, "내가 적은 첫 줄") {
		t.Errorf("저장한 문구가 안 나온다:\n%s", body)
	}
	if strings.Contains(body, defaultHomeText.TitleTop) {
		t.Errorf("기본 문구가 아직 남아 있다:\n%s", body)
	}
	// **사람이 고쳤으면 사전 키를 안 붙인다.** 그 속성이 있으면 preferences.js가
	// 언어를 바꿀 때 textContent를 통째로 갈아치워서 방금 적은 문장이 사라진다.
	if strings.Contains(body, `data-i18n="homeKicker"`) {
		t.Errorf("사람이 적은 눈썹줄에 사전 키가 붙었다. 언어를 바꾸면 조용히 사라진다:\n%s", body)
	}
}

// 표제지 문구는 그대로 이스케이프돼야 한다. 본문 렌더러가 html.WithUnsafe()라
// **임의 태그를 받는 자리를 늘리지 않는다** — 여기는 마크다운도 아니다.
func TestHomeCopyIsEscaped(t *testing.T) {
	sqlDB := testDB(t)
	exec := execer(t, sqlDB)
	exec(`INSERT INTO settings (key, value) VALUES ('home.lead', '<script>alert(1)</script>')`)

	srv, err := New(sqlDB)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	page := get(t, srv.Handler(), "/").Body.String()
	if page == "" || strings.Contains(page, "<script>alert(1)</script>") {
		t.Errorf("설정 문구가 태그로 나갔다:\n%s", mainOf(t, page))
	}
}

// 상태 코드까지 확인한다. 설정 조회가 홈을 통째로 500으로 만들면 안 된다.
func TestHomeStillWorksWithSettings(t *testing.T) {
	sqlDB := testDB(t)
	srv, err := New(sqlDB)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if rec := get(t, srv.Handler(), "/"); rec.Code != http.StatusOK {
		t.Fatalf("홈 상태 코드 %d", rec.Code)
	}
}
