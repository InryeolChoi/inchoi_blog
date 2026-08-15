package db

import (
	"testing"
	"time"
)

// TestTimestampsAreQueryableBySQLite는 저장된 시각을 SQLite의 날짜 함수가 읽을 수
// 있는지 확인한다.
//
// 드라이버 기본 설정은 time.Time을 Go의 String() 형식으로 저장하는데, SQLite는 그걸
// 날짜로 인식하지 못해서 datetime()이 조용히 NULL을 돌려준다. 에러가 아니라 NULL이라
// "발행일 기준 목록"같은 쿼리가 아무 소리 없이 빈 결과를 내게 된다. Open의
// _time_format=sqlite가 그걸 막고 있는지 검증한다.
func TestTimestampsAreQueryableBySQLite(t *testing.T) {
	sqlDB := migratedDB(t)

	published := time.Date(2024, 3, 9, 1, 29, 0, 0, time.UTC)
	_, err := sqlDB.Exec(`
		INSERT INTO posts (slug, title, body, status, source, published_at, created_at, updated_at)
		VALUES ('t', 't', '', 'published', 'native', ?, ?, ?)`,
		published, published, published)
	if err != nil {
		t.Fatalf("INSERT: %v", err)
	}

	var gotDate, gotYear string
	err = sqlDB.QueryRow(
		`SELECT coalesce(date(published_at), ''), coalesce(strftime('%Y', published_at), '') FROM posts`,
	).Scan(&gotDate, &gotYear)
	if err != nil {
		t.Fatalf("날짜 함수 조회: %v", err)
	}

	if gotDate != "2024-03-09" {
		t.Errorf("date(published_at) = %q, want \"2024-03-09\" (SQLite가 형식을 파싱하지 못했다)", gotDate)
	}
	if gotYear != "2024" {
		t.Errorf("strftime('%%Y', published_at) = %q, want \"2024\"", gotYear)
	}
}

// TestTimestampRoundTrip은 넣은 시각이 그대로 돌아오는지 확인한다.
func TestTimestampRoundTrip(t *testing.T) {
	sqlDB := migratedDB(t)

	want := time.Date(2023, 7, 11, 9, 54, 30, 0, time.UTC)
	_, err := sqlDB.Exec(`
		INSERT INTO posts (slug, title, body, status, source, published_at, created_at, updated_at)
		VALUES ('rt', 'rt', '', 'published', 'native', ?, ?, ?)`,
		want, want, want)
	if err != nil {
		t.Fatalf("INSERT: %v", err)
	}

	var got time.Time
	if err := sqlDB.QueryRow(`SELECT published_at FROM posts WHERE slug = 'rt'`).Scan(&got); err != nil {
		t.Fatalf("SELECT: %v", err)
	}

	if !got.UTC().Equal(want) {
		t.Errorf("시각이 왕복에서 변했다: got %s, want %s", got.UTC(), want)
	}
}

// TestPublishedAtOrdering은 발행일 정렬이 의도대로 되는지 본다.
// 블로그 첫 화면이 이 쿼리에 의존한다.
func TestPublishedAtOrdering(t *testing.T) {
	sqlDB := migratedDB(t)

	times := map[string]time.Time{
		"old":    time.Date(2022, 9, 7, 5, 26, 0, 0, time.UTC),
		"middle": time.Date(2024, 3, 9, 1, 29, 0, 0, time.UTC),
		"new":    time.Date(2026, 8, 15, 18, 0, 0, 0, time.UTC),
	}
	for slug, ts := range times {
		_, err := sqlDB.Exec(`
			INSERT INTO posts (slug, title, body, status, source, published_at, created_at, updated_at)
			VALUES (?, ?, '', 'published', 'native', ?, ?, ?)`,
			slug, slug, ts, ts, ts)
		if err != nil {
			t.Fatalf("INSERT %s: %v", slug, err)
		}
	}

	rows, err := sqlDB.Query(`SELECT slug FROM posts ORDER BY published_at DESC`)
	if err != nil {
		t.Fatalf("SELECT: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			t.Fatalf("Scan: %v", err)
		}
		got = append(got, slug)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	want := []string{"new", "middle", "old"}
	if len(got) != len(want) {
		t.Fatalf("행 수가 다르다: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("정렬 순서가 다르다 [%d]: got %s, want %s", i, got[i], want[i])
		}
	}
}
