package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// 글감함. 프로젝트를 하는 동안 남긴 메모를 쌓아 뒀다가, 나중에 AI(OpenRouter)로
// 정리해서 draft 글 하나를 만든다. **최종 편집은 사람이 admin에서 한다** —
// AI가 만든 것은 언제나 draft이고, 공개는 사람이 status를 바꿔야 일어난다.
//
// posts와 따로 둔 이유는 migrations/009_notes.sql에 적었다.

// noteTitleMaxLen과 noteBodyMaxLen은 폼 입력의 상한이다. 메모라 길 이유가
// 없지만, 프로젝트 회고처럼 긴 메모도 있을 수 있어 글 본문(saveMaxBytes)보다는
// 넉넉하게, 무제한은 아니게 둔다.
const noteTitleMaxLen = titleMaxLen
const noteBodyMaxLen = 200 << 10 // 200KB

// NoteRow는 글감함 목록에 한 줄로 찍을 글감이다.
type NoteRow struct {
	ID            int64      `json:"id"`
	Title         string     `json:"title"`
	Status        string     `json:"status"`
	GeneratedSlug *string    `json:"generatedSlug"`
	CreatedAt     *time.Time `json:"createdAt"`
	UpdatedAt     *time.Time `json:"updatedAt"`
}

// NoteDetail은 편집 폼을 채울 글감 하나다.
type NoteDetail struct {
	NoteRow
	Body string `json:"body"`
}

// listNotes는 최근 것부터 준다. 아직 페이지 나누기가 필요할 만큼 쌓일 자리가
// 아니다 — 글과 달리 처리하고 나면 화면에서 할 일이 끝난다.
func (s *store) listNotes() ([]NoteRow, error) {
	rows, err := s.db.Query(`
		SELECT n.id, n.title, n.status, p.slug, n.created_at, n.updated_at
		FROM notes n LEFT JOIN posts p ON p.id = n.generated_post_id
		ORDER BY n.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []NoteRow{}
	for rows.Next() {
		var r NoteRow
		var slug sql.NullString
		if err := rows.Scan(&r.ID, &r.Title, &r.Status, &slug, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		if slug.Valid {
			r.GeneratedSlug = &slug.String
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *store) noteByID(id int64) (*NoteDetail, error) {
	var d NoteDetail
	var slug sql.NullString
	err := s.db.QueryRow(`
		SELECT n.id, n.title, n.body, n.status, p.slug, n.created_at, n.updated_at
		FROM notes n LEFT JOIN posts p ON p.id = n.generated_post_id
		WHERE n.id = ?`, id).
		Scan(&d.ID, &d.Title, &d.Body, &d.Status, &slug, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if slug.Valid {
		d.GeneratedSlug = &slug.String
	}
	return &d, nil
}

func (s *store) createNote(title, body string, now time.Time) (*NoteDetail, error) {
	res, err := s.db.Exec(`INSERT INTO notes (title, body, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		title, body, now, now)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.noteByID(id)
}

// updateNote는 제목·본문만 고친다. **이미 초안을 만들었어도 고칠 수 있다** —
// 초안이 마음에 안 들어 글감을 다시 다듬고 재생성하는 것이 정상적인 쓰임이다.
func (s *store) updateNote(id int64, title, body string, now time.Time) (*NoteDetail, error) {
	res, err := s.db.Exec(`UPDATE notes SET title = ?, body = ?, updated_at = ? WHERE id = ?`,
		title, body, now, id)
	if err != nil {
		return nil, err
	}
	if n, err := res.RowsAffected(); err != nil {
		return nil, err
	} else if n == 0 {
		return nil, nil
	}
	return s.noteByID(id)
}

func (s *store) deleteNote(id int64) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM notes WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// generateNoteDraft는 글감 하나를 OpenRouter로 정리해 draft 글을 만들고,
// 그 글감을 그 글에 연결한다.
//
// **글 생성은 savePost를 그대로 쓴다.** slug 만들기·검증·트랜잭션이 편집기로
// 쓴 글과 똑같이 적용되어야 한다 — 여기서 따로 INSERT를 만들면 언젠가 그
// 규칙과 갈라진다.
func (s *store) generateNoteDraft(ctx context.Context, cfg OpenRouterConfig, id int64, now time.Time) (*PostDetail, error) {
	note, err := s.noteByID(id)
	if err != nil {
		return nil, err
	}
	if note == nil {
		return nil, errNoSuchNote
	}

	// 모델과 프롬프트는 설정에서 고른다(ai.go). 사람이 환경설정에서 고쳤으면
	// 그것이, 아니면 서버 환경변수가, 그것도 없으면 코드 기본값이 쓰인다.
	model, prompt, err := s.aiConfig(cfg)
	if err != nil {
		// **DB를 못 읽은 것이다.** 그냥 올리면 writeGenerateErr의 기본 가지에
		// 걸려 502(남의 서비스 탓)로 나가는데, 이건 이 서버 탓이라 500이어야
		// 한다 — 어디를 봐야 하는지가 달라진다.
		return nil, fmt.Errorf("%w: AI 설정을 읽지 못했다: %v", errLocalFailure, err)
	}

	title, body, err := generateDraftBody(ctx, cfg, model, prompt, note.Title, note.Body)
	if err != nil {
		return nil, err
	}
	if title == "" {
		// 모델이 "# 제목" 형태를 안 지켰다. 글감 제목으로 대신 채운다 —
		// 형식 하나 어겼다고 초안 생성 전체를 실패로 돌리지 않는다.
		title = note.Title
	}

	req := saveReq{Title: title, Body: body, Status: "draft", Visibility: "public"}
	if err := normalizeSave(&req); err != nil {
		return nil, fmt.Errorf("AI가 만든 초안이 저장 규칙에 안 맞는다: %w", err)
	}

	post, err := s.savePost("", req, true, now)
	if err != nil {
		return nil, err
	}

	if _, err := s.db.Exec(`UPDATE notes SET status = 'generated', generated_post_id = ?, updated_at = ? WHERE id = ?`,
		post.ID, now, id); err != nil {
		return nil, err
	}
	return post, nil
}

var errNoSuchNote = errors.New("그런 글감이 없다")

// errLocalFailure는 **이 서버 탓**인 실패다. OpenRouter가 준 오류와 갈라서
// 상태 코드를 다르게 내보낸다(writeGenerateErr).
var errLocalFailure = errors.New("서버 오류")

// ------------------------------------------------------------- HTTP 핸들러

type noteReq struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func normalizeNote(req *noteReq) error {
	req.Title = strings.TrimSpace(req.Title)
	req.Body = strings.TrimSpace(req.Body)
	if req.Title == "" {
		return bad("제목이 비었다")
	}
	if len([]rune(req.Title)) > noteTitleMaxLen {
		return bad("제목이 너무 길다 (%d자까지)", noteTitleMaxLen)
	}
	if req.Body == "" {
		return bad("본문이 비었다")
	}
	if len(req.Body) > noteBodyMaxLen {
		return bad("본문이 너무 크다")
	}
	return nil
}

func (s *Server) handleListNotes(w http.ResponseWriter, r *http.Request) {
	notes, err := s.store.listNotes()
	if err != nil {
		log.Printf("admin 글감 목록 조회 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "글감 목록을 가져오지 못했다")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notes": notes})
}

func (s *Server) handleCreateNote(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, noteBodyMaxLen+1<<16))
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "요청이 너무 크다")
		return
	}
	var req noteReq
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON을 읽지 못했다: "+err.Error())
		return
	}
	if err := normalizeNote(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	note, err := s.store.createNote(req.Title, req.Body, time.Now().UTC())
	if err != nil {
		log.Printf("admin 글감 만들기 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "글감을 만들지 못했다")
		return
	}
	writeJSON(w, http.StatusCreated, note)
}

func noteID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil
}

func (s *Server) handleUpdateNote(w http.ResponseWriter, r *http.Request) {
	id, ok := noteID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "글감 id가 아니다")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, noteBodyMaxLen+1<<16))
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "요청이 너무 크다")
		return
	}
	var req noteReq
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON을 읽지 못했다: "+err.Error())
		return
	}
	if err := normalizeNote(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	note, err := s.store.updateNote(id, req.Title, req.Body, time.Now().UTC())
	if err != nil {
		log.Printf("admin 글감 고치기 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "글감을 고치지 못했다")
		return
	}
	if note == nil {
		writeErr(w, http.StatusNotFound, "그런 글감이 없다")
		return
	}
	writeJSON(w, http.StatusOK, note)
}

func (s *Server) handleDeleteNote(w http.ResponseWriter, r *http.Request) {
	id, ok := noteID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "글감 id가 아니다")
		return
	}
	found, err := s.store.deleteNote(id)
	if err != nil {
		log.Printf("admin 글감 지우기 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "글감을 지우지 못했다")
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "그런 글감이 없다")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleGenerateNote는 글감 하나를 AI로 정리해 draft 글을 만든다.
//
// 이 요청은 일반 페이지보다 오래 걸릴 수 있다. 쓰기 기한을 이 핸들러에서만
// 늘리고, OpenRouter 요청의 기한은 그보다 짧게 둔다.
func (s *Server) handleGenerateNote(w http.ResponseWriter, r *http.Request) {
	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(draftWriteTimeout)); err != nil {
		log.Printf("admin 글감 초안 생성 쓰기 기한 설정 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "초안을 만들지 못했다")
		return
	}
	id, ok := noteID(r)
	if !ok {
		writeErr(w, http.StatusBadRequest, "글감 id가 아니다")
		return
	}
	post, err := s.store.generateNoteDraft(r.Context(), s.openRouter, id, time.Now().UTC())
	if err != nil {
		writeGenerateErr(w, r, err)
		return
	}
	log.Printf("admin 글감 초안 생성: note id=%d -> slug=%q", id, post.Slug)
	writeJSON(w, http.StatusOK, post)
}

func writeGenerateErr(w http.ResponseWriter, r *http.Request, err error) {
	var bi badInput
	switch {
	case errors.As(err, &bi):
		writeErr(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, errNoSuchNote):
		writeErr(w, http.StatusNotFound, err.Error())
	case errors.Is(err, errOpenRouterNotConfigured):
		writeErr(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, errOpenRouterTimeout):
		log.Printf("admin 글감 초안 생성 시간 초과: %s %s: %v", r.Method, r.URL.Path, err)
		writeErr(w, http.StatusGatewayTimeout, errOpenRouterTimeout.Error())
	case errors.Is(err, errLocalFailure):
		log.Printf("admin 글감 초안 생성 실패(서버): %s %s: %v", r.Method, r.URL.Path, err)
		writeErr(w, http.StatusInternalServerError, "초안을 만들지 못했다")
	default:
		log.Printf("admin 글감 초안 생성 실패: %s %s: %v", r.Method, r.URL.Path, err)
		writeErr(w, http.StatusBadGateway, "초안을 만들지 못했다: "+err.Error())
	}
}
