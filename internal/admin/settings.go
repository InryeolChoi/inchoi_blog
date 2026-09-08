package admin

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/inryeol/blog/internal/web"
)

// 사이트 문구를 고치는 자리다. 지금은 홈 표제지 넷뿐이다(migrations/008).
//
// **글쓰기와 같은 이유로 있다.** 문장 하나를 고치려고 코드를 고쳐 배포하는 것은
// "글은 웹 UI에서 직접 쓰고 고친다"와 어긋난다 — 홈에 서는 문장도 결국 글이다.

// settingsMax는 값 하나의 길이 상한이다. 표제지에 서는 문장이라 길어질 이유가
// 없고, 상한이 없으면 이 표가 본문을 담는 자리로 흘러간다.
const settingsMax = 400

type settingsBody struct {
	Values map[string]string `json:"values"`
}

// handleSettings는 지금 값과 기본값을 함께 준다.
//
// **기본값을 같이 주는 이유**는 화면이 그것을 자리표시자로 쓰기 때문이다.
// 빈 칸에 회색으로 기본 문구가 보이면 "비우면 이게 나온다"가 눈에 보인다.
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	saved, err := web.Settings(s.store.db)
	if err != nil {
		log.Printf("admin 설정 조회 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "설정을 읽지 못했다")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"keys":     web.HomeKeys,
		"values":   saved,
		"defaults": web.DefaultHomeText(),
	})
}

// handleSaveSettings는 보낸 값만 고친다.
//
// **여기는 통째로 바꾸기가 아니다.** 글 저장(PUT)은 안 보낸 칸을 비우지만,
// 그건 한 글의 모든 칸을 한 화면이 들고 있기 때문이다. 설정은 나중에 다른
// 화면이 다른 키를 저장하게 되므로, 안 보낸 키를 지우면 그 화면이 이 화면의
// 값을 조용히 날린다.
//
// **빈 값은 지운다.** 행이 없다는 것이 곧 "기본값을 쓴다"라서, 빈 문자열을
// 남겨두면 "비웠다"와 "안 정했다"가 표에서 구별되지 않는다.
func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<16))
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "요청이 너무 크다")
		return
	}
	var req settingsBody
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "요청을 읽지 못했다")
		return
	}

	// **모르는 키는 거절한다.** 받아주면 이 표가 아무나 아무거나 쌓는 자리가
	// 되고, 화면에 안 나오는 행이 남아서 무엇이 쓰이는지 알 수 없게 된다.
	allowed := map[string]bool{}
	for _, k := range web.HomeKeys {
		allowed[k] = true
	}
	for k, v := range req.Values {
		if !allowed[k] {
			writeErr(w, http.StatusBadRequest, "모르는 설정이다")
			return
		}
		if len([]rune(v)) > settingsMax {
			writeErr(w, http.StatusBadRequest, fmt.Sprintf("문구가 너무 길다(%d자까지)", settingsMax))
			return
		}
	}

	tx, err := s.store.db.Begin()
	if err != nil {
		log.Printf("admin 설정 저장 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "설정을 저장하지 못했다")
		return
	}
	defer tx.Rollback()
	for k, v := range req.Values {
		v = strings.TrimSpace(v)
		if v == "" {
			if _, err = tx.Exec(`DELETE FROM settings WHERE key = ?`, k); err != nil {
				break
			}
			continue
		}
		if _, err = tx.Exec(`
			INSERT INTO settings (key, value, updated_at) VALUES (?, ?, datetime('now'))
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			k, v); err != nil {
			break
		}
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		log.Printf("admin 설정 저장 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "설정을 저장하지 못했다")
		return
	}
	log.Printf("admin 설정 저장: %d개", len(req.Values))
	s.handleSettings(w, r)
}
