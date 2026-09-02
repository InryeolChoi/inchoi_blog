// Command importgh는 GitHub 저장소의 마크다운 노트를 DB로 옮긴다.
//
// # 노션 이관과 무엇이 다른가
//
// 노션은 `scripts/dump/`가 고정이라 "몇 번을 돌려도 같은 결과"가 목표였다.
// GitHub은 저장소가 계속 바뀌므로 목표가 하나 다르다 — **"다시 돌리면 지금의
// 저장소와 같아진다"**이다. 그래서 멱등 키가 `source_ref`(출처 경로)다.
//
// # 무엇을 안 하나
//
//   - **분류를 만들지 않는다.** 분류 트리는 categorize·regroup의 일이고,
//     여기서도 만들면 두 곳이 같은 트리를 손대게 된다. 표에 적은 분류가
//     DB에 없으면 실패한다.
//
//   - **본문을 고치지 않는다.** 딱 하나 예외가 아래 escapeBareTags다.
//
//   - **이미지는 받는다** (2026-09-02에 더했다, images.go). 본문의
//     `![](./images/x.png)`를 저장소에서 받아 sha256으로 BLOB에 넣고
//     `/img/{sha256}`으로 다시 쓴다 — 노션 이미지와 같은 표, 같은 멱등 키다.
//
//     go run ./cmd/importgh -db blog.db          # 미리보기 (기본)
//     go run ./cmd/importgh -db blog.db -apply   # 반영
package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/inryeol/blog"
	"github.com/inryeol/blog/internal/curation"
	"github.com/inryeol/blog/internal/db"
	"github.com/inryeol/blog/internal/importer"
)

func main() {
	dbPath := flag.String("db", "", "SQLite 파일 (필수)")
	apply := flag.Bool("apply", false, "실제로 DB에 반영한다")
	flag.Parse()

	if *dbPath == "" {
		log.Fatal("-db 를 줘야 한다")
	}
	sqlDB, err := db.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer sqlDB.Close()
	if _, err := db.Migrate(sqlDB, blog.MigrationsFS()); err != nil {
		log.Fatalf("마이그레이션: %v", err)
	}

	ctx := context.Background()
	docs, err := fetchAll(ctx)
	if err != nil {
		log.Fatal(err)
	}
	// **그림을 받은 뒤에 본문을 다시 쓴다.** sha256을 알아야 `/img/{sha256}`을
	// 적을 수 있어서다. 미리보기에서도 받는다 — 그래야 리포트가 "몇 장을
	// 옮기고 몇 장을 못 찾았나"를 실제 값으로 말한다.
	images, err := fetchImages(ctx, docs)
	if err != nil {
		log.Fatal(err)
	}
	missing := rewriteAll(docs, images)

	report(docs)
	reportImages(images, missing)
	if !*apply {
		fmt.Println("\n미리보기다. 실제로 넣으려면 -apply 를 붙여 다시 실행해라.")
		return
	}
	if err := store(sqlDB, docs, images); err != nil {
		log.Fatal(err)
	}
}

// fetchImages는 글들이 쓰는 그림을 저장소에서 받아온다. 같은 경로가 여러 글에
// 나와도 한 번만 받는다.
func fetchImages(ctx context.Context, docs []fetched) (map[string]repoImage, error) {
	out := map[string]repoImage{}
	for _, d := range docs {
		for _, p := range d.images {
			if _, ok := out[p]; ok {
				continue
			}
			img, err := fetchImage(ctx, d.src, p)
			if err != nil {
				// **한 장이 없다고 전체를 멈추지 않는다.** 그림이 빠진 것은
				// 글이 안 들어오는 것보다 가벼운 사고고, 아래 리포트가 어느
				// 것을 못 받았는지 이름으로 짚어 준다.
				fmt.Fprintf(os.Stderr, "!! 그림을 못 받았다: %v\n", err)
				continue
			}
			out[p] = img
		}
	}
	return out, nil
}

// rewriteAll은 받아온 그림으로 본문의 참조를 바꾼다. 못 바꾼 경로를 돌려준다.
func rewriteAll(docs []fetched, images map[string]repoImage) []string {
	var missing []string
	for i := range docs {
		body, want := rewriteImages(docs[i].body, docs[i].doc.Path, images)
		docs[i].body = body
		missing = append(missing, want...)
	}
	return missing
}

func reportImages(images map[string]repoImage, missing []string) {
	var bytes int
	for _, img := range images {
		bytes += len(img.Data)
	}
	fmt.Printf("\n■ 그림\n  옮김 %d장 (%.1f MB)\n", len(images), float64(bytes)/(1<<20))
	if len(missing) > 0 {
		fmt.Printf("  **못 옮긴 참조 %d개 — 본문에 원문 그대로 남는다**\n", len(missing))
		for _, m := range missing {
			fmt.Printf("    %s\n", m)
		}
	}
}

// fetched는 가져온 파일 하나다.
type fetched struct {
	src  curation.GitHubSource
	doc  curation.GitHubDoc
	body string
	// fixed는 escapeBareTags가 손본 줄이다. 리포트가 짚어준다.
	fixed []string
	// createdAt은 이 파일을 **처음 커밋한 때**다. 목록에 찍히는 작성일이
	// 이것이다 — 이관한 날이 아니라 실제로 쓴 날이어야 한다.
	createdAt time.Time
	// commits는 이 파일을 건드린 커밋 수다. 리포트에만 쓴다.
	commits int
	// images는 이 글이 쓰는 저장소 안 그림의 경로다. fetchAll이 모으고
	// storeImages가 받아온다.
	images []string
	// bold는 짝이 안 되어 <strong>으로 바꾼 굵게 표시다. 리포트가 짚어준다.
	bold []string
}

// fetchAll은 표에 적힌 파일을 전부 받아온다.
func fetchAll(ctx context.Context) ([]fetched, error) {
	var out []fetched
	for _, src := range curation.GitHubSources {
		for _, doc := range src.Docs {
			raw, err := fetchFile(ctx, src, doc)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", src.SourceRef(doc), err)
			}
			body, fixed, bold := prepareBody(raw)
			first, n, err := firstCommit(ctx, src, doc)
			if err != nil {
				return nil, fmt.Errorf("%s 커밋 이력: %w", src.SourceRef(doc), err)
			}
			out = append(out, fetched{
				src: src, doc: doc, body: body, fixed: fixed, bold: bold,
				createdAt: first, commits: n,
				images: collectImagePaths(body, doc.Path),
			})
		}
	}
	return out, nil
}

func fetchFile(ctx context.Context, src curation.GitHubSource, doc curation.GitHubDoc) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", src.Repo, doc.Path)
	if src.Ref != "" {
		url += "?ref=" + src.Ref
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	// 공개 저장소라 토큰 없이도 되지만, 있으면 요청 한도가 60/시간에서
	// 5,000/시간이 된다. gh가 깔려 있으면 그 토큰을 쓴다.
	if tok := githubToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub이 %s를 줬다", res.Status)
	}
	var payload struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.Encoding != "base64" {
		return "", fmt.Errorf("모르는 인코딩: %s", payload.Encoding)
	}
	data, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(payload.Content, "\n", ""))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// firstCommit은 이 파일을 **처음 커밋한 때**와 건드린 커밋 수를 준다.
//
// # 왜 이관한 날이 아닌가
//
// 목록에 찍히는 날짜는 "실제로 쓴 날"이어야 한다. 노션 이관도 같은 이유로
// 이관 시각이 아니라 `original_created_at`(원본 작성일)을 쓴다 — 그 규칙을
// GitHub에서도 그대로 지킨다. 여기서 그 값은 **git이 이미 알고 있다.**
//
// # 왜 마지막 원소인가
//
// GitHub은 커밋을 **새것부터** 준다. 그래서 목록의 마지막이 가장 오래된
// 커밋, 곧 이 파일이 생긴 때다.
//
// per_page는 100이다. 파일 하나를 100번 넘게 고쳤으면 그중 100번째가 "처음"이
// 되어 실제보다 늦어지는데, 그때는 리포트의 커밋 수가 100으로 붙어 있어
// 눈에 띈다 — 조용히 틀리지는 않는다.
func firstCommit(ctx context.Context, src curation.GitHubSource, doc curation.GitHubDoc) (time.Time, int, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/commits?path=%s&per_page=100",
		src.Repo, doc.Path)
	if src.Ref != "" {
		url += "&sha=" + src.Ref
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return time.Time{}, 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if tok := githubToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return time.Time{}, 0, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return time.Time{}, 0, fmt.Errorf("GitHub이 %s를 줬다", res.Status)
	}
	var commits []struct {
		Commit struct {
			Author struct {
				Date time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}
	if err := json.NewDecoder(res.Body).Decode(&commits); err != nil {
		return time.Time{}, 0, err
	}
	if len(commits) == 0 {
		// 커밋이 없을 수는 없지만, 없다고 조용히 오늘 날짜를 지어내지 않는다.
		return time.Time{}, 0, fmt.Errorf("커밋 이력이 비어 있다")
	}
	return commits[len(commits)-1].Commit.Author.Date.UTC(), len(commits), nil
}

func githubToken() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	return os.Getenv("GH_TOKEN")
}

// prepareBody는 받아온 파일을 본문으로 다듬는다.
//
// **세 손질이 한 쌍이다.** 하나만 빠져도 화면이 어긋난다:
//
//	escapeBareTags   `<javascript>`가 화면에서 사라지는 것을 막는다
//	dropLeadingTitle 글 제목이 페이지에 두 번 나오는 것을 막는다
//	promoteHeadings  제목을 뗀 자리만큼 층을 메운다
//
// 한 함수로 묶은 이유는 **호출 순서와 조합 자체가 규칙**이기 때문이다.
// 떼기만 하고 안 올리면 h1 다음이 h3이 되고, 올리기만 하고 안 떼면 글
// 제목이 h2로 한 번 더 나온다.
func prepareBody(raw string) (string, []string, []string) {
	body, fixed := escapeBareTags(raw)
	body, bold := fixUnpairedBold(body)
	return promoteHeadings(dropLeadingTitle(body)), fixed, bold
}

// bareTag는 줄 전체가 `<낱말>` 하나인 것을 찾는다.
//
// 글쓴이는 `<특징>`, `<예시>` 처럼 꺾쇠를 **절 제목 표시**로 썼다. 한글은
// CommonMark의 태그 이름 규칙(`[A-Za-z][A-Za-z0-9-]*`)에 안 맞아서 그대로
// 글자로 남지만, `<javascript>` 처럼 라틴 낱말이면 **raw HTML로 통과해
// 화면에서 사라진다.** 브라우저가 모르는 요소라 아무것도 안 그린다.
//
// 조용히 사라지는 것이라 이관에서 잡는다 — 이 프로젝트가 "변환기는 미지원
// 블록을 조용히 버리지 않는다"고 정해둔 것과 같은 자리다.
var bareTag = regexp.MustCompile(`^<([A-Za-z][A-Za-z0-9-]*)>\s*$`)

func escapeBareTags(body string) (string, []string) {
	lines := strings.Split(body, "\n")
	var fixed []string
	inFence := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		// 코드 블록 안은 어차피 글자로 나온다. 건드리면 코드가 바뀐다.
		if inFence {
			continue
		}
		if m := bareTag.FindStringSubmatch(line); m != nil {
			lines[i] = "&lt;" + m[1] + "&gt;"
			fixed = append(fixed, m[0])
		}
	}
	return strings.Join(lines, "\n"), fixed
}

// dropLeadingTitle은 파일 첫머리의 `# 제목` 한 줄을 뗀다.
//
// 페이지의 <h1>은 템플릿이 그리는 글 제목이다(CLAUDE.md의 "본문 제목은 한
// 단계 내려서 그린다"). 본문에도 남겨두면 같은 말이 두 번 나온다.
//
// **뗀 뒤에는 promoteHeadings로 층을 메워야 한다.** 안 그러면 파일의 `##`가
// 그대로 남아 화면에서 h3이 되고, h1 다음이 h3이라 층이 하나 빈다. 목차
// 들여쓰기(`toc-h2`/`toc-h3`)도 그만큼 밀린다.
func dropLeadingTitle(body string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "# ") {
			return strings.TrimLeft(strings.Join(lines[i+1:], "\n"), "\n")
		}
		break // 첫 내용이 제목이 아니면 건드리지 않는다
	}
	return body
}

// atxHeading은 `#`으로 시작하는 제목 줄이다.
var atxHeading = regexp.MustCompile(`^(#{1,6})(\s)`)

// promoteHeadings는 제목을 한 단계씩 올린다(`##` → `#`).
//
// 이 저장소의 파일은 `#`이 문서 제목이고 절은 `##`부터다. 문서 제목을 뗀
// 뒤에도 절이 `##`이면 화면에서 h3이 되어 h1 바로 아래 층이 빈다.
//
// **코드 블록 안은 건드리지 않는다.** 셸 주석(`# 주석`)이나 마크다운을
// 보여주는 예제가 제목으로 둔갑한다.
func promoteHeadings(body string) string {
	lines := strings.Split(body, "\n")
	inFence := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		m := atxHeading.FindStringSubmatch(line)
		// `#` 하나짜리는 더 올릴 곳이 없다. 그대로 둔다.
		if m == nil || len(m[1]) < 2 {
			continue
		}
		lines[i] = m[1][1:] + line[len(m[1]):]
	}
	return strings.Join(lines, "\n")
}

func report(docs []fetched) {
	fmt.Println(strings.Repeat("─", 68))
	fmt.Printf("GitHub 이관  대상 %d편\n", len(docs))
	fmt.Println(strings.Repeat("─", 68))
	total := 0
	for _, d := range docs {
		total += len(d.body)
		note := ""
		if len(d.fixed) > 0 {
			note = fmt.Sprintf("   ← 꺾쇠 표시 %d개를 글자로 바꿈 %v", len(d.fixed), d.fixed)
		}
		warn := ""
		if d.commits >= 100 {
			warn = "  ← 커밋 100개(상한). 최초 작성일이 실제보다 늦을 수 있다"
		}
		fmt.Printf("  %2d  %-40s %6d자  %s (커밋 %d)%s%s\n",
			d.doc.SortOrder, d.doc.Title, len(d.body),
			d.createdAt.Format("2006-01-02"), d.commits, warn, note)
	}
	fmt.Printf("\n본문 합계 %d자\n", total)
	// 짝이 안 되어 <strong>으로 바꾼 굵게 표시. 그냥 두면 별표가 글자로 보인다.
	boldTotal := 0
	for _, d := range docs {
		boldTotal += len(d.bold)
	}
	if boldTotal > 0 {
		fmt.Printf("\n■ 짝이 안 되는 굵게 %d자리를 <strong>으로 바꿈\n", boldTotal)
	}
}

func store(sqlDB *sql.DB, docs []fetched, images map[string]repoImage) error {
	tx, err := sqlDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 그림을 먼저 넣는다. 본문이 이미 `/img/{sha256}`을 가리키고 있으므로
	// 같은 트랜잭션에서 함께 들어가야 중간에 깨진 상태가 남지 않는다.
	// **멱등 키는 sha256이라** 노션 이미지와 같은 행을 쓴다.
	imgNow := time.Now().UTC()
	for _, img := range images {
		if err := importer.UpsertImage(tx, importer.Image{
			SHA256: img.SHA256, Data: img.Data, MIME: img.MIME,
		}, imgNow); err != nil {
			return err
		}
	}

	// 분류는 여기서 만들지 않는다. 없으면 멈춘다 — 트리를 손대는 곳은
	// categorize·regroup 둘뿐이어야 한다.
	catID := map[string]int64{}
	for _, src := range curation.GitHubSources {
		if _, ok := catID[src.CategorySlug]; ok {
			continue
		}
		var id int64
		err := tx.QueryRow(`SELECT id FROM categories WHERE slug = ?`, src.CategorySlug).Scan(&id)
		if err == sql.ErrNoRows {
			return fmt.Errorf("분류 %q가 DB에 없다. categorize·regroup을 먼저 돌려라", src.CategorySlug)
		}
		if err != nil {
			return err
		}
		catID[src.CategorySlug] = id
	}

	now := time.Now().UTC()
	added, updated := 0, 0
	for _, d := range docs {
		ref := d.src.SourceRef(d.doc)
		slug := importer.Slugify(d.doc.Title)
		status := d.doc.Status
		if status == "" {
			status = "unlisted"
		}
		path := d.src.OriginalPath + " > " + d.doc.Title

		var id int64
		err := tx.QueryRow(`SELECT id FROM posts WHERE source_ref = ?`, ref).Scan(&id)
		switch {
		case err == sql.ErrNoRows:
			// **created_at은 처음 넣을 때만 정한다.** 노션 이관과 같은 규칙이다.
			if _, err := tx.Exec(`
				INSERT INTO posts (slug, title, body, status, source, source_ref,
				                   category_id, sort_order, sort_order_manual,
				                   original_path, original_created_at, created_at, updated_at)
				VALUES (?, ?, ?, ?, 'github', ?, ?, ?, 1, ?, ?, ?, ?)`,
				slug, d.doc.Title, d.body, status, ref,
				catID[d.src.CategorySlug], d.doc.SortOrder, path, d.createdAt, now, now); err != nil {
				return fmt.Errorf("%s 넣기: %w", ref, err)
			}
			added++
		case err != nil:
			return err
		default:
			// **slug는 안 덮는다.** 사람이 admin에서 바꿨을 수 있고, 바꾼 것을
			// 이관이 되돌리면 그 자리가 사람 것이 아니게 된다. 본문과 제목은
			// 저장소가 정본이라 덮는다.
			// **original_created_at도 갱신한다.** 저장소의 커밋 이력이 정본이라,
			// 파일을 지웠다 다시 만들었으면 그 날짜가 진짜 작성일이 된다.
			if _, err := tx.Exec(`
				UPDATE posts SET title = ?, body = ?, status = ?,
				                 category_id = ?, sort_order = ?, sort_order_manual = 1,
				                 original_path = ?, original_created_at = ?, updated_at = ?
				WHERE id = ?`,
				d.doc.Title, d.body, status,
				catID[d.src.CategorySlug], d.doc.SortOrder, path, d.createdAt, now, id); err != nil {
				return fmt.Errorf("%s 갱신: %w", ref, err)
			}
			updated++
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	fmt.Printf("\n새로 넣음 %d편 · 갱신 %d편\n", added, updated)
	return nil
}
