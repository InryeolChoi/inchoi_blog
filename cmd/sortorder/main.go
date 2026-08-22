// sortorder는 posts.sort_order를 채운다. 형제들 사이의 순서다.
//
// 출처가 두 가지고 신뢰도가 다르다:
//
//   - child_page (555건): 부모 페이지의 blocks 배열에서 몇 번째인지. 노션 API가
//     블록을 화면 순서 그대로 주므로 이건 정확한 순서다.
//   - 데이터베이스 자식 (739건): created_time 순위. 노션 API는 데이터베이스 행의
//     화면 순서를 아예 노출하지 않아서 차선책이다. created_time은 분 단위라
//     한 번에 만든 페이지들이 묶이는데, 그런 건 같은 순위로 둬서 "순서 미정"을 남긴다.
//   - 최상위 페이지 (17건): 형제 배열 자체가 없다. 0으로 두고 사람이 정한다.
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

	"github.com/inryeol/blog/internal/curation"
	"github.com/inryeol/blog/internal/db"
	"github.com/inryeol/blog/internal/importer"
	"github.com/inryeol/blog/internal/notion"
)

const rule = "════════════════════════════════════════════════════════════════════════"

// source는 순서를 어디서 얻었는지다.
type source string

const (
	srcChildPage source = "childpage" // 부모 blocks 배열의 위치 (정확)
	srcCreated   source = "created"   // created_time 순위 (차선)
	srcManual    source = "manual"    // 사람이 internal/curation에서 정한 순서
	srcNone      source = "none"      // 정할 근거 없음
)

// assignment는 글 하나의 sort_order 결정 결과다.
type assignment struct {
	pageID  string
	title   string
	order   int
	src     source
	groupID string // 형제 묶음의 식별자 (컨테이너 id 또는 데이터베이스 id)
	// tied는 이 글이 같은 순위를 가진 다른 글과 묶여 있는지다.
	tied    bool
	skipGap bool // 일부 형제를 수동 묶음으로 떼어내 원본 순위에 빈칸이 생긴 묶음
}

func main() {
	dbPath := flag.String("db", "blog.db", "SQLite 파일 경로")
	dumpDir := flag.String("dump", "scripts/dump", "노션 덤프 디렉토리 (읽기 전용)")
	only := flag.String("only", "all", "childpage | created | all")
	apply := flag.Bool("apply", false, "실제로 DB를 고친다. 없으면 미리보기만")
	flag.Parse()

	if *only != "all" && *only != string(srcChildPage) && *only != string(srcCreated) {
		fmt.Fprintf(os.Stderr, "-only 는 childpage, created, all 중 하나여야 한다 (받은 값: %q)\n", *only)
		os.Exit(1)
	}

	sqlDB, err := db.Open(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	titles, err := loadTitles(sqlDB)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	assigns, topLevel, err := computeOrders(*dumpDir, titles)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *only == "all" {
		assigns = applyManualOrders(assigns)
	}

	selected := filterBySource(assigns, *only)
	printPlan(selected, topLevel, *only)

	if !*apply {
		fmt.Println("\n미리보기다. 실제로 고치려면 -apply 를 붙여 다시 실행해라.")
		return
	}

	changed, err := applyOrders(sqlDB, selected)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n적용 실패: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\n%d건의 sort_order를 바꿨다.\n", changed)

	if err := verify(sqlDB, selected); err != nil {
		fmt.Fprintf(os.Stderr, "\n검증 실패: %v\n", err)
		os.Exit(1)
	}
}

func loadTitles(sqlDB *sql.DB) (map[string]string, error) {
	rows, err := sqlDB.Query(`SELECT notion_page_id, title FROM posts WHERE notion_page_id IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("posts 조회: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var id, title string
		if err := rows.Scan(&id, &title); err != nil {
			return nil, fmt.Errorf("posts 스캔: %w", err)
		}
		out[id] = title
	}
	return out, rows.Err()
}

// computeOrders는 덤프를 훑어 글마다 sort_order를 정한다.
// 두 번째 반환값은 최상위 페이지 목록이다(사람이 순서를 정해야 한다).
func computeOrders(dumpDir string, titles map[string]string) ([]assignment, []assignment, error) {
	files, err := filepath.Glob(filepath.Join(dumpDir, "*.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("덤프 목록: %w", err)
	}
	sort.Strings(files)

	// 1) child_page 형제 순서 모으기 + 데이터베이스 자식 모으기
	type dbChild struct {
		pageID    string
		createdAt string
	}
	childOrder := map[string]assignment{}
	dbGroups := map[string][]dbChild{}
	var topLevel []assignment

	for _, f := range files {
		d, err := notion.LoadDump(f)
		if err != nil {
			return nil, nil, err
		}

		for _, s := range d.ChildPageOrder() {
			childOrder[s.PageID] = assignment{
				pageID:  s.PageID,
				order:   s.Index,
				src:     srcChildPage,
				groupID: s.ContainerID,
			}
		}

		switch d.Page.Parent.Type {
		case "database_id":
			dbGroups[d.Page.Parent.DatabaseID] = append(dbGroups[d.Page.Parent.DatabaseID],
				dbChild{pageID: d.Page.ID, createdAt: d.Page.CreatedTime})
		case "workspace":
			topLevel = append(topLevel, assignment{
				pageID: d.Page.ID, title: titles[d.Page.ID], src: srcNone,
			})
		}
	}

	var out []assignment
	for _, a := range childOrder {
		a.title = titles[a.pageID]
		out = append(out, a)
	}

	// 2) 데이터베이스 자식은 created_time 순위로. 같은 시각은 같은 순위를 준다.
	for dbID, kids := range dbGroups {
		sort.Slice(kids, func(i, j int) bool { return kids[i].pageID < kids[j].pageID })

		times := make([]string, len(kids))
		for i, k := range kids {
			times[i] = k.createdAt
		}
		ranks := importer.DenseRank(times)
		tiedCount := map[int]int{}
		for _, r := range ranks {
			tiedCount[r]++
		}

		src := srcCreated
		if importer.AllTied(times) {
			// 전부 같은 시각이면 순서를 전혀 모른다. 전부 0이 되는데, 그게
			// created_time에서 얻은 순서가 아니라는 걸 구분해 둔다.
			src = srcNone
		}
		for i, k := range kids {
			out = append(out, assignment{
				pageID:  k.pageID,
				title:   titles[k.pageID],
				order:   ranks[i],
				src:     src,
				groupID: dbID,
				tied:    tiedCount[ranks[i]] > 1,
			})
		}
	}

	for _, a := range topLevel {
		out = append(out, a)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].pageID < out[j].pageID })
	return out, topLevel, nil
}

func filterBySource(all []assignment, only string) []assignment {
	if only == "all" {
		return all
	}
	var out []assignment
	for _, a := range all {
		if string(a.src) == only {
			out = append(out, a)
		}
	}
	return out
}

// applyManualOrders는 노션에서 복원한 순서보다 사람이 직접 정한 순서를 우선한다.
// import도 같은 표를 적용하므로 어느 도구를 나중에 실행해도 결과가 같다.
func applyManualOrders(assigns []assignment) []assignment {
	edits := curation.PostMetadataByID()
	out := append([]assignment(nil), assigns...)
	fragmented := map[string]bool{}
	for _, a := range out {
		if _, ok := edits[a.pageID]; ok && a.groupID != "" {
			fragmented[a.groupID] = true
		}
	}
	for i := range out {
		originalGroup := out[i].groupID
		edit, ok := edits[out[i].pageID]
		if !ok {
			out[i].skipGap = fragmented[originalGroup]
			continue
		}
		out[i].title = edit.Title
		out[i].order = edit.SortOrder
		out[i].src = srcManual
		out[i].tied = false
		out[i].groupID = "manual:linear-algebra"
	}
	return out
}

func printPlan(assigns []assignment, topLevel []assignment, only string) {
	fmt.Println(rule)
	fmt.Printf("sort_order 계획  (-only %s)\n", only)
	fmt.Println(rule)

	bySrc := map[source][]assignment{}
	for _, a := range assigns {
		bySrc[a.src] = append(bySrc[a.src], a)
	}

	cp := bySrc[srcChildPage]
	cr := bySrc[srcCreated]
	mm := bySrc[srcManual]
	nn := bySrc[srcNone]

	fmt.Printf("\n대상 %d건\n", len(assigns))
	if len(cp) > 0 {
		fmt.Printf("  child_page 배열 위치 (정확)   : %d건, 묶음 %d개\n", len(cp), countGroups(cp))
	}
	if len(cr) > 0 {
		tied := 0
		for _, a := range cr {
			if a.tied {
				tied++
			}
		}
		fmt.Printf("  created_time 순위 (차선)      : %d건, 묶음 %d개\n", len(cr), countGroups(cr))
		fmt.Printf("      그중 같은 시각과 묶인 글   : %d건 (%.1f%%)\n",
			tied, float64(tied)*100/float64(len(cr)))
	}
	if len(mm) > 0 {
		fmt.Printf("  사람이 정한 순서             : %d건, 묶음 %d개\n", len(mm), countGroups(mm))
	}
	if len(nn) > 0 {
		fmt.Printf("  근거 없음 → 0으로 둠          : %d건\n", len(nn))
	}

	if len(cr) > 0 {
		tiedPairs, totalPairs := pairStats(cr)
		decided := totalPairs - tiedPairs
		fmt.Printf("\n순서를 정해야 하는 쌍 %d쌍 중\n", totalPairs)
		fmt.Printf("  created_time으로 정해짐 : %d쌍 (%.1f%%)\n",
			decided, float64(decided)*100/float64(totalPairs))
		fmt.Printf("  같은 분이라 못 정함     : %d쌍 (%.1f%%)\n",
			tiedPairs, float64(tiedPairs)*100/float64(totalPairs))
	}

	if len(cp) > 0 {
		fmt.Println("\n■ child_page 표본 (한 묶음)")
		printSampleGroup(cp)
	}
	if len(cr) > 0 {
		fmt.Println("\n■ created_time 표본 (묶임이 있는 묶음)")
		printSampleGroup(cr)
	}

	if len(topLevel) > 0 && only == "all" {
		fmt.Printf("\n■ 최상위 페이지 %d건 — 형제 배열이 없어 sort_order를 0으로 둔다\n", len(topLevel))
		fmt.Println("  아래 순서는 정하지 않았다. 직접 정해서 알려주면 반영한다.")
		sorted := append([]assignment{}, topLevel...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].title < sorted[j].title })
		for _, a := range sorted {
			fmt.Printf("      %s   (%s)\n", a.title, a.pageID)
		}
	}
}

func countGroups(assigns []assignment) int {
	g := map[string]bool{}
	for _, a := range assigns {
		g[a.groupID] = true
	}
	return len(g)
}

func pairStats(assigns []assignment) (tied, total int) {
	byGroup := map[string][]int{}
	for _, a := range assigns {
		byGroup[a.groupID] = append(byGroup[a.groupID], a.order)
	}
	for _, orders := range byGroup {
		n := len(orders)
		total += n * (n - 1) / 2
		counts := map[int]int{}
		for _, o := range orders {
			counts[o]++
		}
		for _, c := range counts {
			tied += c * (c - 1) / 2
		}
	}
	return tied, total
}

// printSampleGroup은 묶음 하나를 골라 순서대로 보여준다.
func printSampleGroup(assigns []assignment) {
	byGroup := map[string][]assignment{}
	for _, a := range assigns {
		byGroup[a.groupID] = append(byGroup[a.groupID], a)
	}

	best := ""
	bestScore := -1
	for g, items := range byGroup {
		score := len(items)
		if assigns[0].src == srcCreated {
			// 묶임이 섞인 묶음을 보여줘야 동률 처리가 눈에 보인다.
			tied := 0
			for _, a := range items {
				if a.tied {
					tied++
				}
			}
			if tied == 0 || tied == len(items) {
				score = 0
			}
		}
		if score > bestScore {
			bestScore, best = score, g
		}
	}

	items := byGroup[best]
	sort.Slice(items, func(i, j int) bool {
		if items[i].order != items[j].order {
			return items[i].order < items[j].order
		}
		return items[i].title < items[j].title
	})
	fmt.Printf("  묶음 %s (%d건)\n", best[:8], len(items))
	for i, a := range items {
		if i >= 10 {
			fmt.Printf("      … 외 %d건\n", len(items)-10)
			break
		}
		mark := ""
		if a.tied {
			mark = "  ← 같은 시각(순서 미정)"
		}
		fmt.Printf("      %2d  %s%s\n", a.order, truncate(a.title, 44), mark)
	}
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// applyOrders는 값이 실제로 달라지는 글만 갱신한다.
// 그래야 다시 돌렸을 때 0건으로 나와서 멱등한 걸 확인할 수 있다.
func applyOrders(sqlDB *sql.DB, assigns []assignment) (int, error) {
	tx, err := sqlDB.Begin()
	if err != nil {
		return 0, fmt.Errorf("트랜잭션 시작: %w", err)
	}
	defer tx.Rollback()

	sel, err := tx.Prepare(`SELECT sort_order FROM posts WHERE notion_page_id = ?`)
	if err != nil {
		return 0, fmt.Errorf("SELECT 준비: %w", err)
	}
	defer sel.Close()

	upd, err := tx.Prepare(
		`UPDATE posts SET sort_order = ?, updated_at = datetime('now') WHERE notion_page_id = ?`)
	if err != nil {
		return 0, fmt.Errorf("UPDATE 준비: %w", err)
	}
	defer upd.Close()

	changed := 0
	for _, a := range assigns {
		var current int
		switch err := sel.QueryRow(a.pageID).Scan(&current); err {
		case nil:
		case sql.ErrNoRows:
			// 덤프에는 있는데 posts에 없는 페이지. 이관 대상이 아니었다는 뜻이다.
			continue
		default:
			return 0, fmt.Errorf("현재 sort_order 조회(%s): %w", a.pageID, err)
		}
		if current == a.order {
			continue
		}
		if _, err := upd.Exec(a.order, a.pageID); err != nil {
			return 0, fmt.Errorf("sort_order 갱신(%s): %w", a.pageID, err)
		}
		changed++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("커밋: %w", err)
	}
	return changed, nil
}

func verify(sqlDB *sql.DB, assigns []assignment) error {
	fmt.Println()
	fmt.Println(rule)
	fmt.Println("검증")
	fmt.Println(rule)

	// DB에 들어간 값이 계획과 같은지
	mismatch := 0
	checked := 0
	for _, a := range assigns {
		var got int
		switch err := sqlDB.QueryRow(
			`SELECT sort_order FROM posts WHERE notion_page_id = ?`, a.pageID).Scan(&got); err {
		case nil:
			checked++
			if got != a.order {
				mismatch++
			}
		case sql.ErrNoRows:
			continue
		default:
			return fmt.Errorf("검증 조회(%s): %w", a.pageID, err)
		}
	}
	fmt.Printf("\n계획과 다른 값: %d건 / %d건 검사%s\n", mismatch, checked, mark(mismatch == 0))
	if mismatch != 0 {
		return fmt.Errorf("DB 값이 계획과 다르다")
	}

	// 형제 묶음 안에서 순위가 0부터 빈틈없이 이어지는지
	gaps := 0
	byGroup := map[string][]int{}
	for _, a := range assigns {
		if a.src == srcNone || a.skipGap {
			continue
		}
		byGroup[a.groupID] = append(byGroup[a.groupID], a.order)
	}
	for _, orders := range byGroup {
		seen := map[int]bool{}
		max := 0
		for _, o := range orders {
			seen[o] = true
			if o > max {
				max = o
			}
		}
		for i := 0; i <= max; i++ {
			if !seen[i] {
				gaps++
				break
			}
		}
	}
	fmt.Printf("순위에 빈틈이 있는 묶음: %d개%s\n", gaps, mark(gaps == 0))
	if gaps != 0 {
		return fmt.Errorf("순위가 0부터 이어지지 않는 묶음이 있다")
	}

	// 다시 돌려도 안 바뀌어야 한다
	again, err := applyOrders(sqlDB, assigns)
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
