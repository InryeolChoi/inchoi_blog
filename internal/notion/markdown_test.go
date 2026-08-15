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

// TestNumberingRestartIsRecorded는 번호 목록이 끊겼다 다시 시작될 때
// 그 사실이 리포트에 남는지 본다. 노션은 번호를 저장하지 않아서 이어지는
// 목록이었는지 새 목록이었는지 데이터만으로는 알 수 없다.
func TestNumberingRestartIsRecorded(t *testing.T) {
	_, rep := convertJSON(t, blocksJSON(`[
	  {"id":"b1","type":"numbered_list_item","numbered_list_item":{"rich_text":[{"type":"text","plain_text":"하나","annotations":{}}]}},
	  {"id":"b2","type":"code","code":{"language":"c","rich_text":[{"type":"text","plain_text":"x","annotations":{}}]}},
	  {"id":"b3","type":"numbered_list_item","numbered_list_item":{"rich_text":[{"type":"text","plain_text":"둘일까 하나일까","annotations":{}}]}}
	]`))

	var found bool
	for _, iss := range rep.Issues {
		if iss.BlockType == "numbered_list_item" && strings.Contains(iss.Message, "1부터 다시") {
			found = true
		}
	}
	if !found {
		t.Errorf("번호 재시작이 기록되지 않았다: %+v", rep.Issues)
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
