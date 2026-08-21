// categorize는 posts.original_path를 쪼개서 카테고리를 만들고 글에 붙인다.
//
//	original_path: "운영체제 > part 4 : 메모리 관리 > 공룡책 9장 > 3. 페이징"
//	              └ 1단계 ┘ └────── 2단계 ──────┘ └ 버림 ┘ └ 글 제목 ┘
//
// 경로의 마지막 요소는 글 제목 자신이라 카테고리로 쓰지 않는다. 경로에서는 앞의
// 두 개만 쓰고 3번째 이후 조상은 버린다.
//
// 여기서 만든 상위 카테고리를 다시 더 큰 분류로 묶는 건 cmd/regroup이 한다.
// 이 도구는 이미 있는 행의 parent_id를 건드리지 않으므로 그 묶음이 유지된다.
//
// 기본은 미리보기다. 실제로 DB를 고치려면 -apply를 준다.
//
// cmd/import가 아니라 따로 둔 이유: import는 본문을 변환 결과로 덮어써서
// cmd/relink가 고쳐둔 링크를 되돌린다. 카테고리를 붙이자고 그걸 겪을 이유가 없다.
// 이 도구는 DB만 읽고 쓴다.
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
	"github.com/inryeol/blog/internal/importer"
)

const rule = "════════════════════════════════════════════════════════════════════════"

// node는 만들 카테고리 하나다.
type node struct {
	name   string
	slug   string
	parent string // 1단계면 ""
	posts  int    // 이 카테고리에 직접 붙는 글 수
}

// plan은 만들 카테고리와 글별 배정이다.
type plan struct {
	level1 []node
	level2 []node
	// assignByPageID는 글 → 붙을 카테고리 이름. 빈 문자열이면 카테고리 없음.
	assignByPageID map[string]string
	noCategory     []string // 경로가 제목뿐이고 같은 이름의 카테고리도 없는 글
	// coverByCategory는 카테고리 이름 → 그 카테고리의 표지 글 page id다.
	// 표지 글은 노션 최상위 페이지 자신이고, 카테고리당 하나뿐이다.
	coverByCategory map[string]string
	// movedPosts는 사람이 slug로 직접 지정한 글이다 (internal/curation).
	// 경로 처리를 거치지 않으므로 assignByPageID에는 들어가지 않는다.
	movedPosts []movedPost
}

// movedPost는 사람이 카테고리를 직접 정한 글 하나다.
type movedPost struct {
	pageID string
	title  string
	slug   string // 붙일 카테고리 slug
}

// coverCategories는 표지 글이 있는 카테고리 이름을 정렬해 돌려준다.
func (p *plan) coverCategories() []string {
	out := make([]string, 0, len(p.coverByCategory))
	for name := range p.coverByCategory {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
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

	p, err := buildPlan(sqlDB)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printPlan(p)

	if !*apply {
		fmt.Println("\n미리보기다. 실제로 고치려면 -apply 를 붙여 다시 실행해라.")
		return
	}

	changed, err := apply2(sqlDB, p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n적용 실패: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\n카테고리 %d개를 넣거나 갱신했고, 글 %d건의 category_id를 바꿨다.\n",
		len(p.level1)+len(p.level2), changed)

	if err := verify(sqlDB, p); err != nil {
		fmt.Fprintf(os.Stderr, "\n검증 실패: %v\n", err)
		os.Exit(1)
	}
}

func buildPlan(sqlDB *sql.DB) (*plan, error) {
	rows, err := sqlDB.Query(`SELECT notion_page_id, title, original_path FROM posts`)
	if err != nil {
		return nil, fmt.Errorf("posts 조회: %w", err)
	}
	defer rows.Close()

	p := &plan{assignByPageID: map[string]string{}, coverByCategory: map[string]string{}}
	l1Posts := map[string]int{}
	l2Posts := map[string]int{}
	l2Parent := map[string]string{}

	// 사람이 옮기기로 한 글은 경로 처리에서 아예 뺀다. 그래야 그 글의 경로가
	// 카테고리를 만드는 데 쓰이지 않고, 한 카테고리의 글이 전부 빠지면 그
	// 카테고리 자체가 계획에서 사라진다.
	moveBySlug := curation.PostMoveBySlug()

	// 조상이 없는 글은 카테고리 이름이 다 모인 뒤에 다시 본다.
	// 노션 최상위 페이지 자신이 여기 해당하는데, 그 제목이 곧 카테고리 이름이다.
	type orphan struct{ pageID, title string }
	var orphans []orphan

	for rows.Next() {
		var pageID, title string
		var path sql.NullString
		if err := rows.Scan(&pageID, &title, &path); err != nil {
			return nil, fmt.Errorf("posts 스캔: %w", err)
		}

		if slug, ok := moveBySlug[pageID]; ok {
			p.movedPosts = append(p.movedPosts, movedPost{pageID: pageID, title: title, slug: slug})
			// 옮긴 글이라도 그 조상 1단계 분류는 계획에 남긴다(직접 붙는 글 0건으로).
			// 그 분류의 존재 근거가 이 글들뿐일 수 있는데, 그러면 분류가 통째로
			// 사라지면서 자기 표지 글까지 카테고리를 잃는다. 실제로 "최인렬
			// (Inryeol Choi)"가 그랬다 — 그 밑의 글이 이 6건뿐이었다.
			//
			// 2단계는 일부러 안 만든다. 없애려고 옮기는 층이 그거다.
			if a := importer.AssignCategory(pageID, path.String); a.Level1 != "" {
				if _, seen := l1Posts[a.Level1]; !seen {
					l1Posts[a.Level1] = 0
				}
			}
			continue
		}

		a := importer.AssignCategory(pageID, path.String)
		if a.Level1 == "" {
			orphans = append(orphans, orphan{pageID, title})
			continue
		}

		if _, seen := l1Posts[a.Level1]; !seen {
			l1Posts[a.Level1] = 0
		}
		if a.Level2 == "" {
			l1Posts[a.Level1]++
		} else {
			l2Posts[a.Level2]++
			if prev, ok := l2Parent[a.Level2]; ok && prev != a.Level1 {
				// 같은 이름이 서로 다른 부모 밑에 있으면 어느 쪽에 붙일지 정할 수 없다.
				// 지금 덤프에는 없지만, 생기면 조용히 한쪽으로 몰지 않고 멈춘다.
				return nil, fmt.Errorf(
					"2단계 카테고리 %q가 부모 두 개(%q, %q) 밑에 나온다. 규칙을 정해야 한다",
					a.Level2, prev, a.Level1)
			}
			l2Parent[a.Level2] = a.Level1
		}
		p.assignByPageID[pageID] = a.Leaf()
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("posts 조회: %w", err)
	}

	// 노션 최상위 페이지는 경로가 자기 제목뿐이라 조상이 없다. 하지만 그 제목은
	// 하위 글들의 경로에서 상위 카테고리 이름으로 이미 등장한다. 제목이 그 이름과
	// 맞으면 자기 카테고리의 표지 글로 붙인다.
	//
	// 지금 스키마에는 "표지 글"을 표시할 자리가 없다. 그건 나중에 컬럼을 더할 일이고,
	// 여기서는 category_id만 이어둔다.
	droppedL1 := map[string]bool{}
	for _, d := range curation.DropCategories {
		droppedL1[d.SourceName] = true
	}
	for _, o := range orphans {
		if _, ok := l1Posts[o.title]; ok && !droppedL1[o.title] {
			if prev, dup := p.coverByCategory[o.title]; dup {
				// 같은 이름의 최상위 페이지가 둘이면 어느 쪽이 표지인지 알 수 없다.
				return nil, fmt.Errorf(
					"카테고리 %q의 표지 글 후보가 둘이다: %s, %s", o.title, prev, o.pageID)
			}
			l1Posts[o.title]++
			p.assignByPageID[o.pageID] = o.title
			p.coverByCategory[o.title] = o.pageID
			continue
		}
		p.noCategory = append(p.noCategory, o.pageID)
		p.assignByPageID[o.pageID] = ""
	}

	// 사람이 없애기로 한 분류(internal/curation)는 계획에서 뺀다.
	//
	// **없으면 regroup과 서로 싸운다.** regroup이 지운 분류를 categorize가 다음
	// 실행에서 다시 만들고, 그러면 두 도구를 번갈아 돌릴 때마다 트리가 달라진다.
	// 실제로 `수학 & 통계`와 `머신러닝 & 딥러닝`이 그렇게 되살아났다.
	//
	// 2단계는 뺄 필요가 없다. 지금 없애는 것은 전부 노션 최상위(1단계)이고,
	// 그 밑의 2단계는 사람이 다른 곳으로 옮겨서 살아 있다(Moves).
	dropped := map[string]bool{}
	for _, d := range curation.DropCategories {
		dropped[d.SourceName] = true
	}
	for name, n := range l1Posts {
		if dropped[name] {
			continue
		}
		p.level1 = append(p.level1, node{name: name, slug: importer.Slugify(name), posts: n})
	}
	for name, parent := range l2Parent {
		p.level2 = append(p.level2, node{
			name: name, slug: importer.Slugify(name), parent: parent, posts: l2Posts[name],
		})
	}
	// 이름순으로 고정한다. 다시 돌려도 sort_order가 같아야 한다.
	sort.Slice(p.level1, func(i, j int) bool { return p.level1[i].name < p.level1[j].name })
	sort.Slice(p.level2, func(i, j int) bool {
		if p.level2[i].parent != p.level2[j].parent {
			return p.level2[i].parent < p.level2[j].parent
		}
		return p.level2[i].name < p.level2[j].name
	})

	if err := checkSlugCollisions(p); err != nil {
		return nil, err
	}
	return p, nil
}

// checkSlugCollisions는 서로 다른 이름이 같은 slug가 되는지 본다.
// categories.slug는 UNIQUE라서 그냥 넣으면 뒤엣것이 앞엣것을 덮어쓴다.
func checkSlugCollisions(p *plan) error {
	byslug := map[string][]string{}
	for _, n := range append(append([]node{}, p.level1...), p.level2...) {
		if n.slug == "" {
			return fmt.Errorf("카테고리 이름 %q의 slug가 빈 문자열이 된다", n.name)
		}
		byslug[n.slug] = append(byslug[n.slug], n.name)
	}
	var bad []string
	for slug, names := range byslug {
		if len(names) > 1 {
			sort.Strings(names)
			bad = append(bad, fmt.Sprintf("%s ← %s", slug, strings.Join(names, ", ")))
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return fmt.Errorf("서로 다른 이름이 같은 slug가 된다:\n  %s", strings.Join(bad, "\n  "))
	}
	return nil
}

func printPlan(p *plan) {
	fmt.Println(rule)
	fmt.Println("카테고리 생성 계획")
	fmt.Println(rule)

	fmt.Printf("\n1단계 카테고리 %d개, 2단계 %d개\n", len(p.level1), len(p.level2))
	fmt.Printf("카테고리가 붙는 글 %d건, 안 붙는 글 %d건\n",
		len(p.assignByPageID)-len(p.noCategory), len(p.noCategory))

	byParent := map[string][]node{}
	for _, n := range p.level2 {
		byParent[n.parent] = append(byParent[n.parent], n)
	}

	fmt.Println()
	for _, l1 := range p.level1 {
		direct := ""
		if l1.posts > 0 {
			direct = fmt.Sprintf("  · 직접 붙는 글 %d건", l1.posts)
		}
		fmt.Printf("%s  (/%s)%s\n", l1.name, l1.slug, direct)
		for _, l2 := range byParent[l1.name] {
			fmt.Printf("    └ %s  (/%s)  글 %d건\n", l2.name, l2.slug, l2.posts)
		}
	}

	if len(p.coverByCategory) > 0 {
		fmt.Printf("\n■ 자기 카테고리에 붙는 표지 글 %d건\n", len(p.coverByCategory))
		fmt.Println("  노션 최상위 페이지 자신이다. 경로가 제목뿐이지만 그 제목이 카테고리 이름이라")
		fmt.Println("  같은 이름의 카테고리에 붙이고, categories.cover_post_id로도 이어둔다.")
		for _, name := range p.coverCategories() {
			fmt.Printf("      %s\n", name)
		}
	}

	if len(p.movedPosts) > 0 {
		fmt.Printf("\n■ 사람이 카테고리를 직접 정한 글 %d건 (internal/curation)\n", len(p.movedPosts))
		fmt.Println("  경로 처리에서 뺀다. 이 글들의 경로는 카테고리를 만드는 데 쓰이지 않는다.")
		for _, mp := range p.movedPosts {
			fmt.Printf("      %-14s → /%s\n", mp.title, mp.slug)
		}
	}

	if len(p.noCategory) > 0 {
		fmt.Printf("\n■ 카테고리가 없는 글 %d건\n", len(p.noCategory))
		fmt.Println("  조상도 없고 같은 이름의 카테고리도 없다. category_id를 NULL로 둔다.")
	}
}

// apply2는 카테고리를 넣고 글에 붙인다. 전부 한 트랜잭션이다.
// 반환값은 category_id가 실제로 바뀐 글 수다.
func apply2(sqlDB *sql.DB, p *plan) (int, error) {
	tx, err := sqlDB.Begin()
	if err != nil {
		return 0, fmt.Errorf("트랜잭션 시작: %w", err)
	}
	defer tx.Rollback()

	idByName := map[string]int64{}

	// keepParent가 true면 이미 있는 행의 parent_id를 건드리지 않는다.
	//
	// original_path가 알려주는 건 "19개 노션 최상위 + 그 아래 66개"까지다. 그 19개를
	// 다시 더 큰 분류(dev, cs-theory 등) 밑으로 묶는 건 cmd/regroup이 하는 별개의
	// 수작업 층이다. 여기서 parent_id를 NULL로 덮으면 그 묶음이 매번 풀린다.
	//
	// 2단계도 마찬가지로 true를 준다. 처음 INSERT할 때는 경로가 알려준 부모를 그대로
	// 쓰지만, 그 뒤에 사람이 옮긴 것(regroup의 moves)은 존중한다. 경로를 다시 읽어
	// 덮으면 옮긴 게 매번 풀린다. 덤프는 고정이라 경로가 나중에 달라지지 않으므로
	// 잃는 것도 없다.
	upsert := func(n node, parentID any, order int, keepParent bool) error {
		// source_name이 멱등 키다. slug가 아니다.
		//
		// slug를 키로 쓰면 사람이 카테고리 이름을 바꿨을 때(소프트스킬 → tooling)
		// 여기서 못 찾고 옛 이름으로 새로 만들어버린다. 글도 새 쪽으로 딸려간다.
		// source_name은 경로에서 온 이름이라 사람이 바꾸지 않는다.
		//
		// name/slug는 처음 만들 때만 정한다. 이미 있으면 사람이 바꿔둔 걸 존중한다.
		stmt := `
			INSERT INTO categories (parent_id, name, slug, sort_order, source_name)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (source_name) DO UPDATE SET
				parent_id  = excluded.parent_id,
				sort_order = excluded.sort_order`
		if keepParent {
			stmt = `
			INSERT INTO categories (parent_id, name, slug, sort_order, source_name)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (source_name) DO UPDATE SET
				sort_order = excluded.sort_order`
		}
		_, err := tx.Exec(stmt, parentID, n.name, n.slug, order, n.name)
		if err != nil {
			return fmt.Errorf("categories upsert(%s): %w", n.name, err)
		}
		var id int64
		if err := tx.QueryRow(
			`SELECT id FROM categories WHERE source_name = ?`, n.name).Scan(&id); err != nil {
			return fmt.Errorf("categories id 조회(%s): %w", n.name, err)
		}
		idByName[n.name] = id
		return nil
	}

	movedCats := curation.MovedSourceNames()

	// 1단계를 먼저 넣어야 2단계가 부모 id를 참조할 수 있다.
	for i, n := range p.level1 {
		if err := upsert(n, nil, i, true); err != nil {
			return 0, err
		}
	}
	for i, n := range p.level2 {
		// 사람이 옮긴 카테고리는 경로가 알려주는 부모가 이미 없을 수 있다.
		// 그런 건 upsert가 parent_id를 안 건드리므로 부모 id도 필요 없다.
		if movedCats[n.name] {
			// keepParent=true: 사람이 옮겨둔 자리를 그대로 둔다. 새 DB에서는
			// 최상위로 들어갔다가 regroup이 제자리로 옮긴다.
			if err := upsert(n, nil, i, true); err != nil {
				return 0, err
			}
			continue
		}
		parentID, ok := idByName[n.parent]
		if !ok {
			return 0, fmt.Errorf("2단계 %q의 부모 %q를 못 찾았다", n.name, n.parent)
		}
		if err := upsert(n, parentID, i, true); err != nil {
			return 0, err
		}
	}

	// 실제로 값이 달라지는 글만 센다. 그래야 두 번째 실행이 0으로 나온다.
	changed := 0
	for pageID, catName := range p.assignByPageID {
		var want any
		if catName != "" {
			id, ok := idByName[catName]
			if !ok {
				return 0, fmt.Errorf("글 %s에 붙일 카테고리 %q를 못 찾았다", pageID, catName)
			}
			want = id
		}

		var current sql.NullInt64
		err := tx.QueryRow(`SELECT category_id FROM posts WHERE notion_page_id = ?`, pageID).Scan(&current)
		if err != nil {
			return 0, fmt.Errorf("현재 category_id 조회(%s): %w", pageID, err)
		}
		if sameCategory(current, want) {
			continue
		}
		if _, err := tx.Exec(
			`UPDATE posts SET category_id = ?, updated_at = datetime('now') WHERE notion_page_id = ?`,
			want, pageID); err != nil {
			return 0, fmt.Errorf("posts 갱신(%s): %w", pageID, err)
		}
		changed++
	}

	// 사람이 slug로 직접 정한 글. 경로에서 나온 이름이 아니라 slug로 찾는다.
	for _, mp := range p.movedPosts {
		var want int64
		switch err := tx.QueryRow(`SELECT id FROM categories WHERE slug = ?`, mp.slug).Scan(&want); err {
		case nil:
		case sql.ErrNoRows:
			return 0, fmt.Errorf("글 %q를 붙일 카테고리가 없다: slug %q", mp.title, mp.slug)
		default:
			return 0, fmt.Errorf("카테고리 조회(%s): %w", mp.slug, err)
		}

		var current sql.NullInt64
		if err := tx.QueryRow(
			`SELECT category_id FROM posts WHERE notion_page_id = ?`, mp.pageID).Scan(&current); err != nil {
			return 0, fmt.Errorf("현재 category_id 조회(%s): %w", mp.pageID, err)
		}
		if current.Valid && current.Int64 == want {
			continue
		}
		if _, err := tx.Exec(
			`UPDATE posts SET category_id = ?, updated_at = datetime('now') WHERE notion_page_id = ?`,
			want, mp.pageID); err != nil {
			return 0, fmt.Errorf("posts 갱신(%s): %w", mp.pageID, err)
		}
		changed++
	}

	// 표지 글을 카테고리 쪽에 이어둔다. 값이 이미 맞으면 건드리지 않는다.
	for catName, pageID := range p.coverByCategory {
		var want sql.NullInt64
		err := tx.QueryRow(`SELECT id FROM posts WHERE notion_page_id = ?`, pageID).Scan(&want.Int64)
		if err != nil {
			return 0, fmt.Errorf("표지 글 조회(%s): %w", pageID, err)
		}
		want.Valid = true

		var current sql.NullInt64
		err = tx.QueryRow(`SELECT cover_post_id FROM categories WHERE source_name = ?`, catName).Scan(&current)
		if err != nil {
			return 0, fmt.Errorf("현재 표지 글 조회(%s): %w", catName, err)
		}
		if current.Valid && current.Int64 == want.Int64 {
			continue
		}

		// 그 글이 이미 다른 카테고리의 표지면 사람이 옮긴 것이다(regroup의 covers).
		// 그대로 둔다. cover_post_id는 UNIQUE라 여기서 덮으려 들면 제약에 걸려
		// categorize 전체가 실패한다.
		var otherCat sql.NullString
		err = tx.QueryRow(
			`SELECT coalesce(name, '') FROM categories WHERE cover_post_id = ?`, want.Int64).Scan(&otherCat)
		switch {
		case err == nil:
			fmt.Printf("  표지 %q는 이미 %q에 붙어 있다. 사람이 옮긴 것으로 보고 건드리지 않는다.\n",
				catName, otherCat.String)
			continue
		case err != sql.ErrNoRows:
			return 0, fmt.Errorf("표지 중복 검사(%s): %w", catName, err)
		}

		if _, err := tx.Exec(
			`UPDATE categories SET cover_post_id = ? WHERE source_name = ?`, want.Int64, catName); err != nil {
			return 0, fmt.Errorf("표지 글 연결(%s): %w", catName, err)
		}
		changed++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("커밋: %w", err)
	}
	return changed, nil
}

func sameCategory(current sql.NullInt64, want any) bool {
	if want == nil {
		return !current.Valid
	}
	return current.Valid && current.Int64 == want.(int64)
}

func verify(sqlDB *sql.DB, p *plan) error {
	fmt.Println()
	fmt.Println(rule)
	fmt.Println("검증")
	fmt.Println(rule)

	// 계획한 카테고리가 전부 있는지 본다.
	//
	// "1단계가 몇 개인가"로 세지 않는다. 경로에서 뽑은 19개는 cmd/regroup이 더 큰
	// 분류 밑으로 옮겨놨을 수 있어서, 절대 깊이로 판정하면 멀쩡한 상태를 실패로 본다.
	// 대신 계획한 slug가 존재하는지와 부모 관계가 맞는지를 본다.
	missing := 0
	for _, n := range append(append([]node{}, p.level1...), p.level2...) {
		var cnt int
		if err := sqlDB.QueryRow(
			`SELECT count(*) FROM categories WHERE source_name = ?`, n.name).Scan(&cnt); err != nil {
			return fmt.Errorf("카테고리 존재 확인(%s): %w", n.name, err)
		}
		if cnt == 0 {
			missing++
		}
	}
	fmt.Printf("\n계획한 카테고리 %d개 중 DB에 없는 것: %d개%s\n",
		len(p.level1)+len(p.level2), missing, mark(missing == 0))
	if missing != 0 {
		return fmt.Errorf("계획한 카테고리가 DB에 없다")
	}

	// 하위 카테고리가 계획한 부모를 가리키는지.
	// 사람이 일부러 옮긴 것(internal/curation)은 뺀다. 경로와 다른 게 정상이다.
	moved := curation.MovedSourceNames()
	wrongParent := 0
	for _, n := range p.level2 {
		if moved[n.name] {
			continue
		}
		var parentSource sql.NullString
		err := sqlDB.QueryRow(`
			SELECT pp.source_name FROM categories c
			LEFT JOIN categories pp ON c.parent_id = pp.id
			WHERE c.source_name = ?`, n.name).Scan(&parentSource)
		if err != nil {
			return fmt.Errorf("부모 확인(%s): %w", n.name, err)
		}
		if !parentSource.Valid || parentSource.String != n.parent {
			wrongParent++
		}
	}
	fmt.Printf("부모가 계획과 다른 하위 카테고리: %d개%s\n", wrongParent, mark(wrongParent == 0))
	if wrongParent != 0 {
		return fmt.Errorf("하위 카테고리의 부모가 계획과 다르다")
	}

	// 4단계가 생기지 않았는지. 스키마 트리거가 막지만 직접 확인한다.
	var depth4 int
	err := sqlDB.QueryRow(`
		SELECT count(*) FROM categories c
		JOIN categories p ON c.parent_id = p.id
		JOIN categories g ON p.parent_id = g.id
		WHERE g.parent_id IS NOT NULL`).Scan(&depth4)
	if err != nil {
		return fmt.Errorf("깊이 검사: %w", err)
	}
	fmt.Printf("4단계 이상 카테고리: %d개%s\n", depth4, mark(depth4 == 0))
	if depth4 != 0 {
		return fmt.Errorf("카테고리가 3단계를 넘었다")
	}

	// 부모가 실제로 존재하는지 (외래키가 보장하지만 명시적으로 본다)
	var orphan int
	err = sqlDB.QueryRow(`
		SELECT count(*) FROM categories c
		WHERE c.parent_id IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM categories p WHERE p.id = c.parent_id)`).Scan(&orphan)
	if err != nil {
		return fmt.Errorf("부모 존재 검사: %w", err)
	}
	fmt.Printf("부모가 없는 2단계 카테고리: %d개%s\n", orphan, mark(orphan == 0))
	if orphan != 0 {
		return fmt.Errorf("부모가 없는 카테고리가 있다")
	}

	// category_id가 실제 categories.id를 가리키는지
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

	// category_id가 NULL인 글은 조상이 없는 글이어야 한다
	var withCat, nullCat int
	if err := sqlDB.QueryRow(`SELECT count(*) FROM posts WHERE category_id IS NOT NULL`).Scan(&withCat); err != nil {
		return fmt.Errorf("배정된 글 수 조회: %w", err)
	}
	if err := sqlDB.QueryRow(`SELECT count(*) FROM posts WHERE category_id IS NULL`).Scan(&nullCat); err != nil {
		return fmt.Errorf("미배정 글 수 조회: %w", err)
	}
	fmt.Printf("카테고리가 붙은 글: %d건, NULL: %d건%s\n",
		withCat, nullCat, mark(nullCat == len(p.noCategory)))
	if nullCat != len(p.noCategory) {
		return fmt.Errorf("NULL인 글이 %d건인데 조상 없는 글은 %d건이다", nullCat, len(p.noCategory))
	}

	badNull, err := nullWithAncestors(sqlDB)
	if err != nil {
		return err
	}
	fmt.Printf("조상이 있는데 NULL인 글: %d건%s\n", badNull, mark(badNull == 0))
	if badNull != 0 {
		return fmt.Errorf("카테고리가 붙었어야 할 글이 NULL이다")
	}

	// 표지 글이 제대로 이어졌는지
	var coverCount, coverBad int
	if err := sqlDB.QueryRow(
		`SELECT count(*) FROM categories WHERE cover_post_id IS NOT NULL`).Scan(&coverCount); err != nil {
		return fmt.Errorf("표지 글 수 조회: %w", err)
	}
	err = sqlDB.QueryRow(`
		SELECT count(*) FROM categories c
		WHERE c.cover_post_id IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM posts p WHERE p.id = c.cover_post_id)`).Scan(&coverBad)
	if err != nil {
		return fmt.Errorf("표지 글 참조 검사: %w", err)
	}
	// 사람이 붙인 표지(internal/curation)는 categorize의 계획 밖이다.
	//
	// 계획이 이미 표지를 두는 카테고리에 사람이 다시 붙이면 총수가 그대로지만
	// (소개 ← 최인렬 페이지가 그렇다), **계획에 표지가 없던 카테고리**에 붙이면
	// 그만큼 늘어난다. 사람이 노션 2단계 분류에 목차 글을 표지로 달면서 이렇게 됐다.
	// 그래서 겹치지 않는 것만 더해서 센다.
	// **이름이 아니라 DB의 id로 센다.** 사람이 분류 이름을 바꾸면 계획의 이름과
	// 실제 slug가 갈라지기 때문이다(핸즈온 머신러닝 2 → 머신러닝: 기초이론).
	want := map[int64]bool{}
	for name := range p.coverByCategory {
		var id int64
		switch err := sqlDB.QueryRow(
			`SELECT id FROM categories WHERE source_name = ?`, name).Scan(&id); err {
		case nil:
			want[id] = true
		case sql.ErrNoRows: // 없앤 카테고리다
		default:
			return fmt.Errorf("표지 카테고리 조회(%s): %w", name, err)
		}
	}
	humanPages := map[string]bool{}
	for _, cv := range curation.Covers {
		humanPages[cv.NotionPageID] = true
		var id int64
		switch err := sqlDB.QueryRow(
			`SELECT id FROM categories WHERE slug = ?`, cv.Slug).Scan(&id); err {
		case nil:
			want[id] = true
		case sql.ErrNoRows:
			return fmt.Errorf("표지를 붙일 카테고리가 없다: %q", cv.Slug)
		default:
			return fmt.Errorf("표지 카테고리 조회(%s): %w", cv.Slug, err)
		}
	}
	// **한 글은 한 카테고리의 표지만 될 수 있다**(cover_post_id가 UNIQUE다).
	// 사람이 어떤 글을 다른 분류의 표지로 가져가면, 경로가 그 글을 표지로 두려던
	// 카테고리는 표지를 잃는다. `최인렬 (Inryeol Choi)`가 그렇다 — 그 글은
	// `/intro`의 표지다.
	for name, pageID := range p.coverByCategory {
		if !humanPages[pageID] {
			continue
		}
		var id int64
		if err := sqlDB.QueryRow(
			`SELECT id FROM categories WHERE source_name = ?`, name).Scan(&id); err == nil {
			var used int
			_ = sqlDB.QueryRow(
				`SELECT count(*) FROM categories WHERE cover_post_id =
				   (SELECT id FROM posts WHERE notion_page_id = ?) AND id = ?`, pageID, id).Scan(&used)
			if used == 0 {
				delete(want, id)
			}
		}
	}
	extra := len(want) - len(p.coverByCategory)
	wantCovers := len(want)
	fmt.Printf("표지 글이 이어진 카테고리: %d개 (계획 %d개 + 사람이 정한 것 중 새로 붙는 %d개)%s\n",
		coverCount, len(p.coverByCategory), extra, mark(coverCount == wantCovers))
	if coverCount != wantCovers {
		return fmt.Errorf("표지 글 수가 계획과 다르다")
	}
	fmt.Printf("없는 글을 가리키는 표지: %d개%s\n", coverBad, mark(coverBad == 0))
	if coverBad != 0 {
		return fmt.Errorf("cover_post_id가 깨졌다")
	}

	// 표지 글이 자기 카테고리에 속해 있는지.
	// 사람이 붙인 표지는 뺀다. 그 글은 원래 카테고리에 속한 채로 다른 분류의
	// 표지가 되는 것이라(소개 ← 최인렬 페이지) 어긋나는 게 정상이다.
	var coverMismatch int
	err = sqlDB.QueryRow(`
		SELECT count(*) FROM categories c
		JOIN posts p ON p.id = c.cover_post_id
		WHERE p.category_id IS NOT c.id
		  AND p.notion_page_id NOT IN (`+placeholders(len(curation.Covers))+`)`,
		coverPageIDArgs()...).Scan(&coverMismatch)
	if err != nil {
		return fmt.Errorf("표지 글 소속 검사: %w", err)
	}
	fmt.Printf("자기 카테고리에 속하지 않은 표지 글: %d개%s\n", coverMismatch, mark(coverMismatch == 0))
	if coverMismatch != 0 {
		return fmt.Errorf("표지 글이 다른 카테고리에 속해 있다")
	}

	// 다시 돌려도 안 바뀌어야 한다
	p2, err := buildPlan(sqlDB)
	if err != nil {
		return err
	}
	again, err := apply2(sqlDB, p2)
	if err != nil {
		return err
	}
	fmt.Printf("다시 돌렸을 때 바뀐 글: %d건%s\n", again, mark(again == 0))
	if again != 0 {
		return fmt.Errorf("멱등하지 않다")
	}
	return nil
}

// nullWithAncestors는 조상이 있는데도 category_id가 NULL인 글 수를 센다.
func nullWithAncestors(sqlDB *sql.DB) (int, error) {
	rows, err := sqlDB.Query(`SELECT original_path FROM posts WHERE category_id IS NULL`)
	if err != nil {
		return 0, fmt.Errorf("NULL인 글 조회: %w", err)
	}
	defer rows.Close()

	n := 0
	for rows.Next() {
		var path sql.NullString
		if err := rows.Scan(&path); err != nil {
			return 0, fmt.Errorf("NULL인 글 스캔: %w", err)
		}
		if len(importer.PathAncestors(path.String)) > 0 {
			n++
		}
	}
	return n, rows.Err()
}

func mark(ok bool) string {
	if ok {
		return " ✓"
	}
	return " ✗"
}

// placeholders는 IN 절에 넣을 ? 목록을 만든다. n이 0이면 아무것도 안 맞는
// NULL을 준다 (빈 IN ()은 SQLite 문법 오류다).
func placeholders(n int) string {
	if n == 0 {
		return "NULL"
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// coverPageIDArgs는 사람이 표지로 지정한 글의 notion_page_id를 인자로 만든다.
func coverPageIDArgs() []any {
	out := make([]any, 0, len(curation.Covers))
	for _, c := range curation.Covers {
		out = append(out, c.NotionPageID)
	}
	return out
}
