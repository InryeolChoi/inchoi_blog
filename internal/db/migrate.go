package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// migrationFilePattern은 001_init.sql 형식만 받는다.
var migrationFilePattern = regexp.MustCompile(`^(\d+)_([a-z0-9_]+)\.sql$`)

type migration struct {
	version  int
	name     string
	filename string
	sql      string
	checksum string
}

// Migrate는 fsys 안의 마이그레이션 중 아직 적용되지 않은 것을 번호순으로 실행한다.
// 이미 적용된 것은 건너뛰므로 몇 번을 실행해도 결과가 같다.
// 반환값은 이번 호출에서 실제로 적용한 파일명 목록이다(적용된 게 없으면 빈 슬라이스).
//
// down 마이그레이션은 없다. 스키마를 되돌려야 하면 되돌리는 내용의 새 마이그레이션을
// 추가한다. 컬럼이나 테이블을 삭제하는 마이그레이션은 이 프로젝트에서 금지다.
func Migrate(sqlDB *sql.DB, fsys fs.FS) ([]string, error) {
	if err := ensureMigrationsTable(sqlDB); err != nil {
		return nil, err
	}

	available, err := loadMigrations(fsys)
	if err != nil {
		return nil, err
	}

	applied, err := appliedMigrations(sqlDB)
	if err != nil {
		return nil, err
	}

	var ran []string
	for _, m := range available {
		if prevChecksum, ok := applied[m.version]; ok {
			// 이미 적용된 마이그레이션의 내용이 바뀌었다면, DB의 실제 스키마와
			// 파일이 말하는 스키마가 갈라진 것이다. 조용히 넘어가면 안 된다.
			if prevChecksum != m.checksum {
				return nil, fmt.Errorf(
					"이미 적용된 마이그레이션이 수정됐다: %s (적용 당시 %s, 현재 %s). "+
						"적용된 파일은 고치지 말고 새 마이그레이션을 추가해라",
					m.filename, shortSum(prevChecksum), shortSum(m.checksum),
				)
			}
			continue
		}

		if err := applyMigration(sqlDB, m); err != nil {
			return ran, err
		}
		ran = append(ran, m.filename)
	}

	return ran, nil
}

func ensureMigrationsTable(sqlDB *sql.DB) error {
	const stmt = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    checksum   TEXT NOT NULL,
    applied_at TIMESTAMP NOT NULL
)`
	if _, err := sqlDB.Exec(stmt); err != nil {
		return fmt.Errorf("schema_migrations 생성: %w", err)
	}
	return nil
}

// appliedMigrations는 적용된 버전 → 체크섬 맵을 읽어온다.
func appliedMigrations(sqlDB *sql.DB) (map[int]string, error) {
	rows, err := sqlDB.Query("SELECT version, checksum FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("적용된 마이그레이션 조회: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]string)
	for rows.Next() {
		var version int
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, fmt.Errorf("적용된 마이그레이션 조회: %w", err)
		}
		applied[version] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("적용된 마이그레이션 조회: %w", err)
	}
	return applied, nil
}

// applyMigration은 마이그레이션 하나를 트랜잭션 안에서 실행하고 기록까지 남긴다.
// SQLite는 DDL도 트랜잭션에 포함되므로, 중간에 실패하면 스키마 변경과 기록이 같이 롤백된다.
func applyMigration(sqlDB *sql.DB, m migration) error {
	tx, err := sqlDB.Begin()
	if err != nil {
		return fmt.Errorf("%s: 트랜잭션 시작: %w", m.filename, err)
	}
	defer tx.Rollback() // 커밋에 성공했으면 no-op이다.

	if _, err := tx.Exec(m.sql); err != nil {
		return fmt.Errorf("%s: 실행 실패: %w", m.filename, err)
	}

	_, err = tx.Exec(
		"INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, ?)",
		m.version, m.name, m.checksum, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("%s: 적용 기록: %w", m.filename, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%s: 커밋: %w", m.filename, err)
	}
	return nil
}

// loadMigrations는 fsys의 .sql 파일을 읽어 버전 오름차순으로 돌려준다.
func loadMigrations(fsys fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("마이그레이션 디렉토리 읽기: %w", err)
	}

	var migrations []migration
	seen := make(map[int]string)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		name := entry.Name()

		if strings.HasSuffix(name, ".down.sql") {
			return nil, fmt.Errorf(
				"down 마이그레이션은 쓰지 않는다: %s. "+
					"되돌려야 하면 되돌리는 내용의 새 마이그레이션을 추가해라", name)
		}

		match := migrationFilePattern.FindStringSubmatch(name)
		if match == nil {
			return nil, fmt.Errorf(
				"마이그레이션 파일명 형식이 아니다: %s (001_init.sql 형식이어야 한다)", name)
		}

		version, err := strconv.Atoi(match[1])
		if err != nil {
			return nil, fmt.Errorf("%s: 버전 번호 파싱: %w", name, err)
		}
		if version < 1 {
			return nil, fmt.Errorf("%s: 버전 번호는 1 이상이어야 한다", name)
		}
		if prev, dup := seen[version]; dup {
			return nil, fmt.Errorf("마이그레이션 버전 %d가 중복이다: %s, %s", version, prev, name)
		}
		seen[version] = name

		content, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("%s 읽기: %w", name, err)
		}
		sum := sha256.Sum256(content)

		migrations = append(migrations, migration{
			version:  version,
			name:     match[2],
			filename: name,
			sql:      string(content),
			checksum: hex.EncodeToString(sum[:]),
		})
	}

	// 파일명 사전순이 아니라 버전 숫자순으로 돌린다(예: 9 < 10).
	slices.SortFunc(migrations, func(a, b migration) int { return a.version - b.version })

	return migrations, nil
}

func shortSum(checksum string) string {
	if len(checksum) > 12 {
		return checksum[:12]
	}
	return checksum
}
