package markdown

import (
	"html/template"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// 수식은 마크다운 파서가 건드리지 못하게 가로채야 한다.
//
// LaTeX와 CommonMark는 같은 기호를 다르게 읽는다. 파서에 그냥 넘기면
// `\{`는 `{`가 되고(백슬래시 이스케이프), `X_t ... _n`의 밑줄은 기울임으로 짝지어지고,
// `*`는 강조가 된다. 그러면 KaTeX에 넘길 원문이 이미 망가져 있다.
//
// 그래서 `$...$`와 `$$...$$`를 별도 노드로 파싱해서 원문 그대로 들고 있다가,
// HTML로 쓸 때만 이스케이프한다. 실제 수식 렌더링은 브라우저에서 KaTeX가 한다.

var (
	kindInlineMath = gast.NewNodeKind("InlineMath")
	kindBlockMath  = gast.NewNodeKind("BlockMath")
)

// mathNode는 인라인 수식 하나다. 원문 LaTeX를 그대로 들고 있다.
type mathNode struct {
	gast.BaseInline
	value []byte
	block bool
}

// mathBlockNode는 제 줄에서 시작하는 `$$` 수식이다.
//
// **인라인 파서로는 이걸 잡을 수 없다.** 인라인 파싱은 블록이 다 정해진 뒤에
// 도는데, 수식 안의 한 줄이 블록 문법으로 먼저 읽혀서 문단이 쪼개지기 때문이다.
// 실제로 다음이 그렇다:
//
//	$$
//	a\lambda^n + \cdots + c
//	=
//	(\lambda - \lambda_1) \cdots = 0
//	$$
//
// 여기서 `=` 한 글자만 있는 줄은 CommonMark의 **setext 제목 밑줄**이라, 윗줄이
// h1이 되면서 문단이 통째로 부서진다. `-`로 시작하는 줄(목록), `#`(제목),
// 백틱 세 개(코드 펜스)도 마찬가지다. 그래서 블록 파서로 먼저 가로챈다.
type mathBlockNode struct {
	gast.BaseBlock
	value []byte
}

func (n *mathBlockNode) Kind() gast.NodeKind { return kindBlockMath }

func (n *mathBlockNode) Dump(source []byte, level int) {
	gast.DumpHelper(n, source, level, map[string]string{"value": string(n.value)}, nil)
}

func (n *mathNode) Kind() gast.NodeKind {
	if n.block {
		return kindBlockMath
	}
	return kindInlineMath
}

func (n *mathNode) Dump(source []byte, level int) {
	gast.DumpHelper(n, source, level, map[string]string{"value": string(n.value)}, nil)
}

// mathBlockParser는 제 줄에서 시작하는 `$$`를 문단이 되기 전에 가로챈다.
//
// 닫는 `$$`를 만날 때까지의 줄을 원문 그대로 모은다. 안쪽은 무엇이든 글자로만
// 본다 — 제목도 목록도 코드 펜스도 여기서는 LaTeX일 뿐이다.
type mathBlockParser struct{}

func (p *mathBlockParser) Trigger() []byte { return []byte{'$'} }

func (p *mathBlockParser) Open(_ gast.Node, reader text.Reader, pc parser.Context) (gast.Node, parser.State) {
	line, _ := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos < 0 || !hasDoubleDollarPrefix(line[pos:]) {
		return nil, parser.NoChildren
	}
	// 같은 줄에서 닫히면 인라인 파서에게 맡긴다. 여기서 잡으면 문단 하나가
	// 통째로 수식 블록이 돼서 앞뒤 글자가 사라진다.
	if indexDoubleDollar(line[pos+2:]) >= 0 {
		return nil, parser.NoChildren
	}
	reader.AdvanceToEOL()
	return &mathBlockNode{}, parser.NoChildren
}

func (p *mathBlockParser) Continue(node gast.Node, reader text.Reader, _ parser.Context) parser.State {
	line, segment := reader.PeekLine()
	if len(line) == 0 {
		return parser.Close
	}
	// **빈 줄에서 멈춘다.** 변환기가 블록 수식 안에 빈 줄을 남기지 않으므로
	// (normalizeBlockMath), 빈 줄을 만났다는 건 닫는 `$$`가 없다는 뜻이다.
	// 여기서 안 멈추면 짝이 안 맞는 `$$` 하나가 뒤의 문단과 제목을 통째로
	// 수식으로 삼킨다. 실제로 그렇게 망가진 자리가 64개 있었다.
	if util.IsBlank(line) {
		return parser.Close
	}
	// 닫는 `$$`. 그 앞의 내용은 챙기고 블록을 닫는다.
	if end := indexDoubleDollar(line); end >= 0 {
		if end > 0 {
			node.Lines().Append(text.NewSegment(segment.Start, segment.Start+end))
		}
		reader.AdvanceToEOL()
		return parser.Close
	}
	node.Lines().Append(segment)
	reader.AdvanceToEOL()
	return parser.Continue | parser.NoChildren
}

// Close에서 원문을 꺼내 노드에 담는다. 렌더러는 source를 안 보고 값만 쓴다.
func (p *mathBlockParser) Close(node gast.Node, reader text.Reader, _ parser.Context) {
	n := node.(*mathBlockNode)
	n.value = node.Lines().Value(reader.Source())
	node.Lines().Clear()
}

// CanInterruptParagraph는 true다. 문단 바로 다음 줄에서 `$$`가 시작하는 글이
// 있다. 빈 줄을 요구하면 그런 수식을 놓친다.
func (p *mathBlockParser) CanInterruptParagraph() bool { return true }

// CanAcceptIndentedLine은 true다. 목록 항목 안의 수식은 들여쓰여 있다.
func (p *mathBlockParser) CanAcceptIndentedLine() bool { return true }

// hasDoubleDollarPrefix는 줄이 `$$`로 시작하는지 본다.
func hasDoubleDollarPrefix(b []byte) bool {
	return len(b) >= 2 && b[0] == '$' && b[1] == '$'
}

// mathParser는 `$`로 시작하는 수식을 가로챈다.
//
// 두 가지 전제에 기대고 있고, 둘 다 internal/notion 변환기가 보장한다:
//   - 인라인 수식은 한 줄이다 (normalizeInlineMath)
//   - 블록 수식 안에는 빈 줄이 없다 (normalizeBlockMath)
//
// 빈 줄이 없다는 건 `$$...$$`가 통째로 한 문단 안에 들어온다는 뜻이라,
// 블록 파서를 따로 두지 않고 인라인 파서 하나로 둘 다 처리할 수 있다.
//
// 코드 블록과 코드 스팬 안에서는 goldmark가 인라인 파서를 부르지 않는다.
// 그래서 R의 `data1$col`이나 Makefile 변수가 코드로 감싸여 있으면 여기 걸리지 않는다.
type mathParser struct{}

func (p *mathParser) Trigger() []byte { return []byte{'$'} }

func (p *mathParser) Parse(_ gast.Node, block text.Reader, _ parser.Context) gast.Node {
	line, _ := block.PeekLine()
	if len(line) < 2 || line[0] != '$' {
		return nil
	}

	// $$ ... $$ (블록). 한 문단 안에서 여러 줄에 걸칠 수 있다.
	if line[1] == '$' {
		return parseBlockMath(block)
	}

	// $ ... $ (인라인). 같은 줄 안에서만 닫는 $를 찾는다.
	end := indexDollarInLine(line)
	if end < 0 {
		return nil
	}
	value := line[1:end]
	block.Advance(end + 1)
	return &mathNode{value: value, block: false}
}

// parseBlockMath는 `$$ ... $$`를 읽는다. 닫는 `$$`를 못 찾으면 아무것도 하지 않고
// 위치를 되돌린다. 그러면 `$`는 그냥 글자가 되고 뒤의 내용은 마크다운으로 정상
// 렌더링된다.
//
// **닫는 짝은 지금 블록 안에서만 찾는다.** 예전에는 `block.Source()`(문서 전체)를
// 훑어서, 짝이 안 맞는 `$$` 하나가 열리면 문서 뒤쪽 아무 데나 있는 다음 `$$`까지
// 통째로 수식으로 삼켰다. 그 사이의 제목과 코드 블록까지 전부 딸려 들어갔다.
// 실제로 그렇게 망가진 자리가 64개 있었다.
//
// 블록 수식 안에는 빈 줄이 없다는 것을 변환기가 보장하므로(normalizeBlockMath),
// 한 블록 안에서 찾는 것으로 충분하다. reader가 이미 블록 경계에서 끊기기 때문에
// 줄 단위로 훑기만 하면 된다.
func parseBlockMath(block text.Reader) gast.Node {
	startLine, startSeg := block.Position()

	block.Advance(2) // 여는 `$$`
	var value []byte
	for {
		line, _ := block.PeekLine()
		if len(line) == 0 {
			break // 블록 끝. 닫는 짝이 없다
		}
		if end := indexDoubleDollar(line); end >= 0 {
			value = append(value, line[:end]...)
			block.Advance(end + 2)
			return &mathNode{value: value, block: true}
		}
		value = append(value, line...)
		block.AdvanceLine()
	}

	block.SetPosition(startLine, startSeg)
	return nil
}

// indexDoubleDollar는 "$$"가 처음 나오는 위치를 돌려준다. 없으면 -1이다.
func indexDoubleDollar(b []byte) int {
	for i := 0; i+1 < len(b); i++ {
		if b[i] == '$' && b[i+1] == '$' {
			return i
		}
	}
	return -1
}

// indexDollarInLine은 같은 줄에서 닫는 `$` 위치를 돌려준다.
// 줄이 끝나거나 내용이 비어 있으면 -1이다.
func indexDollarInLine(line []byte) int {
	for i := 1; i < len(line); i++ {
		switch line[i] {
		case '\n':
			return -1
		case '$':
			if i == 1 {
				return -1 // `$$`는 여기서 다루지 않는다
			}
			return i
		}
	}
	return -1
}

// mathRenderer는 수식 노드를 KaTeX가 집어갈 수 있는 형태로 쓴다.
// 지금은 서버에서 수식을 그리지 않고 원문만 넘긴다.
type mathRenderer struct{}

func (r *mathRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindInlineMath, r.render)
	reg.Register(kindBlockMath, r.render)
}

// mathValue는 노드 종류에 상관없이 원문 LaTeX와 블록 여부를 꺼낸다.
func mathValue(node gast.Node) ([]byte, bool, bool) {
	switch n := node.(type) {
	case *mathNode:
		return n.value, n.block, true
	case *mathBlockNode:
		return n.value, true, true
	}
	return nil, false, false
}

func (r *mathRenderer) render(w util.BufWriter, _ []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	if !entering {
		return gast.WalkContinue, nil
	}
	value, isBlock, ok := mathValue(node)
	if !ok {
		return gast.WalkContinue, nil
	}
	tag, class := "span", "math math-inline"
	if isBlock {
		tag, class = "div", "math math-display"
	}
	if _, err := w.WriteString("<" + tag + ` class="` + class + `">`); err != nil {
		return gast.WalkStop, err
	}
	// 원문 LaTeX는 HTML 이스케이프만 한다. 내용 자체는 손대지 않는다.
	template.HTMLEscape(w, value)
	if _, err := w.WriteString("</" + tag + ">"); err != nil {
		return gast.WalkStop, err
	}
	return gast.WalkContinue, nil
}

// mathExtension은 위 파서와 렌더러를 goldmark에 끼운다.
type mathExtension struct{}

func (e *mathExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithBlockParsers(
		// 문단이 만들어지기 전에 `$$` 블록을 가져간다. 우선순위는 setext 제목
		// (100)보다 앞이어야 한다. 안 그러면 수식 속 `=` 한 줄짜리가 제목
		// 밑줄로 먼저 읽혀 문단이 부서진다.
		util.Prioritized(&mathBlockParser{}, 99),
	))
	m.Parser().AddOptions(parser.WithInlineParsers(
		// 강조(`*`, `_`)보다 먼저 봐야 수식 안의 기호를 뺏기지 않는다.
		// goldmark의 기본 인라인 파서들은 우선순위 500 이상을 쓴다.
		util.Prioritized(&mathParser{}, 100),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&mathRenderer{}, 100),
	))
}
