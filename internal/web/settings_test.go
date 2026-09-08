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

// **홈에 자유 본문을 쓴다**(2026-09-09). 예전에는 문구 네 줄만 고칠 수 있어서,
// 그 밖의 것을 넣으려면 템플릿을 고쳐 배포해야 했다 — 홈에 무엇을 둘지는
// 사람이 정할 일이지 코드가 정할 일이 아니다.
func TestHomeBodyIsMarkdownLikeAnyPost(t *testing.T) {
	sqlDB := testDB(t)
	exec := execer(t, sqlDB)
	exec(`INSERT INTO settings (key, value) VALUES ('home.body',
	      '## 여기서 쓴 절

- 목록도 된다

식 $x^2$ 도 된다.')`)

	srv, err := New(sqlDB)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	page := get(t, srv.Handler(), "/").Body.String()
	body := mainOf(t, page)
	// **글과 같은 렌더러로 그린다.** `##`이 h3이 되는 것이 그 증거다
	// (markdown.headingShift) — 흉내였다면 h2가 나왔을 것이다.
	if !strings.Contains(body, "<h3") {
		t.Errorf("홈 본문이 안 그려졌다:\n%s", body)
	}
	if !strings.Contains(body, "<li>목록도 된다</li>") {
		t.Errorf("목록이 마크다운으로 안 그려졌다:\n%s", body)
	}
	if !strings.Contains(body, `class="math math-inline"`) {
		t.Errorf("수식이 안 잡혔다:\n%s", body)
	}
	// 수식이 있으면 KaTeX도 함께 실려야 한다 — 글과 같은 판정이다.
	if !strings.Contains(page, "npm/katex@") {
		t.Errorf("홈 본문에 수식이 있는데 KaTeX가 안 실렸다")
	}
}

// 최근 글 수는 사람이 정한다. **0은 "안 보인다"라는 뜻이 있는 값**이라
// 빈 값(안 정함)과 구별해야 한다 — 문구들과 반대다.
func TestHomeRecentCountIsSettable(t *testing.T) {
	sqlDB := testDB(t)
	exec := execer(t, sqlDB)
	now := "datetime('now')"
	exec(`INSERT INTO categories (id, name, slug, sort_order) VALUES (1, '개발', 'dev', 0)`)
	for i := 1; i <= 3; i++ {
		exec(`INSERT INTO posts (slug, title, body, status, source, category_id, sort_order,
		                         original_created_at, created_at, updated_at)
		      VALUES (?, ?, '본문', 'unlisted', 'notion', 1, 0, ?, `+now+`, `+now+`)`,
			"p"+itoa(i), "글 "+itoa(i), "2026-01-0"+itoa(i))
	}

	srv, err := New(sqlDB)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	h := srv.Handler()
	if body := mainOf(t, get(t, h, "/").Body.String()); !strings.Contains(body, "최근에 쓴 글") {
		t.Errorf("기본값으로는 최근 글이 나와야 한다:\n%s", body)
	}

	exec(`INSERT INTO settings (key, value) VALUES ('home.recent', '0')`)
	if body := mainOf(t, get(t, h, "/").Body.String()); strings.Contains(body, "최근에 쓴 글") {
		t.Errorf("0으로 정했는데 최근 글 절이 남았다:\n%s", body)
	}
}
