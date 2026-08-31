package admin

import (
	"net/http"
	"strings"
	"testing"
)

// 지우기는 **무엇을 잃는지 먼저 말한다.** 확인 창의 "예"를 무엇인지 모른 채
// 누르게 하지 않는다.
func TestDeleteTellsWhatWillBeLostFirst(t *testing.T) {
	sqlDB := testDB(t)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := sqlDB.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	// live-post를 draft-post가 가리키고, 분류의 표지이기도 하다.
	exec(`UPDATE posts SET body = '앞말 [보이는 글](/p/live-post) 뒷말' WHERE slug = 'draft-post'`)
	exec(`UPDATE categories SET cover_post_id = 1 WHERE id = 1`)

	s, err := New(sqlDB, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()

	rec := do(t, h, http.MethodGet, "/api/admin/posts/live-post/refs", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("refs가 %d다: %s", rec.Code, rec.Body.String())
	}
	var refs PostRefs
	decode(t, rec, &refs)
	if len(refs.LinkedFrom) != 1 || refs.LinkedFrom[0] != "숨긴 글" {
		t.Errorf("가리키는 글이 %v다", refs.LinkedFrom)
	}
	if len(refs.CoverOf) != 1 || refs.CoverOf[0] != "개발" {
		t.Errorf("표지인 분류가 %v다", refs.CoverOf)
	}
	if !refs.Notion {
		t.Error("노션에서 온 글인데 그렇다고 안 나온다")
	}

	// force 없이 지우면 409 + 무엇을 잃는지
	d := getDetail(t, h, "live-post")
	rec = do(t, h, http.MethodDelete, "/api/admin/posts/live-post",
		mustJSON(t, deleteReq{Rev: d.Rev}))
	if rec.Code != http.StatusConflict {
		t.Fatalf("확인 없는 삭제가 %d다. 409여야 한다", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "coverOf") {
		t.Errorf("409에 무엇을 잃는지가 안 실렸다: %s", rec.Body.String())
	}
	if getDetail(t, h, "live-post").Slug != "live-post" {
		t.Error("확인 없이 지워졌다")
	}
}

// **자식이 있으면 force로도 못 지운다.** 그건 읽고 말고의 문제가 아니라
// 되돌릴 수 없이 사슬이 끊기는 일이다.
func TestDeleteIsBlockedByChildrenEvenWithForce(t *testing.T) {
	h := testHandler(t)

	// draft-post의 부모를 live-post로 둔다.
	d := getDetail(t, h, "draft-post")
	if rec := save(t, h, http.MethodPut, "/api/admin/posts/draft-post", saveReq{
		Slug: "draft-post", Title: d.Title, Body: d.Body, Status: d.Status,
		Rev: d.Rev, ParentSlug: "live-post",
	}); rec.Code != http.StatusOK {
		t.Fatalf("부모 붙이기가 %d다", rec.Code)
	}

	l := getDetail(t, h, "live-post")
	rec := do(t, h, http.MethodDelete, "/api/admin/posts/live-post",
		mustJSON(t, deleteReq{Rev: l.Rev, Force: true}))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("자식이 있는 글의 삭제가 %d다. 400이어야 한다: %s", rec.Code, rec.Body.String())
	}
	if getDetail(t, h, "live-post").Slug != "live-post" {
		t.Error("자식이 있는데 지워졌다")
	}
}

// **지울 때 가리키던 링크를 글자로 푼다.**
//
// 안 풀면 그 링크의 대상이 "posts에 없는 slug"가 되는데, 렌더링 쪽 resolveBody는
// 그걸 **노션 인라인 데이터베이스로 판정한다.** 죽은 링크가 남는 데서 그치지
// 않고 엉뚱한 목록이 펼쳐질 수 있다.
func TestDeleteUnlinksInsteadOfLeavingDeadLinks(t *testing.T) {
	sqlDB := testDB(t)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := sqlDB.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`UPDATE posts SET body = '앞말 [보이는 글](/p/live-post) 뒷말' WHERE slug = 'draft-post'`)

	s, err := New(sqlDB, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()

	d := getDetail(t, h, "live-post")
	rec := do(t, h, http.MethodDelete, "/api/admin/posts/live-post",
		mustJSON(t, deleteReq{Rev: d.Rev, Force: true}))
	if rec.Code != http.StatusOK {
		t.Fatalf("삭제가 %d다: %s", rec.Code, rec.Body.String())
	}

	var body string
	if err := sqlDB.QueryRow(`SELECT body FROM posts WHERE slug = 'draft-post'`).Scan(&body); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "/p/live-post") {
		t.Errorf("죽은 링크가 남았다: %s", body)
	}
	// **글자는 남아야 한다.** 줄째 지우면 문장이 끊긴다.
	if body != "앞말 보이는 글 뒷말" {
		t.Errorf("링크를 푼 결과가 %q다. \"앞말 보이는 글 뒷말\"이어야 한다", body)
	}

	// 표지도 풀린다(ON DELETE SET NULL). 그리고 글은 정말 없다.
	if rec := do(t, h, http.MethodGet, "/api/admin/posts/live-post", ""); rec.Code != http.StatusNotFound {
		t.Errorf("지운 글 조회가 %d다. 404여야 한다", rec.Code)
	}
}

// 지우기도 저장과 같은 rev 표를 쓴다. 목록에서 본 그 글이 맞는지 확인하지
// 않으면, 다른 탭이 고친 글을 모르고 지운다.
func TestDeleteRequiresTheRevItLoaded(t *testing.T) {
	h := testHandler(t)

	for _, tc := range []struct {
		name string
		req  deleteReq
	}{
		{"rev 없음", deleteReq{Force: true}},
		{"낡은 rev", deleteReq{Rev: "2000-01-01 00:00:00", Force: true}},
	} {
		rec := do(t, h, http.MethodDelete, "/api/admin/posts/draft-post", mustJSON(t, tc.req))
		if rec.Code != http.StatusConflict {
			t.Errorf("%s: 상태 코드 %d, 409여야 한다", tc.name, rec.Code)
		}
		if getDetail(t, h, "draft-post").Slug != "draft-post" {
			t.Fatalf("%s: 지워졌다", tc.name)
		}
	}
}

// 잃을 것이 없는 글은 확인 없이 지워진다. 웹에서 잘못 만든 빈 글을 치우는
// 것이 이 기능의 본래 쓰임이라, 거기에 확인 창을 두 번 두지 않는다.
func TestDeletingAFreshPostNeedsNoForce(t *testing.T) {
	h := testHandler(t)

	rec := save(t, h, http.MethodPost, "/api/admin/posts", saveReq{
		Title: "잘못 만든 글", Body: "", Status: "draft",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("%d: %s", rec.Code, rec.Body.String())
	}
	var made PostDetail
	decode(t, rec, &made)

	rec = do(t, h, http.MethodDelete, "/api/admin/posts/"+made.Slug,
		mustJSON(t, deleteReq{Rev: made.Rev}))
	if rec.Code != http.StatusOK {
		t.Fatalf("갓 만든 글의 삭제가 %d다: %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodGet, "/api/admin/posts/"+made.Slug, ""); rec.Code != http.StatusNotFound {
		t.Errorf("지운 글이 %d로 남아 있다", rec.Code)
	}
}

// 지우기는 쓰기다. 남의 사이트에서 온 DELETE가 통하면 안 된다.
func TestDeleteFromAnotherSiteIsRefused(t *testing.T) {
	h := testHandler(t)
	d := getDetail(t, h, "draft-post")
	rec := write(t, h, http.MethodDelete, "/api/admin/posts/draft-post",
		"http://evil.example", mustJSON(t, deleteReq{Rev: d.Rev, Force: true}))
	if rec.Code != http.StatusForbidden {
		t.Errorf("남의 출처에서 온 삭제가 %d다. 403이어야 한다", rec.Code)
	}
	if getDetail(t, h, "draft-post").Slug != "draft-post" {
		t.Error("막았다면서 지워졌다")
	}
}
