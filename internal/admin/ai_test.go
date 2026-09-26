package admin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type waitForCancelBody struct{ ctx context.Context }

func (b waitForCancelBody) Read([]byte) (int, error) {
	<-b.ctx.Done()
	return 0, b.ctx.Err()
}
func (waitForCancelBody) Close() error { return nil }

// OpenRouter가 헤더만 보낸 뒤 생성이 멈춰도, 응답 읽기가 취소되고
// 브라우저에는 원인을 알 수 있는 504가 돌아가야 한다.
func TestDraftResponseTimeout(t *testing.T) {
	oldClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: waitForCancelBody{r.Context()}, Header: make(http.Header)}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = oldClient })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, _, err := generateDraftBody(ctx, OpenRouterConfig{APIKey: testKey}, "test/model", "prompt", "title", "body")
	if !errors.Is(err, errOpenRouterTimeout) {
		t.Fatalf("응답 읽기 시간 초과 = %v", err)
	}
	rec := do(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeGenerateErr(w, r, err)
	}), http.MethodPost, "/api/admin/notes/1/generate", "")
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("시간 초과 응답 = %d, 본문 %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "5분 안에 끝나지 않았다") {
		t.Fatalf("시간 초과 안내 = %s", rec.Body.String())
	}
}

func TestRevisePostReturnsProposalWithoutSaving(t *testing.T) {
	db := testDB(t)
	server, err := New(db, nil, OpenRouterConfig{APIKey: testKey, Model: "test/model"})
	if err != nil {
		t.Fatal(err)
	}
	post, err := server.store.postBySlug("live-post")
	if err != nil {
		t.Fatal(err)
	}

	oldClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var sent openRouterRequest
		if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
			t.Error(err)
		}
		if sent.Model != "test/model" || !strings.Contains(sent.Messages[1].Content, "반복을 줄여줘") {
			t.Errorf("OpenRouter 요청: %+v", sent)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(
			`{"choices":[{"message":{"role":"assistant","content":"# 다듬은 제목\n\n다듬은 본문이다."}}]}`)),
			Header: make(http.Header)}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = oldClient })

	// rev가 오래됐으면 외부 호출 전에 거절한다.
	stale := do(t, server.Handler(), http.MethodPost, "/api/admin/posts/live-post/ai-revise",
		`{"rev":"old","title":"보이는 글","body":"본문","instruction":"반복을 줄여줘"}`)
	if stale.Code != http.StatusConflict {
		t.Fatalf("오래된 rev = %d", stale.Code)
	}

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()
	payload, _ := json.Marshal(reviseReq{Rev: post.Rev, Title: post.Title, Body: post.Body,
		Instruction: "반복을 줄여줘"})
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/admin/posts/live-post/ai-revise", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", ts.URL)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Transport: http.DefaultTransport}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("AI 수정 = %d: %s", resp.StatusCode, body)
	}
	var proposal struct{ Title, Body string }
	if err := json.NewDecoder(resp.Body).Decode(&proposal); err != nil {
		t.Fatal(err)
	}
	if proposal.Title != "다듬은 제목" || proposal.Body != "다듬은 본문이다." {
		t.Errorf("수정안: %+v", proposal)
	}
	unchanged, err := server.store.postBySlug("live-post")
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Title != post.Title || unchanged.Body != post.Body || unchanged.Rev != post.Rev {
		t.Fatal("AI 제안을 요청하기만 했는데 원본 글이 바뀌었다")
	}
}

// aiHandler는 키가 있는 서버를 만든다. **키 값 자체가 응답에 새는지**를 보는
// 시험이 있어서, 픽스처에 알아보기 쉬운 값을 넣는다.
const testKey = "sk-or-test-DO-NOT-LEAK"

func aiServer(t *testing.T, cfg OpenRouterConfig) http.Handler {
	t.Helper()
	s, err := New(testDB(t), nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s.Handler()
}

// **키는 어떤 응답에도 실리지 않는다.** 화면은 "있다/없다"만 알면 되고,
// 값이 한 번이라도 JSON에 들어가면 브라우저 메모리와 캐시, 그리고 보는 사람의
// 개발자 도구에 그대로 남는다.
func TestAISettingsNeverLeaksKey(t *testing.T) {
	h := aiServer(t, OpenRouterConfig{APIKey: testKey, Model: "x/y"})

	rec := do(t, h, "GET", "/api/admin/ai", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/admin/ai = %d, 본문 %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), testKey) {
		t.Fatalf("응답에 키가 들어 있다: %s", rec.Body)
	}

	var got struct {
		Configured bool   `json:"configured"`
		EnvModel   string `json:"envModel"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Configured {
		t.Error("키가 있는데 configured가 false다")
	}
	if got.EnvModel != "x/y" {
		t.Errorf("envModel = %q, 원한 것 x/y", got.EnvModel)
	}
}

// 키가 없을 때도 설정 화면은 열린다. **글감함 자체가 막히면 안 된다** —
// 메모를 쌓는 것과 AI로 정리하는 것은 다른 기능이다.
func TestAISettingsWithoutKey(t *testing.T) {
	h := aiServer(t, OpenRouterConfig{})

	rec := do(t, h, "GET", "/api/admin/ai", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/admin/ai = %d", rec.Code)
	}
	var got struct {
		Configured bool `json:"configured"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Configured {
		t.Error("키가 없는데 configured가 true다")
	}

	// 잔액과 모델 목록은 503이다. **502가 아니다** — 사람이 고칠 수 있는
	// 설정 문제와 남의 서비스 장애를 화면이 구별해서 안내해야 한다.
	for _, path := range []string{"/api/admin/ai/credits", "/api/admin/ai/models"} {
		rec := do(t, h, "GET", path, "")
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("GET %s = %d, 원한 것 503", path, rec.Code)
		}
	}
}

// 무엇이 실제로 쓰일지 고르는 순서: 저장된 값 → 서버 환경변수 → 코드 기본값.
func TestAIConfigPrecedence(t *testing.T) {
	db := testDB(t)
	st := &store{db: db}

	model, prompt, err := st.aiConfig(OpenRouterConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if model != defaultOpenRouterModel {
		t.Errorf("아무것도 없을 때 model = %q, 원한 것 %q", model, defaultOpenRouterModel)
	}
	if prompt != defaultDraftPrompt {
		t.Error("아무것도 없을 때 프롬프트가 기본값이 아니다")
	}

	// 환경변수가 코드 기본값을 이긴다.
	model, _, err = st.aiConfig(OpenRouterConfig{Model: "env/model"})
	if err != nil {
		t.Fatal(err)
	}
	if model != "env/model" {
		t.Errorf("환경변수가 있을 때 model = %q", model)
	}

	// 저장된 값이 환경변수를 이긴다.
	if err := st.saveAISettings(map[string]string{
		aiModelKey: "saved/model", aiPromptKey: "저장된 지시문",
	}); err != nil {
		t.Fatal(err)
	}
	model, prompt, err = st.aiConfig(OpenRouterConfig{Model: "env/model"})
	if err != nil {
		t.Fatal(err)
	}
	if model != "saved/model" {
		t.Errorf("저장값이 있을 때 model = %q", model)
	}
	if prompt != "저장된 지시문" {
		t.Errorf("저장값이 있을 때 prompt = %q", prompt)
	}

	// **비우면 지운다.** 행이 없다는 것이 곧 "안 정했다"이고, 그때는 다시
	// 기본값으로 돌아가야 한다 — 빈 문자열이 남으면 빈 지시문으로 부르게 된다.
	if err := st.saveAISettings(map[string]string{aiModelKey: "  ", aiPromptKey: ""}); err != nil {
		t.Fatal(err)
	}
	model, prompt, err = st.aiConfig(OpenRouterConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if model != defaultOpenRouterModel || prompt != defaultDraftPrompt {
		t.Errorf("비운 뒤 기본값으로 안 돌아갔다: model=%q", model)
	}
}

func TestSaveAIValidation(t *testing.T) {
	h := aiServer(t, OpenRouterConfig{APIKey: testKey})

	// 제공자를 빼먹은 모델 이름은 저장 시점에 막는다. 안 막으면 며칠 뒤
	// "초안 생성"이 실패할 때까지 모른다.
	rec := do(t, h, "PUT", "/api/admin/ai", `{"model":"claude-sonnet-4.5","prompt":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("제공자 없는 모델 = %d, 원한 것 400", rec.Code)
	}

	rec = do(t, h, "PUT", "/api/admin/ai", `{"model":"anthropic/claude-sonnet-4.5","prompt":"짧은 지시문"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("정상 저장 = %d, 본문 %s", rec.Code, rec.Body)
	}
	var got struct {
		Model     string            `json:"model"`
		Effective map[string]string `json:"effective"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Model != "anthropic/claude-sonnet-4.5" {
		t.Errorf("저장 뒤 model = %q", got.Model)
	}
	// 저장 응답도 **무엇이 실제로 쓰일지**를 같이 준다. 화면이 그 문장을 적는다.
	if got.Effective["model"] != "anthropic/claude-sonnet-4.5" {
		t.Errorf("effective.model = %q", got.Effective["model"])
	}
	if strings.Contains(rec.Body.String(), testKey) {
		t.Fatalf("저장 응답에 키가 들어 있다: %s", rec.Body)
	}

	// **한글로 잰다.** 바이트로 세던 때는 이 문자열이 상한의 세 배라 통과처럼
	// 보였다 — 글자 수로 재는 지금은 딱 한 자를 넘겨야 걸린다.
	long := strings.Repeat("가", aiPromptMax+1)
	rec = do(t, h, "PUT", "/api/admin/ai", `{"model":"","prompt":"`+long+`"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("너무 긴 프롬프트 = %d, 원한 것 400", rec.Code)
	}
}

// 화면 쪽은 브라우저 없이 확인할 수 있는 것만 못 박는다. **디자인이 아니라
// 계약이다** — 이 화면이 어느 API를 부르고, 좁은 폭에서 무엇을 지키는지.
func TestAIScreenIsWired(t *testing.T) {
	js, err := staticFS.ReadFile("static/admin.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		// 세 가지를 다루는 화면이다: 모델, 지시문, 잔액.
		`/api/admin/ai`,
		`/api/admin/ai/credits`,
		`/api/admin/ai/models`,
		// 기존 컴포넌트를 쓴다. 새 디자인을 만들지 않는다.
		`ad-card`, `ad-stats`, `ad-input`, `ad-homesave`,
		// 모델 목록은 누를 때만 받는다(수백 개다).
		`쓸 수 있는 모델 불러오기`,
	} {
		if !strings.Contains(string(js), want) {
			t.Errorf("admin.js에 %q가 없다", want)
		}
	}
	// **키를 화면이 다루지 않는다.** 입력 칸이 생기면 값이 브라우저를 지나간다.
	//
	// 환경변수 **이름**(OPENROUTER_API_KEY)은 안내 문구에 나온다. 그건 값이
	// 아니라 "어디에 넣으면 되는지"라서 여기서 막을 것이 아니다 — 막으면
	// 사람에게 방법을 안 알려주는 쪽으로 틀리게 된다.
	for _, never := range []string{`type: "password"`, `apiKey`, `"key"`} {
		if strings.Contains(string(js), never) {
			t.Errorf("admin.js가 키 값을 다루고 있다: %q", never)
		}
	}
	if !strings.Contains(string(js), "키 자체는 화면에 나오지 않는다") {
		t.Error("키를 화면이 안 다룬다는 것을 사람에게 알려주는 문장이 없다")
	}

	// 저장이 끝나면 **가운데 창으로 한 번 멈춰 세운다.** 폼이 길어서 줄 끝의
	// 글자는 눈에 안 들어오고, 모바일에서는 키보드에 가린다. alert이 아니라
	// <dialog>여야 Esc와 포커스 복귀가 브라우저 기본 동작으로 따라온다.
	for _, want := range []string{`showModal()`, `"dialog"`, `ad-modal`, `text: "확인"`} {
		if !strings.Contains(string(js), want) {
			t.Errorf("알림 창에 %q가 없다", want)
		}
	}

	css, err := staticFS.ReadFile("static/admin.css")
	if err != nil {
		t.Fatal(err)
	}
	// 손가락으로 쓰는 화면에서 입력 글자가 16px 아래면 iOS가 확대하고 돌아오지
	// 않는다. `.ad-ai-prompt`는 코드용 폰트(.85rem)를 물려받으므로 반드시 있어야 한다.
	if !strings.Contains(string(css), "@media (pointer: coarse)") ||
		!strings.Contains(string(css), ".ad-ai-prompt, .ad-note-title") {
		t.Error("좁은 화면에서 입력 글자를 키우는 규칙이 없다")
	}
	if !strings.Contains(string(css), ".ad-ai-row { display: flex; flex-wrap: wrap;") {
		t.Error("버튼 줄이 좁은 폭에서 줄을 넘기지 않는다")
	}
	// 알림 창이 좁은 화면 밖으로 나가지 않고, 움직임을 줄이라는 설정을 지킨다.
	// `body.admin > *`가 max-width를 none으로 되돌리므로, 알림 창 규칙은 그보다
	// 세게 적어야 한다 — 안 그러면 창이 화면 폭만큼 늘어난다.
	if !strings.Contains(string(css), "body.admin > .ad-modal") ||
		!strings.Contains(string(css), "calc(100% - 2rem)") {
		t.Error("알림 창이 좁은 폭을 지키지 않는다")
	}
	if !strings.Contains(string(css), "@media (prefers-reduced-motion: no-preference)") {
		t.Error("알림 창 애니메이션에 prefers-reduced-motion이 없다")
	}
}
