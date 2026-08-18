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
}

// resolveBody는 렌더링 직전에 본문을 손본다. 원본 문자열은 건드리지 않는다.
func (s *Server) resolveBody(body, originalPath string) (string, bodyFix, error) {
	var fix bodyFix

	targets := linkTargets(body)
	if len(targets) == 0 {
		return body, fix, nil
	}
	titles, err := s.store.PostTitlesBySlug(targets)
	if err != nil {
		return "", fix, err
	}

	// 죽은 링크가 있을 때만 인라인 데이터베이스를 찾아본다. 대부분의 글은
	// 여기 안 걸려서 조회가 한 번으로 끝난다.
	var groups map[string][]PostSummary
	hasDead := false
	for _, t := range targets {
		if _, ok := titles[t]; !ok {
			hasDead = true
			break
		}
	}
	if hasDead && originalPath != "" {
		if groups, err = s.store.InlineDBGroups(originalPath); err != nil {
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
			if _, live := titles[m[2]]; !live {
				linkText := m[1]
				rows, ok := groups[linkText]
				if !ok || used[linkText] {
					// 짝을 못 찾았다. 억지로 아무 목록이나 붙이면 남의 글이
					// 섞여 들어간다. 그대로 두는 편이 낫다.
					fix.Left++
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
				continue
			}
		}
		if n := strings.Count(line, placeholderLinkText); n > 0 {
			lines[i] = fillPlaceholders(line, titles, &fix)
		}
	}
	return strings.Join(lines, "\n"), fix, nil
}

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
func fillPlaceholders(line string, titles map[string]string, fix *bodyFix) string {
	return postLinkPattern.ReplaceAllStringFunc(line, func(match string) string {
		m := postLinkPattern.FindStringSubmatch(match)
		if m == nil || m[1] != placeholderLinkText {
			return match
		}
		title, ok := titles[m[2]]
		if !ok || title == "" {
			return match
		}
		fix.Titled++
		return "[" + escapeLinkText(title) + "](/p/" + m[2] + m[3] + ")"
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
			{{- with .Children}}{{template "items" .}}{{end -}}
			</li>
		{{- end -}}
	</ul>{{- end -}}
<div class="inline-db"><p class="inline-db-title">{{.Title}}</p>{{template "items" .Rows}}</div>`))

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

// countRows는 중첩까지 포함해 실제로 보이는 글 수를 센다.
func countRows(rows []PostSummary) int {
	n := 0
	for _, r := range rows {
		n += 1 + countRows(r.Children)
	}
	return n
}
