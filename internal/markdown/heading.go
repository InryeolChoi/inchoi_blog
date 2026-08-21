package markdown

import (
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// headingOffset은 본문 제목을 몇 단계 내릴지다.
//
// 페이지의 <h1>은 글 제목(또는 카테고리 이름)이고 템플릿이 그린다. 본문은 그
// 아래에 들어가는 내용이므로 본문의 `# 제목`이 또 <h1>이면 한 페이지에 <h1>이
// 여러 개가 된다 — 문서 구조상 형제가 아닌 것이 형제로 보인다. 그래서 본문
// 제목을 한 단계씩 내려서 페이지 제목의 하위로 만든다.
//
// **앵커(id)는 바뀌지 않는다.** goldmark의 자동 id는 제목 글자에서 만들고
// 단계를 보지 않는다. 그래서 이미 나가 있는 #앵커 링크가 안 깨진다.
const headingOffset = 1

// headingShift는 본문의 제목 단계를 headingOffset만큼 내린다.
//
// 렌더링과 Outline이 **같은 파서**를 쓰므로 둘 다 내려간 값을 본다.
// Outline은 사람이 읽을 단계(본문 기준 1단계부터)로 되돌려서 돌려준다.
type headingShift struct{}

func (headingShift) Transform(doc *gast.Document, _ text.Reader, _ parser.Context) {
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		h, ok := n.(*gast.Heading)
		if !ok {
			return gast.WalkContinue, nil
		}
		// HTML에는 h6까지밖에 없다. 더 내려갈 데가 없으면 그대로 둔다.
		if h.Level += headingOffset; h.Level > 6 {
			h.Level = 6
		}
		return gast.WalkContinue, nil
	})
}
