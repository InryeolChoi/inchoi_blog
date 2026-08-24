package main

import (
	"net/http"
	"testing"
)

// 타임아웃이 0이면 "무제한"이다. 공개 주소에서 무제한은 느린 연결 몇 개로
// 서버가 멎는다는 뜻이라, 실수로 지워지지 않게 여기서 지킨다.
func TestHTTPServerHasEveryTimeout(t *testing.T) {
	srv := httpServer("127.0.0.1:0", http.NotFoundHandler())

	checks := []struct {
		name string
		zero bool
	}{
		{"ReadHeaderTimeout", srv.ReadHeaderTimeout == 0},
		{"ReadTimeout", srv.ReadTimeout == 0},
		{"WriteTimeout", srv.WriteTimeout == 0},
		{"IdleTimeout", srv.IdleTimeout == 0},
	}
	for _, c := range checks {
		if c.zero {
			t.Errorf("%s가 0이다 — 무제한이라는 뜻이다", c.name)
		}
	}
	if srv.MaxHeaderBytes == 0 {
		t.Error("MaxHeaderBytes가 기본값이다")
	}
	// 가장 큰 응답이 3.4MB짜리 이미지 BLOB이다. 쓰기 타임아웃을 읽기보다
	// 짧게 잡으면 느린 회선에서 그림이 잘린다.
	if srv.WriteTimeout <= srv.ReadTimeout {
		t.Errorf("WriteTimeout(%v)이 ReadTimeout(%v)보다 짧다", srv.WriteTimeout, srv.ReadTimeout)
	}
	if srv.ReadHeaderTimeout > srv.ReadTimeout {
		t.Errorf("ReadHeaderTimeout(%v)이 ReadTimeout(%v)보다 길다", srv.ReadHeaderTimeout, srv.ReadTimeout)
	}
}
