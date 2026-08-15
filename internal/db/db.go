// Package db는 SQLite 연결과 스키마 마이그레이션을 담당한다.
package db

import (
	"database/sql"
	"fmt"

	// cgo 없이 순수 Go로 빌드되는 SQLite 드라이버.
	// 크로스 컴파일과 단일 바이너리 배포가 목적이라 mattn/go-sqlite3 대신 이걸 쓴다.
	_ "modernc.org/sqlite"
)

// Open은 SQLite 파일을 열고 커넥션 풀을 반환한다.
//
// PRAGMA는 커넥션 단위 설정이라 풀에서 새 커넥션이 생길 때마다 다시 걸려야 한다.
// 그래서 Exec로 한 번 실행하는 대신 DSN에 실어보낸다. 특히 foreign_keys는 SQLite에서
// 기본값이 OFF라 이걸 빠뜨리면 외래키 제약이 조용히 무시된다.
//
// _time_format=sqlite도 빠뜨리면 안 된다. 이게 없으면 드라이버가 time.Time을
// Go의 String() 형식("2026-08-15 09:55:58.9 +0000 UTC")으로 저장하는데, SQLite의
// date/datetime/strftime이 이걸 파싱하지 못해서 전부 NULL을 돌려준다.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_time_format=sqlite",
		path,
	)

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("SQLite 열기(%s): %w", path, err)
	}

	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("SQLite 연결(%s): %w", path, err)
	}

	// DSN의 _pragma가 실제로 먹었는지 확인한다. 드라이버가 인식하지 못하는 형식이면
	// 에러 없이 무시될 수 있고, 그러면 외래키가 꺼진 채로 돌아간다.
	if err := verifyForeignKeys(sqlDB); err != nil {
		sqlDB.Close()
		return nil, err
	}

	return sqlDB, nil
}

func verifyForeignKeys(sqlDB *sql.DB) error {
	var on int
	if err := sqlDB.QueryRow("PRAGMA foreign_keys").Scan(&on); err != nil {
		return fmt.Errorf("PRAGMA foreign_keys 확인: %w", err)
	}
	if on != 1 {
		return fmt.Errorf("PRAGMA foreign_keys가 켜지지 않았다 (값: %d)", on)
	}
	return nil
}
