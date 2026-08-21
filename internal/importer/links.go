package importer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// PostURLPrefix는 사이트에서 글 하나를 가리키는 경로 접두사다.
//
// 서버 라우트가 `GET /p/{slug}` 하나뿐이라 재작성 결과도 반드시 이 접두사를
// 달아야 한다. 접두사 없이 `/{slug}`로 쓰면 `GET /{l1}` 카테고리 라우트에
// 잡혀서 전부 404가 난다. 실제로 그렇게 됐다가 고쳤다.
const PostURLPrefix = "/p/"

// pageLinkPattern은 마크다운 링크의 경로가 /p/로 시작하는 것만 잡는다.
//
// 앞의 `](`가 필수라 마크다운 링크 형식만 걸린다. 그래서 본문에 있는
// https://app.notion.com/p/... 같은 외부 URL은 걸리지 않는다 — 경로가 /p/로
// 시작하지 않고 호스트 뒤에 붙어 있기 때문이다. 링크 텍스트([...])는 건드리지 않는다.
//
// #조각은 따로 잡는다. 재작성 결과도 /p/를 쓰므로 이 패턴은 이미 바뀐 링크도
// 다시 읽게 되는데, 조각을 경로에 붙여서 읽으면 slug를 못 알아본다.
var pageLinkPattern = regexp.MustCompile(`\]\(/p/([^)\s#]+)(#[^)\s]*)?\)`)

// notionInlineLinkPattern은 노션이 본문 인라인 링크에 쓰는 경로를 잡는다.
//
// 하위 페이지 블록과 달리 이건 rich_text의 href에 그대로 들어 있던 값이라
// /p/ 접두사가 없고, 페이지 ID에서 하이픈을 뺀 32자리 16진수다.
// 뒤에 #조각이 붙으면 페이지 안의 특정 블록을 가리킨다. 조각은 그대로 둔다.
var notionInlineLinkPattern = regexp.MustCompile(`\]\(/([0-9a-f]{32})(#[0-9a-f]{32})?\)`)

// notionAbsoluteLinkPattern은 노션 워크스페이스로 **나가는** 절대 URL을 잡는다.
//
// rich_text의 href가 상대경로가 아니라 전체 주소로 들어온 경우다. 그대로 두면
// 독자가 눌렀을 때 남의 비공개 노션으로 가서 아무것도 못 본다. 가리키는 글이
// 우리에게 있으면 그 글로 보낸다.
var notionAbsoluteLinkPattern = regexp.MustCompile(
	`\]\(https://(?:www\.|app\.)?notion\.(?:so|com)/(?:[^)/\s]+/)?(?:p/)?([0-9a-f]{32})(#[0-9a-f]{32})?\)`)

// bareLinkPattern은 /p/ 접두사가 없는 사이트 내부 절대경로를 잡는다.
//
// 옛 relink가 `/{slug}`로 써둔 링크를 찾아내려는 것이다. 여기 걸린 게 전부
// 글 링크인 것은 아니라서(`/img/...`, `/about`, 아직 못 바꾼 노션 링크 등)
// 경로가 실제 posts.slug일 때만 바꾼다.
var bareLinkPattern = regexp.MustCompile(`\]\(/([^)\s#/][^)\s#]*)(#[^)\s]*)?\)`)

// LinkRewrite는 링크 재작성 결과다.
type LinkRewrite struct {
	// PageLinks는 /p/{페이지ID} 형태에서 바뀐 링크 수다.
	PageLinks int
	// InlineLinks는 노션 인라인 href(/{32자리}) 형태에서 바뀐 링크 수다.
	InlineLinks int
	// AbsoluteLinks는 노션 워크스페이스로 나가던 절대 URL에서 바뀐 링크 수다.
	AbsoluteLinks int
	// BareLinks는 접두사 없는 /{slug} 형태에서 바뀐 링크 수다.
	// 옛 relink가 남긴 것으로, 라우트가 없어 404가 나던 것들이다.
	BareLinks int
	// Unresolved는 대응하는 글을 못 찾아 그대로 둔 대상별 개수다.
	Unresolved map[string]int
}

// Replaced는 바뀐 링크의 총 개수다.
func (r LinkRewrite) Replaced() int {
	return r.PageLinks + r.InlineLinks + r.AbsoluteLinks + r.BareLinks
}

// RewriteLinks는 본문의 노션 링크와 옛 형태 링크를 /p/{slug}로 바꾼다.
//
// slugByPageID에 없는 노션 대상은 바꾸지 않고 그대로 둔다. 가리킬 글이 없는데
// 경로만 바꾸면 깨진 링크가 멀쩡해 보이게 된다. 그런 건 Unresolved로 보고한다.
//
// slugs는 실제로 존재하는 posts.slug 집합이다. 두 군데 쓴다:
//   - 이미 /p/{slug}인 링크를 알아보고 건드리지 않는다 (멱등성)
//   - 접두사 없는 /{경로}가 글을 가리키는지 판별한다
func RewriteLinks(body string, slugByPageID map[string]string, slugs map[string]bool) (string, LinkRewrite) {
	res := LinkRewrite{Unresolved: map[string]int{}}

	out := pageLinkPattern.ReplaceAllStringFunc(body, func(match string) string {
		m := pageLinkPattern.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		target, fragment := m[1], m[2]
		// 이미 최종 형태다. slug가 노션 페이지 ID와 같은 동안에는 아래 조회도
		// 성공하지만, 그걸 교체로 세면 "두 번 돌려도 안 바뀐다"가 거짓이 된다.
		if slugs[target] {
			return match
		}
		slug, ok := slugByPageID[target]
		if !ok {
			res.Unresolved[target]++
			return match
		}
		res.PageLinks++
		return "](" + PostURLPrefix + slug + fragment + ")"
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
		return "](" + PostURLPrefix + slug + fragment + ")"
	})

	// 노션으로 나가는 절대 URL도 같은 대상을 가리킨다. 그대로 두면 독자가
	// 남의 비공개 워크스페이스로 간다.
	out = notionAbsoluteLinkPattern.ReplaceAllStringFunc(out, func(match string) string {
		m := notionAbsoluteLinkPattern.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		compact, fragment := m[1], m[2]
		slug, ok := slugByPageID[compact]
		if !ok {
			res.Unresolved[compact]++
			return match
		}
		res.AbsoluteLinks++
		return "](" + PostURLPrefix + slug + fragment + ")"
	})

	// 옛 relink가 접두사 없이 써둔 링크를 옮긴다. 위 두 단계가 만든 결과는
	// 경로가 `p/`로 시작해서 slugs에 없으므로 여기 다시 걸리지 않는다.
	out = bareLinkPattern.ReplaceAllStringFunc(out, func(match string) string {
		m := bareLinkPattern.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		target, fragment := m[1], m[2]
		if !slugs[target] {
			return match
		}
		res.BareLinks++
		return "](" + PostURLPrefix + target + fragment + ")"
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

// CountPostLinks는 실제 글을 가리키는 /p/ 링크 수를 센다.
func CountPostLinks(body string, slugs map[string]bool) int {
	n := 0
	for _, m := range pageLinkPattern.FindAllStringSubmatch(body, -1) {
		if slugs[m[1]] {
			n++
		}
	}
	return n
}

// CountNotionPageLinks는 아직 노션 페이지 ID를 가리키는 /p/ 링크 수를 센다.
//
// 재작성 후 남는 건 posts에 행이 없는 대상뿐이다(인라인 데이터베이스이거나
// 덤프에 없는 페이지). 그래서 "/p/이면 노션 형태"가 아니라 "경로가 실제
// slug가 아니면 노션 형태"로 판별한다.
func CountNotionPageLinks(body string, slugs map[string]bool) int {
	n := 0
	for _, m := range pageLinkPattern.FindAllStringSubmatch(body, -1) {
		if !slugs[m[1]] {
			n++
		}
	}
	return n
}

// CountNotionInlineLinks는 본문에 남은 노션 인라인 href 수를 센다.
func CountNotionInlineLinks(body string) int {
	return len(notionInlineLinkPattern.FindAllString(body, -1)) +
		len(notionAbsoluteLinkPattern.FindAllString(body, -1))
}

// BareSlugLinks는 /p/ 접두사 없이 글 slug를 가리키는 링크의 대상을 모은다.
//
// 서버 라우트가 /p/{slug}뿐이라 이런 링크는 카테고리 경로로 잡혀 전부 404다.
// 재작성 후에는 0이어야 한다.
//
// 여기 세지 않는 것:
//   - 이미지 경로(/img/)와 /p/ 링크 — 경로가 slug와 다르므로 저절로 빠진다
//   - 아직 못 바꾼 노션 인라인 링크(/{32자리 16진수}) — 따로 세고 있다.
//     여기서 또 세면 의도적으로 남긴 것과 진짜 버그가 구분되지 않는다.
func BareSlugLinks(body string, slugs map[string]bool) []string {
	var out []string
	for _, m := range bareLinkPattern.FindAllStringSubmatch(body, -1) {
		if slugs[m[1]] {
			out = append(out, m[1])
		}
	}
	return out
}

// ValidateRewritten은 재작성된 본문에 접두사 없는 글 링크가 남아 있지 않은지 확인한다.
func ValidateRewritten(body string, slugs map[string]bool) error {
	if n := len(BareSlugLinks(body, slugs)); n > 0 {
		return fmt.Errorf("/p/ 없이 글을 가리키는 링크가 %d개 남아 있다", n)
	}
	return nil
}
