package web

import (
	"database/sql"
	"fmt"
)

// Category는 목록에 보여줄 카테고리 한 줄이다.
type Category struct {
	ID       int64
	Name     string
	Slug     string
	ParentID sql.NullInt64
	// PostCount는 이 카테고리와 그 아래 전체에 붙은 글 수다.
	PostCount int
	// CoverPostSlug는 이 카테고리의 표지 글이다. 없으면 빈 문자열이다.
	CoverPostSlug string
}

// PostSummary는 목록에 보여줄 글 한 줄이다.
type PostSummary struct {
	Slug      string
	Title     string
	Status    string
	SortOrder int
	IsCover   bool
}

// Post는 글 상세다.
type Post struct {
	Slug         string
	Title        string
	Body         string
	Status       string
	OriginalPath sql.NullString
	CreatedAt    sql.NullTime
	// Trail은 최상위부터 이 글이 속한 카테고리까지의 경로다.
	Trail []Category
}

// Image는 이미지 한 장이다.
type Image struct {
	Data []byte
	MIME string
}

// store는 DB 조회를 모아둔 것이다.
type store struct{ db *sql.DB }

// subtreePostCount는 카테고리와 그 하위 전체의 글 수를 세는 SQL 조각이다.
// 카테고리는 3단계까지라 재귀 CTE로 아래를 훑는다.
const subtreePostCount = `
	(SELECT count(*) FROM posts p WHERE p.category_id IN (
		WITH RECURSIVE sub(id) AS (
			SELECT c2.id FROM categories c2 WHERE c2.id = c.id
			UNION ALL
			SELECT k.id FROM categories k JOIN sub ON k.parent_id = sub.id
		) SELECT id FROM sub
	))`

// coverSlug는 카테고리의 표지 글 slug를 가져오는 SQL 조각이다.
const coverSlug = `(SELECT p.slug FROM posts p WHERE p.id = c.cover_post_id)`

func (s *store) scanCategories(rows *sql.Rows) ([]Category, error) {
	defer rows.Close()
	var out []Category
	for rows.Next() {
		var c Category
		var cover sql.NullString
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.ParentID, &c.PostCount, &cover); err != nil {
			return nil, fmt.Errorf("카테고리 스캔: %w", err)
		}
		c.CoverPostSlug = cover.String
		out = append(out, c)
	}
	return out, rows.Err()
}

// TopCategories는 최상위 카테고리를 sort_order 순으로 돌려준다.
func (s *store) TopCategories() ([]Category, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.name, c.slug, c.parent_id, ` + subtreePostCount + `, ` + coverSlug + `
		FROM categories c
		WHERE c.parent_id IS NULL
		ORDER BY c.sort_order, c.name`)
	if err != nil {
		return nil, fmt.Errorf("최상위 카테고리 조회: %w", err)
	}
	return s.scanCategories(rows)
}

// ChildCategories는 주어진 카테고리의 바로 아래 자식들을 돌려준다.
func (s *store) ChildCategories(parentID int64) ([]Category, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.name, c.slug, c.parent_id, `+subtreePostCount+`, `+coverSlug+`
		FROM categories c
		WHERE c.parent_id = ?
		ORDER BY c.sort_order, c.name`, parentID)
	if err != nil {
		return nil, fmt.Errorf("하위 카테고리 조회: %w", err)
	}
	return s.scanCategories(rows)
}

// CategoryBySlug는 slug로 카테고리를 찾는다. 부모까지 맞는지 함께 확인해서
// /a/b 경로의 b가 정말 a의 자식일 때만 찾히게 한다.
func (s *store) CategoryBySlug(slug string, parentID sql.NullInt64) (*Category, error) {
	q := `
		SELECT c.id, c.name, c.slug, c.parent_id, ` + subtreePostCount + `, ` + coverSlug + `
		FROM categories c
		WHERE c.slug = ? AND c.parent_id IS NULL`
	args := []any{slug}
	if parentID.Valid {
		q = `
		SELECT c.id, c.name, c.slug, c.parent_id, ` + subtreePostCount + `, ` + coverSlug + `
		FROM categories c
		WHERE c.slug = ? AND c.parent_id = ?`
		args = append(args, parentID.Int64)
	}

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("카테고리 조회(%s): %w", slug, err)
	}
	cats, err := s.scanCategories(rows)
	if err != nil {
		return nil, err
	}
	if len(cats) == 0 {
		return nil, nil
	}
	return &cats[0], nil
}

// PostsInCategory는 카테고리에 직접 붙은 글을 sort_order 순으로 돌려준다.
// 표지 글도 포함하되 IsCover로 표시한다. 하위 카테고리의 글은 포함하지 않는다.
func (s *store) PostsInCategory(categoryID int64) ([]PostSummary, error) {
	rows, err := s.db.Query(`
		SELECT p.slug, p.title, p.status, p.sort_order,
		       (c.cover_post_id IS NOT NULL AND c.cover_post_id = p.id)
		FROM posts p
		JOIN categories c ON c.id = p.category_id
		WHERE p.category_id = ?
		ORDER BY p.sort_order, p.title`, categoryID)
	if err != nil {
		return nil, fmt.Errorf("카테고리 글 조회: %w", err)
	}
	defer rows.Close()

	var out []PostSummary
	for rows.Next() {
		var p PostSummary
		if err := rows.Scan(&p.Slug, &p.Title, &p.Status, &p.SortOrder, &p.IsCover); err != nil {
			return nil, fmt.Errorf("글 스캔: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PostBySlug는 글 하나를 가져온다. 없으면 nil을 돌려준다.
func (s *store) PostBySlug(slug string) (*Post, error) {
	var p Post
	var categoryID sql.NullInt64
	err := s.db.QueryRow(`
		SELECT slug, title, body, status, original_path, original_created_at, category_id
		FROM posts WHERE slug = ?`, slug).
		Scan(&p.Slug, &p.Title, &p.Body, &p.Status, &p.OriginalPath, &p.CreatedAt, &categoryID)
	switch {
	case err == sql.ErrNoRows:
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("글 조회(%s): %w", slug, err)
	}

	if categoryID.Valid {
		trail, err := s.CategoryTrail(categoryID.Int64)
		if err != nil {
			return nil, err
		}
		p.Trail = trail
	}
	return &p, nil
}

// CategoryTrail은 최상위부터 주어진 카테고리까지의 경로를 돌려준다.
func (s *store) CategoryTrail(categoryID int64) ([]Category, error) {
	rows, err := s.db.Query(`
		WITH RECURSIVE up(id, name, slug, parent_id, depth) AS (
			SELECT id, name, slug, parent_id, 0 FROM categories WHERE id = ?
			UNION ALL
			SELECT c.id, c.name, c.slug, c.parent_id, up.depth + 1
			FROM categories c JOIN up ON c.id = up.parent_id
		)
		SELECT id, name, slug, parent_id FROM up ORDER BY depth DESC`, categoryID)
	if err != nil {
		return nil, fmt.Errorf("카테고리 경로 조회: %w", err)
	}
	defer rows.Close()

	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.ParentID); err != nil {
			return nil, fmt.Errorf("카테고리 경로 스캔: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ImageBySHA256은 이미지 바이트와 MIME 타입을 가져온다. 없으면 nil을 돌려준다.
func (s *store) ImageBySHA256(sha string) (*Image, error) {
	var img Image
	err := s.db.QueryRow(`SELECT data, mime FROM images WHERE sha256 = ?`, sha).
		Scan(&img.Data, &img.MIME)
	switch {
	case err == sql.ErrNoRows:
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("이미지 조회(%s): %w", sha, err)
	}
	return &img, nil
}
