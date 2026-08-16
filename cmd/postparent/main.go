// postparent는 posts.parent_id를 채운다. 글끼리의 부모-자식 관계다.
//
// 카테고리는 3단계까지만 있어서(사람이 정한 8 > 노션 최상위 19 > 노션 2단계 67),
// 그보다 깊었던 노션 계층은 categorize를 거치면서 평평해진다. 예를 들어
// "수학 & 통계 > 수리통계1 > 3. 일변량 분포 : 예시 > 베르누이 분포"의 글은
// 수리통계1 카테고리에 붙고, 그 사이의 "3. 일변량 분포 : 예시"는 카테고리가 아니라
// 같은 카테고리의 다른 글이 된다. 그래서 목록이 한 층으로 나열된다.
// parent_id가 그 잃어버린 층을 되살린다.
//
// 출처는 sort_order와 같다. 부모 페이지 덤프의 child_page 블록이다.
//
//   - 덤프 파일 {page}.json의 blocks 안에 있는 child_page 블록은 전부 그 페이지의
//     직속 하위 페이지다. 덤프 스크립트가 child_page/child_database 안으로는
//     재귀하지 않기 때문이다(scripts/notion-block-dump.mjs). 그래서 단이나 토글
//     안에 깊이 박혀 있어도 부모 페이지는 덤프의 주인 페이지 하나로 정해진다.
//   - 부모가 데이터베이스인 페이지는 그 데이터베이스를 담고 있는 페이지를 부모로 삼는다.
//     child_database 블록의 id가 곧 데이터베이스 id다.
//
// 한 번에 한 카테고리만 다룬다. 대상은 그 카테고리에 직접 붙은 글이고,
// 부모가 다른 카테고리에 있어도(카테고리 표지 글 등) 그대로 가리킨다.
// 고치는 행은 언제나 대상 카테고리의 글뿐이다.
//
// 기본은 미리보기다. 실제로 DB를 고치려면 -apply를 준다.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/inryeol/blog/internal/db"
	"github.com/inryeol/blog/internal/notion"
)

const rule = "════════════════════════════════════════════════════════════════════════"

// pathSep는 original_path의 구분자다.
const pathSep = " > "

// source는 부모를 어디서 얻었는지다.
type source string

const (
	srcChildPage source = "childpage" // 부모 덤프의 child_page 블록 (정확)
	srcDatabase  source = "database"  // 데이터베이스를 담은 페이지 (정확하지만 한 단계 건너뜀)
)

// post는 DB에 있는 글 하나다.
type post struct {
	id        int64
	pageID    string
	title     string
	catID     sql.NullInt64
	sortOrder int
	path      sql.NullString
	parentID  sql.NullInt64 // 지금 DB에 들어 있는 값
}

// plan은 글 하나의 parent_id 결정 결과다.
type plan struct {
	child  *post
	parent *post // nil이면 붙일 부모를 못 찾았다는 뜻이다
	src    source
	// reason은 parent가 nil일 때 왜 못 찾았는지다.
	reason string
	// pathOK는 original_path가 부모-자식 관계와 맞는지다.
	pathOK bool
}

func main() {
	dbPath := flag.String("db", "blog.db", "SQLite 파일 경로")
	dumpDir := flag.String("dump", "scripts/dump", "노션 덤프 디렉토리 (읽기 전용)")
	catSlug := flag.String("category", "", "대상 카테고리 slug (필수)")
	apply := flag.Bool("apply", false, "실제로 DB를 고친다. 없으면 미리보기만")
	flag.Parse()

	if *catSlug == "" {
		fmt.Fprintln(os.Stderr, "-category 로 대상 카테고리 slug를 줘라. 전체를 한 번에 고치지 않는다.")
		os.Exit(1)
	}

	if err := run(*dbPath, *dumpDir, *catSlug, *apply); err != nil {
		fmt.Fprintf(os.Stderr, "\n%v\n", err)
		os.Exit(1)
	}
}

func run(dbPath, dumpDir, catSlug string, apply bool) error {
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	catID, catName, err := resolveCategory(sqlDB, catSlug)
	if err != nil {
		return err
	}

	byPage, byID, err := loadPosts(sqlDB)
	if err != nil {
		return err
	}

	parentOf, srcOf, err := scanDumps(dumpDir)
	if err != nil {
		return err
	}

	plans := buildPlans(catID, byPage, parentOf, srcOf)
	if len(plans) == 0 {
		return fmt.Errorf("카테고리 %q(%d)에 직접 붙은 글이 없다", catName, catID)
	}

	if err := checkCycles(plans, byID); err != nil {
		return err
	}

	printPlan(catName, catID, plans, byID)

	if !apply {
		fmt.Println("\n미리보기다. 실제로 고치려면 -apply 를 붙여 다시 실행해라.")
		return nil
	}

	changed, err := applyParents(sqlDB, plans)
	if err != nil {
		return fmt.Errorf("적용 실패: %w", err)
	}
	fmt.Printf("\n%d건의 parent_id를 바꿨다.\n", changed)

	return verify(sqlDB, plans)
}

// resolveCategory는 slug로 카테고리를 찾는다. slug는 부모가 다르면 겹칠 수 있어서
// 둘 이상 맞으면 조용히 하나를 고르지 않고 멈춘다.
func resolveCategory(sqlDB *sql.DB, slug string) (int64, string, error) {
	rows, err := sqlDB.Query(`
		SELECT c.id, c.name,
		       coalesce((SELECT p.name FROM categories p WHERE p.id = c.parent_id), '(최상위)')
		FROM categories c WHERE c.slug = ?`, slug)
	if err != nil {
		return 0, "", fmt.Errorf("카테고리 조회(%s): %w", slug, err)
	}
	defer rows.Close()

	type hit struct {
		id     int64
		name   string
		parent string
	}
	var hits []hit
	for rows.Next() {
		var h hit
		if err := rows.Scan(&h.id, &h.name, &h.parent); err != nil {
			return 0, "", fmt.Errorf("카테고리 스캔: %w", err)
		}
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return 0, "", err
	}

	switch len(hits) {
	case 0:
		return 0, "", fmt.Errorf("slug가 %q인 카테고리가 없다", slug)
	case 1:
		return hits[0].id, hits[0].name, nil
	default:
		var b strings.Builder
		fmt.Fprintf(&b, "slug %q인 카테고리가 %d개다. 어느 쪽인지 알 수 없어 멈춘다:\n", slug, len(hits))
		for _, h := range hits {
			fmt.Fprintf(&b, "    id=%d  %s  (부모: %s)\n", h.id, h.name, h.parent)
		}
		return 0, "", fmt.Errorf("%s", b.String())
	}
}

func loadPosts(sqlDB *sql.DB) (map[string]*post, map[int64]*post, error) {
	rows, err := sqlDB.Query(`
		SELECT id, notion_page_id, title, category_id, sort_order, original_path, parent_id
		FROM posts WHERE notion_page_id IS NOT NULL`)
	if err != nil {
		return nil, nil, fmt.Errorf("posts 조회: %w", err)
	}
	defer rows.Close()

	byPage := map[string]*post{}
	byID := map[int64]*post{}
	for rows.Next() {
		var p post
		if err := rows.Scan(&p.id, &p.pageID, &p.title, &p.catID, &p.sortOrder, &p.path, &p.parentID); err != nil {
			return nil, nil, fmt.Errorf("posts 스캔: %w", err)
		}
		byPage[p.pageID] = &p
		byID[p.id] = &p
	}
	return byPage, byID, rows.Err()
}

// scanDumps는 노션 페이지 id → 부모 페이지 id 대응을 덤프에서 모은다.
func scanDumps(dumpDir string) (map[string]string, map[string]source, error) {
	files, err := filepath.Glob(filepath.Join(dumpDir, "*.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("덤프 목록: %w", err)
	}
	if len(files) == 0 {
		return nil, nil, fmt.Errorf("덤프가 없다: %s", dumpDir)
	}
	sort.Strings(files)

	parentOf := map[string]string{}
	srcOf := map[string]source{}
	dbOwner := map[string]string{}    // 데이터베이스 id → 그걸 담고 있는 페이지 id
	inDatabase := map[string]string{} // 페이지 id → 그 페이지가 속한 데이터베이스 id

	for _, f := range files {
		d, err := notion.LoadDump(f)
		if err != nil {
			return nil, nil, err
		}

		// 이 덤프에 나오는 child_page는 전부 이 페이지의 직속 자식이다.
		// (덤프 스크립트가 child_page 안으로 재귀하지 않으므로 다른 페이지의
		//  자식이 섞여 들어올 수 없다.)
		for _, s := range d.ChildPageOrder() {
			parentOf[s.PageID] = d.Page.ID
			srcOf[s.PageID] = srcChildPage
		}

		var walk func(blocks []notion.Block)
		walk = func(blocks []notion.Block) {
			for _, b := range blocks {
				if b.Type == "child_database" {
					dbOwner[b.ID] = d.Page.ID
				}
				walk(b.Children)
			}
		}
		walk(d.Blocks)

		if d.Page.Parent.Type == "database_id" {
			inDatabase[d.Page.ID] = d.Page.Parent.DatabaseID
		}
	}

	// 데이터베이스 행은 데이터베이스를 담은 페이지를 부모로 삼는다.
	// child_page 쪽이 이미 정한 것은 덮지 않는다(그쪽이 한 단계도 건너뛰지 않는다).
	for pageID, dbID := range inDatabase {
		if _, ok := parentOf[pageID]; ok {
			continue
		}
		if owner, ok := dbOwner[dbID]; ok && owner != pageID {
			parentOf[pageID] = owner
			srcOf[pageID] = srcDatabase
		}
	}

	return parentOf, srcOf, nil
}

func buildPlans(catID int64, byPage map[string]*post, parentOf map[string]string, srcOf map[string]source) []plan {
	var out []plan
	for _, p := range byPage {
		if !p.catID.Valid || p.catID.Int64 != catID {
			continue
		}
		pl := plan{child: p, src: srcOf[p.pageID]}

		parentPage, ok := parentOf[p.pageID]
		switch {
		case !ok:
			pl.reason = "덤프에 부모가 없다 (최상위 페이지이거나 부모 덤프가 없다)"
		case parentPage == p.pageID:
			pl.reason = "자기 자신을 부모로 가리킨다"
		default:
			parent, ok := byPage[parentPage]
			if !ok {
				pl.reason = fmt.Sprintf("부모 페이지 %s가 posts에 없다", parentPage)
			} else {
				pl.parent = parent
			}
		}

		pl.pathOK = pathConsistent(pl.child, pl.parent)
		out = append(out, pl)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].child.sortOrder != out[j].child.sortOrder {
			return out[i].child.sortOrder < out[j].child.sortOrder
		}
		return out[i].child.title < out[j].child.title
	})
	return out
}

// pathConsistent는 original_path가 부모-자식 관계와 맞는지 본다.
// 자식 경로에서 마지막 한 칸(자기 제목)을 떼면 부모 경로와 같아야 한다.
//
// 경로를 나눌 때 마지막 구분자만 본다. 제목 자체에 " > "가 들어 있어도
// 앞쪽에서 잘못 자르지 않게 하려는 것이다.
func pathConsistent(child, parent *post) bool {
	if parent == nil {
		return false
	}
	if !child.path.Valid || !parent.path.Valid {
		return false
	}
	i := strings.LastIndex(child.path.String, pathSep)
	if i < 0 {
		return false
	}
	return child.path.String[:i] == parent.path.String
}

// checkCycles는 계획을 반영했을 때 부모 사슬이 돌지 않는지 본다.
// 대상 밖의 글은 지금 DB에 들어 있는 parent_id를 따라간다.
func checkCycles(plans []plan, byID map[int64]*post) error {
	next := map[int64]int64{}
	for id, p := range byID {
		if p.parentID.Valid {
			next[id] = p.parentID.Int64
		}
	}
	for _, pl := range plans {
		if pl.parent != nil {
			next[pl.child.id] = pl.parent.id
		} else {
			delete(next, pl.child.id)
		}
	}

	for _, pl := range plans {
		seen := map[int64]bool{pl.child.id: true}
		for cur, ok := next[pl.child.id]; ok; cur, ok = next[cur] {
			if seen[cur] {
				return fmt.Errorf("부모 사슬이 돈다: 글 %d(%s)에서 시작해 %d로 돌아온다",
					pl.child.id, pl.child.title, cur)
			}
			seen[cur] = true
		}
	}
	return nil
}

func printPlan(catName string, catID int64, plans []plan, byID map[int64]*post) {
	fmt.Println(rule)
	fmt.Printf("parent_id 계획  —  카테고리 %s (id=%d), 글 %d건\n", catName, catID, len(plans))
	fmt.Println(rule)

	var set, keepNull, changed, outside int
	bySrc := map[source]int{}
	pathBad := []plan{}
	for _, pl := range plans {
		if pl.parent == nil {
			keepNull++
			continue
		}
		set++
		bySrc[pl.src]++
		if !pl.child.parentID.Valid || pl.child.parentID.Int64 != pl.parent.id {
			changed++
		}
		if !pl.parent.catID.Valid || pl.parent.catID.Int64 != catID {
			outside++
		}
		if !pl.pathOK {
			pathBad = append(pathBad, pl)
		}
	}

	fmt.Printf("\n부모를 찾음            : %d건\n", set)
	if bySrc[srcChildPage] > 0 {
		fmt.Printf("    child_page 블록    : %d건 (정확)\n", bySrc[srcChildPage])
	}
	if bySrc[srcDatabase] > 0 {
		fmt.Printf("    데이터베이스 소유자 : %d건 (데이터베이스 한 단계를 건너뛴다)\n", bySrc[srcDatabase])
	}
	fmt.Printf("부모 없음 → NULL 유지  : %d건\n", keepNull)
	fmt.Printf("실제로 값이 바뀔 글    : %d건\n", changed)
	if outside > 0 {
		fmt.Printf("부모가 이 카테고리 밖  : %d건 (고치는 행은 여전히 이 카테고리 글뿐이다)\n", outside)
	}

	fmt.Printf("\noriginal_path와 맞는가 : %d건 / %d건 검사%s\n",
		set-len(pathBad), set, mark(len(pathBad) == 0))
	for _, pl := range pathBad {
		fmt.Printf("    ✗ %s\n        자식 경로: %s\n        부모 경로: %s\n",
			pl.child.title, pl.child.path.String, pl.parent.path.String)
	}

	if keepNull > 0 {
		fmt.Println("\n■ 부모를 못 찾은 글")
		for _, pl := range plans {
			if pl.parent == nil {
				fmt.Printf("    %s  (%s)\n        %s\n", pl.child.title, pl.child.pageID, pl.reason)
			}
		}
	}

	printTree(plans, byID, catID)
}

// printTree는 계획대로 붙였을 때의 모습을 들여쓰기로 보여준다.
// 부모가 이 카테고리 밖에 있는 글은 그 부모를 뿌리로 삼아 묶어서 보여준다.
func printTree(plans []plan, byID map[int64]*post, catID int64) {
	fmt.Println("\n■ 붙인 뒤의 모습")

	kids := map[int64][]plan{}
	inSet := map[int64]bool{}
	for _, pl := range plans {
		inSet[pl.child.id] = true
	}

	var roots []plan
	extRoots := map[int64]bool{} // 이 카테고리 밖에 있는 부모
	for _, pl := range plans {
		if pl.parent == nil {
			roots = append(roots, pl)
			continue
		}
		kids[pl.parent.id] = append(kids[pl.parent.id], pl)
		if !inSet[pl.parent.id] {
			extRoots[pl.parent.id] = true
		}
	}

	sortKids := func(s []plan) {
		sort.Slice(s, func(i, j int) bool {
			if s[i].child.sortOrder != s[j].child.sortOrder {
				return s[i].child.sortOrder < s[j].child.sortOrder
			}
			return s[i].child.title < s[j].child.title
		})
	}
	for _, v := range kids {
		sortKids(v)
	}

	var walk func(id int64, depth int)
	walk = func(id int64, depth int) {
		for _, pl := range kids[id] {
			fmt.Printf("    %s%2d  %s\n", strings.Repeat("  ", depth), pl.child.sortOrder, pl.child.title)
			walk(pl.child.id, depth+1)
		}
	}

	var extIDs []int64
	for id := range extRoots {
		extIDs = append(extIDs, id)
	}
	sort.Slice(extIDs, func(i, j int) bool { return extIDs[i] < extIDs[j] })
	for _, id := range extIDs {
		p := byID[id]
		fmt.Printf("    ┌ %s  ← 이 카테고리 밖의 글 (id=%d)\n", p.title, p.id)
		walk(id, 1)
	}

	sortKids(roots)
	for _, pl := range roots {
		fmt.Printf("    %2d  %s  ← 부모 없음\n", pl.child.sortOrder, pl.child.title)
		walk(pl.child.id, 1)
	}
}

// applyParents는 값이 실제로 달라지는 글만 갱신한다.
// 그래야 다시 돌렸을 때 0건으로 나와서 멱등한 걸 확인할 수 있다.
func applyParents(sqlDB *sql.DB, plans []plan) (int, error) {
	tx, err := sqlDB.Begin()
	if err != nil {
		return 0, fmt.Errorf("트랜잭션 시작: %w", err)
	}
	defer tx.Rollback()

	upd, err := tx.Prepare(
		`UPDATE posts SET parent_id = ?, updated_at = datetime('now') WHERE id = ?`)
	if err != nil {
		return 0, fmt.Errorf("UPDATE 준비: %w", err)
	}
	defer upd.Close()

	changed := 0
	for _, pl := range plans {
		// 부모를 못 찾은 글은 건드리지 않는다. 이미 들어 있는 값을 지우지 않는다.
		if pl.parent == nil {
			continue
		}
		if pl.child.parentID.Valid && pl.child.parentID.Int64 == pl.parent.id {
			continue
		}
		if _, err := upd.Exec(pl.parent.id, pl.child.id); err != nil {
			return 0, fmt.Errorf("parent_id 갱신(%d %s): %w", pl.child.id, pl.child.title, err)
		}
		changed++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("커밋: %w", err)
	}
	return changed, nil
}

func verify(sqlDB *sql.DB, plans []plan) error {
	fmt.Println()
	fmt.Println(rule)
	fmt.Println("검증")
	fmt.Println(rule)

	mismatch, checked := 0, 0
	for _, pl := range plans {
		if pl.parent == nil {
			continue
		}
		var got sql.NullInt64
		if err := sqlDB.QueryRow(`SELECT parent_id FROM posts WHERE id = ?`, pl.child.id).Scan(&got); err != nil {
			return fmt.Errorf("검증 조회(%d): %w", pl.child.id, err)
		}
		checked++
		if !got.Valid || got.Int64 != pl.parent.id {
			mismatch++
		}
	}
	fmt.Printf("\n계획과 다른 값: %d건 / %d건 검사%s\n", mismatch, checked, mark(mismatch == 0))
	if mismatch != 0 {
		return fmt.Errorf("DB 값이 계획과 다르다")
	}

	// 다시 돌려도 안 바뀌어야 한다
	for _, pl := range plans {
		if pl.parent != nil {
			pl.child.parentID = sql.NullInt64{Int64: pl.parent.id, Valid: true}
		}
	}
	again, err := applyParents(sqlDB, plans)
	if err != nil {
		return err
	}
	fmt.Printf("다시 돌렸을 때 바뀐 글: %d건%s\n", again, mark(again == 0))
	if again != 0 {
		return fmt.Errorf("멱등하지 않다")
	}
	return nil
}

func mark(ok bool) string {
	if ok {
		return " ✓"
	}
	return " ✗"
}
