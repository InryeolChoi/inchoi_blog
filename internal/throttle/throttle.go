// Package throttle은 한 주소에서 쏟아지는 요청 수를 제한한다.
//
// # 이것이 막는 것과 못 막는 것
//
// **진짜 DDoS는 여기서 못 막는다.** 회선을 채우는 공격은 Go 코드가 한 줄
// 돌기 전에 1 vCPU짜리 기계의 네트워크를 먼저 채운다. 그건 앞단(Caddy)이나
// 그 앞(클라우드 방어)이 할 일이고, 여기서 막는 척하면 안 하는 것보다 나쁘다.
//
// 여기가 실제로 막는 것은 이런 것들이다:
//
//	긁어가는 봇 하나가 초당 수백 번 두드려 1GB짜리 기계를 재우는 것
//	admin 로그인·API를 반복해서 두드리는 것
//	실수로 무한 루프를 도는 스크립트 하나
//
// # 왜 표준 라이브러리로 쓰나
//
// golang.org/x/time/rate가 이 일을 하지만, 그걸 쓰면 **주소별 보관과 청소는
// 어차피 우리가 써야 한다** — 그게 이 파일 분량의 대부분이다. 토큰 통 자체는
// 뺄셈 한 줄이라 의존성을 늘릴 값이 없다.
package throttle

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Limit은 한 주소에 허용할 속도다.
type Limit struct {
	// Rate는 초당 채워지는 토큰 수, 즉 오래 지속될 때의 허용 속도다.
	Rate float64
	// Burst는 통의 크기다. 한 번에 몰아 쓸 수 있는 양이라, 한 페이지가
	// 자산 여러 개를 동시에 부르는 것을 여기서 받아준다.
	Burst float64
}

// bucket은 주소 하나의 토큰 통이다.
type bucket struct {
	tokens float64
	seen   time.Time
}

// Limiter는 주소별 토큰 통을 들고 있다. 여러 고루틴에서 같이 써도 된다.
type Limiter struct {
	limit Limit
	// max는 기억할 주소 수의 상한이다.
	//
	// **이게 없으면 이 방어가 곧 공격 수단이 된다.** 출발지 주소를 바꿔가며
	// 두드리면 맵이 무한히 자라서, 요청을 막으려던 것이 1GB짜리 기계의
	// 메모리를 먹는다.
	max int

	mu      sync.Mutex
	buckets map[string]*bucket
	now     func() time.Time // 테스트가 시간을 쥐고 흔들 수 있게
}

// New는 Limiter를 만든다.
func New(limit Limit, max int) *Limiter {
	return &Limiter{
		limit:   limit,
		max:     max,
		buckets: make(map[string]*bucket),
		now:     time.Now,
	}
}

// Allow는 이 주소가 지금 한 번 더 요청해도 되는지 본다.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		// 새 주소를 넣기 전에 자리를 만든다.
		if len(l.buckets) >= l.max {
			l.evict(now)
		}
		b = &bucket{tokens: l.limit.Burst}
		l.buckets[key] = b
	} else {
		// 지난 시간만큼 채운다. 통보다 많이 담기지는 않는다.
		b.tokens += now.Sub(b.seen).Seconds() * l.limit.Rate
		if b.tokens > l.limit.Burst {
			b.tokens = l.limit.Burst
		}
	}
	b.seen = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// evict는 자리를 만든다.
//
// **가득 찼다고 요청을 막지 않는다.** 그러면 주소를 바꿔가며 두드리는 것만으로
// 멀쩡한 사람까지 막히는, 스스로 무너지는 방어가 된다. 대신 통이 다시 가득 찬
// (= 한동안 안 온) 주소를 버린다. 그것들은 기억해봐야 어차피 통과시킬 것들이다.
func (l *Limiter) evict(now time.Time) {
	full := l.limit.Burst
	for k, b := range l.buckets {
		if b.tokens+now.Sub(b.seen).Seconds()*l.limit.Rate >= full {
			delete(l.buckets, k)
		}
	}
	// 그래도 안 줄었으면 — 전부가 지금 두드리는 중이라는 뜻이다 — 절반을
	// 아무렇게나 버린다. 맵이 자라는 것을 막는 것이 먼저다.
	if len(l.buckets) >= l.max {
		n := len(l.buckets) / 2
		for k := range l.buckets {
			if n == 0 {
				break
			}
			delete(l.buckets, k)
			n--
		}
	}
}

// Len은 지금 기억하고 있는 주소 수다. 테스트와 진단에 쓴다.
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}

// ClientIP는 요청을 보낸 쪽의 주소를 고른다.
//
// # X-Forwarded-For를 믿는 근거
//
// **이 값은 누구나 지어낼 수 있는 헤더다.** 그런데도 믿는 이유는 이 배포에서
// blog가 127.0.0.1:8080에만 붙기 때문이다 — 밖에서 이 포트에 직접 닿을 수
// 없고, 들어오는 길은 Caddy 하나뿐이다. Caddy는 자기가 본 주소를 XFF **맨
// 뒤에** 붙인다.
//
// 그래서 **맨 뒤 값을 쓴다.** 맨 앞을 쓰면 공격자가 지어낸 값이 그대로 키가
// 되어, 요청마다 다른 주소인 척하며 제한을 통째로 피해간다.
//
// **blog를 밖에 직접 노출하는 순간 이 전제가 깨진다.** 그때는 이 함수도 같이
// 고쳐야 한다 — 소켓이 loopback에 붙어 있는 것이 이 코드의 안전 근거다.
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		last := xff
		for i := len(xff) - 1; i >= 0; i-- {
			if xff[i] == ',' {
				last = xff[i+1:]
				break
			}
		}
		if ip := net.ParseIP(trimSpace(last)); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func trimSpace(s string) string {
	i, j := 0, len(s)
	for i < j && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return s[i:j]
}

// Middleware는 제한을 걸어 next로 넘긴다.
//
// 넘치면 **429와 Retry-After**를 준다. 오류 화면을 그리지 않는 이유는,
// 넘치는 상황에서 67KB짜리 페이지를 만들어 내보내는 것이 곧 공격을 돕는
// 일이기 때문이다. 자산 라우트가 오류 화면 대신 글자 한 줄을 쓰는 것과 같다.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	retry := strconv.Itoa(int(1/max(l.limit.Rate, 0.001)) + 1)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow(ClientIP(r)) {
			w.Header().Set("Retry-After", retry)
			// 중간 캐시가 이 응답을 들고 있으면 안 된다. 제한이 풀린 뒤에도
			// 그 사람에게 429가 계속 나간다.
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "요청이 너무 잦다. 잠시 뒤에 다시.", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
