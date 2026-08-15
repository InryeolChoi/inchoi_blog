// categorize는 posts.original_path를 쪼개서 카테고리를 만들고 글에 붙인다.
//
//	original_path: "운영체제 > part 4 : 메모리 관리 > 공룡책 9장 > 3. 페이징"
//	              └ 1단계 ┘ └────── 2단계 ──────┘ └ 버림 ┘ └ 글 제목 ┘
//
// 경로의 마지막 요소는 글 제목 자신이라 카테고리로 쓰지 않는다. 카테고리는 최대
// 2단계라 3번째 이후 조상도 버린다.
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
	noCategory     []string // 경로가 제목뿐이라 카테고리가 없는 글
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
	rows, err := sqlDB.Query(`SELECT notion_page_id, original_path FROM posts`)
	if err != nil {
		return nil, fmt.Errorf("posts 조회: %w", err)
	}
	defer rows.Close()

	p := &plan{assignByPageID: map[string]string{}}
	l1Posts := map[string]int{}
	l2Posts := map[string]int{}
	l2Parent := map[string]string{}

	for rows.Next() {
		var pageID string
		var path sql.NullString
		if err := rows.Scan(&pageID, &path); err != nil {
			return nil, fmt.Errorf("posts 스캔: %w", err)
		}

		a := importer.AssignCategory(pageID, path.String)
		if a.Level1 == "" {
			p.noCategory = append(p.noCategory, pageID)
			p.assignByPageID[pageID] = ""
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

	for name, n := range l1Posts {
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

	if len(p.noCategory) > 0 {
		fmt.Printf("\n■ 카테고리가 없는 글 %d건\n", len(p.noCategory))
		fmt.Println("  original_path가 글 제목뿐이라 조상이 없다. category_id를 NULL로 둔다.")
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

	upsert := func(n node, parentID any, order int) error {
		// slug가 멱등 키다. 다시 돌려도 행이 늘지 않는다.
		_, err := tx.Exec(`
			INSERT INTO categories (parent_id, name, slug, sort_order)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (slug) DO UPDATE SET
				parent_id  = excluded.parent_id,
				name       = excluded.name,
				sort_order = excluded.sort_order`,
			parentID, n.name, n.slug, order)
		if err != nil {
			return fmt.Errorf("categories upsert(%s): %w", n.name, err)
		}
		var id int64
		if err := tx.QueryRow(`SELECT id FROM categories WHERE slug = ?`, n.slug).Scan(&id); err != nil {
			return fmt.Errorf("categories id 조회(%s): %w", n.slug, err)
		}
		idByName[n.name] = id
		return nil
	}

	// 1단계를 먼저 넣어야 2단계가 부모 id를 참조할 수 있다.
	for i, n := range p.level1 {
		if err := upsert(n, nil, i); err != nil {
			return 0, err
		}
	}
	for i, n := range p.level2 {
		parentID, ok := idByName[n.parent]
		if !ok {
			return 0, fmt.Errorf("2단계 %q의 부모 %q를 못 찾았다", n.name, n.parent)
		}
		if err := upsert(n, parentID, i); err != nil {
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

	var l1, l2 int
	if err := sqlDB.QueryRow(`SELECT count(*) FROM categories WHERE parent_id IS NULL`).Scan(&l1); err != nil {
		return fmt.Errorf("1단계 수 조회: %w", err)
	}
	if err := sqlDB.QueryRow(`SELECT count(*) FROM categories WHERE parent_id IS NOT NULL`).Scan(&l2); err != nil {
		return fmt.Errorf("2단계 수 조회: %w", err)
	}
	fmt.Printf("\n카테고리: 1단계 %d개, 2단계 %d개 (계획 %d/%d)%s\n",
		l1, l2, len(p.level1), len(p.level2),
		mark(l1 == len(p.level1) && l2 == len(p.level2)))
	if l1 != len(p.level1) || l2 != len(p.level2) {
		return fmt.Errorf("카테고리 개수가 계획과 다르다")
	}

	// 3단계가 생기지 않았는지. 스키마 트리거가 막지만 직접 확인한다.
	var depth3 int
	err := sqlDB.QueryRow(`
		SELECT count(*) FROM categories c
		JOIN categories p ON c.parent_id = p.id
		WHERE p.parent_id IS NOT NULL`).Scan(&depth3)
	if err != nil {
		return fmt.Errorf("깊이 검사: %w", err)
	}
	fmt.Printf("3단계 이상 카테고리: %d개%s\n", depth3, mark(depth3 == 0))
	if depth3 != 0 {
		return fmt.Errorf("카테고리가 2단계를 넘었다")
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
