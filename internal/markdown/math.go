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

// mathNode는 수식 하나다. 원문 LaTeX를 그대로 들고 있다.
type mathNode struct {
	gast.BaseInline
	value []byte
	block bool
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
	line, seg := block.PeekLine()
	if len(line) < 2 || line[0] != '$' {
		return nil
	}

	// $$ ... $$ (블록). 한 문단 안에서 여러 줄에 걸칠 수 있다.
	if line[1] == '$' {
		rest := block.Source()[seg.Start+2:]
		end := indexDoubleDollar(rest)
		if end < 0 {
			return nil
		}
		value := rest[:end]
		block.Advance(2 + end + 2)
		return &mathNode{value: value, block: true}
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

func (r *mathRenderer) render(w util.BufWriter, _ []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	if !entering {
		return gast.WalkContinue, nil
	}
	n := node.(*mathNode)
	tag, class := "span", "math math-inline"
	if n.block {
		tag, class = "div", "math math-display"
	}
	if _, err := w.WriteString("<" + tag + ` class="` + class + `">`); err != nil {
		return gast.WalkStop, err
	}
	// 원문 LaTeX는 HTML 이스케이프만 한다. 내용 자체는 손대지 않는다.
	template.HTMLEscape(w, n.value)
	if _, err := w.WriteString("</" + tag + ">"); err != nil {
		return gast.WalkStop, err
	}
	return gast.WalkContinue, nil
}

// mathExtension은 위 파서와 렌더러를 goldmark에 끼운다.
type mathExtension struct{}

func (e *mathExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(
		// 강조(`*`, `_`)보다 먼저 봐야 수식 안의 기호를 뺏기지 않는다.
		// goldmark의 기본 인라인 파서들은 우선순위 500 이상을 쓴다.
		util.Prioritized(&mathParser{}, 100),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&mathRenderer{}, 100),
	))
}
