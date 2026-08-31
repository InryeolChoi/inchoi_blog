package throttle

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// 시계를 손에 쥔 Limiter. 실제 시간을 기다리는 테스트는 느리고 가끔 틀린다.
func testLimiter(limit Limit, max int) (*Limiter, *time.Time) {
	l := New(limit, max)
	now := time.Unix(1000, 0)
	l.now = func() time.Time { return now }
	return l, &now
}

func TestBurstThenRefill(t *testing.T) {
	l, now := testLimiter(Limit{Rate: 1, Burst: 3}, 100)

	// 통 크기만큼은 몰아 쓸 수 있다 — 페이지 하나가 자산 여러 개를 동시에 부른다.
	for i := 0; i < 3; i++ {
		if !l.Allow("a") {
			t.Fatalf("%d번째가 막혔다. burst만큼은 통과해야 한다", i+1)
		}
	}
	if l.Allow("a") {
		t.Fatal("통이 비었는데 통과했다")
	}

	// 1초 지나면 토큰 하나가 찬다.
	*now = now.Add(time.Second)
	if !l.Allow("a") {
		t.Fatal("1초 뒤에도 막혔다")
	}
	if l.Allow("a") {
		t.Fatal("토큰 하나만 찼는데 둘이 통과했다")
	}
}

// 주소끼리 통을 나눠 쓰면 안 된다. 한 사람이 넘쳤다고 다른 사람이 막히면
// 그게 곧 남을 막는 공격 수단이 된다.
func TestAddressesDoNotShareABucket(t *testing.T) {
	l, _ := testLimiter(Limit{Rate: 1, Burst: 1}, 100)
	if !l.Allow("a") {
		t.Fatal("a의 첫 요청이 막혔다")
	}
	if l.Allow("a") {
		t.Fatal("a의 둘째 요청이 통과했다")
	}
	if !l.Allow("b") {
		t.Fatal("a가 넘쳤다고 b까지 막혔다")
	}
}

// **주소를 바꿔가며 두드려도 메모리가 무한히 자라지 않는다.** 이 상한이
// 없으면 제한 장치 자체가 메모리를 먹는 공격 수단이 된다.
func TestMemoryIsBounded(t *testing.T) {
	l, _ := testLimiter(Limit{Rate: 1, Burst: 1}, 50)
	for i := 0; i < 500; i++ {
		l.Allow(string(rune('가' + i)))
	}
	if got := l.Len(); got > 50 {
		t.Errorf("주소 %d개를 기억하고 있다. 50개가 상한이다", got)
	}
}

// 가득 찼을 때 **버리는 것은 요청이 아니라 기억이다.** 오래 안 온(통이 다시
// 가득 찬) 주소를 버리고 새 주소는 받아준다.
func TestEvictionDropsIdleNotNew(t *testing.T) {
	l, now := testLimiter(Limit{Rate: 1, Burst: 2}, 3)
	l.Allow("옛1")
	l.Allow("옛2")
	l.Allow("옛3")
	// 셋 다 통이 다시 가득 찰 만큼 쉰다.
	*now = now.Add(10 * time.Second)
	if !l.Allow("새") {
		t.Fatal("맵이 가득 찼다고 새 주소를 막았다")
	}
}

func TestMiddlewareGives429WithRetryAfter(t *testing.T) {
	l, _ := testLimiter(Limit{Rate: 0.5, Burst: 1}, 100)
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("첫 요청이 %d다", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("넘친 요청이 %d다. 429여야 한다", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After가 없다. 언제 다시 와야 하는지 말해줘야 한다")
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("429가 캐시되면 제한이 풀린 뒤에도 계속 막힌 것처럼 보인다")
	}
}

// **XFF는 맨 뒤를 쓴다.** 맨 앞은 보낸 쪽이 지어낼 수 있는 값이라, 그걸 키로
// 쓰면 요청마다 다른 주소인 척하며 제한을 통째로 피해간다. 맨 뒤는 Caddy가
// 자기 눈으로 보고 붙인 값이다.
func TestClientIPUsesTheLastXFFHop(t *testing.T) {
	for _, tc := range []struct {
		name, xff, remote, want string
	}{
		{"헤더 없음", "", "127.0.0.1:9999", "127.0.0.1"},
		{"한 칸", "203.0.113.9", "127.0.0.1:9999", "203.0.113.9"},
		{"지어낸 값 + 진짜", "1.2.3.4, 203.0.113.9", "127.0.0.1:9999", "203.0.113.9"},
		{"쓰레기 값", "not-an-ip", "127.0.0.1:9999", "127.0.0.1"},
		{"IPv6", "2001:db8::1", "127.0.0.1:9999", "2001:db8::1"},
	} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = tc.remote
		if tc.xff != "" {
			r.Header.Set("X-Forwarded-For", tc.xff)
		}
		if got := ClientIP(r); got != tc.want {
			t.Errorf("%s: ClientIP = %q, want %q", tc.name, got, tc.want)
		}
	}
}
