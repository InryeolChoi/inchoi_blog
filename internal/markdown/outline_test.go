package markdown

import (
	"regexp"
	"strings"
	"testing"
)

// TestOutlineCollectsHeadings는 h1~h3를 문서 순서대로 뽑는지 본다.
func TestOutlineCollectsHeadings(t *testing.T) {
	r := New()
	src := "# 하나\n\n본문\n\n## 둘\n\n### 셋\n\n#### 넷은 안 나온다\n\n## 다섯\n"

	got := r.Outline(src)

	want := []Heading{{1, "", "하나"}, {2, "", "둘"}, {3, "", "셋"}, {2, "", "다섯"}}
	if len(got) != len(want) {
		t.Fatalf("개수가 다르다: %+v", got)
	}
	for i := range want {
		if got[i].Level != want[i].Level || got[i].Text != want[i].Text {
			t.Errorf("[%d] got %+v, want level=%d text=%q", i, got[i], want[i].Level, want[i].Text)
		}
		if got[i].ID == "" {
			t.Errorf("[%d] id가 비었다: %+v", i, got[i])
		}
	}
}

// TestOutlineIDsMatchRenderedHTML은 목차의 앵커가 본문 HTML의 id와 같은지 본다.
// 두 번 파싱하므로 자동 id 생성이 두 번 같은 답을 내야 한다. 제목이 겹칠 때
// 뒤에 붙는 -1, -2까지 맞아야 링크가 안 깨진다.
func TestOutlineIDsMatchRenderedHTML(t *testing.T) {
	r := New()
	src := "## 버전 확인\n\n가\n\n### 버전 확인\n\n나\n\n## 버전 확인\n"

	html, err := r.Render(src)
	if err != nil {
		t.Fatal(err)
	}
	// headingShift가 단계를 내리므로 본문 제목은 h2~h6로 나온다.
	ids := map[string]bool{}
	for _, m := range regexp.MustCompile(`<h[1-6] id="([^"]+)"`).FindAllStringSubmatch(string(html), -1) {
		ids[m[1]] = true
	}

	out := r.Outline(src)
	if len(out) != 3 {
		t.Fatalf("제목 수가 다르다: %+v", out)
	}
	seen := map[string]bool{}
	for _, h := range out {
		if !ids[h.ID] {
			t.Errorf("본문에 없는 앵커다: %q (본문 id: %v)", h.ID, ids)
		}
		if seen[h.ID] {
			t.Errorf("같은 앵커가 두 번 나왔다: %q", h.ID)
		}
		seen[h.ID] = true
	}
}

// TestOutlineStripsInlineMarkup은 꾸밈을 벗기고 글자만 남기는지 본다.
func TestOutlineStripsInlineMarkup(t *testing.T) {
	r := New()

	got := r.Outline("## **굵은** 제목과 `코드`\n")

	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	if want := "굵은 제목과 코드"; got[0].Text != want {
		t.Errorf("got %q, want %q", got[0].Text, want)
	}
}

// TestOutlineKeepsMathSource는 제목 속 수식을 원문 LaTeX로 남기는지 본다.
// 본문도 아직 원문이 그대로 보이므로 목차만 비면 오히려 어긋난다.
func TestOutlineKeepsMathSource(t *testing.T) {
	r := New()

	got := r.Outline("## 분포 $\\chi^2(r)$ 정리\n")

	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	if !strings.Contains(got[0].Text, `\chi^2(r)`) {
		t.Errorf("수식이 빠졌다: %q", got[0].Text)
	}
}

// TestOutlineIgnoresHeadingsInCodeBlocks는 코드 블록 안의 #을 제목으로 세지
// 않는지 본다. 셸 주석이 든 코드가 많다.
func TestOutlineIgnoresHeadingsInCodeBlocks(t *testing.T) {
	r := New()

	got := r.Outline("```sh\n# 진짜 제목이 아니다\necho hi\n```\n\n## 진짜 제목\n")

	if len(got) != 1 || got[0].Text != "진짜 제목" {
		t.Errorf("got %+v", got)
	}
}

func TestOutlineEmpty(t *testing.T) {
	if got := New().Outline("제목 없는 본문\n"); len(got) != 0 {
		t.Errorf("got %+v", got)
	}
}
