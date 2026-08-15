// Package blog는 모듈 루트에서 바이너리에 포함시킬 자산을 노출한다.
//
// embed는 상위 디렉토리를 참조할 수 없어서(//go:embed ../migrations 는 불가능하다)
// migrations/를 저장소 루트에 두려면 embed 선언도 루트 패키지에 있어야 한다.
package blog

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// MigrationsFS는 migrations/ 안의 SQL 파일만 담긴 파일시스템을 돌려준다.
// db.Migrate에 그대로 넘기면 된다.
func MigrationsFS() fs.FS {
	sub, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		// embed 경로가 컴파일 시점에 고정이라 여기까지 올 수 없다.
		panic(err)
	}
	return sub
}
