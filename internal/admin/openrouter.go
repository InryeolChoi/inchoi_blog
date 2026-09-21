package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// draftSystemPrompt는 글감을 초안으로 정리할 때 쓰는 지시문이다.
//
// **첫 줄을 "# 제목"으로 받는다.** GitHub 이관(prepareBody)이 파일 첫머리의
// 제목을 떼어 posts.title에 넣는 것과 같은 관례다 — 본문 안에 <h1>을 또
// 두면 페이지 제목과 두 번 나온다.
const draftSystemPrompt = `당신은 이 블로그 필자가 프로젝트를 하며 남긴 글감(메모)을 받아
공개할 수 있는 한국어 기술 블로그 글의 초안으로 정리하는 도우미다.

규칙:
- 메모에 없는 사실이나 수치를 지어내지 않는다. 메모에 없는 것은 쓰지 않는다.
- 결과의 첫 줄은 "# 제목" 형태로 글 제목만 담는다. 그 뒤로 빈 줄 하나, 그다음부터 본문이다.
- 본문은 마크다운으로 쓴다. 코드는 코드 펜스 안에, 수식은 $...$ 또는 $$...$$로 쓴다.
- 문체는 담백하고 직접적으로. 과장하거나 광고 문구처럼 쓰지 않는다.
- 이건 초안이다. 사람이 마지막으로 다시 읽고 고친다는 것을 전제로, 메모의 뜻을 지어내지 않고 옮기는 데 집중한다.`

// OpenRouterConfig는 OpenRouter 호출에 필요한 설정이다.
//
// **APIKey가 비어 있으면 글감함은 여전히 쓸 수 있다.** 메모를 쓰고 모으는
// 것과 AI로 정리하는 것은 다른 기능이라, 키가 없다고 글감함 자체를 막을
// 이유가 없다 — "초안 생성" 버튼만 실패한다.
type OpenRouterConfig struct {
	APIKey string
	Model  string
}

// defaultOpenRouterModel은 키만 있고 모델을 안 정했을 때 쓴다.
const defaultOpenRouterModel = "anthropic/claude-sonnet-4.5"

// **badInput이 아니라 errors.New다.** notes.go의 writeGenerateErr가 이걸
// 503으로 구별해서 돌려줘야 하는데, badInput이면 errors.As가 먼저 걸려
// 400(사람이 고칠 수 있는 입력 오류)으로 잘못 분류된다.
var errOpenRouterNotConfigured = errors.New("OpenRouter가 설정되지 않았다. 서버에 BLOG_OPENROUTER_API_KEY를 설정해라")

type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterRequest struct {
	Model    string              `json:"model"`
	Messages []openRouterMessage `json:"messages"`
}

type openRouterResponse struct {
	Choices []struct {
		Message openRouterMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// draftHTTPTimeout은 이 요청 하나의 상한이다. 글 하나를 통째로 쓰게 시키므로
// 여유를 둔다. cmd/blog의 서버 WriteTimeout(60초)보다 짧게 잡아, 서버가
// 연결을 끊기 전에 여기서 먼저 사람이 읽을 수 있는 오류로 끝나게 한다.
const draftHTTPTimeout = 50 * time.Second

// generateDraftBody는 글감 본문을 OpenRouter에 보내 (제목, 본문) 초안을 받는다.
func generateDraftBody(ctx context.Context, cfg OpenRouterConfig, noteTitle, noteBody string) (title, body string, err error) {
	if cfg.APIKey == "" {
		return "", "", errOpenRouterNotConfigured
	}
	model := cfg.Model
	if model == "" {
		model = defaultOpenRouterModel
	}

	reqBody, err := json.Marshal(openRouterRequest{
		Model: model,
		Messages: []openRouterMessage{
			{Role: "system", Content: draftSystemPrompt},
			{Role: "user", Content: "글감 제목: " + noteTitle + "\n\n글감 본문:\n" + noteBody},
		},
	})
	if err != nil {
		return "", "", err
	}

	ctx, cancel := context.WithTimeout(ctx, draftHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	// OpenRouter가 대시보드에 요청 출처를 표시하는 데 쓴다. 필수는 아니다.
	req.Header.Set("HTTP-Referer", "https://inquieto.dev")
	req.Header.Set("X-Title", "blog 글감함")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("OpenRouter 호출 실패: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", "", fmt.Errorf("OpenRouter 응답을 읽지 못했다: %w", err)
	}

	var out openRouterResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return "", "", fmt.Errorf("OpenRouter 응답을 파싱하지 못했다 (HTTP %d)", resp.StatusCode)
	}
	if out.Error != nil {
		return "", "", fmt.Errorf("OpenRouter 오류: %s", out.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("OpenRouter가 HTTP %d를 돌려줬다", resp.StatusCode)
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", "", errors.New("OpenRouter가 빈 응답을 돌려줬다")
	}

	title, body = splitDraftTitle(out.Choices[0].Message.Content)
	return title, body, nil
}

// splitDraftTitle은 모델이 낸 첫 줄("# 제목")을 title로, 나머지를 body로 가른다.
//
// **첫 줄이 그 형태가 아니어도 실패하지 않는다.** 모델이 지시를 어길 수
// 있으므로, 그때는 통째로 본문으로 두고 제목은 글감 제목으로 대신 채운다
// (호출부가 그렇게 한다) — 초안 생성이 형식 하나 때문에 통째로 막히면 안 된다.
func splitDraftTitle(text string) (title, body string) {
	text = strings.TrimLeft(text, "\ufeff \n\r\t")
	lines := strings.SplitN(text, "\n", 2)
	first := strings.TrimSpace(lines[0])
	if strings.HasPrefix(first, "#") {
		title = strings.TrimSpace(strings.TrimLeft(first, "#"))
		rest := ""
		if len(lines) > 1 {
			rest = lines[1]
		}
		return title, strings.TrimLeft(rest, "\n")
	}
	return "", strings.TrimSpace(text)
}
