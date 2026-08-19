package markdown

import (
	"strings"
	"testing"
)

func render(t *testing.T, src string) string {
	t.Helper()
	out, err := New().Render(src)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return string(out)
}

func TestBasicMarkdown(t *testing.T) {
	got := render(t, "# 제목\n\n본문 **굵게** 그리고 *기울임*.\n")
	for _, want := range []string{"<h1", "제목", "<strong>굵게</strong>", "<em>기울임</em>"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q가 없다:\n%s", want, got)
		}
	}
}

func TestCodeBlockKeepsLanguageClass(t *testing.T) {
	got := render(t, "```c\nint main() {}\n```\n")
	if !strings.Contains(got, `class="language-c"`) {
		t.Errorf("언어 클래스가 없다:\n%s", got)
	}
	if !strings.Contains(got, "int main() {}") {
		t.Errorf("코드 내용이 없다:\n%s", got)
	}
}

func TestGFMTableAndTaskList(t *testing.T) {
	got := render(t, "| 이름 | 설명 |\n| --- | --- |\n| fork | 복제 |\n")
	if !strings.Contains(got, "<table") {
		t.Errorf("표가 렌더링되지 않았다:\n%s", got)
	}

	got = render(t, "- [x] 완료\n- [ ] 미완\n")
	if !strings.Contains(got, `type="checkbox"`) {
		t.Errorf("체크박스가 렌더링되지 않았다:\n%s", got)
	}
}

func TestDetailsPassesThrough(t *testing.T) {
	got := render(t, "<details>\n<summary>개념</summary>\n\n안쪽 내용\n\n</details>\n")
	for _, want := range []string{"<details>", "<summary>개념</summary>", "안쪽 내용"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q가 없다:\n%s", want, got)
		}
	}
}

// ---------- 수식 ----------

// TestInlineMathKeepsLatexIntact는 마크다운 파서가 LaTeX를 건드리지 않는지 본다.
// 밑줄은 기울임으로, 백슬래시+중괄호는 이스케이프로 먹힐 수 있다.
func TestInlineMathKeepsLatexIntact(t *testing.T) {
	got := render(t, `식은 $\{X_t,~~ t \ge 0\}$ 이다.`+"\n")

	if !strings.Contains(got, `class="math math-inline"`) {
		t.Fatalf("인라인 수식 노드가 안 나왔다:\n%s", got)
	}
	// 백슬래시와 중괄호가 그대로 남아야 한다.
	if !strings.Contains(got, `\{X_t,~~ t \ge 0\}`) {
		t.Errorf("LaTeX가 변형됐다:\n%s", got)
	}
	// 밑줄이 기울임이 되면 안 된다.
	if strings.Contains(got, "<em>") {
		t.Errorf("수식 안의 밑줄이 기울임이 됐다:\n%s", got)
	}
}

func TestBlockMathKeepsLatexIntact(t *testing.T) {
	src := "$$\n\\sum_{i=1}^{k}\\;(A \\cap B_{i})\n$$\n"
	got := render(t, src)

	if !strings.Contains(got, `class="math math-display"`) {
		t.Fatalf("블록 수식 노드가 안 나왔다:\n%s", got)
	}
	if !strings.Contains(got, `\sum_{i=1}^{k}\;(A \cap B_{i})`) {
		t.Errorf("LaTeX가 변형됐다:\n%s", got)
	}
}

// TestMathIsHTMLEscaped는 수식 안의 <, & 가 HTML로 새지 않는지 본다.
func TestMathIsHTMLEscaped(t *testing.T) {
	got := render(t, `$a < b \& c$`+"\n")
	if strings.Contains(got, "a < b") {
		t.Errorf("< 가 이스케이프되지 않았다:\n%s", got)
	}
	if !strings.Contains(got, "&lt;") {
		t.Errorf("< 가 &lt;로 안 바뀌었다:\n%s", got)
	}
}

// TestDollarInCodeIsNotMath는 코드 안의 $가 수식으로 잡히지 않는지 본다.
// R의 data1$col, Makefile 변수 등이 여기 해당한다. 실제 본문에 90페이지 있다.
func TestDollarInCodeIsNotMath(t *testing.T) {
	got := render(t, "코드 스팬 `data1$col` 과 `df$name` 입니다.\n")
	if strings.Contains(got, "math-inline") {
		t.Errorf("코드 스팬 안의 $를 수식으로 잡았다:\n%s", got)
	}

	got = render(t, "```make\nCFLAGS = $(FLAGS)\nTARGET = $@\n```\n")
	if strings.Contains(got, "math-inline") {
		t.Errorf("코드 블록 안의 $를 수식으로 잡았다:\n%s", got)
	}
}

// TestLoneDollarIsPlainText는 짝이 없는 $가 그냥 글자로 남는지 본다.
func TestLoneDollarIsPlainText(t *testing.T) {
	got := render(t, "가격은 100$ 입니다.\n")
	if strings.Contains(got, "math") {
		t.Errorf("짝 없는 $를 수식으로 잡았다:\n%s", got)
	}
	if !strings.Contains(got, "100$") {
		t.Errorf("$가 사라졌다:\n%s", got)
	}
}

// TestTwoInlineMathOnOneLine은 한 줄에 수식이 둘일 때 각각 잡히는지 본다.
func TestTwoInlineMathOnOneLine(t *testing.T) {
	got := render(t, `$a+b$ 와 $c+d$ 를 비교한다.`+"\n")
	if n := strings.Count(got, "math-inline"); n != 2 {
		t.Errorf("수식이 %d개 잡혔다. 2개여야 한다:\n%s", n, got)
	}
}

// TestMathAdjacentToProseDollar는 본문의 진짜 $와 수식이 섞여 있을 때를 본다.
// 이건 짝짓기가 애매한 경우라, 어떻게 나오는지 고정해둔다.
func TestMathAdjacentToProseDollar(t *testing.T) {
	got := render(t, `변수 $x$ 의 값과 가격 100$ 를 비교.`+"\n")
	if n := strings.Count(got, "math-inline"); n != 1 {
		t.Errorf("수식이 %d개 잡혔다. 1개여야 한다:\n%s", n, got)
	}
	if !strings.Contains(got, "100$") {
		t.Errorf("본문의 $가 사라졌다:\n%s", got)
	}
}

// TestBlockMathSurvivesSetextUnderline은 수식 안에 `=` 한 글자만 있는 줄이 있어도
// 수식이 통째로 남는지 본다.
//
// 이게 이 파서가 블록 파서인 이유다. 인라인 파싱은 블록이 다 정해진 뒤에 도는데,
// `=`만 있는 줄은 CommonMark의 setext 제목 밑줄이라 윗줄이 h1이 되면서 문단이
// 부서진다. 그러면 인라인 파서에게는 이미 조각난 것만 온다.
func TestBlockMathSurvivesSetextUnderline(t *testing.T) {
	src := "$$\na\\lambda^n + \n\\cdots + c\n= \n(\\lambda - 1)\n= 0\n$$\n"

	got := render(t, src)

	if strings.Contains(got, "<h1") {
		t.Errorf("수식 속 `=` 줄이 제목이 됐다:\n%s", got)
	}
	for _, want := range []string{`class="math math-display"`, `a\lambda^n`, `(\lambda - 1)`, `= 0`} {
		if !strings.Contains(got, want) {
			t.Errorf("%q가 없다:\n%s", want, got)
		}
	}
}

// TestBlockMathInListItem은 목록 항목 안에 들여쓴 수식도 잡는지 본다.
func TestBlockMathInListItem(t *testing.T) {
	got := render(t, "- 항목\n\n  $$\n  x = 1\n  $$\n\n- 다음\n")

	if !strings.Contains(got, `class="math math-display"`) {
		t.Errorf("들여쓴 수식을 못 잡았다:\n%s", got)
	}
	if !strings.Contains(got, "다음") {
		t.Errorf("뒤 항목이 사라졌다:\n%s", got)
	}
}

// TestUnclosedBlockMathStopsAtBlankLine은 닫는 `$$`가 없을 때 뒤의 문단을
// 삼키지 않는지 본다.
//
// 예전에는 문서 전체에서 다음 `$$`를 찾아서, 짝이 안 맞는 `$$` 하나가 뒤의
// 제목과 코드 블록까지 통째로 수식으로 만들었다. 실제로 64자리가 그랬다.
// 변환기가 블록 수식 안에 빈 줄을 남기지 않으므로 빈 줄에서 멈추면 된다.
func TestUnclosedBlockMathStopsAtBlankLine(t *testing.T) {
	got := render(t, "$$\nx = 1\n\n다음 문단\n")

	if !strings.Contains(got, "<p>다음 문단</p>") {
		t.Errorf("뒤 문단이 수식에 딸려 들어갔다:\n%s", got)
	}
}

// TestUnclosedBlockMathDoesNotEatHeading은 닫는 `$$`가 없어도 뒤의 제목이
// 제목으로 남는지 본다.
func TestUnclosedBlockMathDoesNotEatHeading(t *testing.T) {
	got := render(t, "$$\nx = 1\n\n## 제목\n\n$$\ny = 2\n$$\n")

	if !strings.Contains(got, "<h2") || !strings.Contains(got, "제목") {
		t.Errorf("제목이 사라졌다:\n%s", got)
	}
	if !strings.Contains(got, "y = 2") {
		t.Errorf("뒤 수식이 사라졌다:\n%s", got)
	}
}

// TestSingleLineBlockMathStaysInline은 한 줄로 닫히는 `$$...$$`는 블록 파서가
// 가져가지 않는지 본다. 가져가면 그 줄의 앞뒤 글자가 사라진다.
func TestSingleLineBlockMathStaysInline(t *testing.T) {
	got := render(t, "$$x = 1$$\n")

	if !strings.Contains(got, `class="math math-display">x = 1<`) {
		t.Errorf("한 줄 수식이 깨졌다:\n%s", got)
	}
}

// TestBlockMathKeepsLineBreaks는 여러 줄 배치를 그대로 넘기는지 본다.
// LaTeX는 줄바꿈으로 정렬을 표현하는 경우가 있다.
func TestBlockMathKeepsLineBreaks(t *testing.T) {
	got := render(t, "$$\n\\begin{align*}\na &= b \\\\\nc &= d\n\\end{align*}\n$$\n")

	if !strings.Contains(got, "\\begin{align*}\na &amp;= b") {
		t.Errorf("줄바꿈이 사라졌다:\n%s", got)
	}
}

// TestFencedCodeAfterBlockMath는 수식 뒤의 코드 펜스가 코드로 남는지 본다.
// 예전에는 코드 블록이 통째로 수식 안에 들어간 자리가 있었다.
func TestFencedCodeAfterBlockMath(t *testing.T) {
	got := render(t, "$$\nx = 1\n$$\n\n```r\nm <- 2\n```\n")

	if !strings.Contains(got, `<code class="language-r">`) {
		t.Errorf("코드 블록이 사라졌다:\n%s", got)
	}
}
