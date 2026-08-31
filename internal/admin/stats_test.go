package admin

import (
	"net/http"
	"testing"
)

// 데이터 보기는 **읽기 전용이다.** 세어보는 화면이 DB를 건드리면 그게 사고다.
func TestStatsDoesNotTouchTheDatabase(t *testing.T) {
	sqlDB := testDB(t)
	s, err := New(sqlDB, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()

	before := ""
	if err := sqlDB.QueryRow(
		`SELECT group_concat(id || '|' || slug || '|' || status || '|' || updated_at)
		 FROM posts ORDER BY id`).Scan(&before); err != nil {
		t.Fatal(err)
	}

	if rec := do(t, h, http.MethodGet, "/api/admin/stats", ""); rec.Code != http.StatusOK {
		t.Fatalf("상태 코드 %d: %s", rec.Code, rec.Body.String())
	}

	after := ""
	if err := sqlDB.QueryRow(
		`SELECT group_concat(id || '|' || slug || '|' || status || '|' || updated_at)
		 FROM posts ORDER BY id`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Error("데이터 보기가 DB를 바꿨다")
	}
}

// 세는 값들이 서로 맞아떨어지는지 본다. **합이 안 맞는 통계는 없느니만 못하다** —
// 화면이 거짓말을 하면 그걸 보고 내린 판단이 전부 틀어진다.
func TestStatsNumbersAddUp(t *testing.T) {
	h := testHandler(t)
	rec := do(t, h, http.MethodGet, "/api/admin/stats", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("상태 코드 %d", rec.Code)
	}
	var st Stats
	decode(t, rec, &st)

	if got := st.Posts.Draft + st.Posts.Unlisted + st.Posts.Published; got != st.Posts.Total {
		t.Errorf("status 합이 %d, 전체는 %d다", got, st.Posts.Total)
	}
	// 픽스처는 live-post(unlisted)와 draft-post(draft) 둘이다.
	if st.Posts.Total != 2 || st.Posts.Draft != 1 || st.Posts.Unlisted != 1 {
		t.Errorf("글 수가 %+v다", st.Posts)
	}
	sum := 0
	for _, s := range st.Sources {
		sum += s.Count
	}
	if sum != st.Posts.Total {
		t.Errorf("출처 합이 %d, 전체는 %d다", sum, st.Posts.Total)
	}
	// 분류별 직속 글 수의 합도 전체와 같아야 한다(분류 없는 글이 없을 때).
	if st.Orphans.NoCategory == 0 {
		catSum := 0
		for _, c := range st.Cats {
			catSum += c.Posts
		}
		if catSum != st.Posts.Total {
			t.Errorf("분류별 합이 %d, 전체는 %d다", catSum, st.Posts.Total)
		}
	}
}

// **웹에서 쓴 글을 따로 센다.** 재이관이 되살리지 않는 글이라 그 수를 아는 것이
// upload-db.sh의 가드를 이해하는 데 필요하다.
func TestStatsCountsNativePosts(t *testing.T) {
	h := testHandler(t)

	var before Stats
	decode(t, do(t, h, http.MethodGet, "/api/admin/stats", ""), &before)
	if before.Orphans.Native != 0 {
		t.Fatalf("픽스처에 native 글이 %d편 있다", before.Orphans.Native)
	}

	if rec := save(t, h, http.MethodPost, "/api/admin/posts", saveReq{
		Title: "웹에서 쓴 글", Body: "x", Status: "draft",
	}); rec.Code != http.StatusCreated {
		t.Fatalf("저장이 %d다", rec.Code)
	}

	var after Stats
	decode(t, do(t, h, http.MethodGet, "/api/admin/stats", ""), &after)
	if after.Orphans.Native != 1 {
		t.Errorf("웹에서 쓴 글이 %d편으로 나온다. 1편이어야 한다", after.Orphans.Native)
	}
}

// 안 쓰이는 이미지를 짚어준다. 글을 지워도 BLOB은 남으므로(청소 도구가 아직
// 없다) 그 수가 화면에 드러나야 한다.
func TestStatsFindsUnusedImages(t *testing.T) {
	sqlDB := testDB(t)
	s, err := New(sqlDB, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()

	// 하나는 본문이 쓰고, 하나는 아무도 안 쓴다.
	used := "aaaa000000000000000000000000000000000000000000000000000000000001"
	unused := "bbbb000000000000000000000000000000000000000000000000000000000002"
	for _, sha := range []string{used, unused} {
		if _, err := sqlDB.Exec(
			`INSERT INTO images (sha256, data, mime, created_at) VALUES (?, ?, 'image/png', datetime('now'))`,
			sha, []byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := sqlDB.Exec(
		`UPDATE posts SET body = '![](/img/' || ? || ')' WHERE slug = 'live-post'`, used); err != nil {
		t.Fatal(err)
	}

	var st Stats
	decode(t, do(t, h, http.MethodGet, "/api/admin/stats", ""), &st)
	if st.Images.Count != 2 {
		t.Errorf("이미지가 %d장이다", st.Images.Count)
	}
	if st.Images.Unused != 1 {
		t.Errorf("안 쓰는 이미지가 %d장으로 나온다. 1장이어야 한다", st.Images.Unused)
	}
}

// 읽기라도 관문 밖은 아니다 — 이 화면은 아카이브의 속을 보여준다.
func TestStatsIsBehindTheGate(t *testing.T) {
	h, _ := authHandler(t, fakeGitHub(t, testLogin))
	if rec := get(t, h, "/api/admin/stats", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("로그인 없이 %d다. 401이어야 한다", rec.Code)
	}
}
