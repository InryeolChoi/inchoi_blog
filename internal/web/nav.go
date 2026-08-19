package web

// 사이드바는 모든 페이지에 나온다. 카테고리 전체(93개)를 트리로 그리되,
// 최상위 8개만 펼쳐두고 그 아래(19 + 66)는 접어둔다 — 한 번에 다 펼치면
// 목록이 화면보다 길어져서 지금 어디 있는지가 오히려 안 보인다.
//
// 지금 보고 있는 곳으로 가는 길만 펼쳐둔다. 그래서 페이지를 열면 내 위치가
// 이미 열려 있고, 나머지는 눌러서 연다.

// markNav는 트리에 "펼침"과 "현재 위치" 표시를 넣는다.
//
// 표시를 Go에서 다는 이유: Go 템플릿은 재귀할 때 dot이 바뀌어서 바깥의 맵을
// 같이 넘기기가 번거롭다. 노드가 자기 상태를 들고 있으면 템플릿은 그리기만
// 하면 된다.
func markNav(tree []NavCategory, open map[int64]bool, active int64) {
	for i := range tree {
		c := &tree[i]
		c.Active = c.ID == active
		// 자식이 없으면 펼칠 것도 없다. 여는 표시가 남아 있으면 템플릿이
		// 빈 화살표를 그린다.
		c.Open = len(c.Children) > 0 && open[c.ID]
		markNav(c.Children, open, active)
	}
}

// openTrail은 카테고리 경로를 "펼쳐둘 목록"으로 바꾼다.
//
// 마지막 칸(지금 보고 있는 카테고리)도 넣는다. 그 아래 분류가 있으면 열어서
// 바로 보이게 하는 편이 낫다 — 한 번 더 누르게 할 이유가 없다.
func openTrail(trail []Category) (map[int64]bool, int64) {
	open := make(map[int64]bool, len(trail))
	var active int64
	for _, c := range trail {
		open[c.ID] = true
		active = c.ID
	}
	return open, active
}
