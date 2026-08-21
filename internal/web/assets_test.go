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
	} {
		if got := needsFor(template.HTML(tc.body)).Code; got != tc.want {
			t.Errorf("%s: Code=%v, want %v", tc.name, got, tc.want)
		}
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
