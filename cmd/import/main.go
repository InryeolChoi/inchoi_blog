// import는 노션 덤프를 마크다운으로 변환하고, 원하면 DB에 넣는 CLI다.
//
// -db를 주지 않으면 변환 결과를 out/에만 쓰고 DB는 건드리지 않는다.
//
// 덤프 디렉토리는 읽기 전용으로만 다룬다. 재수집에 43분이 걸리므로 이 프로그램은
// 어떤 경우에도 덤프에 쓰거나 지우지 않는다.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/inryeol/blog"
	"github.com/inryeol/blog/internal/db"
	"github.com/inryeol/blog/internal/importer"
	"github.com/inryeol/blog/internal/notion"
)

// convertedPage는 변환 결과 한 건이다. out/에 쓴 것과 DB에 넣는 것이 같은
// 문자열이어야 해서, 파일을 다시 읽지 않고 이 값을 그대로 쓴다.
type convertedPage struct {
	pageID    string
	markdown  string
	createdAt *time.Time
	imageURLs map[string]string
}

// failure는 변환 자체가 실패한 페이지다.
type failure struct {
	path string
	err  error
}

func main() {
	dumpDir := flag.String("dump", "scripts/dump", "노션 덤프 디렉토리 (읽기 전용)")
	outDir := flag.String("out", "out", "마크다운 출력 디렉토리")
	pages := flag.String("pages", "", "변환할 page id 목록 (쉼표 구분). 비우면 전체")
	limit := flag.Int("limit", 0, "변환할 최대 페이지 수 (0이면 제한 없음)")
	verbose := flag.Bool("v", false, "페이지별 리포트를 모두 출력")
	dbPath := flag.String("db", "", "SQLite 파일 경로. 지정하면 DB에도 넣는다 (비우면 마크다운만)")
	statusCSV := flag.String("status", "scripts/notion-page-status.csv", "페이지별 status가 든 CSV")
	flag.Parse()

	files, err := selectFiles(*dumpDir, *pages, *limit)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "변환할 페이지가 없다")
		os.Exit(1)
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "출력 디렉토리 생성: %v\n", err)
		os.Exit(1)
	}

	reports := make([]notion.Report, 0, len(files))
	converted := make([]convertedPage, 0, len(files))
	var failures []failure

	for _, path := range files {
		dump, err := notion.LoadDump(path)
		if err != nil {
			failures = append(failures, failure{path, err})
			continue
		}

		md, report := notion.Convert(dump)

		outPath := filepath.Join(*outDir, dump.Page.ID+".md")
		if err := os.WriteFile(outPath, []byte(md), 0o644); err != nil {
			failures = append(failures, failure{path, err})
			continue
		}
		reports = append(reports, report)
		converted = append(converted, convertedPage{
			pageID:    dump.Page.ID,
			markdown:  md,
			createdAt: parseNotionTime(dump.Page.CreatedTime),
			imageURLs: dump.ImageSources(),
		})

		if *verbose {
			fmt.Print(report.String())
			fmt.Println()
		}
	}

	printFullReport(reports, failures, *outDir)

	if *dbPath == "" {
		return
	}
	res, err := runImport(*dbPath, *statusCSV, *dumpDir, converted)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nDB 이관 실패: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()
	fmt.Println(rule)
	fmt.Println("DB 이관")
	fmt.Println(rule)
	fmt.Printf("\nposts  %d건\nimages %d건\n→ %s\n", res.posts, res.images, *dbPath)
	if res.overwrote > 0 {
		fmt.Printf("\n!! 이번에 넣은 글 중 %d건은 이미 있던 글이라 본문을 덮어썼다.\n", res.overwrote)
		fmt.Println("   본문은 변환기가 만든 것으로 돌아갔으므로, 링크를 slug로 바꿔뒀다면")
		fmt.Println("   `go run ./cmd/relink -db <db> -apply` 를 다시 돌려야 한다.")
	}
	if len(res.skipped) > 0 {
		fmt.Printf("\n!! status CSV에 없어서 건너뛴 페이지 %d개:\n", len(res.skipped))
		for _, id := range res.skipped {
			fmt.Printf("   %s\n", id)
		}
	}
}

// selectFiles는 변환할 덤프 파일 경로를 고른다.
func selectFiles(dumpDir, pages string, limit int) ([]string, error) {
	if pages != "" {
		var files []string
		for _, id := range strings.Split(pages, ",") {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			path := filepath.Join(dumpDir, id+".json")
			if _, err := os.Stat(path); err != nil {
				return nil, fmt.Errorf("덤프가 없다: %s", path)
			}
			files = append(files, path)
		}
		return files, nil
	}

	entries, err := os.ReadDir(dumpDir)
	if err != nil {
		return nil, fmt.Errorf("덤프 디렉토리 읽기(%s): %w", dumpDir, err)
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			files = append(files, filepath.Join(dumpDir, entry.Name()))
		}
	}
	sort.Strings(files)
	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}
	return files, nil
}

const rule = "════════════════════════════════════════════════════════════════════════"

func printFullReport(reports []notion.Report, failures []failure, outDir string) {
	printVerification(reports, failures)
	printSuspectedLoss(reports)
	printStats(reports, failures, outDir)
}

// ---------- 전수 검증 ----------

func printVerification(reports []notion.Report, failures []failure) {
	fmt.Println(rule)
	fmt.Println("전수 검증")
	fmt.Println(rule)

	if len(failures) > 0 {
		fmt.Printf("\n[변환 실패] %d건\n", len(failures))
		for _, f := range failures {
			fmt.Printf("  %s\n    %v\n", filepath.Base(f.path), f.err)
		}
	}

	// 이미지 개수 불일치
	fmt.Println("\n■ 이미지 개수 불일치 (원본 블록 vs 마크다운 참조)")
	var mismatched []notion.Report
	for _, r := range reports {
		if !r.ImagesMatch() {
			mismatched = append(mismatched, r)
		}
	}
	if len(mismatched) == 0 {
		fmt.Println("  없음 ✓")
	} else {
		for _, r := range mismatched {
			fmt.Printf("  %s  원본 %d → 결과 %d  (%s)\n",
				r.PageID, r.SourceImages, r.OutputImages, titleOf(r))
		}
	}

	// 외부 URL로 남은 이미지
	fmt.Println("\n■ 로컬 파일이 없어 외부 URL로 남은 이미지")
	total := 0
	found := false
	for _, r := range reports {
		issues := r.IssuesOfKind(notion.KindExternalImage)
		if len(issues) == 0 {
			continue
		}
		found = true
		total += len(issues)
		fmt.Printf("  %s  %d건  (%s)\n", r.PageID, len(issues), titleOf(r))
		for _, iss := range issues {
			fmt.Printf("      %s\n", iss.Message)
		}
	}
	if !found {
		fmt.Println("  없음 ✓")
	} else {
		fmt.Printf("  ─ 합계 %d건\n", total)
	}

	// 미지원 / 모르는 블록 타입
	fmt.Println("\n■ 미지원·누락 블록 타입")
	type loc struct {
		pageID, title, path, msg string
	}
	byType := map[string][]loc{}
	for _, r := range reports {
		for _, kind := range []notion.Kind{notion.KindUnknownBlock, notion.KindUnsupportedBlock} {
			for _, iss := range r.IssuesOfKind(kind) {
				key := string(kind) + " / " + iss.BlockType
				byType[key] = append(byType[key], loc{r.PageID, titleOf(r), iss.Path, iss.Message})
			}
		}
	}
	if len(byType) == 0 {
		fmt.Println("  없음 ✓")
	} else {
		keys := sortedKeysByCount(byType)
		for _, k := range keys {
			locs := byType[k]
			fmt.Printf("  %s — %d건\n", k, len(locs))
			shown := locs
			if len(shown) > 5 {
				shown = shown[:5]
			}
			for _, l := range shown {
				fmt.Printf("      %s  %s  (%s)\n", l.pageID, l.path, l.title)
			}
			if len(locs) > len(shown) {
				fmt.Printf("      … 외 %d건\n", len(locs)-len(shown))
			}
		}
	}
}

// ---------- 유실 의심 ----------

func printSuspectedLoss(reports []notion.Report) {
	fmt.Println()
	fmt.Println(rule)
	fmt.Println("유실 의심")
	fmt.Println(rule)

	// 길이 비율이 낮은 순
	type ratioRow struct {
		r     notion.Report
		ratio float64
	}
	var rows []ratioRow
	for _, r := range reports {
		if r.SourceTextLen == 0 {
			continue // 원본에 글자가 없으면 비율을 따질 게 없다
		}
		rows = append(rows, ratioRow{r, float64(r.OutputTextLen) / float64(r.SourceTextLen)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ratio < rows[j].ratio })

	fmt.Println("\n■ 원본 대비 결과가 짧은 페이지 상위 20개")
	fmt.Println("  (마크다운 문법이 붙으므로 정상이면 1.0보다 큼. 1.0 미만이면 내용이 빠진 것)")
	limit := 20
	if len(rows) < limit {
		limit = len(rows)
	}
	if limit == 0 {
		fmt.Println("  대상 없음")
	}
	for _, row := range rows[:limit] {
		mark := " "
		if row.r.TextShrank() {
			mark = "✗"
		}
		fmt.Printf("  %s %.2fx  원본 %5d자 → 결과 %5d자  %s  (%s)\n",
			mark, row.ratio, row.r.SourceTextLen, row.r.OutputTextLen, row.r.PageID, titleOf(row.r))
	}

	// 결과가 비었거나 극단적으로 짧은 페이지.
	//
	// 원본이 비어 있어서 결과도 빈 것과, 원본에 내용이 있는데 결과가 빈 것은
	// 완전히 다른 얘기다. 앞의 것은 노션에 원래 빈 페이지가 많다는 뜻일 뿐이고,
	// 뒤의 것만 변환기가 내용을 잃었다는 신호다.
	fmt.Println("\n■ 결과가 비었거나 극단적으로 짧은 페이지")
	var lostAll, emptySource, tiny []notion.Report
	for _, r := range reports {
		switch {
		case r.OutputTextLen == 0 && r.SourceTextLen > 0:
			lostAll = append(lostAll, r)
		case r.OutputTextLen == 0:
			emptySource = append(emptySource, r)
		case r.OutputTextLen < 20 && r.SourceTextLen > 0:
			tiny = append(tiny, r)
		}
	}

	fmt.Printf("  원본에 내용이 있는데 결과가 0자 (진짜 유실): %d건", len(lostAll))
	if len(lostAll) > 0 {
		fmt.Println()
		for _, r := range lostAll {
			fmt.Printf("      %s  원본 %d자, 블록 %s  (%s)\n",
				r.PageID, r.SourceTextLen, briefBlockTypes(r.BlockTypes), titleOf(r))
		}
	} else {
		fmt.Println(" ✓")
	}

	fmt.Printf("  원본도 0자라서 결과도 0자 (유실 아님, 노션의 빈 페이지): %d건\n", len(emptySource))

	fmt.Printf("  결과 20자 미만: %d건", len(tiny))
	if len(tiny) > 0 {
		fmt.Println()
		for _, r := range tiny {
			fmt.Printf("      %s  원본 %d자 → 결과 %d자, 블록 %s  (%s)\n",
				r.PageID, r.SourceTextLen, r.OutputTextLen,
				briefBlockTypes(r.BlockTypes), titleOf(r))
		}
	} else {
		fmt.Println(" ✓")
	}
}

// ---------- 통계 ----------

func printStats(reports []notion.Report, failures []failure, outDir string) {
	fmt.Println()
	fmt.Println(rule)
	fmt.Println("통계")
	fmt.Println(rule)

	var (
		okCount     int
		srcImages   int
		outImages   int
		srcCaptions int
		outCaptions int
		continued   int
		restarted   int
		blockTypes  = map[string]int{}
		kinds       = map[notion.Kind]int{}
	)
	for _, r := range reports {
		if r.OK() {
			okCount++
		}
		srcImages += r.SourceImages
		outImages += r.OutputImages
		srcCaptions += r.SourceCaptions
		outCaptions += r.OutputCaptions
		continued += r.NumberingContinued
		restarted += r.NumberingRestarted
		for t, n := range r.BlockTypes {
			blockTypes[t] += n
		}
		for _, iss := range r.Issues {
			kinds[iss.Kind]++
		}
	}

	fmt.Printf("\n변환 성공 %d개 / 실패 %d개 → %s/\n", len(reports), len(failures), outDir)
	fmt.Printf("문제 없음(OK) %d개 / 확인 필요 %d개\n", okCount, len(reports)-okCount)
	fmt.Printf("이미지  원본 %d개 → 결과 참조 %d개%s\n", srcImages, outImages, checkMark(srcImages == outImages))
	fmt.Printf("캡션    원본 %d개 → 결과 %d개%s\n", srcCaptions, outCaptions, checkMark(srcCaptions == outCaptions))

	fmt.Println("\n■ 블록 타입별 처리 건수")
	for _, t := range sortedKeysByValue(blockTypes) {
		fmt.Printf("  %8d  %s\n", blockTypes[t], t)
	}

	fmt.Println("\n■ 목록 번호")
	fmt.Printf("  이어감  %d건  (코드·그림·수식·표·빈 문단이 끼었을 때)\n", continued)
	fmt.Printf("  재시작  %d건  (제목이나 도입 문단이 끼었을 때)\n", restarted)
	fmt.Printf("  합계    %d건\n", continued+restarted)

	if len(kinds) > 0 {
		fmt.Println("\n■ 이슈 종류별")
		type kv struct {
			k notion.Kind
			v int
		}
		var items []kv
		for k, v := range kinds {
			items = append(items, kv{k, v})
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].v != items[j].v {
				return items[i].v > items[j].v
			}
			return items[i].k < items[j].k
		})
		for _, it := range items {
			fmt.Printf("  %8d  %s\n", it.v, it.k)
		}
	}
}

// ---------- 보조 ----------

func titleOf(r notion.Report) string {
	if r.Title == "" {
		return "(제목 없음)"
	}
	return r.Title
}

func checkMark(ok bool) string {
	if ok {
		return " ✓"
	}
	return " ✗"
}

func briefBlockTypes(counts map[string]int) string {
	if len(counts) == 0 {
		return "(없음)"
	}
	keys := sortedKeysByValue(counts)
	if len(keys) > 4 {
		keys = keys[:4]
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s×%d", k, counts[k]))
	}
	return strings.Join(parts, ", ")
}

func sortedKeysByValue(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys
}

func sortedKeysByCount[T any](m map[string][]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(m[keys[i]]) != len(m[keys[j]]) {
			return len(m[keys[i]]) > len(m[keys[j]])
		}
		return keys[i] < keys[j]
	})
	return keys
}

// parseNotionTime은 노션의 ISO8601 시각을 파싱한다. 형식이 다르면 nil을 준다.
func parseNotionTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	t = t.UTC()
	return &t
}

// ---------- DB 이관 ----------

// importResult는 DB 이관 결과 요약이다.
type importResult struct {
	posts     int
	images    int
	overwrote int      // 이번 실행 전에 이미 있던 글 수
	skipped   []string // status CSV에 없어서 건너뛴 페이지
}

// runImport는 변환된 글과 이미지 파일을 DB에 넣는다.
//
// 전부 한 트랜잭션에서 처리한다. 중간에 실패하면 아무것도 들어가지 않는다.
// 반쯤 들어간 DB를 손으로 정리하는 것보다 통째로 다시 도는 게 낫다.
func runImport(dbPath, statusCSV, dumpDir string, converted []convertedPage) (*importResult, error) {
	meta, err := importer.LoadPageMeta(statusCSV)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer sqlDB.Close()

	applied, err := db.Migrate(sqlDB, blog.MigrationsFS())
	if err != nil {
		return nil, fmt.Errorf("마이그레이션: %w", err)
	}
	for _, name := range applied {
		fmt.Printf("마이그레이션 적용: %s\n", name)
	}

	// 이번에 넣을 페이지 중 이미 DB에 있는 것만 센다. 그것들만 본문이 덮어써지고,
	// relink로 고쳐둔 링크가 같이 되돌아간다. 전체 글 수를 세면 신규만 넣는
	// 경우에도 경고가 떠서, 진짜 덮어쓴 때와 구분이 안 된다.
	existing := 0
	for _, cp := range converted {
		var n int
		if err := sqlDB.QueryRow(
			`SELECT count(*) FROM posts WHERE notion_page_id = ?`, cp.pageID).Scan(&n); err != nil {
			return nil, fmt.Errorf("기존 글 확인(%s): %w", cp.pageID, err)
		}
		existing += n
	}

	tx, err := sqlDB.Begin()
	if err != nil {
		return nil, fmt.Errorf("트랜잭션 시작: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	res := &importResult{overwrote: existing}

	for _, cp := range converted {
		m, ok := meta[cp.pageID]
		if !ok {
			// status를 모르는 채로 기본값을 찍으면 나중에 무엇이 추정값인지 알 수 없다.
			res.skipped = append(res.skipped, cp.pageID)
			continue
		}
		post := importer.Post{
			Slug:              cp.pageID, // 지금은 페이지 ID 그대로. 나중에 제목 기반으로 다시 쓴다.
			Title:             m.Title,
			Body:              cp.markdown,
			Status:            m.Status,
			Source:            "notion",
			NotionPageID:      cp.pageID,
			OriginalPath:      m.FullPath,
			OriginalCreatedAt: cp.createdAt,
		}
		if err := importer.UpsertPost(tx, post, now); err != nil {
			return nil, err
		}
		res.posts++
	}

	imageURLs := map[string]string{}
	for _, cp := range converted {
		for sha, url := range cp.imageURLs {
			imageURLs[sha] = url
		}
	}

	imageDir := filepath.Join(dumpDir, "images")
	entries, err := os.ReadDir(imageDir)
	if err != nil {
		return nil, fmt.Errorf("이미지 디렉토리 읽기(%s): %w", imageDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		mime, ok := importer.MIMEForFile(name)
		if !ok {
			return nil, fmt.Errorf("확장자를 모르는 이미지 파일이다: %s", name)
		}
		data, err := os.ReadFile(filepath.Join(imageDir, name))
		if err != nil {
			return nil, fmt.Errorf("이미지 읽기(%s): %w", name, err)
		}
		sha := strings.TrimSuffix(name, filepath.Ext(name))
		if err := importer.UpsertImage(tx, importer.Image{
			SHA256:      sha,
			Data:        data,
			MIME:        mime,
			OriginalURL: imageURLs[sha],
		}, now); err != nil {
			return nil, err
		}
		res.images++
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("커밋: %w", err)
	}
	return res, nil
}
