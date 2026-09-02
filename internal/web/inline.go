package web

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"
)

// 본문에는 눌러도 아무 데도 못 가는 링크가 두 종류 있다. 둘 다 변환기가 페이지를
// 하나씩 따로 처리하느라 그 시점에 알 수 없었던 것을 자리표시자로 남긴 결과다.
// 여기서는 DB의 body를 건드리지 않고 **렌더링 직전에만** 고쳐 넣는다.
//
//  1. [페이지 링크](/p/{slug})  — link_to_page 블록. 페이로드에 page_id만 있고
//     제목이 없어서 변환기가 자리표시자를 넣었다. 대상 글의 title로 바꾼다.
//  2. [제목](/p/{데이터베이스 id}) — child_database 블록. 노션 데이터베이스는
//     페이지가 아니라 posts에 절대 안 들어가므로 이 링크는 영원히 404다.
//     그 데이터베이스의 행이던 글들을 찾아 목록으로 펼친다.
//
// 본문을 고치는 게 아니라 렌더링 때만 고치는 이유: body는 정본이고, import를
// 다시 돌리면 어차피 덮어써진다. 여기서 처리하면 재이관과 무관하게 항상 맞는다.

// placeholderLinkText는 변환기가 link_to_page에 넣는 자리표시자다
// (internal/notion/markdown.go). 이 글자와 정확히 같을 때만 바꾼다.
const placeholderLinkText = "페이지 링크"

// postLinkPattern은 본문의 글 링크를 잡는다. relink가 만든 /p/ 형태다.
var postLinkPattern = regexp.MustCompile(`\[([^\[\]]*)\]\(/p/([^)\s#]+)(#[^)\s]*)?\)`)

// bodyFix는 한 편의 본문에서 무엇을 고쳤는지다. 테스트와 점검에서 본다.
type bodyFix struct {
	Titled   int // 자리표시자를 진짜 제목으로 바꾼 링크
	Expanded int // 목록으로 펼친 인라인 데이터베이스
	Rows     int // 그렇게 드러난 글
	Left     int // 짝을 못 찾아 그대로 둔 죽은 링크
	Grouped  int // 낱개 링크를 묶어 만든 목록 상자
	Unlinked int // 숨긴 글을 가리켜서 링크를 풀고 글자만 남긴 자리
	// Shown은 본문 링크와 펼친 목록에 나온 글의 slug다. 카테고리 페이지의
	// 목차와 아래 목록이 겹치는지 볼 때 쓴다.
	Shown map[string]bool
}

// resolveBody는 렌더링 직전에 본문을 손본다. 원본 문자열은 건드리지 않는다.
func (s *store) resolveBody(body, originalPath string) (string, bodyFix, error) {
	var fix bodyFix

	targets := linkTargets(body)
	if len(targets) == 0 {
		return body, fix, nil
	}
	metas, err := s.PostSummariesBySlug(targets)
	if err != nil {
		return "", fix, err
	}
	// **숨긴 글을 가리키는 링크를 먼저 푼다.** 이 뒤의 모든 판정이 "링크가 아직
	// 본문에 있는가"를 보기 때문에 순서가 중요하다 — 나중에 풀면 이미 Shown에
	// 들어갔거나 상자로 묶인 뒤다.
	body = s.unlinkHidden(body, metas, &fix)

	// 살아 있는 글 링크는 그 자체로 본문에 이미 보이는 길이다. 표지 글의 목차가
	// 이런 링크를 여러 개 직접 쌓아 만든 경우도 있어서, 인라인 DB로 펼친 행만
	// 세면 아래 "글" 목록과의 중복을 놓친다.
	for _, target := range targets {
		meta, live := metas[target]
		if !live || meta.Hidden {
			continue
		}
		if fix.Shown == nil {
			fix.Shown = map[string]bool{}
		}
		fix.Shown[target] = true
	}

	// 죽은 링크가 있을 때만 인라인 데이터베이스를 찾아본다. 대부분의 글은
	// 여기 안 걸려서 조회가 한 번으로 끝난다.
	var groups map[string][]PostSummary
	hasDead := false
	for _, t := range targets {
		// 숨긴 글은 이미 링크가 풀려서 본문에 남아 있지 않다. 인라인
		// 데이터베이스 후보로 세면 안 된다.
		if meta, ok := metas[t]; !ok || meta.Hidden {
			hasDead = true
			break
		}
	}
	if hasDead && originalPath != "" {
		if groups, err = s.InlineDBGroups(originalPath); err != nil {
			return "", fix, err
		}
	}

	// 같은 이름의 데이터베이스가 한 글에 두 번 걸려 있으면 첫 번째만 펼친다.
	// 두 번째까지 같은 목록을 펼치면 같은 글이 두 벌로 보인다.
	used := make(map[string]bool, len(groups))

	lines := strings.Split(body, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := postLinkPattern.FindStringSubmatch(trimmed); m != nil && m[0] == trimmed {
			if _, live := metas[m[2]]; !live {
				linkText := m[1]
				rows, ok := groups[linkText]
				if !ok || used[linkText] {
					// 짝을 못 찾았다. 억지로 아무 목록이나 붙이면 남의 글이
					// 섞여 들어간다. 그대로 두는 편이 낫다.
					fix.Left++
					continue
				}
				if len(rows) == 0 {
					// 이름은 맞는데 펼칠 행이 하나도 안 남았다 — 그 목록의
					// 글이 전부 숨긴 글이다. 링크로 두면 눌러서 404다.
					// 글자는 그 절의 제목이라 남긴다.
					lines[i] = unescapeLinkText(linkText)
					used[linkText] = true
					fix.Unlinked++
					continue
				}
				used[linkText] = true
				html, err := inlineDBHTML(linkText, rows)
				if err != nil {
					return "", fix, err
				}
				lines[i] = html
				fix.Expanded++
				fix.Rows += countRows(rows)
				if fix.Shown == nil {
					fix.Shown = map[string]bool{}
				}
				collectSlugs(rows, fix.Shown)
				continue
			}
		}
		if n := strings.Count(line, placeholderLinkText); n > 0 {
			lines[i] = fillPlaceholders(line, metas, &fix)
		}
	}
	// **묶기는 맨 마지막이다.** 자리표시자가 진짜 제목이 된 뒤라야 상자 안의
	// 글자가 "페이지 링크"가 아니라 글 제목이 된다.
	out, grouped := groupLinkRuns(strings.Join(lines, "\n"), metas)
	fix.Grouped = grouped
	return out, fix, nil
}

// unlinkHidden은 **숨긴 글을 가리키는 링크를 글자로 푼다.**
//
// draft는 `/p/{slug}`가 404다. 링크를 그대로 두면 눌러야 없는 줄 아는 길이 되고,
// 줄째 지우면 문장이 끊긴다. 그래서 링크만 벗기고 글자는 남긴다 — 죽은 링크를
// 렌더링 직전에만 손보고 DB의 body는 건드리지 않는다는 이 파일의 원칙 그대로다.
//
// **`[페이지 링크]` 자리표시자는 진짜 제목으로 바꿔서 남긴다.** 그냥 풀면
// 본문에 "페이지 링크"라는 말이 글자로 드러난다. 제목은 metas가 들고 있다.
func (s *store) unlinkHidden(body string, metas map[string]PostSummary, fix *bodyFix) string {
	return postLinkPattern.ReplaceAllStringFunc(body, func(match string) string {
		m := postLinkPattern.FindStringSubmatch(match)
		if m == nil {
			return match
		}
		meta, ok := metas[m[2]]
		if !ok || !meta.Hidden {
			return match
		}
		fix.Unlinked++
		if m[1] == placeholderLinkText && meta.Title != "" {
			return meta.Title
		}
		return unescapeLinkText(m[1])
	})
}

// unescapeLinkText는 링크 글자 자리의 이스케이프를 되돌린다. 링크를 풀어
// 맨 글자로 내보낼 때 `\[`가 그대로 보이지 않게 한다(escapeLinkText의 반대).
var linkTextUnescaper = strings.NewReplacer(`\[`, `[`, `\]`, `]`, `\\`, `\`)

func unescapeLinkText(s string) string { return linkTextUnescaper.Replace(s) }

// linkTargets는 본문에 나오는 /p/ 링크의 대상을 중복 없이 모은다.
func linkTargets(body string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range postLinkPattern.FindAllStringSubmatch(body, -1) {
		if !seen[m[2]] {
			seen[m[2]] = true
			out = append(out, m[2])
		}
	}
	return out
}

// fillPlaceholders는 자리표시자 링크의 글자를 대상 글의 제목으로 바꾼다.
// 대상이 posts에 없으면 손대지 않는다 — 바꿀 제목 자체가 없다.
func fillPlaceholders(line string, metas map[string]PostSummary, fix *bodyFix) string {
	return postLinkPattern.ReplaceAllStringFunc(line, func(match string) string {
		m := postLinkPattern.FindStringSubmatch(match)
		if m == nil || m[1] != placeholderLinkText {
			return match
		}
		meta, ok := metas[m[2]]
		if !ok || meta.Title == "" {
			return match
		}
		fix.Titled++
		return "[" + escapeLinkText(meta.Title) + "](/p/" + m[2] + m[3] + ")"
	})
}

// escapeLinkText는 제목을 마크다운 링크 글자 자리에 넣을 수 있게 만든다.
// 제목에 대괄호나 백슬래시가 든 글이 8건 있다. 그대로 넣으면 링크가 깨진다.
var linkTextEscaper = strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`)

func escapeLinkText(s string) string { return linkTextEscaper.Replace(s) }

// inlineDBTmpl은 펼친 목록의 HTML이다.
//
// 결과를 마크다운 본문 안에 그대로 끼워 넣기 때문에 **줄바꿈이 없어야 한다.**
// CommonMark의 HTML 블록은 빈 줄에서 끝나므로, 중간에 빈 줄이 생기면 뒷부분이
// 마크다운으로 다시 해석돼 태그가 글자로 보인다. 그래서 한 줄로 뽑는다.
//
// 글자는 html/template가 이스케이프한다. 손으로 문자열을 이어 붙이지 않는 이유다.
var inlineDBTmpl = template.Must(template.New("inlinedb").Parse(
	`{{- define "items" -}}<ul class="list">
		{{- range . -}}
			<li><a href="/p/{{.Slug}}">{{.Title}}</a>
			{{- if eq .Status "draft"}} <span class="status">{{.Status}}</span>{{end -}}
			{{- with .Date}}<span class="date">{{.}}</span>{{end -}}
			{{- with .Children}}{{template "items" .}}{{end -}}
			</li>
		{{- end -}}
	</ul>{{- end -}}
<div class="inline-db">{{with .Title}}<p class="inline-db-title">{{.}}</p>{{end}}{{template "items" .Rows}}</div>`))

// inlineDBHTML은 데이터베이스 하나를 목록 HTML로 만든다.
func inlineDBHTML(title string, rows []PostSummary) (string, error) {
	var b strings.Builder
	data := struct {
		Title string
		Rows  []PostSummary
	}{title, rows}
	if err := inlineDBTmpl.Execute(&b, data); err != nil {
		return "", fmt.Errorf("인라인 데이터베이스 렌더링(%s): %w", title, err)
	}
	out := b.String()
	if strings.ContainsAny(out, "\n\r") {
		// 여기 걸리면 위 템플릿의 공백 제어가 깨진 것이다. 그대로 내보내면
		// 본문 중간에 태그가 글자로 드러난다.
		return "", fmt.Errorf("인라인 데이터베이스 HTML에 줄바꿈이 있다(%s)", title)
	}
	return out, nil
}

// collectSlugs는 중첩까지 훑어 slug를 모은다.
func collectSlugs(rows []PostSummary, into map[string]bool) {
	for _, r := range rows {
		into[r.Slug] = true
		collectSlugs(r.Children, into)
	}
}

// countRows는 중첩까지 포함해 실제로 보이는 글 수를 센다.
func countRows(rows []PostSummary) int {
	n := 0
	for _, r := range rows {
		n += 1 + countRows(r.Children)
	}
	return n
}

// linkRow는 본문 링크 한 줄을 목록 한 줄로 만든다.
//
// **글자는 본문 것을 쓰고 나머지만 DB에서 가져온다.** 링크 글자는 사람이 그
// 자리에 쓴 것이라 제목과 다를 수 있고(자리표시자는 이미 진짜 제목으로 바뀐
// 뒤다), 작성일과 status는 본문에 아예 없다. 짝이 없으면 — 죽은 링크라 —
// 날짜와 뱃지 없이 글자만 남는다. 상자를 못 만드는 것보다 낫다.
func linkRow(slug, text string, metas map[string]PostSummary) PostSummary {
	row := PostSummary{Slug: slug, Title: text}
	if meta, ok := metas[slug]; ok {
		row.Status = meta.Status
		row.CreatedAt = meta.CreatedAt
	}
	return row
}

// standaloneLink는 제 문단을 통째로 차지한 글 링크다.
var standaloneLink = regexp.MustCompile(`^\[([^\[\]]+)\]\(/p/([^)\s#]+)\)$`)

// groupLinkRuns는 **줄줄이 이어진 글 링크를 목록 상자로 묶는다.**
//
// 노션에서 한 절의 머리에 두던 목록이 두 모양으로 들어온다. 인라인 데이터베이스는
// 이미 상자로 펼치는데(expandInlineDB), 사람이 link_to_page 블록을 여러 개 쌓아
// 만든 목록은 링크가 낱개로 흩어져 나온다. 같은 것을 두 모양으로 보여줄 이유가 없다.
//
// **두 개 이상 이어질 때만 묶는다.** 하나짜리는 목록이 아니라 문장 사이의 링크다.
// 문단 하나를 통째로 차지한 링크만 본다 — 목록 항목 안이나 문장 속 링크는 글자
// 그대로 둔다(바깥 링크를 카드로 만드는 규칙과 같은 결이다).
func groupLinkRuns(body string, metas map[string]PostSummary) (string, int) {
	lines := strings.Split(body, "\n")
	var out []string
	grouped := 0

	for i := 0; i < len(lines); {
		if standaloneLink.FindStringSubmatch(strings.TrimSpace(lines[i])) == nil {
			out = append(out, lines[i])
			i++
			continue
		}
		// 링크와 빈 줄이 이어지는 동안 모은다.
		var rows []PostSummary
		j := i
		for j < len(lines) {
			t := strings.TrimSpace(lines[j])
			if m := standaloneLink.FindStringSubmatch(t); m != nil {
				rows = append(rows, linkRow(m[2], m[1], metas))
				j++
				continue
			}
			if t == "" {
				// 다음에 링크가 더 있을 때만 빈 줄을 건너뛴다. 아니면 여기서 끝이다.
				k := j
				for k < len(lines) && strings.TrimSpace(lines[k]) == "" {
					k++
				}
				if k < len(lines) && standaloneLink.MatchString(strings.TrimSpace(lines[k])) {
					j = k
					continue
				}
			}
			break
		}
		// **하나짜리는 그 절의 내용이 그것뿐일 때만 묶는다** (2026-09-02).
		//
		// 원래는 둘 이상만 묶었다. 근거는 "하나짜리는 목록이 아니라 문장
		// 사이의 링크다"였고 대체로 옳다 — 63개 중 36개가 실제로 그렇다.
		// 그런데 나머지 27개는 제목 바로 아래에 링크 하나만 있고 다음
		// 제목까지 아무것도 없는 자리다. 그건 **항목이 하나뿐인 목록**이지
		// 문장 사이의 링크가 아니다.
		//
		// 실제로 `확률분포 정리` 표지에서 `다변량, 연속형 분포`는 상자인데
		// 바로 아래 `다변량, 이산형 분포`는 맨 링크 하나로 나왔다. 같은
		// 구조가 같아 보이지 않으면 읽는 사람이 그 차이를 뜻으로 읽는다.
		if len(rows) < 2 && !isWholeSection(lines, i, j) {
			out = append(out, lines[i])
			i++
			continue
		}
		html, err := inlineDBHTML("", rows)
		if err != nil {
			// 상자를 못 만들면 원문 그대로 둔다. 링크는 여전히 눌린다.
			out = append(out, lines[i:j]...)
			i = j
			continue
		}
		out = append(out, html)
		grouped++
		i = j
	}
	return strings.Join(out, "\n"), grouped
}

// mdHeading은 마크다운 제목 줄이다.
var mdHeading = regexp.MustCompile(`^#{1,6}\s`)

// isWholeSection은 lines[i:j]의 링크 묶음이 **한 절의 내용 전부**인지 본다.
// 바로 앞의 빈 줄 아닌 줄이 제목이고, 바로 뒤가 제목이거나 글의 끝이면 그렇다.
//
// 이 검사가 가르는 것은 "항목 하나짜리 목록"과 "문장 사이의 링크"다. 앞뒤에
// 산문이 있으면 그건 문장의 일부라 상자로 감싸면 글이 끊긴다.
func isWholeSection(lines []string, i, j int) bool {
	a := i - 1
	for a >= 0 && strings.TrimSpace(lines[a]) == "" {
		a--
	}
	if a < 0 || !mdHeading.MatchString(strings.TrimSpace(lines[a])) {
		return false
	}
	b := j
	for b < len(lines) && strings.TrimSpace(lines[b]) == "" {
		b++
	}
	return b >= len(lines) || mdHeading.MatchString(strings.TrimSpace(lines[b]))
}
