package web

import "sort"

// 최근에 쓴 글이 맨 앞에 서는 분류다.
//
// # 왜 예외가 필요한가
//
// 이 아카이브의 기본 순서는 **읽는 차례**다(`sortPosts`) — 제목 앞 번호를
// 따르고 그 뒤는 제목 자연 정렬이라, 1강부터 11강까지가 순서대로 선다. 배운
// 것을 정리한 글에는 그게 맞다.
//
// 그런데 일지처럼 쌓이는 글은 **읽는 차례가 곧 쓴 차례의 역순**이다. 새로 쓴
// 것이 맨 아래로 밀리면 매번 끝까지 내려가야 새 글을 본다. `라이프`가 그 자리라
// 사람이 정했다(2026-09-08).
//
// # 왜 코드에 두나
//
// 갈래 카드·바깥 링크·`deferredSections`와 같은 이유다 — **화면 장치라 웹
// 코드에 둔다.** 표지 본문(DB)에 적으면 다음 `import -db`가 덮어써서 사라지고,
// `internal/curation`에 적으면 웹이 그 패키지를 읽게 되어 "DB가 정본"이 깨진다.
//
// # 왜 sort_order를 안 쓰나
//
// 사람이 정한 순서(`sort_order_manual`)로도 같은 배열을 만들 수 있지만, 그건
// **한 번 정한 차례**라 글을 새로 쓸 때마다 다시 정해줘야 한다. "항상 최근
// 글이 앞"은 그런 뜻이 아니다 — 규칙이라 코드가 든다.
var recentFirstCategories = map[string]bool{
	"life": true,
}

func recentFirstCategory(slug string) bool { return recentFirstCategories[slug] }

// sortNewestFirst는 목록을 원본 작성일 내림차순으로 세운다. 중첩된 하위 글도
// 같은 규칙으로 다시 세운다 — 한 목록 안에서 층마다 순서가 다르면 읽는 사람이
// 그 차이를 뜻으로 읽는다.
//
// **날짜가 없는 글은 뒤로 보낸다.** 어느 날짜 옆에 놓을지 알 방법이 없다 —
// 번호 없는 글을 번호 있는 글 사이에 끼우지 않는 것과 같은 판단이다. 그
// 뒤쪽끼리는 `sortPosts`가 이미 세워둔 차례를 그대로 둔다(안정 정렬).
func sortNewestFirst(in []PostSummary) {
	sort.SliceStable(in, func(i, j int) bool {
		a, b := in[i].CreatedAt, in[j].CreatedAt
		if a.Valid != b.Valid {
			return a.Valid
		}
		if !a.Valid {
			return false
		}
		return a.Time.After(b.Time)
	})
	for i := range in {
		sortNewestFirst(in[i].Children)
	}
}
