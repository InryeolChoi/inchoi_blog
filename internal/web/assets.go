package web

import (
	"html/template"
	"regexp"
	"strings"
)

// assetNeeds는 이 페이지가 CDN에서 무엇을 받아야 하는지다.
//
// 예전에는 모든 페이지가 KaTeX와 highlight.js를 무조건 받았다. 수식이 있는 글은
// 1408편 중 297편, 언어가 붙은 코드 블록이 있는 글은 그보다 조금 많을 뿐인데
// 나머지 글까지 스타일시트와 스크립트를 네 개씩 받았다.
//
// 본문 HTML을 보고 정한다. 서버가 이미 .math와 language-* 클래스를 붙여뒀으므로
// 그 표시를 세면 된다 — 원문 마크다운을 다시 파싱할 필요가 없다.
type assetNeeds struct {
	// Math는 .math 요소가 있는지다. KaTeX가 그것만 골라서 그린다.
	Math bool
	// Code는 **색칠될** 코드 블록이 있는지다. 아래 skipLangs는 색칠 대상이
	// 아니라서 그것만 있는 글에는 highlight.js를 받지 않는다.
	Code bool
	// Langs는 본문에 나온 언어 이름이다. common 묶음에 없어서 따로 받는
	// 언어(latex, dockerfile, powershell)를 그때만 받으려고 둔다.
	Langs map[string]bool
}

// Lang은 템플릿에서 쓴다. {{if .Assets.Lang "latex"}} 꼴.
func (a assetNeeds) Lang(name string) bool { return a.Langs[name] }

// langClass는 코드 블록에 붙은 언어 클래스를 잡는다.
// internal/markdown/codeblock.go가 넣는 형태와 같아야 한다.
var langClass = regexp.MustCompile(`class="language-([\w+#-]+)"`)

// skipLangs는 언어 이름이 붙어 있어도 색칠하지 않는 값이다.
// **static/highlight-init.js의 같은 목록과 맞춰야 한다.** 한쪽만 고치면
// 스크립트를 안 받았는데 칠할 것이 있거나, 받았는데 칠할 것이 없다.
var skipLangs = map[string]bool{"text": true, "plaintext": true, "plain": true, "none": true}

// needsFor는 본문 HTML을 보고 받아야 할 것을 정한다.
func needsFor(body template.HTML) assetNeeds {
	s := string(body)
	n := assetNeeds{Langs: map[string]bool{}}

	// math.go가 내보내는 형태는 <span class="math math-inline"> / <div class="math math-display">다.
	n.Math = strings.Contains(s, `class="math math-`)

	for _, m := range langClass.FindAllStringSubmatch(s, -1) {
		lang := strings.ToLower(m[1])
		n.Langs[lang] = true
		if !skipLangs[lang] {
			n.Code = true
		}
	}
	return n
}
