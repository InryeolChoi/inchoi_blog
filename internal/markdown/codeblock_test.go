package markdown

import (
	"strings"
	"testing"
)

func TestCodeBlockLanguageLabel(t *testing.T) {
	got := render(t, "```python\nprint(1)\n```\n")
	if !strings.Contains(got, `<div class="codeblock">`) {
		t.Errorf("껍데기가 없다:\n%s", got)
	}
	if !strings.Contains(got, `<span class="lang">python</span>`) {
		t.Errorf("언어 라벨이 없다:\n%s", got)
	}
	// 라벨은 <pre>보다 먼저 나와야 CSS가 오른쪽 위에 얹을 수 있다.
	if strings.Index(got, `class="lang"`) > strings.Index(got, "<pre>") {
		t.Errorf("라벨이 pre 뒤에 있다:\n%s", got)
	}
	// 하이라이팅이 보는 클래스는 그대로 남아야 한다.
	if !strings.Contains(got, `class="language-python"`) {
		t.Errorf("언어 클래스가 없다:\n%s", got)
	}
}

// text 265건은 "언어를 모른다"는 뜻이라 라벨을 붙이지 않는다.
// 다만 클래스는 남겨야 highlight-init.js가 "칠하지 말 것"으로 알아본다.
func TestCodeBlockSkipsLabelForUnknownLanguage(t *testing.T) {
	for _, lang := range []string{"text", "plaintext", "plain", "txt", "none"} {
		got := render(t, "```"+lang+"\n무언가\n```\n")
		if strings.Contains(got, `class="lang"`) {
			t.Errorf("%s에 라벨이 붙었다:\n%s", lang, got)
		}
		if !strings.Contains(got, `class="language-`+lang+`"`) {
			t.Errorf("%s 클래스가 사라졌다:\n%s", lang, got)
		}
	}
}

func TestCodeBlockWithoutLanguage(t *testing.T) {
	got := render(t, "```\n그냥 코드\n```\n")
	if strings.Contains(got, `class="lang"`) {
		t.Errorf("언어가 없는데 라벨이 붙었다:\n%s", got)
	}
	if strings.Contains(got, "language-") {
		t.Errorf("언어가 없는데 클래스가 붙었다:\n%s", got)
	}
	if !strings.Contains(got, "그냥 코드") {
		t.Errorf("코드 내용이 없다:\n%s", got)
	}
}

// 정보 문자열은 사람이 손으로 적은 것이라 무엇이든 들어온다. 라벨은 화면에
// 그대로 나가므로 형태가 이상하면 라벨만 생략한다.
func TestCodeLabelRejectsOddInfoStrings(t *testing.T) {
	cases := map[string]string{
		"python":                         "python",
		"Python":                         "python",
		"python title=x.py":              "python", // 첫 낱말만 본다
		"c++":                            "c++",
		"objective-c":                    "objective-c",
		"":                               "",
		"   ":                            "",
		"<script>":                       "",
		"한국어":                            "",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": "", // 너무 길다
	}
	for info, want := range cases {
		if got := codeLabel([]byte(info)); got != want {
			t.Errorf("codeLabel(%q) = %q, want %q", info, got, want)
		}
	}
}

// 코드 안의 <, & 는 그대로 글자로 보여야 한다.
func TestCodeBlockEscapes(t *testing.T) {
	got := render(t, "```c\nif (a < b && c > d) {}\n```\n")
	if !strings.Contains(got, "a &lt; b &amp;&amp; c &gt; d") {
		t.Errorf("이스케이프가 안 됐다:\n%s", got)
	}
}
