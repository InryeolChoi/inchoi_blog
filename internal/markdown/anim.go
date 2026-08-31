package markdown

import (
	"bytes"
	"html/template"
	"regexp"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// 본문에 애니메이션을 넣는 길은 **이름 하나뿐이다.**
//
//	:::anim sort-bubble
//
// # 왜 본문에 <script>를 쓰게 두지 않는가
//
// 렌더러가 `html.WithUnsafe()`라(render.go) 본문에 `<script>`를 쓰면 **이미
// 실행된다.** 그래서 "되냐"가 아니라 "그래도 되냐"가 문제다. 본문에 임의 JS를
// 담기 시작하면 그 글 자체가 XSS 벡터가 되고 — 4단계에서 AI가 본문을 만들기
// 시작하면 특히 그렇다 — 나중에 CSP를 걸 길도 막힌다. "HTML → CSS → 마지막에
// JS" 원칙과도 어긋난다.
//
// 대신 **이름 붙인 컴포넌트 중에서 고르게 한다.** 본문에는 이름만 들어가고
// 실제 JS는 internal/web/static/anim.js에 사람이 쓴 파일로 있다. 본문은 안전한
// 글자로 남고, 같은 애니메이션을 여러 글에서 다시 쓸 수 있다.
//
// # 스크립트가 없으면
//
// 애니메이션은 JS 없이는 할 수 없는 일이라 대체할 그림이 없다. 그래도 **빈
// 자리를 남기지는 않는다** — 서버가 무엇이 있어야 하는지 글자로 적어두고,
// anim.js가 뜨면 그 자리를 대신 채운다.

// animName은 컴포넌트 이름으로 쓸 수 있는 형태다. **HTML 속성값으로 그대로
// 나가므로** 좁혀둔다.
var animName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,40}$`)

// animOpen은 여는 줄이다. 여기서만 시작한다.
//
// **줄 끝의 개행을 먼저 떼고 견준다.** PeekLine은 개행을 붙여 주는데 Go의
// `$`는 (멀티라인이 아니면) 마지막 개행 **앞**에서 맞지 않는다 — Perl과 다른
// 점이고, 여기서 실제로 한 번 걸렸다.
var animOpen = regexp.MustCompile(`^:::anim[ \t]+(\S+)[ \t]*$`)

// animBlock은 이름 하나를 들고 있는 노드다.
type animBlock struct {
	gast.BaseBlock
	Name string
}

var kindAnim = gast.NewNodeKind("Anim")

func (n *animBlock) Kind() gast.NodeKind { return kindAnim }
func (n *animBlock) Dump(src []byte, level int) {
	gast.DumpHelper(n, src, level, map[string]string{"Name": n.Name}, nil)
}

// animParser는 `:::anim 이름` 한 줄을 가져간다.
//
// **한 줄짜리다.** 닫는 `:::`을 요구하지 않는다 — 지금 넣을 것이 이름뿐이라
// 여는 줄과 닫는 줄 사이가 늘 비고, 빈 껍데기를 요구하면 쓰는 사람이 자꾸
// 빠뜨린다. 나중에 옵션을 받게 되면 그때 본문 있는 형태를 더한다.
type animParser struct{}

func (p *animParser) Trigger() []byte { return []byte{':'} }

func (p *animParser) Open(parent gast.Node, reader text.Reader, pc parser.Context) (gast.Node, parser.State) {
	line, _ := reader.PeekLine()
	m := animOpen.FindSubmatch(bytes.TrimRight(line, "\r\n"))
	if m == nil {
		return nil, parser.NoChildren
	}
	name := string(m[1])
	// **모르는 모양이면 아예 안 가져간다.** 그러면 그 줄이 평범한 문단으로
	// 남아 글자 그대로 보인다 — 조용히 사라지는 것보다 낫다.
	if !animName.MatchString(name) {
		return nil, parser.NoChildren
	}
	reader.Advance(len(line))
	return &animBlock{Name: name}, parser.NoChildren
}

func (p *animParser) Continue(node gast.Node, reader text.Reader, pc parser.Context) parser.State {
	return parser.Close
}

func (p *animParser) Close(node gast.Node, reader text.Reader, pc parser.Context) {}
func (p *animParser) CanInterruptParagraph() bool                                 { return true }
func (p *animParser) CanAcceptIndentedLine() bool                                 { return false }

// animRenderer는 자리와 대체 글자를 낸다. **움직이는 것은 anim.js가 만든다.**
type animRenderer struct{}

func (r *animRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindAnim, r.render)
}

func (r *animRenderer) render(w util.BufWriter, source []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	if !entering {
		return gast.WalkContinue, nil
	}
	n := node.(*animBlock)
	if _, err := w.WriteString(`<div class="anim" data-anim="`); err != nil {
		return gast.WalkStop, err
	}
	// 이름은 위에서 형태를 좁혔지만 **그래도 이스케이프한다.** 검사와 출력
	// 사이에 사람이 하나를 고치는 일이 생긴다.
	template.HTMLEscape(w, []byte(n.Name))
	if _, err := w.WriteString(`"><p class="anim-fallback">애니메이션 <code>`); err != nil {
		return gast.WalkStop, err
	}
	template.HTMLEscape(w, []byte(n.Name))
	_, err := w.WriteString(`</code> — 스크립트가 있어야 움직인다.</p></div>` + "\n")
	return gast.WalkContinue, err
}

// animExtension은 위 파서와 렌더러를 goldmark에 끼운다.
type animExtension struct{}

func (e *animExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithBlockParsers(
		// 문단보다 먼저 봐야 한다. 수식 블록 파서(99)와 같은 이유로 낮게 둔다.
		util.Prioritized(&animParser{}, 98),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&animRenderer{}, 100),
	))
}
