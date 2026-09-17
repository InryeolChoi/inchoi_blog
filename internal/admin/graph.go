package admin

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"
)

// GraphNode는 지도 화면의 점 하나(글 하나)다.
type GraphNode struct {
	ID       int64  `json:"id"`
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Category string `json:"category"`
	// Top은 최상위 분류 이름이다. 지도(admin.js)가 이 값으로 점을 묶어
	// 자리를 잡는다. 분류가 없으면 빈 문자열.
	Top string `json:"top"`
}

// GraphEdge는 두 글 사이의 선이다. Kind가 "parent"면 글 계층(하위 글), "link"면
// 본문 안의 /p/ 링크다 — 지도가 이 둘을 다르게 그린다.
type GraphEdge struct {
	Source int64  `json:"source"`
	Target int64  `json:"target"`
	Kind   string `json:"kind"`
}

// Graph는 지도 화면이 통째로 받는 자료다.
type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// graphLinkPattern은 본문 안의 마크다운 글 링크를 잡는다. 저장에 쓰는
// pageLinkPattern(importer/links.go)과 같은 모양이지만, 여기서는 이미
// /p/{slug}로 자리잡은 뒤라 slugByPageID 없이 slug만 뽑으면 된다.
var graphLinkPattern = regexp.MustCompile(`\]\(/p/([^)\s#]+)(?:#[^)\s]*)?\)`)

// graph는 전체 글 지도를 만든다.
//
// **본문은 링크를 뽑는 자리에서만 쓰고 응답에는 담지 않는다.** 950여 편의
// 본문을 통째로 브라우저에 보내면 지도 하나 그리는데 몇 메가바이트가 오간다.
func (s *store) graph() (*Graph, error) {
	cats, err := s.categories()
	if err != nil {
		return nil, err
	}
	// categoryID → 최상위 분류 이름. Path가 "A > B > C"면 첫 조각이 최상위다.
	topByCatID := make(map[int64]string, len(cats))
	for _, c := range cats {
		top := c.Path
		if i := strings.Index(top, ">"); i >= 0 {
			top = top[:i]
		}
		topByCatID[c.ID] = strings.TrimSpace(top)
	}

	rows, err := s.db.Query(`
		SELECT p.id, p.slug, p.title, p.status, coalesce(c.name, ''), p.category_id, p.parent_id, p.body
		FROM posts p
		LEFT JOIN categories c ON c.id = p.category_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type row struct {
		id, catID, parentID                int64
		hasCat, hasParent                  bool
		slug, title, status, catName, body string
	}
	var all []row
	slugToID := map[string]int64{}
	for rows.Next() {
		var r row
		var catID, parentID sql.NullInt64
		if err := rows.Scan(&r.id, &r.slug, &r.title, &r.status, &r.catName, &catID, &parentID, &r.body); err != nil {
			return nil, err
		}
		r.catID, r.hasCat = catID.Int64, catID.Valid
		r.parentID, r.hasParent = parentID.Int64, parentID.Valid
		all = append(all, r)
		slugToID[r.slug] = r.id
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := &Graph{Nodes: make([]GraphNode, 0, len(all)), Edges: []GraphEdge{}}
	seen := map[[2]int64]bool{} // (source,target) 중복 링크를 한 번만 그린다
	for _, r := range all {
		n := GraphNode{ID: r.id, Slug: r.slug, Title: r.title, Status: r.status, Category: r.catName}
		if r.hasCat {
			n.Top = topByCatID[r.catID]
		}
		out.Nodes = append(out.Nodes, n)

		if r.hasParent {
			out.Edges = append(out.Edges, GraphEdge{Source: r.parentID, Target: r.id, Kind: "parent"})
		}
		for _, m := range graphLinkPattern.FindAllStringSubmatch(r.body, -1) {
			targetID, ok := slugToID[m[1]]
			if !ok || targetID == r.id {
				continue
			}
			key := [2]int64{r.id, targetID}
			if seen[key] {
				continue
			}
			seen[key] = true
			out.Edges = append(out.Edges, GraphEdge{Source: r.id, Target: targetID, Kind: "link"})
		}
	}
	return out, nil
}

// handleGraph는 지도 화면에 자료를 준다.
func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	g, err := s.store.graph()
	if err != nil {
		log.Printf("admin 지도 조회 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "지도를 만들지 못했다")
		return
	}
	b, err := json.Marshal(g)
	if err != nil {
		log.Printf("admin 지도 인코딩 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "지도를 만들지 못했다")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(b)
}
