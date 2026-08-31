package admin

import (
	"database/sql"
	"log"
	"net/http"
)

// 데이터 보기 — **이 아카이브가 지금 어떤 상태인지 한 화면에서 본다.**
//
// # 왜 필요한가
//
// 글이 1,356편이라 목록을 넘겨서는 전체 모양이 안 보인다. "draft가 몇 편이고
// 어느 분류에 쏠려 있나", "글 없는 분류가 있나", "웹에서 쓴 글이 몇 편인가"
// 같은 것은 지금까지 sqlite3을 직접 열어야 알 수 있었다 — 그게 "DB를 손으로
// 열지 마라"는 원칙과 부딪힌다.
//
// # 무엇을 안 넣었나
//
// **방문자 수·조회수 같은 것은 없다.** 이 서버는 그런 것을 남기지 않고,
// 남기기 시작하면 읽는 사람을 세는 일이 된다. 여기서 세는 것은 전부
// **내가 쓴 것**이다.

// Stats는 데이터 보기 한 화면이다.
type Stats struct {
	Posts   StatusCounts   `json:"posts"`
	Sources []NamedCount   `json:"sources"`
	Years   []NamedCount   `json:"years"`
	Cats    []CategoryStat `json:"categories"`
	Images  ImageStats     `json:"images"`
	Body    BodyStats      `json:"body"`
	Orphans Orphans        `json:"orphans"`
}

type StatusCounts struct {
	Draft     int `json:"draft"`
	Unlisted  int `json:"unlisted"`
	Published int `json:"published"`
	Total     int `json:"total"`
}

type NamedCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// CategoryStat는 분류 한 줄이다. **직속 글만 센다** — 하위까지 더하면
// 상위 분류가 전부를 삼켜서 쏠림이 안 보인다.
type CategoryStat struct {
	Path      string `json:"path"`
	Posts     int    `json:"posts"`
	Drafts    int    `json:"drafts"`
	BodyBytes int    `json:"bodyBytes"`
}

type ImageStats struct {
	Count int `json:"count"`
	Bytes int `json:"bytes"`
	// Unused는 어느 글도 본문에서 쓰지 않는 이미지다. 글을 지워도 BLOB은
	// 남으므로(청소 도구가 아직 없다) 그 수가 여기서 드러난다.
	Unused int `json:"unused"`
}

type BodyStats struct {
	Bytes  int `json:"bytes"`
	Empty  int `json:"empty"`
	Median int `json:"median"`
	Max    int `json:"max"`
}

// Orphans는 손볼 곳이다. **세는 것이 아니라 짚어주는 것이 목적이다.**
type Orphans struct {
	NoCategory int `json:"noCategory"`
	NoDate     int `json:"noDate"`
	EmptyCats  int `json:"emptyCats"`
	// Native는 웹에서 쓴 글이다. 재이관이 되살리지 않는 글이라 따로 센다.
	Native int `json:"native"`
}

func (s *store) stats() (*Stats, error) {
	out := &Stats{
		Sources: []NamedCount{}, Years: []NamedCount{}, Cats: []CategoryStat{},
	}

	row := s.db.QueryRow(`
		SELECT count(*),
		       sum(status = 'draft'), sum(status = 'unlisted'), sum(status = 'published')
		FROM posts`)
	if err := row.Scan(&out.Posts.Total, &out.Posts.Draft, &out.Posts.Unlisted,
		&out.Posts.Published); err != nil {
		return nil, err
	}

	named := func(q string) ([]NamedCount, error) {
		rows, err := s.db.Query(q)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		list := []NamedCount{}
		for rows.Next() {
			var n NamedCount
			if err := rows.Scan(&n.Name, &n.Count); err != nil {
				return nil, err
			}
			list = append(list, n)
		}
		return list, rows.Err()
	}

	var err error
	if out.Sources, err = named(
		`SELECT source, count(*) FROM posts GROUP BY source ORDER BY count(*) DESC`); err != nil {
		return nil, err
	}
	// 원본 작성일 기준이다. 이관 시점이 아니라 **쓴 해**를 본다.
	if out.Years, err = named(`
		SELECT strftime('%Y', original_created_at), count(*)
		FROM posts WHERE original_created_at IS NOT NULL
		GROUP BY 1 ORDER BY 1`); err != nil {
		return nil, err
	}

	// 분류별. 경로는 3단계까지라 재귀 없이 두 번 조인하면 된다.
	rows, err := s.db.Query(`
		SELECT coalesce(g.name || ' > ', '') || coalesce(pa.name || ' > ', '') || c.name,
		       count(p.id),
		       coalesce(sum(p.status = 'draft'), 0),
		       coalesce(sum(length(p.body)), 0)
		FROM categories c
		LEFT JOIN categories pa ON pa.id = c.parent_id
		LEFT JOIN categories g  ON g.id  = pa.parent_id
		LEFT JOIN posts p ON p.category_id = c.id
		GROUP BY c.id
		ORDER BY count(p.id) DESC, c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var c CategoryStat
		if err := rows.Scan(&c.Path, &c.Posts, &c.Drafts, &c.BodyBytes); err != nil {
			return nil, err
		}
		out.Cats = append(out.Cats, c)
		if c.Posts == 0 {
			out.Orphans.EmptyCats++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 본문 크기. 중앙값은 SQLite에 함수가 없어서 정렬 후 가운데를 집는다.
	if err := s.db.QueryRow(`
		SELECT coalesce(sum(length(body)), 0), coalesce(sum(length(body) = 0), 0),
		       coalesce(max(length(body)), 0)
		FROM posts`).Scan(&out.Body.Bytes, &out.Body.Empty, &out.Body.Max); err != nil {
		return nil, err
	}
	if err := s.db.QueryRow(`
		SELECT length(body) FROM posts ORDER BY length(body)
		LIMIT 1 OFFSET (SELECT count(*) / 2 FROM posts)`).Scan(&out.Body.Median); err != nil &&
		err != sql.ErrNoRows {
		return nil, err
	}

	// 이미지. **안 쓰이는 것은 본문 전체를 훑어야 안다** — 445장이라 이만하면 된다.
	if err := s.db.QueryRow(`
		SELECT count(*), coalesce(sum(length(data)), 0) FROM images`).
		Scan(&out.Images.Count, &out.Images.Bytes); err != nil {
		return nil, err
	}
	if err := s.db.QueryRow(`
		SELECT count(*) FROM images i
		WHERE NOT EXISTS (SELECT 1 FROM posts p WHERE instr(p.body, i.sha256) > 0)`).
		Scan(&out.Images.Unused); err != nil {
		return nil, err
	}

	if err := s.db.QueryRow(`
		SELECT coalesce(sum(category_id IS NULL), 0),
		       coalesce(sum(original_created_at IS NULL), 0),
		       coalesce(sum(notion_page_id IS NULL), 0)
		FROM posts`).
		Scan(&out.Orphans.NoCategory, &out.Orphans.NoDate, &out.Orphans.Native); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.stats()
	if err != nil {
		log.Printf("admin 데이터 조회 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "데이터를 가져오지 못했다")
		return
	}
	writeJSON(w, http.StatusOK, st)
}
