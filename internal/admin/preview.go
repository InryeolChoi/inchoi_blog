package admin

import (
	"encoding/json"
	"io"
	"log"
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

// saveReq는 편집 폼이 보내는 것이다. 3단계에서 이 구조 그대로 DB에 들어간다.
type saveReq struct {
	Slug   string `json:"slug"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Status string `json:"status"`
}

// handleSave는 **아직 아무것도 저장하지 않는다.**
//
// 로드맵 3단계(저장)가 오기 전까지 이 자리는 일부러 비어 있다. 그래도 라우트와
// 요청 모양은 지금 만들어 둔다 — 화면 쪽 배선이 이미 맞아 있으면 3단계에서
// 고칠 곳이 이 함수 하나가 된다.
//
// **200을 주지 않는다.** 성공으로 답하면 화면이 "저장됨"이라고 말하고, 쓰는
// 사람은 안 들어간 글을 들어갔다고 믿는다. 501(Not Implemented)로 분명히 한다.
func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, previewMaxBytes))
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "본문이 너무 크다")
		return
	}
	var req saveReq
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON을 읽지 못했다: "+err.Error())
		return
	}
	if slug := r.PathValue("slug"); slug != "" {
		req.Slug = slug
	}
	log.Printf("admin 저장 요청(아직 저장 안 함): slug=%q title=%q status=%q 본문 %d바이트",
		req.Slug, req.Title, req.Status, len(req.Body))
	writeErr(w, http.StatusNotImplemented,
		"저장은 아직 안 만들었다 (로드맵 3단계). 서버 로그에 요청만 남겼다.")
}

// uploadMaxBytes는 이미지 업로드로 받을 최대 크기다. 지금 DB에 있는 가장 큰
// 이미지가 3.3MB라 그보다 넉넉하게 잡는다.
const uploadMaxBytes = 8 << 20 // 8MB

// handleUpload도 **아직 아무것도 저장하지 않는다.** 받은 파일을 버린다.
//
// 그래도 파일을 실제로 읽어서 크기와 형식을 확인한다 — 화면이 "고른 파일이
// 서버까지 갔다"는 것까지는 지금 확인할 수 있어야 껍데기가 쓸모 있다.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(uploadMaxBytes); err != nil {
		writeErr(w, http.StatusBadRequest, "파일을 읽지 못했다: "+err.Error())
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "image 필드가 없다")
		return
	}
	defer file.Close()

	// io.Copy로 버린다. 크기를 알려면 끝까지 읽어야 하고, 다음 단계에서
	// 여기가 sha256 계산 + BLOB 저장이 될 자리다.
	n, err := io.Copy(io.Discard, io.LimitReader(file, uploadMaxBytes+1))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "파일을 읽다 실패했다")
		return
	}
	if n > uploadMaxBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "파일이 너무 크다 (8MB까지)")
		return
	}
	log.Printf("admin 이미지 업로드(아직 저장 안 함): %q %s %d바이트",
		header.Filename, header.Header.Get("Content-Type"), n)
	writeErr(w, http.StatusNotImplemented,
		"이미지 저장은 아직 안 만들었다 (로드맵 3단계). 파일은 서버까지 왔고 버렸다.")
}
