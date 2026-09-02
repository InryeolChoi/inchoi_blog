package markdown

import (
	"unicode"
	"unicode/utf8"
)

// CommonMark의 강조 짝 규칙.
//
// # 왜 여기 있나
//
// 이건 노션의 사정도 GitHub의 사정도 아니라 **CommonMark의 규칙**이다. 두
// 이관 도구가 같은 자리에서 같은 판정을 해야 하는데, 각자 들고 있으면 언젠가
// 갈라진다 — 렌더러와 CSS를 admin이 공개 쪽에서 가져다 쓰는 것과 같은 이유다.
//
// 원래는 `internal/notion`에만 있었다. GitHub에서 옮겨온 마크다운에도 같은
// 자리가 94군데 있어서(2026-09-02) 정본을 여기로 옮겼다.

// CanPairEmphasis는 `**알맹이**` 꼴이 CommonMark에서 짝이 되는지 본다.
//
// 규칙은 "여는 기호는 왼쪽으로, 닫는 기호는 오른쪽으로 붙어야 한다"인데,
// **문장부호가 끼면 조건이 하나 더 붙는다**: 닫는 기호 바로 앞이 문장부호면
// 기호 뒤가 공백이거나 문장부호여야 한다. 한국어에서 이게 자주 걸린다 —
// `**릴레이션(relation)**이라고`는 닫는 기호 앞이 `)`이고 뒤가 `이`라서
// 짝이 안 지어지고 별표가 글자로 보인다.
//
// prev는 앞 글자, next는 뒤 글자다. 줄의 처음과 끝(0)은 공백으로 친다.
func CanPairEmphasis(core string, prev, next rune) bool {
	open, _ := utf8.DecodeRuneInString(core)
	closing, _ := utf8.DecodeLastRuneInString(core)
	// 여는 기호: 알맹이 첫 글자가 문장부호면 기호 앞이 공백이나 문장부호여야 한다.
	if IsPunct(open) && !IsSpaceOrEdge(prev) && !IsPunct(prev) {
		return false
	}
	// 닫는 기호: 알맹이 끝 글자가 문장부호면 기호 뒤가 공백이나 문장부호여야 한다.
	if IsPunct(closing) && !IsSpaceOrEdge(next) && !IsPunct(next) {
		return false
	}
	return true
}

// IsSpaceOrEdge는 공백이거나 줄의 끝(0)인지 본다.
func IsSpaceOrEdge(r rune) bool { return r == 0 || unicode.IsSpace(r) }

// IsPunct는 CommonMark가 말하는 문장부호다. 기호(S 범주)도 여기 든다 —
// `✅`나 `→` 같은 것도 문장부호로 친다.
func IsPunct(r rune) bool { return unicode.IsPunct(r) || unicode.IsSymbol(r) }
