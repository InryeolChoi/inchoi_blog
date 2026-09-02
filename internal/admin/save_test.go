package admin

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// getDetail은 글 하나를 API로 가져온다. rev를 받아야 저장할 수 있어서,
// 거의 모든 저장 테스트가 이걸 먼저 부른다 — 화면이 하는 것과 같은 순서다.
func getDetail(t *testing.T, h http.Handler, slug string) PostDetail {
	t.Helper()
	rec := do(t, h, http.MethodGet, "/api/admin/posts/"+slug, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("%s 조회가 %d다: %s", slug, rec.Code, rec.Body.String())
	}
	var got PostDetail
	decode(t, rec, &got)
	return got
}

func save(t *testing.T, h http.Handler, method, path string, req saveReq) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, h, method, path, mustJSON(t, req))
}

// decode는 응답 JSON을 읽는다. 실패하면 그대로 멈춘다 — 반쯤 읽은 값으로
// 이어가면 뒤의 실패가 진짜 원인을 가린다.
func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("응답을 읽지 못했다: %v\n%s", err, rec.Body.String())
	}
}

// 새 글이 실제로 DB에 들어가는지 본다. **notion_page_id는 반드시 NULL이고
// source는 native여야 한다** — deploy/upload-guard.sql이 그 NULL 하나로
// "이관이 아니라 사람이 여기서 쓴 글"을 알아본다. 노션 id를 채우면 그 글은
// 가드에 안 걸리고 다음 upload-db.sh에 조용히 사라진다.
func TestCreateMarksThePostAsNative(t *testing.T) {
	sqlDB := testDB(t)
	s, err := New(sqlDB, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()

	rec := save(t, h, http.MethodPost, "/api/admin/posts", saveReq{
		Title: "웹에서 쓴 글", Body: "여기서 썼다.", Status: "draft",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("상태 코드 %d: %s", rec.Code, rec.Body.String())
	}
	var got PostDetail
	decode(t, rec, &got)
	if got.Slug != "웹에서-쓴-글" {
		t.Errorf("slug가 %q다. 제목에서 만들어져야 한다", got.Slug)
	}

	var source string
	var notionID sql.NullString
	if err := sqlDB.QueryRow(`SELECT source, notion_page_id FROM posts WHERE slug = ?`, got.Slug).
		Scan(&source, &notionID); err != nil {
		t.Fatal(err)
	}
	if source != "native" {
		t.Errorf("source가 %q다. native여야 한다", source)
	}
	if notionID.Valid {
		t.Errorf("notion_page_id가 %q다. NULL이어야 upload-guard가 알아본다", notionID.String)
	}
}

// 고치기가 실제로 본문을 바꾸는지, 그리고 **rev를 안 주면 거절하는지** 본다.
// rev 검사를 건너뛰면 두 탭에서 연 글이 조용히 서로를 지운다.
func TestUpdateRequiresTheRevItLoaded(t *testing.T) {
	h := testHandler(t)
	before := getDetail(t, h, "live-post")

	// rev 없이 → 409
	rec := save(t, h, http.MethodPut, "/api/admin/posts/live-post", saveReq{
		Slug: "live-post", Title: "덮어쓴 제목", Body: "덮어쓴 본문", Status: "draft",
	})
	if rec.Code != http.StatusConflict {
		t.Errorf("rev 없는 저장이 %d다. 409여야 한다", rec.Code)
	}
	if now := getDetail(t, h, "live-post"); now.Body != before.Body {
		t.Error("rev 없는 저장이 본문을 바꿨다")
	}

	// 엉뚱한 rev → 409
	rec = save(t, h, http.MethodPut, "/api/admin/posts/live-post", saveReq{
		Slug: "live-post", Title: "덮어쓴 제목", Body: "덮어쓴 본문", Status: "draft",
		Rev: "2000-01-01 00:00:00",
	})
	if rec.Code != http.StatusConflict {
		t.Errorf("낡은 rev 저장이 %d다. 409여야 한다", rec.Code)
	}

	// 맞는 rev → 저장되고 **새 rev가 온다**
	rec = save(t, h, http.MethodPut, "/api/admin/posts/live-post", saveReq{
		Slug: "live-post", Title: "고친 제목", Body: "고친 본문", Status: "published",
		Rev: before.Rev,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("상태 코드 %d: %s", rec.Code, rec.Body.String())
	}
	var got PostDetail
	decode(t, rec, &got)
	if got.Title != "고친 제목" || got.Body != "고친 본문" {
		t.Errorf("저장된 것이 %q / %q다", got.Title, got.Body)
	}
	if got.Rev == before.Rev {
		t.Error("rev가 그대로다. 화면이 이어서 또 저장할 수 없다")
	}
	// status가 처음 published가 될 때 published_at이 찍혀야 한다.
	if got.PublishedAt == nil {
		t.Error("published인데 published_at이 비었다")
	}
}

// published_at은 **처음 한 번만** 찍힌다. 다시 저장할 때마다 갱신되면
// "언제 공개했나"가 "마지막으로 저장한 때"가 된다.
func TestPublishedAtIsStampedOnce(t *testing.T) {
	h := testHandler(t)
	d := getDetail(t, h, "live-post")

	rec := save(t, h, http.MethodPut, "/api/admin/posts/live-post", saveReq{
		Slug: "live-post", Title: d.Title, Body: d.Body, Status: "published", Rev: d.Rev,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("%d: %s", rec.Code, rec.Body.String())
	}
	first := getDetail(t, h, "live-post")
	if first.PublishedAt == nil {
		t.Fatal("published_at이 안 찍혔다")
	}

	rec = save(t, h, http.MethodPut, "/api/admin/posts/live-post", saveReq{
		Slug: "live-post", Title: "또 고침", Body: d.Body, Status: "published", Rev: first.Rev,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("%d: %s", rec.Code, rec.Body.String())
	}
	second := getDetail(t, h, "live-post")
	if !second.PublishedAt.Equal(*first.PublishedAt) {
		t.Errorf("published_at이 바뀌었다: %v → %v", first.PublishedAt, second.PublishedAt)
	}
}

// **slug를 바꾸면 그 글을 가리키던 본문 링크도 같이 바꿔야 한다.**
// 안 그러면 다른 글 몇 편이 조용히 404를 가리킨다.
func TestRenamingASlugRewritesLinksToIt(t *testing.T) {
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
	rec := save(t, h, http.MethodPut, "/api/admin/posts/live-post", saveReq{
		Slug: "새-이름", Title: d.Title, Body: d.Body, Status: d.Status, Rev: d.Rev,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("%d: %s", rec.Code, rec.Body.String())
	}

	var body string
	if err := sqlDB.QueryRow(`SELECT body FROM posts WHERE slug = 'draft-post'`).Scan(&body); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "/p/live-post") {
		t.Errorf("옛 slug를 가리키는 링크가 남았다: %s", body)
	}
	if !strings.Contains(body, "](/p/새-이름)") {
		t.Errorf("새 slug로 안 바뀌었다: %s", body)
	}
}

// 사람이 고칠 수 있는 잘못과 지금 DB 상태와의 부딪힘을 갈라 준다.
// 전부 500이면 글 쓰는 사람은 자기가 뭘 잘못했는지 알 수 없다.
func TestSaveRejectsBadInput(t *testing.T) {
	h := testHandler(t)
	d := getDetail(t, h, "live-post")

	for _, tc := range []struct {
		name string
		req  saveReq
		want int
	}{
		{"제목 없음", saveReq{Title: "", Body: "x", Status: "draft"}, http.StatusBadRequest},
		{"모르는 status", saveReq{Title: "제목", Body: "x", Status: "hidden"}, http.StatusBadRequest},
		{"쓸 수 없는 slug", saveReq{Slug: "Bad Slug!", Title: "제목", Body: "x", Status: "draft"}, http.StatusBadRequest},
		{"없는 분류", saveReq{Title: "제목", Body: "x", Status: "draft", CategoryID: ptr(int64(999))}, http.StatusBadRequest},
		{"없는 부모", saveReq{Title: "제목", Body: "x", Status: "draft", ParentSlug: "없는-글"}, http.StatusBadRequest},
		{"못 읽는 날짜", saveReq{Title: "제목", Body: "x", Status: "draft", OriginalCreatedAt: "어제"}, http.StatusBadRequest},
		{"이미 쓰는 slug", saveReq{Slug: "live-post", Title: "제목", Body: "x", Status: "draft"}, http.StatusConflict},
	} {
		rec := save(t, h, http.MethodPost, "/api/admin/posts", tc.req)
		if rec.Code != tc.want {
			t.Errorf("%s: 상태 코드 %d, %d여야 한다 (%s)", tc.name, rec.Code, tc.want, rec.Body.String())
		}
	}

	// 자기 자신을 부모로 두는 것도 막는다.
	rec := save(t, h, http.MethodPut, "/api/admin/posts/live-post", saveReq{
		Slug: "live-post", Title: d.Title, Body: d.Body, Status: d.Status,
		Rev: d.Rev, ParentSlug: "live-post",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("자기 자신을 부모로 둔 저장이 %d다. 400이어야 한다", rec.Code)
	}
}

// posts.parent_id에는 categories와 달리 순환을 막는 트리거가 없다.
// 도는 사슬을 한 번 만들면 목록을 그리는 쪽이 전부 방어해야 한다.
func TestSaveRefusesAParentCycle(t *testing.T) {
	h := testHandler(t)

	// draft-post의 부모를 live-post로 둔다.
	d := getDetail(t, h, "draft-post")
	rec := save(t, h, http.MethodPut, "/api/admin/posts/draft-post", saveReq{
		Slug: "draft-post", Title: d.Title, Body: d.Body, Status: d.Status,
		Rev: d.Rev, ParentSlug: "live-post",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("부모 붙이기가 %d다: %s", rec.Code, rec.Body.String())
	}

	// 이제 live-post의 부모를 draft-post로 두려 하면 사슬이 돈다.
	l := getDetail(t, h, "live-post")
	rec = save(t, h, http.MethodPut, "/api/admin/posts/live-post", saveReq{
		Slug: "live-post", Title: l.Title, Body: l.Body, Status: l.Status,
		Rev: l.Rev, ParentSlug: "draft-post",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("도는 사슬을 만드는 저장이 %d다. 400이어야 한다: %s", rec.Code, rec.Body.String())
	}
}

// 저장이 실패하면 **아무것도 안 들어가야 한다.** slug 바꾸기가 UNIQUE에
// 걸리는 자리를 골라, 본문까지 통째로 되돌아가는지 본다.
func TestFailedSaveLeavesNothingBehind(t *testing.T) {
	sqlDB := testDB(t)
	s, err := New(sqlDB, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()

	before := getDetail(t, h, "live-post")
	rec := save(t, h, http.MethodPut, "/api/admin/posts/live-post", saveReq{
		// draft-post가 이미 쓰는 slug다.
		Slug: "draft-post", Title: "바뀐 제목", Body: "바뀐 본문", Status: "published",
		Rev: before.Rev,
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("상태 코드 %d, 409여야 한다: %s", rec.Code, rec.Body.String())
	}
	after := getDetail(t, h, "live-post")
	if after.Title != before.Title || after.Body != before.Body ||
		after.Status != before.Status || after.Rev != before.Rev {
		t.Errorf("실패한 저장이 흔적을 남겼다:\n전: %+v\n후: %+v", before, after)
	}
}

// 노션에서 온 글은 admin에서 고쳐도 다음 재이관이 되돌린다. 화면이 그걸
// 말해주려면 서버가 출처를 알려줘야 한다.
func TestDetailTellsWhereThePostCameFrom(t *testing.T) {
	h := testHandler(t)
	if d := getDetail(t, h, "live-post"); d.Source != "notion" {
		t.Errorf("source가 %q다", d.Source)
	}

	rec := save(t, h, http.MethodPost, "/api/admin/posts", saveReq{
		Title: "새로 쓴 것", Body: "본문", Status: "draft",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("%d: %s", rec.Code, rec.Body.String())
	}
	var got PostDetail
	decode(t, rec, &got)
	if got.Source != "native" || got.NotionPageID != nil {
		t.Errorf("새 글이 source=%q notion=%v다", got.Source, got.NotionPageID)
	}
	if got.Managed {
		t.Error("새 글이 curation 관리 대상으로 나온다")
	}
}

// slug 규칙은 카테고리와 **같은 함수**가 정한다. 여기 규칙을 다시 적으면
// 언젠가 두 벌이 된다.
func TestValidSlugMatchesTheCategoryRule(t *testing.T) {
	for _, tc := range []struct {
		slug string
		want bool
	}{
		{"live-post", true},
		{"선형대수", true}, // 카테고리 slug가 이미 한글을 쓴다
		{"part-4-메모리", true},
		{"8327f5f2-11ac-4591", true}, // 노션에서 온 UUID 꼴
		{"", false},
		{"Bad Slug", false}, // 대문자와 공백
		{"has_underscore", false},
		{"-앞에하이픈", false},
		{"뒤에하이픈-", false},
		{"두개--하이픈", false},
		{strings.Repeat("가", slugMaxLen+1), false},
	} {
		if got := validSlug(tc.slug); got != tc.want {
			t.Errorf("validSlug(%q) = %v, want %v", tc.slug, got, tc.want)
		}
	}
}

func ptr[T any](v T) *T { return &v }

// 저장이 **도중에** 실패하면 앞서 쓴 것까지 통째로 되돌아가야 한다.
//
// 앞의 TestFailedSaveLeavesNothingBehind는 첫 UPDATE에서 걸리는 경우라
// 롤백이 있으나 없으나 결과가 같다 — 그건 트랜잭션을 확인하지 못한다.
// 여기서는 **UPDATE가 성공한 뒤** 링크 재작성이 터지게 만들어 둘을 가른다.
// 트리거로 터뜨리는 이유는 그것이 이 경로에서 실제로 실패할 수 있는 유일한
// 자리이기 때문이다(본문 재작성은 다른 행을 건드린다).
func TestSaveRollsBackWhatItAlreadyWrote(t *testing.T) {
	sqlDB := testDB(t)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := sqlDB.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`UPDATE posts SET body = '[보이는 글](/p/live-post)' WHERE slug = 'draft-post'`)
	// 링크 재작성(= draft-post의 body를 고치는 UPDATE)만 골라 터뜨린다.
	exec(`CREATE TRIGGER boom BEFORE UPDATE OF body ON posts
	      WHEN old.slug = 'draft-post'
	      BEGIN SELECT RAISE(ABORT, 'boom'); END`)

	s, err := New(sqlDB, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()

	before := getDetail(t, h, "live-post")
	rec := save(t, h, http.MethodPut, "/api/admin/posts/live-post", saveReq{
		Slug: "새-이름", Title: "바뀐 제목", Body: "바뀐 본문", Status: before.Status,
		Rev: before.Rev,
	})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("상태 코드 %d, 500이어야 한다: %s", rec.Code, rec.Body.String())
	}

	// **글이 옛 slug 그대로 있어야 한다.** 롤백이 없으면 이름은 바뀌고
	// 링크만 옛것으로 남아, 다른 글이 404를 가리키게 된다.
	var slug, title, body string
	if err := sqlDB.QueryRow(`SELECT slug, title, body FROM posts WHERE id = ?`, before.ID).
		Scan(&slug, &title, &body); err != nil {
		t.Fatal(err)
	}
	if slug != before.Slug || title != before.Title || body != before.Body {
		t.Errorf("실패한 저장이 남았다: slug=%q title=%q", slug, title)
	}
}

// **공개 범위는 status와 다른 축이라 따로 왕복해야 한다.** 여기가 조용히
// 무시되면 화면에서 private을 골라도 글이 그대로 공개로 남는다.
func TestVisibilityRoundTrips(t *testing.T) {
	sqlDB := testDB(t)
	s, err := New(sqlDB, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()

	rec := save(t, h, http.MethodPost, "/api/admin/posts", saveReq{
		Title: "숨긴 글", Body: "여기만 본다.", Status: "draft", Visibility: "private",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("상태 코드 %d: %s", rec.Code, rec.Body.String())
	}
	var got PostDetail
	decode(t, rec, &got)
	if got.Visibility != "private" {
		t.Errorf("응답의 visibility가 %q다", got.Visibility)
	}
	var vis string
	if err := sqlDB.QueryRow(`SELECT visibility FROM posts WHERE slug = ?`, got.Slug).Scan(&vis); err != nil {
		t.Fatal(err)
	}
	if vis != "private" {
		t.Errorf("DB의 visibility가 %q다. private이어야 한다", vis)
	}

	// 고칠 때 되돌려 보내면 그대로 남아야 한다. PUT은 통째로 바꾸기라
	// 이 칸을 빠뜨리면 글이 조용히 공개가 된다.
	rec = save(t, h, http.MethodPut, "/api/admin/posts/"+got.Slug, saveReq{
		Slug: got.Slug, Title: got.Title, Body: "고쳤다.", Status: "draft",
		Visibility: "private", Rev: got.Rev,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("고치기 상태 코드 %d: %s", rec.Code, rec.Body.String())
	}
	decode(t, rec, &got)
	if got.Visibility != "private" {
		t.Errorf("고친 뒤 visibility가 %q다", got.Visibility)
	}
}

// **빈 값은 public이고 모르는 값은 거절이다.** 오타 하나가 통과하면 그 글은
// 비공개가 아니라 아무 조건에도 안 걸리는 공개 글이 된다.
func TestVisibilityDefaultsToPublicAndRejectsJunk(t *testing.T) {
	sqlDB := testDB(t)
	s, err := New(sqlDB, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()

	rec := save(t, h, http.MethodPost, "/api/admin/posts", saveReq{
		Title: "그냥 글", Body: "본문", Status: "draft",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("상태 코드 %d: %s", rec.Code, rec.Body.String())
	}
	var got PostDetail
	decode(t, rec, &got)
	if got.Visibility != "public" {
		t.Errorf("안 보냈는데 visibility가 %q다. public이어야 한다", got.Visibility)
	}

	rec = save(t, h, http.MethodPost, "/api/admin/posts", saveReq{
		Title: "오타 글", Body: "본문", Status: "draft", Visibility: "prvate",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("오타를 %d로 받았다. 400이어야 한다: %s", rec.Code, rec.Body.String())
	}
}
