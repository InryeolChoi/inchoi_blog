package main

import (
	"strings"
	"testing"
)

// TestFixUnpairedBoldOnlyTouchesBrokenPairs는 **짝이 되는 것은 그대로 두는지**
// 본다. 본문은 사람이 웹에서 고칠 것이라 되도록 마크다운으로 남겨야 한다.
func TestFixUnpairedBoldOnlyTouchesBrokenPairs(t *testing.T) {
	in := "**멀쩡한 굵게**는 그대로다.\n**경로 표현(Path Expression)**으로 찾는다.\n"
	got, fixed := fixUnpairedBold(in)

	if !strings.Contains(got, "**멀쩡한 굵게**") {
		t.Errorf("짝이 되는 굵게를 건드렸다:\n%s", got)
	}
	if !strings.Contains(got, "<strong>경로 표현(Path Expression)</strong>") {
		t.Errorf("짝이 안 되는 굵게를 안 고쳤다:\n%s", got)
	}
	if len(fixed) != 1 {
		t.Errorf("고친 자리가 %d개다, want 1", len(fixed))
	}
}

// TestFixUnpairedBoldSkipsCodeFence는 코드 블록 안을 안 건드리는지 본다.
// 거기서는 `n**2`가 글자로 나오는 것이 맞다.
func TestFixUnpairedBoldSkipsCodeFence(t *testing.T) {
	in := "```python\nx = int(n**(0.5))**2이다\n```\n"
	got, fixed := fixUnpairedBold(in)

	if got != in {
		t.Errorf("코드 블록을 건드렸다:\n%s", got)
	}
	if len(fixed) != 0 {
		t.Errorf("코드 블록에서 %d개를 고쳤다", len(fixed))
	}
}

// TestFixUnpairedBoldSkipsInlineCode는 인라인 코드 안도 안 건드리는지 본다.
func TestFixUnpairedBoldSkipsInlineCode(t *testing.T) {
	in := "식은 `int(n**(0.5))**2`이다.\n"
	got, _ := fixUnpairedBold(in)

	if got != in {
		t.Errorf("인라인 코드를 건드렸다:\n%s", got)
	}
}
