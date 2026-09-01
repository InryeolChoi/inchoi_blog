package web

import (
	"fmt"
	"html/template"
	"strings"
	"sync"
)

// SiteCSS는 이 사이트의 스타일시트 전체다. layout.html의 `sitecss` 템플릿에서
// 뽑아온다.
//
// **admin 화면(internal/admin)이 이걸 쓰라고 열어둔 것이다.** admin의 미리보기가
// 실제 글 화면과 다르게 보이면 그건 미리보기가 아니다. CSS를 복사해서 admin 쪽에
// 한 벌 더 두면 반드시 갈라지므로, 정본은 layout.html 하나로 두고 여기서 읽어간다.
//
// 결과에 Go 템플릿 액션은 없다(순수 CSS라 확인했다). 그래서 한 번 뽑아 캐시한다.
func SiteCSS() (template.CSS, error) {
	siteCSSOnce.Do(func() {
		t, err := template.ParseFS(templateFS, "templates/layout.html")
		if err != nil {
			siteCSSErr = fmt.Errorf("layout 파싱: %w", err)
			return
		}
		var b strings.Builder
		if err := t.ExecuteTemplate(&b, "sitecss", nil); err != nil {
			siteCSSErr = fmt.Errorf("sitecss 실행: %w", err)
			return
		}
		siteCSS = template.CSS(b.String())
	})
	return siteCSS, siteCSSErr
}

var (
	siteCSSOnce sync.Once
	siteCSS     template.CSS
	siteCSSErr  error
)

// PreviewAssetTags는 KaTeX·highlight.js·mermaid를 받아오는 <link>/<script> 태그다.
// layout.html의 `katex-cdn`, `hljs-cdn`, `mermaid-cdn`에서 그대로 뽑아온다.
//
// **admin 미리보기가 쓰라고 열어둔 것이다.** 공개 페이지는 본문에 수식이나 코드가
// 있을 때만 받지만(assets.go), admin은 작업 도구라 무엇을 칠지 미리 알 수 없어서
// 늘 받는다.
//
// 주소와 버전만 베끼고 **SRI 해시를 안 맞추면 브라우저가 아무 말 없이 실행을
// 거부한다.** 그래서 복사본을 두지 않고 여기서 읽어간다.
func PreviewAssetTags() (template.HTML, error) {
	previewTagsOnce.Do(func() {
		t, err := template.ParseFS(templateFS, "templates/layout.html")
		if err != nil {
			previewTagsErr = fmt.Errorf("layout 파싱: %w", err)
			return
		}
		var b strings.Builder
		for _, name := range []string{"katex-cdn", "hljs-cdn", "mermaid-cdn"} {
			if err := t.ExecuteTemplate(&b, name, nil); err != nil {
				previewTagsErr = fmt.Errorf("%s 실행: %w", name, err)
				return
			}
			b.WriteString("\n")
		}
		previewTags = template.HTML(b.String())
	})
	return previewTags, previewTagsErr
}

var (
	previewTagsOnce sync.Once
	previewTags     template.HTML
	previewTagsErr  error
)
