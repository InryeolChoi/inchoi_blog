package markdown

import (
	"strings"
	"testing"
)

// 본문에 애니메이션을 넣는 길은 이름 하나뿐이다. **본문에 <script>를 담는
// 길은 열지 않는다** — 렌더러가 html.WithUnsafe()라 그건 이미 실행된다.
func TestAnimTakesOnlyANameNeverScript(t *testing.T) {
	r := New()
	for _, tc := range []struct {
		name string
		in   string
		want string   // 결과에 있어야 하는 것
		not  []string // 있으면 안 되는 것
	}{
		{
			"이름 하나", ":::anim sort-bubble\n",
			`<div class="anim" data-anim="sort-bubble">`, nil,
		},
		{
			"문단 사이", "앞말\n\n:::anim binary-search\n\n뒷말\n",
			`data-anim="binary-search"`, nil,
		},
		{
			// **모르는 모양이면 아예 안 가져간다.** 그러면 그 줄이 평범한
			// 문단으로 남아 글자 그대로 보인다 — 조용히 사라지는 것보다 낫다.
			"띄어쓰기가 들어간 이름", ":::anim bad name\n",
			":::anim bad name", []string{`class="anim"`},
		},
		{
			// 이름에 태그를 넣으려 하면 **지시로 안 받아준다.** 그 줄은 평범한
			// 문단이 되고, 문단 안의 raw HTML이 통과하는 것은 이 렌더러가
			// html.WithUnsafe()이기 때문이지 여기서 생긴 구멍이 아니다
			// (본문은 사람이 쓰는 것이라는 전제다 — render.go 참고).
			"태그를 이름에 넣으려 함", ":::anim <script>alert(1)</script>\n",
			":::anim", []string{`class="anim"`, `data-anim`},
		},
		{
			"대문자와 기호", ":::anim Sort_Bubble\n",
			":::anim Sort_Bubble", []string{`class="anim"`},
		},
		{
			// 문장 속에 있으면 지시가 아니다. 줄 처음에서만 시작한다.
			"문장 속", "글 안에서 :::anim sort-bubble 이렇게 쓰면\n",
			":::anim sort-bubble", []string{`class="anim"`},
		},
	} {
		got, err := r.Render(tc.in)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if !strings.Contains(string(got), tc.want) {
			t.Errorf("%s: %q가 없다\n%s", tc.name, tc.want, got)
		}
		for _, no := range tc.not {
			if strings.Contains(string(got), no) {
				t.Errorf("%s: %q가 있으면 안 된다\n%s", tc.name, no, got)
			}
		}
	}
}

// 스크립트가 없어도 **빈 자리를 남기지 않는다.** 무엇이 있어야 하는지 글자로
// 적어두고 anim.js가 그 자리를 대신 채운다.
func TestAnimLeavesSomethingToReadWithoutScript(t *testing.T) {
	got, err := New().Render(":::anim sort-bubble\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "anim-fallback") {
		t.Errorf("대체 글자가 없다: %s", got)
	}
	if !strings.Contains(string(got), "sort-bubble") {
		t.Errorf("무엇이 있어야 하는지 안 적혀 있다: %s", got)
	}
}
