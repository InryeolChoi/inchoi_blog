package web

import (
	"net/http"
	"strings"
	"testing"
)

// 이 파일이 지키는 것은 하나다: **고치는 길은 기본으로 닫혀 있고, 로그인이
// 확인된 요청에만 열린다.**
//
// web은 "읽기 전용 공개 페이지"라고 못박아 둔 패키지다. 여기에 쓰기로 가는
// 문이 생겼으므로, 그 문이 실수로 열리는 일이 없다는 것을 코드가 아니라
// 테스트가 지켜야 한다.

const editMarks = "고치기 버튼·편집기 스크립트"

func hasEditor(body string) bool {
	return strings.Contains(body, "data-inline-edit=") ||
		strings.Contains(body, "/static/inline-edit.js")
}

// **옵션을 안 주면 아무것도 안 나간다.** `-admin` 없이 뜬 서버가 지금까지의
// 배포이고, 거기서 글 화면이 한 바이트도 달라지면 안 된다.
func TestEditorIsAbsentWithoutTheOption(t *testing.T) {
	h := handlerFor(t, seedTestDB(t))
	for _, path := range []string{"/", "/p/list-post", "/dev"} {
		body := get(t, h, path).Body.String()
		if hasEditor(body) {
			t.Errorf("%s: 옵션을 안 줬는데 %s가 나갔다", path, editMarks)
		}
	}
}

// 옵션을 줘도 **로그인이 확인되지 않으면 안 나간다.** 남이 읽는 화면에
// 편집기가 실려 나가면, 그 자체가 "여기 관리 화면이 있다"는 신호가 된다.
func TestEditorIsAbsentForAnonymousReaders(t *testing.T) {
	h := handlerFor(t, seedTestDB(t), WithEditor(func(*http.Request) string { return "" }))
	body := get(t, h, "/p/list-post").Body.String()
	if hasEditor(body) {
		t.Errorf("로그인하지 않은 요청에 %s가 나갔다", editMarks)
	}
}

// 로그인이 확인되면 그때 나온다. 앞의 두 테스트가 "언제나 안 나온다"로
// 통과하는 것을 막는다.
func TestEditorAppearsForTheLoggedInWriter(t *testing.T) {
	h := handlerFor(t, seedTestDB(t), WithEditor(func(*http.Request) string { return "InryeolChoi" }))
	body := get(t, h, "/p/list-post").Body.String()
	if !strings.Contains(body, `data-inline-edit="list-post"`) {
		t.Error("고치기 자리가 없다")
	}
	if !strings.Contains(body, "/static/inline-edit.js") {
		t.Error("편집기 스크립트가 안 실렸다")
	}
	// 팔레트도 같이 실려야 한다. admin 편집기와 같은 조각 목록을 쓴다.
	if !strings.Contains(body, "/static/palette.js") {
		t.Error("팔레트가 안 실렸다")
	}
}

// **요청마다 다시 묻는다.** 한 번 로그인한 것을 서버가 기억해두고 그 뒤로
// 계속 열어두면, 세션이 풀린 뒤에도 화면이 열린 채로 남는다.
func TestEditorIsDecidedPerRequest(t *testing.T) {
	calls := 0
	logged := true
	h := handlerFor(t, seedTestDB(t), WithEditor(func(*http.Request) string {
		calls++
		if logged {
			return "InryeolChoi"
		}
		return ""
	}))

	if !hasEditor(get(t, h, "/p/list-post").Body.String()) {
		t.Fatal("로그인 상태인데 안 나온다")
	}
	logged = false
	if hasEditor(get(t, h, "/p/list-post").Body.String()) {
		t.Error("세션이 풀렸는데도 편집기가 나간다")
	}
	if calls < 2 {
		t.Errorf("물어본 횟수가 %d다. 요청마다 다시 물어야 한다", calls)
	}
}

// **카테고리의 표지 글도 고칠 수 있어야 한다.** 표지는 본문을 카테고리
// 화면에 그대로 펼치는데, 지금까지는 그 자리가 post.html이 아니라서
// 로그인해도 고치기 버튼이 없었다. category.html도 같은 슬롯을 낸다.
func TestEditorAppearsOnCategoryCovers(t *testing.T) {
	h := handlerFor(t, seedTestDB(t), WithEditor(func(*http.Request) string { return "InryeolChoi" }))
	body := get(t, h, "/dev/language").Body.String()
	if !strings.Contains(body, `data-inline-edit="cover-language"`) {
		t.Error("표지 글의 고치기 자리가 없다")
	}
}

// 표지가 없는 카테고리에는 고칠 본문이 없다. **스크립트 자체는 로그인
// 세션마다 실리지만**(post.html도 마찬가지다), 자리(`data-inline-edit`)는
// 안 나가야 한다 — 나가면 누른 버튼이 가리킬 글이 없다.
func TestEditorIsAbsentWithoutACoverBody(t *testing.T) {
	h := handlerFor(t, seedTestDB(t), WithEditor(func(*http.Request) string { return "InryeolChoi" }))
	body := get(t, h, "/dev").Body.String()
	if strings.Contains(body, "data-inline-edit=") {
		t.Error("표지 본문이 없는 카테고리에 고치기 자리가 나갔다")
	}
}

// 편집기가 열려도 **draft를 가리는 규칙은 그대로다.** 로그인은 "고칠 수
// 있다"는 뜻이지 "공개 화면의 규칙이 달라진다"는 뜻이 아니다. 그 둘이 섞이면
// 어느 화면이 무엇을 보여주는지 아무도 모르게 된다.
func TestEditorDoesNotUnhideDrafts(t *testing.T) {
	h := handlerFor(t, seedTestDB(t), WithEditor(func(*http.Request) string { return "InryeolChoi" }))
	if rec := get(t, h, "/p/draft-post"); rec.Code != http.StatusNotFound {
		t.Errorf("로그인했더니 draft가 %d로 보인다. 404여야 한다", rec.Code)
	}
}

// ── 새 글로 가는 길 ──────────────────────────────────────────────
//
// **분류 화면에만 둔다.** 이 버튼이 사이드바의 admin 링크보다 더 아는 것은
// "이 분류" 하나뿐이라, 넘길 분류가 없는 곳(홈·글 상세)에서는 같은 곳으로
// 가는 길을 한 벌 더 그리는 것이 된다.

// 로그인하지 않았으면 **주소 자체를 안 만든다.** 화면이 권한을 판단하지
// 않는다 — 버튼을 감추는 것이 아니라 서버가 길을 안 낸다.
//
// **여기는 관문이 두 겹이다.** server.go가 URL을 안 만들고, pagetools도
// `.Editor`를 다시 본다. 그래서 서버 쪽 조건을 지워도 이 테스트는 통과한다 —
// 템플릿이 막기 때문이다. 돌연변이로 확인한 사실이고, 구멍이 아니라 두 겹인
// 것이다(트랜잭션의 "롤백을 아예 안 함"이 안 잡히는 것과 같은 성질).
// **한 겹으로 줄이면 이 테스트가 그때부터 진짜로 지킨다.**
func TestNewPostLinkIsAbsentForAnonymousReaders(t *testing.T) {
	h := handlerFor(t, seedTestDB(t), WithEditor(func(*http.Request) string { return "" }))
	for _, path := range []string{"/", "/dev", "/dev/language", "/p/list-post"} {
		if body := get(t, h, path).Body.String(); strings.Contains(body, "/admin/new") {
			t.Errorf("%s: 로그인하지 않았는데 새 글 링크가 나갔다", path)
		}
	}
}

// 옵션을 아예 안 준 서버에서는 더더욱 없다. `-admin` 없이 뜬 배포가 그렇다.
func TestNewPostLinkIsAbsentWithoutTheOption(t *testing.T) {
	h := handlerFor(t, seedTestDB(t))
	if body := get(t, h, "/dev").Body.String(); strings.Contains(body, "/admin/new") {
		t.Error("옵션을 안 줬는데 새 글 링크가 나갔다")
	}
}

// 로그인하면 분류 화면에 나온다. **표지도 글도 없는 분류에도 나와야 한다** —
// 거기가 원래 길이 통째로 없던 자리다.
func TestNewPostLinkAppearsOnCategoryPages(t *testing.T) {
	h := handlerFor(t, seedTestDB(t), WithEditor(func(*http.Request) string { return "InryeolChoi" }))
	for _, path := range []string{"/dev", "/dev/language"} {
		body := get(t, h, path).Body.String()
		if !strings.Contains(body, `class="new-here"`) {
			t.Errorf("%s: 새 글 링크가 없다", path)
		}
		// 한 화면에 한 벌이다. 사이드바·상단 바에도 두면 같은 버튼이 두 번 나온다.
		if n := strings.Count(body, `class="new-here"`); n != 1 {
			t.Errorf("%s: 새 글 링크가 %d개다. 한 벌이어야 한다", path, n)
		}
	}
}

// **넘길 분류가 없는 화면에는 안 나온다.** 홈과 글 상세가 그렇다.
// 글 상세는 activeCat이 차 있어도(사이드바를 펼치느라) 분류 화면이 아니다 —
// 그 둘을 가르지 못하면 이 규칙이 조용히 무너진다.
func TestNewPostLinkIsAbsentWhereThereIsNoCategoryToCarry(t *testing.T) {
	h := handlerFor(t, seedTestDB(t), WithEditor(func(*http.Request) string { return "InryeolChoi" }))
	for _, path := range []string{"/", "/p/list-post"} {
		if body := get(t, h, path).Body.String(); strings.Contains(body, "/admin/new") {
			t.Errorf("%s: 넘길 분류가 없는데 새 글 링크가 나갔다", path)
		}
	}
}

// **분류를 미리 골라 넘긴다.** 안 그러면 방금 보던 것을 잊고 admin에서
// 분류를 처음부터 다시 고르게 된다.
func TestNewPostLinkCarriesTheCategory(t *testing.T) {
	h := handlerFor(t, seedTestDB(t), WithEditor(func(*http.Request) string { return "InryeolChoi" }))
	body := get(t, h, "/dev").Body.String()
	if !strings.Contains(body, "/admin/new?category=") {
		t.Error("분류 화면인데 분류를 안 넘긴다")
	}
}

// 글 상세에는 `편집`이 대신 있다. 새 글이 빠졌다고 손질할 길까지 사라지면 안 된다.
func TestEditMenuStillStandsOnPosts(t *testing.T) {
	h := handlerFor(t, seedTestDB(t), WithEditor(func(*http.Request) string { return "InryeolChoi" }))
	body := get(t, h, "/p/list-post").Body.String()
	if !strings.Contains(body, `class="edit-menu"`) {
		t.Error("글 상세에 편집 커튼이 없다")
	}
	if !strings.Contains(body, `data-inline-edit="list-post"`) {
		t.Error("고칠 대상이 안 실렸다")
	}
}

// **로그인해야 그 버튼이 나온다.** 이 조건을 화면(HTML)으로만 확인하면
// pagetools의 `.Editor` 검사가 한 겹 더 막아줘서, 서버가 로그인 여부를
// 빼먹어도 테스트가 통과한다. 실제로 그 돌연변이가 안 잡혔다.
// 그래서 판정을 직접 겨냥한다.
func TestNewPostURLNeedsBothLoginAndCategory(t *testing.T) {
	for _, c := range []struct {
		name   string
		editor string
		cat    int64
		want   string
	}{
		{"로그인 + 분류", "InryeolChoi", 12, "/admin/new?category=12"},
		{"로그인했지만 넘길 분류가 없다", "InryeolChoi", 0, ""},
		{"분류는 있지만 로그인하지 않았다", "", 12, ""},
		{"둘 다 없다", "", 0, ""},
	} {
		if got := newPostURL(c.editor, c.cat); got != c.want {
			t.Errorf("%s: newPostURL(%q, %d) = %q, 원하는 값 %q", c.name, c.editor, c.cat, got, c.want)
		}
	}
}

// 고칠 수 있는 화면에서는 **수식이 없어도** KaTeX를 받는다.
//
// 편집기는 작업 도구라 무엇을 칠지 미리 알 수 없다 — admin이 늘 받는 것과 같은
// 판단이다. 이게 없으면 수식 하나 없던 글에 수식을 쓸 때 미리보기도, 커서 위의
// 수식 상자도 아무것도 안 그린다.
//
// **읽는 사람에게는 안 나간다.** 그게 이 테스트의 나머지 절반이다 — 자산을
// 페이지별로 가른 이유가 통째로 무의미해지면 안 된다.
func TestEditableScreensLoadTheMathTools(t *testing.T) {
	sqlDB := seedTestDB(t)
	editor := handlerFor(t, sqlDB, WithEditor(func(*http.Request) string { return "InryeolChoi" }))
	reader := handlerFor(t, sqlDB)

	// 본문에 수식이 없는 글이다.
	const path = "/p/category-post"
	page := get(t, editor, path).Body.String()
	if !strings.Contains(page, "npm/katex@") {
		t.Errorf("고칠 수 있는 화면인데 KaTeX가 안 실렸다:\n%s", page)
	}
	if !strings.Contains(page, `src="/static/math-live.js"`) {
		t.Errorf("커서 위 수식 상자 스크립트가 안 실렸다:\n%s", page)
	}
	if got := get(t, reader, path).Body.String(); strings.Contains(got, "npm/katex@") {
		t.Errorf("읽는 사람에게도 KaTeX가 나갔다. 페이지별로 가른 것이 무의미해진다")
	}
	if got := get(t, reader, path).Body.String(); strings.Contains(got, "math-live.js") {
		t.Errorf("읽는 사람에게 편집 도구가 나갔다")
	}
}
