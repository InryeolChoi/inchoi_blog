// import는 노션 덤프를 DB로 이관하는 CLI다.
//
// 아직 뼈대만 있다. 지금은 덤프를 훑어서 무엇이 있는지 보고만 하고, 변환과 INSERT는
// 구현돼 있지 않다.
//
// 덤프 디렉토리는 읽기 전용으로만 다룬다. 재수집에 43분이 걸리므로 이 프로그램은
// 어떤 경우에도 덤프에 쓰거나 지우지 않는다.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/inryeol/blog"
	"github.com/inryeol/blog/internal/db"
)

func main() {
	dumpDir := flag.String("dump", "scripts/dump", "노션 덤프 디렉토리 (읽기 전용)")
	dbPath := flag.String("db", "blog.db", "SQLite 파일 경로")
	flag.Parse()

	pages, images, err := surveyDump(*dumpDir)
	if err != nil {
		log.Fatal(err)
	}

	sqlDB, err := db.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer sqlDB.Close()

	applied, err := db.Migrate(sqlDB, blog.MigrationsFS())
	if err != nil {
		log.Fatalf("마이그레이션 실패: %v", err)
	}
	for _, name := range applied {
		log.Printf("마이그레이션 적용: %s", name)
	}

	fmt.Printf("덤프: 페이지 %d개, 이미지 %d개 (%s)\n", pages, images, *dumpDir)
	fmt.Println("이관 로직은 아직 구현되지 않았다. DB에 아무것도 쓰지 않았다.")
}

// surveyDump는 덤프 디렉토리의 페이지 JSON과 이미지 개수를 센다. 읽기만 한다.
func surveyDump(dir string) (pages, images int, err error) {
	info, err := os.Stat(dir)
	if err != nil {
		return 0, 0, fmt.Errorf("덤프 디렉토리를 열 수 없다(%s): %w", dir, err)
	}
	if !info.IsDir() {
		return 0, 0, fmt.Errorf("덤프 경로가 디렉토리가 아니다: %s", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, fmt.Errorf("덤프 디렉토리 읽기(%s): %w", dir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			pages++
		}
	}

	imageDir := filepath.Join(dir, "images")
	if _, statErr := os.Stat(imageDir); statErr == nil {
		err = filepath.WalkDir(imageDir, func(_ string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !d.IsDir() {
				images++
			}
			return nil
		})
		if err != nil {
			return 0, 0, fmt.Errorf("이미지 디렉토리 읽기(%s): %w", imageDir, err)
		}
	}

	return pages, images, nil
}
