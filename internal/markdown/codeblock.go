package markdown

import (
	"html/template"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// 코드 블록에 언어 이름을 라벨로 붙인다.
//
// goldmark 기본 렌더러는 `<pre><code class="language-python">`까지만 낸다.
// 라벨을 놓을 자리가 없어서 바깥에 껍데기를 하나 두른다:
//
//	<div class="codeblock"><span class="lang">python</span><pre><code …>
//
// **브라우저가 아니라 서버에서 붙인다.** 언어는 이미 본문에 적혀 있으니 JS를
// 기다릴 이유가 없고, 스크립트가 못 떠도 라벨은 남는다. highlight.js는 여전히
// 안쪽 `code.language-*`만 보므로 하이라이팅은 그대로다.

// labelledLanguage는 라벨로 찍어도 되는 언어 이름인지 본다.
//
// 정보 문자열은 사람이 손으로 적은 것이라 무엇이든 들어올 수 있다. 라벨은
// 화면에 그대로 나가므로 형태를 좁혀둔다. 여기 안 걸리면 라벨만 생략하고
// `class="language-…"`는 그대로 낸다 — 하이라이팅 쪽 판단은 브라우저 몫이다.
var labelledLanguage = regexp.MustCompile(`^[a-z0-9][a-z0-9+#._-]{0,19}$`)

// unlabelledLanguages는 라벨을 붙이지 않는 언어다.
//
// `text`는 변환기가 "언어를 모른다"는 뜻으로 붙인 것이지 언어 이름이 아니다.
// 265건이나 되는데 전부 `text`라고 찍으면 알려주는 것 없이 시선만 끈다.
// highlight-init.js가 이 값을 하이라이팅에서 빼는 것과 같은 이유다.
var unlabelledLanguages = map[string]bool{
	"text":      true,
	"plaintext": true,
	"plain":     true,
	"txt":       true,
	"none":      true,
}

// codeLabel은 정보 문자열에서 라벨로 쓸 언어 이름을 뽑는다.
// 라벨을 붙이지 않을 때는 빈 문자열을 준다.
func codeLabel(info []byte) string {
	lang := strings.ToLower(strings.TrimSpace(string(info)))
	if i := strings.IndexAny(lang, " \t"); i >= 0 {
		lang = lang[:i]
	}
	if lang == "" || unlabelledLanguages[lang] || !labelledLanguage.MatchString(lang) {
		return ""
	}
	return lang
}

// codeBlockRenderer는 펜스 코드 블록을 껍데기로 감싸 그린다.
type codeBlockRenderer struct{}

func (r *codeBlockRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(gast.KindFencedCodeBlock, r.render)
}

func (r *codeBlockRenderer) render(w util.BufWriter, source []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	n := node.(*gast.FencedCodeBlock)
	if !entering {
		_, err := w.WriteString("</code></pre></div>\n")
		return gast.WalkContinue, err
	}

	if _, err := w.WriteString(`<div class="codeblock">`); err != nil {
		return gast.WalkStop, err
	}
	// 라벨을 <pre>보다 먼저 낸다. CSS가 이걸 오른쪽 위에 얹는다.
	if lang := codeLabel(n.Language(source)); lang != "" {
		if _, err := w.WriteString(`<span class="lang">` + lang + `</span>`); err != nil {
			return gast.WalkStop, err
		}
	}
	if _, err := w.WriteString("<pre><code"); err != nil {
		return gast.WalkStop, err
	}
	// class는 하이라이팅이 보는 값이라 라벨 규칙과 별개로 원문을 따른다.
	// 라벨을 생략한 `text`도 여기에는 남는다.
	if lang := n.Language(source); len(lang) > 0 {
		if _, err := w.WriteString(` class="language-`); err != nil {
			return gast.WalkStop, err
		}
		template.HTMLEscape(w, lang)
		if _, err := w.WriteString(`"`); err != nil {
			return gast.WalkStop, err
		}
	}
	if err := w.WriteByte('>'); err != nil {
		return gast.WalkStop, err
	}

	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		template.HTMLEscape(w, line.Value(source))
	}
	return gast.WalkContinue, nil
}

// codeBlockExtension은 위 렌더러를 goldmark에 끼운다.
type codeBlockExtension struct{}

func (e *codeBlockExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		// 기본 HTML 렌더러는 1000에 등록돼 있다. 그보다 앞이어야 이긴다.
		util.Prioritized(&codeBlockRenderer{}, 100),
	))
}
