package admin

import (
	"database/sql"
	"sort"
	"time"

	"github.com/inryeol/blog/internal/curation"
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
//
// 목록에 없는 것들이 여기 다 있어야 한다. 글 하나를 실제로 고치려면 본문만으로
// 부족하다 — 어느 분류에 붙어 있고, 어느 글의 자식이고, 형제 사이 몇 번째인지가
// 전부 posts의 다른 칸에 있다.
type PostDetail struct {
	PostRow
	Body string `json:"body"`

	// Rev는 **덮어쓰기 사고를 막는 표**다. 불러올 때 준 값을 저장할 때 그대로
	// 돌려받아, DB의 지금 값과 다르면 거절한다. 두 탭에서 같은 글을 열어
	// 각각 저장하면 나중 것이 앞의 것을 통째로 지우는데, 그게 조용히 일어난다.
	//
	// updated_at을 **글자 그대로** 쓴다. time.Time으로 오가면 JSON 왕복에서
	// 정밀도가 깎여 멀쩡한 저장이 거절당한다.
	Rev string `json:"rev"`

	// Source는 이 글이 어디서 왔는지다. notion이면 재이관이 본문을 덮으므로
	// 화면이 경고를 띄운다.
	Source       string  `json:"source"`
	NotionPageID *string `json:"notionPageId"`
	// Managed는 사람이 정한 예외(internal/curation)가 이 글의 제목·날짜·순서나
	// 본문을 관리하고 있는지다. 그렇다면 여기서 고쳐도 다음 재이관이 되돌린다.
	Managed bool `json:"managed"`

	CategoryID *int64 `json:"categoryId"`
	ParentID   *int64 `json:"parentId"`
	// ParentSlug는 부모 글의 slug다. 폼이 id가 아니라 이걸 주고받는다 —
	// 사람이 알아볼 수 있는 값이어야 잘못 붙였을 때 눈에 띈다.
	ParentSlug  string `json:"parentSlug"`
	ParentTitle string `json:"parentTitle"`
	SortOrder   int    `json:"sortOrder"`

	PublishedAt *time.Time `json:"publishedAt"`
}

// CategoryRow는 메타 패널의 분류 선택지 하나다.
type CategoryRow struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Slug  string `json:"slug"`
	Depth int    `json:"depth"`
	Path  string `json:"path"`
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
	var updated, created, published sql.NullTime
	var notionID sql.NullString
	var categoryID, parentID sql.NullInt64
	err := s.db.QueryRow(`
		SELECT p.id, p.slug, p.title, p.status,
		       coalesce(c.name, ''), length(p.body),
		       p.updated_at, p.original_created_at, p.body,
		       cast(p.updated_at AS TEXT),
		       p.source, p.notion_page_id,
		       p.category_id, p.parent_id, p.sort_order, p.published_at,
		       coalesce(pp.slug, ''), coalesce(pp.title, '')
		FROM posts p
		LEFT JOIN categories c ON c.id = p.category_id
		LEFT JOIN posts pp ON pp.id = p.parent_id
		WHERE p.slug = ?`, slug).
		Scan(&p.ID, &p.Slug, &p.Title, &p.Status,
			&p.Category, &p.BodyBytes, &updated, &created, &p.Body,
			&p.Rev, &p.Source, &notionID,
			&categoryID, &parentID, &p.SortOrder, &published,
			&p.ParentSlug, &p.ParentTitle)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.UpdatedAt, p.CreatedAt = nullTime(updated), nullTime(created)
	p.PublishedAt = nullTime(published)
	if notionID.Valid {
		p.NotionPageID = &notionID.String
		p.Managed = curationManages(notionID.String)
	}
	if categoryID.Valid {
		p.CategoryID = &categoryID.Int64
	}
	if parentID.Valid {
		p.ParentID = &parentID.Int64
	}
	return &p, nil
}

// curationManages는 사람이 정한 예외가 이 글을 관리하고 있는지다.
//
// **여기 걸린 글은 admin에서 고쳐도 다음 `import -db`가 되돌린다.** 제목·날짜·
// 순서는 curation의 표가 이기고 본문은 변환 결과가 통째로 덮는다. 화면이
// 그걸 미리 말해줘야 한다 — 저장은 되는데 다음 이관에 사라지는 것이
// 가장 나쁜 결과다.
func curationManages(pageID string) bool {
	if _, ok := curation.PostMetadataByID()[pageID]; ok {
		return true
	}
	if _, ok := curation.PostTitleByID()[pageID]; ok {
		return true
	}
	for _, e := range curation.BodyEdits {
		if e.NotionPageID == pageID {
			return true
		}
	}
	for _, a := range curation.BodyAppends {
		if a.NotionPageID == pageID {
			return true
		}
	}
	return false
}

// categories는 메타 패널이 쓸 분류 목록이다. 87개뿐이라 전부 읽어 Go에서
// 경로를 만든다 — web.NavTree가 한 번의 조회로 트리를 그리는 것과 같다.
func (s *store) categories() ([]CategoryRow, error) {
	rows, err := s.db.Query(`SELECT id, name, slug, parent_id, sort_order FROM categories`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type node struct {
		row    CategoryRow
		parent sql.NullInt64
		order  int
	}
	byID := map[int64]*node{}
	var all []*node
	for rows.Next() {
		n := &node{}
		if err := rows.Scan(&n.row.ID, &n.row.Name, &n.row.Slug, &n.parent, &n.order); err != nil {
			return nil, err
		}
		byID[n.row.ID] = n
		all = append(all, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 경로는 이름을 위로 거슬러 이어붙인다. 깊이가 3으로 막혀 있으므로
	// (마이그레이션 002의 트리거) 여기서 도는 일은 없지만, 그래도 횟수를 센다 —
	// 트리거가 지키는 것을 코드가 믿고 도는 자리를 만들지 않는다.
	for _, n := range all {
		path, cur, depth := n.row.Name, n, 0
		for cur.parent.Valid && depth < 8 {
			p, ok := byID[cur.parent.Int64]
			if !ok {
				break
			}
			path, cur, depth = p.row.Name+" > "+path, p, depth+1
		}
		n.row.Depth, n.row.Path = depth, path
	}

	out := make([]CategoryRow, 0, len(all))
	for _, n := range all {
		out = append(out, n.row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
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
