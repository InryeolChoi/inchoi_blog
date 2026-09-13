package admin

import (
	"net/http"
	"strconv"
	"testing"
)

// 프로젝트 카드 화면(internal/web/deck.go)이 "그림 없는 하위 분류가 하나라도
// 있으면 통째로 포기한다"는 규칙과 부딪히지 않는지는 여기서 보지 않는다 —
// 그건 카드 그림(cardArtBySlug)을 사람이 채우는 별개의 단계다. 여기서는
// 분류가 실제로 만들어지고, 곧바로 그 글에 붙일 수 있는지만 본다.
func TestCreateCategoryMakesOneUsableRightAway(t *testing.T) {
	h := testHandler(t)

	rec := do(t, h, http.MethodPost, "/api/admin/categories", `{"name":"위잉위잉"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("상태 코드 %d: %s", rec.Code, rec.Body.String())
	}
	var cat CategoryRow
	decode(t, rec, &cat)
	if cat.Slug != "위잉위잉" {
		t.Errorf("slug가 %q다", cat.Slug)
	}
	if cat.Depth != 0 || cat.Path != "위잉위잉" {
		t.Errorf("최상위 분류인데 depth=%d path=%q", cat.Depth, cat.Path)
	}

	// 목록에도 곧바로 나타난다 — 편집기가 다시 불러도 방금 만든 게 보인다.
	list := do(t, h, http.MethodGet, "/api/admin/categories", "")
	var got struct {
		Categories []CategoryRow `json:"categories"`
	}
	decode(t, list, &got)
	found := false
	for _, c := range got.Categories {
		if c.ID == cat.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("방금 만든 분류가 목록에 없다")
	}

	// 그 분류로 글을 바로 쓸 수 있다.
	saveRec := save(t, h, http.MethodPost, "/api/admin/posts", saveReq{
		Title: "테스트 글", Body: "본문", Status: "draft", CategoryID: &cat.ID,
	})
	if saveRec.Code != http.StatusCreated {
		t.Fatalf("상태 코드 %d: %s", saveRec.Code, saveRec.Body.String())
	}
}

// 자식 분류를 만들 때는 부모의 경로가 앞에 붙는다.
func TestCreateCategoryUnderAParentBuildsThePath(t *testing.T) {
	h := testHandler(t)

	parentRec := do(t, h, http.MethodPost, "/api/admin/categories", `{"name":"부모"}`)
	var parent CategoryRow
	decode(t, parentRec, &parent)

	childRec := do(t, h, http.MethodPost, "/api/admin/categories",
		`{"name":"자식","parentId":`+strconv.FormatInt(parent.ID, 10)+`}`)
	if childRec.Code != http.StatusCreated {
		t.Fatalf("상태 코드 %d: %s", childRec.Code, childRec.Body.String())
	}
	var child CategoryRow
	decode(t, childRec, &child)
	if child.Path != "부모 > 자식" {
		t.Errorf("path가 %q다", child.Path)
	}
	if child.Depth != 1 {
		t.Errorf("depth가 %d다", child.Depth)
	}
}

// 이름이 비었거나, 없는 부모를 가리키거나, 이미 쓰는 slug와 겹치면 사람이
// 고칠 수 있는 잘못으로 400을 준다 — 500으로 뭉개면 무엇이 잘못인지 안 보인다.
func TestCreateCategoryRejectsBadInput(t *testing.T) {
	h := testHandler(t)

	if rec := do(t, h, http.MethodPost, "/api/admin/categories", `{"name":"  "}`); rec.Code != http.StatusBadRequest {
		t.Errorf("빈 이름인데 %d다: %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodPost, "/api/admin/categories", `{"name":"고아","parentId":999999}`); rec.Code != http.StatusBadRequest {
		t.Errorf("없는 부모인데 %d다: %s", rec.Code, rec.Body.String())
	}

	do(t, h, http.MethodPost, "/api/admin/categories", `{"name":"겹침"}`)
	if rec := do(t, h, http.MethodPost, "/api/admin/categories", `{"name":"겹침"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("slug가 겹치는데 %d다: %s", rec.Code, rec.Body.String())
	}
}
