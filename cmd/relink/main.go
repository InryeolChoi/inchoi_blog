// relink는 글 본문의 노션 링크를 사이트 내부 글 링크(/p/{slug})로 바꾼다.
//
// 세 가지 형태를 다룬다:
//   - /p/{페이지ID}      하위 페이지 블록과 link_to_page에서 나온 것
//   - /{32자리 16진수}   본문 인라인 링크(rich_text의 href)에서 나온 것
//   - /{slug}            옛 relink가 접두사 없이 써둔 것
//
// 앞의 둘은 이관 직후 노션 페이지 ID를 가리킨다. slug가 정해지고 나면 이 도구로
// 한 번에 옮긴다. slug를 다시 바꾸면 링크도 같이 바꿔야 한다.
//
// 세 번째는 옛 버전이 남긴 자국이다. 그때는 /{slug}로 썼는데 서버 라우트는
// GET /p/{slug} 하나뿐이라 그 링크가 전부 카테고리 경로로 잡혀 404가 났다.
//
// 기본은 미리보기다. 실제로 DB를 고치려면 -apply를 준다.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/inryeol/blog/internal/db"
	"github.com/inryeol/blog/internal/importer"
)

// page는 재작성 대상 글 하나다.
type page struct {
	id           int64
	notionPageID string
	title        string
	slug         string
	body         string
}

const rule = "════════════════════════════════════════════════════════════════════════"

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

	pages, slugByPageID, slugs, err := loadPages(sqlDB)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rewritten, stats := rewriteAll(pages, slugByPageID, slugs)
	printPlan(stats, len(pages))

	if !*apply {
		fmt.Println("\n미리보기다. 실제로 고치려면 -apply 를 붙여 다시 실행해라.")
		return
	}

	if err := applyRewrites(sqlDB, rewritten); err != nil {
		fmt.Fprintf(os.Stderr, "\n적용 실패: %v\n", err)
		os.Exit(1)
	}

	if err := verify(sqlDB, stats); err != nil {
		fmt.Fprintf(os.Stderr, "\n검증 실패: %v\n", err)
		os.Exit(1)
	}
}

// loadPages는 글 전체와 함께 두 개의 표를 만든다: 페이지 ID → slug 대응표와
// 실제로 존재하는 slug 집합. 뒤엣것이 있어야 이미 /p/{slug}인 링크와
// 아직 노션 ID를 가리키는 링크를 구분할 수 있다.
func loadPages(sqlDB *sql.DB) ([]page, map[string]string, map[string]bool, error) {
	rows, err := sqlDB.Query(`SELECT id, notion_page_id, title, slug, body FROM posts`)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("posts 조회: %w", err)
	}
	defer rows.Close()

	var pages []page
	slugByPageID := map[string]string{}
	slugs := map[string]bool{}
	for rows.Next() {
		var p page
		var notionPageID sql.NullString
		if err := rows.Scan(&p.id, &notionPageID, &p.title, &p.slug, &p.body); err != nil {
			return nil, nil, nil, fmt.Errorf("posts 스캔: %w", err)
		}
		p.notionPageID = notionPageID.String
		pages = append(pages, p)
		slugs[p.slug] = true
		if notionPageID.Valid && notionPageID.String != "" {
			slugByPageID[notionPageID.String] = p.slug
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, fmt.Errorf("posts 조회: %w", err)
	}
	// 노션 링크는 하이픈 있는 형태와 없는 형태를 둘 다 쓴다.
	return pages, importer.SlugIndex(slugByPageID), slugs, nil
}

// stats는 재작성 전체 집계다.
type stats struct {
	pageLinks      int // /p/{페이지ID} → /p/{slug}
	inlineLinks    int // 노션 인라인 href → /p/{slug}
	bareLinks      int // 접두사 없는 /{slug} → /p/{slug}
	pagesChanged   int
	unresolved     map[string]int
	unresolvedRefs map[string][]string // 대상 → 그 링크가 있던 글 제목
}

func (s stats) replaced() int { return s.pageLinks + s.inlineLinks + s.bareLinks }

func rewriteAll(pages []page, slugByPageID map[string]string, slugs map[string]bool) ([]page, stats) {
	st := stats{unresolved: map[string]int{}, unresolvedRefs: map[string][]string{}}
	var changed []page

	for _, p := range pages {
		newBody, res := importer.RewriteLinks(p.body, slugByPageID, slugs)
		st.pageLinks += res.PageLinks
		st.inlineLinks += res.InlineLinks
		st.bareLinks += res.BareLinks
		for target, n := range res.Unresolved {
			st.unresolved[target] += n
			st.unresolvedRefs[target] = append(st.unresolvedRefs[target], p.title)
		}
		if newBody != p.body {
			p.body = newBody
			changed = append(changed, p)
			st.pagesChanged++
		}
	}
	return changed, st
}

func printPlan(st stats, totalPages int) {
	fmt.Println(rule)
	fmt.Println("링크 재작성")
	fmt.Println(rule)

	unresolvedTotal := 0
	for _, n := range st.unresolved {
		unresolvedTotal += n
	}

	fmt.Printf("\n대상 글                    : %d개\n", totalPages)
	fmt.Printf("바뀌는 링크                : %d개\n", st.replaced())
	fmt.Printf("  하위 페이지 링크 (/p/)   : %d개\n", st.pageLinks)
	fmt.Printf("  본문 인라인 노션 링크    : %d개\n", st.inlineLinks)
	fmt.Printf("  접두사 없는 옛 링크      : %d개\n", st.bareLinks)
	fmt.Printf("바뀌는 글                  : %d개\n", st.pagesChanged)
	fmt.Printf("그대로 두는 링크           : %d개 (고유 대상 %d개)\n", unresolvedTotal, len(st.unresolved))

	if len(st.unresolved) == 0 {
		return
	}

	fmt.Println("\n■ 가리킬 글이 없어 그대로 두는 링크")
	fmt.Println("  posts에 해당 notion_page_id가 없다(인라인 데이터베이스이거나 덤프에 없는 페이지).")
	fmt.Println("  경로만 slug 모양으로 바꾸면 깨진 링크가 멀쩡해 보이므로 노션 형태로 남긴다.")

	targets := make([]string, 0, len(st.unresolved))
	for t := range st.unresolved {
		targets = append(targets, t)
	}
	sort.Slice(targets, func(i, j int) bool {
		if st.unresolved[targets[i]] != st.unresolved[targets[j]] {
			return st.unresolved[targets[i]] > st.unresolved[targets[j]]
		}
		return targets[i] < targets[j]
	})

	shown := targets
	if len(shown) > 10 {
		shown = shown[:10]
	}
	for _, t := range shown {
		fmt.Printf("  %s  %d곳  (예: %s)\n", t, st.unresolved[t], st.unresolvedRefs[t][0])
	}
	if len(targets) > len(shown) {
		fmt.Printf("  … 외 %d개 대상\n", len(targets)-len(shown))
	}
}

// applyRewrites는 바뀐 본문을 한 트랜잭션에서 반영한다.
func applyRewrites(sqlDB *sql.DB, pages []page) error {
	tx, err := sqlDB.Begin()
	if err != nil {
		return fmt.Errorf("트랜잭션 시작: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE posts SET body = ?, updated_at = datetime('now') WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("UPDATE 준비: %w", err)
	}
	defer stmt.Close()

	for _, p := range pages {
		if _, err := stmt.Exec(p.body, p.id); err != nil {
			return fmt.Errorf("UPDATE(%s): %w", p.notionPageID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("커밋: %w", err)
	}
	fmt.Printf("\n%d개 글의 본문을 갱신했다.\n", len(pages))
	return nil
}

// verify는 반영 후 DB를 다시 읽어 확인한다.
func verify(sqlDB *sql.DB, st stats) error {
	fmt.Println()
	fmt.Println(rule)
	fmt.Println("검증")
	fmt.Println(rule)

	pages, slugByPageID, slugs, err := loadPages(sqlDB)
	if err != nil {
		return err
	}

	postLinks := 0
	remainingPage := 0
	remainingInline := 0
	unresolvedLeft := 0
	bare := map[string]int{}
	for _, p := range pages {
		postLinks += importer.CountPostLinks(p.body, slugs)
		remainingPage += importer.CountNotionPageLinks(p.body, slugs)
		remainingInline += importer.CountNotionInlineLinks(p.body)
		for _, target := range importer.BareSlugLinks(p.body, slugs) {
			bare[target]++
		}
	}
	for _, n := range st.unresolved {
		unresolvedLeft += n
	}

	fmt.Printf("\n글을 가리키는 /p/ 링크   : %d개\n", postLinks)

	remaining := remainingPage + remainingInline
	fmt.Printf("남은 노션 형태 링크      : %d개 (/p/ %d개, 인라인 %d개)\n",
		remaining, remainingPage, remainingInline)
	fmt.Printf("  그중 가리킬 글이 없어 둔 것: %d개\n", unresolvedLeft)

	unexpected := remaining - unresolvedLeft
	if unexpected != 0 {
		return fmt.Errorf("설명되지 않는 노션 형태 링크가 %d개 있다", unexpected)
	}
	fmt.Println("  설명되지 않는 것          : 0개 ✓")

	// 서버 라우트는 GET /p/{slug} 하나뿐이다. 접두사 없이 slug를 가리키는 링크가
	// 남아 있으면 GET /{l1} 카테고리 라우트에 잡혀서 404가 난다.
	if len(bare) > 0 {
		targets := make([]string, 0, len(bare))
		total := 0
		for t, n := range bare {
			targets = append(targets, t)
			total += n
		}
		sort.Strings(targets)
		fmt.Printf("\n!! /p/ 없이 글을 가리키는 링크 %d개 (대상 %d종):\n", total, len(targets))
		shown := targets
		if len(shown) > 10 {
			shown = shown[:10]
		}
		for _, t := range shown {
			fmt.Printf("   /%s  %d곳\n", t, bare[t])
		}
		if len(targets) > len(shown) {
			fmt.Printf("   … 외 %d개 대상\n", len(targets)-len(shown))
		}
		return fmt.Errorf("접두사 없는 글 링크가 %d개 남아 있다", total)
	}
	fmt.Println("/p/ 없이 글을 가리키는 링크: 0개 ✓")

	// 두 번 돌려도 더 바뀌지 않아야 한다.
	_, again := rewriteAll(pages, slugByPageID, slugs)
	if again.replaced() != 0 {
		return fmt.Errorf("다시 돌리니 %d개가 또 바뀐다 (멱등하지 않다)", again.replaced())
	}
	fmt.Println("다시 돌려도 더 바뀌지 않는다 ✓")
	return nil
}
