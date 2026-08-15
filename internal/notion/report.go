package notion

import (
	"fmt"
	"sort"
	"strings"
)

// Severity는 변환 중 발견한 문제의 심각도다.
type Severity string

const (
	// SevWarn은 내용이 유실됐거나 유실됐을 수 있는 경우다. 사람이 봐야 한다.
	SevWarn Severity = "warn"
	// SevNote는 의도적으로 다르게 옮긴 경우다(레이아웃 평탄화 등). 내용은 남아 있다.
	SevNote Severity = "note"
)

// Kind는 이슈의 종류다. 집계할 때 메시지 문자열을 뒤지지 않으려고 둔다.
type Kind string

const (
	KindUnknownBlock     Kind = "unknown-block"     // 변환기가 모르는 타입
	KindUnsupportedBlock Kind = "unsupported-block" // 노션이 API로 안 주는 블록
	KindExternalImage    Kind = "external-image"    // 로컬 파일 없이 외부 URL로 남은 이미지
	KindMissingImage     Kind = "missing-image"     // 로컬 파일도 URL도 없는 이미지
	KindExpiringURL      Kind = "expiring-url"      // 노션 호스팅 파일(URL 만료됨)
	KindMissingURL       Kind = "missing-url"       // URL이 없는 링크/미디어 블록
	KindOrphanTableRow   Kind = "orphan-table-row"  // 표 밖의 table_row
	KindBadTable         Kind = "bad-table"         // 행이 없거나 이상한 표
	KindTableNoHeader    Kind = "table-no-header"   // 헤더 없는 표에 빈 헤더를 넣음
	KindFlattenedColumns Kind = "flattened-columns" // 단 레이아웃을 세로로 폄
	KindDroppedTOC       Kind = "dropped-toc"       // 목차 블록 제거
	KindChildLink        Kind = "child-link"        // 하위 페이지/DB 링크 (slug 재작성 필요)
	KindSyncedCopy       Kind = "synced-copy"       // 사본 synced_block 건너뜀
	KindEmbedAsLink      Kind = "embed-as-link"     // 임베드를 링크로 바꿈
	KindNumberContinued  Kind = "number-continued"  // 끊긴 번호 목록을 이어감
)

// Issue는 블록 하나에서 발견한 문제다.
type Issue struct {
	Severity  Severity
	Kind      Kind
	BlockType string
	BlockID   string
	// Path는 블록 위치다. 예: "blocks[3] > toggle > blocks[1]"
	Path    string
	Message string
}

// Report는 페이지 하나의 변환 검증 결과다.
type Report struct {
	PageID string
	Title  string

	// SourceImages는 원본의 image 블록 수, OutputImages는 결과 마크다운의 이미지 참조 수다.
	SourceImages int
	OutputImages int

	// SourceCaptions는 원본에서 캡션이 달려 있던 블록 수,
	// OutputCaptions는 그중 결과에 캡션이 남은 수다.
	SourceCaptions int
	OutputCaptions int

	// SourceTextLen은 원본 rich_text의 plain_text 총 길이(룬 기준),
	// OutputTextLen은 결과 마크다운 길이(룬 기준)다.
	// 마크다운 문법이 더해지므로 정상이면 Output이 Source보다 크다.
	SourceTextLen int
	OutputTextLen int

	// BlockTypes는 원본에 등장한 블록 타입별 개수다.
	BlockTypes map[string]int

	// NumberingContinued/NumberingRestarted는 번호 목록이 다른 블록에 끊겼을 때
	// 번호를 이어간 횟수와 1부터 다시 매긴 횟수다.
	NumberingContinued int
	NumberingRestarted int

	Issues []Issue
}

// CountKind는 특정 종류의 이슈 수를 센다.
func (r Report) CountKind(kind Kind) int {
	n := 0
	for _, iss := range r.Issues {
		if iss.Kind == kind {
			n++
		}
	}
	return n
}

// IssuesOfKind는 특정 종류의 이슈를 돌려준다.
func (r Report) IssuesOfKind(kind Kind) []Issue {
	var out []Issue
	for _, iss := range r.Issues {
		if iss.Kind == kind {
			out = append(out, iss)
		}
	}
	return out
}

// imageRefPattern이 아니라 결과 문자열을 직접 세는 이유는 마크다운 이미지 문법이
// 코드 블록 안에 우연히 들어 있을 수 있어서다. countImageRefs가 코드 펜스를 건너뛴다.

// Warnings는 심각도가 warn인 이슈만 돌려준다.
func (r Report) Warnings() []Issue {
	var out []Issue
	for _, iss := range r.Issues {
		if iss.Severity == SevWarn {
			out = append(out, iss)
		}
	}
	return out
}

// ImagesMatch는 원본 이미지 수와 결과 이미지 참조 수가 같은지 본다.
func (r Report) ImagesMatch() bool { return r.SourceImages == r.OutputImages }

// CaptionsMatch는 캡션이 유실되지 않았는지 본다.
func (r Report) CaptionsMatch() bool { return r.SourceCaptions == r.OutputCaptions }

// textShrinkThreshold는 이 비율보다 짧아지면 내용이 유실된 걸로 본다.
// 마크다운 문법이 붙으므로 결과는 보통 원본보다 길다. 짧아졌다면 뭔가 빠진 것이다.
const textShrinkThreshold = 0.95

// TextShrank는 결과가 원본보다 눈에 띄게 짧아졌는지 본다.
func (r Report) TextShrank() bool {
	if r.SourceTextLen == 0 {
		return false
	}
	return float64(r.OutputTextLen) < float64(r.SourceTextLen)*textShrinkThreshold
}

// OK는 사람이 확인할 문제가 없는지 본다.
func (r Report) OK() bool {
	return r.ImagesMatch() && r.CaptionsMatch() && !r.TextShrank() && len(r.Warnings()) == 0
}

// String은 페이지 한 건의 리포트를 사람이 읽을 형태로 만든다.
func (r Report) String() string {
	var b strings.Builder

	status := "OK"
	if !r.OK() {
		status = "확인 필요"
	}
	fmt.Fprintf(&b, "[%s] %s (%s)\n", status, r.Title, r.PageID)

	imgMark := "✓"
	if !r.ImagesMatch() {
		imgMark = "✗"
	}
	fmt.Fprintf(&b, "  이미지   %s 원본 %d개 → 결과 참조 %d개\n", imgMark, r.SourceImages, r.OutputImages)

	capMark := "✓"
	if !r.CaptionsMatch() {
		capMark = "✗"
	}
	fmt.Fprintf(&b, "  캡션     %s 원본 %d개 → 결과 %d개\n", capMark, r.SourceCaptions, r.OutputCaptions)

	lenMark := "✓"
	if r.TextShrank() {
		lenMark = "✗"
	}
	ratio := 0.0
	if r.SourceTextLen > 0 {
		ratio = float64(r.OutputTextLen) / float64(r.SourceTextLen)
	}
	fmt.Fprintf(&b, "  길이     %s 원본 %d자 → 결과 %d자 (%.2fx)\n",
		lenMark, r.SourceTextLen, r.OutputTextLen, ratio)

	fmt.Fprintf(&b, "  블록     %s\n", formatBlockTypes(r.BlockTypes))

	if len(r.Issues) == 0 {
		return b.String()
	}
	b.WriteString("  이슈:\n")
	for _, iss := range r.Issues {
		mark := "!"
		if iss.Severity == SevNote {
			mark = "-"
		}
		fmt.Fprintf(&b, "    %s [%s] %s\n        위치: %s\n", mark, iss.BlockType, iss.Message, iss.Path)
	}
	return b.String()
}

func formatBlockTypes(counts map[string]int) string {
	if len(counts) == 0 {
		return "(없음)"
	}
	type kv struct {
		k string
		v int
	}
	items := make([]kv, 0, len(counts))
	for k, v := range counts {
		items = append(items, kv{k, v})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].v != items[j].v {
			return items[i].v > items[j].v
		}
		return items[i].k < items[j].k
	})

	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, fmt.Sprintf("%s×%d", it.k, it.v))
	}
	return strings.Join(parts, ", ")
}
