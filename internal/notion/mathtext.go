package notion

import (
	"regexp"
	"strings"
)

// 수식 안의 한글을 \text{}로 감싼다.
//
// KaTeX는 수식 모드의 글자를 수학 글꼴로 그린다. 라틴 문자는 이탤릭 변수로
// 보이면 그만이지만 **한글은 자모가 겹쳐 뭉개진다.** 실제로 `1. 확률`의
// `"조건부확률" = \dfrac{"결합확률"}{"주변확률"}`이 읽을 수 없게 나왔다.
//
// 한글이 들어간 수식은 24개(글 11편)뿐이고 전부 "설명하려고 한글을 그냥 쓴"
// 자리다 — 첨자(`X_{선수}`), 행렬 칸(`정답의 시작`), 분수(`\dfrac{정밀도}{재현율}`)
// 같은 것들이다. 그러니 한글 덩어리를 통째로 \text{}에 넣으면 된다.

// hangulRun은 한글 덩어리다. **덩어리 사이의 공백은 함께 잡는다** —
// 수식 모드에서 공백은 무시되므로, `행의 갯수`를 따로 감싸면 `행의갯수`가 된다.
var hangulRun = regexp.MustCompile(`[가-힣]+(?: +[가-힣]+)*`)

// textCommand는 이미 글자 모드인 자리다. \text, \textbf, \textrm … 을 모두 잡는다.
var textCommand = regexp.MustCompile(`\\text[a-zA-Z]*\{`)

// wrapHangulInText는 수식에서 \text{} 밖에 있는 한글을 \text{}로 감싼다.
//
// 이미 \text{...} 안에 있는 한글은 건드리지 않는다. 두 번 감싸면 중괄호만
// 늘어나고, 원문이 이미 옳게 쓴 자리를 우리가 다시 쓰는 셈이다.
func wrapHangulInText(expr string) string {
	if !hasHangul(expr) {
		return expr
	}
	var sb strings.Builder
	for i := 0; i < len(expr); {
		loc := textCommand.FindStringIndex(expr[i:])
		if loc == nil {
			sb.WriteString(hangulRun.ReplaceAllString(expr[i:], `\text{$0}`))
			break
		}
		start, open := i+loc[0], i+loc[1] // open은 여는 `{` 바로 다음이다
		sb.WriteString(hangulRun.ReplaceAllString(expr[i:start], `\text{$0}`))
		end := matchBrace(expr, open)
		sb.WriteString(expr[start:end]) // \text{...} 안은 그대로 둔다
		i = end
	}
	return sb.String()
}

func hasHangul(s string) bool {
	for _, r := range s {
		if r >= '가' && r <= '힣' {
			return true
		}
	}
	return false
}

// matchBrace는 open(여는 `{` 다음 자리)에서 시작해 짝이 되는 `}` **다음** 자리를
// 돌려준다. 짝이 없으면 문자열 끝이다 — 원문이 깨진 것이므로 우리가 고치지 않는다.
//
// `\{`는 글자로 쓴 중괄호라 세지 않는다.
func matchBrace(s string, open int) int {
	depth := 1
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++ // 다음 한 글자는 이스케이프된 것이라 건너뛴다
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return i + 1
			}
		}
	}
	return len(s)
}
