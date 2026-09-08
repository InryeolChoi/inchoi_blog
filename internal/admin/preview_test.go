package admin

import (
	"net/http"
	"strings"
	"testing"
)

// 블록 편집기가 볼 것: 문단마다 **원문 조각과 그린 결과가 짝**이어야 한다.
func TestBlocksPairSourceWithItsRender(t *testing.T) {
	h := testHandler(t)
	rec := do(t, h, http.MethodPost, "/api/admin/blocks",
		`{"markdown":"## 제목\n\n문단이다.\n\n` + "```" + `py\nx = 1\n\ny = 2\n` + "```" + `"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("상태 코드 %d: %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Blocks []struct{ Src, HTML string } `json:"blocks"`
	}
	decode(t, rec, &got)
	if len(got.Blocks) != 3 {
		t.Fatalf("블록이 %d개다. 제목·문단·코드 셋이어야 한다: %+v", len(got.Blocks), got.Blocks)
	}
	// **코드 블록 안의 빈 줄에서 안 잘린다.** 잘리면 반쪽짜리 코드 두 개가 되고,
	// 여는 백틱만 든 조각은 그 자체로 마크다운이 아니다.
	if !strings.Contains(got.Blocks[2].Src, "y = 2") {
		t.Errorf("코드 블록이 쪼개졌다: %q", got.Blocks[2].Src)
	}
	// 그린 것은 발행될 바로 그 HTML이다 — 코드 라벨까지 붙어 있어야 한다.
	if !strings.Contains(got.Blocks[2].HTML, `class="lang"`) {
		t.Errorf("코드 라벨이 없다. 공개 화면과 다르게 그렸다는 뜻이다: %q", got.Blocks[2].HTML)
	}
	// **`##`이 h3으로 나온다.** 본문 제목은 한 단계 내려 그리기 때문이고
	// (markdown.headingShift), 그 사실 자체가 여기서 공개 화면과 같은 렌더러를
	// 쓴다는 증거다 — 흉내였다면 h2가 나왔을 것이다.
	if !strings.Contains(got.Blocks[0].HTML, "<h3") {
		t.Errorf("제목이 h3으로 안 나왔다. 공개 화면과 다르게 그렸다: %q", got.Blocks[0].HTML)
	}
}
