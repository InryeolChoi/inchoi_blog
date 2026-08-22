package importer

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/inryeol/blog"
	"github.com/inryeol/blog/internal/db"
)

func migratedDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if _, err := db.Migrate(sqlDB, blog.MigrationsFS()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return sqlDB
}

func inTx(t *testing.T, sqlDB *sql.DB, fn func(*sql.Tx) error) error {
	t.Helper()
	tx, err := sqlDB.Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func samplePost() Post {
	created := time.Date(2023, 7, 11, 9, 54, 0, 0, time.UTC)
	return Post{
		Slug:              "abc-123",
		Title:             "리다이렉션과 파이프",
		Body:              "# 본문",
		Status:            "unlisted",
		Source:            "notion",
		NotionPageID:      "abc-123",
		OriginalPath:      "école 42 > pipex",
		OriginalCreatedAt: &created,
	}
}

func TestUpsertPostInserts(t *testing.T) {
	sqlDB := migratedDB(t)
	now := time.Now().UTC()

	if err := inTx(t, sqlDB, func(tx *sql.Tx) error {
		return UpsertPost(tx, samplePost(), now)
	}); err != nil {
		t.Fatalf("UpsertPost: %v", err)
	}

	var title, status, source, path string
	var createdAt time.Time
	err := sqlDB.QueryRow(`
		SELECT title, status, source, original_path, original_created_at
		FROM posts WHERE notion_page_id = 'abc-123'`).
		Scan(&title, &status, &source, &path, &createdAt)
	if err != nil {
		t.Fatalf("조회: %v", err)
	}
	if title != "리다이렉션과 파이프" || status != "unlisted" || source != "notion" {
		t.Errorf("값이 다르다: %q %q %q", title, status, source)
	}
	if path != "école 42 > pipex" {
		t.Errorf("original_path가 다르다: %q", path)
	}
	if !createdAt.UTC().Equal(time.Date(2023, 7, 11, 9, 54, 0, 0, time.UTC)) {
		t.Errorf("original_created_at이 다르다: %s", createdAt)
	}
}

// TestUpsertPostIsIdempotent는 같은 페이지를 두 번 넣어도 한 건만 남는지 본다.
// 재이관이 안전해야 한다는 게 notion_page_id를 UNIQUE로 둔 이유다.
func TestUpsertPostIsIdempotent(t *testing.T) {
	sqlDB := migratedDB(t)
	now := time.Now().UTC()

	for i := 0; i < 2; i++ {
		if err := inTx(t, sqlDB, func(tx *sql.Tx) error {
			return UpsertPost(tx, samplePost(), now)
		}); err != nil {
			t.Fatalf("%d번째 UpsertPost: %v", i+1, err)
		}
	}

	var count int
	if err := sqlDB.QueryRow(`SELECT count(*) FROM posts`).Scan(&count); err != nil {
		t.Fatalf("조회: %v", err)
	}
	if count != 1 {
		t.Errorf("행이 %d건이다. 재이관에서 중복이 생겼다", count)
	}
}

// TestUpsertPostUpdatesBodyKeepsCreatedAt은 다시 이관하면 본문은 갱신되고
// 최초 생성 시각은 유지되는지 본다.
func TestUpsertPostUpdatesBodyKeepsCreatedAt(t *testing.T) {
	sqlDB := migratedDB(t)
	first := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	second := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	p := samplePost()
	if err := inTx(t, sqlDB, func(tx *sql.Tx) error { return UpsertPost(tx, p, first) }); err != nil {
		t.Fatalf("첫 UpsertPost: %v", err)
	}
	p.Body = "# 고친 본문"
	if err := inTx(t, sqlDB, func(tx *sql.Tx) error { return UpsertPost(tx, p, second) }); err != nil {
		t.Fatalf("두 번째 UpsertPost: %v", err)
	}

	var body string
	var createdAt, updatedAt time.Time
	err := sqlDB.QueryRow(`SELECT body, created_at, updated_at FROM posts`).
		Scan(&body, &createdAt, &updatedAt)
	if err != nil {
		t.Fatalf("조회: %v", err)
	}
	if body != "# 고친 본문" {
		t.Errorf("본문이 갱신되지 않았다: %q", body)
	}
	if !createdAt.UTC().Equal(first) {
		t.Errorf("created_at이 바뀌었다: %s (want %s)", createdAt.UTC(), first)
	}
	if !updatedAt.UTC().Equal(second) {
		t.Errorf("updated_at이 갱신되지 않았다: %s (want %s)", updatedAt.UTC(), second)
	}
}

func TestUpsertPostPreservesRecoveredSortOrderUnlessExplicit(t *testing.T) {
	sqlDB := migratedDB(t)
	now := time.Now().UTC()
	p := samplePost()

	if err := inTx(t, sqlDB, func(tx *sql.Tx) error { return UpsertPost(tx, p, now) }); err != nil {
		t.Fatalf("첫 UpsertPost: %v", err)
	}
	if _, err := sqlDB.Exec(`UPDATE posts SET sort_order = 27 WHERE notion_page_id = 'abc-123'`); err != nil {
		t.Fatalf("sort_order 준비: %v", err)
	}

	// 일반 재이관은 sortorder가 복원한 값을 지우면 안 된다.
	if err := inTx(t, sqlDB, func(tx *sql.Tx) error { return UpsertPost(tx, p, now) }); err != nil {
		t.Fatalf("두 번째 UpsertPost: %v", err)
	}
	var got int
	if err := sqlDB.QueryRow(`SELECT sort_order FROM posts WHERE notion_page_id = 'abc-123'`).Scan(&got); err != nil {
		t.Fatalf("조회: %v", err)
	}
	if got != 27 {
		t.Fatalf("일반 재이관이 복원 순서를 덮었다: got %d, want 27", got)
	}

	// 사람이 명시한 값은 재이관 때 고정한다.
	manual := 4
	p.SortOrder = &manual
	if err := inTx(t, sqlDB, func(tx *sql.Tx) error { return UpsertPost(tx, p, now) }); err != nil {
		t.Fatalf("수동 순서 UpsertPost: %v", err)
	}
	if err := sqlDB.QueryRow(`SELECT sort_order FROM posts WHERE notion_page_id = 'abc-123'`).Scan(&got); err != nil {
		t.Fatalf("조회: %v", err)
	}
	if got != 4 {
		t.Errorf("수동 순서가 적용되지 않았다: got %d, want 4", got)
	}
}

func TestUpsertPostRejectsBadStatus(t *testing.T) {
	sqlDB := migratedDB(t)
	p := samplePost()
	p.Status = "archived"

	err := inTx(t, sqlDB, func(tx *sql.Tx) error {
		return UpsertPost(tx, p, time.Now().UTC())
	})
	if err == nil {
		t.Fatal("잘못된 status인데 통과했다")
	}
	if !strings.Contains(err.Error(), "archived") {
		t.Errorf("에러에 문제 값이 안 나온다: %v", err)
	}
}

func TestUpsertPostRejectsEmptyNotionPageID(t *testing.T) {
	sqlDB := migratedDB(t)
	p := samplePost()
	p.NotionPageID = ""

	err := inTx(t, sqlDB, func(tx *sql.Tx) error {
		return UpsertPost(tx, p, time.Now().UTC())
	})
	if err == nil {
		t.Fatal("notion_page_id가 비었는데 통과했다")
	}
}

// TestUpsertPostNullsEmptyOriginalPath는 빈 문자열이 NULL로 들어가는지 본다.
// ""와 NULL이 섞이면 나중에 "원본 경로를 모르는 글" 찾기가 어려워진다.
func TestUpsertPostNullsEmptyOriginalPath(t *testing.T) {
	sqlDB := migratedDB(t)
	p := samplePost()
	p.OriginalPath = ""
	p.OriginalCreatedAt = nil

	if err := inTx(t, sqlDB, func(tx *sql.Tx) error {
		return UpsertPost(tx, p, time.Now().UTC())
	}); err != nil {
		t.Fatalf("UpsertPost: %v", err)
	}

	var pathNull, createdNull bool
	err := sqlDB.QueryRow(`
		SELECT original_path IS NULL, original_created_at IS NULL FROM posts`).
		Scan(&pathNull, &createdNull)
	if err != nil {
		t.Fatalf("조회: %v", err)
	}
	if !pathNull || !createdNull {
		t.Errorf("빈 값이 NULL로 안 들어갔다: path=%v created=%v", pathNull, createdNull)
	}
}

func TestUpsertImageRoundTrip(t *testing.T) {
	sqlDB := migratedDB(t)
	data := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a}

	if err := inTx(t, sqlDB, func(tx *sql.Tx) error {
		return UpsertImage(tx, Image{
			SHA256:      "deadbeef",
			Data:        data,
			MIME:        "image/png",
			OriginalURL: "https://example.com/x.png",
		}, time.Now().UTC())
	}); err != nil {
		t.Fatalf("UpsertImage: %v", err)
	}

	var got []byte
	var mime, url string
	err := sqlDB.QueryRow(`SELECT data, mime, original_url FROM images WHERE sha256 = 'deadbeef'`).
		Scan(&got, &mime, &url)
	if err != nil {
		t.Fatalf("조회: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("BLOB이 왕복에서 변했다: %v", got)
	}
	if mime != "image/png" || url != "https://example.com/x.png" {
		t.Errorf("값이 다르다: %q %q", mime, url)
	}
}

func TestUpsertImageIsIdempotent(t *testing.T) {
	sqlDB := migratedDB(t)
	img := Image{SHA256: "abc", Data: []byte{1, 2, 3}, MIME: "image/png"}

	for i := 0; i < 2; i++ {
		if err := inTx(t, sqlDB, func(tx *sql.Tx) error {
			return UpsertImage(tx, img, time.Now().UTC())
		}); err != nil {
			t.Fatalf("%d번째 UpsertImage: %v", i+1, err)
		}
	}

	var count int
	if err := sqlDB.QueryRow(`SELECT count(*) FROM images`).Scan(&count); err != nil {
		t.Fatalf("조회: %v", err)
	}
	if count != 1 {
		t.Errorf("행이 %d건이다. 같은 sha256이 중복 저장됐다", count)
	}
}

func TestUpsertImageRejectsEmptyData(t *testing.T) {
	sqlDB := migratedDB(t)
	err := inTx(t, sqlDB, func(tx *sql.Tx) error {
		return UpsertImage(tx, Image{SHA256: "abc", MIME: "image/png"}, time.Now().UTC())
	})
	if err == nil {
		t.Fatal("바이트가 없는데 통과했다")
	}
}

func TestMIMEForFile(t *testing.T) {
	tests := []struct {
		name string
		want string
		ok   bool
	}{
		{"abc.png", "image/png", true},
		{"abc.PNG", "image/png", true},
		{"abc.jpg", "image/jpeg", true},
		{"abc.jpeg", "image/jpeg", true},
		{"abc.gif", "image/gif", true},
		{"abc.webp", "image/webp", true},
		{"abc.xyz", "", false},
		{"abc", "", false},
	}
	for _, tt := range tests {
		got, ok := MIMEForFile(tt.name)
		if got != tt.want || ok != tt.ok {
			t.Errorf("MIMEForFile(%q) = (%q, %v), want (%q, %v)", tt.name, got, ok, tt.want, tt.ok)
		}
	}
}

// TestLoadPageMetaHandlesCommasInTitles는 제목에 콤마가 든 행에서
// 컬럼이 밀리지 않는지 본다. 줄을 콤마로 쪼개면 status 자리에 엉뚱한 값이 들어간다.
func TestLoadPageMetaHandlesCommasInTitles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.csv")
	content := `page_id,title,full_path,status
p1,"제목에, 콤마가, 있다","루트 > 하위",draft
p2,보통 제목,루트,unlisted
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	meta, err := LoadPageMeta(path)
	if err != nil {
		t.Fatalf("LoadPageMeta: %v", err)
	}
	if len(meta) != 2 {
		t.Fatalf("행 수가 다르다: %d", len(meta))
	}
	if meta["p1"].Title != "제목에, 콤마가, 있다" {
		t.Errorf("제목이 잘렸다: %q", meta["p1"].Title)
	}
	if meta["p1"].Status != "draft" {
		t.Errorf("status 컬럼이 밀렸다: %q", meta["p1"].Status)
	}
	if meta["p2"].Status != "unlisted" {
		t.Errorf("status가 다르다: %q", meta["p2"].Status)
	}
}

func TestLoadPageMetaRejectsMissingColumn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.csv")
	if err := os.WriteFile(path, []byte("page_id,title\np1,제목\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := LoadPageMeta(path); err == nil {
		t.Fatal("필요한 컬럼이 없는데 통과했다")
	}
}
