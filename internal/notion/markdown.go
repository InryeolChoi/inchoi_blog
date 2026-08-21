package notion

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// ImageURLPrefix는 결과 마크다운이 이미지를 가리킬 경로 앞부분이다.
// 실제 바이트는 images 테이블에 sha256으로 저장되고, 서버가 이 경로로 서빙한다.
const ImageURLPrefix = "/img/"

// PageURLPrefix는 아직 slug가 없는 상태에서 다른 노션 페이지를 가리킬 때 쓴다.
// 전체 이관이 끝나고 slug가 정해지면 이 링크들을 slug로 다시 쓴다.
const PageURLPrefix = "/p/"

// 노션에서 이미 비어 있던 블록 자리에 남기는 표시다. 렌더링에는 안 보이지만
// 원본과 대조할 때 "여기에 뭔가 있었다"는 사실이 남는다.
const (
	EmptyImageMarker    = "<!-- 원본에 비어있던 이미지 -->"
	EmptyEquationMarker = "<!-- 원본에 비어있던 수식 -->"
)

// chunk는 렌더링된 블록 하나다. list가 true면 앞 chunk와 빈 줄 없이 붙인다.
type chunk struct {
	text string
	list bool
}

type converter struct {
	report   Report
	captions []string // 원본에 있던 캡션 텍스트. 결과에 남았는지 나중에 대조한다.
}

// Convert는 덤프 하나를 마크다운으로 바꾸고 검증 리포트를 함께 돌려준다.
//
// 미지원 블록은 조용히 버리지 않는다. 렌더러가 없는 타입은 warn 이슈로 기록하고,
// 의도적으로 다르게 옮기는 타입(레이아웃 평탄화 등)은 note 이슈로 남긴다.
func Convert(d *Dump) (string, Report) {
	c := &converter{
		report: Report{
			PageID:     d.Page.ID,
			Title:      d.Page.Title(),
			BlockTypes: map[string]int{},
		},
	}

	c.surveySource(d.Blocks)

	chunks := c.renderBlocks(d.Blocks, "blocks")
	md := joinChunks(chunks)
	if md != "" {
		md += "\n"
	}

	// 결과 문자열을 직접 세어서 검증한다. 렌더링 중에 센 값을 쓰면 조립 단계에서
	// 내용이 사라져도 알아채지 못한다.
	c.report.OutputImages = countImageRefs(md)
	c.report.OutputCaptions = countPresent(md, c.captions)
	c.report.OutputTextLen = len([]rune(md))

	return md, c.report
}

// surveySource는 렌더링 전에 원본을 훑어서 기대값(이미지 수, 캡션, 텍스트 길이)을 센다.
func (c *converter) surveySource(blocks []Block) {
	for _, b := range blocks {
		c.report.BlockTypes[b.Type]++

		if b.Type == "image" {
			c.report.SourceImages++
		}

		var body struct {
			RichText   []RichText   `json:"rich_text"`
			Caption    []RichText   `json:"caption"`
			Cells      [][]RichText `json:"cells"`
			Title      string       `json:"title"`
			Expression string       `json:"expression"`
		}
		if err := b.decodeBody(&body); err == nil {
			c.report.SourceTextLen += len([]rune(PlainText(body.RichText)))
			c.report.SourceTextLen += len([]rune(PlainText(body.Caption)))
			c.report.SourceTextLen += len([]rune(body.Title))
			c.report.SourceTextLen += len([]rune(body.Expression))
			for _, cell := range body.Cells {
				c.report.SourceTextLen += len([]rune(PlainText(cell)))
			}
			if PlainText(body.Caption) != "" {
				c.report.SourceCaptions++
				// 결과에서 찾을 때는 렌더링된 형태로 찾아야 한다. 캡션 안에 인라인
				// 코드나 굵게 같은 서식이 있으면 결과에는 백틱이나 별표가 끼어들어서,
				// 서식 없는 원문으로 찾으면 멀쩡한 캡션도 유실로 잡힌다.
				c.captions = append(c.captions, renderRichText(body.Caption))
			}
		}

		c.surveySource(b.Children)
	}
}

func (c *converter) warn(b Block, path string, kind Kind, format string, args ...any) {
	c.report.Issues = append(c.report.Issues, Issue{
		Severity:  SevWarn,
		Kind:      kind,
		BlockType: b.Type,
		BlockID:   b.ID,
		Path:      path,
		Message:   fmt.Sprintf(format, args...),
	})
}

func (c *converter) note(b Block, path string, kind Kind, format string, args ...any) {
	c.report.Issues = append(c.report.Issues, Issue{
		Severity:  SevNote,
		Kind:      kind,
		BlockType: b.Type,
		BlockID:   b.ID,
		Path:      path,
		Message:   fmt.Sprintf(format, args...),
	})
}

// renderBlocks는 형제 블록들을 차례로 렌더링한다. path는 이슈 위치 표시용이다.
func (c *converter) renderBlocks(blocks []Block, path string) []chunk {
	var out []chunk
	lastNumber := 0      // 직전 numbered_list_item에 매긴 번호 (0이면 아직 없음)
	softRun := true      // 직전 번호 항목 이후 끼어든 블록이 전부 목록을 안 끊는 종류인지
	interrupted := false // 직전 번호 항목과 지금 사이에 다른 블록이 있었는지

	for i, b := range blocks {
		blockPath := fmt.Sprintf("%s[%d]:%s", path, i, b.Type)

		numbering := 0
		if b.Type == "numbered_list_item" {
			numbering = numberFor(lastNumber, softRun)
			if interrupted {
				if numbering > 1 {
					c.report.NumberingContinued++
					c.note(b, blockPath, KindNumberContinued,
						"중간에 다른 블록이 끼었지만 같은 목록으로 보고 번호를 %d로 이었다", numbering)
				} else {
					// 제목이나 도입 문단이 끼었다는 뜻이다. 새 목록으로 보는 게 맞아서
					// 이슈로는 남기지 않고 집계만 한다(덤프 전체에서 400건이 넘는다).
					c.report.NumberingRestarted++
				}
			}
			lastNumber = numbering
			softRun = true
			interrupted = false
		} else {
			if lastNumber > 0 {
				interrupted = true
			}
			if !isSoftInterrupt(b) {
				softRun = false
			}
		}

		// table_row는 table이 직접 처리한다. 여기서 만났다면 부모 없이 떠 있는 것이다.
		if b.Type == "table_row" {
			c.warn(b, blockPath, KindOrphanTableRow, "table 밖의 table_row다. 표 밖에서는 옮길 방법이 없어 건너뛴다")
			continue
		}

		text, isList := c.renderBlock(b, blockPath, numbering)
		// 노션 문단은 끝에 개행이나 공백이 붙어 있는 경우가 잦다. 그대로 두면
		// 블록 사이 구분자와 겹쳐 빈 줄이 여러 개 생긴다.
		text = strings.TrimRight(text, " \t\n")
		if text == "" {
			continue
		}
		out = append(out, chunk{text: text, list: isList})
	}
	return out
}

// numberFor는 번호 목록 항목에 매길 번호를 정한다.
//
// 노션은 목록 번호를 저장하지 않고 렌더링할 때 매긴다. 그래서 중간에 다른 블록이
// 끼면 목록이 끝난 건지 이어지는 건지 데이터만으로는 알 수 없고, 무엇이 끼었는지로
// 판단할 수밖에 없다.
func numberFor(lastNumber int, softRun bool) int {
	if lastNumber > 0 && softRun {
		return lastNumber + 1
	}
	return 1
}

// softInterruptTypes는 사이에 끼어도 번호 목록을 끊지 않는 블록들이다.
//
// 항목에 딸린 예시나 그림에 해당하는 것들만 넣었다. 이런 블록은 목록 항목을 보충할 뿐
// 새 목록을 시작하지 않는다.
//
// 반대로 제목과 (내용이 있는) 문단은 목록을 끊는다. 문단은 대개 "알고리즘은 다음과
// 같이 작동한다" 같은 새 목록의 도입문이고, 제목은 아예 새 절이기 때문이다.
var softInterruptTypes = map[string]bool{
	"code":         true,
	"image":        true,
	"equation":     true,
	"table":        true,
	"video":        true,
	"file":         true,
	"embed":        true,
	"bookmark":     true,
	"link_preview": true,
}

// isSoftInterrupt는 이 블록이 번호 목록을 끊지 않는지 본다.
// 내용이 빈 문단은 노션에서 여백으로 흔히 쓰이므로 끊지 않는 것으로 본다.
func isSoftInterrupt(b Block) bool {
	if softInterruptTypes[b.Type] {
		return true
	}
	if b.Type != "paragraph" {
		return false
	}
	var body struct {
		RichText []RichText `json:"rich_text"`
	}
	_ = b.decodeBody(&body)
	return strings.TrimSpace(PlainText(body.RichText)) == "" && len(b.Children) == 0
}

// renderBlock은 블록 하나를 렌더링한다. 두 번째 반환값은 리스트 항목인지 여부다.
func (c *converter) renderBlock(b Block, path string, number int) (string, bool) {
	switch b.Type {
	case "paragraph":
		// **자식을 들여쓰지 않는다.** 마크다운에는 "문단의 자식"이라는 구조가 없어서
		// 들여써도 아무 뜻이 없고, 대신 위험하다: CommonMark에서 4칸 들여쓰기는
		// 코드 블록이라, 문단 안에 문단이 겹치면 두 칸씩 쌓여 4칸을 넘고 그 아래
		// 목록이 통째로 회색 상자가 된다. 그 안의 `$수식$`은 코드 안이라 글자로 굳는다.
		// 실제로 `1. 확률`에서 그랬다. 제목(heading_*)도 같은 이유로 안 들여쓴다.
		return c.withChildren(b, path, c.richTextBody(b), "", false), false

	case "heading_1", "heading_2", "heading_3":
		level := map[string]string{"heading_1": "#", "heading_2": "##", "heading_3": "###"}[b.Type]
		head := strings.TrimSpace(level + " " + c.richTextBody(b))
		return c.withChildren(b, path, head, "", false), false

	case "bulleted_list_item":
		return c.listItem(b, path, "- "), true

	case "numbered_list_item":
		return c.listItem(b, path, fmt.Sprintf("%d. ", number)), true

	case "to_do":
		var body struct {
			Checked bool `json:"checked"`
		}
		_ = b.decodeBody(&body)
		marker := "- [ ] "
		if body.Checked {
			marker = "- [x] "
		}
		return c.listItem(b, path, marker), true

	case "quote":
		return c.blockquote(b, path, ""), false

	case "callout":
		var body struct {
			Icon *struct {
				Emoji string `json:"emoji"`
			} `json:"icon"`
		}
		_ = b.decodeBody(&body)
		prefix := ""
		if body.Icon != nil && body.Icon.Emoji != "" {
			prefix = body.Icon.Emoji + " "
		}
		return c.blockquote(b, path, prefix), false

	case "code":
		return c.codeBlock(b), false

	case "equation":
		var body struct {
			Expression string `json:"expression"`
		}
		_ = b.decodeBody(&body)
		if strings.TrimSpace(body.Expression) == "" {
			// 노션에 빈 수식 블록이 남아 있는 경우다. 아무것도 안 남기면 나중에
			// 원본을 볼 때 여기에 무언가 있었다는 사실조차 알 수 없다.
			c.note(b, path, KindEmptyBlock, "원본에 비어있던 수식 블록이다. 자리만 주석으로 남긴다")
			return EmptyEquationMarker, false
		}
		return "$$\n" + normalizeBlockMath(body.Expression) + "\n$$", false

	case "divider":
		return "---", false

	case "image":
		return c.image(b, path), false

	case "table":
		return c.table(b, path), false

	case "toggle":
		return c.toggle(b, path), false

	case "column_list":
		// 마크다운에는 단 개념이 없다. 단을 위에서 아래로 이어붙인다. 내용은 남는다.
		c.note(b, path, KindFlattenedColumns, "여러 단 레이아웃을 세로로 폈다 (내용은 유지)")
		return joinChunks(c.renderBlocks(b.Children, path)), false

	case "column":
		return joinChunks(c.renderBlocks(b.Children, path)), false

	case "synced_block":
		return c.syncedBlock(b, path), false

	case "table_of_contents":
		// 목차는 본문에서 빼고 렌더링할 때 제목으로 다시 만든다. 원본 내용이 아니다.
		c.note(b, path, KindDroppedTOC, "목차 블록은 옮기지 않는다 (렌더링 시 제목에서 다시 생성)")
		return "", false

	case "child_page":
		return c.childLink(b, path, "title")

	case "child_database":
		var body struct {
			Title string `json:"title"`
		}
		_ = b.decodeBody(&body)
		c.note(b, path, KindChildLink, "인라인 데이터베이스다. 하위 글 목록으로 다시 엮어야 한다")
		title := body.Title
		if title == "" {
			title = "(제목 없는 데이터베이스)"
		}
		return fmt.Sprintf("[%s](%s%s)", title, PageURLPrefix, b.ID), false

	case "link_to_page":
		var body struct {
			PageID     string `json:"page_id"`
			DatabaseID string `json:"database_id"`
		}
		_ = b.decodeBody(&body)
		target := body.PageID
		if target == "" {
			target = body.DatabaseID
		}
		if target == "" {
			c.warn(b, path, KindMissingURL, "가리키는 대상이 없는 link_to_page다")
			return "", false
		}
		return fmt.Sprintf("[페이지 링크](%s%s)", PageURLPrefix, target), false

	case "bookmark", "link_preview", "embed":
		return c.linkBlock(b, path), false

	case "video":
		return c.mediaLink(b, path, "영상")

	case "file":
		return c.mediaLink(b, path, "첨부파일")

	case "unsupported":
		var body struct {
			BlockType string `json:"block_type"`
		}
		_ = b.decodeBody(&body)
		c.warn(b, path, KindUnsupportedBlock, "노션이 API로 못 주는 블록이다 (원래 타입: %s). 원본을 직접 봐야 한다",
			orDefault(body.BlockType, "알 수 없음"))
		return "", false

	default:
		c.warn(b, path, KindUnknownBlock, "변환기가 모르는 블록 타입이다. 내용이 빠졌다")
		return "", false
	}
}

// richTextBody는 블록의 rich_text를 마크다운 인라인 서식으로 바꾼다.
func (c *converter) richTextBody(b Block) string {
	var body struct {
		RichText []RichText `json:"rich_text"`
	}
	_ = b.decodeBody(&body)
	return renderRichText(body.RichText)
}

// listItem은 리스트 항목을 만들고 자식을 marker 너비만큼 들여쓴다.
// 들여쓰기 폭이 marker 너비와 다르면 중첩 리스트가 코드 블록으로 잘못 해석될 수 있다.
func (c *converter) listItem(b Block, path, marker string) string {
	text := marker + c.richTextBody(b)
	return c.withChildren(b, path, text, strings.Repeat(" ", len([]rune(marker))), true)
}

// withChildren은 블록 본문 뒤에 자식들을 indent만큼 들여써서 붙인다.
//
// tightList가 true고 첫 자식이 리스트 항목이면 빈 줄 없이 붙인다. 리스트 항목과
// 그 하위 리스트 사이에 빈 줄이 있으면 마크다운이 리스트 전체를 느슨한 리스트로 보고
// 항목마다 문단을 감싸서, 노션에서 촘촘하던 목록이 띄엄띄엄해진다.
func (c *converter) withChildren(b Block, path, text, indent string, tightList bool) string {
	if len(b.Children) == 0 {
		return text
	}
	childChunks := c.renderBlocks(b.Children, path+">children")
	children := joinChunks(childChunks)
	if children == "" {
		return text
	}
	if text == "" {
		return indentLines(children, indent)
	}

	sep := "\n\n"
	if tightList && childChunks[0].list {
		sep = "\n"
	}
	return text + sep + indentLines(children, indent)
}

// blockquote는 본문과 자식을 통째로 인용문으로 만든다.
func (c *converter) blockquote(b Block, path, prefix string) string {
	text := prefix + c.richTextBody(b)
	if len(b.Children) > 0 {
		if children := joinChunks(c.renderBlocks(b.Children, path+">children")); children != "" {
			text += "\n\n" + children
		}
	}
	return indentLines(text, "> ")
}

// toggle은 마크다운에 대응이 없어서 <details>로 옮긴다. 접힘 상태가 유지된다.
//
// summary는 마크다운이 아니라 **HTML로** 낸다. <details>는 raw HTML 블록이고
// CommonMark는 HTML 블록 안을 인라인 파싱하지 않아서, `**목차**`를 그대로 넣으면
// 별표가 글자로 보인다. 덤프 전체에서 summary에 붙은 서식은 굵게 69개와 코드 6개다.
//
// 펼쳐도 아무것도 안 나오는 토글은 내보내지 않는다. 그런 껍데기가 35개인데
// 이유가 셋으로 갈린다: 목차만 들어 있어서 25개, 노션에서 원래부터 비어 있어서
// 8개, 자식은 있었지만 변환 결과가 비어서 2개. 화면에 나오는 모습은 셋 다
// 같으므로 다 뺀다. 다만 **왜** 비었는지는 리포트에 종류를 나눠 남긴다 —
// 마지막 2개는 변환기가 뭔가를 버렸다는 신호라 묻히면 안 된다.
//
// 뺄 때 summary에 적혀 있던 글자도 같이 사라진다. 대개 "p.123", "코드" 같은
// 라벨이지만 문장인 것이 2개 있다. 리포트에 그 글자를 같이 찍어두는 이유가
// 이것이다.
func (c *converter) toggle(b Block, path string) string {
	// 목차만 들어 있던 토글은 안을 렌더링해볼 것도 없이 뺀다. 렌더링하면 목차
	// 블록이 제 몫의 dropped-toc 기록까지 남겨서 같은 일이 두 번 세어진다.
	if len(b.Children) > 0 && onlyTOC(b.Children) {
		c.note(b, path, KindDroppedTOCToggle, "목차만 들어 있던 토글이라 토글째로 뺐다")
		return ""
	}

	children := joinChunks(c.renderBlocks(b.Children, path+">children"))
	if children == "" {
		// 자식이 아예 없던 토글과, 자식을 넣어뒀는데 그 내용이 빈 토글은 다르다.
		// 앞의 것은 라벨만 붙은 빈 껍데기라 통째로 뺀다. 뒤의 것은 사람이 거기에
		// 무언가를 쓰려고 자리를 잡아둔 것이고, 실제로 이런 토글의 summary는
		// 라벨이 아니라 문장이었다. 껍데기만 벗기고 글자는 문단으로 남긴다.
		if len(b.Children) == 0 {
			c.note(b, path, KindDroppedEmptyToggle,
				"노션에서도 비어 있던 토글이라 뺐다 (summary: %s)", plainSummary(b))
			return ""
		}
		// 문단이 되므로 HTML이 아니라 마크다운으로 낸다. <summary> 안과 달리
		// 여기는 goldmark가 인라인 파싱을 하는 자리다.
		text := c.richTextBody(b)
		if text == "" {
			c.note(b, path, KindDroppedEmptyToggle, "자식도 summary도 비어서 뺐다")
			return ""
		}
		c.note(b, path, KindToggleToParagraph,
			"자식이 비어 있어 토글 껍데기만 벗기고 summary를 문단으로 남겼다 (%s)", plainSummary(b))
		return text
	}

	summary := c.richTextBodyHTML(b)
	if summary == "" {
		summary = "펼치기"
	}

	var sb strings.Builder
	sb.WriteString("<details>\n<summary>")
	sb.WriteString(summary)
	sb.WriteString("</summary>\n\n")
	if children != "" {
		sb.WriteString(children)
		sb.WriteString("\n\n")
	}
	sb.WriteString("</details>")
	return sb.String()
}

// plainSummary는 리포트에 찍을 summary 글자다. 토글을 통째로 뺄 때 무엇이
// 같이 사라졌는지 남기려고 서식 없이 뽑는다.
func plainSummary(b Block) string {
	var body struct {
		RichText []RichText `json:"rich_text"`
	}
	_ = b.decodeBody(&body)
	var sb strings.Builder
	for _, rt := range body.RichText {
		sb.WriteString(rt.PlainText)
	}
	return strings.TrimSpace(sb.String())
}

// onlyTOC는 자식이 전부 목차 블록인지 본다. 빈 목록은 false다 — 자식이 없는
// 토글은 노션에서 원래 비어 있던 것이라 여기서 판단하지 않는다.
func onlyTOC(children []Block) bool {
	if len(children) == 0 {
		return false
	}
	for _, ch := range children {
		if ch.Type != "table_of_contents" {
			return false
		}
	}
	return true
}

// codeBlock은 코드 블록을 만든다. 내용에 백틱 울타리가 있으면 울타리를 더 길게 쓴다.
func (c *converter) codeBlock(b Block) string {
	var body struct {
		RichText []RichText `json:"rich_text"`
		Caption  []RichText `json:"caption"`
		Language string     `json:"language"`
	}
	_ = b.decodeBody(&body)

	code := PlainText(body.RichText) // 코드 안에서는 서식을 적용하지 않는다.
	fence := longestFence(code)

	var sb strings.Builder
	sb.WriteString(fence)
	sb.WriteString(markdownLanguage(body.Language))
	sb.WriteString("\n")
	sb.WriteString(code)
	if !strings.HasSuffix(code, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString(fence)

	if caption := renderRichText(body.Caption); caption != "" {
		sb.WriteString("\n\n")
		sb.WriteString(caption)
	}
	return sb.String()
}

// image는 로컬에 받아둔 sha256 경로를 가리킨다.
// 로컬 파일이 없으면(다운로드 실패했거나 외부 URL) 원본 URL로 두고 경고한다.
func (c *converter) image(b Block, path string) string {
	var body struct {
		Caption []RichText `json:"caption"`
		Type    string     `json:"type"`
		Local   *struct {
			SHA256 string `json:"sha256"`
		} `json:"local"`
		File     *struct{ URL string } `json:"file"`
		External *struct{ URL string } `json:"external"`
	}
	_ = b.decodeBody(&body)

	caption := renderRichText(body.Caption)

	if body.Local != nil && body.Local.SHA256 != "" {
		return fmt.Sprintf("![%s](%s%s)", caption, ImageURLPrefix, body.Local.SHA256)
	}

	url := ""
	if body.External != nil {
		url = body.External.URL
	} else if body.File != nil {
		url = body.File.URL
	}
	if url == "" {
		// 노션 쪽에서 이미 깨진 이미지다(external.url이 빈 문자열). 가리킬 곳이 없어서
		// 빈 참조를 남기면 렌더링만 깨진다. 자리만 주석으로 표시한다.
		c.warn(b, path, KindMissingImage,
			"원본에 URL이 비어 있는 이미지다. 주석만 남긴다 (노션 원본을 확인해야 한다)")
		return EmptyImageMarker
	}
	c.warn(b, path, KindExternalImage, "로컬로 받아둔 파일이 없어 외부 URL을 그대로 쓴다 (링크가 깨질 수 있다): %s",
		truncate(url, 80))
	return fmt.Sprintf("![%s](%s)", caption, url)
}

// table은 자식 table_row들을 마크다운 표로 만든다.
func (c *converter) table(b Block, path string) string {
	var body struct {
		TableWidth      int  `json:"table_width"`
		HasColumnHeader bool `json:"has_column_header"`
	}
	_ = b.decodeBody(&body)

	rows := make([][]string, 0, len(b.Children))
	for _, child := range b.Children {
		if child.Type != "table_row" {
			c.warn(child, path+">children", KindBadTable, "표 안에 table_row가 아닌 블록이 있다. 건너뛴다")
			continue
		}
		var rowBody struct {
			Cells [][]RichText `json:"cells"`
		}
		_ = child.decodeBody(&rowBody)

		cells := make([]string, 0, len(rowBody.Cells))
		for _, cell := range rowBody.Cells {
			// 셀 안의 개행은 표를 깨뜨리므로 <br>로 바꾼다.
			text := strings.ReplaceAll(renderRichText(cell), "\n", "<br>")
			cells = append(cells, strings.ReplaceAll(text, "|", "\\|"))
		}
		rows = append(rows, cells)
	}

	if len(rows) == 0 {
		c.warn(b, path, KindBadTable, "행이 없는 표다")
		return ""
	}

	width := body.TableWidth
	if width == 0 {
		width = len(rows[0])
	}

	var sb strings.Builder
	writeRow := func(cells []string) {
		sb.WriteString("|")
		for i := 0; i < width; i++ {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			sb.WriteString(" " + cell + " |")
		}
		sb.WriteString("\n")
	}

	body_ := rows
	if body.HasColumnHeader {
		writeRow(rows[0])
		body_ = rows[1:]
	} else {
		// 마크다운 표는 헤더 행이 필수다. 노션에 헤더가 없으면 빈 헤더를 넣는다.
		writeRow(make([]string, width))
		c.note(b, path, KindTableNoHeader, "노션 표에 헤더 행이 없어 빈 헤더를 넣었다")
	}

	sb.WriteString("|")
	for i := 0; i < width; i++ {
		sb.WriteString(" --- |")
	}
	sb.WriteString("\n")

	for _, row := range body_ {
		writeRow(row)
	}
	return strings.TrimRight(sb.String(), "\n")
}

// syncedBlock은 다른 곳의 내용을 비추는 블록이다.
func (c *converter) syncedBlock(b Block, path string) string {
	var body struct {
		SyncedFrom *struct {
			BlockID string `json:"block_id"`
		} `json:"synced_from"`
	}
	_ = b.decodeBody(&body)

	if body.SyncedFrom != nil && body.SyncedFrom.BlockID != "" {
		// 사본이다. 원본 블록이 다른 페이지에 있으므로 여기서 펼치면 내용이 중복된다.
		c.note(b, path, KindSyncedCopy, "다른 블록(%s)을 비추는 사본이다. 원본 쪽에서만 옮긴다",
			body.SyncedFrom.BlockID)
		return ""
	}
	return joinChunks(c.renderBlocks(b.Children, path+">children"))
}

// childLink는 하위 페이지 링크를 만든다. slug가 정해지기 전이라 page id로 걸어둔다.
func (c *converter) childLink(b Block, path, titleField string) (string, bool) {
	var body map[string]any
	_ = b.decodeBody(&body)
	title, _ := body[titleField].(string)
	if title == "" {
		title = "(제목 없음)"
	}
	c.note(b, path, KindChildLink, "하위 페이지 링크다. 이관 후 slug로 다시 써야 한다")
	return fmt.Sprintf("[%s](%s%s)", title, PageURLPrefix, b.ID), false
}

// linkBlock은 bookmark/link_preview/embed를 링크로 만든다.
func (c *converter) linkBlock(b Block, path string) string {
	var body struct {
		URL     string     `json:"url"`
		Caption []RichText `json:"caption"`
	}
	_ = b.decodeBody(&body)

	if body.URL == "" {
		c.warn(b, path, KindMissingURL, "URL이 없는 %s 블록이다", b.Type)
		return ""
	}
	label := renderRichText(body.Caption)
	if label == "" {
		label = body.URL
	}
	if b.Type == "embed" {
		c.note(b, path, KindEmbedAsLink, "임베드를 링크로 바꿨다 (내장 표시는 사라진다)")
	}
	return fmt.Sprintf("[%s](%s)", label, body.URL)
}

// mediaLink는 video/file처럼 노션이 호스팅하는 파일을 링크로 만든다.
func (c *converter) mediaLink(b Block, path, kind string) (string, bool) {
	var body struct {
		Type     string                `json:"type"`
		Caption  []RichText            `json:"caption"`
		File     *struct{ URL string } `json:"file"`
		External *struct{ URL string } `json:"external"`
	}
	_ = b.decodeBody(&body)

	url := ""
	if body.External != nil {
		url = body.External.URL
	} else if body.File != nil {
		url = body.File.URL
	}
	if url == "" {
		c.warn(b, path, KindMissingURL, "%s 블록에 URL이 없다", kind)
		return "", false
	}

	if body.Type == "file" {
		// 노션이 호스팅하는 파일의 URL은 서명이 붙은 임시 주소라 곧 만료된다.
		// 덤프 스크립트는 이미지만 받아뒀고 이 파일들은 받지 않았다.
		c.warn(b, path, KindExpiringURL, "%s이 노션 호스팅 파일이다. URL이 만료되므로 따로 받아와야 한다", kind)
	}

	label := renderRichText(body.Caption)
	if label == "" {
		label = kind
	}
	return fmt.Sprintf("[%s](%s)", label, url), false
}

// blankLines는 빈 줄(개행이 둘 이상 연달아 오는 것)을 찾는다.
var blankLines = regexp.MustCompile(`\n[ \t]*\n[\s]*`)

// normalizeInlineMath는 인라인 수식을 한 줄로 편다.
//
// LaTeX는 수식 모드에서 공백의 종류와 길이를 구분하지 않는다(줄바꿈도 공백일 뿐이고,
// 진짜 줄바꿈은 \\로 쓴다). 그래서 한 줄로 펴도 수식의 의미는 그대로다.
//
// 반면 마크다운에서는 빈 줄이 문단을 끊는다. 개행이 든 수식을 그대로 두면 여는 $와
// 닫는 $가 서로 다른 문단으로 갈라져서 수식이 렌더링되지 않고 $가 글자로 보인다.
// 리스트 안에서는 이어지는 줄에 들여쓰기까지 끼어들어 수식 문자열이 더 변형된다.
func normalizeInlineMath(expr string) string {
	return strings.Join(strings.Fields(expr), " ")
}

// normalizeBlockMath는 블록 수식에서 빈 줄만 없앤다.
//
// 여러 줄 배치는 align 환경 등에서 읽기 좋으므로 그대로 두되, 빈 줄은 지운다.
// $$ ... $$ 안의 빈 줄은 마크다운 파서가 문단 경계로 볼 수 있어서 수식 블록이
// 중간에 끊길 위험이 있다.
func normalizeBlockMath(expr string) string {
	return strings.TrimSpace(blankLines.ReplaceAllString(expr, "\n"))
}

// renderRichText는 rich_text 배열을 인라인 마크다운으로 바꾼다.
func renderRichText(rts []RichText) string {
	var sb strings.Builder
	for _, rt := range rts {
		sb.WriteString(renderRichTextOne(rt))
	}
	return sb.String()
}

func renderRichTextOne(rt RichText) string {
	if rt.Type == "equation" && rt.Equation != nil {
		return "$" + normalizeInlineMath(rt.Equation.Expression) + "$"
	}

	text := rt.PlainText
	if text == "" {
		return ""
	}

	// 코드가 먼저다. 코드 안에서는 나머지 서식 기호가 글자 그대로 보여야 한다.
	if rt.Annotations.Code {
		text = "`" + text + "`"
	} else {
		if rt.Annotations.Bold {
			text = "**" + text + "**"
		}
		if rt.Annotations.Italic {
			text = "*" + text + "*"
		}
		if rt.Annotations.Strikethrough {
			text = "~~" + text + "~~"
		}
		if rt.Annotations.Underline {
			// 마크다운에 밑줄이 없다. HTML로 남긴다.
			text = "<u>" + text + "</u>"
		}
	}

	if rt.Href != "" {
		text = "[" + text + "](" + rt.Href + ")"
	}
	return text
}

// richTextBodyHTML은 블록 본문을 인라인 **HTML**로 바꾼다. <summary>처럼
// 마크다운이 닿지 않는 자리에 쓴다.
func (c *converter) richTextBodyHTML(b Block) string {
	var body struct {
		RichText []RichText `json:"rich_text"`
	}
	_ = b.decodeBody(&body)
	return renderRichTextHTML(body.RichText)
}

// renderRichTextHTML은 rich_text 배열을 인라인 HTML로 바꾼다.
//
// renderRichText와 짝이다. 다른 점은 서식을 마크다운 기호가 아니라 태그로 내고,
// 글자를 HTML 이스케이프한다는 것 — 결과가 raw HTML 자리에 그대로 들어가므로
// 원문의 <, & 가 태그로 읽히면 안 된다.
func renderRichTextHTML(rts []RichText) string {
	var sb strings.Builder
	for _, rt := range rts {
		sb.WriteString(renderRichTextHTMLOne(rt))
	}
	return sb.String()
}

func renderRichTextHTMLOne(rt RichText) string {
	// 수식은 마크다운 쪽과 달리 그릴 방법이 없다. 여기(<summary>)는 우리 수식
	// 확장이 닿지 않는 raw HTML 자리라 $...$를 넣어봐야 글자로 보인다.
	// 덤프 전체에서 summary에 수식이 든 경우는 0건이라 원문만 남긴다.
	if rt.Type == "equation" && rt.Equation != nil {
		return html.EscapeString(normalizeInlineMath(rt.Equation.Expression))
	}

	text := rt.PlainText
	if text == "" {
		return ""
	}
	text = html.EscapeString(text)

	// 코드가 먼저다. 코드 안에서는 나머지 서식을 입히지 않는다.
	if rt.Annotations.Code {
		text = "<code>" + text + "</code>"
	} else {
		if rt.Annotations.Bold {
			text = "<strong>" + text + "</strong>"
		}
		if rt.Annotations.Italic {
			text = "<em>" + text + "</em>"
		}
		if rt.Annotations.Strikethrough {
			text = "<del>" + text + "</del>"
		}
		if rt.Annotations.Underline {
			text = "<u>" + text + "</u>"
		}
	}

	if rt.Href != "" {
		text = `<a href="` + html.EscapeString(rt.Href) + `">` + text + "</a>"
	}
	return text
}

// joinChunks는 렌더링된 블록들을 이어붙인다.
// 리스트 항목끼리는 빈 줄 없이 붙여야 하나의 리스트로 인식된다.
func joinChunks(chunks []chunk) string {
	var sb strings.Builder
	for i, ch := range chunks {
		if i > 0 {
			if ch.list && chunks[i-1].list {
				sb.WriteString("\n")
			} else {
				sb.WriteString("\n\n")
			}
		}
		sb.WriteString(ch.text)
	}
	return sb.String()
}

// indentLines는 빈 줄을 뺀 모든 줄 앞에 prefix를 붙인다.
// 빈 줄에 공백을 남기면 마크다운이 문단을 잇지 못한다.
func indentLines(text, prefix string) string {
	if prefix == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = strings.TrimRight(prefix, " ")
			continue
		}
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

// longestFence는 코드 내용과 겹치지 않는 백틱 울타리를 고른다.
func longestFence(code string) string {
	longest := 0
	for _, line := range strings.Split(code, "\n") {
		trimmed := strings.TrimSpace(line)
		count := 0
		for _, r := range trimmed {
			if r != '`' {
				break
			}
			count++
		}
		if count == len(trimmed) && count > longest {
			longest = count
		}
	}
	if longest < 3 {
		return "```"
	}
	return strings.Repeat("`", longest+1)
}

// markdownLanguage는 노션 언어 이름을 마크다운 하이라이터가 아는 이름으로 바꾼다.
func markdownLanguage(lang string) string {
	switch lang {
	case "plain text", "":
		return "text"
	case "c++":
		return "cpp"
	case "c#":
		return "csharp"
	case "docker":
		return "dockerfile"
	case "objective-c":
		return "objectivec"
	default:
		return strings.ReplaceAll(lang, " ", "-")
	}
}

// countImageRefs는 마크다운의 이미지 참조 수를 센다.
// 코드 블록 안의 "![...](...)"는 실제 이미지가 아니므로 건너뛴다.
func countImageRefs(md string) int {
	count := 0
	inFence := false
	var fence string

	for _, line := range strings.Split(md, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case !inFence && strings.HasPrefix(trimmed, "```"):
			inFence = true
			fence = trimmed[:countLeading(trimmed, '`')]
			continue
		case inFence && strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, "`") == "":
			inFence = false
			continue
		case inFence:
			continue
		}
		count += strings.Count(line, "![")
	}
	return count
}

func countLeading(s string, r rune) int {
	n := 0
	for _, c := range s {
		if c != r {
			break
		}
		n++
	}
	return n
}

// countPresent는 needles 중 몇 개가 md 안에 남아 있는지 센다.
func countPresent(md string, needles []string) int {
	n := 0
	for _, needle := range needles {
		if needle != "" && strings.Contains(md, needle) {
			n++
		}
	}
	return n
}

func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
