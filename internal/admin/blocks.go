package admin

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/inryeol/blog/internal/markdown"
)

// 블록 인라인 편집기의 뒷단이다.
//
// 평소에는 그려진 화면만 보이다가 문단을 누르면 그 문단만 원문이 된다. 그러려면
// 화면이 **원문 조각과 그린 결과를 짝지어** 들고 있어야 한다.
//
// # 왜 서버가 자르고 서버가 그리나
//
// **그리는 것이 발행될 바로 그 HTML이어야 하기 때문이다.** 브라우저에서 흉내
// 내면 편집기에서 본 것과 발행 뒤 화면이 갈리는데, 그건 이 방식을 고른 이유
// 자체를 없앤다(미리보기가 렌더러·CSS·CDN 태그를 공개 쪽에서 그대로 가져다
// 쓰는 것과 같은 규칙이다).
//
// # 블록 하나씩 그리는 것이 문서 전체와 다르지 않나
//
// 다르다. 제목 id는 문서 안에서 겹치면 번호가 붙고(`headingIDs`), 목차도 문서
// 전체를 봐야 나온다. **그건 저장한 뒤 다시 받아온 화면이 정본이다** — 편집
// 중에는 그 문단이 어떻게 보일지만 알면 된다. 실제 글 1,252편으로 확인한 것은
// "잘라 이어 붙여도 문서 전체의 화면이 같다"는 쪽이다(markdown.SplitBlocks).

type blocksReq struct {
	Markdown string `json:"markdown"`
}

type blockOut struct {
	Src  string `json:"src"`
	HTML string `json:"html"`
}

type blocksResp struct {
	Blocks []blockOut `json:"blocks"`
}

// handleBlocks는 마크다운을 문단으로 자르고 각각을 그려 돌려준다.
//
// **하나만 다시 그릴 때도 같은 자리를 쓴다.** 문단 하나를 보내면 블록 하나가
// 돌아온다 — 고친 문단이 여럿으로 갈리는 경우(빈 줄을 넣었을 때)가 있어서
// 개수를 미리 정해두지 않는다.
func (s *Server) handleBlocks(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, previewMaxBytes))
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "본문이 너무 크다")
		return
	}
	var req blocksReq
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON을 읽지 못했다: "+err.Error())
		return
	}

	out := blocksResp{Blocks: []blockOut{}}
	for _, src := range markdown.SplitBlocks(req.Markdown) {
		html, err := s.md.Render(src)
		if err != nil {
			writeErr(w, http.StatusUnprocessableEntity, "렌더링 실패: "+err.Error())
			return
		}
		out.Blocks = append(out.Blocks, blockOut{Src: src, HTML: string(html)})
	}
	writeJSON(w, http.StatusOK, out)
}
