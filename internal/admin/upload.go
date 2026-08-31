package admin

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	// image.DecodeConfig가 알아볼 형식을 등록한다. 쓰는 것은 부작용뿐이다.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// uploadMaxBytes는 이미지 업로드로 받을 최대 크기다. 지금 DB에 있는 가장 큰
// 이미지가 3.3MB라 그보다 넉넉하게 잡는다.
const uploadMaxBytes = 8 << 20 // 8MB

// allowedImageTypes는 받아서 저장할 형식이다.
//
// **SVG는 일부러 뺐다.** `/img/{sha256}`이 images.mime을 그대로 Content-Type에
// 실어 보내는데, image/svg+xml은 브라우저가 문서로 열고 그 안의 <script>가
// **우리 도메인에서** 실행된다. 그림 하나를 올리는 일이 사이트 전체에 대한
// XSS가 되는 것이라, 그림처럼 보인다고 그림으로 다루지 않는다.
var allowedImageTypes = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
	"image/gif":  "gif",
	"image/webp": "webp",
}

// uploadResp는 화면이 본문에 마크다운을 끼우는 데 필요한 것이다.
type uploadResp struct {
	SHA256 string `json:"sha256"`
	URL    string `json:"url"`
	MIME   string `json:"mime"`
	Bytes  int    `json:"bytes"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	// Existed는 같은 내용이 이미 DB에 있었다는 뜻이다. 사고가 아니라 정상이고,
	// 화면이 "이미 있던 그림이다"라고 말해줄 수 있어야 한다.
	Existed bool `json:"existed"`
	// Markdown은 본문에 그대로 끼울 한 줄이다. 만드는 규칙을 화면과 서버
	// 두 곳에 두지 않으려고 서버가 만들어 보낸다.
	Markdown string `json:"markdown"`
}

// handleUpload는 이미지를 받아 DB에 넣는다.
//
// **멱등 키는 내용의 sha256이다.** 파일 이름이 아니다 — 같은 그림을 두 번
// 올려도 행이 늘지 않고, 이름을 바꿔 올려도 새 그림이 되지 않는다.
// cmd/import가 이미지 445개를 넣을 때 쓴 규칙과 같다.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	// **여기서만 읽기 마감을 미룬다.**
	//
	// 서버 전체의 ReadTimeout은 15초다(cmd/blog의 httpServer). 그건 GET만
	// 받던 시절의 값이라 8MB짜리 업로드에는 초당 546KB가 필요하다 — 느린
	// 회선에서는 못 끝낸다. 그렇다고 서버 값을 늘리면 **공개 GET의 방어까지**
	// 같이 헐거워진다. 이 요청 하나만 늘리는 것이 맞다.
	//
	// 헤더 단계(ReadHeaderTimeout 5초)는 그대로다. 느린 연결로 소켓을 붙들고
	// 있는 공격은 거기서 막힌다.
	if rc := http.NewResponseController(w); rc != nil {
		deadline := time.Now().Add(2 * time.Minute)
		// 지원하지 않는 ResponseWriter면 ErrNotSupported다. 그때는 서버
		// 기본값으로 도는 것뿐이라 실패로 다루지 않는다.
		_ = rc.SetReadDeadline(deadline)
		_ = rc.SetWriteDeadline(deadline)
	}

	// **ParseMultipartForm의 인자는 "메모리에 둘 양"이지 상한이 아니다.**
	// 나머지는 디스크로 흘러가므로, 진짜 상한은 MaxBytesReader로 건다.
	r.Body = http.MaxBytesReader(w, r.Body, uploadMaxBytes+1<<10)
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "파일을 읽지 못했다 (8MB까지): "+err.Error())
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "image 필드가 없다")
		return
	}
	defer file.Close()

	// 통째로 읽는다. sha256을 내려면 어차피 끝까지 봐야 하고, 8MB가 상한이다.
	data, err := io.ReadAll(io.LimitReader(file, uploadMaxBytes+1))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "파일을 읽다 실패했다")
		return
	}
	if len(data) > uploadMaxBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "파일이 너무 크다 (8MB까지)")
		return
	}
	if len(data) == 0 {
		writeErr(w, http.StatusBadRequest, "빈 파일이다")
		return
	}

	// **형식은 내용을 보고 정한다.** 브라우저가 보낸 Content-Type도 파일
	// 확장자도 올리는 쪽이 마음대로 적을 수 있는 값이라, 그걸 믿고
	// images.mime에 넣으면 그 값이 그대로 응답 헤더가 된다.
	mime := http.DetectContentType(data)
	ext, ok := allowedImageTypes[mime]
	if !ok {
		writeErr(w, http.StatusUnsupportedMediaType,
			fmt.Sprintf("%s는 받지 않는다. png·jpeg·gif·webp만 된다", mime))
		return
	}

	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])

	// 크기는 아는 만큼만 적는다. webp는 표준 라이브러리가 못 읽어서 NULL이 된다 —
	// 없는 값을 0으로 적으면 "가로 0픽셀"이 되어 뒤에 읽는 쪽을 속인다.
	var width, height *int
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		width, height = &cfg.Width, &cfg.Height
	}

	existed, err := s.store.saveImage(sha, data, mime, width, height, time.Now().UTC())
	if err != nil {
		log.Printf("admin 이미지 저장 실패(%s): %v", sha[:12], err)
		writeErr(w, http.StatusInternalServerError, "이미지를 저장하지 못했다")
		return
	}

	resp := uploadResp{
		SHA256: sha, URL: "/img/" + sha, MIME: mime,
		Bytes: len(data), Existed: existed,
		// alt는 비워 둔다. 파일 이름을 alt로 넣으면 "IMG_4821.png"가 화면
		// 낭독기에 읽힌다. 무엇인지는 글 쓰는 사람이 적는다.
		Markdown: "![](/img/" + sha + ")",
	}
	if width != nil {
		resp.Width, resp.Height = *width, *height
	}
	log.Printf("admin 이미지 %s: %q %s %d바이트 sha=%s.%s",
		map[bool]string{true: "이미 있음", false: "저장"}[existed],
		header.Filename, mime, len(data), sha[:12], ext)
	writeJSON(w, http.StatusOK, resp)
}

// saveImage는 이미지를 넣는다. 이미 있으면 (true, nil)이고 아무것도 안 바꾼다.
//
// **있는 행을 덮지 않는다.** sha256이 같으면 내용이 같다는 뜻이라 덮을 것이
// 없고, caption·original_url처럼 사람이나 이관이 채워둔 칸을 지울 이유는
// 더더욱 없다.
func (s *store) saveImage(sha string, data []byte, mime string, w, h *int, now time.Time) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT count(*) FROM images WHERE sha256 = ?`, sha).Scan(&n)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	_, err = s.db.Exec(`
		INSERT INTO images (sha256, data, mime, width, height, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, sha, data, mime, w, h, now)
	if err != nil {
		// 같은 그림을 두 번 동시에 올리면 여기서 만난다. 그건 실패가 아니다.
		if isUniqueSHA(err) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func isUniqueSHA(err error) bool {
	e := err.Error()
	return strings.Contains(e, "UNIQUE") && strings.Contains(e, "images.sha256")
}
