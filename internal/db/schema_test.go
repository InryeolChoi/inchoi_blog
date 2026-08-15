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

// TestCategoryDepthLimit는 카테고리가 2단계를 넘지 못하게 막는 트리거를 확인한다.
func TestCategoryDepthLimit(t *testing.T) {
	sqlDB := migratedDB(t)

	if _, err := sqlDB.Exec(`INSERT INTO categories (id, parent_id, name, slug) VALUES (1, NULL, '개발', 'dev')`); err != nil {
		t.Fatalf("1단계 카테고리: %v", err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO categories (id, parent_id, name, slug) VALUES (2, 1, 'Go', 'go')`); err != nil {
		t.Fatalf("2단계 카테고리: %v", err)
	}

	_, err := sqlDB.Exec(`INSERT INTO categories (id, parent_id, name, slug) VALUES (3, 2, '동시성', 'concurrency')`)
	if err == nil {
		t.Fatal("3단계 카테고리가 통과했다")
	}
	if !strings.Contains(err.Error(), "2단계") {
		t.Errorf("에러 메시지가 기대와 다르다: %v", err)
	}

	// UPDATE로 우회하는 것도 막혀야 한다.
	if _, err := sqlDB.Exec(`INSERT INTO categories (id, parent_id, name, slug) VALUES (4, NULL, '기타', 'misc')`); err != nil {
		t.Fatalf("최상위 카테고리: %v", err)
	}
	if _, err := sqlDB.Exec(`UPDATE categories SET parent_id = 2 WHERE id = 4`); err == nil {
		t.Error("UPDATE로 3단계가 만들어졌다")
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
