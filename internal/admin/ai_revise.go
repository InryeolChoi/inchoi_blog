package admin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// 기존 글은 AI가 직접 저장하지 않는다. 제안을 편집기로 돌려주고, 사람이
// 확인한 다음 기존 저장 경로의 rev 검사와 트랜잭션을 거치게 한다.
const revisePrompt = `당신은 한국어 기술 블로그 글의 편집자다. 기존 글과 작성자의 수정 요청을 받아 수정안을 마크다운으로 돌려준다.

- 기존 글에 없는 사실, 수치, 코드, 인용을 지어내지 않는다.
- 작성자가 요청한 범위만 고친다. 기존 링크, 이미지, 코드, 수식과 기술적 의미를 보존한다.
- 제목 변경을 요청받지 않았다면 제목을 그대로 둔다.
- 결과의 첫 줄은 "# 제목"이다. 빈 줄 하나 뒤에 본문을 쓴다. 설명이나 코드 펜스로 결과 전체를 감싸지 않는다.
- 본문의 절 제목은 "##"부터 쓴다. 임의의 HTML이나 script를 새로 넣지 않는다.
- 평서형 "~다" 문체를 유지한다.`

const reviseInstructionMax = 2000

type reviseReq struct {
	Rev         string `json:"rev"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	Instruction string `json:"instruction"`
}

func (s *Server) handleRevisePost(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2*saveMaxBytes))
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "요청이 너무 크다")
		return
	}
	var req reviseReq
	if err := json.Unmarshal(data, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON을 읽지 못했다")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Instruction = strings.TrimSpace(req.Instruction)
	if req.Rev == "" || req.Title == "" || req.Instruction == "" ||
		len([]rune(req.Title)) > titleMaxLen || len(req.Body) > saveMaxBytes ||
		len([]rune(req.Instruction)) > reviseInstructionMax {
		writeErr(w, http.StatusBadRequest, "제목·수정 요청을 적고 글의 크기를 확인해라")
		return
	}

	post, err := s.store.postBySlug(r.PathValue("slug"))
	if err != nil {
		writeGenerateErr(w, r, fmt.Errorf("%w: 글을 읽지 못했다: %v", errLocalFailure, err))
		return
	}
	if post == nil {
		writeErr(w, http.StatusNotFound, "그런 글이 없다")
		return
	}
	if post.Rev != req.Rev {
		writeErr(w, http.StatusConflict, errRevMismatch.Error())
		return
	}

	model, _, err := s.store.aiConfig(s.openRouter)
	if err != nil {
		writeGenerateErr(w, r, fmt.Errorf("%w: AI 설정을 읽지 못했다: %v", errLocalFailure, err))
		return
	}
	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(draftWriteTimeout)); err != nil {
		writeErr(w, http.StatusInternalServerError, "AI 수정 요청을 시작하지 못했다")
		return
	}
	result, err := completeOpenRouter(r.Context(), s.openRouter, model, revisePrompt,
		"수정 요청:\n"+req.Instruction+"\n\n기존 제목: "+req.Title+"\n\n기존 본문:\n"+req.Body)
	if err != nil {
		writeGenerateErr(w, r, err)
		return
	}
	title, body := splitDraftTitle(result)
	if title == "" {
		title = req.Title
	}
	if strings.TrimSpace(body) == "" || len([]rune(title)) > titleMaxLen || len(body) > saveMaxBytes {
		writeErr(w, http.StatusBadGateway, "AI가 저장 가능한 수정안을 돌려주지 않았다")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"title": title, "body": body, "model": model})
}
