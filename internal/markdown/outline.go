package markdown

import (
	"strconv"
	"strings"
	"unicode"

	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// Heading은 본문에서 뽑은 제목 한 줄이다.
type Heading struct {
	Level int
	// ID는 제목에 달리는 앵커다. 본문 HTML의 id와 같은 값이라 #ID로 건너뛸 수 있다.
	ID   string
	Text string
}

// outlineMaxLevel은 목차에 넣을 가장 깊은 단계다. h4 아래는 본문 안의 잔가지라
// 목차에 넣으면 목차가 본문만큼 길어진다.
const outlineMaxLevel = 3

// Outline은 본문의 h1~h3를 문서에 나온 순서대로 뽑는다.
//
// id는 goldmark가 본문을 그릴 때 다는 것과 **같은 값**이다. 같은 파서 설정으로
// 같은 원문을 읽으면 자동 id 생성이 같은 결과를 내기 때문이다(제목이 겹칠 때
// 붙는 -1, -2까지 포함해서). 그래서 여기서 만든 앵커가 본문에서 반드시 찾아진다.
//
// 원문이 아니라 **렌더링에 넘길 바로 그 문자열**을 줘야 한다. 웹 쪽에서 죽은
// 링크를 손본 뒤라 원문과 다르다.
func (r *Renderer) Outline(source string) []Heading {
	src := []byte(source)
	doc := r.md.Parser().Parse(text.NewReader(src), parser.WithContext(newParseContext()))

	var out []Heading
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		h, ok := n.(*gast.Heading)
		if !ok {
			return gast.WalkContinue, nil
		}
		// AST의 단계는 headingShift가 내려둔 값이다. 목차에는 본문 기준
		// 단계(1부터)를 돌려준다 — 템플릿이 toc-h{{.Level}}로 들여쓰기 때문에
		// 여기서 되돌리지 않으면 들여쓰기가 통째로 한 칸 밀린다.
		level := h.Level - headingOffset
		if level < 1 {
			level = 1
		}
		if level > outlineMaxLevel {
			return gast.WalkContinue, nil
		}
		id, ok := h.AttributeString("id")
		if !ok {
			// WithAutoHeadingID가 꺼져 있으면 앵커가 없다. 링크할 데가 없으니 뺀다.
			return gast.WalkSkipChildren, nil
		}
		idBytes, ok := id.([]byte)
		if !ok {
			return gast.WalkSkipChildren, nil
		}
		text := headingText(h, src)
		if text == "" {
			return gast.WalkSkipChildren, nil
		}
		out = append(out, Heading{Level: level, ID: string(idBytes), Text: text})
		return gast.WalkSkipChildren, nil
	})
	return out
}

// headingText는 제목의 글자만 모은다.
//
// 굵게/기울임 같은 꾸밈은 벗기고 안쪽 글자만 쓴다. 수식은 원문 LaTeX를 그대로
// 넣는다 — 본문에서도 아직 그렇게 보이므로 목차만 다르면 오히려 어긋난다.
func headingText(h *gast.Heading, src []byte) string {
	var b []byte
	_ = gast.Walk(h, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		switch t := n.(type) {
		case *gast.Text:
			b = append(b, t.Segment.Value(src)...)
		case *gast.String:
			b = append(b, t.Value...)
		case *mathNode:
			b = append(b, t.value...)
		}
		return gast.WalkContinue, nil
	})
	return string(b)
}

// headingIDs는 제목에 붙일 앵커를 만든다. goldmark의 기본 구현 대신 쓴다.
//
// 기본 구현은 **한 바이트가 아닌 글자를 전부 버린다.** 그래서 한글 제목이 전부
// `heading`, `heading-1`, `heading-2`…가 된다. 링크는 되지만 주소에 뜻이 없고,
// 무엇보다 **글을 고쳐 제목 하나만 늘려도 뒤쪽 앵커가 통째로 밀린다.**
//
// 규칙은 이 프로젝트의 slug와 같다: 소문자 + 공백/`:`/`/`/`.`→하이픈, 나머지
// 기호는 버리고, 한글은 그대로 둔다.
type headingIDs struct {
	used map[string]bool
}

func newHeadingIDs() *headingIDs { return &headingIDs{used: map[string]bool{}} }

func (h *headingIDs) Generate(value []byte, kind gast.NodeKind) []byte {
	base := slugify(string(value))
	if base == "" {
		base = "section"
		if kind != gast.KindHeading {
			base = "id"
		}
	}
	id := base
	for i := 1; h.used[id]; i++ {
		id = base + "-" + strconv.Itoa(i)
	}
	h.used[id] = true
	return []byte(id)
}

func (h *headingIDs) Put(value []byte) { h.used[string(value)] = true }

// slugify는 제목을 앵커로 바꾼다.
func slugify(s string) string {
	var b strings.Builder
	lastHyphen := true // 앞머리 하이픈을 막는다
	for _, r := range strings.TrimSpace(strings.ToLower(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastHyphen = false
		case unicode.IsSpace(r), r == ':', r == '/', r == '.', r == '-', r == '_':
			if !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}
