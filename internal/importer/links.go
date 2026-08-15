package importer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// pageLinkPattern은 마크다운 링크의 경로가 /p/로 시작하는 것만 잡는다.
//
// 앞의 `](`가 필수라 마크다운 링크 형식만 걸린다. 그래서 본문에 있는
// https://app.notion.com/p/... 같은 외부 URL은 걸리지 않는다 — 경로가 /p/로
// 시작하지 않고 호스트 뒤에 붙어 있기 때문이다. 링크 텍스트([...])는 건드리지 않는다.
var pageLinkPattern = regexp.MustCompile(`\]\(/p/([^)\s]+)\)`)

// notionInlineLinkPattern은 노션이 본문 인라인 링크에 쓰는 경로를 잡는다.
//
// 하위 페이지 블록과 달리 이건 rich_text의 href에 그대로 들어 있던 값이라
// /p/ 접두사가 없고, 페이지 ID에서 하이픈을 뺀 32자리 16진수다.
// 뒤에 #조각이 붙으면 페이지 안의 특정 블록을 가리킨다. 조각은 그대로 둔다.
var notionInlineLinkPattern = regexp.MustCompile(`\]\(/([0-9a-f]{32})(#[0-9a-f]{32})?\)`)

// LinkRewrite는 링크 재작성 결과다.
type LinkRewrite struct {
	// PageLinks는 /p/{페이지ID} 형태에서 바뀐 링크 수다.
	PageLinks int
	// InlineLinks는 노션 인라인 href(/{32자리}) 형태에서 바뀐 링크 수다.
	InlineLinks int
	// Unresolved는 대응하는 글을 못 찾아 그대로 둔 대상별 개수다.
	Unresolved map[string]int
}

// Replaced는 바뀐 링크의 총 개수다.
func (r LinkRewrite) Replaced() int { return r.PageLinks + r.InlineLinks }

// RewriteLinks는 본문의 /p/{notion_page_id} 링크를 /{slug}로 바꾼다.
//
// slugByPageID에 없는 대상은 바꾸지 않고 그대로 둔다. 가리킬 글이 없는데 경로만
// 바꾸면 깨진 링크가 멀쩡해 보이게 된다. 그런 건 Unresolved로 보고한다.
func RewriteLinks(body string, slugByPageID map[string]string) (string, LinkRewrite) {
	res := LinkRewrite{Unresolved: map[string]int{}}

	out := pageLinkPattern.ReplaceAllStringFunc(body, func(match string) string {
		m := pageLinkPattern.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		pageID := m[1]
		slug, ok := slugByPageID[pageID]
		if !ok {
			res.Unresolved[pageID]++
			return match
		}
		res.PageLinks++
		return "](/" + slug + ")"
	})

	// 노션 인라인 href도 같은 대상을 가리킨다. 여기서 안 바꾸면 사이트 안에서
	// 깨진 경로로 남는다. 키는 하이픈을 뺀 형태다.
	out = notionInlineLinkPattern.ReplaceAllStringFunc(out, func(match string) string {
		m := notionInlineLinkPattern.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		compact, fragment := m[1], m[2]
		slug, ok := slugByPageID[compact]
		if !ok {
			res.Unresolved[compact]++
			return match
		}
		res.InlineLinks++
		return "](/" + slug + fragment + ")"
	})

	return out, res
}

// SlugIndex는 페이지 ID로 slug를 찾는 표를 만든다.
//
// 노션 링크는 하이픈이 있는 형태(하위 페이지 블록)와 없는 형태(인라인 href)를
// 둘 다 쓰므로 양쪽 키를 모두 넣는다.
func SlugIndex(slugByPageID map[string]string) map[string]string {
	out := make(map[string]string, len(slugByPageID)*2)
	for pageID, slug := range slugByPageID {
		out[pageID] = slug
		out[strings.ReplaceAll(pageID, "-", "")] = slug
	}
	return out
}

// UnresolvedTargets는 대응하는 글을 못 찾은 대상을 많이 나온 순으로 돌려준다.
func (r LinkRewrite) UnresolvedTargets() []string {
	out := make([]string, 0, len(r.Unresolved))
	for id := range r.Unresolved {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool {
		if r.Unresolved[out[i]] != r.Unresolved[out[j]] {
			return r.Unresolved[out[i]] > r.Unresolved[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}

// UnresolvedCount는 그대로 둔 링크의 총 개수다.
func (r LinkRewrite) UnresolvedCount() int {
	n := 0
	for _, c := range r.Unresolved {
		n += c
	}
	return n
}

// CountPageLinks는 본문에 남은 /p/ 링크 수를 센다. 재작성 후 검증에 쓴다.
func CountPageLinks(body string) int {
	return len(pageLinkPattern.FindAllString(body, -1))
}

// CountNotionInlineLinks는 본문에 남은 노션 인라인 href 수를 센다.
func CountNotionInlineLinks(body string) int {
	return len(notionInlineLinkPattern.FindAllString(body, -1))
}

// slugLinkPattern은 재작성된 링크(/slug)를 잡는다. 검증용이다.
var slugLinkPattern = regexp.MustCompile(`\]\(/([^)/\s][^)\s]*)\)`)

// notionCompactID는 하이픈 없는 노션 페이지 ID다.
var notionCompactID = regexp.MustCompile(`^[0-9a-f]{32}$`)

// SlugLinkTargets는 본문에서 글을 가리키는 내부 링크의 slug를 모은다.
//
// slug가 아닌 것은 뺀다:
//   - 이미지 경로(/img/)
//   - 아직 못 바꾼 /p/ 링크
//   - 아직 못 바꾼 노션 인라인 링크(/{32자리 16진수})
//
// 뒤의 둘은 "가리킬 글이 없어 그대로 둔 링크"로 이미 따로 세고 있다. 여기서 또 세면
// 같은 링크가 "깨진 slug"로 한 번 더 보고돼서, 의도적으로 남긴 것과 진짜 버그가
// 구분되지 않는다.
//
// #조각은 떼고 경로 부분만 돌려준다.
func SlugLinkTargets(body string) []string {
	var out []string
	for _, m := range slugLinkPattern.FindAllStringSubmatch(body, -1) {
		target := m[1]
		if strings.HasPrefix(target, "img/") || strings.HasPrefix(target, "p/") {
			continue
		}
		if i := strings.IndexByte(target, '#'); i >= 0 {
			target = target[:i]
		}
		if target == "" || notionCompactID.MatchString(target) {
			continue
		}
		out = append(out, target)
	}
	return out
}

// ValidateRewritten은 재작성된 본문에 /p/가 남아 있지 않은지 확인한다.
func ValidateRewritten(body string) error {
	if n := CountPageLinks(body); n > 0 {
		return fmt.Errorf("/p/ 링크가 %d개 남아 있다", n)
	}
	return nil
}
