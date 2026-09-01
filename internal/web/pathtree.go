package web

import "strings"

// 카테고리 화면의 평평한 대량 목록을 **노션 경로에 남아 있는 층**으로 다시 묶는다.
//
// # 왜 필요한가
//
// 카테고리는 3단계가 끝이라 그보다 깊었던 노션 계층은 categorize를 거치며
// 평평해진다(CLAUDE.md "글 계층"). `cmd/postparent`가 `parent_id`로 그 층을
// 되살렸지만 지금까지 5개 카테고리 172건뿐이고, 나머지는 목록이 한 층으로
// 쏟아진다. `Language > 프로그래밍 언어`가 그 극단이었다 — C·C++·Java·Python·
// R이 뒤섞인 **111편이 한 목록**이라 무엇을 보러 왔든 찾을 수가 없었다.
//
// # 무엇을 근거로 묶나
//
// **`original_path`뿐이다.** 없는 구조를 지어내지 않는다는 `sort_order`의
// 원칙과 같다. 글의 경로에서 이 분류의 `source_name`을 찾고, 그 뒤에서 자기
// 제목 앞까지 남은 칸이 곧 categorize가 없앤 층이다:
//
//	Language > 프로그래밍 언어 > R > 확률 만들기 > 확률함수 & 분포함수 (1)
//	           └ source_name    └────── 잃어버린 층 ─────┘ └ 자기 제목
//
// 그 칸 이름과 **제목이 같은 글이 목록에 있으면 그 글이 그 마디**다(`C`,
// `R`, `3. HTTP 메시지`처럼). 없으면 posts에 행이 없는 노션 인라인
// 데이터베이스라, 링크가 아니라 **이름표**로 남긴다 — `InlineDBGroups`가
// 본문에서 하는 판정과 같은 것을 목록에서 한다.
//
// # 언제 포기하나
//
// 조용히 반만 묶는 것이 여기서 제일 나쁜 결과라, 하나라도 어긋나면 통째로
// 포기하고 평소 목록으로 돌아간다(갈래 카드가 그림 없는 갈래 하나에 통째로
// 포기하는 것과 같다). 아래 `pathTree`의 네 가지 거절 조건 참고.

// pathTreeMinPosts는 경로로 다시 묶기 시작하는 글 수다.
//
// **근거가 있어도 짧은 목록은 안 묶는다.** 일곱 줄짜리 목록에 이름표 세 개를
// 얹으면 알려주는 것보다 시선을 뺏는 것이 많다. 열두 편이면 한 화면에서
// 훑기 어려워지는 지점이라 거기서 자른다.
const pathTreeMinPosts = 12

// PostNode는 경로로 되살린 계층의 한 마디다.
//
// Post가 있으면 글이고(링크), 없으면 posts에 행이 없는 마디다(이름표만).
// 둘을 한 타입으로 두는 이유: 화면에서는 같은 목록의 형제이고, 어느 쪽인지는
// **데이터가 정하지 화면이 고르는 것이 아니다.**
type PostNode struct {
	Post     *PostSummary
	Label    string
	Children []PostNode
}

// Name은 정렬과 화면에 쓸 이 마디의 이름이다.
func (n PostNode) Name() string {
	if n.Post != nil {
		return n.Post.Title
	}
	return n.Label
}

// pathTree는 카테고리의 글 목록을 경로로 다시 묶는다. 못 묶으면 nil이다.
//
// nil을 돌려주면 화면은 예전 그대로 `posttree`를 그린다. **error가 아니라
// nil이다** — 반쪽짜리 묶음보다 평소 목록이 낫다는 판단이지 고장이 아니다.
func pathTree(cat Category, posts []PostSummary) []PostNode {
	// ① 노션에서 온 분류가 아니면 닻이 없다. 사람이 만든 분류는 source_name이
	//    NULL이고, 그 밑의 글 경로에는 그 이름이 아예 안 나온다.
	if cat.SourceName == "" || len(posts) < pathTreeMinPosts {
		return nil
	}
	// ② parent_id가 이미 층을 만든 목록에는 손대지 않는다. 두 벌의 계층이
	//    서로 다른 답을 내면 어느 쪽이 맞는지 화면만 보고는 알 수 없다.
	//    `postparent`가 채운 5개 카테고리가 여기 걸린다.
	for _, p := range posts {
		if len(p.Children) > 0 {
			return nil
		}
	}

	type item struct {
		post *PostSummary
		rest []string
	}
	items := make([]item, 0, len(posts))
	for i := range posts {
		rest, ok := lostLevels(cat.SourceName, posts[i].OriginalPath.String)
		if !ok {
			// ③ 한 편이라도 닻을 못 찾으면 통째로 포기한다. 경로가 다른
			//    글은 curation이 다른 분류에서 데려온 것이라(선형대수 11편이
			//    그렇다) 그 경로로 묶으면 남의 층을 빌려오게 된다.
			return nil
		}
		items = append(items, item{post: &posts[i], rest: rest})
	}

	// 형제가 하나뿐인 마디는 층을 하나 늘릴 뿐 아무것도 가르지 않는다.
	// 모두가 같은 첫 칸을 갖고 있으면 그 칸을 벗겨내고 다음 칸을 본다.
	//
	// **빈 목록에서 도는 것을 여기서 막는다.** 항목이 하나도 없으면 아래
	// 반복문은 same을 뒤집을 기회가 없어서 벗길 것도 없이 영원히 돈다.
	// 위 ①의 최소 개수 검사가 지금은 그 경우를 막고 있지만, 40줄 떨어진
	// 검사에 반복문의 종료를 맡기지 않는다.
	for len(items) > 0 {
		head := ""
		same := true
		for _, it := range items {
			if len(it.rest) == 0 {
				same = false
				break
			}
			if head == "" {
				head = it.rest[0]
			} else if it.rest[0] != head {
				same = false
				break
			}
		}
		if !same {
			break
		}
		for i := range items {
			items[i].rest = items[i].rest[1:]
		}
	}

	root := &pathNode{}
	for _, it := range items {
		root.at(it.rest).posts = append(root.at(it.rest).posts, it.post)
	}
	out := root.build()

	// ④ 층이 목록을 절반 이하로 줄이지 못하면 안 쓴다. 45편짜리 백준 목록처럼
	//    노션에서도 원래 평평했던 곳이 여기서 걸러진다 — 묶을 것이 없으면
	//    이름표만 몇 개 늘고 목록은 그대로다.
	if len(out)*2 > len(posts) {
		return nil
	}
	return out
}

// lostLevels는 글의 경로에서 분류 뒤에 남은 칸을 준다. 자기 제목은 뺀다.
//
// **마지막에 나온 이름을 닻으로 삼는다.** 같은 이름이 경로에 두 번 나오는
// 자리가 실제로 있다 — `네트워크 > HTTP 완벽 가이드 > HTTP 완벽 가이드 > …`는
// 분류가 된 페이지 안에 같은 이름의 인라인 데이터베이스가 또 있는 것이라,
// 앞엣것을 잡으면 아무것도 가르지 못하는 층이 하나 낀다.
func lostLevels(sourceName, path string) ([]string, bool) {
	if path == "" {
		return nil, false
	}
	comps := strings.Split(path, " > ")
	anchor := -1
	// 마지막 칸은 글 제목 자신이라 닻이 될 수 없다(1408건 전부 그렇다).
	for i := 0; i < len(comps)-1; i++ {
		if comps[i] == sourceName {
			anchor = i
		}
	}
	if anchor < 0 {
		return nil, false
	}
	return comps[anchor+1 : len(comps)-1], true
}

// pathNode는 묶는 동안만 쓰는 자리다. kids의 순서를 따로 들고 있는 이유는
// 맵의 순회 순서가 실행마다 달라서, 그대로 두면 같은 DB가 매번 다른 화면을
// 낸다 — 정렬로 덮이더라도 이름이 같은 마디의 앞뒤가 흔들린다.
type pathNode struct {
	posts []*PostSummary
	kids  map[string]*pathNode
	order []string
}

// at은 rest 경로의 마디를 찾거나 만든다.
func (n *pathNode) at(rest []string) *pathNode {
	cur := n
	for _, name := range rest {
		if cur.kids == nil {
			cur.kids = map[string]*pathNode{}
		}
		next, ok := cur.kids[name]
		if !ok {
			next = &pathNode{}
			cur.kids[name] = next
			cur.order = append(cur.order, name)
		}
		cur = next
	}
	return cur
}

// build는 한 마디의 자식들을 화면용 목록으로 만든다.
//
// 자식 마디 이름과 **제목이 같은 글**이 이 마디에 놓여 있으면 그 글이 곧
// 그 마디다. 안 그러면 posts에 행이 없는 노션 데이터베이스라 이름표로 남는다.
func (n *pathNode) build() []PostNode {
	used := make([]bool, len(n.posts))
	out := make([]PostNode, 0, len(n.order)+len(n.posts))
	for _, name := range n.order {
		node := PostNode{Label: name, Children: n.kids[name].build()}
		for i, p := range n.posts {
			if !used[i] && p.Title == name {
				used[i] = true
				node.Post = p
				node.Label = ""
				break
			}
		}
		out = append(out, node)
	}
	for i, p := range n.posts {
		if !used[i] {
			out = append(out, PostNode{Post: p})
		}
	}
	sortNodes(out)
	return out
}

// sortNodes는 형제를 목록과 같은 규칙으로 세운다 — 사람이 정한 순서가 맨 앞,
// 그다음 제목 앞 번호, 그 뒤는 자연 정렬이다(sortPosts와 같다).
//
// **이름표도 같은 자리에서 함께 센다.** 원래 형제였던 것들이라, 글만 따로
// 세우면 `3. HTTP 메시지`와 `4. 커넥션 관리` 사이에 번호 없는 이름표가 낀다.
func sortNodes(in []PostNode) {
	rows := make([]PostSummary, len(in))
	for i, n := range in {
		rows[i] = PostSummary{ID: int64(i), Title: n.Name()}
		if n.Post != nil {
			rows[i].SortOrder = n.Post.SortOrder
			rows[i].ManualOrder = n.Post.ManualOrder
		}
	}
	sortPosts(rows)
	out := make([]PostNode, len(in))
	for i, r := range rows {
		out[i] = in[r.ID]
	}
	copy(in, out)
}

// countNodes는 이 목록이 실제로 몇 편을 담고 있는지 센다. 이름표는 글이
// 아니므로 세지 않는다.
func countNodes(nodes []PostNode) int {
	n := 0
	for _, node := range nodes {
		if node.Post != nil {
			n++
		}
		n += countNodes(node.Children)
	}
	return n
}
