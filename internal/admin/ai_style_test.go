package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeOpenRouter는 OpenRouter 호출을 가로채 보낸 요청을 모아 둔다.
func fakeOpenRouter(t *testing.T, reply string) *[]openRouterRequest {
	t.Helper()
	var sent []openRouterRequest
	old := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var req openRouterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		sent = append(sent, req)
		b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{
			"message": map[string]string{"role": "assistant", "content": reply}}}})
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(b))),
			Header: make(http.Header)}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = old })
	return &sent
}

func addNativePost(t *testing.T, s *Server, slug, body string) int64 {
	t.Helper()
	now := time.Now().UTC()
	res, err := s.store.db.Exec(`INSERT INTO posts (slug, title, body, status, source, created_at, updated_at)
		VALUES (?, ?, ?, 'draft', 'native', ?, ?)`, slug, "제목 "+slug, body, now, now)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

// postLive는 실제 서버로 POST한다. 응답 기한을 늘리는 핸들러는
// httptest.ResponseRecorder에서 SetWriteDeadline을 못 쓴다.
func postLive(t *testing.T, h http.Handler, path string) (int, []byte) {
	t.Helper()
	ts := httptest.NewServer(h)
	defer ts.Close()
	// 기본 클라이언트는 OpenRouter 가짜로 바뀌어 있으므로 따로 쓴다.
	req, err := http.NewRequest(http.MethodPost, ts.URL+path, strings.NewReader("{}"))
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
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func TestWithStyle(t *testing.T) {
	if got := withStyle("지시문", ""); got != "지시문" {
		t.Errorf("가이드가 없으면 지시문 그대로여야 한다: %q", got)
	}
	got := withStyle("지시문", "짧게 쓴다.")
	if !strings.HasPrefix(got, "지시문\n\n# 문체 가이드") || !strings.HasSuffix(got, "짧게 쓴다.") ||
		!strings.Contains(got, "형식 규칙을 따른다") {
		t.Errorf("이어 붙인 결과: %q", got)
	}
}

// 문체의 근거는 **직접 쓴 긴 글뿐**이다. AI 초안, 이관 글, 짧은 글은 뺀다.
func TestStyleSourcesExcludeAIDraftsMigratedAndShort(t *testing.T) {
	s, err := New(testDB(t), nil, OpenRouterConfig{})
	if err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("직접 쓴 문장이다. ", 40)
	addNativePost(t, s, "mine", long)
	addNativePost(t, s, "tiny", "짧다.")
	aiID := addNativePost(t, s, "from-ai", long)
	if _, err := s.store.db.Exec(`INSERT INTO notes (title, body, status, generated_post_id)
		VALUES ('글감', '메모', 'generated', ?)`, aiID); err != nil {
		t.Fatal(err)
	}
	// testDB의 노션 글(live-post)은 source가 notion이라 들어오면 안 된다.

	got, err := s.store.styleSources()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Slug != "mine" {
		t.Fatalf("근거 글 = %+v, 원한 것 mine 하나", got)
	}
}

func TestProposeStyleDoesNotSave(t *testing.T) {
	s, err := New(testDB(t), nil, OpenRouterConfig{APIKey: testKey, Model: "test/model"})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()

	// 근거 글이 없으면 외부 호출 전에 막는다.
	if code, _ := postLive(t, h, "/api/admin/ai/style/propose"); code != http.StatusConflict {
		t.Fatalf("근거 글 없음 = %d, 원한 것 409", code)
	}

	addNativePost(t, s, "mine", strings.Repeat("나는 이렇게 판단했다. ", 40))
	sent := fakeOpenRouter(t, "  - \"~다\"로 끝낸다.  ")
	code, body := postLive(t, h, "/api/admin/ai/style/propose")
	if code != http.StatusOK {
		t.Fatalf("제안 = %d: %s", code, body)
	}
	if len(*sent) != 1 || (*sent)[0].Messages[0].Content != styleProposePrompt ||
		!strings.Contains((*sent)[0].Messages[1].Content, "나는 이렇게 판단했다.") {
		t.Fatalf("OpenRouter 요청이 이상하다: %+v", *sent)
	}
	var got struct {
		Proposal string
		Sources  []styleSource
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Proposal != `- "~다"로 끝낸다.` || len(got.Sources) != 1 {
		t.Errorf("제안 응답: %+v", got)
	}
	if strings.Contains(string(body), testKey) {
		t.Fatal("제안 응답에 키가 들어 있다")
	}
	// **제안만 한다.** 수락 전에는 저장되지 않는다.
	if style, _ := s.store.aiStyle(); style != "" {
		t.Fatalf("제안을 받기만 했는데 저장됐다: %q", style)
	}
}

func TestSaveStyleAndDraftUsesIt(t *testing.T) {
	s, err := New(testDB(t), nil, OpenRouterConfig{APIKey: testKey, Model: "test/model"})
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()

	tooLong := strings.Repeat("가", aiStyleMax+1)
	if rec := do(t, h, "PUT", "/api/admin/ai/style", `{"style":"`+tooLong+`"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("너무 긴 가이드 = %d, 원한 것 400", rec.Code)
	}
	rec := do(t, h, "PUT", "/api/admin/ai/style", `{"style":" 짧게 쓴다. "}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("저장 = %d: %s", rec.Code, rec.Body)
	}
	var got struct{ Style string }
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.Style != "짧게 쓴다." {
		t.Fatalf("저장 응답: %s", rec.Body)
	}

	// 초안 생성 호출의 시스템 프롬프트 뒤에 가이드가 붙는다.
	res, err := s.store.db.Exec(`INSERT INTO notes (title, body) VALUES ('글감', '메모 본문')`)
	if err != nil {
		t.Fatal(err)
	}
	noteID, _ := res.LastInsertId()
	sent := fakeOpenRouter(t, "# 초안\n\n본문이다.")
	if _, err := s.store.generateNoteDraft(context.Background(), s.openRouter, noteID, time.Now(), nil); err != nil {
		t.Fatal(err)
	}
	system := (*sent)[0].Messages[0].Content
	if !strings.HasPrefix(system, defaultDraftPrompt) || !strings.HasSuffix(system, "짧게 쓴다.") {
		t.Errorf("시스템 프롬프트에 가이드가 안 붙었다: ...%q", system[max(0, len(system)-80):])
	}

	// 비우면 지운다 — 다음 초안은 지시문만 쓴다.
	if rec := do(t, h, "PUT", "/api/admin/ai/style", `{"style":""}`); rec.Code != http.StatusOK {
		t.Fatalf("비우기 = %d", rec.Code)
	}
	if style, _ := s.store.aiStyle(); style != "" {
		t.Errorf("비웠는데 남았다: %q", style)
	}
}
