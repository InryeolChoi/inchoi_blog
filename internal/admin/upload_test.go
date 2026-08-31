package admin

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// pngBytes는 진짜 PNG 하나를 만든다. 바이트를 손으로 적어두면 무엇이 왜
// 그 값인지 알 수 없고, 형식 판별이 내용을 본다는 것도 확인할 수 없다.
func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// upload는 multipart로 파일 하나를 올린다.
func upload(t *testing.T, h http.Handler, field, filename string, data []byte, ctype string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/api/admin/images", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set("Origin", "http://"+r.Host)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// 올린 그림이 **정말로 DB에 들어가는지**와, 멱등 키가 파일 이름이 아니라
// 내용의 sha256인지 본다.
func TestUploadStoresTheImageOnceByContent(t *testing.T) {
	sqlDB := testDB(t)
	s, err := New(sqlDB, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()

	data := pngBytes(t, 7, 5)
	sum := sha256.Sum256(data)
	want := hex.EncodeToString(sum[:])

	rec := upload(t, h, "image", "그림.png", data, "image/png")
	if rec.Code != http.StatusOK {
		t.Fatalf("상태 코드 %d: %s", rec.Code, rec.Body.String())
	}
	var got uploadResp
	decode(t, rec, &got)

	if got.SHA256 != want {
		t.Errorf("sha256이 %q다. 내용 해시 %q여야 한다", got.SHA256, want)
	}
	if got.URL != "/img/"+want {
		t.Errorf("URL이 %q다", got.URL)
	}
	if got.Markdown != "![](/img/"+want+")" {
		t.Errorf("본문에 끼울 마크다운이 %q다", got.Markdown)
	}
	if got.Existed {
		t.Error("처음 올린 것이 existed다")
	}
	if got.Width != 7 || got.Height != 5 {
		t.Errorf("크기가 %dx%d다. 7x5여야 한다", got.Width, got.Height)
	}

	// 실제로 DB에 바이트 그대로 있는지
	var blob []byte
	var mime string
	if err := sqlDB.QueryRow(`SELECT data, mime FROM images WHERE sha256 = ?`, want).
		Scan(&blob, &mime); err != nil {
		t.Fatalf("DB에서 못 찾았다: %v", err)
	}
	if !bytes.Equal(blob, data) {
		t.Error("저장된 바이트가 올린 것과 다르다")
	}
	if mime != "image/png" {
		t.Errorf("mime이 %q다", mime)
	}

	// **이름만 바꿔 다시 올려도 행이 안 는다.** 멱등 키는 내용이다.
	rec = upload(t, h, "image", "완전히-다른-이름.png", data, "image/png")
	if rec.Code != http.StatusOK {
		t.Fatalf("두 번째 업로드가 %d다: %s", rec.Code, rec.Body.String())
	}
	decode(t, rec, &got)
	if !got.Existed {
		t.Error("같은 내용을 다시 올렸는데 existed가 아니다")
	}
	var n int
	if err := sqlDB.QueryRow(`SELECT count(*) FROM images`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("images가 %d행이다. 1행이어야 한다", n)
	}
}

// **SVG를 받으면 안 된다.** /img/{sha256}이 images.mime을 그대로
// Content-Type으로 실어 보내므로, image/svg+xml은 브라우저가 문서로 열고
// 그 안의 <script>가 우리 도메인에서 실행된다. 그림 하나 올리는 일이
// 사이트 전체에 대한 XSS가 된다.
func TestUploadRefusesThingsThatAreNotSafeImages(t *testing.T) {
	sqlDB := testDB(t)
	s, err := New(sqlDB, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := s.Handler()

	for _, tc := range []struct {
		name  string
		data  []byte
		ctype string
		want  int
	}{
		{
			"SVG는 안 받는다",
			[]byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
			"image/svg+xml", http.StatusUnsupportedMediaType,
		},
		{
			// Content-Type을 image/png라고 **거짓으로** 적어도 내용을 보고 거절한다.
			"HTML에 png라고 이름표만 붙인 것",
			[]byte("<html><body><script>alert(1)</script></body></html>"),
			"image/png", http.StatusUnsupportedMediaType,
		},
		{"빈 파일", nil, "image/png", http.StatusBadRequest},
	} {
		rec := upload(t, h, "image", "x.png", tc.data, tc.ctype)
		if rec.Code != tc.want {
			t.Errorf("%s: 상태 코드 %d, %d여야 한다 (%s)", tc.name, rec.Code, tc.want, rec.Body.String())
		}
	}

	var n int
	if err := sqlDB.QueryRow(`SELECT count(*) FROM images`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("거절한 것이 %d개 저장됐다", n)
	}
}

// 필드 이름이 다르면 무엇을 받아야 할지 알 수 없다.
func TestUploadNeedsTheImageField(t *testing.T) {
	h := testHandler(t)
	rec := upload(t, h, "file", "x.png", pngBytes(t, 2, 2), "image/png")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("상태 코드 %d, 400이어야 한다", rec.Code)
	}
}
