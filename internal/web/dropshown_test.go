package web

import (
	"database/sql"
	"testing"
)

func sum(slug, path string, kids ...PostSummary) PostSummary {
	return PostSummary{
		Slug: slug, Title: slug,
		OriginalPath: sql.NullString{String: path, Valid: true},
		Children:     kids,
	}
}

func slugsOf(list []PostSummary) []string {
	var out []string
	var walk func([]PostSummary)
	walk = func(l []PostSummary) {
		for _, p := range l {
			out = append(out, p.Slug)
			walk(p.Children)
		}
	}
	walk(list)
	return out
}

// TestDropShownDropsPathDescendants는 표지가 안내한 글의 **자손**까지 목록에서
// 빠지는지 본다.
//
// `parent_id`는 5개 카테고리에만 채워져 있어서, 대부분의 글은 부모가 목차에
// 있어도 자손이 뿌리로 남는다. 그러면 `pathTree`가 그것들을 경로로 다시 묶으며
// 이미 뺀 부모를 이름표로 되살린다 — 표지의 목록과 아래 `글`이 같은 사슬을
// 두 벌로 보여준다. `HTTP 완벽 가이드`가 실제로 그랬다.
func TestDropShownDropsPathDescendants(t *testing.T) {
	base := "네트워크 > HTTP 완벽 가이드 > HTTP 완벽 가이드"
	posts := []PostSummary{
		sum("msg", base+" > 3. HTTP 메시지"),
		sum("get", base+" > 3. HTTP 메시지 > 메서드 > GET"),
		sum("del", base+" > 3. HTTP 메시지 > 메서드 > DELETE"),
		sum("other", base+" > 9. 다른 글"),
	}
	got := slugsOf(dropShownPostTrees(posts, map[string]bool{"msg": true}))

	if len(got) != 1 || got[0] != "other" {
		t.Errorf("자손이 남았다: %v (want [other])", got)
	}
}

// TestDropShownKeepsSiblingsWithSharedPrefix는 **이름이 앞에 겹치는 형제**를
// 자손으로 잘못 보지 않는지 본다. 경로 비교에 구분자 ` > `를 붙이는 이유다 —
// 안 붙이면 `3. HTTP 메시지`가 `3. HTTP 메시지 확장`도 삼킨다.
func TestDropShownKeepsSiblingsWithSharedPrefix(t *testing.T) {
	posts := []PostSummary{
		sum("a", "루트 > 가"),
		sum("ab", "루트 > 가나"),
	}
	got := slugsOf(dropShownPostTrees(posts, map[string]bool{"a": true}))

	if len(got) != 1 || got[0] != "ab" {
		t.Errorf("이름이 겹치는 형제를 자손으로 봤다: %v (want [ab])", got)
	}
}

// TestDropShownLeavesUnguidedPosts는 표지가 안내하지 않은 갈래는 그대로
// 남기는지 본다. 빼면 그 글로 가는 길이 사라진다.
func TestDropShownLeavesUnguidedPosts(t *testing.T) {
	posts := []PostSummary{sum("x", "루트 > 엑스"), sum("y", "루트 > 와이")}
	got := slugsOf(dropShownPostTrees(posts, map[string]bool{}))

	if len(got) != 2 {
		t.Errorf("안내하지 않은 글을 뺐다: %v", got)
	}
}
