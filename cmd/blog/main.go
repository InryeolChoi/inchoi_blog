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
	"time"

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
	if err := httpServer(*addr, srv.Handler()).ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// httpServer는 타임아웃을 건 서버를 만든다.
//
// **http.ListenAndServe의 기본값은 "무제한"이다.** 연결을 열어놓고 요청을
// 끝내지 않는 클라이언트가 고루틴과 파일 디스크립터를 계속 물고 있어서,
// 공개 주소에 그대로 두면 느린 연결 몇 개로 서버가 멎는다. 인증이 없는
// 읽기 전용 서버라 더 그렇다.
//
// 값은 이 서버가 실제로 하는 일에서 나왔다:
//   - 헤더 5초, 본문 15초 — GET뿐이라 받을 것이 사실상 헤더밖에 없다.
//   - 쓰기 60초 — 가장 큰 응답이 3.4MB짜리 이미지 BLOB이다. 60초면
//     57KB/s에서도 끝난다. 짧게 잡으면 느린 회선에서 그림이 잘린다.
//   - 유휴 60초 — keep-alive 연결을 그보다 오래 붙들지 않는다.
func httpServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
		// 기본값(1MB)보다 좁힌다. 이 서버의 정상 요청에는 쿠키도 인증
		// 헤더도 없어서 몇 KB를 넘길 이유가 없다.
		MaxHeaderBytes: 64 << 10,
	}
}
