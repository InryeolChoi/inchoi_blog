// blog는 블로그 서버다. 시작할 때 스키마 마이그레이션을 적용하고 HTTP를 연다.
//
// 지금은 읽기 전용 공개 페이지만 있다. 인증과 접근 제어는 아직 없지만
// **draft 글은 어디에도 안 보이고 /p/{slug}도 404다.**
// 로컬에서 draft까지 확인하려면 `-drafts`를 준다.
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
	// 기본값은 "가린다"다. 켜고 끄는 것을 실수해도 새는 방향이 아니라
	// 막는 방향으로 틀리게 둔다.
	drafts := flag.Bool("drafts", false, "draft 글까지 보여준다 (로컬 확인용)")
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

	var opts []web.Option
	if *drafts {
		opts = append(opts, web.WithDrafts())
	}
	srv, err := web.New(sqlDB, opts...)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("http://%s 에서 대기 중 (db: %s, draft %s)",
		*addr, *dbPath, map[bool]string{true: "보임", false: "가림"}[*drafts])
	if err := http.ListenAndServe(*addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
