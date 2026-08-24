package web

import (
	"fmt"
	"net/http"
	"strings"
)

// errorInfo는 404·500 화면에 찍을 것이다.
//
// 글자를 두 벌 들고 있는 이유: 서버는 한국어를 그리고, 브라우저에서
// preferences.js의 고정 사전이 `data-i18n` 키를 보고 세 언어로 바꾼다.
// 본문 번역(Translator API)에 맡기지 않는 이유는 이게 공통 UI이기 때문이다 —
// 사이드바나 푸터와 같은 대접을 받아야 한다.
type errorInfo struct {
	Code     int
	TitleKey string
	Title    string
	LeadKey  string
	Lead     string
}

var errorInfos = map[int]errorInfo{
	http.StatusNotFound: {
		Code:     http.StatusNotFound,
		TitleKey: "notFoundTitle", Title: "여기에는 아무것도 없습니다",
		LeadKey: "notFoundLead", Lead: "주소가 바뀌었거나, 아직 공개하지 않은 글일 수 있습니다.",
	},
	http.StatusInternalServerError: {
		Code:     http.StatusInternalServerError,
		TitleKey: "serverErrorTitle", Title: "서버가 이 요청을 끝내지 못했습니다",
		LeadKey: "serverErrorLead", Lead: "잠시 뒤에 다시 열어보세요. 문제는 기록에 남았습니다.",
	},
}

// notFound는 404 화면을 그린다. **페이지 라우트에서만 쓴다** —
// /img/와 /static/은 그림과 스크립트라 HTML을 돌려줄 자리가 아니다.
func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	s.renderError(w, r, http.StatusNotFound)
}

// fail은 처리 중에 난 오류를 로그에 남기고 500 화면을 그린다.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	fmt.Printf("요청 처리 실패: %s %s: %v\n", r.Method, r.URL.Path, err)
	s.renderError(w, r, http.StatusInternalServerError)
}

// renderError는 오류 화면을 그린다.
//
// **render를 거치지 않는다.** render는 실패하면 fail을 부르고 fail은 다시
// 여기로 오므로, 그 길로 가면 사이드바 조회가 한 번 삐끗할 때 무한히 돈다.
// 여기서는 사이드바를 "되면 좋은 것"으로 다룬다 — 못 가져오면 없는 채로
// 그린다. 오류 화면만은 DB가 어떻든 나가야 한다.
//
// 템플릿마저 실패하면 마지막으로 글자만 내보낸다.
func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int) {
	info, ok := errorInfos[status]
	if !ok {
		info = errorInfos[http.StatusInternalServerError]
		info.Code = status
	}

	data := pageData{Title: fmt.Sprintf("%d · 열렬히.뛰기", info.Code), Err: &info}
	if nav, err := s.store.NavTree(); err == nil {
		markNav(nav, nil, 0)
		data.Nav = nav
		for _, c := range nav {
			data.TotalPosts += c.PostCount
		}
	} else {
		fmt.Printf("오류 화면의 사이드바 조회 실패: %v\n", err)
	}

	// 상태 코드를 먼저 정하고, 본문은 다 그린 뒤에 한 번에 내보낸다.
	// 템플릿이 도중에 실패해도 반쯤 그린 화면이 나가지 않는다.
	var buf strings.Builder
	t, ok := s.pages["error.html"]
	if ok {
		if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
			fmt.Printf("오류 화면 렌더링 실패: %v\n", err)
			ok = false
		}
	}
	if !ok {
		http.Error(w, fmt.Sprintf("%d %s", info.Code, http.StatusText(info.Code)), info.Code)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// 오류 화면은 캐시하지 않는다. 글 하나를 공개로 바꿨는데 중간 캐시가
	// 404를 들고 있으면 그 글은 계속 없는 것이 된다.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(info.Code)
	if _, err := w.Write([]byte(buf.String())); err != nil {
		fmt.Printf("오류 화면 쓰기 실패: %v\n", err)
	}
}
