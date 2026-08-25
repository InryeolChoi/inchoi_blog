package admin

import (
	"database/sql"
	"time"
)

// store는 admin이 보는 DB다.
//
// **internal/web의 store와 일부러 따로 둔다.** 공개 쪽은 draft를 어디에서도
// 안 보여주는 것이 규칙이고(notHidden), admin은 반대로 **draft가 주인공**이다.
// 같은 조회를 플래그로 갈라 쓰면 언젠가 공개 쪽에 draft가 샌다 — 두 개의 다른
// 시선이지 한 조회의 옵션이 아니다.
type store struct{ db *sql.DB }

// PostRow는 목록에 한 줄로 찍을 글이다. 본문은 안 싣는다 — 1,356편의 본문을
// 다 실으면 목록 한 번에 수십 MB가 나간다.
type PostRow struct {
	ID        int64      `json:"id"`
	Slug      string     `json:"slug"`
	Title     string     `json:"title"`
	Status    string     `json:"status"`
	Category  string     `json:"category"`
	BodyBytes int        `json:"bodyBytes"`
	UpdatedAt *time.Time `json:"updatedAt"`
	CreatedAt *time.Time `json:"createdAt"`
}

// PostDetail은 편집 폼을 채울 글 하나다.
type PostDetail struct {
	PostRow
	Body string `json:"body"`
}

// Statuses는 status가 가질 수 있는 값이다. 스키마의 CHECK과 같아야 한다.
//
//	draft    아직 안 쓴 자리      — 공개 서버에서 숨긴다
//	unlisted 아카이브 본문        — 보인다
//	published 골라 앞에 세운 글   — 보인다 (+ 나중에 홈에)
var Statuses = []string{"draft", "unlisted", "published"}

func nullTime(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

// listPosts는 글 전체를 최근 수정순으로 돌려준다.
//
// 이번 단계는 뼈대라 검색도 필터도 페이지 나누기도 없다. 다만 **limit은 건다** —
// 1,356행을 한 번에 JSON으로 말면 브라우저가 목록을 그리기 전에 멈춘다.
func (s *store) listPosts(limit int) ([]PostRow, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.slug, p.title, p.status,
		       coalesce(c.name, ''), length(p.body),
		       p.updated_at, p.original_created_at
		FROM posts p
		LEFT JOIN categories c ON c.id = p.category_id
		ORDER BY p.updated_at DESC, p.id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []PostRow{} // nil이면 JSON이 null이 된다. 빈 목록은 []여야 한다.
	for rows.Next() {
		var p PostRow
		var updated, created sql.NullTime
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Status,
			&p.Category, &p.BodyBytes, &updated, &created); err != nil {
			return nil, err
		}
		p.UpdatedAt, p.CreatedAt = nullTime(updated), nullTime(created)
		out = append(out, p)
	}
	return out, rows.Err()
}

// postBySlug는 편집할 글 하나를 본문까지 가져온다. 없으면 (nil, nil)이다.
func (s *store) postBySlug(slug string) (*PostDetail, error) {
	var p PostDetail
	var updated, created sql.NullTime
	err := s.db.QueryRow(`
		SELECT p.id, p.slug, p.title, p.status,
		       coalesce(c.name, ''), length(p.body),
		       p.updated_at, p.original_created_at, p.body
		FROM posts p
		LEFT JOIN categories c ON c.id = p.category_id
		WHERE p.slug = ?`, slug).
		Scan(&p.ID, &p.Slug, &p.Title, &p.Status,
			&p.Category, &p.BodyBytes, &updated, &created, &p.Body)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.UpdatedAt, p.CreatedAt = nullTime(updated), nullTime(created)
	return &p, nil
}

// counts는 status별 글 수다. 목록 머리에 찍어서 지금 무엇이 얼마나 있는지 보인다.
func (s *store) counts() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT status, count(*) FROM posts GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for _, st := range Statuses {
		out[st] = 0
	}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}
		out[st] = n
	}
	return out, rows.Err()
}
