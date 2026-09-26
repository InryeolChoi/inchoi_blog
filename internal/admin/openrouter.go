package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// defaultDraftPrompt는 글감을 초안으로 정리할 때 쓰는 지시문의 **기본값**이다.
//
// **첫 줄을 "# 제목"으로 받는다.** GitHub 이관(prepareBody)이 파일 첫머리의
// 제목을 떼어 posts.title에 넣는 것과 같은 관례다 — 본문 안에 <h1>을 또
// 두면 페이지 제목과 두 번 나온다. 이 관례는 프롬프트를 고쳐도 지켜야 한다
// (안 지키면 splitDraftTitle이 제목을 못 찾고 글감 제목으로 대신 채운다).
//
// **사람이 admin 환경설정에서 고친다**(ai.go). 여기 있는 것은 아무도 안 고쳤을
// 때 쓰는 문장이고, 홈 문구와 같은 이유로 DB에 미리 넣지 않는다 — 행이 없다는
// 것이 곧 "아무도 안 정했다"라서 그동안은 코드에서 다듬을 수 있다.
const defaultDraftPrompt = `당신은 이 블로그 필자가 프로젝트를 하며 남긴 글감(메모)을 받아
공개할 수 있는 한국어 기술 블로그 글의 초안으로 정리하는 도우미다.

# 가장 중요한 규칙

- 메모에 없는 사실, 수치, 코드, 인용을 지어내지 않는다. 메모에 없는 것은 쓰지 않는다.
- 메모가 얇으면 얇은 글을 쓴다. 분량을 채우려고 일반론을 덧붙이지 않는다.
- 아는 것과 모르는 것을 뭉개지 않는다. 메모에서 확실하지 않은 것은 확실하지 않다고 쓴다.

# 형식

- 결과의 첫 줄은 "# 제목" 형태로 글 제목만 담는다. 그 뒤로 빈 줄 하나, 그다음부터 본문이다.
- 본문의 절 제목은 "##"부터 쓴다. 본문 안에 "#"(h1)을 다시 쓰지 않는다.
- 코드는 언어 이름을 붙인 코드 펜스 안에 쓴다(예: ` + "```go" + `).
- 수식은 $...$(문장 안) 또는 $$...$$(따로 선 줄)로 쓴다.
- 표와 목록은 마크다운 문법을 쓴다.
- **HTML 태그와 <script>를 쓰지 않는다.** 애니메이션이 필요하면 지어내지 말고 글로 설명한다.

# 문체

- 평서형 "~다"로 끝낸다. "~습니다", "~해요"를 쓰지 않는다.
- 담백하고 직접적으로 쓴다. 과장하거나 광고 문구처럼 쓰지 않는다.
  "놀랍게도", "정말 중요한", "~에 대해 알아보자" 같은 말을 쓰지 않는다.
- **무엇을 했는지보다 왜 그렇게 했는지를 남긴다.** 고른 이유, 버린 대안, 막혔던 지점이
  이 블로그가 남기려는 것이다. 절차만 나열한 글은 다시 읽을 값이 없다.
- 강조는 굵게(**)로 한 문단에 많아야 하나만 쓴다. 문장 전체를 굵게 하지 않는다.
- 첫 문단에서 이 글이 무엇을 다루는지 바로 말한다. "들어가며" 같은 빈 도입부를 두지 않는다.

# 끝으로

이건 초안이다. 사람이 마지막으로 다시 읽고 고친다는 것을 전제로, 메모의 뜻을
지어내지 않고 옮기는 데 집중한다. 애매한 자리는 매끄럽게 덮지 말고 그대로 두어,
고칠 곳이 어디인지 사람이 보게 한다.`

// OpenRouterConfig는 OpenRouter 호출에 필요한 설정이다.
//
// **APIKey가 비어 있으면 글감함은 여전히 쓸 수 있다.** 메모를 쓰고 모으는
// 것과 AI로 정리하는 것은 다른 기능이라, 키가 없다고 글감함 자체를 막을
// 이유가 없다 — "초안 생성" 버튼만 실패한다.
type OpenRouterConfig struct {
	APIKey string
	Model  string
}

// defaultOpenRouterModel은 **아무도 모델을 안 고른 상태**에서 쓰는 값이다.
// 사람이 admin 환경설정에서 고르면 그쪽이 이긴다(ai.go의 aiConfig).
//
// 한국어 글의 결이 가장 안정적이라 고른 값이다. 초안 한 편에 $0.05 안팎이라
// 가장 싼 축은 아니다 — 싸게 돌리고 싶으면 화면에서 google/gemini-2.5-flash
// 같은 것으로 바꾼다. 여기 값을 고치려고 배포할 이유는 없다.
const defaultOpenRouterModel = "anthropic/claude-sonnet-5"

// **badInput이 아니라 errors.New다.** notes.go의 writeGenerateErr가 이걸
// 503으로 구별해서 돌려줘야 하는데, badInput이면 errors.As가 먼저 걸려
// 400(사람이 고칠 수 있는 입력 오류)으로 잘못 분류된다.
var errOpenRouterNotConfigured = errors.New("OpenRouter가 설정되지 않았다. 서버에 OPENROUTER_API_KEY를 설정해라")
var errOpenRouterTimeout = errors.New("OpenRouter 초안 생성이 5분 안에 끝나지 않았다. 잠시 후 다시 시도하거나 더 빠른 모델을 골라라")

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

// 글 한 편을 생성하는 동안 모델이 토큰을 내보내는 시간이 길 수 있다.
// 핸들러의 쓰기 기한보다 먼저 끝나야 시간 초과 오류를 브라우저에 보낼 수 있다.
const draftHTTPTimeout = 5 * time.Minute
const draftWriteTimeout = draftHTTPTimeout + 15*time.Second

// generateDraftBody는 글감 본문을 OpenRouter에 보내 (제목, 본문) 초안을 받는다.
//
// model과 prompt는 **이미 정해져서 들어온다**(store.aiConfig). 여기서 기본값을
// 또 고르지 않는다 — 두 군데서 고르면 화면이 보여주는 모델과 실제로 부른 모델이
// 갈라진다.
func generateDraftBody(ctx context.Context, cfg OpenRouterConfig, model, prompt, noteTitle, noteBody string) (title, body string, err error) {
	if cfg.APIKey == "" {
		return "", "", errOpenRouterNotConfigured
	}

	reqBody, err := json.Marshal(openRouterRequest{
		Model: model,
		Messages: []openRouterMessage{
			{Role: "system", Content: prompt},
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
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", "", fmt.Errorf("%w: %v", errOpenRouterTimeout, err)
		}
		return "", "", fmt.Errorf("OpenRouter 호출 실패: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", "", fmt.Errorf("%w: %v", errOpenRouterTimeout, err)
		}
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

// ------------------------------------------------- 잔액과 모델 목록 조회
//
// 둘 다 **읽기만 한다.** 글감함에서 초안을 만들기 전에 "지금 무슨 모델을
// 쓰고, 돈이 얼마나 남았나"를 화면에서 보려고 있다 — 그걸 모르면 버튼을
// 누르기 전까지 알 수 없고, 잔액이 떨어졌을 때 오류 메시지만 보게 된다.

// lookupTimeout은 조회 요청의 상한이다. 초안 생성과 달리 사람이 화면을
// 띄워 놓고 기다리는 자리라 짧게 둔다 — 느리면 "못 가져왔다"가 낫다.
const lookupTimeout = 10 * time.Second

// openRouterGET은 OpenRouter의 읽기 API 하나를 불러 out에 담는다.
func openRouterGET(ctx context.Context, cfg OpenRouterConfig, path string, out any) error {
	if cfg.APIKey == "" {
		return errOpenRouterNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openrouter.ai/api/v1"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("OpenRouter 호출 실패: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("OpenRouter 응답을 읽지 못했다: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OpenRouter가 HTTP %d를 돌려줬다", resp.StatusCode)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("OpenRouter 응답을 파싱하지 못했다: %w", err)
	}
	return nil
}

// Credits는 이 키에 남은 금액이다.
//
// **Remaining을 서버에서 계산해 내려준다.** 화면에서 빼면 두 숫자의 뜻을
// 화면도 알아야 하고, 나중에 OpenRouter가 칸을 바꿀 때 고칠 자리가 둘이 된다.
type Credits struct {
	Total     float64 `json:"total"`
	Used      float64 `json:"used"`
	Remaining float64 `json:"remaining"`
}

func fetchCredits(ctx context.Context, cfg OpenRouterConfig) (*Credits, error) {
	var out struct {
		Data struct {
			TotalCredits float64 `json:"total_credits"`
			TotalUsage   float64 `json:"total_usage"`
		} `json:"data"`
	}
	if err := openRouterGET(ctx, cfg, "/credits", &out); err != nil {
		return nil, err
	}
	return &Credits{
		Total:     out.Data.TotalCredits,
		Used:      out.Data.TotalUsage,
		Remaining: out.Data.TotalCredits - out.Data.TotalUsage,
	}, nil
}

// ModelInfo는 고를 수 있는 모델 하나다. 화면의 자동완성 목록이 쓴다.
//
// **값은 전부 글자다.** 가격은 OpenRouter가 "0.000003" 같은 글자로 주는데,
// 숫자로 바꿔 두면 0이 "공짜"인지 "안 알려줬다"인지 구별이 사라진다. 여기서는
// 보여주기만 하므로 받은 그대로 넘긴다.
type ModelInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Prompt  string `json:"prompt"`
	Output  string `json:"output"`
	Context int    `json:"context"`
}

func fetchModels(ctx context.Context, cfg OpenRouterConfig) ([]ModelInfo, error) {
	var out struct {
		Data []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			ContextLength int    `json:"context_length"`
			Pricing       struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
		} `json:"data"`
	}
	if err := openRouterGET(ctx, cfg, "/models", &out); err != nil {
		return nil, err
	}
	models := make([]ModelInfo, 0, len(out.Data))
	for _, m := range out.Data {
		models = append(models, ModelInfo{
			ID: m.ID, Name: m.Name, Context: m.ContextLength,
			Prompt: m.Pricing.Prompt, Output: m.Pricing.Completion,
		})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models, nil
}
