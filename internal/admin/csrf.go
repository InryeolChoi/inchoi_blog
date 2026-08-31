package admin

import (
	"log"
	"net/http"
	"net/url"
	"strings"
)

// sameOrigin은 남의 사이트에서 온 쓰기 요청을 막는다.
//
// # 왜 세션 쿠키만으로 부족한가
//
// 세션 쿠키는 SameSite=Lax라 남의 페이지에서 보낸 POST에는 안 붙는다. 그래서
// 지금도 실제로는 막힌다 — 다만 **그건 브라우저가 막아주는 것**이다. 우리는
// 그 판단을 남에게 맡기고 있고, 옛 브라우저 하나나 이상한 클라이언트 하나가
// 그 약속을 안 지키면 막을 것이 아무것도 없다. 여기가 서버 자신의 눈이다.
//
// # 무엇을 보나
//
// Origin 헤더 하나다. **브라우저는 POST·PUT·DELETE에 Origin을 반드시 붙이고,
// 그 값은 스크립트가 바꿀 수 없다.** 우리 호스트와 다르면 남의 페이지에서
// 온 것이다.
//
//	Origin이 있고 우리 것    → 통과
//	Origin이 있고 남의 것    → 403
//	Origin이 없음            → 403 (아래 참고)
//
// **Origin이 없으면 막는다.** 브라우저가 아닌 것(curl 등)에는 Origin이 없는데,
// 그런 요청을 통과시키면 검사 전체가 "헤더를 빼면 그만"이 된다. 스크립트로
// 부를 일이 생기면 그때는 세션이 아니라 토큰을 쓸 자리다.
//
// # 읽기는 안 본다
//
// GET/HEAD는 상태를 안 바꾼다. 남의 페이지가 우리 GET을 부를 수는 있지만
// 응답을 읽지는 못한다(그건 CORS가 막는다).
func sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if safeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		if origin == "" {
			deny(w, r, "Origin 헤더가 없다")
			return
		}
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" {
			deny(w, r, "Origin을 읽지 못했다")
			return
		}
		// r.Host는 프록시를 거쳐도 원래 이름 그대로다(Caddy가 그대로 넘긴다).
		// 포트까지 포함해 견준다 — 같은 이름의 다른 포트는 다른 출처다.
		if !strings.EqualFold(u.Host, r.Host) {
			deny(w, r, "다른 출처에서 왔다")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func safeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

// deny는 거절한다. **거절 사유에 남이 보낸 값을 되찍지 않는다** — 남이 지은
// 글자가 우리 화면이나 로그 한 줄로 그대로 나가는 자리를 만들지 않는다
// (거절당한 GitHub 계정 이름을 화면에 안 찍는 것과 같은 이유다).
func deny(w http.ResponseWriter, r *http.Request, why string) {
	log.Printf("admin CSRF 거절: %s %s (%s)", r.Method, r.URL.Path, why)
	// API는 JSON, 폼(로그아웃)은 글자 한 줄이다. fetch()가 받는 자리에
	// HTML이 오면 파싱에서 터지고 진짜 이유가 가려진다.
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeErr(w, http.StatusForbidden, "다른 사이트에서 온 요청은 받지 않는다")
		return
	}
	http.Error(w, "다른 사이트에서 온 요청은 받지 않는다", http.StatusForbidden)
}
