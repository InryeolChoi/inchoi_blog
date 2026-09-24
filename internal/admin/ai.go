package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/inryeol/blog/internal/web"
)

// 글감함이 쓰는 AI 설정이다. **키는 여기서 다루지 않는다** — 키는 서버의
// 환경변수(OPENROUTER_API_KEY)이고, 화면은 "설정됨/안 됨"만 안다.
//
// # 왜 모델과 프롬프트는 화면에서 고치는가
//
// 둘 다 secret이 아니고, 글을 쓰다 보면 계속 손보게 되는 값이다. 코드에 박아
// 두면 문장 하나 다듬으려고 배포를 해야 하는데, 그건 홈 문구를 settings 표로
// 옮긴 것과 같은 이유로 어긋난다(migrations/008) — 결국 사람이 정할 문장이다.
//
// # 왜 키는 화면에서 못 고치는가
//
// 화면에서 받으면 그 값을 DB에 담아야 하고, blog.db는 로컬로 내려받아 이관
// 작업에 쓰는 파일이다(deploy/fetch-db.sh). 키가 거기 섞이면 백업과 작업본
// 전부가 secret을 들고 다니게 된다. 키는 인스턴스에만 있는 편이 맞다.

// 이 화면이 고칠 수 있는 설정 키다. settings 표를 홈 문구와 나눠 쓰되
// 접두사로 구별한다 — 핸들러가 이 둘만 쓰므로 다른 키는 여기로 못 들어온다.
const (
	aiModelKey  = "ai.model"
	aiPromptKey = "ai.prompt"
)

// 길이 상한. 프롬프트는 문단 몇 개를 적는 자리라 홈 문구(settingsMax=400)보다
// 훨씬 넉넉하되, 무제한은 아니다 — 여기가 본문을 담는 자리로 흘러가면 안 된다.
const (
	aiModelMax  = 200
	aiPromptMax = 8 << 10
)

// aiConfig는 지금 초안 생성에 실제로 쓰일 설정이다.
//
// **고르는 자리는 여기 하나다.** 저장된 값 → 서버 환경변수 → 코드 기본값 순이고,
// 화면도 생성도 이 함수가 돌려준 것을 쓴다. 두 군데서 고르면 화면이 보여주는
// 모델과 실제로 부른 모델이 갈라진다.
func (s *store) aiConfig(cfg OpenRouterConfig) (model, prompt string, err error) {
	saved, err := web.Settings(s.db)
	if err != nil {
		return "", "", err
	}
	model = strings.TrimSpace(saved[aiModelKey])
	if model == "" {
		model = strings.TrimSpace(cfg.Model)
	}
	if model == "" {
		model = defaultOpenRouterModel
	}
	prompt = strings.TrimSpace(saved[aiPromptKey])
	if prompt == "" {
		prompt = defaultDraftPrompt
	}
	return model, prompt, nil
}

// saveAISettings는 보낸 값만 고친다. **빈 값은 지운다** — 행이 없다는 것이 곧
// "기본값을 쓴다"라서, 빈 문자열을 남기면 "비웠다"와 "안 정했다"가 구별되지
// 않는다(settings.go와 같은 규칙).
func (s *store) saveAISettings(values map[string]string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for k, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			if _, err := tx.Exec(`DELETE FROM settings WHERE key = ?`, k); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO settings (key, value, updated_at) VALUES (?, ?, datetime('now'))
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			k, v); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ------------------------------------------------------------- HTTP 핸들러

// handleAI는 지금 설정과 기본값을 함께 준다.
//
// **기본값을 같이 주는 이유**는 화면이 그것을 자리표시자로 쓰기 때문이다 —
// 빈 칸에 회색으로 기본 프롬프트가 보이면 "비우면 이게 쓰인다"가 눈에 보인다.
//
// **잔액은 여기서 안 부른다.** OpenRouter가 느리거나 죽어도 설정 화면은 떠야
// 한다 — 붙여 두면 남의 서비스 하나가 내 설정 화면을 못 열게 만든다.
func (s *Server) handleAI(w http.ResponseWriter, r *http.Request) {
	saved, err := web.Settings(s.store.db)
	if err != nil {
		log.Printf("admin AI 설정 조회 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "설정을 읽지 못했다")
		return
	}
	model, prompt, err := s.store.aiConfig(s.openRouter)
	if err != nil {
		log.Printf("admin AI 설정 조회 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "설정을 읽지 못했다")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		// 저장된 값. 안 고쳤으면 빈 값이고, 화면은 자리표시자를 보여준다.
		"model":  saved[aiModelKey],
		"prompt": saved[aiPromptKey],
		// 실제로 쓰일 값. 저장된 값이 비었을 때 무엇이 쓰이는지 화면이 그대로 적는다.
		"effective": map[string]string{"model": model, "prompt": prompt},
		"defaults": map[string]string{
			"model":  defaultOpenRouterModel,
			"prompt": defaultDraftPrompt,
		},
		// 키가 서버에 있는지. **값은 절대 안 보낸다.**
		"configured": s.openRouter.APIKey != "",
		// 환경변수로 모델을 정해 뒀는지. 화면이 "서버가 정한 값"이라고 적는다.
		"envModel": s.openRouter.Model,
	})
}

type aiReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

func (s *Server) handleSaveAI(w http.ResponseWriter, r *http.Request) {
	// 읽기 상한은 **바이트**다. 한글 한 자가 3바이트라 글자 상한의 세 배에
	// 여유를 더해 둔다 — 여기서 먼저 잘리면 "너무 길다"가 아니라 413이 나와서
	// 사람이 무엇이 문제인지 알 수 없다.
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4*aiPromptMax+1<<16))
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "요청이 너무 크다")
		return
	}
	var req aiReq
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "요청을 읽지 못했다")
		return
	}
	req.Model = strings.TrimSpace(req.Model)
	req.Prompt = strings.TrimSpace(req.Prompt)
	if len([]rune(req.Model)) > aiModelMax {
		writeErr(w, http.StatusBadRequest, "모델 이름이 너무 길다")
		return
	}
	// **모델 이름의 생김새도 본다.** OpenRouter의 id는 "제공자/모델" 꼴이라,
	// 사람이 모델 이름만 적어 넣는 흔한 실수를 저장 시점에 잡는다 — 안 그러면
	// 며칠 뒤 "초안 생성"이 404로 실패할 때까지 모른다.
	if req.Model != "" && !strings.Contains(req.Model, "/") {
		writeErr(w, http.StatusBadRequest, `모델은 "제공자/모델" 꼴이다 (예: `+defaultOpenRouterModel+`)`)
		return
	}
	// **글자 수로 잰다.** 바이트로 재면 한글은 한 자가 3바이트라 상한이
	// 3분의 1로 줄어, 화면에 적어둔 수와 실제로 걸리는 수가 달라진다.
	// 모델 이름도 같은 기준이다.
	if len([]rune(req.Prompt)) > aiPromptMax {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("프롬프트가 너무 길다(%d자까지)", aiPromptMax))
		return
	}

	if err := s.store.saveAISettings(map[string]string{
		aiModelKey:  req.Model,
		aiPromptKey: req.Prompt,
	}); err != nil {
		log.Printf("admin AI 설정 저장 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "설정을 저장하지 못했다")
		return
	}
	log.Printf("admin AI 설정 저장: model=%q prompt=%d자", req.Model, len([]rune(req.Prompt)))
	s.handleAI(w, r)
}

// handleAICredits는 이 키에 남은 금액을 준다.
//
// 키가 없으면 503이다 — 글감함의 "초안 생성"과 같은 구별이라, 화면이 두 자리에
// 같은 안내를 적을 수 있다.
func (s *Server) handleAICredits(w http.ResponseWriter, r *http.Request) {
	credits, err := fetchCredits(r.Context(), s.openRouter)
	if err != nil {
		writeLookupErr(w, r, err, "잔액")
		return
	}
	writeJSON(w, http.StatusOK, credits)
}

// handleAIModels는 고를 수 있는 모델 목록을 준다. 화면의 자동완성이 쓴다.
func (s *Server) handleAIModels(w http.ResponseWriter, r *http.Request) {
	models, err := fetchModels(r.Context(), s.openRouter)
	if err != nil {
		writeLookupErr(w, r, err, "모델 목록")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

func writeLookupErr(w http.ResponseWriter, r *http.Request, err error, what string) {
	if errors.Is(err, errOpenRouterNotConfigured) {
		writeErr(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	log.Printf("admin OpenRouter %s 조회 실패: %s %s: %v", what, r.Method, r.URL.Path, err)
	writeErr(w, http.StatusBadGateway, what+"을 가져오지 못했다: "+err.Error())
}
