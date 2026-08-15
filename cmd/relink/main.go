// relink는 글 본문의 노션 링크를 사이트 내부 slug 링크로 바꾼다.
//
// 두 가지 형태를 다룬다:
//   - /p/{페이지ID}      하위 페이지 블록과 link_to_page에서 나온 것
//   - /{32자리 16진수}   본문 인라인 링크(rich_text의 href)에서 나온 것
//
// 이관 직후에는 둘 다 노션 페이지 ID를 가리킨다. slug가 정해지고 나면
// 이 도구로 한 번에 옮긴다. slug를 다시 바꾸면 링크도 같이 바꿔야 한다.
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

	pages, slugByPageID, err := loadPages(sqlDB)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rewritten, stats := rewriteAll(pages, slugByPageID)
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

func loadPages(sqlDB *sql.DB) ([]page, map[string]string, error) {
	rows, err := sqlDB.Query(`SELECT id, notion_page_id, title, slug, body FROM posts`)
	if err != nil {
		return nil, nil, fmt.Errorf("posts 조회: %w", err)
	}
	defer rows.Close()

	var pages []page
	slugByPageID := map[string]string{}
	for rows.Next() {
		var p page
		var notionPageID sql.NullString
		if err := rows.Scan(&p.id, &notionPageID, &p.title, &p.slug, &p.body); err != nil {
			return nil, nil, fmt.Errorf("posts 스캔: %w", err)
		}
		p.notionPageID = notionPageID.String
		pages = append(pages, p)
		if notionPageID.Valid && notionPageID.String != "" {
			slugByPageID[notionPageID.String] = p.slug
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("posts 조회: %w", err)
	}
	// 노션 링크는 하이픈 있는 형태와 없는 형태를 둘 다 쓴다.
	return pages, importer.SlugIndex(slugByPageID), nil
}

// stats는 재작성 전체 집계다.
type stats struct {
	pageLinks      int // /p/{페이지ID} → /{slug}
	inlineLinks    int // 노션 인라인 href → /{slug}
	pagesChanged   int
	unresolved     map[string]int
	unresolvedRefs map[string][]string // 대상 → 그 링크가 있던 글 제목
}

func (s stats) replaced() int { return s.pageLinks + s.inlineLinks }

func rewriteAll(pages []page, slugByPageID map[string]string) ([]page, stats) {
	st := stats{unresolved: map[string]int{}, unresolvedRefs: map[string][]string{}}
	var changed []page

	for _, p := range pages {
		newBody, res := importer.RewriteLinks(p.body, slugByPageID)
		st.pageLinks += res.PageLinks
		st.inlineLinks += res.InlineLinks
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

	pages, slugByPageID, err := loadPages(sqlDB)
	if err != nil {
		return err
	}
	slugs := map[string]bool{}
	for _, p := range pages {
		slugs[p.slug] = true
	}

	remainingPage := 0
	remainingInline := 0
	unresolvedLeft := 0
	badTargets := map[string]int{}
	for _, p := range pages {
		remainingPage += importer.CountPageLinks(p.body)
		remainingInline += importer.CountNotionInlineLinks(p.body)
		for _, target := range importer.SlugLinkTargets(p.body) {
			if !slugs[target] {
				badTargets[target]++
			}
		}
	}
	for _, n := range st.unresolved {
		unresolvedLeft += n
	}

	remaining := remainingPage + remainingInline
	fmt.Printf("\n남은 노션 형태 링크      : %d개 (/p/ %d개, 인라인 %d개)\n",
		remaining, remainingPage, remainingInline)
	fmt.Printf("  그중 가리킬 글이 없어 둔 것: %d개\n", unresolvedLeft)

	unexpected := remaining - unresolvedLeft
	if unexpected != 0 {
		return fmt.Errorf("설명되지 않는 노션 형태 링크가 %d개 있다", unexpected)
	}
	fmt.Println("  설명되지 않는 것          : 0개 ✓")

	if len(badTargets) > 0 {
		targets := make([]string, 0, len(badTargets))
		for t := range badTargets {
			targets = append(targets, t)
		}
		sort.Strings(targets)
		fmt.Printf("\n!! posts.slug에 없는 경로를 가리키는 링크 %d종:\n", len(badTargets))
		for _, t := range targets {
			fmt.Printf("   /%s  %d곳\n", t, badTargets[t])
		}
		return fmt.Errorf("존재하지 않는 slug를 가리키는 링크가 있다")
	}
	fmt.Println("모든 내부 링크가 실제 posts.slug를 가리킨다 ✓")

	// 두 번 돌려도 더 바뀌지 않아야 한다.
	_, again := rewriteAll(pages, slugByPageID)
	if again.replaced() != 0 {
		return fmt.Errorf("다시 돌리니 %d개가 또 바뀐다 (멱등하지 않다)", again.replaced())
	}
	fmt.Println("다시 돌려도 더 바뀌지 않는다 ✓")
	return nil
}
