package web

import (
	"database/sql"
	"testing"
	"time"
)

// 편집기의 `형제 순서` 패널이 볼 목록이 **화면과 같은지** 본다.
//
// 이 함수가 web에 있는 이유가 그것이다. admin이 제 눈으로 "부모가 같은 글"을
// 모으면 화면과 다른 트리가 나오고, 그러면 옮긴 대로 안 선다.

func orderOf(t *testing.T, sqlDB *sql.DB, slug string) SiblingList {
	t.Helper()
	got, err := SiblingOrder(sqlDB, slug)
	if err != nil {
		t.Fatalf("SiblingOrder(%s): %v", slug, err)
	}
	return got
}

func siblingTitles(list SiblingList) []string {
	out := make([]string, 0, len(list.Items))
	for _, it := range list.Items {
		out = append(out, it.Title)
	}
	return out
}

func same(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// seedFlat은 표지도 갈래 카드도 없는 평범한 분류 하나를 만든다.
func seedFlat(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB := testDB(t)
	exec := execer(t, sqlDB)
	now := time.Now().UTC()

	exec(`INSERT INTO categories (id, name, slug, sort_order) VALUES (1, '개발', 'dev', 0)`)
	post := func(id int, slug, title string, order int, manual int) {
		exec(`INSERT INTO posts (id, slug, title, body, status, source, category_id,
		                         sort_order, sort_order_manual, created_at, updated_at)
		      VALUES (?, ?, ?, '본문', 'unlisted', 'notion', 1, ?, ?, ?, ?)`,
			id, slug, title, order, manual, now, now)
	}
	// 제목 앞 번호도 없고 사람이 정한 순서도 없으니 화면은 자연 정렬이다.
	post(1, "b-post", "나 글", 5, 0)
	post(2, "a-post", "가 글", 0, 0)
	post(3, "c-post", "다 글", 3, 0)
	return sqlDB
}

// 목록은 **화면 순서 그대로**여야 한다. sort_order 순도, id 순도 아니다.
func TestSiblingOrderMatchesTheScreen(t *testing.T) {
	sqlDB := seedFlat(t)
	got := orderOf(t, sqlDB, "c-post")
	if got.Reason != "" {
		t.Fatalf("정할 수 있어야 하는데 거절했다: %s", got.Reason)
	}
	want := []string{"가 글", "나 글", "다 글"}
	if !same(siblingTitles(got), want) {
		t.Errorf("목록이 %v다. 화면과 같은 %v여야 한다", siblingTitles(got), want)
	}
	current := ""
	for _, it := range got.Items {
		if it.Current {
			current = it.Slug
		}
	}
	if current != "c-post" {
		t.Errorf("지금 글 표시가 %q다. 목록에서 어디인지 못 보여준다", current)
	}
}

// **draft는 목록에 안 선다.** 그러면 정할 순서도 없다 — 로그인해서 고치는
// 중이어도 마찬가지다. draft를 숨기는 것은 로그인 여부가 아니라 status가
// 정하는 일이라, 여기서 예외를 두면 화면과 다른 목록이 나온다.
func TestSiblingOrderRefusesHiddenPosts(t *testing.T) {
	sqlDB := seedFlat(t)
	exec := execer(t, sqlDB)
	exec(`UPDATE posts SET status = 'draft' WHERE slug = 'c-post'`)
	if got := orderOf(t, sqlDB, "c-post"); got.Reason == "" {
		t.Errorf("draft인데 순서를 정할 수 있다고 한다: %v", siblingTitles(got))
	}

	exec(`UPDATE posts SET status = 'unlisted', visibility = 'private' WHERE slug = 'c-post'`)
	if got := orderOf(t, sqlDB, "c-post"); got.Reason == "" {
		t.Errorf("비공개인데 순서를 정할 수 있다고 한다: %v", siblingTitles(got))
	}
}

// 표지 글은 본문이 목록 위에 통째로 펼쳐지므로 목록에 서지 않는다.
func TestSiblingOrderRefusesTheCover(t *testing.T) {
	sqlDB := seedFlat(t)
	exec := execer(t, sqlDB)
	exec(`UPDATE categories SET cover_post_id = 1 WHERE id = 1`)
	if got := orderOf(t, sqlDB, "b-post"); got.Reason == "" {
		t.Errorf("표지인데 순서를 정할 수 있다고 한다: %v", siblingTitles(got))
	}
}

// **표지가 안내하는 글도 순서를 정할 수 있다.** 그 글은 카테고리 목록에서
// 빠지지만, 표지 본문이 펼친 상자 안의 차례는 `sort_order`가 그대로 정한다
// (InlineDBGroups). 공개 946편 중 573편이 이 자리라, 여기서 물러나면 이
// 기능이 대부분의 글에 안 닿는다.
func TestSiblingOrderFollowsTheCoverList(t *testing.T) {
	sqlDB := testDB(t)
	exec := execer(t, sqlDB)
	now := time.Now().UTC()

	exec(`INSERT INTO categories (id, name, slug, sort_order) VALUES (1, '수학', 'math', 0)`)
	// 표지 글이 인라인 데이터베이스 하나를 링크한다. 그 이름(`분포`)이
	// 자식 글들의 original_path 가운데 칸이다.
	exec(`INSERT INTO posts (id, slug, title, body, status, source, category_id, sort_order,
	                         original_path, created_at, updated_at)
	      VALUES (1, 'cover', '수학', ?, 'unlisted', 'notion', 1, 0, '수학', ?, ?)`,
		"안내\n\n[분포](/p/없는-데이터베이스)\n", now, now)
	exec(`UPDATE categories SET cover_post_id = 1 WHERE id = 1`)

	row := func(id int, slug, title, path string, order, manual int) {
		exec(`INSERT INTO posts (id, slug, title, body, status, source, category_id, sort_order,
		                         sort_order_manual, original_path, created_at, updated_at)
		      VALUES (?, ?, ?, '본문', 'unlisted', 'notion', 1, ?, ?, ?, ?, ?)`,
			id, slug, title, order, manual, path, now, now)
	}
	// 사람이 정한 차례다(베타 → 감마). 안 정하면 상자도 제목 자연 정렬이라
	// 감마가 앞에 온다 — 그 값을 뒤집는 것이 이 패널이 하는 일이다.
	row(2, "beta", "베타분포", "수학 > 분포 > 베타분포", 0, 1)
	row(3, "gamma", "감마분포", "수학 > 분포 > 감마분포", 1, 1)
	// 표지가 **안 가리키는** 글을 같은 분류에 하나 둔다. 카테고리 목록으로
	// 답했다면 이 글이 섞여 나오므로, 두 갈래가 여기서 갈린다.
	row(4, "other", "다른 글", "수학 > 다른 글", 0, 0)

	got := orderOf(t, sqlDB, "gamma")
	if got.Reason != "" {
		t.Fatalf("표지가 펼친 상자인데 거절했다: %s", got.Reason)
	}
	want := []string{"베타분포", "감마분포"}
	if !same(siblingTitles(got), want) {
		t.Errorf("목록이 %v다. 상자에 보이는 %v여야 한다", siblingTitles(got), want)
	}

}
