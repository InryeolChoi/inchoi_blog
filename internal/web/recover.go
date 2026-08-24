package web

import (
	"fmt"
	"net/http"
	"runtime/debug"
)

// recovering은 핸들러의 panic이 프로세스를 죽이지 않게 막는다.
//
// **net/http는 이미 panic을 잡는다.** 다만 잡은 뒤에 하는 일이 연결을 그냥
// 끊는 것이라, 브라우저는 빈 화면이나 "연결이 재설정됨"을 본다. 여기서
// 가로채면 무슨 일이 있었는지 로그에 스택으로 남고, 읽는 사람에게는 이
// 사이트의 500 화면이 간다.
//
// **panic은 응답을 쓰는 도중에도 난다.** 템플릿 실행이 본문 절반을 내보낸
// 뒤에 터지면 상태 코드는 이미 200으로 나가 있어서 500으로 바꿀 수 없다.
// 그래서 무엇이든 쓰기 시작했는지를 기록해두고, 안 썼을 때만 500 화면을
// 그린다. 이미 썼으면 로그만 남기고 연결을 끊는다 — 잘린 HTML 뒤에 오류
// 화면을 덧붙이면 더 읽기 힘들다.
func (s *Server) recovering(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tw := &trackedWriter{ResponseWriter: w}
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			// http.ErrAbortHandler은 "조용히 끊어라"라는 약속된 신호다.
			// 우리가 잡아서 로그를 남기면 그 뜻을 어기는 것이라 되던진다.
			if v == http.ErrAbortHandler {
				panic(v)
			}
			fmt.Printf("panic: %s %s: %v\n%s", r.Method, r.URL.Path, v, debug.Stack())
			if tw.wrote {
				return
			}
			s.renderError(tw, r, http.StatusInternalServerError)
		}()
		next.ServeHTTP(tw, r)
	})
}

// trackedWriter는 응답을 이미 내보내기 시작했는지 기억한다.
type trackedWriter struct {
	http.ResponseWriter
	wrote bool
}

func (w *trackedWriter) WriteHeader(code int) {
	w.wrote = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *trackedWriter) Write(b []byte) (int, error) {
	w.wrote = true
	return w.ResponseWriter.Write(b)
}
