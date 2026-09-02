package main

import (
	"strings"
	"testing"
)

// **`<javascript>`는 화면에서 사라진다.** CommonMark의 태그 이름 규칙
// (`[A-Za-z][A-Za-z0-9-]*`)에 맞아서 raw HTML로 통과하고, 브라우저는 모르는
// 요소라 아무것도 안 그린다. 글쓴이는 그걸 절 제목 표시로 썼다.
//
// 한글 표시(`<특징>`)는 규칙에 안 맞아 그냥 글자로 남는다 — 우연히 안전한
// 것이므로 건드리지 않는다. 손대면 이미 멀쩡한 것을 바꾸는 셈이다.
func TestEscapeBareTagsOnlyTouchesWhatDisappears(t *testing.T) {
	for _, tc := range []struct {
		name  string
		in    string
		want  string
		fixed int
	}{
		{"라틴 낱말은 사라지므로 고친다", "<javascript>\n", "&lt;javascript&gt;\n", 1},
		{"한글은 그냥 글자다", "<특징>\n", "<특징>\n", 0},
		{"공백이 있으면 태그가 아니다", "<람다식 예제>\n", "<람다식 예제>\n", 0},
		{"진짜 태그도 줄 전체면 고친다", "<br>\n", "&lt;br&gt;\n", 1},
		{
			// **코드 블록 안은 건드리지 않는다.** 고치면 코드가 바뀐다.
			"코드 블록 안", "```text\n<javascript>\n```\n", "```text\n<javascript>\n```\n", 0,
		},
		{"문장 속은 건드리지 않는다", "앞말 <javascript> 뒷말\n", "앞말 <javascript> 뒷말\n", 0},
	} {
		got, fixed := escapeBareTags(tc.in)
		if got != tc.want {
			t.Errorf("%s: %q → %q, want %q", tc.name, tc.in, got, tc.want)
		}
		if len(fixed) != tc.fixed {
			t.Errorf("%s: 고친 줄 %d개, want %d", tc.name, len(fixed), tc.fixed)
		}
	}
}

// 파일 첫머리의 `# 제목`을 떼고 남은 제목을 한 단계 올린다.
//
// **둘은 한 쌍이다.** 떼기만 하면 절이 `##`로 남아 화면에서 h3이 되고,
// h1(글 제목) 바로 아래 층이 빈다. 목차 들여쓰기도 그만큼 밀린다.
func TestDropTitleAndPromoteKeepHeadingsContiguous(t *testing.T) {
	in := "# 3. 람다식 써보기!\n## 복습\n내용\n### 1. 함수형 인터페이스\n"
	got := promoteHeadings(dropLeadingTitle(in))
	want := "# 복습\n내용\n## 1. 함수형 인터페이스\n"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

// 첫 내용이 제목이 아니면 아무것도 떼지 않는다. 그런 파일이 실제로 있다
// (chapter8은 `## 컬랙션 팩토리`로 시작한다).
func TestDropLeadingTitleLeavesBodyWithoutOne(t *testing.T) {
	in := "## 컬랙션 팩토리\n내용\n"
	if got := dropLeadingTitle(in); got != in {
		t.Errorf("제목이 없는데 뗐다: %q", got)
	}
}

// 코드 블록 안의 `# 주석`을 제목으로 착각하면 안 된다.
func TestPromoteHeadingsSkipsCodeFences(t *testing.T) {
	in := "## 절\n```bash\n## 이건 주석이다\n```\n### 하위 절\n"
	got := promoteHeadings(in)
	if !strings.Contains(got, "## 이건 주석이다") {
		t.Errorf("코드 안의 주석을 건드렸다:\n%s", got)
	}
	if !strings.HasPrefix(got, "# 절\n") {
		t.Errorf("절을 안 올렸다:\n%s", got)
	}
	if !strings.Contains(got, "## 하위 절") {
		t.Errorf("하위 절을 안 올렸다:\n%s", got)
	}
}

// `#` 하나짜리는 더 올릴 곳이 없다.
func TestPromoteHeadingsLeavesTopLevelAlone(t *testing.T) {
	in := "# 이미 최상위\n"
	if got := promoteHeadings(in); got != in {
		t.Errorf("최상위 제목을 건드렸다: %q", got)
	}
}

// prepareBody는 손질 넷을 한 쌍으로 묶는다. **조합 자체가 규칙이라** 그것을
// 여기서 지킨다 — 예전에는 손질 하나하나만 테스트해서, 호출부에서 하나를
// 빼도 아무 테스트도 실패하지 않았다.
func TestPrepareBodyDoesAllThree(t *testing.T) {
	raw := "# 3. 람다식 써보기!\n## 복습\n<javascript>\n### 하위\n\n**경로 표현(Path Expression)**으로 찾는다\n"
	body, fixed, bold := prepareBody(raw)

	// 짝이 안 되는 굵게는 <strong>으로 나가야 한다. 안 그러면 별표가 글자로 보인다.
	if !strings.Contains(body, "<strong>경로 표현(Path Expression)</strong>") {
		t.Errorf("짝이 안 되는 굵게를 안 고쳤다 — 별표가 글자로 보인다:\n%s", body)
	}
	if len(bold) != 1 {
		t.Errorf("고친 굵게가 %d개다, want 1", len(bold))
	}

	if strings.Contains(body, "# 3. 람다식 써보기!") {
		t.Error("글 제목을 안 뗐다 — 페이지에 같은 말이 두 번 나온다")
	}
	if !strings.HasPrefix(body, "# 복습") {
		t.Errorf("제목을 안 올렸다 — h1 다음이 h3이 된다:\n%s", body)
	}
	if !strings.Contains(body, "## 하위") {
		t.Errorf("하위 제목을 안 올렸다:\n%s", body)
	}
	if !strings.Contains(body, "&lt;javascript&gt;") {
		t.Errorf("꺾쇠 표시를 안 고쳤다 — 화면에서 사라진다:\n%s", body)
	}
	if len(fixed) != 1 {
		t.Errorf("고친 줄이 %d개다, want 1", len(fixed))
	}
}
