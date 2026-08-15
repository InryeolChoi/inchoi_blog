// import는 노션 덤프를 마크다운으로 변환하는 CLI다.
//
// 지금은 변환 결과를 파일로만 쓴다. DB에는 아직 아무것도 넣지 않는다.
// 전체를 돌리기 전에 소수 페이지로 결과를 눈으로 확인하는 게 목적이다.
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

	"github.com/inryeol/blog/internal/notion"
)

func main() {
	dumpDir := flag.String("dump", "scripts/dump", "노션 덤프 디렉토리 (읽기 전용)")
	outDir := flag.String("out", "out", "마크다운 출력 디렉토리")
	pages := flag.String("pages", "", "변환할 page id 목록 (쉼표 구분). 비우면 전체")
	limit := flag.Int("limit", 0, "변환할 최대 페이지 수 (0이면 제한 없음)")
	verbose := flag.Bool("v", false, "페이지별 리포트를 모두 출력 (기본은 문제 있는 것만)")
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
	for _, path := range files {
		dump, err := notion.LoadDump(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		md, report := notion.Convert(dump)

		outPath := filepath.Join(*outDir, dump.Page.ID+".md")
		if err := os.WriteFile(outPath, []byte(md), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "쓰기(%s): %v\n", outPath, err)
			os.Exit(1)
		}
		reports = append(reports, report)
	}

	printReports(reports, *verbose, *outDir)
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

// printReports는 페이지별 리포트와 전체 집계를 출력한다.
func printReports(reports []notion.Report, verbose bool, outDir string) {
	var (
		okCount        int
		imageMismatch  int
		captionLoss    int
		textShrink     int
		srcImages      int
		outImages      int
		unsupported    = map[string]int{}
		notes          = map[string]int{}
		warnBlockTypes = map[string]int{}
	)

	for _, r := range reports {
		srcImages += r.SourceImages
		outImages += r.OutputImages

		if r.OK() {
			okCount++
		}
		if !r.ImagesMatch() {
			imageMismatch++
		}
		if !r.CaptionsMatch() {
			captionLoss++
		}
		if r.TextShrank() {
			textShrink++
		}

		for _, iss := range r.Issues {
			switch iss.Severity {
			case notion.SevWarn:
				warnBlockTypes[iss.BlockType]++
				if iss.BlockType == "unsupported" || strings.Contains(iss.Message, "모르는 블록") {
					unsupported[iss.BlockType]++
				}
			case notion.SevNote:
				notes[iss.BlockType]++
			}
		}

		if verbose || !r.OK() {
			fmt.Print(r.String())
			fmt.Println()
		}
	}

	fmt.Println(strings.Repeat("=", 72))
	fmt.Printf("변환: %d페이지 → %s/\n", len(reports), outDir)
	fmt.Printf("문제 없음: %d / %d\n", okCount, len(reports))
	fmt.Printf("이미지: 원본 %d개 → 결과 참조 %d개", srcImages, outImages)
	if srcImages == outImages {
		fmt.Println(" ✓")
	} else {
		fmt.Printf(" ✗ (%d개 차이)\n", srcImages-outImages)
	}
	if imageMismatch > 0 {
		fmt.Printf("이미지 개수 불일치: %d페이지\n", imageMismatch)
	}
	if captionLoss > 0 {
		fmt.Printf("캡션 유실: %d페이지\n", captionLoss)
	}
	if textShrink > 0 {
		fmt.Printf("텍스트 급감: %d페이지\n", textShrink)
	}
	if len(warnBlockTypes) > 0 {
		fmt.Printf("경고가 난 블록 타입: %s\n", formatCounts(warnBlockTypes))
	}
	if len(notes) > 0 {
		fmt.Printf("의도적으로 다르게 옮긴 블록: %s\n", formatCounts(notes))
	}
}

func formatCounts(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s×%d", k, counts[k]))
	}
	return strings.Join(parts, ", ")
}
