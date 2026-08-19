package markdown

import (
	"strings"
	"testing"
)

func TestExternalLinkAloneBecomesCard(t *testing.T) {
	got := render(t, "[유클리드 호제법](https://en.wikipedia.org/wiki/Euclidean_algorithm)\n")
	for _, want := range []string{
		`class="extcard"`,
		`href="https://en.wikipedia.org/wiki/Euclidean_algorithm"`,
		`rel="noreferrer"`,
		`class="extcard-t">유클리드 호제법</span>`,
		`class="extcard-h">en.wikipedia.org</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("%q가 없다:\n%s", want, got)
		}
	}
}

// 문장 속 링크까지 카드로 만들면 "자세한 건 [여기]를 봐라"가 상자로 끊긴다.
func TestExternalLinkInSentenceStaysText(t *testing.T) {
	got := render(t, "자세한 건 [여기](https://example.com/x)를 봐라.\n")
	if strings.Contains(got, "extcard") {
		t.Errorf("문장 속 링크가 카드가 됐다:\n%s", got)
	}
	if !strings.Contains(got, `<a href="https://example.com/x">여기</a>`) {
		t.Errorf("보통 링크가 아니다:\n%s", got)
	}
}

// 내부 글 링크는 절대 카드가 되지 않는다. 이게 깨지면 본문 링크 670개가
// 통째로 카드가 된다.
func TestInternalLinkNeverBecomesCard(t *testing.T) {
	for _, src := range []string{
		"[다른 글](/p/abc-def)\n",
		"[분류](/dev/language)\n",
		"[메일](mailto:a@b.c)\n",
	} {
		if got := render(t, src); strings.Contains(got, "extcard") {
			t.Errorf("%q가 카드가 됐다:\n%s", src, got)
		}
	}
}

// 캡션이 없는 bookmark는 링크 글자가 URL 그대로다. 그대로 쓰면 두 줄이
// 같은 말을 두 번 한다.
func TestCardLabelsWithoutCaption(t *testing.T) {
	got := render(t, "[https://github.com/InryeolChoi/blog](https://github.com/InryeolChoi/blog)\n")
	if !strings.Contains(got, `class="extcard-t">github.com</span>`) {
		t.Errorf("호스트가 제목이 아니다:\n%s", got)
	}
	if !strings.Contains(got, `class="extcard-h">/InryeolChoi/blog</span>`) {
		t.Errorf("경로가 아래 줄이 아니다:\n%s", got)
	}
}

// 노션이 호스팅하던 첨부파일 URL은 서명 때문에 쿼리만 1,500자가 넘는다.
func TestCardDropsQueryString(t *testing.T) {
	long := "https://prod-files-secure.s3.us-west-2.amazonaws.com/x/y/Practice.pdf?X-Amz-Signature=" +
		strings.Repeat("a", 300)
	got := render(t, "["+long+"]("+long+")\n")
	// 화면에 찍히는 두 줄에는 쿼리가 없어야 한다. href에는 있어야 누르면 간다.
	for _, cls := range []string{"extcard-t", "extcard-h"} {
		if strings.Contains(visibleSpan(got, cls), "X-Amz-Signature") {
			t.Errorf("%s에 쿼리가 새어나왔다:\n%s", cls, got)
		}
	}
	if !strings.Contains(got, "X-Amz-Signature=aaa") {
		t.Errorf("href에서 쿼리가 사라졌다:\n%s", got)
	}
}

func TestAutoLinkBecomesCard(t *testing.T) {
	got := render(t, "https://github.com/InryeolChoi\n")
	if !strings.Contains(got, "extcard") {
		t.Errorf("맨 URL이 카드가 안 됐다:\n%s", got)
	}
	if !strings.Contains(got, `class="extcard-t">github.com</span>`) {
		t.Errorf("호스트가 제목이 아니다:\n%s", got)
	}
}

// 카드는 링크 안에 들어가므로 제목에 태그가 섞이면 안 된다.
func TestCardEscapesLabel(t *testing.T) {
	got := render(t, "[<script>x</script>](https://example.com)\n")
	if strings.Contains(got, "<script>") {
		t.Errorf("태그가 그대로 나갔다:\n%s", got)
	}
}

// 목차(Outline)는 카드가 된 문단을 제목으로 착각하면 안 된다.
func TestOutlineIgnoresCards(t *testing.T) {
	heads := New().Outline("# 하나\n\n[영상](https://youtube.com/watch?v=x)\n\n# 둘\n")
	if len(heads) != 2 {
		t.Fatalf("제목 2개를 기대했는데 %d개다: %+v", len(heads), heads)
	}
}

// visibleSpan은 주어진 클래스의 span 안 글자만 꺼낸다. href처럼 눈에 안 보이는
// 곳과 구분해서 보려고 쓴다.
func visibleSpan(html, class string) string {
	open := `<span class="` + class + `">`
	i := strings.Index(html, open)
	if i < 0 {
		return ""
	}
	rest := html[i+len(open):]
	j := strings.Index(rest, "</span>")
	if j < 0 {
		return rest
	}
	return rest[:j]
}
