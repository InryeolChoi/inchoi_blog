package web

import (
	"database/sql"
	"fmt"
)

// 편집기의 `형제 순서` 패널이 볼 목록을 만든다.
//
// # 왜 web에 있나
//
// **패널이 보여줄 목록은 카테고리 화면이 그리는 바로 그 목록이어야 한다.**
// 미리보기가 실제 화면과 같아야 하는 것과 같은 규칙이다 — 화면과 다른 트리를
// 끌어 놓게 하면 옮긴 대로 안 서고, 그러면 안 하느니만 못하다.
//
// 그래서 admin이 제 눈으로 다시 세지 않고 이 함수를 부른다. 순진하게 "부모가
// 같은 글"을 모으면 안 된다: 화면의 층은 `parent_id`(5개 분류 172건)에서만
// 오는 것이 아니라 `original_path`로 다시 묶은 것(pathtree.go)과 표지 본문이
// 펼친 것에서도 온다. `프로그래밍 언어`가 그 극단이다 — 화면엔 상자 열 개인데
// 부모로 모으면 191편이 한 줄로 쏟아진다.
//
// # 못 정하는 자리가 있다
//
// 그때는 error가 아니라 **Reason을 채워 돌려준다.** 화면이 못 그리는 것이지
// 고장이 아니다 — 갈래 카드가 그림 없는 갈래에서 목록으로 물러나는 것과 같다.
// 편집기는 그 말을 그대로 적고 숫자 칸으로 돌아간다.

// Sibling은 순서를 정할 목록의 한 줄이다.
type Sibling struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
	// Current는 지금 고치고 있는 글이다.
	Current bool `json:"current"`
}

// SiblingList는 한 글이 화면에서 서 있는 목록이다.
type SiblingList struct {
	// Reason이 비어 있을 때만 순서를 정할 수 있다. 비어 있지 않으면 왜 못
	// 정하는지를 사람이 읽을 말로 담는다.
	Reason string `json:"reason"`
	// Items는 **화면 순서 그대로**다.
	Items []Sibling `json:"items"`
}

// SiblingOrder는 slug의 글이 분류 화면에서 서 있는 목록을 화면 순서대로 준다.
//
// **draft를 가린 눈으로 본다.** 편집기가 로그인한 사람의 것이어도 마찬가지다 —
// draft를 숨기는 것은 로그인 여부가 아니라 그 글의 status가 정하는 일이고
// (CLAUDE.md "draft는 남에게 안 보인다"), 목록에 안 서는 글에는 정할 순서도 없다.
func SiblingOrder(db *sql.DB, slug string) (SiblingList, error) {
	st := &store{db: db}

	post, err := st.PostBySlug(slug)
	if err != nil {
		return SiblingList{}, err
	}
	if post == nil {
		return SiblingList{Reason: "이 글은 지금 공개 목록에 안 선다(draft이거나 비공개다). 목록에 서는 글끼리만 순서를 정할 수 있다."}, nil
	}
	if len(post.Trail) == 0 {
		return SiblingList{Reason: "분류가 없어서 설 목록이 없다."}, nil
	}

	cat, err := st.categoryByID(post.Trail[len(post.Trail)-1].ID)
	if err != nil {
		return SiblingList{}, err
	}
	if cat == nil {
		return SiblingList{Reason: "분류를 찾지 못했다."}, nil
	}
	if cat.CoverPostSlug == slug {
		return SiblingList{Reason: "이 글은 이 분류의 표지다. 본문이 목록 위에 통째로 펼쳐지므로 목록에 서지 않는다."}, nil
	}
	_, basePath := crumbs(post.Trail)

	posts, err := st.PostsInCategory(cat.ID)
	if err != nil {
		return SiblingList{}, err
	}
	// 표지 본문이 이미 안내한 글은 화면의 목록에서 빠진다(handleCategory와 같다).
	// **본문을 그릴 필요는 없다** — 어느 글을 가리키는지는 resolveBody가 이미 안다.
	var lists map[string][]PostSummary
	if cat.CoverPostSlug != "" && !listOnlyCategory(cat.Slug) {
		cover, err := st.PostBySlug(cat.CoverPostSlug)
		if err != nil {
			return SiblingList{}, err
		}
		if cover != nil {
			_, fix, err := st.resolveBody(cover.Body, cover.OriginalPath.String)
			if err != nil {
				return SiblingList{}, err
			}
			posts = dropShownPostTrees(posts, fix.Shown)
			lists = fix.Lists
		}
	}

	children, err := st.ChildCategories(cat.ID)
	if err != nil {
		return SiblingList{}, err
	}
	// 목록 자체를 안 그리는 화면이 둘 있다(category.html의 `not (or .Deck .Bundles)`).
	// basePath는 링크를 만드는 데만 쓰이므로 여기서 답을 바꾸지 않는다.
	deck, err := deckFor(st, *cat, basePath, children)
	if err != nil {
		return SiblingList{}, err
	}
	if len(deck) > 0 {
		return SiblingList{Reason: "이 분류는 갈래 카드만 보여준다. 화면에 글 목록이 없어서 정할 순서도 없다."}, nil
	}
	if len(bundlesFor(cat.Slug, children, posts, basePath)) > 0 {
		return SiblingList{Reason: "이 분류는 사람이 정한 묶음으로 그린다(internal/web/bundle.go). 순서도 거기서 정해진다."}, nil
	}

	var got SiblingList
	if tree := pathTree(*cat, posts); tree != nil {
		got = siblingsInTree(tree, slug)
	} else {
		got = siblingsInNest(posts, slug)
	}
	// **`notInList`일 때만 더 찾아본다.** 그 밖의 이유는 "여기 있는데 못
	// 정한다"라서, 그것을 덮으면 진짜 이유가 가려진다(이름표가 섞인 층).
	if got.Reason != notInList {
		return got, nil
	}

	// 목록에 없으면 **표지 본문이 펼친 상자**를 본다. 그 상자의 차례도
	// **목록과 같은 규칙**이 정한다 — InlineDBGroups가 마지막에 sortPosts를
	// 부르므로 사람이 정한 순서가 거기서도 그대로 이긴다. 수리통계1처럼 표지가
	// 전부 안내하는 분류가 실제로 그렇고, 그런 글이 공개 946편 중 573편이다.
	for _, rows := range lists {
		if found := siblingsInNest(rows, slug); found.Reason == "" {
			return found, nil
		}
	}
	return SiblingList{Reason: notInList}, nil
}

// categoryByID는 분류 한 줄을 표지·source_name까지 가져온다.
//
// CategoryTrail은 id·이름·slug만 채우는데, pathTree는 `source_name`을 닻으로
// 쓰고 표지 판정에는 `cover_post_id`가 필요하다.
func (s *store) categoryByID(id int64) (*Category, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.name, c.slug, c.parent_id, `+s.subtreePostCount()+`, `+s.coverSlug()+`, c.source_name
		FROM categories c
		WHERE c.id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("카테고리 조회(%d): %w", id, err)
	}
	cats, err := s.scanCategories(rows)
	if err != nil {
		return nil, err
	}
	if len(cats) == 0 {
		return nil, nil
	}
	return &cats[0], nil
}

// notInList는 목록 어디에도 그 글이 없을 때의 말이다. 표지 본문이 안내해서
// 빠진 경우가 대부분이라, 그때는 순서가 목록이 아니라 **본문의 차례**다.
const notInList = "이 글은 표지 글의 목차가 안내한다. 그때는 목록이 아니라 표지 본문의 차례를 따르므로, 순서는 본문을 고쳐서 바꾼다."

// siblingsInNest는 parent_id로 묶인 목록에서 그 글이 있는 층을 찾는다.
func siblingsInNest(posts []PostSummary, slug string) SiblingList {
	var found []Sibling
	var walk func([]PostSummary)
	walk = func(level []PostSummary) {
		if found != nil {
			return
		}
		for _, p := range level {
			if p.Slug == slug {
				out := make([]Sibling, 0, len(level))
				for _, q := range level {
					out = append(out, Sibling{Slug: q.Slug, Title: q.Title, Current: q.Slug == slug})
				}
				found = out
				return
			}
		}
		for _, p := range level {
			walk(p.Children)
		}
	}
	walk(posts)
	if found == nil {
		return SiblingList{Reason: notInList}
	}
	return SiblingList{Items: found}
}

// siblingsInTree는 경로로 되살린 층에서 그 글이 있는 층을 찾는다.
//
// **이름표가 섞인 층은 거절한다.** 이름표는 posts에 행이 없는 마디라
// (`Untitled` 같은 인라인 데이터베이스) 순서를 적어둘 자리가 없다. 그런데
// sortNodes가 사람이 정한 순서를 맨 앞에 놓으므로, 글에만 순서를 매기면
// **이름표가 통째로 아래로 밀린다** — 사람이 옮긴 적 없는 것이 움직인다.
func siblingsInTree(nodes []PostNode, slug string) SiblingList {
	var found *SiblingList
	var walk func([]PostNode)
	walk = func(level []PostNode) {
		if found != nil {
			return
		}
		here := false
		for _, n := range level {
			if n.Post != nil && n.Post.Slug == slug {
				here = true
				break
			}
		}
		if here {
			out := make([]Sibling, 0, len(level))
			for _, n := range level {
				if n.Post == nil {
					found = &SiblingList{Reason: "이 목록에는 글이 아닌 이름표(" + n.Label + ")가 섞여 있다. 이름표에는 순서를 적을 자리가 없어서, 글만 옮기면 이름표가 통째로 아래로 밀린다."}
					return
				}
				out = append(out, Sibling{Slug: n.Post.Slug, Title: n.Post.Title, Current: n.Post.Slug == slug})
			}
			found = &SiblingList{Items: out}
			return
		}
		for _, n := range level {
			walk(n.Children)
		}
	}
	walk(nodes)
	if found == nil {
		return SiblingList{Reason: notInList}
	}
	return *found
}
