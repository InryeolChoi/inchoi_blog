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
	// Copy는 복사 버튼을 달 코드 블록이 있는지다.
	//
	// **Code를 재활용하면 안 된다.** 그건 "색칠할 것이 있는가"이고, `text`
	// 265건처럼 일부러 색칠을 건너뛰는 블록만 있는 글은 Code가 거짓인데 복사할
	// 코드는 그대로 있다. 껍데기 없는 <pre>(들여쓰기 코드 블록)도 마찬가지라
	// 언어 클래스가 아니라 <pre> 자체를 센다.
	Copy bool
	// Mermaid는 그릴 다이어그램이 있는지다. mermaid는 3.5MB짜리라 나오는
	// 페이지에서만 받는다 — 지금 21편이다.
	Mermaid bool
	// Anim은 이름 붙인 애니메이션 자리가 있는지다. 있을 때만 static/anim.js를 받는다.
	Anim bool
	// YouTube는 유튜브 재생 자리가 있는지다. 있을 때만 static/youtube.js를 받는다.
	YouTube bool
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
//
// **mermaid가 여기 있는 이유는 다르다.** 앞의 넷은 "칠하지 말라"는 뜻이지만
// mermaid는 hljs에 아예 없는 언어다. 예전에는 그 30건 때문에 highlight.js를
// 받아놓고 정작 칠할 것이 없었다 — 이제 그 블록은 다이어그램으로 그려진다.
var skipLangs = map[string]bool{"text": true, "plaintext": true, "plain": true, "none": true, "mermaid": true}

// needsFor는 본문 HTML을 보고 받아야 할 것을 정한다.
func needsFor(body template.HTML) assetNeeds {
	s := string(body)
	n := assetNeeds{Langs: map[string]bool{}}

	// math.go가 내보내는 형태는 <span class="math math-inline"> / <div class="math math-display">다.
	n.Math = strings.Contains(s, `class="math math-`)
	n.YouTube = strings.Contains(s, `class="ytembed"`)
	// anim.go가 내보내는 형태는 <div class="anim" data-anim="이름">이다.
	n.Anim = strings.Contains(s, `class="anim" data-anim=`)
	// 본문에 진짜 <pre>가 있을 때만이다. 코드 안에 적힌 `&lt;pre` 글자는
	// 이미 이스케이프돼 있어서 걸리지 않는다.
	n.Copy = strings.Contains(s, "<pre")

	for _, m := range langClass.FindAllStringSubmatch(s, -1) {
		lang := strings.ToLower(m[1])
		n.Langs[lang] = true
		if lang == "mermaid" {
			n.Mermaid = true
		}
		if !skipLangs[lang] {
			n.Code = true
		}
	}
	return n
}
