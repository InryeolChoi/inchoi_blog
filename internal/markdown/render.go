// Package markdown은 글 본문(마크다운)을 HTML로 바꾼다.
//
// goldmark를 쓴다. CommonMark 명세를 그대로 따르고, 순수 Go라 cgo 없이 빌드되며,
// 파서와 렌더러를 확장할 수 있다. 확장이 필요한 이유는 수식 때문이다 — math.go 참고.
// gomarkdown이나 blackfriday는 CommonMark 준수도가 낮고 확장 지점이 마땅치 않다.
package markdown

import (
	"bytes"
	"html/template"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// Renderer는 마크다운을 HTML로 바꾼다. 여러 고루틴에서 같이 써도 된다.
type Renderer struct {
	md goldmark.Markdown
}

// New는 이 프로젝트의 본문에 맞춘 렌더러를 만든다.
func New() *Renderer {
	return &Renderer{
		md: goldmark.New(
			goldmark.WithExtensions(
				// 변환기가 표, ~~취소선~~, - [x] 체크박스를 만들어낸다. 전부 GFM 확장이다.
				extension.GFM,
				&mathExtension{},
				// 코드 블록에 언어 라벨을 붙인다.
				&codeBlockExtension{},
				// 제 문단을 통째로 차지한 외부 링크를 카드로 그린다.
				&extCardExtension{},
			),
			goldmark.WithParserOptions(
				// 제목에 id를 달아 목차와 앵커 링크에 쓴다.
				parser.WithAutoHeadingID(),
				// 본문 제목을 한 단계 내린다. 페이지의 <h1>은 템플릿이 그리는
				// 글 제목이라, 본문의 `# 제목`까지 <h1>이면 한 페이지에 <h1>이
				// 여러 개가 된다 (heading.go 참고).
				parser.WithASTTransformers(util.Prioritized(headingShift{}, 100)),
			),
			goldmark.WithRendererOptions(
				// 변환기가 <details>, <u>, <br>, 빈 블록 주석을 그대로 넣는다.
				// 이건 우리가 만든 것이라 통과시킨다.
				//
				// 다만 본문은 결국 사람이 웹에서 고칠 것이므로, 신뢰할 수 없는 입력이
				// 들어오기 시작하면 여기서 HTML을 막고 허용 목록을 두거나
				// 정제 단계를 넣어야 한다.
				html.WithUnsafe(),
			),
		),
	}
}

// newParseContext는 파싱 한 번에 쓸 문맥을 만든다.
//
// 제목 앵커 생성기를 갈아끼우려고 있다. 생성기는 이미 쓴 id를 기억하는 상태를
// 들고 있어서 파싱마다 새로 만들어야 한다 — 돌려 쓰면 두 번째 글부터 앵커에
// -1, -2가 붙는다.
func newParseContext() parser.Context {
	return parser.NewContext(parser.WithIDs(newHeadingIDs()))
}

// Render는 마크다운을 HTML로 바꾼다.
func (r *Renderer) Render(source string) (template.HTML, error) {
	var buf bytes.Buffer
	if err := r.md.Convert([]byte(source), &buf, parser.WithContext(newParseContext())); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}
