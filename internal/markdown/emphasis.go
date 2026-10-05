package markdown

import (
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// 한글 곁의 `**`는 CommonMark 기본 규칙으로는 굵게 안 되는 경우가 있다.
//
// 규칙상 닫는 `**`는 **앞이 문장부호이면 뒤가 공백이나 문장부호여야** 닫을 수
// 있다. 영어는 뒤에 보통 공백이 오지만 한국어는 조사가 붙는다.
//
//	**"~다"**로 끝낸다      → 닫지 못해 `**`가 그대로 나온다
//	문장 **강조(괄호)**를 쓴다
//	**`code`**를 쓴다
//
// 그래서 `*`에 한해, 맞닿은 글자가 한중일 문자이면 이 문장부호 제한을 풀어
// 준다. 영어 글의 동작은 CommonMark 그대로다. `_`는 단어 안 밑줄(snake_case)을
// 지켜야 해서 건드리지 않는다.
type cjkEmphasisExtension struct{}

func (e *cjkEmphasisExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(
		// goldmark의 기본 강조 파서(500)보다 먼저 `*`를 가져간다.
		util.Prioritized(&cjkEmphasisParser{}, 450),
	))
}

type cjkEmphasisProcessor struct{}

func (cjkEmphasisProcessor) IsDelimiter(b byte) bool { return b == '*' }

func (cjkEmphasisProcessor) CanOpenCloser(opener, closer *parser.Delimiter) bool {
	return opener.Char == closer.Char
}

func (cjkEmphasisProcessor) OnMatch(consumes int) ast.Node { return ast.NewEmphasis(consumes) }

type cjkEmphasisParser struct{}

func (*cjkEmphasisParser) Trigger() []byte { return []byte{'*'} }

func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hangul, unicode.Hiragana, unicode.Katakana)
}

func (*cjkEmphasisParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	before := block.PrecendingCharacter()
	line, segment := block.PeekLine()

	n := 0
	for n < len(line) && line[n] == '*' {
		n++
	}
	if n == 0 {
		return nil
	}
	after := rune(' ')
	if n < len(line) {
		after = util.ToRune(line, n)
	}

	beforePunct, beforeSpace := util.IsPunctRune(before), util.IsSpaceRune(before)
	afterPunct, afterSpace := util.IsPunctRune(after), util.IsSpaceRune(after)
	// CommonMark의 left/right-flanking에서 "문장부호 곁" 제한만 CJK 곁에서 푼다.
	canOpen := !afterSpace && (!afterPunct || beforeSpace || beforePunct || isCJK(before))
	canClose := !beforeSpace && (!beforePunct || afterSpace || afterPunct || isCJK(after))

	node := parser.NewDelimiter(canOpen, canClose, n, '*', cjkEmphasisProcessor{})
	node.Segment = segment.WithStop(segment.Start + n)
	block.Advance(n)
	pc.PushDelimiter(node)
	return node
}
