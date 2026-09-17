package admin

import (
	"net/http"
	"testing"
)

// 지도는 부모-자식과 본문 링크 둘 다를 선으로 낸다. 링크가 있는데도 안 잡히면
// 지도에서 그 관계가 통째로 사라진다.
func TestGraphIncludesParentAndBodyLinks(t *testing.T) {
	h := testHandler(t)

	rec := save(t, h, http.MethodPost, "/api/admin/posts", saveReq{
		Title: "부모 글", Body: "본문", Status: "draft",
	})
	var parent PostDetail
	decode(t, rec, &parent)

	childRec := save(t, h, http.MethodPost, "/api/admin/posts", saveReq{
		Title: "자식 글", Body: "본문", Status: "draft", ParentSlug: parent.Slug,
	})
	var child PostDetail
	decode(t, childRec, &child)

	linkRec := save(t, h, http.MethodPost, "/api/admin/posts", saveReq{
		Title: "링크 거는 글", Body: "[부모로](/p/" + parent.Slug + ")", Status: "draft",
	})
	var linker PostDetail
	decode(t, linkRec, &linker)

	got := do(t, h, http.MethodGet, "/api/admin/graph", "")
	if got.Code != http.StatusOK {
		t.Fatalf("상태 코드 %d: %s", got.Code, got.Body.String())
	}
	var g Graph
	decode(t, got, &g)

	nodeIDs := map[int64]bool{}
	for _, n := range g.Nodes {
		nodeIDs[n.ID] = true
	}
	for _, id := range []int64{parent.ID, child.ID, linker.ID} {
		if !nodeIDs[id] {
			t.Errorf("글 id=%d가 지도에 없다", id)
		}
	}

	var hasParentEdge, hasLinkEdge bool
	for _, e := range g.Edges {
		if e.Kind == "parent" && e.Source == parent.ID && e.Target == child.ID {
			hasParentEdge = true
		}
		if e.Kind == "link" && e.Source == linker.ID && e.Target == parent.ID {
			hasLinkEdge = true
		}
	}
	if !hasParentEdge {
		t.Error("부모-자식 선이 없다")
	}
	if !hasLinkEdge {
		t.Error("본문 링크 선이 없다")
	}
}

// 같은 글을 두 번 링크해도 선은 한 번만 나온다 — 안 그러면 자주 참조하는
// 글 사이에 겹친 선이 쌓여 지도가 진하게 뭉친다.
func TestGraphDedupesRepeatedLinks(t *testing.T) {
	h := testHandler(t)

	rec := save(t, h, http.MethodPost, "/api/admin/posts", saveReq{
		Title: "가리키는 대상", Body: "본문", Status: "draft",
	})
	var target PostDetail
	decode(t, rec, &target)

	save(t, h, http.MethodPost, "/api/admin/posts", saveReq{
		Title:  "두 번 거는 글",
		Body:   "[하나](/p/" + target.Slug + ") 그리고 [둘](/p/" + target.Slug + ")",
		Status: "draft",
	})

	got := do(t, h, http.MethodGet, "/api/admin/graph", "")
	var g Graph
	decode(t, got, &g)

	n := 0
	for _, e := range g.Edges {
		if e.Kind == "link" && e.Target == target.ID {
			n++
		}
	}
	if n != 1 {
		t.Errorf("같은 대상으로 가는 링크 선이 %d개다 (1개를 바랐다)", n)
	}
}
