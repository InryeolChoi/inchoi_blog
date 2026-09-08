package markdown

import "strings"

// 마크다운을 **사람이 문단으로 느끼는 단위**로 자른다.
//
// # 왜 필요한가
//
// 블록 인라인 편집기가 쓴다. 평소에는 그려진 화면만 보이다가 문단을 누르면 그
// 문단만 원문이 되는데, 그러려면 "여기부터 여기까지가 한 문단"을 알아야 한다.
//
// # 왜 빈 줄만으로는 안 되나
//
// 빈 줄로 자르면 **한 덩어리로 다뤄야 하는 것이 쪼개진다.** 코드 블록 안의 빈
// 줄에서 잘리면 반쪽짜리 코드 두 개가 되고, 여는 백틱만 든 조각은 그 자체로
// 마크다운이 아니다. 블록 수식도, 항목 사이가 빈 목록도 같다.
//
// # 무엇을 보장하나
//
// **잘라서 다시 `\n\n`으로 이어 붙이면 원문과 같은 화면이 나온다.** 이게 이
// 함수의 계약이다 — 저장은 언제나 이어 붙인 문자열을 보내므로, 이게 깨지면
// 글을 열었다 닫기만 해도 본문이 달라진다. 실제 글 1,252편으로 확인한다
// (blocks_test.go).
func SplitBlocks(src string) []string {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	var out []string
	var cur []string

	flush := func() {
		block := strings.Trim(strings.Join(cur, "\n"), "\n")
		if strings.TrimSpace(block) != "" {
			out = append(out, block)
		}
		cur = nil
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// ① 코드 울타리. 여는 줄과 같은 기호로 닫힐 때까지 통째로 한 덩어리다.
		//    **닫는 줄이 없으면 끝까지 간다** — 그건 원문이 그런 것이라,
		//    억지로 끊으면 없는 코드 블록을 지어내는 셈이다.
		if fence := fenceOf(trimmed); fence != "" {
			flush()
			cur = append(cur, line)
			for i++; i < len(lines); i++ {
				cur = append(cur, lines[i])
				if closesFence(strings.TrimSpace(lines[i]), fence) {
					break
				}
			}
			flush()
			continue
		}

		// ② 블록 수식. 여는 `$$`와 닫는 `$$` 사이는 한 덩어리다. 변환기가 그
		//    안에 빈 줄을 남기지 않지만(normalizeBlockMath), 사람이 웹에서 쓴
		//    글에는 있을 수 있다.
		//
		//    **문단 한가운데의 `$$`에서는 안 끊는다**(`len(cur) == 0`). 거기서
		//    끊으면 한 문단이 셋으로 갈리는데, 원문에서는 그것이 문단 안의
		//    수식이라 화면이 달라진다 — 실제로 테스트가 잡았다.
		if trimmed == "$$" && len(cur) == 0 {
			flush()
			cur = append(cur, line)
			for i++; i < len(lines); i++ {
				cur = append(cur, lines[i])
				if strings.TrimSpace(lines[i]) == "$$" {
					break
				}
			}
			flush()
			continue
		}
		// ③ 빈 줄. 여기서 끊는데, **목록 안이면 안 끊는다** — 항목 사이가 빈
		//    목록(느슨한 목록)은 CommonMark에서 한 목록이고, 쪼개면 화면에서
		//    항목 간격이 달라진다.
		if trimmed == "" {
			if inList(cur) && continuesList(lines, i+1) {
				cur = append(cur, line)
				continue
			}
			flush()
			continue
		}

		// ④ 제목은 언제나 혼자 선다. 바로 아래 문단과 붙여두면 제목을 고치려고
		//    눌렀을 때 본문까지 통째로 원문이 된다.
		if strings.HasPrefix(trimmed, "#") && headingLevel(trimmed) > 0 {
			flush()
			out = append(out, line)
			continue
		}

		cur = append(cur, line)
	}
	flush()
	return out
}

// fenceOf는 이 줄이 여는 코드 울타리면 그 기호를, 아니면 빈 문자열을 준다.
func fenceOf(trimmed string) string {
	for _, mark := range []string{"```", "~~~"} {
		if strings.HasPrefix(trimmed, mark) {
			return mark
		}
	}
	return ""
}

// closesFence는 이 줄이 그 울타리를 닫는지 본다. 닫는 줄에는 정보 문자열이
// 붙지 않으므로 기호만 있어야 한다.
func closesFence(trimmed, fence string) bool {
	return strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, strings.TrimSpace(fence[:1])) == ""
}

// headingLevel은 ATX 제목의 단계다. 아니면 0이다.
//
// `#`이 여섯 개까지고 그 뒤에 공백이 와야 한다. `#태그`처럼 붙여 쓴 것은
// 제목이 아니다 — CommonMark의 규칙이자, 본문에 그런 줄이 실제로 있다.
func headingLevel(trimmed string) int {
	n := 0
	for n < len(trimmed) && trimmed[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return 0
	}
	if n == len(trimmed) {
		return n // `##`만 있는 줄도 빈 제목이다.
	}
	if trimmed[n] == ' ' || trimmed[n] == '\t' {
		return n
	}
	return 0
}

// inList는 지금 모으고 있는 덩어리가 목록인지 본다.
func inList(cur []string) bool {
	for _, l := range cur {
		if listMarker(l) {
			return true
		}
	}
	return false
}

// continuesList는 빈 줄 **다음** 줄이 그 목록을 이어가는지 본다.
// 다음 항목이거나 들여쓴 이어짐이면 같은 목록이다.
func continuesList(lines []string, i int) bool {
	if i >= len(lines) {
		return false
	}
	l := lines[i]
	if strings.TrimSpace(l) == "" {
		return false // 빈 줄이 둘이면 목록이 끝난 것으로 본다.
	}
	return listMarker(l) || strings.HasPrefix(l, "  ") || strings.HasPrefix(l, "\t")
}

// listMarker는 이 줄이 목록 항목으로 시작하는지 본다. 들여쓴 항목도 항목이다.
func listMarker(line string) bool {
	t := strings.TrimLeft(line, " \t")
	if t == "" {
		return false
	}
	if (t[0] == '-' || t[0] == '*' || t[0] == '+') && len(t) > 1 && (t[1] == ' ' || t[1] == '\t') {
		return true
	}
	n := 0
	for n < len(t) && t[n] >= '0' && t[n] <= '9' {
		n++
	}
	if n > 0 && n < len(t) && (t[n] == '.' || t[n] == ')') &&
		n+1 < len(t) && (t[n+1] == ' ' || t[n+1] == '\t') {
		return true
	}
	return false
}
