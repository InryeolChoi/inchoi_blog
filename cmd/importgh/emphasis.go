package main

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/inryeol/blog/internal/markdown"
)

// 짝이 안 지어지는 `**굵게**`를 `<strong>`으로 바꾼다.
//
// # 무엇이 문제인가
//
// CommonMark는 닫는 기호 바로 앞이 문장부호면 **기호 뒤가 공백이거나
// 문장부호여야** 짝으로 인정한다. 한국어에서 이게 자주 걸린다 —
// `**경로 표현(Path Expression)**으로`는 닫는 기호 앞이 `)`이고 뒤가 `으`라서
// 짝이 안 지어지고 **별표가 그대로 화면에 나온다.**
//
// 노션 이관은 이미 같은 자리를 `<strong>`으로 내보내고 있었다(186자리).
// GitHub에서 옮겨온 마크다운에도 94자리가 있어서 같은 처리를 여기에 둔다.
// 판정 규칙은 `internal/markdown`이 정본이라 두 도구가 갈라지지 않는다.
//
// # 왜 마크다운으로 안 두나
//
// 본문은 사람이 웹에서 고칠 것이라 되도록 마크다운으로 두는 편이 낫다.
// 그래서 **짝이 안 되는 자리만** HTML로 낸다 — 나머지 `**굵게**`는 그대로다.

// boldSpan은 한 줄 안의 `**알맹이**`를 찾는다.
//
// **여러 줄에 걸친 것은 안 본다.** 마크다운에서 굵게가 문단을 넘는 일은
// 드물고, 줄 단위로 보면 코드 펜스를 세는 것과 같은 흐름으로 처리할 수 있다.
var boldSpan = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)

// fixUnpairedBold는 짝이 안 되는 굵게 표시를 `<strong>`으로 바꾼다.
// 바꾼 자리의 알맹이를 함께 돌려준다(리포트용).
func fixUnpairedBold(body string) (string, []string) {
	lines := strings.Split(body, "\n")
	var fixed []string
	inFence := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		// **코드 블록 안은 건드리지 않는다.** 거기서는 `**`가 글자로 나오는
		// 것이 맞다 — 파이썬 거듭제곱 `n**2`가 그렇다. escapeBareTags가
		// 같은 이유로 펜스를 세는 것과 같다.
		if inFence {
			continue
		}
		out, got := fixLineBold(line)
		lines[i] = out
		fixed = append(fixed, got...)
	}
	return strings.Join(lines, "\n"), fixed
}

func fixLineBold(line string) (string, []string) {
	locs := boldSpan.FindAllStringSubmatchIndex(line, -1)
	if locs == nil {
		return line, nil
	}
	var b strings.Builder
	var fixed []string
	last := 0
	for _, m := range locs {
		start, end := m[0], m[1]
		core := line[m[2]:m[3]]
		// 인라인 코드 안이면 건드리지 않는다. `` `n**2` `` 같은 것이다.
		if inInlineCode(line, start) {
			continue
		}
		prev := lastRuneOf(line[:start])
		next := firstRuneOf(line[end:])
		if markdown.CanPairEmphasis(core, prev, next) {
			continue
		}
		b.WriteString(line[last:start])
		b.WriteString("<strong>")
		b.WriteString(core)
		b.WriteString("</strong>")
		last = end
		fixed = append(fixed, core)
	}
	if last == 0 {
		return line, nil
	}
	b.WriteString(line[last:])
	return b.String(), fixed
}

// inInlineCode는 그 자리가 백틱 쌍 안인지 본다. 앞쪽의 백틱 수가 홀수면 안이다.
func inInlineCode(line string, at int) bool {
	return strings.Count(line[:at], "`")%2 == 1
}

func lastRuneOf(s string) rune {
	if s == "" {
		return 0
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	return r
}

func firstRuneOf(s string) rune {
	if s == "" {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(s)
	return r
}
