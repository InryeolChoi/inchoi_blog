package markdown

import (
	"strings"
	"testing"
)

// SplitBlocks의 계약은 하나다: **잘라서 다시 이어 붙이면 같은 화면이 나온다.**
// 저장이 언제나 이어 붙인 문자열을 보내므로, 이게 깨지면 글을 열었다 닫기만
// 해도 본문이 달라진다.
func sameRender(t *testing.T, src string) {
	t.Helper()
	md := New()
	before, err := md.Render(src)
	if err != nil {
		t.Fatalf("원문 렌더링: %v", err)
	}
	after, err := md.Render(strings.Join(SplitBlocks(src), "\n\n"))
	if err != nil {
		t.Fatalf("이어 붙인 것 렌더링: %v", err)
	}
	// **문서 맨 끝의 빈 줄은 빼고 견준다.** 이어 붙인 것에는 그 줄이 없는데,
	// 그건 화면에 아무 차이도 내지 않는다 — 실제 글 1,252편에서 갈린 자리가
	// 23편 있었고 전부 이것이었다.
	if strings.TrimRight(string(before), " \t\r\n") != strings.TrimRight(string(after), " \t\r\n") {
		t.Errorf("자르고 이어 붙이니 화면이 달라졌다.\n블록: %q\n전:\n%s\n후:\n%s",
			SplitBlocks(src), before, after)
	}
}

func TestSplitKeepsWhatMustStayTogether(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			"코드 블록 안의 빈 줄에서 안 자른다",
			"앞 문단\n\n```go\nfunc main() {\n\n\tprintln(1)\n}\n```\n\n뒤 문단",
			[]string{"앞 문단", "```go\nfunc main() {\n\n\tprintln(1)\n}\n```", "뒤 문단"},
		},
		{
			"닫는 줄이 없는 울타리는 끝까지 한 덩어리다",
			"앞\n\n```\n안 닫힘\n\n계속",
			[]string{"앞", "```\n안 닫힘\n\n계속"},
		},
		{
			"블록 수식은 한 덩어리다",
			"앞\n\n$$\na = b\n\nc = d\n$$\n\n뒤",
			[]string{"앞", "$$\na = b\n\nc = d\n$$", "뒤"},
		},
		{
			// 문단 한가운데의 `$$`는 **안 끊는다.** 거기서 끊으면 한 문단이
			// 셋으로 갈리는데 원문에서는 문단 안의 수식이라 화면이 달라진다.
			"문단 안의 수식은 그 문단째로 한 덩어리다",
			"앞\n$$x = 1$$\n뒤",
			[]string{"앞\n$$x = 1$$\n뒤"},
		},
		{
			"제 자리에 선 블록 수식은 혼자 선다",
			"앞\n\n$$\nx = 1\n$$\n\n뒤",
			[]string{"앞", "$$\nx = 1\n$$", "뒤"},
		},
		{
			"제목은 언제나 혼자 선다",
			"## 제목\n바로 아래 문단",
			[]string{"## 제목", "바로 아래 문단"},
		},
		{
			"#뒤에 공백이 없으면 제목이 아니다",
			"#태그 같은 줄",
			[]string{"#태그 같은 줄"},
		},
		{
			"항목 사이가 빈 목록은 한 덩어리다",
			"- 하나\n\n- 둘\n\n다음 문단",
			[]string{"- 하나\n\n- 둘", "다음 문단"},
		},
		{
			"목록 다음의 평범한 문단은 갈린다",
			"- 하나\n- 둘\n\n다음 문단",
			[]string{"- 하나\n- 둘", "다음 문단"},
		},
		{
			"표는 빈 줄이 없으니 저절로 한 덩어리다",
			"| 가 | 나 |\n|---|---|\n| 1 | 2 |\n\n뒤",
			[]string{"| 가 | 나 |\n|---|---|\n| 1 | 2 |", "뒤"},
		},
		{
			"빈 줄이 여러 개여도 빈 블록을 만들지 않는다",
			"앞\n\n\n\n뒤",
			[]string{"앞", "뒤"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := SplitBlocks(c.src)
			if len(got) != len(c.want) {
				t.Fatalf("블록이 %d개다(%q). %d개여야 한다(%q)", len(got), got, len(c.want), c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("%d번째가\n%q\n여야 하는데\n%q", i, c.want[i], got[i])
				}
			}
			sameRender(t, c.src)
		})
	}
}

// 화면이 안 바뀌는지를 따로 본다. 위의 표는 "어떻게 잘리나"를 보고, 이건
// **잘라도 되나**를 본다 — 둘 중 뒤엣것이 계약이다.
func TestSplitRoundTripKeepsTheRender(t *testing.T) {
	srcs := []string{
		"# 제목\n\n문단 하나.\n\n- 목록\n- 둘\n\n> 인용\n> 이어짐\n\n```py\nx = 1\n```\n",
		"수식 $x^2$ 이 든 문단.\n\n$$\n\\int_0^1 f(x)\\,dx\n$$\n\n끝.",
		"| a | b |\n|---|---|\n| 1 | 2 |\n\n표 다음 문단.",
		"1. 하나\n2. 둘\n\n3. 셋\n\n평범한 문단",
		":::anim sort-bubble\n\n애니메이션 다음 문단",
		"들여쓴 이어짐이 있는 목록:\n\n- 항목\n\n  그 항목의 둘째 문단\n\n- 다음 항목",
		"이미지\n\n![](/img/abc)\n\n[링크](/p/some-post)",
	}
	for _, src := range srcs {
		sameRender(t, src)
	}
}
