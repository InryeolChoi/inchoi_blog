package markdown

import (
	"html/template"
	"net/url"
	"strings"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// 본문 안의 외부 링크를 노션의 bookmark처럼 카드로 그린다.
//
// **문단 하나가 통째로 링크일 때만** 카드가 된다. 노션에서 bookmark와
// link_preview는 블록이었고(변환기가 `[캡션](url)` 한 줄로 옮겼다), 문장 속
// 인라인 링크는 원래 글자였다. 그 구분을 그대로 살린다 — 문장 중간의 링크까지
// 카드로 만들면 "자세한 건 [여기]를 봐라" 같은 문장이 세 줄짜리 상자로 끊긴다.
//
// 현재 본문의 외부 링크 117개 중 71개(글 48편)가 제 문단을 통째로 차지한다.
// 나머지 46개는 문장 안에 있어서 글자 링크로 남는다.
//
// 내부 글 링크(`/p/...`)는 절대 카드가 되지 않는다. 스킴이 있는 http(s) 절대
// 주소만 본다.

var kindExtCard = gast.NewNodeKind("ExternalCard")

// extCardNode는 카드 하나다. 문단을 통째로 대신한다.
type extCardNode struct {
	gast.BaseBlock
	URL   string
	Title string
	Sub   string
	// YTVideo가 비어 있지 않으면 카드 대신 유튜브 재생 자리로 그린다
	// (youtube.go). YTList는 재생목록 id다.
	YTVideo string
	YTList  string
}

func (n *extCardNode) Kind() gast.NodeKind { return kindExtCard }

func (n *extCardNode) Dump(source []byte, level int) {
	gast.DumpHelper(n, source, level, map[string]string{
		"URL": n.URL, "Title": n.Title, "Sub": n.Sub,
	}, nil)
}

// externalURL은 카드로 만들 수 있는 주소인지 본다.
//
// http/https만 받는다. 상대 경로(`/p/...`)와 `mailto:` 같은 것은 카드가 아니다.
func externalURL(raw string) (*url.URL, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return nil, false
	}
	switch u.Scheme {
	case "http", "https":
		return u, true
	}
	return nil, false
}

// cardLabels는 카드에 찍을 두 줄을 정한다.
//
// 변환기는 캡션이 없는 bookmark의 링크 글자를 URL 그대로 둔다. 그럴 때 URL을
// 제목 자리에 그대로 쓰면 두 줄이 같은 말을 두 번 하게 된다. 그래서:
//
//	캡션이 있다  → 제목=캡션,   아래=호스트
//	캡션이 없다  → 제목=호스트, 아래=경로
func cardLabels(u *url.URL, linkText string) (title, sub string) {
	host := strings.TrimPrefix(u.Host, "www.")
	// 쿼리는 버린다. 노션이 호스팅하던 첨부파일 URL은 서명이 붙어 있어
	// 쿼리만 1,500자가 넘는다 — 카드에 보여줄 값이 아니다.
	path := strings.TrimSuffix(u.EscapedPath(), "/")

	text := strings.TrimSpace(linkText)
	if text != "" && text != u.String() && text != strings.TrimSuffix(u.String(), "/") {
		return text, host
	}
	if path == "" {
		return host, ""
	}
	return host, path
}

// linkLabel은 링크 안쪽 글자를 모은다. 꾸밈은 벗기고 글자만 쓴다.
func linkLabel(n gast.Node, source []byte) string {
	var b strings.Builder
	_ = gast.Walk(n, func(c gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		switch t := c.(type) {
		case *gast.Text:
			b.Write(t.Segment.Value(source))
		case *gast.String:
			b.Write(t.Value)
		}
		return gast.WalkContinue, nil
	})
	return b.String()
}

// soloLink는 문단이 링크 하나만 담고 있으면 그 주소와 글자를 돌려준다.
//
// 자식이 정확히 하나여야 한다. 앞뒤에 글자가 조금이라도 붙어 있으면 그건
// 문장이지 bookmark 블록이 아니다.
func soloLink(p *gast.Paragraph, source []byte) (*url.URL, string, bool) {
	if p.ChildCount() != 1 {
		return nil, "", false
	}
	switch n := p.FirstChild().(type) {
	case *gast.Link:
		u, ok := externalURL(string(n.Destination))
		if !ok {
			return nil, "", false
		}
		return u, linkLabel(n, source), true
	case *gast.AutoLink:
		// GFM의 Linkify가 만든 맨 URL이다. 글자가 곧 주소라 캡션이 없다.
		u, ok := externalURL(string(n.URL(source)))
		if !ok {
			return nil, "", false
		}
		return u, "", true
	}
	return nil, "", false
}

// extCardTransformer는 조건에 맞는 문단을 카드 노드로 바꾼다.
type extCardTransformer struct{}

func (t *extCardTransformer) Transform(doc *gast.Document, reader text.Reader, _ parser.Context) {
	source := reader.Source()

	// 걷는 도중에 트리를 고치면 순회가 흔들린다. 먼저 모으고 나중에 바꾼다.
	var found []*gast.Paragraph
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if !entering {
			return gast.WalkContinue, nil
		}
		if p, ok := n.(*gast.Paragraph); ok {
			if _, _, ok := soloLink(p, source); ok {
				found = append(found, p)
			}
			return gast.WalkSkipChildren, nil
		}
		return gast.WalkContinue, nil
	})

	for _, p := range found {
		u, label, ok := soloLink(p, source)
		if !ok {
			continue
		}
		title, sub := cardLabels(u, label)
		card := &extCardNode{URL: u.String(), Title: title, Sub: sub}
		if id, list := ytVideoID(u); id != "" {
			card.YTVideo, card.YTList = id, list
		}
		p.Parent().ReplaceChild(p.Parent(), p, card)
	}
}

// extCardIcon은 카드 왼쪽의 기본 아이콘이다.
//
// 지금은 호스트에 상관없이 같은 그림을 쓴다. 진짜 파비콘은 이관 때 받아
// DB에 넣기로 했다(CLAUDE.md "남은 일" 참고) — 글을 열 때마다 외부에서
// 불러오면 독자 IP가 제3자에게 가고 오프라인에서 빈칸이 되기 때문이다.
// 파비콘이 들어와도 바뀌는 건 이 자리뿐이고 카드 마크업은 그대로다.
const extCardIcon = `<svg viewBox="0 0 16 16" aria-hidden="true" focusable="false">` +
	`<path d="M6.9 9.1a2.6 2.6 0 0 0 3.9.3l2-2a2.6 2.6 0 0 0-3.7-3.7l-1.1 1.1"` +
	` fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>` +
	`<path d="M9.1 6.9a2.6 2.6 0 0 0-3.9-.3l-2 2a2.6 2.6 0 0 0 3.7 3.7l1.1-1.1"` +
	` fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>`

// extCardRenderer는 카드 노드를 HTML로 쓴다.
type extCardRenderer struct{}

func (r *extCardRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindExtCard, r.render)
}

func (r *extCardRenderer) render(w util.BufWriter, _ []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	if !entering {
		return gast.WalkContinue, nil
	}
	n, ok := node.(*extCardNode)
	if !ok {
		return gast.WalkContinue, nil
	}

	if n.YTVideo != "" {
		return renderYouTube(w, n)
	}

	// rel은 밖으로 나가는 링크에만 붙인다. 우리 주소가 상대 쪽 로그에
	// 남지 않게 하려는 것이다.
	if _, err := w.WriteString(`<a class="extcard" rel="noreferrer" href="`); err != nil {
		return gast.WalkStop, err
	}
	template.HTMLEscape(w, []byte(n.URL))
	if _, err := w.WriteString(`"><span class="extcard-ico" aria-hidden="true">` +
		extCardIcon + `</span><span class="extcard-txt"><span class="extcard-t">`); err != nil {
		return gast.WalkStop, err
	}
	template.HTMLEscape(w, []byte(n.Title))
	if _, err := w.WriteString(`</span>`); err != nil {
		return gast.WalkStop, err
	}
	if n.Sub != "" {
		if _, err := w.WriteString(`<span class="extcard-h">`); err != nil {
			return gast.WalkStop, err
		}
		template.HTMLEscape(w, []byte(n.Sub))
		if _, err := w.WriteString(`</span>`); err != nil {
			return gast.WalkStop, err
		}
	}
	if _, err := w.WriteString(`</span><span class="extcard-go" aria-hidden="true">&#8599;</span></a>` + "\n"); err != nil {
		return gast.WalkStop, err
	}
	return gast.WalkContinue, nil
}

// extCardExtension은 위 변환기와 렌더러를 goldmark에 끼운다.
type extCardExtension struct{}

func (e *extCardExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(&extCardTransformer{}, 100),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&extCardRenderer{}, 100),
	))
}
