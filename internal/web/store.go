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
	// 카테고리를 열면 이 글의 본문을 목록 위에 그대로 펼쳐 보여준다.
	CoverPostSlug string
}

// PostSummary는 목록에 보여줄 글 한 줄이다.
type PostSummary struct {
	ID        int64
	ParentID  sql.NullInt64
	Slug      string
	Title     string
	Status    string
	SortOrder int
	IsCover   bool
	// Children은 이 글에 달린 하위 글이다. nestPosts가 채운다.
	Children []PostSummary
}

// Post는 글 상세다.
type Post struct {
	ID           int64
	ParentID     sql.NullInt64
	Slug         string
	Title        string
	Body         string
	Status       string
	OriginalPath sql.NullString
	CreatedAt    sql.NullTime
	// Trail은 최상위부터 이 글이 속한 카테고리까지의 경로다.
	Trail []Category
	// Ancestors는 위에서부터 이 글의 부모까지의 글 사슬이다.
	Ancestors []PostSummary
	// Children은 이 글의 직속 하위 글이다.
	Children []PostSummary
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

// postColumns는 목록 한 줄에 필요한 컬럼이다. 표지 여부를 보려면 categories와
// 조인해야 해서 c 별칭이 있는 쿼리에서만 쓴다.
const postColumns = `p.id, p.parent_id, p.slug, p.title, p.status, p.sort_order,
	       (c.cover_post_id IS NOT NULL AND c.cover_post_id = p.id)`

func scanPostSummaries(rows *sql.Rows) ([]PostSummary, error) {
	defer rows.Close()
	var out []PostSummary
	for rows.Next() {
		var p PostSummary
		if err := rows.Scan(&p.ID, &p.ParentID, &p.Slug, &p.Title, &p.Status, &p.SortOrder, &p.IsCover); err != nil {
			return nil, fmt.Errorf("글 스캔: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PostsInCategory는 카테고리에 직접 붙은 글을 sort_order 순으로 돌려준다.
// 표지 글도 포함하되 IsCover로 표시한다. 하위 카테고리의 글은 포함하지 않는다.
//
// parent_id로 계층을 되살려 중첩된 형태로 돌려준다. 노션 계층이 카테고리 3단계보다
// 깊었던 글들은 같은 카테고리 안에서 서로 부모-자식이라, 그냥 나열하면 한 층으로
// 평평해진다.
func (s *store) PostsInCategory(categoryID int64) ([]PostSummary, error) {
	rows, err := s.db.Query(`
		SELECT `+postColumns+`
		FROM posts p
		JOIN categories c ON c.id = p.category_id
		WHERE p.category_id = ?
		ORDER BY p.sort_order, p.title`, categoryID)
	if err != nil {
		return nil, fmt.Errorf("카테고리 글 조회: %w", err)
	}
	flat, err := scanPostSummaries(rows)
	if err != nil {
		return nil, err
	}
	return nestPosts(flat), nil
}

// nestPosts는 평평한 목록을 parent_id로 묶는다. 들어온 순서를 형제 순서로 그대로 쓴다.
//
// 뿌리는 parent_id가 NULL인 글과, 부모가 이 목록 밖에 있는 글이다. 후자가 흔하다.
// 카테고리의 최상단 글들은 부모가 그 카테고리의 표지 글(보통 한 단계 위 카테고리에
// 붙어 있다)이기 때문이다.
func nestPosts(flat []PostSummary) []PostSummary {
	if len(flat) == 0 {
		return nil
	}

	idx := make(map[int64]int, len(flat))
	for i, p := range flat {
		idx[p.ID] = i
	}

	kids := make([][]int, len(flat))
	var roots []int
	for i, p := range flat {
		if p.ParentID.Valid {
			if j, ok := idx[p.ParentID.Int64]; ok && j != i {
				kids[j] = append(kids[j], i)
				continue
			}
		}
		roots = append(roots, i)
	}

	// placed는 두 가지 일을 한다: 아래에서 빠진 글을 찾는 것과, parent_id가 도는
	// 데이터에서 무한히 내려가지 않게 막는 것이다. 이미 그린 글은 자식으로 다시
	// 그리지 않는다.
	placed := make([]bool, len(flat))
	var build func(i int) PostSummary
	build = func(i int) PostSummary {
		placed[i] = true
		n := flat[i]
		n.Children = nil
		for _, k := range kids[i] {
			if placed[k] {
				continue
			}
			n.Children = append(n.Children, build(k))
		}
		return n
	}

	out := make([]PostSummary, 0, len(roots))
	for _, i := range roots {
		out = append(out, build(i))
	}

	// parent_id가 서로를 가리키며 도는 글은 어느 뿌리에서도 닿지 않는다. 그런 게
	// 생기더라도 목록에서 조용히 사라지면 안 되므로 남은 것을 뿌리로 붙인다.
	for i := range flat {
		if !placed[i] {
			out = append(out, build(i))
		}
	}
	return out
}

// ChildPosts는 글의 직속 하위 글을 sort_order 순으로 돌려준다.
// 카테고리가 달라도 포함한다. 계층은 카테고리와 별개다.
func (s *store) ChildPosts(postID int64) ([]PostSummary, error) {
	rows, err := s.db.Query(`
		SELECT `+postColumns+`
		FROM posts p
		LEFT JOIN categories c ON c.id = p.category_id
		WHERE p.parent_id = ?
		ORDER BY p.sort_order, p.title`, postID)
	if err != nil {
		return nil, fmt.Errorf("하위 글 조회: %w", err)
	}
	return scanPostSummaries(rows)
}

// PostAncestors는 위에서부터 이 글의 부모까지를 돌려준다. 자기 자신은 뺀다.
//
// depth에 상한을 둔다. posts.parent_id에는 categories와 달리 깊이를 지키는
// 트리거가 없어서, 사슬이 도는 데이터가 들어오면 재귀 CTE가 끝나지 않는다.
func (s *store) PostAncestors(postID int64) ([]PostSummary, error) {
	rows, err := s.db.Query(`
		WITH RECURSIVE up(id, depth) AS (
			SELECT parent_id, 0 FROM posts WHERE id = ? AND parent_id IS NOT NULL
			UNION ALL
			SELECT p.parent_id, up.depth + 1
			FROM posts p JOIN up ON p.id = up.id
			WHERE p.parent_id IS NOT NULL AND up.depth < 32
		)
		SELECT `+postColumns+`, up.depth
		FROM up JOIN posts p ON p.id = up.id
		LEFT JOIN categories c ON c.id = p.category_id
		ORDER BY up.depth DESC`, postID)
	if err != nil {
		return nil, fmt.Errorf("상위 글 조회: %w", err)
	}
	defer rows.Close()

	var out []PostSummary
	for rows.Next() {
		var p PostSummary
		var depth int
		if err := rows.Scan(&p.ID, &p.ParentID, &p.Slug, &p.Title, &p.Status, &p.SortOrder, &p.IsCover, &depth); err != nil {
			return nil, fmt.Errorf("상위 글 스캔: %w", err)
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
		SELECT id, parent_id, slug, title, body, status, original_path, original_created_at, category_id
		FROM posts WHERE slug = ?`, slug).
		Scan(&p.ID, &p.ParentID, &p.Slug, &p.Title, &p.Body, &p.Status,
			&p.OriginalPath, &p.CreatedAt, &categoryID)
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

	if p.Ancestors, err = s.PostAncestors(p.ID); err != nil {
		return nil, err
	}
	if p.Children, err = s.ChildPosts(p.ID); err != nil {
		return nil, err
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
