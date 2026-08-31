package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// write는 Origin을 마음대로 정해서 쓰기 요청을 보낸다. do()는 늘 우리 출처를
// 붙이므로(브라우저 흉내) 여기서는 따로 만든다.
func write(t *testing.T, h http.Handler, method, path, origin, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// **남의 사이트에서 온 쓰기는 막힌다.**
//
// 세션 쿠키가 SameSite=Lax라 브라우저가 이미 한 겹 막아주지만, 그건 남에게
// 맡긴 판단이다. 서버가 자기 눈으로도 봐야 한다.
func TestWritesFromAnotherSiteAreRefused(t *testing.T) {
	h := testHandler(t)

	for _, tc := range []struct {
		name   string
		slug   string
		origin string
		want   int
	}{
		// httptest.NewRequest의 Host는 example.com이다.
		{"우리 출처", "우리-출처", "http://example.com", http.StatusCreated},
		{"남의 사이트", "남의-사이트", "http://evil.example", http.StatusForbidden},
		// 같은 이름의 다른 포트는 다른 출처다.
		{"같은 이름 다른 포트", "다른-포트", "http://example.com:8080", http.StatusForbidden},
		// **Origin이 없으면 막는다.** 통과시키면 검사 전체가 "헤더를 빼면 그만"이 된다.
		{"Origin 없음", "origin-없음", "", http.StatusForbidden},
		{"읽을 수 없는 Origin", "못-읽는-origin", "not-a-url", http.StatusForbidden},
	} {
		payload := mustJSON(t, saveReq{Slug: tc.slug, Title: tc.name, Body: "x", Status: "draft"})
		rec := write(t, h, http.MethodPost, "/api/admin/posts", tc.origin, payload)
		if rec.Code != tc.want {
			t.Errorf("%s: 상태 코드 %d, %d여야 한다 (%s)",
				tc.name, rec.Code, tc.want, strings.TrimSpace(rec.Body.String()))
		}
		// **상태 코드만 보고 넘어가지 않는다.** "막았다"고 답하면서 쓰는
		// 경우를 그러면 못 잡는다. 슬러그를 하나씩 따로 두어 앞 사례가
		// 만든 글이 다음 검사를 속이지 않게 한다.
		got := do(t, h, http.MethodGet, "/api/admin/posts/"+tc.slug, "")
		created := got.Code == http.StatusOK
		if created != (tc.want == http.StatusCreated) {
			t.Errorf("%s: 글이 생겼나=%v (상태 코드는 %d였다)", tc.name, created, rec.Code)
		}
	}
}

// 읽기는 안 본다. GET은 상태를 안 바꾸고, 남의 페이지가 응답을 읽는 것은
// CORS가 막는다. 여기서 GET까지 막으면 로그인 화면조차 못 연다.
func TestReadsAreNotBlockedByOrigin(t *testing.T) {
	h := testHandler(t)
	rec := write(t, h, http.MethodGet, "/api/admin/posts", "http://evil.example", "")
	if rec.Code != http.StatusOK {
		t.Errorf("남의 출처에서 온 GET이 %d다. 읽기는 통과해야 한다", rec.Code)
	}
}

// API에는 HTML이 아니라 JSON으로 거절한다. fetch()가 받는 자리라 HTML이
// 오면 파싱에서 터지고 진짜 이유가 가려진다.
func TestCSRFRefusalIsJSONForAPIs(t *testing.T) {
	h := testHandler(t)
	rec := write(t, h, http.MethodPost, "/api/admin/preview", "http://evil.example", "{}")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("상태 코드 %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type이 %q다. JSON이어야 한다", ct)
	}
	var got map[string]string
	decode(t, rec, &got)
	if got["error"] == "" {
		t.Error("error 칸이 비었다")
	}
}
