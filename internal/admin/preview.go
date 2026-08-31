package admin

import (
	"encoding/json"
	"io"
	"net/http"
)

// previewMaxBytes는 미리보기로 받을 마크다운의 최대 크기다.
//
// 지금 가장 긴 글이 약 40KB다. 여유를 두되 무제한으로 열지는 않는다 —
// 인증이 없는 동안 이 엔드포인트가 마크다운 파서에 아무거나 먹일 수 있는
// 유일한 자리다.
const previewMaxBytes = 1 << 20 // 1MB

type previewReq struct {
	Markdown string `json:"markdown"`
}

type previewResp struct {
	HTML string `json:"html"`
	// Outline은 글 상세에 붙는 것과 같은 목차다. 미리보기에서도 제목 구조가
	// 어떻게 잡히는지 같이 보여준다.
	Outline []outlineItem `json:"outline"`
}

type outlineItem struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
	ID    string `json:"id"`
}

// handlePreview는 마크다운을 HTML로 바꿔 돌려준다.
//
// **공개 페이지와 같은 렌더러를 쓴다**(markdown.New). 수식·코드 라벨·외부 링크
// 카드·제목 한 단계 내리기가 전부 그 안에 있어서, 여기서 본 것과 발행 뒤 화면이
// 같다. 미리보기 전용 렌더러를 따로 두면 반드시 갈라진다.
//
// # 아직 다른 점 하나
//
// 실제 글 화면은 렌더링 직전에 죽은 링크 두 종류를 손본다(web의 resolveBody —
// `[페이지 링크]` 자리표시자와 인라인 데이터베이스 펼치기). 그건 노션에서 온
// 본문에만 해당하고 DB 조회가 필요해서 여기서는 안 한다. **새로 쓰는 글에는
// 애초에 그런 링크가 없으므로 지금은 차이가 없다.** 옛 글을 admin에서 열어
// 고칠 때만 미리보기와 실제 화면이 그만큼 다르다.
func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, previewMaxBytes))
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "본문이 너무 크다")
		return
	}
	var req previewReq
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON을 읽지 못했다: "+err.Error())
		return
	}

	html, err := s.md.Render(req.Markdown)
	if err != nil {
		// 마크다운 렌더링은 웬만해선 실패하지 않는다. 실패하면 원인을 그대로
		// 보여준다 — 글 쓰는 사람이 방금 친 것 때문일 테니 감출 이유가 없다.
		writeErr(w, http.StatusUnprocessableEntity, "렌더링 실패: "+err.Error())
		return
	}

	out := previewResp{HTML: string(html), Outline: []outlineItem{}}
	for _, h := range s.md.Outline(req.Markdown) {
		out.Outline = append(out.Outline, outlineItem{Level: h.Level, Text: h.Text, ID: h.ID})
	}
	writeJSON(w, http.StatusOK, out)
}
