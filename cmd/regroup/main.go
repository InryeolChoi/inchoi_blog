// regroup은 노션에서 온 최상위 카테고리들을 더 큰 분류 밑으로 묶는다.
//
//	dev > Language > 프로그래밍 언어
//	└ 새로 만드는 것   └ 노션 최상위    └ 노션 2단계 (그대로 딸려 내려감)
//
// 노션 워크스페이스의 최상위 구조는 블로그의 분류로 쓰기엔 너무 잘다. cmd/categorize가
// 경로에서 뽑아낸 19개 위에 사람이 정한 8개를 얹는다.
//
// 이 도구는 카테고리를 지우지 않는다. 이름과 slug는 그대로 두고 parent_id만 바꾼다
// (이름을 바꾸라고 지정한 것만 예외).
//
// 기본은 미리보기다. 실제로 DB를 고치려면 -apply를 준다.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/inryeol/blog/internal/curation"
	"github.com/inryeol/blog/internal/db"
)

const rule = "════════════════════════════════════════════════════════════════════════"

// group은 새로 만들 최상위 분류 하나다.
type group struct {
	slug string
	name string
	// members는 이 분류 밑으로 내릴 기존 카테고리 이름들이다.
	members []string
}

// rename은 옮기면서 이름과 slug까지 바꾸는 경우다.
type rename struct {
	fromName string
	toName   string
	toSlug   string
}

// groups는 새 최상위 분류와 그 아래로 내릴 기존 카테고리다.
// 순서가 곧 sort_order다.
var groups = []group{
	{slug: "intro", name: "소개", members: []string{"최인렬 (Inryeol Choi)"}},
	{slug: "algorithm", name: "알고리즘", members: []string{
		"알고리즘: 이론", "알고리즘: 실전",
	}},
	{slug: "cs-theory", name: "CS 이론", members: []string{
		"운영체제", "네트워크", "컴퓨터 시스템", "데이터베이스", "가상화기술",
	}},
	{slug: "dev", name: "개발", members: []string{
		"Language", "소프트스킬", "모바일 프로그래밍", "웹 프로그래밍", "리눅스 & 쉘",
	}},
	{slug: "math-stat", name: "수학 & 통계", members: []string{
		"수학 & 통계", "머신러닝 & 딥러닝",
	}},
	{slug: "project", name: "프로젝트", members: []string{
		"école 42", "Projects", "전주 데이터분석",
	}},
	{slug: "career", name: "커리어", members: []string{"취업 준비"}},
	{slug: "life", name: "라이프", members: nil},
}

// renames는 옮기면서 이름도 바꾸는 카테고리다.
var renames = []rename{
	{fromName: "소프트스킬", toName: "tooling", toSlug: "tooling"},
}

// category는 DB에 있는 카테고리 한 줄이다.
type category struct {
	id         int64
	name       string
	slug       string
	parentID   sql.NullInt64
	sourceName sql.NullString
	children   int
	// grandchildren은 자식의 자식 수다. 이게 0보다 크면 서브트리 높이가 2라서
	// 새 최상위 밑으로 넣으면 4단계가 된다.
	grandchildren int
	posts         int
}

func main() {
	dbPath := flag.String("db", "blog.db", "SQLite 파일 경로")
	apply := flag.Bool("apply", false, "실제로 DB를 고친다. 없으면 미리보기만")
	flag.Parse()

	sqlDB, err := db.Open(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	cat, err := loadCategories(sqlDB)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := checkPlan(cat); err != nil {
		fmt.Fprintf(os.Stderr, "계획 검사 실패: %v\n", err)
		os.Exit(1)
	}
	printPlan(cat)

	if !*apply {
		fmt.Println("\n미리보기다. 실제로 고치려면 -apply 를 붙여 다시 실행해라.")
		return
	}

	if err := applyGroups(sqlDB, cat); err != nil {
		fmt.Fprintf(os.Stderr, "\n적용 실패: %v\n", err)
		os.Exit(1)
	}
	if err := verify(sqlDB); err != nil {
		fmt.Fprintf(os.Stderr, "\n검증 실패: %v\n", err)
		os.Exit(1)
	}
}

// catalog는 카테고리를 이름과 slug 두 가지로 찾을 수 있게 담는다.
type catalog struct {
	byName   map[string]*category
	bySlug   map[string]*category
	bySource map[string]*category // source_name = 경로에서 온 이름 (사람이 안 바꾼다)
}

// member는 배정표의 이름으로 실제 카테고리를 찾는다.
//
// 두 번 돌려도 같은 걸 찾아야 한다:
//   - 이름을 바꾼 뒤에는 옛 이름으로 못 찾으므로 바뀐 slug로도 찾아본다.
//   - 새 최상위 분류가 기존 카테고리와 이름이 겹칠 수 있다("수학 & 통계"가 그렇다).
//     이름 색인에서 새 분류를 빼두었으므로 원래 카테고리가 잡힌다.
func (c catalog) member(name string) (*category, bool) {
	// source_name이 가장 안정적이다. 사람이 이름을 바꿔도 그대로다.
	if cat, ok := c.bySource[name]; ok {
		return cat, true
	}
	if cat, ok := c.byName[name]; ok {
		return cat, true
	}
	for _, r := range renames {
		if r.fromName == name {
			if cat, ok := c.bySlug[r.toSlug]; ok {
				return cat, true
			}
		}
	}
	return nil, false
}

func loadCategories(sqlDB *sql.DB) (*catalog, error) {
	rows, err := sqlDB.Query(`
		SELECT c.id, c.name, c.slug, c.parent_id, c.source_name,
		       (SELECT count(*) FROM categories k WHERE k.parent_id = c.id),
		       (SELECT count(*) FROM categories g
		          JOIN categories k ON g.parent_id = k.id
		         WHERE k.parent_id = c.id),
		       (SELECT count(*) FROM posts p WHERE p.category_id = c.id)
		FROM categories c`)
	if err != nil {
		return nil, fmt.Errorf("categories 조회: %w", err)
	}
	defer rows.Close()

	newSlugs := map[string]bool{}
	for _, g := range groups {
		newSlugs[g.slug] = true
	}

	cat := &catalog{
		byName:   map[string]*category{},
		bySlug:   map[string]*category{},
		bySource: map[string]*category{},
	}
	for rows.Next() {
		var c category
		if err := rows.Scan(&c.id, &c.name, &c.slug, &c.parentID, &c.sourceName,
			&c.children, &c.grandchildren, &c.posts); err != nil {
			return nil, fmt.Errorf("categories 스캔: %w", err)
		}
		row := c
		cat.bySlug[row.slug] = &row
		if row.sourceName.Valid {
			cat.bySource[row.sourceName.String] = &row
		}
		// 새 최상위 분류는 이름 색인에 넣지 않는다. 기존 카테고리와 이름이 겹치면
		// 배정표의 이름이 엉뚱하게 새 분류를 가리키게 된다.
		if !newSlugs[row.slug] {
			cat.byName[row.name] = &row
		}
	}
	return cat, rows.Err()
}

// checkPlan은 계획이 실제 DB와 맞는지 미리 본다.
// 없는 카테고리를 옮기려 하거나, 옮길 대상을 빠뜨렸으면 여기서 멈춘다.
func checkPlan(cat *catalog) error {
	planned := map[string]string{} // 기존 이름 → 새 부모 slug
	newSlugs := map[string]bool{}
	for _, g := range groups {
		newSlugs[g.slug] = true
		for _, m := range g.members {
			if prev, dup := planned[m]; dup {
				return fmt.Errorf("%q가 %q와 %q 두 곳에 배정돼 있다", m, prev, g.slug)
			}
			planned[m] = g.slug
		}
	}

	var missing []string
	for m := range planned {
		if _, ok := cat.member(m); !ok {
			missing = append(missing, m)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("배정표에 있는데 DB에 없는 카테고리: %s", strings.Join(missing, ", "))
	}

	// 지금 최상위인데 배정표에 없는 것이 있으면 알려준다. 조용히 두면
	// 새 구조 밖에 혼자 남는다.
	var unassigned []string
	for name, c := range cat.byName {
		if c.parentID.Valid || newSlugs[c.slug] {
			continue
		}
		if _, ok := planned[name]; !ok {
			unassigned = append(unassigned, name)
		}
	}
	if len(unassigned) > 0 {
		sort.Strings(unassigned)
		return fmt.Errorf("최상위인데 배정표에 없는 카테고리: %s", strings.Join(unassigned, ", "))
	}

	// 이름을 바꾸려는 대상이 실제로 있는지 (이미 바뀐 뒤면 새 slug로 찾힌다)
	for _, r := range renames {
		if _, ok := cat.member(r.fromName); !ok {
			return fmt.Errorf("이름을 바꾸려는 카테고리가 없다: %q", r.fromName)
		}
	}

	// 서브트리 높이가 1을 넘으면 3단계 안에 못 넣는다.
	// (새 최상위 + 이 카테고리 + 그 자식 = 3단계가 상한)
	// 트리거도 막지만, 여기서 먼저 걸러야 어느 카테고리가 문제인지 알 수 있다.
	var tooDeep []string
	for m := range planned {
		c, _ := cat.member(m)
		if c.grandchildren > 0 {
			tooDeep = append(tooDeep, fmt.Sprintf("%s(손자 %d개)", m, c.grandchildren))
		}
	}
	if len(tooDeep) > 0 {
		sort.Strings(tooDeep)
		return fmt.Errorf("손자까지 있어 3단계 안에 못 넣는 카테고리: %s", strings.Join(tooDeep, ", "))
	}

	// moves가 가리키는 양쪽이 다 있는지, 그리고 옮겨도 3단계를 넘지 않는지.
	for _, mv := range curation.Moves {
		c, ok := cat.bySource[mv.SourceName]
		if !ok {
			return fmt.Errorf("옮기려는 카테고리가 없다: source_name %q", mv.SourceName)
		}
		if _, ok := cat.bySlug[mv.ToSlug]; !ok {
			return fmt.Errorf("%q를 옮길 부모 카테고리가 없다: slug %q", mv.SourceName, mv.ToSlug)
		}
		if !newSlugs[mv.ToSlug] {
			return fmt.Errorf("%q의 새 부모 %q가 최상위 분류가 아니다", mv.SourceName, mv.ToSlug)
		}
		if c.grandchildren > 0 {
			return fmt.Errorf("%q는 손자까지 있어 옮기면 4단계가 된다", mv.SourceName)
		}
	}

	// covers가 가리키는 카테고리가 있는지. 글은 DB를 봐야 알 수 있어 여기선 안 본다.
	for _, cv := range curation.Covers {
		if _, ok := cat.bySlug[cv.Slug]; !ok {
			return fmt.Errorf("표지를 붙일 카테고리가 없다: slug %q", cv.Slug)
		}
	}
	return nil
}

func printPlan(cat *catalog) {
	fmt.Println(rule)
	fmt.Println("카테고리 재구성 계획")
	fmt.Println(rule)

	renameBy := map[string]rename{}
	for _, r := range renames {
		renameBy[r.fromName] = r
	}

	totalMembers, totalChildren, totalPosts := 0, 0, 0
	fmt.Println()
	for _, g := range groups {
		fmt.Printf("%s  (/%s)\n", g.name, g.slug)
		if len(g.members) == 0 {
			fmt.Println("    (아직 소속 카테고리 없음)")
			continue
		}
		for _, m := range g.members {
			c, _ := cat.member(m)
			totalMembers++
			totalChildren += c.children
			totalPosts += c.posts
			label := c.name
			if r, ok := renameBy[m]; ok && c.name != r.toName {
				label = fmt.Sprintf("%s → %s (slug: %s → %s)", m, r.toName, c.slug, r.toSlug)
			}
			fmt.Printf("    └ %s   [자식 %d개, 직접 붙은 글 %d건]\n", label, c.children, c.posts)
		}
	}

	fmt.Printf("\n새로 만들 최상위 : %d개\n", len(groups))
	fmt.Printf("2단계로 내릴 것  : %d개\n", totalMembers)
	fmt.Printf("3단계가 될 것    : %d개 (딸려 내려가는 하위 카테고리)\n", totalChildren)
	fmt.Printf("이름 변경        : %d개\n", len(renames))

	if len(curation.Moves) > 0 {
		fmt.Printf("\n■ 사람이 정한 카테고리 이동 %d건\n", len(curation.Moves))
		for _, mv := range curation.Moves {
			c := cat.bySource[mv.SourceName]
			from := "(최상위)"
			if c.parentID.Valid {
				for _, x := range cat.byName {
					if x.id == c.parentID.Int64 {
						from = x.name
						break
					}
				}
			}
			fmt.Printf("      %s : %s → /%s   [글 %d건]\n", c.name, from, mv.ToSlug, c.posts)
			fmt.Printf("          %s\n", mv.Why)
		}
	}

	if len(curation.Covers) > 0 {
		fmt.Printf("\n■ 사람이 만든 분류의 표지 글 %d건\n", len(curation.Covers))
		for _, cv := range curation.Covers {
			fmt.Printf("      /%s ← %s\n", cv.Slug, cv.NotionPageID)
			fmt.Printf("          %s\n", cv.Why)
		}
	}

	if len(curation.DropCategories) > 0 {
		fmt.Printf("\n■ 없앨 카테고리 %d개\n", len(curation.DropCategories))
		for _, dc := range curation.DropCategories {
			c, ok := cat.bySource[dc.SourceName]
			if !ok {
				fmt.Printf("      %s   (이미 없다)\n", dc.SourceName)
				continue
			}
			fmt.Printf("      %s   [글 %d건, 하위 분류 %d개]\n", c.name, c.posts, c.children)
			fmt.Printf("          %s\n", dc.Why)
			if c.posts > 0 || c.children > 0 {
				fmt.Println("          ← 딸린 게 있어 지우지 않는다. 먼저 categorize를 돌려라")
			}
		}
	}
}

func applyGroups(sqlDB *sql.DB, cat *catalog) error {
	tx, err := sqlDB.Begin()
	if err != nil {
		return fmt.Errorf("트랜잭션 시작: %w", err)
	}
	defer tx.Rollback()

	// 1) 새 최상위 분류를 넣는다. slug가 멱등 키다.
	groupID := map[string]int64{}
	for i, g := range groups {
		_, err := tx.Exec(`
			INSERT INTO categories (parent_id, name, slug, sort_order)
			VALUES (NULL, ?, ?, ?)
			ON CONFLICT (slug) DO UPDATE SET
				name       = excluded.name,
				parent_id  = NULL,
				sort_order = excluded.sort_order`,
			g.name, g.slug, i)
		if err != nil {
			return fmt.Errorf("최상위 분류 %q: %w", g.slug, err)
		}
		var id int64
		if err := tx.QueryRow(`SELECT id FROM categories WHERE slug = ?`, g.slug).Scan(&id); err != nil {
			return fmt.Errorf("최상위 분류 id 조회(%s): %w", g.slug, err)
		}
		groupID[g.slug] = id
	}

	// 2) 이름을 바꿀 것부터 처리한다. 옮긴 뒤에 하면 이름으로 못 찾는다.
	for _, r := range renames {
		c, ok := cat.member(r.fromName)
		if !ok {
			return fmt.Errorf("이름을 바꾸려는 카테고리가 없다: %q", r.fromName)
		}
		if c.name == r.toName && c.slug == r.toSlug {
			continue // 이미 바뀌어 있다
		}
		if _, err := tx.Exec(
			`UPDATE categories SET name = ?, slug = ? WHERE id = ?`, r.toName, r.toSlug, c.id); err != nil {
			return fmt.Errorf("이름 변경(%s → %s): %w", r.fromName, r.toName, err)
		}
	}

	// 3) 기존 카테고리를 새 분류 밑으로 옮긴다.
	//    트리거가 깊이를 검사하므로, 3단계를 넘기면 여기서 에러가 난다.
	moved := 0
	for _, g := range groups {
		for j, m := range g.members {
			c, ok := cat.member(m)
			if !ok {
				return fmt.Errorf("옮기려는 카테고리가 없다: %q", m)
			}
			if c.parentID.Valid && c.parentID.Int64 == groupID[g.slug] {
				continue // 이미 그 밑에 있다
			}
			if _, err := tx.Exec(
				`UPDATE categories SET parent_id = ?, sort_order = ? WHERE id = ?`,
				groupID[g.slug], j, c.id); err != nil {
				return fmt.Errorf("%q를 %q 밑으로 이동: %w", m, g.slug, err)
			}
			moved++
		}
	}

	// 4) 사람이 정한 개별 이동. groups가 끝난 뒤에 해야 새 부모 id가 다 있다.
	handMoved := 0
	for _, mv := range curation.Moves {
		c, ok := cat.bySource[mv.SourceName]
		if !ok {
			return fmt.Errorf("옮기려는 카테고리가 없다: source_name %q", mv.SourceName)
		}
		parentID, ok := groupID[mv.ToSlug]
		if !ok {
			return fmt.Errorf("%q의 새 부모 %q를 못 찾았다", mv.SourceName, mv.ToSlug)
		}
		if c.parentID.Valid && c.parentID.Int64 == parentID {
			continue // 이미 그 밑에 있다
		}
		if _, err := tx.Exec(
			`UPDATE categories SET parent_id = ? WHERE id = ?`, parentID, c.id); err != nil {
			return fmt.Errorf("%q를 %q 밑으로 이동: %w", mv.SourceName, mv.ToSlug, err)
		}
		handMoved++
	}

	// 5) 사람이 만든 분류의 표지 글. notion_page_id가 멱등 키다.
	coverSet := 0
	for _, cv := range curation.Covers {
		var postID int64
		switch err := tx.QueryRow(
			`SELECT id FROM posts WHERE notion_page_id = ?`, cv.NotionPageID).Scan(&postID); err {
		case nil:
		case sql.ErrNoRows:
			return fmt.Errorf("표지로 쓸 글이 없다: notion_page_id %s", cv.NotionPageID)
		default:
			return fmt.Errorf("표지 글 조회(%s): %w", cv.NotionPageID, err)
		}

		// cover_post_id는 UNIQUE다. 한 글은 한 카테고리의 표지만 될 수 있다.
		// 그 글이 노션 최상위 페이지라면 categorize가 이미 자기 카테고리에 붙여뒀다.
		// 새로 만드는 게 아니라 옮기는 것이므로 옛 자리를 먼저 비운다.
		if _, err := tx.Exec(
			`UPDATE categories SET cover_post_id = NULL
			 WHERE cover_post_id = ? AND slug <> ?`, postID, cv.Slug); err != nil {
			return fmt.Errorf("옛 표지 자리 비우기(%s): %w", cv.Slug, err)
		}

		res, err := tx.Exec(`
			UPDATE categories SET cover_post_id = ?
			WHERE slug = ? AND (cover_post_id IS NULL OR cover_post_id <> ?)`,
			postID, cv.Slug, postID)
		if err != nil {
			return fmt.Errorf("표지 지정(%s): %w", cv.Slug, err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			coverSet++
		}
	}

	// 6) 사람이 없애기로 한 카테고리. 글도 자식도 없을 때만 지운다.
	//    딸린 게 남아 있으면 조용히 잃지 않도록 에러로 멈춘다.
	dropped := 0
	for _, dc := range curation.DropCategories {
		var id int64
		switch err := tx.QueryRow(
			`SELECT id FROM categories WHERE source_name = ?`, dc.SourceName).Scan(&id); err {
		case nil:
		case sql.ErrNoRows:
			continue // 이미 지워졌다
		default:
			return fmt.Errorf("지울 카테고리 조회(%s): %w", dc.SourceName, err)
		}

		var posts, kids int
		if err := tx.QueryRow(
			`SELECT (SELECT count(*) FROM posts p WHERE p.category_id = c.id),
			        (SELECT count(*) FROM categories k WHERE k.parent_id = c.id)
			 FROM categories c WHERE c.id = ?`, id).Scan(&posts, &kids); err != nil {
			return fmt.Errorf("딸린 것 조회(%s): %w", dc.SourceName, err)
		}
		if posts > 0 || kids > 0 {
			return fmt.Errorf(
				"카테고리 %q를 지우려는데 글 %d건, 하위 분류 %d개가 남아 있다. "+
					"먼저 curation.PostMoves로 옮겨라", dc.SourceName, posts, kids)
		}
		if _, err := tx.Exec(`DELETE FROM categories WHERE id = ?`, id); err != nil {
			return fmt.Errorf("카테고리 삭제(%s): %w", dc.SourceName, err)
		}
		dropped++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("커밋: %w", err)
	}
	fmt.Printf("\n최상위 분류 %d개 확보, %d개 카테고리를 옮겼다.\n", len(groups), moved)
	fmt.Printf("사람이 정한 개별 이동 %d건, 표지 지정 %d건, 카테고리 삭제 %d개.\n",
		handMoved, coverSet, dropped)
	return nil
}

func verify(sqlDB *sql.DB) error {
	fmt.Println()
	fmt.Println(rule)
	fmt.Println("검증")
	fmt.Println(rule)

	depths := map[int]int{}
	rows, err := sqlDB.Query(`
		WITH RECURSIVE d(id, depth) AS (
			SELECT id, 1 FROM categories WHERE parent_id IS NULL
			UNION ALL
			SELECT c.id, d.depth + 1 FROM categories c JOIN d ON c.parent_id = d.id
		)
		SELECT depth, count(*) FROM d GROUP BY depth ORDER BY depth`)
	if err != nil {
		return fmt.Errorf("깊이 조회: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var depth, n int
		if err := rows.Scan(&depth, &n); err != nil {
			return fmt.Errorf("깊이 스캔: %w", err)
		}
		depths[depth] = n
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("깊이 조회: %w", err)
	}

	fmt.Printf("\n깊이별 카테고리: 1단계 %d, 2단계 %d, 3단계 %d\n",
		depths[1], depths[2], depths[3])

	if depths[1] != len(groups) {
		return fmt.Errorf("1단계가 %d개다. %d개여야 한다", depths[1], len(groups))
	}
	fmt.Printf("1단계가 계획한 %d개인가: 예 ✓\n", len(groups))

	for d := 4; d <= 10; d++ {
		if depths[d] > 0 {
			return fmt.Errorf("%d단계 카테고리가 %d개 있다", d, depths[d])
		}
	}
	fmt.Println("4단계 이상: 0개 ✓")

	// 트리에 들어가지 못한 카테고리가 없는지 (순환 등으로 재귀에서 빠진 것)
	var total, reached int
	if err := sqlDB.QueryRow(`SELECT count(*) FROM categories`).Scan(&total); err != nil {
		return fmt.Errorf("전체 수 조회: %w", err)
	}
	for _, n := range depths {
		reached += n
	}
	fmt.Printf("트리에 닿는 카테고리: %d / %d개%s\n", reached, total, mark(reached == total))
	if reached != total {
		return fmt.Errorf("트리에서 닿지 않는 카테고리가 있다 (순환 가능성)")
	}

	// 글이 여전히 유효한 카테고리를 가리키는지
	var badRef int
	err = sqlDB.QueryRow(`
		SELECT count(*) FROM posts
		WHERE category_id IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM categories c WHERE c.id = posts.category_id)`).Scan(&badRef)
	if err != nil {
		return fmt.Errorf("category_id 참조 검사: %w", err)
	}
	fmt.Printf("없는 카테고리를 가리키는 글: %d건%s\n", badRef, mark(badRef == 0))
	if badRef != 0 {
		return fmt.Errorf("category_id가 깨졌다")
	}

	// moves가 실제로 반영됐는지
	for _, mv := range curation.Moves {
		var parentSlug string
		err := sqlDB.QueryRow(`
			SELECT coalesce((SELECT p.slug FROM categories p WHERE p.id = c.parent_id), '')
			FROM categories c WHERE c.source_name = ?`, mv.SourceName).Scan(&parentSlug)
		if err != nil {
			return fmt.Errorf("이동 검증(%s): %w", mv.SourceName, err)
		}
		fmt.Printf("%s의 부모: /%s%s\n", mv.SourceName, parentSlug, mark(parentSlug == mv.ToSlug))
		if parentSlug != mv.ToSlug {
			return fmt.Errorf("%q가 /%s 밑에 있어야 하는데 /%s 밑에 있다", mv.SourceName, mv.ToSlug, parentSlug)
		}
	}

	// covers가 실제로 반영됐는지
	for _, cv := range curation.Covers {
		var ok bool
		err := sqlDB.QueryRow(`
			SELECT c.cover_post_id IS NOT NULL AND c.cover_post_id = p.id
			FROM categories c JOIN posts p ON p.notion_page_id = ?
			WHERE c.slug = ?`, cv.NotionPageID, cv.Slug).Scan(&ok)
		if err != nil {
			return fmt.Errorf("표지 검증(%s): %w", cv.Slug, err)
		}
		fmt.Printf("/%s의 표지 글 지정%s\n", cv.Slug, mark(ok))
		if !ok {
			return fmt.Errorf("/%s에 표지 글이 안 붙었다", cv.Slug)
		}
	}

	// 없애기로 한 카테고리가 실제로 사라졌는지
	for _, dc := range curation.DropCategories {
		var n int
		if err := sqlDB.QueryRow(
			`SELECT count(*) FROM categories WHERE source_name = ?`, dc.SourceName).Scan(&n); err != nil {
			return fmt.Errorf("삭제 검증(%s): %w", dc.SourceName, err)
		}
		fmt.Printf("카테고리 %q 삭제됨%s\n", dc.SourceName, mark(n == 0))
		if n != 0 {
			return fmt.Errorf("카테고리 %q가 아직 있다", dc.SourceName)
		}
	}
	return nil
}

func mark(ok bool) string {
	if ok {
		return " ✓"
	}
	return " ✗"
}
