// blog는 블로그 서버다. 시작할 때 스키마 마이그레이션을 적용하고 HTTP를 연다.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/inryeol/blog"
	"github.com/inryeol/blog/internal/db"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP 리스닝 주소")
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

	// TODO: html/template + embed.FS로 실제 페이지를 렌더링한다. 지금은 연결 확인용.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		var published, total int
		err := sqlDB.QueryRowContext(r.Context(),
			`SELECT count(*) FILTER (WHERE status = 'published'), count(*) FROM posts`,
		).Scan(&published, &total)
		if err != nil {
			log.Printf("글 수 조회: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "blog: 발행 %d건 / 전체 %d건\n", published, total)
	})

	log.Printf("%s 에서 대기 중 (db: %s)", *addr, *dbPath)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
