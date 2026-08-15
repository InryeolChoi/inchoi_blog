package importer

import (
	"strings"
	"unicode"
)

// pathSeparator는 노션 경로를 잇는 구분자다. ("운영체제 > part 4 > ...")
const pathSeparator = ">"

// PathAncestors는 original_path에서 글 자신을 뺀 조상 이름들을 돌려준다.
//
// original_path의 마지막 요소는 글 제목 자신이다(덤프 1311건 전부 그렇다).
// 그대로 카테고리로 쓰면 글이 자기 제목과 같은 이름의 카테고리에 들어간다.
// 그래서 마지막 요소는 항상 뺀다.
//
//	"운영체제 > part 4 > 공룡책 9장 > 3. 페이징"  →  [운영체제, part 4, 공룡책 9장]
//	"école 42 > Netpractice"                      →  [école 42]
//	"Netpractice"                                 →  []
func PathAncestors(originalPath string) []string {
	if strings.TrimSpace(originalPath) == "" {
		return nil
	}
	parts := strings.Split(originalPath, pathSeparator)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	// 마지막은 글 제목이므로 뺀다. 조상이 없으면 빈 슬라이스가 아니라 nil을 준다.
	if len(out) <= 1 {
		return nil
	}
	return out[:len(out)-1]
}

// Slugify는 카테고리 이름을 URL에 쓸 형태로 바꾼다.
//
// 소문자로 내리고 공백을 하이픈으로 바꾼 뒤, 글자와 숫자와 하이픈만 남긴다.
// 한글은 글자로 취급해 그대로 둔다 — 로마자로 옮기면 원래 이름을 알아볼 수 없다.
//
//	"Part 2"          → "part-2"
//	"수학 & 통계"      → "수학-통계"
//	"part 4 : 메모리"  → "part-4-메모리"
func Slugify(name string) string {
	var b strings.Builder
	prevHyphen := false

	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevHyphen = false
		case unicode.IsSpace(r), r == '-', r == '_', r == '/', r == ':', r == '.':
			// 구분자 구실을 하는 문자만 하이픈으로 바꾼다. 하이픈이 연달아 나오지 않게 한다.
			if !prevHyphen && b.Len() > 0 {
				b.WriteRune('-')
				prevHyphen = true
			}
		default:
			// 그 외 기호(&, (, ) 등)는 버린다.
		}
	}
	return strings.Trim(b.String(), "-")
}

// CategoryAssignment는 글 하나가 어느 카테고리에 들어갈지다.
type CategoryAssignment struct {
	NotionPageID string
	// Level1은 최상위 카테고리 이름이다. 빈 문자열이면 카테고리가 없다.
	Level1 string
	// Level2는 2단계 카테고리 이름이다. 빈 문자열이면 1단계까지만 있다.
	Level2 string
}

// Category는 자기 이름은 Level2가 있으면 Level2, 없으면 Level1이다.
// 글이 실제로 붙는 카테고리 이름을 돌려준다.
func (a CategoryAssignment) Leaf() string {
	if a.Level2 != "" {
		return a.Level2
	}
	return a.Level1
}

// AssignCategory는 경로에서 1단계와 2단계 카테고리를 뽑는다.
//
// 조상이 하나뿐이면(경로가 "école 42 > Netpractice" 처럼 짧으면) 2단계가 없다.
// 이때 글은 1단계 카테고리에 붙인다. 카테고리는 최대 2단계라 3번째 이후 조상은 버린다.
func AssignCategory(notionPageID, originalPath string) CategoryAssignment {
	a := CategoryAssignment{NotionPageID: notionPageID}
	anc := PathAncestors(originalPath)
	if len(anc) >= 1 {
		a.Level1 = anc[0]
	}
	if len(anc) >= 2 {
		a.Level2 = anc[1]
	}
	return a
}
