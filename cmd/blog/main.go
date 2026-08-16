// blog는 블로그 서버다. 시작할 때 스키마 마이그레이션을 적용하고 HTTP를 연다.
//
// 지금은 읽기 전용 공개 페이지만 있다. 인증도 접근 제어도 없고
// status(draft/unlisted/published)를 가리지 않는다. 로컬 확인용이다.
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/inryeol/blog"
	"github.com/inryeol/blog/internal/db"
	"github.com/inryeol/blog/internal/web"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP 리스닝 주소")
	dbPath := flag.String("db", "blog.db", "SQLite 파일 경로")
	flag.Parse()

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

	srv, err := web.New(sqlDB)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("http://%s 에서 대기 중 (db: %s)", *addr, *dbPath)
	if err := http.ListenAndServe(*addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
