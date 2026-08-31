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

// 편집기가 열려도 **draft를 가리는 규칙은 그대로다.** 로그인은 "고칠 수
// 있다"는 뜻이지 "공개 화면의 규칙이 달라진다"는 뜻이 아니다. 그 둘이 섞이면
// 어느 화면이 무엇을 보여주는지 아무도 모르게 된다.
func TestEditorDoesNotUnhideDrafts(t *testing.T) {
	h := handlerFor(t, seedTestDB(t), WithEditor(func(*http.Request) string { return "InryeolChoi" }))
	if rec := get(t, h, "/p/draft-post"); rec.Code != http.StatusNotFound {
		t.Errorf("로그인했더니 draft가 %d로 보인다. 404여야 한다", rec.Code)
	}
}
