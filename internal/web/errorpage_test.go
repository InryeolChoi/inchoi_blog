package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

// 없는 길도 이 사이트의 화면으로 알려준다. 상태 코드는 그대로 404여야 한다 —
// 화면이 예뻐졌다고 200을 주면 검색엔진과 링크 검사기에게 거짓말이 된다.
func TestNotFoundRendersSiteErrorPage(t *testing.T) {
	h := testServer(t)
	for _, path := range []string{
		"/p/없는-글",
		"/없는-분류",
		"/dev/language/python/너무/깊은/경로",
	} {
		rec := get(t, h, path)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: 상태 코드 %d, 404여야 한다", path, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `class="errpage"`) {
			t.Errorf("%s: 오류 화면이 아니라 기본 404다:\n%s", path, body)
		}
		// 사이드바가 함께 나와야 다른 길로 갈 수 있다. 막다른 화면이면
		// 뒤로 가기 말고는 방법이 없다.
		if !strings.Contains(body, `id="prefs-shell"`) || !strings.Contains(sideOf(t, body), `href="/dev"`) {
			t.Errorf("%s: 오류 화면에 사이트 껍데기가 없다", path)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s: Cache-Control=%q, 오류 화면은 캐시하지 않는다", path, got)
		}
	}
}

// draft는 없는 글과 똑같이 다뤄야 한다. 오류 화면이 "숨긴 글"과 "없는 글"을
// 다르게 말하면 그 차이 자체가 draft가 있다는 신호가 된다.
func TestHiddenPostGetsTheSameNotFoundPage(t *testing.T) {
	h := testServer(t)
	hidden := get(t, h, "/p/draft-post")
	missing := get(t, h, "/p/아예-없는-글")
	if hidden.Code != http.StatusNotFound {
		t.Fatalf("draft 상태 코드 %d", hidden.Code)
	}
	if mainOf(t, hidden.Body.String()) != mainOf(t, missing.Body.String()) {
		t.Error("숨긴 글과 없는 글의 화면이 다르다")
	}
}

// 그림과 스크립트 라우트는 HTML을 돌려줄 자리가 아니다. <img>가 오류 화면을
// 받으면 깨진 그림 아이콘 하나로 끝나고, 6KB짜리 페이지만 헛되이 나간다.
func TestAssetRoutesStayPlainNotFound(t *testing.T) {
	h := testServer(t)
	for _, path := range []string{
		"/static/없는파일.js",
		"/img/" + strings.Repeat("cd", 32),
	} {
		rec := get(t, h, path)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: 상태 코드 %d", path, rec.Code)
		}
		if strings.Contains(rec.Body.String(), `class="errpage"`) {
			t.Errorf("%s: 자산 라우트가 HTML 오류 화면을 돌려줬다", path)
		}
	}
}

// 오류 화면의 글자는 공통 UI라 고정 사전이 세 언어로 바꾼다. 키는 코드에
// 있고 화면에는 `data-i18n` 값으로만 나가서, 이름을 바꿔도 조용히 한국어로
// 남는다. 그래서 여기서 지킨다.
func TestErrorPageTextIsInEveryDictionary(t *testing.T) {
	js, err := os.ReadFile("static/preferences.js")
	if err != nil {
		t.Fatal(err)
	}
	langs := regexp.MustCompile(`(?m)^    (ko|en|es): \{`).FindAllStringIndex(string(js), -1)
	if len(langs) != 3 {
		t.Fatalf("사전이 3개가 아니다: %d", len(langs))
	}
	var keys []string
	for _, info := range errorInfos {
		keys = append(keys, info.TitleKey, info.LeadKey)
	}
	keys = append(keys, "backHome")
	for i, loc := range langs {
		end := len(js)
		if i+1 < len(langs) {
			end = langs[i+1][0]
		}
		block := string(js[loc[0]:end])
		for _, key := range keys {
			if !strings.Contains(block, key+":") {
				t.Errorf("사전 %s에 %q가 없다", strings.TrimSpace(block[:8]), key)
			}
		}
	}
}

// net/http도 panic을 잡지만, 잡은 뒤에 하는 일이 연결을 그냥 끊는 것이라
// 브라우저는 빈 화면을 본다. 여기서 가로채 500 화면을 돌려준다.
func TestPanicBecomesErrorPage(t *testing.T) {
	srv, err := New(seedTestDB(t))
	if err != nil {
		t.Fatal(err)
	}
	h := srv.recovering(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("일부러 터뜨린다")
	}))

	rec := get(t, h, "/p/list-post")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("상태 코드 %d, 500이어야 한다", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `class="errpage"`) {
		t.Errorf("panic 뒤에 오류 화면이 안 나왔다:\n%s", rec.Body.String())
	}
}

// panic은 응답을 쓰는 도중에도 난다. 그때는 상태 코드가 이미 나가 있어서
// 500으로 바꿀 수 없다 — 잘린 본문 뒤에 오류 화면을 덧붙이지 않고 끊는다.
func TestPanicMidResponseDoesNotAppendErrorPage(t *testing.T) {
	srv, err := New(seedTestDB(t))
	if err != nil {
		t.Fatal(err)
	}
	h := srv.recovering(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("절반쯤 그린 화면")); err != nil {
			t.Error(err)
		}
		panic("쓰다가 터졌다")
	}))

	rec := get(t, h, "/p/list-post")
	if rec.Code != http.StatusOK {
		t.Errorf("이미 보낸 상태 코드가 %d로 바뀌었다", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `class="errpage"`) {
		t.Error("잘린 본문 뒤에 오류 화면이 덧붙었다")
	}
}

// http.ErrAbortHandler은 "조용히 끊어라"라는 약속된 신호다. 우리가 삼키면
// 그 뜻이 사라지고 로그만 지저분해진다.
func TestAbortHandlerPanicIsNotSwallowed(t *testing.T) {
	srv, err := New(seedTestDB(t))
	if err != nil {
		t.Fatal(err)
	}
	h := srv.recovering(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	defer func() {
		if v := recover(); v != http.ErrAbortHandler {
			t.Errorf("되던지지 않았다: %v", v)
		}
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	t.Error("panic이 여기까지 오지 않았다")
}

// 오류 화면만은 DB가 어떻든 나가야 한다. 사이드바 조회가 실패해도 화면 없이
// 끝내지 않고 사이드바만 뺀 채 그린다.
func TestErrorPageSurvivesADeadDatabase(t *testing.T) {
	sqlDB := seedTestDB(t)
	srv, err := New(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	h := srv.Handler()
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	// DB가 닫혔으니 조회부터 실패한다 — 500이 맞다. 확인하려는 것은
	// 상태 코드가 아니라 **사이드바 조회까지 실패한 상황에서도 화면이
	// 나온다**는 것이다.
	rec := get(t, h, "/없는-분류")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("상태 코드 %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `class="errpage"`) {
		t.Errorf("DB가 닫힌 뒤에 오류 화면이 안 나왔다:\n%s", rec.Body.String())
	}
}
