package notion

import (
	"encoding/json"
	"strings"
	"testing"
)

// convertJSON은 덤프 JSON 문자열을 그대로 파싱해서 변환한다.
// 구조체를 손으로 짜는 대신 실제 언마샬 경로를 그대로 태운다.
func convertJSON(t *testing.T, dumpJSON string) (string, Report) {
	t.Helper()
	var d Dump
	if err := json.Unmarshal([]byte(dumpJSON), &d); err != nil {
		t.Fatalf("덤프 파싱: %v", err)
	}
	return Convert(&d)
}

// blocksJSON은 블록 배열만 감싸서 덤프 모양으로 만든다.
func blocksJSON(blocks string) string {
	return `{"page":{"id":"p1","properties":{}},"blocks":` + blocks + `}`
}

func TestRichTextAnnotations(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"paragraph","paragraph":{"rich_text":[
	    {"type":"text","plain_text":"보통 ","annotations":{}},
	    {"type":"text","plain_text":"굵게","annotations":{"bold":true}},
	    {"type":"text","plain_text":" ","annotations":{}},
	    {"type":"text","plain_text":"기울임","annotations":{"italic":true}},
	    {"type":"text","plain_text":" ","annotations":{}},
	    {"type":"text","plain_text":"코드","annotations":{"code":true}},
	    {"type":"text","plain_text":" ","annotations":{}},
	    {"type":"text","plain_text":"취소","annotations":{"strikethrough":true}}
	  ]}}
	]`))

	want := "보통 **굵게** *기울임* `코드` ~~취소~~"
	if strings.TrimSpace(md) != want {
		t.Errorf("got  %q\nwant %q", strings.TrimSpace(md), want)
	}
}

// TestCodeAnnotationSuppressesOtherFormatting은 코드 서식 안에서 마크다운 기호가
// 글자 그대로 남는지 본다. `**foo**`처럼 쓰면 코드 안에 별표가 보여야 한다.
func TestCodeAnnotationSuppressesOtherFormatting(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"paragraph","paragraph":{"rich_text":[
	    {"type":"text","plain_text":"malloc()","annotations":{"code":true,"bold":true}}
	  ]}}
	]`))

	if strings.TrimSpace(md) != "`malloc()`" {
		t.Errorf("코드 서식 안에 다른 서식이 섞였다: %q", strings.TrimSpace(md))
	}
}

func TestLinkRendering(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"paragraph","paragraph":{"rich_text":[
	    {"type":"text","plain_text":"문서","href":"https://example.com","annotations":{"bold":true}}
	  ]}}
	]`))

	if strings.TrimSpace(md) != "[**문서**](https://example.com)" {
		t.Errorf("링크 렌더링이 다르다: %q", strings.TrimSpace(md))
	}
}

func TestHeadings(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"heading_1","heading_1":{"rich_text":[{"type":"text","plain_text":"일","annotations":{}}]}},
	  {"id":"b2","type":"heading_2","heading_2":{"rich_text":[{"type":"text","plain_text":"이","annotations":{}}]}},
	  {"id":"b3","type":"heading_3","heading_3":{"rich_text":[{"type":"text","plain_text":"삼","annotations":{}}]}}
	]`))

	want := "# 일\n\n## 이\n\n### 삼"
	if strings.TrimSpace(md) != want {
		t.Errorf("got\n%s\nwant\n%s", md, want)
	}
}

// TestNestedBulletedList는 리스트 안의 리스트가 부모 항목에 속하도록 들여쓰이는지 본다.
// 들여쓰기 폭이 틀리면 중첩이 풀리거나 코드 블록으로 해석된다.
func TestNestedBulletedList(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"bulleted_list_item","has_children":true,
	   "bulleted_list_item":{"rich_text":[{"type":"text","plain_text":"바깥","annotations":{}}]},
	   "children":[
	     {"id":"b2","type":"bulleted_list_item","has_children":true,
	      "bulleted_list_item":{"rich_text":[{"type":"text","plain_text":"안쪽","annotations":{}}]},
	      "children":[
	        {"id":"b3","type":"bulleted_list_item",
	         "bulleted_list_item":{"rich_text":[{"type":"text","plain_text":"더 안쪽","annotations":{}}]}}
	      ]}
	   ]}
	]`))

	want := "- 바깥\n  - 안쪽\n    - 더 안쪽"
	if strings.TrimSpace(md) != want {
		t.Errorf("중첩 들여쓰기가 다르다:\ngot\n%s\nwant\n%s", md, want)
	}
}

// TestNumberedListNumbering은 번호가 형제 안에서 1부터 매겨지고,
// 다른 블록이 끼면 다시 1로 돌아가는지 본다.
func TestNumberedListNumbering(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"numbered_list_item","numbered_list_item":{"rich_text":[{"type":"text","plain_text":"하나","annotations":{}}]}},
	  {"id":"b2","type":"numbered_list_item","numbered_list_item":{"rich_text":[{"type":"text","plain_text":"둘","annotations":{}}]}},
	  {"id":"b3","type":"numbered_list_item","numbered_list_item":{"rich_text":[{"type":"text","plain_text":"셋","annotations":{}}]}},
	  {"id":"b4","type":"divider","divider":{}},
	  {"id":"b5","type":"numbered_list_item","numbered_list_item":{"rich_text":[{"type":"text","plain_text":"다시 하나","annotations":{}}]}}
	]`))

	for _, want := range []string{"1. 하나", "2. 둘", "3. 셋", "1. 다시 하나"} {
		if !strings.Contains(md, want) {
			t.Errorf("%q가 없다:\n%s", want, md)
		}
	}
}

// TestListItemsStayTight는 리스트 항목 사이에 빈 줄이 끼지 않는지 본다.
// 빈 줄이 들어가면 마크다운이 항목마다 별개 리스트로 볼 수 있다.
func TestListItemsStayTight(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"bulleted_list_item","bulleted_list_item":{"rich_text":[{"type":"text","plain_text":"하나","annotations":{}}]}},
	  {"id":"b2","type":"bulleted_list_item","bulleted_list_item":{"rich_text":[{"type":"text","plain_text":"둘","annotations":{}}]}}
	]`))

	if strings.TrimSpace(md) != "- 하나\n- 둘" {
		t.Errorf("리스트 항목 사이가 떨어졌다: %q", md)
	}
}

func TestToDoCheckbox(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"to_do","to_do":{"checked":true,"rich_text":[{"type":"text","plain_text":"완료","annotations":{}}]}},
	  {"id":"b2","type":"to_do","to_do":{"checked":false,"rich_text":[{"type":"text","plain_text":"미완","annotations":{}}]}}
	]`))

	if !strings.Contains(md, "- [x] 완료") || !strings.Contains(md, "- [ ] 미완") {
		t.Errorf("체크박스가 다르다:\n%s", md)
	}
}

func TestCodeBlockKeepsLanguage(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"code","code":{"language":"c++","rich_text":[{"type":"text","plain_text":"int main() {}","annotations":{}}]}}
	]`))

	if !strings.HasPrefix(strings.TrimSpace(md), "```cpp\n") {
		t.Errorf("언어 정보가 유실됐다:\n%s", md)
	}
	if !strings.Contains(md, "int main() {}") {
		t.Errorf("코드 내용이 유실됐다:\n%s", md)
	}
}

// TestCodeBlockContainingFence는 코드 안에 백틱 울타리가 있을 때
// 울타리를 더 길게 써서 코드 블록이 중간에 끊기지 않는지 본다.
func TestCodeBlockContainingFence(t *testing.T) {
	// 백틱은 raw string을 끝내버리므로 자리표시자를 넣고 나중에 바꾼다.
	tmpl := blocksJSON(`[
	  {"id":"b1","type":"code","code":{"language":"markdown","rich_text":[
	    {"type":"text","plain_text":"FENCE\ncode\nFENCE","annotations":{}}]}}
	]`)
	md, _ := convertJSON(t, strings.ReplaceAll(tmpl, "FENCE", "\x60\x60\x60"))

	if !strings.HasPrefix(strings.TrimSpace(md), "\x60\x60\x60\x60markdown") {
		t.Errorf("울타리가 길어지지 않아 코드가 중간에 끊긴다:\n%s", md)
	}
}

// TestCodeBlockDoesNotApplyFormatting은 코드 안의 별표가 굵게 변하지 않는지 본다.
func TestCodeBlockDoesNotApplyFormatting(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"code","code":{"language":"c","rich_text":[
	    {"type":"text","plain_text":"int *p = &x;","annotations":{"bold":true}}]}}
	]`))

	if !strings.Contains(md, "int *p = &x;") {
		t.Errorf("코드 내용이 변형됐다:\n%s", md)
	}
	if strings.Contains(md, "**int") {
		t.Errorf("코드 블록에 서식이 적용됐다:\n%s", md)
	}
}

func TestEquationBlock(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"equation","equation":{"expression":"E = mc^2"}}
	]`))

	if strings.TrimSpace(md) != "$$\nE = mc^2\n$$" {
		t.Errorf("수식 블록이 다르다: %q", md)
	}
}

func TestInlineEquation(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"paragraph","paragraph":{"rich_text":[
	    {"type":"text","plain_text":"식은 ","annotations":{}},
	    {"type":"equation","equation":{"expression":"x^2"},"plain_text":"x^2","annotations":{}},
	    {"type":"text","plain_text":" 이다","annotations":{}}
	  ]}}
	]`))

	if strings.TrimSpace(md) != "식은 $x^2$ 이다" {
		t.Errorf("인라인 수식이 다르다: %q", strings.TrimSpace(md))
	}
}

func TestImageUsesLocalSHA256(t *testing.T) {
	md, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"image","image":{"type":"file","caption":[{"type":"text","plain_text":"그림 1","annotations":{}}],
	   "local":{"sha256":"abc123","file":"images/abc123.png","contentType":"image/png"}}}
	]`))

	want := "![그림 1](/img/abc123)"
	if strings.TrimSpace(md) != want {
		t.Errorf("got %q, want %q", strings.TrimSpace(md), want)
	}
	if rep.SourceImages != 1 || rep.OutputImages != 1 {
		t.Errorf("이미지 집계가 다르다: 원본 %d, 결과 %d", rep.SourceImages, rep.OutputImages)
	}
	if !rep.CaptionsMatch() {
		t.Errorf("캡션 집계가 다르다: 원본 %d, 결과 %d", rep.SourceCaptions, rep.OutputCaptions)
	}
}

// TestImageWithoutLocalWarns는 로컬 파일이 없는 이미지를 조용히 버리지 않는지 본다.
// 실제 덤프에 이런 이미지가 23개 있다(전부 외부 URL).
func TestImageWithoutLocalWarns(t *testing.T) {
	md, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"image","image":{"type":"external","caption":[],
	   "external":{"url":"https://example.com/x.png"}}}
	]`))

	if !strings.Contains(md, "https://example.com/x.png") {
		t.Errorf("외부 URL이 유실됐다: %q", md)
	}
	warnings := rep.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("경고가 1건이어야 하는데 %d건: %+v", len(warnings), warnings)
	}
	if rep.OutputImages != 1 {
		t.Errorf("외부 이미지도 참조로 세야 한다: %d", rep.OutputImages)
	}
}

func TestToggleBecomesDetails(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"toggle","has_children":true,
	   "toggle":{"rich_text":[{"type":"text","plain_text":"개념","annotations":{}}]},
	   "children":[
	     {"id":"b2","type":"paragraph","paragraph":{"rich_text":[{"type":"text","plain_text":"안쪽 내용","annotations":{}}]}}
	   ]}
	]`))

	for _, want := range []string{"<details>", "<summary>개념</summary>", "안쪽 내용", "</details>"} {
		if !strings.Contains(md, want) {
			t.Errorf("%q가 없다:\n%s", want, md)
		}
	}
}

func TestToggleSummaryIsHTMLNotMarkdown(t *testing.T) {
	// <details>는 raw HTML 블록이라 CommonMark가 안을 인라인 파싱하지 않는다.
	// summary에 마크다운 기호를 넣으면 별표·백틱이 글자로 보인다.
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"toggle","has_children":true,
	   "toggle":{"rich_text":[
	     {"type":"text","plain_text":"목차","annotations":{"bold":true}},
	     {"type":"text","plain_text":" 와 ","annotations":{}},
	     {"type":"text","plain_text":"code()","annotations":{"code":true}}
	   ]},
	   "children":[
	     {"id":"b2","type":"paragraph","paragraph":{"rich_text":[{"type":"text","plain_text":"안쪽","annotations":{}}]}}
	   ]}
	]`))

	want := "<summary><strong>목차</strong> 와 <code>code()</code></summary>"
	if !strings.Contains(md, want) {
		t.Errorf("summary가 HTML이 아니다:\n%s", md)
	}
	for _, bad := range []string{"**목차**", "`code()`"} {
		if strings.Contains(md, bad) {
			t.Errorf("마크다운 기호 %q가 그대로 남았다:\n%s", bad, md)
		}
	}
}

func TestToggleSummaryEscapesHTML(t *testing.T) {
	// 결과가 raw HTML 자리에 그대로 들어가므로 원문의 <, & 가 태그로 읽히면 안 된다.
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"toggle","has_children":true,
	   "toggle":{"rich_text":[{"type":"text","plain_text":"a < b && c","annotations":{}}]},
	   "children":[
	     {"id":"b2","type":"paragraph","paragraph":{"rich_text":[{"type":"text","plain_text":"안쪽","annotations":{}}]}}
	   ]}
	]`))

	if !strings.Contains(md, "<summary>a &lt; b &amp;&amp; c</summary>") {
		t.Errorf("summary를 이스케이프하지 않았다:\n%s", md)
	}
}

func TestTOCOnlyToggleIsDropped(t *testing.T) {
	// 목차 블록을 버리고 나면 눌러도 아무것도 안 나오는 빈 껍데기만 남는다.
	md, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"toggle","has_children":true,
	   "toggle":{"rich_text":[{"type":"text","plain_text":"목차","annotations":{}}]},
	   "children":[{"id":"b2","type":"table_of_contents","table_of_contents":{}}]},
	  {"id":"b3","type":"paragraph","paragraph":{"rich_text":[{"type":"text","plain_text":"본문","annotations":{}}]}}
	]`))

	if strings.Contains(md, "<details>") {
		t.Errorf("목차만 든 토글이 남았다:\n%s", md)
	}
	if !strings.Contains(md, "본문") {
		t.Errorf("뒤따르는 본문이 사라졌다:\n%s", md)
	}
	if rep.CountKind(KindDroppedTOCToggle) != 1 {
		t.Errorf("dropped-toc-toggle 기록이 없다: %+v", rep)
	}
}

func TestToggleWithTOCAndContentStays(t *testing.T) {
	// 목차 말고 다른 것도 들어 있으면 토글은 남는다. 목차만 빠진다.
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"toggle","has_children":true,
	   "toggle":{"rich_text":[{"type":"text","plain_text":"목차","annotations":{}}]},
	   "children":[
	     {"id":"b2","type":"table_of_contents","table_of_contents":{}},
	     {"id":"b3","type":"paragraph","paragraph":{"rich_text":[{"type":"text","plain_text":"안쪽","annotations":{}}]}}
	   ]}
	]`))

	if !strings.Contains(md, "<details>") || !strings.Contains(md, "안쪽") {
		t.Errorf("내용이 있는 토글까지 지웠다:\n%s", md)
	}
}

func TestEmptyToggleIsDropped(t *testing.T) {
	// 펼쳐도 아무것도 안 나오는 껍데기는 이유를 가리지 않고 뺀다.
	md, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"toggle","has_children":false,
	   "toggle":{"rich_text":[{"type":"text","plain_text":"p.123","annotations":{}}]}},
	  {"id":"b2","type":"paragraph","paragraph":{"rich_text":[{"type":"text","plain_text":"본문","annotations":{}}]}}
	]`))

	if strings.Contains(md, "<details>") {
		t.Errorf("빈 토글이 남았다:\n%s", md)
	}
	if !strings.Contains(md, "본문") {
		t.Errorf("뒤따르는 본문이 사라졌다:\n%s", md)
	}
	if rep.CountKind(KindDroppedEmptyToggle) != 1 {
		t.Errorf("dropped-empty-toggle 기록이 없다: %+v", rep)
	}
	// 무엇이 같이 사라졌는지 리포트에 남아야 한다.
	if iss := rep.IssuesOfKind(KindDroppedEmptyToggle); len(iss) != 1 || !strings.Contains(iss[0].Message, "p.123") {
		t.Errorf("사라진 summary 글자가 기록에 없다: %+v", iss)
	}
}

func TestToggleWithLostChildrenIsDroppedAndReported(t *testing.T) {
	// 자식이 있었는데 변환 결과가 비었다면 변환기가 뭔가를 버렸다는 신호다.
	// 토글은 빼되 기록은 남긴다.
	md, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"toggle","has_children":true,
	   "toggle":{"rich_text":[{"type":"text","plain_text":"개념","annotations":{}}]},
	   "children":[{"id":"b2","type":"table_of_contents","table_of_contents":{}},
	               {"id":"b3","type":"table_of_contents","table_of_contents":{}}]}
	]`))

	if strings.Contains(md, "<details>") {
		t.Errorf("빈 토글이 남았다:\n%s", md)
	}
	if rep.CountKind(KindDroppedTOCToggle) != 1 {
		t.Errorf("목차만 든 토글로 기록되지 않았다: %+v", rep)
	}
}

func TestQuoteAndCallout(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"quote","quote":{"rich_text":[{"type":"text","plain_text":"인용문","annotations":{}}]}},
	  {"id":"b2","type":"callout","callout":{"icon":{"type":"emoji","emoji":"💡"},
	   "rich_text":[{"type":"text","plain_text":"강조","annotations":{}}]}}
	]`))

	if !strings.Contains(md, "> 인용문") {
		t.Errorf("인용문이 다르다:\n%s", md)
	}
	if !strings.Contains(md, "> 💡 강조") {
		t.Errorf("콜아웃이 다르다:\n%s", md)
	}
}

func TestTableWithHeader(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"table","has_children":true,
	   "table":{"table_width":2,"has_column_header":true},
	   "children":[
	     {"id":"r1","type":"table_row","table_row":{"cells":[
	       [{"type":"text","plain_text":"이름","annotations":{}}],
	       [{"type":"text","plain_text":"설명","annotations":{}}]]}},
	     {"id":"r2","type":"table_row","table_row":{"cells":[
	       [{"type":"text","plain_text":"fork","annotations":{}}],
	       [{"type":"text","plain_text":"프로세스 복제","annotations":{}}]]}}
	   ]}
	]`))

	for _, want := range []string{"| 이름 | 설명 |", "| --- | --- |", "| fork | 프로세스 복제 |"} {
		if !strings.Contains(md, want) {
			t.Errorf("%q가 없다:\n%s", want, md)
		}
	}
}

// TestTableWithoutHeaderStillValid는 노션 표에 헤더가 없어도
// 마크다운 표로 성립하는지 본다(마크다운은 헤더 행이 필수다).
func TestTableWithoutHeaderStillValid(t *testing.T) {
	md, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"table","has_children":true,
	   "table":{"table_width":2,"has_column_header":false},
	   "children":[
	     {"id":"r1","type":"table_row","table_row":{"cells":[
	       [{"type":"text","plain_text":"가","annotations":{}}],
	       [{"type":"text","plain_text":"나","annotations":{}}]]}}
	   ]}
	]`))

	if !strings.Contains(md, "| 가 | 나 |") {
		t.Errorf("행 내용이 유실됐다:\n%s", md)
	}
	if !strings.Contains(md, "| --- | --- |") {
		t.Errorf("구분 행이 없어 표로 인식되지 않는다:\n%s", md)
	}
	if len(rep.Warnings()) != 0 {
		t.Errorf("헤더 없는 표는 경고가 아니라 note여야 한다: %+v", rep.Warnings())
	}
}

// TestTableCellWithPipeIsEscaped는 셀 안의 파이프가 열을 쪼개지 않는지 본다.
func TestTableCellWithPipeIsEscaped(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"table","has_children":true,
	   "table":{"table_width":1,"has_column_header":true},
	   "children":[
	     {"id":"r1","type":"table_row","table_row":{"cells":[
	       [{"type":"text","plain_text":"a | b","annotations":{}}]]}}
	   ]}
	]`))

	if !strings.Contains(md, `a \| b`) {
		t.Errorf("파이프가 이스케이프되지 않았다:\n%s", md)
	}
}

// TestUnknownBlockTypeWarns는 모르는 타입을 조용히 버리지 않는지 본다.
func TestUnknownBlockTypeWarns(t *testing.T) {
	_, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"breadcrumb","breadcrumb":{}}
	]`))

	warnings := rep.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("경고가 1건이어야 하는데 %d건", len(warnings))
	}
	if warnings[0].BlockType != "breadcrumb" {
		t.Errorf("경고의 블록 타입이 다르다: %s", warnings[0].BlockType)
	}
	if warnings[0].Path == "" {
		t.Error("경고에 위치 정보가 없다")
	}
}

// TestUnsupportedBlockWarns는 노션이 API로 못 주는 블록을 경고로 남기는지 본다.
func TestUnsupportedBlockWarns(t *testing.T) {
	_, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"unsupported","unsupported":{"block_type":"alias"}}
	]`))

	warnings := rep.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("경고가 1건이어야 하는데 %d건", len(warnings))
	}
	if !strings.Contains(warnings[0].Message, "alias") {
		t.Errorf("원래 블록 타입이 경고에 없다: %s", warnings[0].Message)
	}
}

// TestWarningPathShowsNesting은 중첩된 블록의 위치가 경고에 드러나는지 본다.
func TestWarningPathShowsNesting(t *testing.T) {
	_, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"toggle","has_children":true,
	   "toggle":{"rich_text":[]},
	   "children":[
	     {"id":"b2","type":"breadcrumb","breadcrumb":{}}
	   ]}
	]`))

	warnings := rep.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("경고가 1건이어야 하는데 %d건", len(warnings))
	}
	path := warnings[0].Path
	if !strings.Contains(path, "toggle") || !strings.Contains(path, "children") {
		t.Errorf("중첩 위치가 드러나지 않는다: %s", path)
	}
}

// TestColumnListFlattens는 단 레이아웃의 내용이 유실되지 않는지 본다.
func TestColumnListFlattens(t *testing.T) {
	md, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"column_list","has_children":true,"column_list":{},
	   "children":[
	     {"id":"c1","type":"column","has_children":true,"column":{"width_ratio":0.5},
	      "children":[{"id":"p1","type":"paragraph","paragraph":{"rich_text":[{"type":"text","plain_text":"왼쪽","annotations":{}}]}}]},
	     {"id":"c2","type":"column","has_children":true,"column":{"width_ratio":0.5},
	      "children":[{"id":"p2","type":"paragraph","paragraph":{"rich_text":[{"type":"text","plain_text":"오른쪽","annotations":{}}]}}]}
	   ]}
	]`))

	if !strings.Contains(md, "왼쪽") || !strings.Contains(md, "오른쪽") {
		t.Errorf("단 내용이 유실됐다:\n%s", md)
	}
	if len(rep.Warnings()) != 0 {
		t.Errorf("레이아웃 평탄화는 경고가 아니라 note여야 한다: %+v", rep.Warnings())
	}
}

// TestSyncedBlockCopyIsSkipped는 사본 synced_block을 펼쳐서 내용을 중복시키지 않는지 본다.
func TestSyncedBlockCopyIsSkipped(t *testing.T) {
	md, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"synced_block","has_children":true,
	   "synced_block":{"synced_from":{"type":"block_id","block_id":"orig"}},
	   "children":[{"id":"p1","type":"paragraph","paragraph":{"rich_text":[{"type":"text","plain_text":"복제된 내용","annotations":{}}]}}]}
	]`))

	if strings.Contains(md, "복제된 내용") {
		t.Errorf("사본 블록이 펼쳐져 내용이 중복된다:\n%s", md)
	}
	if len(rep.Issues) == 0 {
		t.Error("사본을 건너뛴 사실이 기록되지 않았다")
	}
}

// TestSyncedBlockOriginalIsRendered는 원본 synced_block은 내용을 살리는지 본다.
func TestSyncedBlockOriginalIsRendered(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"synced_block","has_children":true,
	   "synced_block":{"synced_from":null},
	   "children":[{"id":"p1","type":"paragraph","paragraph":{"rich_text":[{"type":"text","plain_text":"원본 내용","annotations":{}}]}}]}
	]`))

	if !strings.Contains(md, "원본 내용") {
		t.Errorf("원본 synced_block의 내용이 유실됐다:\n%s", md)
	}
}

// TestImageInsideCodeBlockNotCounted는 코드 예제 안의 이미지 문법을
// 실제 이미지로 잘못 세지 않는지 본다.
func TestImageInsideCodeBlockNotCounted(t *testing.T) {
	_, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"code","code":{"language":"markdown","rich_text":[
	    {"type":"text","plain_text":"![예시](/img/foo)","annotations":{}}]}}
	]`))

	if rep.OutputImages != 0 {
		t.Errorf("코드 블록 안의 이미지 문법을 세었다: %d", rep.OutputImages)
	}
	if rep.SourceImages != 0 {
		t.Errorf("원본 이미지 수가 틀렸다: %d", rep.SourceImages)
	}
}

func TestPageTitleFromVaryingPropertyName(t *testing.T) {
	for _, propName := range []string{"이름", "title", "Name", "단원"} {
		dump := `{"page":{"id":"p1","properties":{"` + propName +
			`":{"id":"title","type":"title","title":[{"type":"text","plain_text":"제목입니다","annotations":{}}]}}},"blocks":[]}`
		_, rep := convertJSON(t, dump)
		if rep.Title != "제목입니다" {
			t.Errorf("프로퍼티 이름이 %q일 때 제목을 못 읽었다: %q", propName, rep.Title)
		}
	}
}

// TestTextLengthTracksContent는 길이 비교가 실제로 유실을 잡아내는지 본다.
func TestTextLengthTracksContent(t *testing.T) {
	_, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"paragraph","paragraph":{"rich_text":[{"type":"text","plain_text":"열글자입니다다","annotations":{}}]}}
	]`))

	if rep.SourceTextLen != 7 {
		t.Errorf("원본 길이가 틀렸다: %d", rep.SourceTextLen)
	}
	if rep.TextShrank() {
		t.Errorf("줄지 않았는데 급감으로 판정했다: 원본 %d, 결과 %d", rep.SourceTextLen, rep.OutputTextLen)
	}
}

// TestTextShrinkDetected는 내용이 통째로 빠지면 길이 검사가 잡아내는지 본다.
func TestTextShrinkDetected(t *testing.T) {
	_, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"breadcrumb","breadcrumb":{"rich_text":[
	    {"type":"text","plain_text":"이 긴 내용은 변환기가 모르는 타입이라 통째로 사라진다","annotations":{}}]}}
	]`))

	if !rep.TextShrank() {
		t.Errorf("내용이 사라졌는데 길이 검사가 통과했다: 원본 %d, 결과 %d",
			rep.SourceTextLen, rep.OutputTextLen)
	}
}

// TestNestedListStaysTight는 리스트 항목과 그 하위 리스트 사이에 빈 줄이 생기지
// 않는지 본다. 빈 줄이 있으면 마크다운이 리스트 전체를 느슨한 리스트로 보고
// 항목마다 문단을 감싸서, 노션에서 촘촘하던 목록이 띄엄띄엄해진다.
func TestNestedListStaysTight(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"bulleted_list_item","has_children":true,
	   "bulleted_list_item":{"rich_text":[{"type":"text","plain_text":"부모","annotations":{}}]},
	   "children":[
	     {"id":"b2","type":"bulleted_list_item",
	      "bulleted_list_item":{"rich_text":[{"type":"text","plain_text":"자식","annotations":{}}]}}
	   ]}
	]`))

	if strings.TrimSpace(md) != "- 부모\n  - 자식" {
		t.Errorf("중첩 리스트 사이에 빈 줄이 있다: %q", strings.TrimSpace(md))
	}
}

// TestListItemWithParagraphChildKeepsBlankLine은 자식이 리스트가 아니라 문단이면
// 빈 줄을 유지하는지 본다. 빈 줄이 없으면 문단이 앞 항목에 이어붙는다.
func TestListItemWithParagraphChildKeepsBlankLine(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"bulleted_list_item","has_children":true,
	   "bulleted_list_item":{"rich_text":[{"type":"text","plain_text":"항목","annotations":{}}]},
	   "children":[
	     {"id":"b2","type":"paragraph",
	      "paragraph":{"rich_text":[{"type":"text","plain_text":"딸린 설명","annotations":{}}]}}
	   ]}
	]`))

	if strings.TrimSpace(md) != "- 항목\n\n  딸린 설명" {
		t.Errorf("문단 자식의 빈 줄이 사라졌다: %q", strings.TrimSpace(md))
	}
}

// TestTrailingNewlineInParagraphCollapsed는 노션 문단 끝의 개행이
// 빈 줄을 여러 개 만들지 않는지 본다.
func TestTrailingNewlineInParagraphCollapsed(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"paragraph","paragraph":{"rich_text":[{"type":"text","plain_text":"본문\n","annotations":{}}]}},
	  {"id":"b2","type":"paragraph","paragraph":{"rich_text":[{"type":"text","plain_text":"다음","annotations":{}}]}}
	]`))

	if strings.Contains(md, "\n\n\n") {
		t.Errorf("빈 줄이 두 개 이상 연속된다: %q", md)
	}
	if strings.TrimSpace(md) != "본문\n\n다음" {
		t.Errorf("got %q", strings.TrimSpace(md))
	}
}

// TestNumberingContinuesAcrossCodeBlock은 코드 블록이 끼어도 번호가 이어지는지 본다.
// 노션 원본에서 이렇게 보이는 것을 확인했다(85fc2ef1 페이지).
func TestNumberingContinuesAcrossCodeBlock(t *testing.T) {
	md, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"numbered_list_item","numbered_list_item":{"rich_text":[{"type":"text","plain_text":"페이지 테이블을 2개 만든다.","annotations":{}}]}},
	  {"id":"b2","type":"code","code":{"language":"text","rich_text":[{"type":"text","plain_text":"x","annotations":{}}]}},
	  {"id":"b3","type":"numbered_list_item","numbered_list_item":{"rich_text":[{"type":"text","plain_text":"페이지 테이블을 3개 만든다.","annotations":{}}]}}
	]`))

	if !strings.Contains(md, "1. 페이지 테이블을 2개 만든다.") {
		t.Errorf("첫 항목이 1이 아니다:\n%s", md)
	}
	if !strings.Contains(md, "2. 페이지 테이블을 3개 만든다.") {
		t.Errorf("코드 블록 뒤 항목의 번호가 이어지지 않았다:\n%s", md)
	}

	var noted bool
	for _, iss := range rep.Issues {
		if iss.BlockType == "numbered_list_item" && strings.Contains(iss.Message, "이었다") {
			noted = true
		}
	}
	if !noted {
		t.Errorf("번호를 이어붙인 사실이 기록되지 않았다: %+v", rep.Issues)
	}
}

// TestNumberingContinuesAcrossImageAndEquation은 그림과 수식도 목록을 끊지 않는지 본다.
func TestNumberingContinuesAcrossImageAndEquation(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"numbered_list_item","numbered_list_item":{"rich_text":[{"type":"text","plain_text":"하나","annotations":{}}]}},
	  {"id":"b2","type":"image","image":{"type":"file","caption":[],"local":{"sha256":"abc"}}},
	  {"id":"b3","type":"numbered_list_item","numbered_list_item":{"rich_text":[{"type":"text","plain_text":"둘","annotations":{}}]}},
	  {"id":"b4","type":"equation","equation":{"expression":"x^2"}},
	  {"id":"b5","type":"numbered_list_item","numbered_list_item":{"rich_text":[{"type":"text","plain_text":"셋","annotations":{}}]}}
	]`))

	for _, want := range []string{"1. 하나", "2. 둘", "3. 셋"} {
		if !strings.Contains(md, want) {
			t.Errorf("%q가 없다:\n%s", want, md)
		}
	}
}

// TestNumberingContinuesAcrossEmptyParagraph은 여백용 빈 문단이
// 목록을 끊지 않는지 본다. 노션에서 줄 간격 용도로 흔히 쓰인다.
func TestNumberingContinuesAcrossEmptyParagraph(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"numbered_list_item","numbered_list_item":{"rich_text":[{"type":"text","plain_text":"하나","annotations":{}}]}},
	  {"id":"b2","type":"paragraph","paragraph":{"rich_text":[]}},
	  {"id":"b3","type":"numbered_list_item","numbered_list_item":{"rich_text":[{"type":"text","plain_text":"둘","annotations":{}}]}}
	]`))

	if !strings.Contains(md, "2. 둘") {
		t.Errorf("빈 문단이 목록을 끊었다:\n%s", md)
	}
}

// TestNumberingRestartsAfterHeading은 제목이 끼면 새 목록으로 보는지 본다.
// 제목은 새 절이므로 그 아래 목록은 1부터 시작해야 한다.
func TestNumberingRestartsAfterHeading(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"numbered_list_item","numbered_list_item":{"rich_text":[{"type":"text","plain_text":"앞 목록","annotations":{}}]}},
	  {"id":"b2","type":"heading_2","heading_2":{"rich_text":[{"type":"text","plain_text":"새 절","annotations":{}}]}},
	  {"id":"b3","type":"numbered_list_item","numbered_list_item":{"rich_text":[{"type":"text","plain_text":"새 목록","annotations":{}}]}}
	]`))

	if !strings.Contains(md, "1. 새 목록") {
		t.Errorf("제목 뒤인데 번호가 1부터 시작하지 않았다:\n%s", md)
	}
}

// TestNumberingRestartsAfterLeadInParagraph은 내용이 있는 문단이 끼면
// 새 목록으로 보는지 본다. 이런 문단은 대개 "알고리즘은 다음과 같이 작동한다" 같은
// 새 목록의 도입문이다.
func TestNumberingRestartsAfterLeadInParagraph(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"numbered_list_item","numbered_list_item":{"rich_text":[{"type":"text","plain_text":"연결 리스트 상의 다음 원소 포인터","annotations":{}}]}},
	  {"id":"b2","type":"paragraph","paragraph":{"rich_text":[{"type":"text","plain_text":"알고리즘은 다음과 같이 작동한다.","annotations":{}}]}},
	  {"id":"b3","type":"numbered_list_item","numbered_list_item":{"rich_text":[{"type":"text","plain_text":"페이지 번호를 해싱한다.","annotations":{}}]}}
	]`))

	if !strings.Contains(md, "1. 페이지 번호를 해싱한다.") {
		t.Errorf("도입 문단 뒤인데 번호가 1부터 시작하지 않았다:\n%s", md)
	}
}

// TestUninterruptedNumberingIsNotFlagged는 끊기지 않은 목록에는
// 불필요한 알림이 붙지 않는지 본다.
func TestUninterruptedNumberingIsNotFlagged(t *testing.T) {
	_, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"numbered_list_item","numbered_list_item":{"rich_text":[{"type":"text","plain_text":"하나","annotations":{}}]}},
	  {"id":"b2","type":"numbered_list_item","numbered_list_item":{"rich_text":[{"type":"text","plain_text":"둘","annotations":{}}]}}
	]`))

	if len(rep.Issues) != 0 {
		t.Errorf("멀쩡한 목록에 알림이 붙었다: %+v", rep.Issues)
	}
}

// TestCaptionWithFormattingIsNotReportedLost는 서식이 들어간 캡션을
// 유실로 잘못 잡지 않는지 본다. 렌더링된 캡션에는 백틱이나 별표가 끼어들어서
// 서식 없는 원문으로 대조하면 멀쩡한 캡션이 사라진 것처럼 보인다.
func TestCaptionWithFormattingIsNotReportedLost(t *testing.T) {
	md, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"code","code":{"language":"sql","rich_text":[
	    {"type":"text","plain_text":"SELECT 1","annotations":{}}],
	   "caption":[
	    {"type":"text","plain_text":"salary가 NULL일 경우 ","annotations":{}},
	    {"type":"text","plain_text":"N/A","annotations":{"code":true}},
	    {"type":"text","plain_text":"를 반환","annotations":{}}]}}
	]`))

	if rep.SourceCaptions != 1 {
		t.Fatalf("원본 캡션 수가 틀렸다: %d", rep.SourceCaptions)
	}
	if !rep.CaptionsMatch() {
		t.Errorf("서식 있는 캡션을 유실로 잡았다: 원본 %d → 결과 %d\n%s",
			rep.SourceCaptions, rep.OutputCaptions, md)
	}
	if !strings.Contains(md, "`N/A`") {
		t.Errorf("캡션의 코드 서식이 결과에 없다:\n%s", md)
	}
}

// TestInlineEquationWithNewlinesStaysOnOneLine은 개행이 든 인라인 수식이
// 한 줄로 펴지는지 본다. 여러 줄로 두면 마크다운에서 빈 줄이 문단을 끊어
// 여는 $와 닫는 $가 갈라지고, 수식이 렌더링되지 않고 $가 글자로 보인다.
func TestInlineEquationWithNewlinesStaysOnOneLine(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"paragraph","paragraph":{"rich_text":[
	    {"type":"equation","equation":{"expression":"A = (A \\cap B) \\cup (A \\cap B^c)\n\n"},"plain_text":"x","annotations":{}}
	  ]}}
	]`))

	got := strings.TrimSpace(md)
	if strings.Contains(got, "\n") {
		t.Errorf("인라인 수식이 여러 줄로 나뉘었다: %q", got)
	}
	want := `$A = (A \cap B) \cup (A \cap B^c)$`
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// TestInlineEquationInListKeepsLatexIntact는 리스트 안의 인라인 수식에
// 들여쓰기가 끼어들어 LaTeX 문자열이 변형되지 않는지 본다.
func TestInlineEquationInListKeepsLatexIntact(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"bulleted_list_item","has_children":true,
	   "bulleted_list_item":{"rich_text":[{"type":"text","plain_text":"항목","annotations":{}}]},
	   "children":[
	     {"id":"b2","type":"bulleted_list_item","bulleted_list_item":{"rich_text":[
	       {"type":"equation","equation":{"expression":"A \\to B\\; then\\; \nB^{c} \\to A^{c}"},"plain_text":"x","annotations":{}}]}}
	   ]}
	]`))

	want := `$A \to B\; then\; B^{c} \to A^{c}$`
	if !strings.Contains(md, want) {
		t.Errorf("LaTeX에 들여쓰기가 끼어들었다:\n%s", md)
	}
}

// TestBlockEquationDropsBlankLines는 블록 수식 안의 빈 줄이 사라지는지 본다.
// $$ 안의 빈 줄은 마크다운 파서가 문단 경계로 볼 수 있다.
func TestBlockEquationDropsBlankLines(t *testing.T) {
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"equation","equation":{"expression":"\\begin {align*}\n\n\\because A \\\\\n\n&1 = P(S)\n\\end {align*}\n"}}
	]`))

	got := strings.TrimSpace(md)
	if strings.Contains(got, "\n\n") {
		t.Errorf("블록 수식 안에 빈 줄이 남았다: %q", got)
	}
	// 줄 구분 자체는 유지돼야 한다 (align 환경 가독성).
	if !strings.Contains(got, "\\begin {align*}\n\\because A") {
		t.Errorf("여러 줄 배치가 깨졌다: %q", got)
	}
	// LaTeX의 진짜 줄바꿈인 \\ 는 그대로 남아야 한다.
	if !strings.Contains(got, `\\`) {
		t.Errorf(`LaTeX 줄바꿈 \\ 가 사라졌다: %q`, got)
	}
}

// TestBlockEquationKeepsBackslashesAndBraces는 이스케이프가 끼어들지 않는지 본다.
func TestBlockEquationKeepsBackslashesAndBraces(t *testing.T) {
	expr := `\sum_{i=1}^{k}\;(A \cap B_{i}) \mid \{x\}`
	md, _ := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"equation","equation":{"expression":"\\sum_{i=1}^{k}\\;(A \\cap B_{i}) \\mid \\{x\\}"}}
	]`))

	if !strings.Contains(md, expr) {
		t.Errorf("LaTeX가 변형됐다:\ngot  %q\nwant %q", strings.TrimSpace(md), expr)
	}
	for _, bad := range []string{`\\_`, `\\{`, `\\|`} {
		if strings.Contains(md, bad) {
			t.Errorf("이스케이프가 추가됐다 (%s):\n%s", bad, md)
		}
	}
}

// TestEmptyImageLeavesComment는 노션에서 이미 비어 있던 이미지 자리에
// 주석이 남는지 본다. 빈 참조 ![]()를 남기면 렌더링이 깨지고,
// 아무것도 안 남기면 원본 대조 때 흔적이 사라진다.
func TestEmptyImageLeavesComment(t *testing.T) {
	md, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"paragraph","paragraph":{"rich_text":[{"type":"text","plain_text":"앞","annotations":{}}]}},
	  {"id":"b2","type":"image","image":{"type":"external","caption":[],"external":{"url":""},"local":null}},
	  {"id":"b3","type":"paragraph","paragraph":{"rich_text":[{"type":"text","plain_text":"뒤","annotations":{}}]}}
	]`))

	if !strings.Contains(md, EmptyImageMarker) {
		t.Errorf("빈 이미지 자리에 주석이 없다:\n%s", md)
	}
	if strings.Contains(md, "![](") || strings.Contains(md, "![]()") {
		t.Errorf("빈 이미지 참조가 남았다:\n%s", md)
	}
	// 주석은 이미지 참조로 세면 안 된다.
	if rep.OutputImages != 0 {
		t.Errorf("주석을 이미지 참조로 세었다: %d", rep.OutputImages)
	}
	if rep.SourceImages != 1 {
		t.Errorf("원본 이미지 수가 틀렸다: %d", rep.SourceImages)
	}
	// 앞뒤 문단이 그대로 있어야 한다.
	if !strings.Contains(md, "앞") || !strings.Contains(md, "뒤") {
		t.Errorf("주변 내용이 사라졌다:\n%s", md)
	}
}

// TestEmptyEquationLeavesComment는 빈 수식 블록 자리에 주석이 남는지 본다.
func TestEmptyEquationLeavesComment(t *testing.T) {
	md, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"equation","equation":{"expression":"   "}}
	]`))

	if !strings.Contains(md, EmptyEquationMarker) {
		t.Errorf("빈 수식 자리에 주석이 없다:\n%s", md)
	}
	if strings.Contains(md, "$$") {
		t.Errorf("빈 수식 구분자가 남았다:\n%s", md)
	}
	if rep.CountKind(KindEmptyBlock) != 1 {
		t.Errorf("빈 블록이 기록되지 않았다: %+v", rep.Issues)
	}
}
