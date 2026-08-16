package db

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/inryeol/blog"
)

// migratedDB는 001_init.sql까지 적용된 테스트 DB를 준다.
func migratedDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB := openTestDB(t)
	if _, err := Migrate(sqlDB, blog.MigrationsFS()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return sqlDB
}

// insertPost는 posts에 한 건 넣는다. 제약 조건 테스트용이라 에러를 그대로 돌려준다.
func insertPost(sqlDB *sql.DB, slug, status string, categoryID, parentID any) error {
	now := time.Now().UTC()
	_, err := sqlDB.Exec(`
		INSERT INTO posts (slug, title, body, status, parent_id, category_id, source, created_at, updated_at)
		VALUES (?, ?, '', ?, ?, ?, 'native', ?, ?)`,
		slug, slug, status, parentID, categoryID, now, now)
	return err
}

func TestPostStatusCheckConstraint(t *testing.T) {
	sqlDB := migratedDB(t)

	for _, status := range []string{"draft", "published", "unlisted"} {
		if err := insertPost(sqlDB, "ok-"+status, status, nil, nil); err != nil {
			t.Errorf("status=%s는 허용돼야 하는데: %v", status, err)
		}
	}

	for _, status := range []string{"archived", "PUBLISHED", ""} {
		err := insertPost(sqlDB, "bad-"+status, status, nil, nil)
		if err == nil {
			t.Errorf("status=%q가 통과했다 (CHECK 제약이 없다)", status)
		}
	}
}

// TestForeignKeysAreEnforced는 PRAGMA foreign_keys가 실제로 켜져 있는지 본다.
// 꺼져 있으면 존재하지 않는 카테고리를 가리켜도 조용히 들어간다.
func TestForeignKeysAreEnforced(t *testing.T) {
	sqlDB := migratedDB(t)

	if err := insertPost(sqlDB, "dangling-category", "draft", 9999, nil); err == nil {
		t.Error("없는 category_id인데 INSERT가 통과했다 (외래키가 꺼져 있다)")
	}
	if err := insertPost(sqlDB, "dangling-parent", "draft", nil, 9999); err == nil {
		t.Error("없는 parent_id인데 INSERT가 통과했다 (외래키가 꺼져 있다)")
	}
}

func TestPostSlugIsUnique(t *testing.T) {
	sqlDB := migratedDB(t)

	if err := insertPost(sqlDB, "same-slug", "draft", nil, nil); err != nil {
		t.Fatalf("첫 INSERT: %v", err)
	}
	if err := insertPost(sqlDB, "same-slug", "draft", nil, nil); err == nil {
		t.Error("중복 slug가 통과했다")
	}
}

// TestNotionPageIDUniqueAllowsMultipleNulls는 재이관 멱등 키가 제 역할을 하면서도
// 노션에서 오지 않은 글 여러 개를 막지 않는지 확인한다.
func TestNotionPageIDUniqueAllowsMultipleNulls(t *testing.T) {
	sqlDB := migratedDB(t)
	now := time.Now().UTC()

	insert := func(slug string, notionID any) error {
		_, err := sqlDB.Exec(`
			INSERT INTO posts (slug, title, body, status, source, notion_page_id, created_at, updated_at)
			VALUES (?, ?, '', 'draft', 'notion', ?, ?, ?)`,
			slug, slug, notionID, now, now)
		return err
	}

	if err := insert("native-1", nil); err != nil {
		t.Fatalf("notion_page_id NULL 첫 건: %v", err)
	}
	if err := insert("native-2", nil); err != nil {
		t.Errorf("notion_page_id가 NULL인 두 번째 글이 막혔다: %v", err)
	}

	if err := insert("notion-1", "page-abc"); err != nil {
		t.Fatalf("notion_page_id 있는 건: %v", err)
	}
	if err := insert("notion-2", "page-abc"); err == nil {
		t.Error("같은 notion_page_id가 두 번 들어갔다 (재이관 멱등성이 깨진다)")
	}
}

// TestPostTagsCascade는 글을 지우면 태그 연결만 사라지고 태그 자체는 남는지 확인한다.
func TestPostTagsCascade(t *testing.T) {
	sqlDB := migratedDB(t)

	if err := insertPost(sqlDB, "tagged", "draft", nil, nil); err != nil {
		t.Fatalf("INSERT post: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO tags (id, name, slug) VALUES (1, 'Go', 'go')`); err != nil {
		t.Fatalf("INSERT tag: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO post_tags (post_id, tag_id) SELECT id, 1 FROM posts WHERE slug = 'tagged'`); err != nil {
		t.Fatalf("INSERT post_tags: %v", err)
	}

	if _, err := sqlDB.Exec(`DELETE FROM posts WHERE slug = 'tagged'`); err != nil {
		t.Fatalf("DELETE post: %v", err)
	}

	var links, tags int
	if err := sqlDB.QueryRow(`SELECT count(*) FROM post_tags`).Scan(&links); err != nil {
		t.Fatalf("post_tags 조회: %v", err)
	}
	if links != 0 {
		t.Errorf("글을 지웠는데 태그 연결이 남았다: %d건", links)
	}
	if err := sqlDB.QueryRow(`SELECT count(*) FROM tags`).Scan(&tags); err != nil {
		t.Fatalf("tags 조회: %v", err)
	}
	if tags != 1 {
		t.Errorf("글을 지웠는데 태그 자체가 사라졌다: %d건", tags)
	}
}

// TestImageSha256IsUnique는 이미지 중복 제거 키를 확인한다.
func TestImageSha256IsUnique(t *testing.T) {
	sqlDB := migratedDB(t)
	now := time.Now().UTC()

	insert := func() error {
		_, err := sqlDB.Exec(
			`INSERT INTO images (sha256, data, mime, created_at) VALUES ('abc123', ?, 'image/png', ?)`,
			[]byte{0x89, 0x50, 0x4e, 0x47}, now)
		return err
	}

	if err := insert(); err != nil {
		t.Fatalf("첫 INSERT: %v", err)
	}
	if err := insert(); err == nil {
		t.Error("같은 sha256이 두 번 들어갔다 (중복 제거가 안 된다)")
	}
}

// ---------- 002: 카테고리 3단계 ----------

// insertCategory는 카테고리를 하나 넣고 id를 돌려준다.
func insertCategory(t *testing.T, sqlDB *sql.DB, name string, parentID any) int64 {
	t.Helper()
	res, err := sqlDB.Exec(
		`INSERT INTO categories (name, slug, parent_id) VALUES (?, ?, ?)`, name, name, parentID)
	if err != nil {
		t.Fatalf("카테고리 %q 삽입: %v", name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}
	return id
}

// TestCategoryDepth3IsAllowed는 3단계까지는 들어가는지 본다.
func TestCategoryDepth3IsAllowed(t *testing.T) {
	sqlDB := migratedDB(t)

	l1 := insertCategory(t, sqlDB, "dev", nil)
	l2 := insertCategory(t, sqlDB, "Language", l1)
	insertCategory(t, sqlDB, "프로그래밍 언어", l2)

	var depth3 int
	err := sqlDB.QueryRow(`
		SELECT count(*) FROM categories c
		JOIN categories p ON c.parent_id = p.id
		WHERE p.parent_id IS NOT NULL`).Scan(&depth3)
	if err != nil {
		t.Fatalf("깊이 조회: %v", err)
	}
	if depth3 != 1 {
		t.Errorf("3단계가 %d개다. 1개여야 한다", depth3)
	}
}

// TestCategoryDepth4InsertIsBlocked는 4단계 삽입이 막히는지 본다.
func TestCategoryDepth4InsertIsBlocked(t *testing.T) {
	sqlDB := migratedDB(t)

	l1 := insertCategory(t, sqlDB, "dev", nil)
	l2 := insertCategory(t, sqlDB, "Language", l1)
	l3 := insertCategory(t, sqlDB, "프로그래밍 언어", l2)

	_, err := sqlDB.Exec(
		`INSERT INTO categories (name, slug, parent_id) VALUES ('Java', 'java', ?)`, l3)
	if err == nil {
		t.Fatal("4단계 삽입이 통과했다")
	}
	if !strings.Contains(err.Error(), "3단계") {
		t.Errorf("에러 메시지가 기대와 다르다: %v", err)
	}
}

// TestCategoryMoveLeafRespectsDepth는 자식 없는 노드를 옮길 때
// 부모 깊이만 보고 판정하는지 확인한다.
func TestCategoryMoveLeafRespectsDepth(t *testing.T) {
	sqlDB := migratedDB(t)

	l1 := insertCategory(t, sqlDB, "dev", nil)
	l2 := insertCategory(t, sqlDB, "Language", l1)
	l3 := insertCategory(t, sqlDB, "프로그래밍 언어", l2)
	leaf := insertCategory(t, sqlDB, "떠도는것", nil)

	// 깊이 2 밑으로 = 3단계. 허용돼야 한다.
	if _, err := sqlDB.Exec(`UPDATE categories SET parent_id = ? WHERE id = ?`, l2, leaf); err != nil {
		t.Fatalf("깊이 2 밑으로 이동이 막혔다: %v", err)
	}
	// 깊이 3 밑으로 = 4단계. 막혀야 한다.
	if _, err := sqlDB.Exec(`UPDATE categories SET parent_id = ? WHERE id = ?`, l3, leaf); err == nil {
		t.Error("깊이 3 밑으로 이동이 통과했다")
	}
}

// TestCategoryMoveSubtreeCountsChildren은 자식을 데리고 옮길 때
// 그 자식들의 깊이까지 계산하는지 본다.
//
// 001의 트리거는 이걸 못 봐서, 자식이 있는 최상위 노드를 다른 최상위 밑으로
// 옮기면 3단계가 조용히 생겼다. 그때는 2단계가 상한이었으므로 위반이었다.
func TestCategoryMoveSubtreeCountsChildren(t *testing.T) {
	sqlDB := migratedDB(t)

	newTop := insertCategory(t, sqlDB, "dev", nil)

	// 자식 하나만 있는 노드(높이 1) → 깊이 1 밑으로 가면 3단계. 허용.
	oneLevel := insertCategory(t, sqlDB, "Language", nil)
	insertCategory(t, sqlDB, "프로그래밍 언어", oneLevel)
	if _, err := sqlDB.Exec(`UPDATE categories SET parent_id = ? WHERE id = ?`, newTop, oneLevel); err != nil {
		t.Fatalf("높이 1짜리 서브트리 이동이 막혔다: %v", err)
	}

	// 손자까지 있는 노드(높이 2) → 어디로 옮겨도 4단계. 막혀야 한다.
	twoLevel := insertCategory(t, sqlDB, "네트워크", nil)
	mid := insertCategory(t, sqlDB, "HTTP", twoLevel)
	insertCategory(t, sqlDB, "상태코드", mid)

	_, err := sqlDB.Exec(`UPDATE categories SET parent_id = ? WHERE id = ?`, newTop, twoLevel)
	if err == nil {
		t.Fatal("높이 2짜리 서브트리를 옮기는 게 통과했다 (4단계가 된다)")
	}
	if !strings.Contains(err.Error(), "3단계") {
		t.Errorf("에러 메시지가 기대와 다르다: %v", err)
	}
}

// TestCategoryMoveSubtreeUnderDepth2IsBlocked는 자식 있는 노드를
// 깊이 2 밑으로 옮기면 막히는지 본다 (1+1+... = 4단계).
func TestCategoryMoveSubtreeUnderDepth2IsBlocked(t *testing.T) {
	sqlDB := migratedDB(t)

	l1 := insertCategory(t, sqlDB, "dev", nil)
	l2 := insertCategory(t, sqlDB, "Language", l1)

	withChild := insertCategory(t, sqlDB, "웹", nil)
	insertCategory(t, sqlDB, "React", withChild)

	_, err := sqlDB.Exec(`UPDATE categories SET parent_id = ? WHERE id = ?`, l2, withChild)
	if err == nil {
		t.Fatal("자식 있는 노드를 깊이 2 밑으로 옮기는 게 통과했다")
	}
}

// TestCategorySelfParentIsBlocked는 자기 자신을 부모로 삼는 걸 막는지 본다.
// 깊이 계산이 무한히 도는 상태다.
func TestCategorySelfParentIsBlocked(t *testing.T) {
	sqlDB := migratedDB(t)

	id := insertCategory(t, sqlDB, "dev", nil)
	_, err := sqlDB.Exec(`UPDATE categories SET parent_id = ? WHERE id = ?`, id, id)
	if err == nil {
		t.Fatal("자기 자신을 부모로 삼는 게 통과했다")
	}
	if !strings.Contains(err.Error(), "자기 자신") {
		t.Errorf("에러 메시지가 기대와 다르다: %v", err)
	}
}

// TestCategoryMoveToTopLevelAlwaysAllowed는 최상위로 빼는 건
// 언제나 허용되는지 본다 (깊이가 줄어드는 방향이다).
func TestCategoryMoveToTopLevelAlwaysAllowed(t *testing.T) {
	sqlDB := migratedDB(t)

	l1 := insertCategory(t, sqlDB, "dev", nil)
	l2 := insertCategory(t, sqlDB, "Language", l1)
	insertCategory(t, sqlDB, "프로그래밍 언어", l2)

	if _, err := sqlDB.Exec(`UPDATE categories SET parent_id = NULL WHERE id = ?`, l2); err != nil {
		t.Errorf("최상위로 빼는 게 막혔다: %v", err)
	}
}

// ---------- 003: source_name ----------

// TestCategorySourceNameSurvivesRename은 이름과 slug를 바꿔도
// source_name이 그대로 남는지 본다. 이게 categorize의 멱등 키다.
func TestCategorySourceNameSurvivesRename(t *testing.T) {
	sqlDB := migratedDB(t)

	_, err := sqlDB.Exec(`
		INSERT INTO categories (name, slug, source_name) VALUES ('소프트스킬', '소프트스킬', '소프트스킬')`)
	if err != nil {
		t.Fatalf("INSERT: %v", err)
	}
	if _, err := sqlDB.Exec(
		`UPDATE categories SET name = 'tooling', slug = 'tooling' WHERE source_name = '소프트스킬'`); err != nil {
		t.Fatalf("이름 변경: %v", err)
	}

	var name, slug string
	err = sqlDB.QueryRow(
		`SELECT name, slug FROM categories WHERE source_name = '소프트스킬'`).Scan(&name, &slug)
	if err != nil {
		t.Fatalf("source_name으로 못 찾는다: %v", err)
	}
	if name != "tooling" || slug != "tooling" {
		t.Errorf("이름 변경이 반영되지 않았다: %q %q", name, slug)
	}
}

// TestCategorySourceNameIsUnique는 같은 경로 이름이 두 번 들어가지 않는지 본다.
func TestCategorySourceNameIsUnique(t *testing.T) {
	sqlDB := migratedDB(t)

	if _, err := sqlDB.Exec(
		`INSERT INTO categories (name, slug, source_name) VALUES ('a', 'a', '운영체제')`); err != nil {
		t.Fatalf("첫 INSERT: %v", err)
	}
	if _, err := sqlDB.Exec(
		`INSERT INTO categories (name, slug, source_name) VALUES ('b', 'b', '운영체제')`); err == nil {
		t.Error("같은 source_name이 두 번 들어갔다")
	}
}

// TestCategorySourceNameAllowsMultipleNulls는 사람이 만든 상위 분류가
// 여럿 있어도 되는지 본다. 그런 건 경로에서 온 게 아니라 source_name이 NULL이다.
func TestCategorySourceNameAllowsMultipleNulls(t *testing.T) {
	sqlDB := migratedDB(t)

	for _, slug := range []string{"dev", "cs-theory", "algorithm"} {
		if _, err := sqlDB.Exec(
			`INSERT INTO categories (name, slug, source_name) VALUES (?, ?, NULL)`, slug, slug); err != nil {
			t.Fatalf("source_name이 NULL인 카테고리 %q: %v", slug, err)
		}
	}
	var n int
	if err := sqlDB.QueryRow(
		`SELECT count(*) FROM categories WHERE source_name IS NULL`).Scan(&n); err != nil {
		t.Fatalf("조회: %v", err)
	}
	if n != 3 {
		t.Errorf("NULL인 카테고리가 %d개다. 3개여야 한다", n)
	}
}

// ---------- 004: cover_post_id ----------

// coverFixture는 카테고리 하나와 그 안의 글 하나를 만든다.
func coverFixture(t *testing.T, sqlDB *sql.DB) (catID, postID int64) {
	t.Helper()
	catID = insertCategory(t, sqlDB, "운영체제", nil)
	now := time.Now().UTC()
	res, err := sqlDB.Exec(`
		INSERT INTO posts (slug, title, body, status, source, category_id, created_at, updated_at)
		VALUES ('os', '운영체제', '', 'unlisted', 'notion', ?, ?, ?)`, catID, now, now)
	if err != nil {
		t.Fatalf("INSERT post: %v", err)
	}
	postID, err = res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}
	return catID, postID
}

func TestCategoryCoverPostRoundTrip(t *testing.T) {
	sqlDB := migratedDB(t)
	catID, postID := coverFixture(t, sqlDB)

	if _, err := sqlDB.Exec(
		`UPDATE categories SET cover_post_id = ? WHERE id = ?`, postID, catID); err != nil {
		t.Fatalf("표지 글 연결: %v", err)
	}

	var got int64
	if err := sqlDB.QueryRow(
		`SELECT cover_post_id FROM categories WHERE id = ?`, catID).Scan(&got); err != nil {
		t.Fatalf("조회: %v", err)
	}
	if got != postID {
		t.Errorf("cover_post_id = %d, want %d", got, postID)
	}
}

// TestCategoryCoverPostForeignKey는 없는 글을 표지로 지정하지 못하는지 본다.
func TestCategoryCoverPostForeignKey(t *testing.T) {
	sqlDB := migratedDB(t)
	catID, _ := coverFixture(t, sqlDB)

	if _, err := sqlDB.Exec(
		`UPDATE categories SET cover_post_id = 999999 WHERE id = ?`, catID); err == nil {
		t.Error("없는 글을 표지로 지정하는 게 통과했다")
	}
}

// TestCategoryCoverPostClearedOnDelete는 표지 글을 지우면 연결만 풀리고
// 카테고리는 남는지 본다 (ON DELETE SET NULL).
func TestCategoryCoverPostClearedOnDelete(t *testing.T) {
	sqlDB := migratedDB(t)
	catID, postID := coverFixture(t, sqlDB)

	if _, err := sqlDB.Exec(
		`UPDATE categories SET cover_post_id = ? WHERE id = ?`, postID, catID); err != nil {
		t.Fatalf("표지 글 연결: %v", err)
	}
	// 글이 그 카테고리에 속해 있으면 category_id 외래키가 삭제를 막지 않는지 함께 본다.
	if _, err := sqlDB.Exec(`DELETE FROM posts WHERE id = ?`, postID); err != nil {
		t.Fatalf("글 삭제: %v", err)
	}

	var cover sql.NullInt64
	var cnt int
	if err := sqlDB.QueryRow(
		`SELECT count(*), max(cover_post_id) FROM categories WHERE id = ?`, catID).Scan(&cnt, &cover); err != nil {
		t.Fatalf("조회: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("카테고리가 사라졌다")
	}
	if cover.Valid {
		t.Errorf("표지 연결이 안 풀렸다: %d", cover.Int64)
	}
}

// TestCategoryCoverPostIsUnique는 한 글이 두 카테고리의 표지가 되지 못하는지 본다.
func TestCategoryCoverPostIsUnique(t *testing.T) {
	sqlDB := migratedDB(t)
	catA, postID := coverFixture(t, sqlDB)
	catB := insertCategory(t, sqlDB, "네트워크", nil)

	if _, err := sqlDB.Exec(
		`UPDATE categories SET cover_post_id = ? WHERE id = ?`, postID, catA); err != nil {
		t.Fatalf("첫 연결: %v", err)
	}
	if _, err := sqlDB.Exec(
		`UPDATE categories SET cover_post_id = ? WHERE id = ?`, postID, catB); err == nil {
		t.Error("같은 글이 두 카테고리의 표지가 됐다")
	}
}

// TestCategoryCoverPostAllowsMultipleNulls는 표지가 없는 카테고리가
// 여럿 있어도 되는지 본다.
func TestCategoryCoverPostAllowsMultipleNulls(t *testing.T) {
	sqlDB := migratedDB(t)
	for _, name := range []string{"a", "b", "c"} {
		insertCategory(t, sqlDB, name, nil)
	}
	var n int
	if err := sqlDB.QueryRow(
		`SELECT count(*) FROM categories WHERE cover_post_id IS NULL`).Scan(&n); err != nil {
		t.Fatalf("조회: %v", err)
	}
	if n != 3 {
		t.Errorf("표지 없는 카테고리가 %d개다. 3개여야 한다", n)
	}
}
