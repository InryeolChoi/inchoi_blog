// blog는 블로그 서버다. 시작할 때 스키마 마이그레이션을 적용하고 HTTP를 연다.
//
// 지금은 읽기 전용 공개 페이지만 있다. 인증과 접근 제어는 아직 없지만
// **draft 글은 어디에도 안 보이고 /p/{slug}도 404다.**
// 로컬에서 draft까지 확인하려면 `-drafts`를 준다.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
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
	// **기본값은 "안 띄운다"다.** 이제 인증이 붙었지만(로드맵 2단계) 그렇다고
	// 늘 열어둘 이유는 없다 — 공개 서버가 하는 일은 읽기고, 안 여는 것이
	// 공격 면적이 제일 작다. -drafts와 같은 원칙이다: 켜고 끄는 것을 실수해도
	// 새는 방향이 아니라 막는 방향으로 틀리게 둔다.
	adminOn := flag.Bool("admin", false, "admin 화면을 연다 (GitHub 로그인 설정이 있어야 한다)")
	// **위험한 것은 이름이 위험해야 한다.** 인증 없이 admin을 여는 길을 없애면
	// 로컬에서 화면을 못 보고, 조용한 기본값으로 두면 언젠가 배포에 딸려간다.
	// 그래서 길게 적게 만들고, 아래에서 loopback이 아니면 아예 안 뜨게 한다.
	noAuth := flag.Bool("admin-no-auth", false, "admin을 인증 없이 연다 (loopback 전용)")
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

	// **admin을 먼저 만든다.** 공개 화면의 "고치기"가 admin에게 "지금 누가
	// 들어와 있나"를 물어야 하기 때문이다. -admin이 없으면 그 질문 자체가
	// 없고, 그러면 글 화면은 예전 그대로다 — 버튼도 스크립트도 안 나간다.
	var adm *admin.Server
	adminState := "닫힘"
	var auth *admin.AuthConfig
	if *adminOn {
		var err error
		if auth, err = adminAuth(*addr, *noAuth); err != nil {
			log.Fatalf("admin: %v", err)
		}
		if adm, err = admin.New(sqlDB, auth); err != nil {
			log.Fatal(err)
		}
		opts = append(opts, web.WithEditor(adm.LoginFor))
	}

	srv, err := web.New(sqlDB, opts...)
	if err != nil {
		log.Fatal(err)
	}

	handler := srv.Handler()
	if adm != nil {
		handler = withAdmin(handler, adm)
		if auth == nil {
			adminState = "열림 (인증 없음)"
			log.Printf("admin: http://%s/admin — **인증이 없다. 이 주소를 밖에 열지 마라.**", *addr)
		} else {
			adminState = "열림"
			log.Printf("admin: http://%s/admin — GitHub 로그인 (허용: %s)",
				*addr, strings.Join(auth.AllowedLogins, ", "))
		}
	}

	log.Printf("http://%s 에서 대기 중 (db: %s, draft %s, admin %s)",
		*addr, *dbPath,
		map[bool]string{true: "보임", false: "가림"}[*drafts],
		adminState)
	if err := httpServer(*addr, handler).ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// admin 로그인 설정은 환경변수에서 읽는다. **플래그로 받지 않는다** — client
// secret이 플래그면 `ps`에 그대로 보이고 셸 히스토리에도 남는다.
const (
	envClientID     = "BLOG_GITHUB_CLIENT_ID"
	envClientSecret = "BLOG_GITHUB_CLIENT_SECRET"
	envLogins       = "BLOG_ADMIN_LOGINS"
	envSessionKey   = "BLOG_SESSION_KEY"
)

// sessionKeyMinLen은 세션 서명 키의 최소 길이다. 짧은 키는 HMAC을 무르게
// 만들어서, 서명을 맞춰내면 아무 계정으로나 세션을 지어낼 수 있다.
const sessionKeyMinLen = 32

// adminAuth는 admin에 붙일 인증 설정을 만든다.
//
// **이 함수의 요점은 "애매하면 안 뜬다"이다.** 인증이 반쯤 설정된 채로 서버가
// 뜨는 것이 여기서 제일 나쁜 결과라, 모자라면 nil이 아니라 error를 낸다.
// 인증 없이 여는 길은 -admin-no-auth 하나뿐이고 그것도 loopback에서만 된다.
func adminAuth(addr string, noAuth bool) (*admin.AuthConfig, error) {
	id := os.Getenv(envClientID)
	secret := os.Getenv(envClientSecret)
	logins := os.Getenv(envLogins)
	configured := id != "" || secret != "" || logins != ""

	if noAuth {
		// **설정이 있는데 -admin-no-auth를 주면 거절한다.** 둘 다 주는 것은
		// 사람이 무엇을 원하는지 알 수 없는 상태고, 조용히 인증을 끄는 쪽으로
		// 고르면 그게 사고다.
		if configured {
			return nil, fmt.Errorf("-admin-no-auth와 %s 설정을 같이 줬다. 하나만 골라라", envClientID)
		}
		if !isLoopback(addr) {
			return nil, fmt.Errorf("-admin-no-auth는 loopback에서만 된다 (지금 -addr %q). "+
				"밖에 열려면 %s / %s / %s를 설정해라", addr, envClientID, envClientSecret, envLogins)
		}
		return nil, nil
	}

	var missing []string
	for _, e := range []struct{ name, val string }{
		{envClientID, id}, {envClientSecret, secret}, {envLogins, logins},
	} {
		if strings.TrimSpace(e.val) == "" {
			missing = append(missing, e.name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("-admin에는 GitHub 로그인 설정이 필요하다. 없는 것: %s. "+
			"로컬에서 화면만 볼 것이면 -admin-no-auth를 써라 (loopback 전용)",
			strings.Join(missing, ", "))
	}

	cfg := &admin.AuthConfig{
		ClientID:      id,
		ClientSecret:  secret,
		AllowedLogins: strings.Split(logins, ","),
	}
	if k := os.Getenv(envSessionKey); k != "" {
		if len(k) < sessionKeyMinLen {
			return nil, fmt.Errorf("%s가 너무 짧다 (%d바이트). %d바이트 이상으로 해라",
				envSessionKey, len(k), sessionKeyMinLen)
		}
		cfg.SessionKey = []byte(k)
	}
	return cfg, nil
}

// isLoopback은 이 주소가 이 기계 밖에서 닿을 수 없는 곳인지 본다.
//
// **호스트가 비어 있으면(":8080") loopback이 아니다.** 그건 모든 인터페이스에
// 붙는다는 뜻이라 밖에서 그대로 들어온다. 여기서 틀리면 인증 없는 글쓰기
// 화면이 공개 주소에 열린다.
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// 포트가 없는 형태면 통째로 호스트로 본다.
		host = addr
	}
	if host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

// withAdmin은 공개 핸들러 앞에 admin을 붙인다.
//
// 바깥 mux를 하나 더 두는 이유: 공개 mux에는 "GET /"라는 catch-all이 있어서
// 어디에도 안 걸린 경로를 404 화면으로 받는다. 같은 mux에 admin을 넣으면 두
// 패키지가 라우팅을 나눠 갖게 되므로, 여기서 접두사로만 가른다.
// ServeMux는 더 구체적인 패턴을 먼저 고르므로 "/admin/"이 "/"를 이긴다.
func withAdmin(public http.Handler, adm *admin.Server) http.Handler {
	root := http.NewServeMux()
	root.Handle("/admin", adm.Handler())
	root.Handle("/admin/", adm.Handler())
	root.Handle("/api/admin/", adm.Handler())
	root.Handle("/", public)
	return root
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
