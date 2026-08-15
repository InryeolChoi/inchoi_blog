package db

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/inryeol/blog"
)

// openTestDB는 테스트용 임시 DB를 연다.
// ":memory:"를 쓰지 않는 이유: database/sql은 커넥션 풀이라 커넥션마다 별개의
// 인메모리 DB가 생겨서 마이그레이션을 건 DB와 조회하는 DB가 달라질 수 있다.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return sqlDB
}

func tableExists(t *testing.T, sqlDB *sql.DB, name string) bool {
	t.Helper()
	var count int
	err := sqlDB.QueryRow(
		"SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name,
	).Scan(&count)
	if err != nil {
		t.Fatalf("sqlite_master 조회(%s): %v", name, err)
	}
	return count > 0
}

func indexExists(t *testing.T, sqlDB *sql.DB, name string) bool {
	t.Helper()
	var count int
	err := sqlDB.QueryRow(
		"SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = ?", name,
	).Scan(&count)
	if err != nil {
		t.Fatalf("sqlite_master 조회(%s): %v", name, err)
	}
	return count > 0
}

// TestMigrateCreatesSchema는 실제 migrations/001_init.sql이 테이블과 인덱스를
// 만들어내는지 확인한다.
func TestMigrateCreatesSchema(t *testing.T) {
	sqlDB := openTestDB(t)

	ran, err := Migrate(sqlDB, blog.MigrationsFS())
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if len(ran) == 0 {
		t.Fatal("첫 실행인데 적용된 마이그레이션이 없다")
	}
	if ran[0] != "001_init.sql" {
		t.Errorf("첫 마이그레이션이 001_init.sql이 아니다: %s", ran[0])
	}

	for _, table := range []string{"posts", "categories", "tags", "post_tags", "images", "schema_migrations"} {
		if !tableExists(t, sqlDB, table) {
			t.Errorf("테이블이 없다: %s", table)
		}
	}

	for _, index := range []string{
		"idx_posts_parent_id", "idx_posts_category_id", "idx_posts_status",
		"idx_categories_parent_id", "idx_post_tags_tag_id",
	} {
		if !indexExists(t, sqlDB, index) {
			t.Errorf("인덱스가 없다: %s", index)
		}
	}
}

// TestMigrateIsIdempotent는 러너를 두 번 돌려도 안전한지 확인한다.
// applied_at까지 비교해서, 두 번째 실행이 "다시 적용하고 기록만 덮어쓴" 게 아니라
// 정말 건너뛴 것임을 확인한다.
func TestMigrateIsIdempotent(t *testing.T) {
	sqlDB := openTestDB(t)

	first, err := Migrate(sqlDB, blog.MigrationsFS())
	if err != nil {
		t.Fatalf("첫 Migrate: %v", err)
	}

	var countBefore int
	var appliedAtBefore string
	row := sqlDB.QueryRow("SELECT count(*), coalesce(max(applied_at), '') FROM schema_migrations")
	if err := row.Scan(&countBefore, &appliedAtBefore); err != nil {
		t.Fatalf("적용 기록 조회: %v", err)
	}
	if countBefore != len(first) {
		t.Fatalf("적용 기록 수(%d)와 반환된 목록 수(%d)가 다르다", countBefore, len(first))
	}

	second, err := Migrate(sqlDB, blog.MigrationsFS())
	if err != nil {
		t.Fatalf("두 번째 Migrate: %v", err)
	}
	if len(second) != 0 {
		t.Errorf("두 번째 실행에서 적용된 게 있다: %v", second)
	}

	var countAfter int
	var appliedAtAfter string
	row = sqlDB.QueryRow("SELECT count(*), coalesce(max(applied_at), '') FROM schema_migrations")
	if err := row.Scan(&countAfter, &appliedAtAfter); err != nil {
		t.Fatalf("적용 기록 조회: %v", err)
	}
	if countAfter != countBefore {
		t.Errorf("적용 기록 수가 변했다: %d → %d", countBefore, countAfter)
	}
	if appliedAtAfter != appliedAtBefore {
		t.Errorf("applied_at이 변했다 (마이그레이션이 다시 실행됐다): %s → %s",
			appliedAtBefore, appliedAtAfter)
	}
}

// TestMigrateAppliesOnlyNew는 이미 적용된 게 있는 DB에 새 마이그레이션을 추가했을 때
// 새 것만 실행되는지 확인한다.
func TestMigrateAppliesOnlyNew(t *testing.T) {
	sqlDB := openTestDB(t)

	v1 := fstest.MapFS{
		"001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE a (id INTEGER PRIMARY KEY);`)},
	}
	if _, err := Migrate(sqlDB, v1); err != nil {
		t.Fatalf("v1 Migrate: %v", err)
	}

	v2 := fstest.MapFS{
		"001_init.sql":      v1["001_init.sql"],
		"002_add_table.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE b (id INTEGER PRIMARY KEY);`)},
	}
	ran, err := Migrate(sqlDB, v2)
	if err != nil {
		t.Fatalf("v2 Migrate: %v", err)
	}

	if len(ran) != 1 || ran[0] != "002_add_table.sql" {
		t.Fatalf("002만 적용돼야 하는데: %v", ran)
	}
	if !tableExists(t, sqlDB, "b") {
		t.Error("테이블 b가 생성되지 않았다")
	}
}

// TestMigrateOrdersByVersionNotFilename은 10 이상의 버전이 사전순으로 밀리지 않는지 본다.
// 파일명 사전순이면 "010"이 "002"보다 먼저 와서 순서가 깨진다.
func TestMigrateOrdersByVersionNotFilename(t *testing.T) {
	sqlDB := openTestDB(t)

	fsys := fstest.MapFS{
		"010_third.sql":  &fstest.MapFile{Data: []byte(`CREATE TABLE c (id INTEGER PRIMARY KEY);`)},
		"002_second.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE b (id INTEGER PRIMARY KEY);`)},
		"001_first.sql":  &fstest.MapFile{Data: []byte(`CREATE TABLE a (id INTEGER PRIMARY KEY);`)},
	}
	ran, err := Migrate(sqlDB, fsys)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	want := []string{"001_first.sql", "002_second.sql", "010_third.sql"}
	if len(ran) != len(want) {
		t.Fatalf("적용 개수가 다르다: %v", ran)
	}
	for i := range want {
		if ran[i] != want[i] {
			t.Errorf("순서가 다르다 [%d]: got %s, want %s", i, ran[i], want[i])
		}
	}
}

// TestMigrateRejectsModifiedMigration은 이미 적용된 파일이 수정됐을 때 막는지 확인한다.
func TestMigrateRejectsModifiedMigration(t *testing.T) {
	sqlDB := openTestDB(t)

	original := fstest.MapFS{
		"001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE a (id INTEGER PRIMARY KEY);`)},
	}
	if _, err := Migrate(sqlDB, original); err != nil {
		t.Fatalf("첫 Migrate: %v", err)
	}

	modified := fstest.MapFS{
		"001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE a (id INTEGER PRIMARY KEY, extra TEXT);`)},
	}
	_, err := Migrate(sqlDB, modified)
	if err == nil {
		t.Fatal("수정된 마이그레이션인데 에러가 없다")
	}
	if !strings.Contains(err.Error(), "수정됐다") {
		t.Errorf("에러 메시지가 기대와 다르다: %v", err)
	}
}

func TestMigrateRejectsBadFilenames(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantErr  string
	}{
		{"번호 없음", "init.sql", "형식이 아니다"},
		{"구분자 없음", "001-init.sql", "형식이 아니다"},
		{"down 마이그레이션", "001_init.down.sql", "down 마이그레이션"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlDB := openTestDB(t)
			fsys := fstest.MapFS{
				tt.filename: &fstest.MapFile{Data: []byte(`CREATE TABLE a (id INTEGER PRIMARY KEY);`)},
			}
			_, err := Migrate(sqlDB, fsys)
			if err == nil {
				t.Fatalf("%s인데 에러가 없다", tt.name)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("에러 메시지가 기대와 다르다: %v", err)
			}
		})
	}
}

func TestMigrateRejectsDuplicateVersion(t *testing.T) {
	sqlDB := openTestDB(t)

	fsys := fstest.MapFS{
		"001_init.sql":  &fstest.MapFile{Data: []byte(`CREATE TABLE a (id INTEGER PRIMARY KEY);`)},
		"001_other.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE b (id INTEGER PRIMARY KEY);`)},
	}
	_, err := Migrate(sqlDB, fsys)
	if err == nil {
		t.Fatal("버전이 중복인데 에러가 없다")
	}
	if !strings.Contains(err.Error(), "중복") {
		t.Errorf("에러 메시지가 기대와 다르다: %v", err)
	}
}

// TestMigrateRollsBackOnFailure는 마이그레이션이 중간에 실패하면 앞부분의 DDL과
// 적용 기록이 함께 롤백되는지 확인한다.
func TestMigrateRollsBackOnFailure(t *testing.T) {
	sqlDB := openTestDB(t)

	fsys := fstest.MapFS{
		"001_broken.sql": &fstest.MapFile{Data: []byte(
			"CREATE TABLE a (id INTEGER PRIMARY KEY);\nTHIS IS NOT SQL;",
		)},
	}
	if _, err := Migrate(sqlDB, fsys); err == nil {
		t.Fatal("깨진 SQL인데 에러가 없다")
	}

	if tableExists(t, sqlDB, "a") {
		t.Error("실패한 마이그레이션의 테이블 a가 남아 있다 (롤백되지 않았다)")
	}

	var count int
	if err := sqlDB.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("적용 기록 조회: %v", err)
	}
	if count != 0 {
		t.Errorf("실패한 마이그레이션이 기록에 남았다: %d건", count)
	}
}
