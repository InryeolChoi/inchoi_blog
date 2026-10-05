package admin

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/inryeol/blog/internal/web"
)

// 문체 가이드는 필자가 직접 쓴 글에서 뽑은 문체 규칙이다. 초안을 만들 때
// 지시문 뒤에 자동으로 붙는다(withStyle).
//
// # 왜 지시문(ai.prompt)과 따로 두는가
//
// 지시문에는 코드가 기대는 형식 규칙이 있다 — 첫 줄이 "# 제목"이어야 한다
// (splitDraftTitle). 문체는 글이 쌓일 때마다 손보는 값이다. 한 문자열에
// 두면 문체를 고치다가 형식 규칙을 지우게 된다.
//
// # 왜 파일이 아니라 settings인가
//
// 저장소 폴더의 파일은 바이너리에 포함되어 서버가 고칠 수 없다. settings는
// 배포 없이 고치고, DB 백업에 함께 들어간다.
//
// # 왜 갱신을 자동으로 덮어쓰지 않는가
//
// 모델이 가이드를 한 번 이상하게 뽑으면 그 문체가 모든 다음 초안에 퍼진다.
// 그래서 갱신은 **제안만 돌려주고**, 사람이 수락해야 저장된다(기존 글 AI
// 수정과 같은 방식이다).
const (
	aiStyleKey = "ai.style"
	aiStyleMax = 4 << 10 // 글자 수
)

// 문체를 뽑을 근거 글을 고르는 한도다. 가이드 하나를 뽑으려고 글 전체를
// 보내지 않는다 — 글자 수를 제한하되 최신 글부터 채운다.
const (
	styleSourceMinChars = 300
	styleSourcePerPost  = 6000
	styleSourceTotal    = 60000
)

const styleProposePrompt = `당신은 한 필자의 블로그 글을 읽고 그 필자의 문체 규칙을 정리하는 편집자다.

필자의 글들과 지금 쓰는 문체 가이드가 주어진다. 글에서 실제로 보이는 문체 규칙으로 가이드를 새로 쓴다.

- 글에서 관찰되는 것만 쓴다. 보이지 않는 습관을 지어내지 않는다.
- 지금 가이드의 규칙이 글로 뒷받침되면 유지하고, 글과 어긋나면 고친다.
- 어미, 문장 길이, 첫 문장을 시작하는 방식, 이유와 판단을 말하는 방식, 목록과 강조를 쓰는 방식, 자주 쓰는 표현과 쓰지 않는 표현을 다룬다.
- 글의 주제와 내용을 요약하지 않는다. 문체 규칙만 쓴다.
- 규칙은 모델이 따를 수 있게 명령형이나 평서형 짧은 항목으로 쓴다.
- 결과는 가이드 본문 마크다운만 쓴다. 제목, 설명, 코드 펜스로 감싸지 않는다.
- 전체를 3,500자 이내로 쓴다.
- 글이 적거나 짧아 근거가 약한 규칙은 그렇게 표시하지 말고 쓰지 않는다.`

// aiStyle은 저장된 문체 가이드를 준다. 안 정했으면 빈 문자열이다.
func (s *store) aiStyle() (string, error) {
	saved, err := web.Settings(s.db)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(saved[aiStyleKey]), nil
}

// withStyle은 초안 생성 지시문 뒤에 문체 가이드를 붙인다. 가이드가 비어 있으면
// 지시문을 그대로 돌려준다.
//
// **형식 규칙이 이긴다고 적는다.** 가이드가 "제목 없이 시작" 같은 규칙을 품어도
// 첫 줄의 "# 제목"은 코드가 기대는 계약이다.
func withStyle(prompt, style string) string {
	if style == "" {
		return prompt
	}
	return prompt + "\n\n# 문체 가이드\n\n" +
		"아래는 필자가 직접 쓴 글에서 뽑은 문체 규칙이다. 위의 형식 규칙과 부딪히면 형식 규칙을 따른다.\n\n" + style
}

type styleSource struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
	Body  string `json:"-"`
}

// styleSources는 문체를 뽑을 근거 글을 최신 글부터 고른다.
//
//   - 웹에서 쓴 글(native)만 본다. 노션·GitHub 이관 글은 대부분 개조식 학습
//     메모라 블로그 문체의 근거가 못 된다.
//   - **AI가 만든 초안은 뺀다**(notes.generated_post_id). AI가 쓴 글에서 규칙을
//     뽑으면 가이드가 자기 출력을 되먹임해 점점 한쪽으로 쏠린다.
//   - 짧은 글은 뺀다. 문장이 거의 없는 글에서 읽을 규칙이 없다.
//   - status와 visibility는 가리지 않는다. 제안은 admin만 요청하고, 결과는
//     문체 규칙이라 원문이 화면에 나가지 않는다. 다만 원문은 OpenRouter로 간다.
func (s *store) styleSources() ([]styleSource, error) {
	rows, err := s.db.Query(`
		SELECT slug, title, body FROM posts
		WHERE source = 'native'
		  AND id NOT IN (SELECT generated_post_id FROM notes WHERE generated_post_id IS NOT NULL)
		ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []styleSource
	total := 0
	for rows.Next() {
		var p styleSource
		if err := rows.Scan(&p.Slug, &p.Title, &p.Body); err != nil {
			return nil, err
		}
		runes := []rune(p.Body)
		if len(runes) < styleSourceMinChars {
			continue
		}
		if len(runes) > styleSourcePerPost {
			runes = runes[:styleSourcePerPost]
			p.Body = string(runes)
		}
		if total+len(runes) > styleSourceTotal {
			break
		}
		total += len(runes)
		out = append(out, p)
	}
	return out, rows.Err()
}

// handleProposeStyle은 새 문체 가이드를 **제안**한다. 저장하지 않는다.
func (s *Server) handleProposeStyle(w http.ResponseWriter, r *http.Request) {
	sources, err := s.store.styleSources()
	if err != nil {
		writeGenerateErr(w, r, fmt.Errorf("%w: 근거 글을 읽지 못했다: %v", errLocalFailure, err))
		return
	}
	if len(sources) == 0 {
		writeErr(w, http.StatusConflict, "문체를 뽑을 글이 없다. 웹에서 직접 쓴 글이 "+
			fmt.Sprint(styleSourceMinChars)+"자 이상이어야 한다(AI가 만든 초안은 뺀다)")
		return
	}
	model, _, err := s.store.aiConfig(s.openRouter)
	if err != nil {
		writeGenerateErr(w, r, fmt.Errorf("%w: AI 설정을 읽지 못했다: %v", errLocalFailure, err))
		return
	}
	current, err := s.store.aiStyle()
	if err != nil {
		writeGenerateErr(w, r, fmt.Errorf("%w: 문체 가이드를 읽지 못했다: %v", errLocalFailure, err))
		return
	}
	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(draftWriteTimeout)); err != nil {
		writeErr(w, http.StatusInternalServerError, "문체 제안을 시작하지 못했다")
		return
	}

	var user strings.Builder
	user.WriteString("지금 가이드:\n")
	if current == "" {
		user.WriteString("(없다)")
	} else {
		user.WriteString(current)
	}
	user.WriteString("\n\n필자의 글:")
	for _, p := range sources {
		user.WriteString("\n\n=== " + p.Title + "\n" + p.Body)
	}

	started := time.Now()
	result, err := completeOpenRouter(r.Context(), s.openRouter, model, styleProposePrompt, user.String(), nil)
	if err != nil {
		writeGenerateErr(w, r, err)
		return
	}
	proposal := strings.TrimSpace(result)
	if proposal == "" || len([]rune(proposal)) > aiStyleMax {
		writeErr(w, http.StatusBadGateway, fmt.Sprintf("AI가 쓸 수 있는 가이드를 돌려주지 않았다(%d자까지)", aiStyleMax))
		return
	}
	log.Printf("admin 문체 가이드 제안: 근거 글 %d편 (%s)", len(sources), time.Since(started).Round(time.Millisecond))
	writeJSON(w, http.StatusOK, map[string]any{
		"proposal": proposal,
		"model":    model,
		"sources":  sources,
	})
}

type styleReq struct {
	Style string `json:"style"`
}

// handleSaveStyle은 사람이 수락하거나 고친 문체 가이드를 저장한다. 빈 값은 지운다.
func (s *Server) handleSaveStyle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4*aiStyleMax+1<<16))
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "요청이 너무 크다")
		return
	}
	var req styleReq
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "요청을 읽지 못했다")
		return
	}
	req.Style = strings.TrimSpace(req.Style)
	if len([]rune(req.Style)) > aiStyleMax {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("문체 가이드가 너무 길다(%d자까지)", aiStyleMax))
		return
	}
	if err := s.store.saveAISettings(map[string]string{aiStyleKey: req.Style}); err != nil {
		log.Printf("admin 문체 가이드 저장 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "문체 가이드를 저장하지 못했다")
		return
	}
	log.Printf("admin 문체 가이드 저장: %d자", len([]rune(req.Style)))
	s.handleAI(w, r)
}
