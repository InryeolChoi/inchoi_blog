// Package importer는 변환된 마크다운과 이미지 파일을 DB에 넣는다.
//
// 이관은 여러 번 돌려도 결과가 같아야 한다. posts는 notion_page_id, images는
// sha256을 키로 삼아 이미 있으면 갱신한다. 두 컬럼 모두 스키마에서 UNIQUE다.
package importer

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PageMeta는 notion-page-status.csv 한 줄에서 이관에 쓰는 부분이다.
type PageMeta struct {
	PageID   string
	Title    string
	FullPath string
	Status   string
}

// LoadPageMeta는 status CSV를 읽는다.
//
// 제목에 콤마가 들어간 행이 있어서 반드시 CSV 파서로 읽어야 한다.
// 줄을 콤마로 쪼개면 컬럼이 밀린다.
func LoadPageMeta(path string) (map[string]PageMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("status CSV 열기(%s): %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("status CSV 헤더: %w", err)
	}
	col := map[string]int{}
	for i, name := range header {
		col[name] = i
	}
	for _, need := range []string{"page_id", "title", "full_path", "status"} {
		if _, ok := col[need]; !ok {
			return nil, fmt.Errorf("status CSV에 %q 컬럼이 없다", need)
		}
	}

	out := map[string]PageMeta{}
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("status CSV 읽기: %w", err)
		}
		m := PageMeta{
			PageID:   rec[col["page_id"]],
			Title:    rec[col["title"]],
			FullPath: rec[col["full_path"]],
			Status:   rec[col["status"]],
		}
		if m.PageID == "" {
			continue
		}
		out[m.PageID] = m
	}
	return out, nil
}

// Post는 posts 테이블에 넣을 한 건이다.
type Post struct {
	Slug              string
	Title             string
	Body              string
	Status            string
	Source            string
	NotionPageID      string
	OriginalPath      string
	OriginalCreatedAt *time.Time
	// SortOrder가 nil이면 기존에 sortorder 도구로 복원한 값을 보존한다.
	// 사람이 명시한 순서만 포인터로 넘겨 재이관 때도 같은 값으로 고정한다.
	SortOrder *int
}

// validStatus는 스키마의 CHECK 제약과 같은 값들이다.
// DB가 거절하기 전에 여기서 먼저 걸러 어느 페이지가 문제인지 알 수 있게 한다.
var validStatus = map[string]bool{"draft": true, "published": true, "unlisted": true}

// UpsertPost는 글 한 건을 넣거나 갱신한다. notion_page_id가 멱등 키다.
//
// created_at은 처음 넣을 때만 정하고 다시 이관해도 유지한다. updated_at만 갱신한다.
func UpsertPost(tx *sql.Tx, p Post, now time.Time) error {
	if !validStatus[p.Status] {
		return fmt.Errorf("%s: status가 draft/published/unlisted 중 하나여야 한다 (받은 값: %q)",
			p.NotionPageID, p.Status)
	}
	if p.NotionPageID == "" {
		return fmt.Errorf("%s: notion_page_id가 비었다 (재이관 멱등 키다)", p.Slug)
	}

	_, err := tx.Exec(`
		INSERT INTO posts (
			slug, title, body, status, source,
			notion_page_id, original_path, original_created_at,
			sort_order, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, COALESCE(?, 0), ?, ?)
		ON CONFLICT (notion_page_id) DO UPDATE SET
			slug                = excluded.slug,
			title               = excluded.title,
			body                = excluded.body,
			status              = excluded.status,
			original_path       = excluded.original_path,
			original_created_at = excluded.original_created_at,
			sort_order          = CASE WHEN ? IS NULL THEN posts.sort_order ELSE excluded.sort_order END,
			updated_at          = excluded.updated_at`,
		p.Slug, p.Title, p.Body, p.Status, p.Source,
		p.NotionPageID, nullString(p.OriginalPath), nullTime(p.OriginalCreatedAt),
		nullInt(p.SortOrder), now, now, nullInt(p.SortOrder))
	if err != nil {
		return fmt.Errorf("posts upsert(%s): %w", p.NotionPageID, err)
	}
	return nil
}

// Image는 images 테이블에 넣을 한 건이다.
type Image struct {
	SHA256      string
	Data        []byte
	MIME        string
	OriginalURL string
}

// UpsertImage는 이미지 한 건을 넣거나 갱신한다. sha256이 멱등 키다.
func UpsertImage(tx *sql.Tx, img Image, now time.Time) error {
	if img.SHA256 == "" {
		return fmt.Errorf("sha256이 비었다")
	}
	if len(img.Data) == 0 {
		return fmt.Errorf("%s: 이미지 바이트가 없다", img.SHA256)
	}

	_, err := tx.Exec(`
		INSERT INTO images (sha256, data, mime, original_url, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (sha256) DO UPDATE SET
			data         = excluded.data,
			mime         = excluded.mime,
			original_url = excluded.original_url`,
		img.SHA256, img.Data, img.MIME, nullString(img.OriginalURL), now)
	if err != nil {
		return fmt.Errorf("images upsert(%s): %w", img.SHA256, err)
	}
	return nil
}

// mimeByExt는 저장된 이미지 파일 확장자에 대응하는 MIME 타입이다.
var mimeByExt = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
	".bmp":  "image/bmp",
	".avif": "image/avif",
	".heic": "image/heic",
}

// MIMEForFile은 파일 이름의 확장자로 MIME 타입을 고른다.
// 모르는 확장자면 두 번째 반환값이 false다. 임의로 추측하지 않는다.
func MIMEForFile(name string) (string, bool) {
	mime, ok := mimeByExt[strings.ToLower(filepath.Ext(name))]
	return mime, ok
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

func nullInt(n *int) any {
	if n == nil {
		return nil
	}
	return *n
}
