package web

import (
	"html/template"
	"testing"
)

func TestNeedsForMath(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{"인라인 수식", `<p>값은 <span class="math math-inline">\sqrt x</span>다</p>`, true},
		{"블록 수식", `<div class="math math-display">x = 1</div>`, true},
		{"수식 없음", `<p>그냥 글이다</p>`, false},
		{"math라는 글자만", `<p>math를 배웠다</p>`, false},
	} {
		if got := needsFor(template.HTML(tc.body)).Math; got != tc.want {
			t.Errorf("%s: Math=%v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestNeedsForCodeMatchesHighlightInit은 서버의 판단이 highlight-init.js와
// 같은지 본다. 한쪽만 바뀌면 스크립트를 안 받았는데 칠할 것이 있거나,
// 받았는데 칠할 것이 없다.
func TestNeedsForCodeMatchesHighlightInit(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{"python", `<pre><code class="language-python">x = 1</code></pre>`, true},
		{"text는 칠하지 않는다", `<pre><code class="language-text">그냥</code></pre>`, false},
		{"plaintext도", `<pre><code class="language-plaintext">그냥</code></pre>`, false},
		{"언어 없는 코드", `<pre><code>그냥</code></pre>`, false},
		{"text와 python이 같이", `<pre><code class="language-text">a</code></pre><pre><code class="language-python">b</code></pre>`, true},
		{"코드 없음", `<p>글만 있다</p>`, false},
		// mermaid는 hljs에 아예 없는 언어다. 예전에는 그 30건 때문에
		// highlight.js를 받아놓고 정작 칠할 것이 없었다.
		{"mermaid만", `<pre><code class="language-mermaid">graph LR</code></pre>`, false},
		{"mermaid와 python이 같이", `<pre><code class="language-mermaid">graph LR</code></pre><pre><code class="language-python">b</code></pre>`, true},
	} {
		if got := needsFor(template.HTML(tc.body)).Code; got != tc.want {
			t.Errorf("%s: Code=%v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestNeedsForCopyCountsPreNotHighlighting은 복사 버튼의 판정이 하이라이팅
// 판정과 **다른 것**인지 본다.
//
// Code를 재활용하면 `text`만 있는 글(265건)과 껍데기 없는 <pre>에서 버튼이
// 사라진다. 색칠하지 않는 코드도 복사할 코드다.
func TestNeedsForCopyCountsPreNotHighlighting(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{"펜스 코드 블록", `<div class="codeblock"><span class="lang">python</span><pre><code class="language-python">x = 1</code></pre></div>`, true},
		{"text만 있어도 센다", `<pre><code class="language-text">그냥</code></pre>`, true},
		{"껍데기 없는 pre", `<pre><code>들여쓴 코드</code></pre>`, true},
		{"코드 없음", `<p>글만 있다</p>`, false},
		{"본문에 적힌 pre 글자", `<p>여기서 &lt;pre&gt;를 설명한다</p>`, false},
	} {
		if got := needsFor(template.HTML(tc.body)).Copy; got != tc.want {
			t.Errorf("%s: Copy=%v, want %v", tc.name, got, tc.want)
		}
	}

	// text만 있는 글은 색칠은 안 하지만 복사는 한다 — 두 판정이 갈리는 자리다.
	n := needsFor(template.HTML(`<pre><code class="language-text">그냥</code></pre>`))
	if n.Code {
		t.Error("text만 있는데 highlight.js를 받으려 한다")
	}
	if !n.Copy {
		t.Error("text만 있는 글에서 복사 버튼이 빠졌다")
	}
}

func TestNeedsForExtraLanguages(t *testing.T) {
	n := needsFor(template.HTML(`<pre><code class="language-latex">\alpha</code></pre>`))
	if !n.Lang("latex") {
		t.Error("latex를 못 잡았다")
	}
	if n.Lang("powershell") {
		t.Error("없는 언어를 잡았다")
	}
}

// TestNeedsForMermaid는 3.5MB짜리 mermaid를 **그릴 것이 있는 페이지에서만**
// 받는지 본다. KaTeX·highlight.js를 페이지별로 가른 것과 같은 이유이고,
// 여기서는 그 무게 차이가 훨씬 크다.
func TestNeedsForMermaid(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{"mermaid 코드 블록", `<pre><code class="language-mermaid">graph LR</code></pre>`, true},
		{"다른 언어", `<pre><code class="language-python">x = 1</code></pre>`, false},
		{"코드 없음", `<p>글만 있다</p>`, false},
		{"본문에 적힌 mermaid 글자", `<p>mermaid를 배웠다</p>`, false},
	} {
		if got := needsFor(template.HTML(tc.body)).Mermaid; got != tc.want {
			t.Errorf("%s: Mermaid=%v, want %v", tc.name, got, tc.want)
		}
	}

	// mermaid만 있는 글은 다이어그램은 그리지만 색칠할 것은 없다 —
	// 두 판정이 갈리는 자리다.
	n := needsFor(template.HTML(`<pre><code class="language-mermaid">graph LR</code></pre>`))
	if !n.Mermaid || n.Code {
		t.Errorf("mermaid만 있는 글: Mermaid=%v Code=%v, want true/false", n.Mermaid, n.Code)
	}
	// 복사는 한다. 그릴 수 없더라도 원문은 코드다.
	if !n.Copy {
		t.Error("mermaid 블록에 복사 버튼이 안 붙는다")
	}
}
