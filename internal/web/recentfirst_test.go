package web

import (
	"regexp"
	"strings"
	"testing"
)

// 일지처럼 쌓이는 분류는 **최근에 쓴 글이 맨 앞**이다(2026-09-08, `라이프`).
//
// 기본 순서는 읽는 차례라(제목 앞 번호 → 자연 정렬) 새 글이 아래로 밀린다.
// 그런 자리에서는 매번 끝까지 내려가야 새 글을 본다.
func TestLifeListsNewestFirst(t *testing.T) {
	sqlDB := testDB(t)
	exec := execer(t, sqlDB)

	// 두 분류에 **같은 글들**을 넣는다. 제목 자연 정렬과 날짜순이 정반대가
	// 되도록 지어서, 규칙이 실제로 갈리는지 한 화면에서 견준다.
	exec(`INSERT INTO categories (id, name, slug, sort_order) VALUES (1, '라이프', 'life', 0)`)
	exec(`INSERT INTO categories (id, name, slug, sort_order) VALUES (2, '개발', 'dev', 1)`)
	post := func(id int, slug, title, date string, cat int) {
		exec(`INSERT INTO posts (id, slug, title, body, status, source, category_id,
		                         sort_order, original_created_at, created_at, updated_at)
		      VALUES (?, ?, ?, '본문', 'unlisted', 'notion', ?, 0, ?, datetime('now'), datetime('now'))`,
			id, slug, title, cat, date)
	}
	for i, c := range []int{1, 2} {
		post(10+i*10, "a-"+itoa(c), "가 글", "2024-01-01", c)
		post(11+i*10, "b-"+itoa(c), "나 글", "2025-06-01", c)
		post(12+i*10, "c-"+itoa(c), "다 글", "2023-03-01", c)
	}

	srv, err := New(sqlDB)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	h := srv.Handler()

	titles := func(path string) []string {
		t.Helper()
		body := mainOf(t, get(t, h, path).Body.String())
		out := []string{}
		for _, m := range regexp.MustCompile(`/p/[^"]+">([^<]+)</a>`).FindAllStringSubmatch(body, -1) {
			out = append(out, m[1])
		}
		return out
	}

	if got := strings.Join(titles("/life"), " "); got != "나 글 가 글 다 글" {
		t.Errorf("라이프가 %q다. 최근 글부터인 \"나 글 가 글 다 글\"이어야 한다", got)
	}
	// 다른 분류는 그대로 읽는 차례다. 규칙이 새는지 함께 본다.
	if got := strings.Join(titles("/dev"), " "); got != "가 글 나 글 다 글" {
		t.Errorf("개발이 %q다. 예전 규칙대로 \"가 글 나 글 다 글\"이어야 한다", got)
	}
}

// 날짜 없는 글은 뒤로 보낸다. 어느 날짜 옆에 놓을지 알 방법이 없다 —
// 번호 없는 글을 번호 있는 글 사이에 안 끼우는 것과 같은 판단이다.
func TestLifePutsUndatedPostsLast(t *testing.T) {
	sqlDB := testDB(t)
	exec := execer(t, sqlDB)
	exec(`INSERT INTO categories (id, name, slug, sort_order) VALUES (1, '라이프', 'life', 0)`)
	exec(`INSERT INTO posts (id, slug, title, body, status, source, category_id, sort_order,
	                         original_created_at, created_at, updated_at)
	      VALUES (1, 'dated', '날짜 있는 글', '본문', 'unlisted', 'notion', 1, 0,
	              '2020-01-01', datetime('now'), datetime('now'))`)
	exec(`INSERT INTO posts (id, slug, title, body, status, source, category_id, sort_order,
	                         created_at, updated_at)
	      VALUES (2, 'undated', '날짜 없는 글', '본문', 'unlisted', 'notion', 1, 0,
	              datetime('now'), datetime('now'))`)

	srv, err := New(sqlDB)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	body := mainOf(t, get(t, srv.Handler(), "/life").Body.String())
	if strings.Index(body, "날짜 있는 글") > strings.Index(body, "날짜 없는 글") {
		t.Errorf("날짜 없는 글이 앞에 왔다:\n%s", body)
	}
}

// **순서를 따로 정할 수 없다고 말한다.** 여기서 옮겨봐야 다음에 쓴 글이 그
// 앞에 서므로, 되지도 않는 일을 되는 것처럼 보여주지 않는다.
func TestSiblingOrderRefusesRecentFirstCategories(t *testing.T) {
	sqlDB := testDB(t)
	exec := execer(t, sqlDB)
	exec(`INSERT INTO categories (id, name, slug, sort_order) VALUES (1, '라이프', 'life', 0)`)
	exec(`INSERT INTO posts (id, slug, title, body, status, source, category_id, sort_order,
	                         original_created_at, created_at, updated_at)
	      VALUES (1, 'one', '글 하나', '본문', 'unlisted', 'notion', 1, 0,
	              '2020-01-01', datetime('now'), datetime('now'))`)
	if got := orderOf(t, sqlDB, "one"); got.Reason == "" {
		t.Errorf("날짜순 분류인데 순서를 정할 수 있다고 한다: %v", siblingTitles(got))
	}
}

func itoa(n int) string { return string(rune('0' + n)) }
