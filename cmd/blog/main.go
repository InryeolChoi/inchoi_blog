// blog는 블로그 서버다. 시작할 때 스키마 마이그레이션을 적용하고 HTTP를 연다.
//
// 지금은 읽기 전용 공개 페이지만 있다. 인증과 접근 제어는 아직 없지만
// **draft 글은 어디에도 안 보이고 /p/{slug}도 404다.**
// 로컬에서 draft까지 확인하려면 `-drafts`를 준다.
package main

import (
	"database/sql"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/inryeol/blog"
	"github.com/inryeol/blog/internal/admin"
	"github.com/inryeol/blog/internal/db"
	"github.com/inryeol/blog/internal/web"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP 리스닝 주소")
	dbPath := flag.String("db", "blog.db", "SQLite 파일 경로")
	// 기본값은 "가린다"다. 켜고 끄는 것을 실수해도 새는 방향이 아니라
	// 막는 방향으로 틀리게 둔다.
	drafts := flag.Bool("drafts", false, "draft 글까지 보여준다 (로컬 확인용)")
	// **기본값이 "안 띄운다"인 이유는 인증이 아직 없기 때문이다.** 로드맵 2단계가
	// 끝나기 전까지 이 화면은 아무나 글을 고칠 수 있는 화면이고, 그래서 배포
	// 유닛(deploy/blog.service)은 이 플래그를 주지 않는다. -drafts와 같은 원칙이다 —
	// 켜고 끄는 것을 실수해도 새는 방향이 아니라 막는 방향으로 틀리게 둔다.
	adminOn := flag.Bool("admin", false, "admin 화면을 연다 (인증 없음 — 로컬 전용)")
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

	handler := srv.Handler()
	if *adminOn {
		if handler, err = withAdmin(handler, sqlDB); err != nil {
			log.Fatal(err)
		}
		log.Printf("admin: http://%s/admin — **인증이 없다. 이 주소를 밖에 열지 마라.**", *addr)
	}

	log.Printf("http://%s 에서 대기 중 (db: %s, draft %s, admin %s)",
		*addr, *dbPath,
		map[bool]string{true: "보임", false: "가림"}[*drafts],
		map[bool]string{true: "열림", false: "닫힘"}[*adminOn])
	if err := httpServer(*addr, handler).ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// withAdmin은 공개 핸들러 앞에 admin을 붙인다.
//
// 바깥 mux를 하나 더 두는 이유: 공개 mux에는 "GET /"라는 catch-all이 있어서
// 어디에도 안 걸린 경로를 404 화면으로 받는다. 같은 mux에 admin을 넣으면 두
// 패키지가 라우팅을 나눠 갖게 되므로, 여기서 접두사로만 가른다.
// ServeMux는 더 구체적인 패턴을 먼저 고르므로 "/admin/"이 "/"를 이긴다.
func withAdmin(public http.Handler, sqlDB *sql.DB) (http.Handler, error) {
	adm, err := admin.New(sqlDB)
	if err != nil {
		return nil, err
	}
	root := http.NewServeMux()
	root.Handle("/admin", adm.Handler())
	root.Handle("/admin/", adm.Handler())
	root.Handle("/api/admin/", adm.Handler())
	root.Handle("/", public)
	return root, nil
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
