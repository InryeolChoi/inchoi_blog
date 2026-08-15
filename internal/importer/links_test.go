package importer

import (
	"strings"
	"testing"
)

func TestRewriteLinksReplacesPathOnly(t *testing.T) {
	slugs := map[string]string{"abc-123": "paging"}
	body := "앞 [3. 페이징 (paging)](/p/abc-123) 뒤"

	got, res := RewriteLinks(body, slugs)

	want := "앞 [3. 페이징 (paging)](/paging) 뒤"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if res.Replaced() != 1 {
		t.Errorf("교체 수가 다르다: %d", res.Replaced())
	}
}

// TestRewriteLinksLeavesLinkTextAlone은 링크 텍스트에 /p/ 같은 문자열이 있어도
// 건드리지 않는지 본다. 경로만 바꿔야 한다.
func TestRewriteLinksLeavesLinkTextAlone(t *testing.T) {
	slugs := map[string]string{"abc-123": "paging"}
	body := "[경로는 /p/abc-123 이다](/p/abc-123)"

	got, res := RewriteLinks(body, slugs)

	want := "[경로는 /p/abc-123 이다](/paging)"
	if got != want {
		t.Errorf("링크 텍스트가 바뀌었다:\ngot  %q\nwant %q", got, want)
	}
	if res.Replaced() != 1 {
		t.Errorf("교체 수가 다르다: %d", res.Replaced())
	}
}

// TestRewriteLinksIgnoresExternalNotionURLs는 본문의 노션 웹 주소를
// 건드리지 않는지 본다. 실제 덤프에 https://app.notion.com/p/... 링크가 3개 있다.
func TestRewriteLinksIgnoresExternalNotionURLs(t *testing.T) {
	slugs := map[string]string{"a0f2738171764b0c8dc8d3475ab74e8d": "something"}
	body := "[MGF를 이용](https://app.notion.com/p/a0f2738171764b0c8dc8d3475ab74e8d#5045d6)"

	got, res := RewriteLinks(body, slugs)

	if got != body {
		t.Errorf("외부 URL이 바뀌었다:\ngot  %q\nwant %q", got, body)
	}
	if res.Replaced() != 0 {
		t.Errorf("외부 URL을 교체했다: %d", res.Replaced())
	}
}

// TestRewriteLinksKeepsUnresolved는 대응하는 글이 없으면 그대로 두는지 본다.
// 인라인 데이터베이스(child_database)가 여기 해당한다. 페이지가 아니라서
// posts에 행이 없는데 경로만 바꾸면 깨진 링크가 멀쩡해 보인다.
func TestRewriteLinksKeepsUnresolved(t *testing.T) {
	slugs := map[string]string{"known": "known-slug"}
	body := "[있는 글](/p/known) 그리고 [인라인 DB](/p/unknown-db)"

	got, res := RewriteLinks(body, slugs)

	if !strings.Contains(got, "[있는 글](/known-slug)") {
		t.Errorf("매칭되는 링크가 안 바뀌었다: %q", got)
	}
	if !strings.Contains(got, "[인라인 DB](/p/unknown-db)") {
		t.Errorf("매칭 안 되는 링크를 건드렸다: %q", got)
	}
	if res.Replaced() != 1 {
		t.Errorf("교체 수가 다르다: %d", res.Replaced())
	}
	if res.Unresolved["unknown-db"] != 1 {
		t.Errorf("미해결이 기록되지 않았다: %+v", res.Unresolved)
	}
	if res.UnresolvedCount() != 1 {
		t.Errorf("미해결 총계가 다르다: %d", res.UnresolvedCount())
	}
}

func TestRewriteLinksHandlesMultiplePerBody(t *testing.T) {
	slugs := map[string]string{"a": "sa", "b": "sb", "c": "sc"}
	body := "[A](/p/a)\n\n- [B](/p/b)\n- [C](/p/c)\n"

	got, res := RewriteLinks(body, slugs)

	for _, want := range []string{"[A](/sa)", "[B](/sb)", "[C](/sc)"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q가 없다:\n%s", want, got)
		}
	}
	if res.Replaced() != 3 {
		t.Errorf("교체 수가 다르다: %d", res.Replaced())
	}
}

// TestRewriteLinksIsIdempotent는 두 번 돌려도 결과가 같은지 본다.
func TestRewriteLinksIsIdempotent(t *testing.T) {
	slugs := map[string]string{"a": "sa"}
	body := "[A](/p/a)"

	once, _ := RewriteLinks(body, slugs)
	twice, res := RewriteLinks(once, slugs)

	if once != twice {
		t.Errorf("두 번째 실행에서 또 바뀌었다: %q → %q", once, twice)
	}
	if res.Replaced() != 0 {
		t.Errorf("두 번째 실행에서 교체가 일어났다: %d", res.Replaced())
	}
}

// TestRewriteLinksLeavesImagePathsAlone은 이미지 경로를 건드리지 않는지 본다.
func TestRewriteLinksLeavesImagePathsAlone(t *testing.T) {
	slugs := map[string]string{"a": "sa"}
	body := "![](/img/deadbeef) [A](/p/a)"

	got, _ := RewriteLinks(body, slugs)

	if !strings.Contains(got, "![](/img/deadbeef)") {
		t.Errorf("이미지 경로가 바뀌었다: %q", got)
	}
}

func TestCountPageLinks(t *testing.T) {
	body := "[A](/p/a) [B](/p/b) [외부](https://app.notion.com/p/x) [C](/sc)"
	if got := CountPageLinks(body); got != 2 {
		t.Errorf("CountPageLinks = %d, want 2", got)
	}
}

func TestValidateRewritten(t *testing.T) {
	if err := ValidateRewritten("[A](/sa)"); err != nil {
		t.Errorf("깨끗한 본문인데 에러가 났다: %v", err)
	}
	if err := ValidateRewritten("[A](/p/a)"); err == nil {
		t.Error("/p/가 남았는데 통과했다")
	}
}

func TestSlugLinkTargetsSkipsImages(t *testing.T) {
	body := "![](/img/abc) [A](/sa) [B](/sb) [외부](https://example.com)"
	got := SlugLinkTargets(body)

	want := map[string]bool{"sa": true, "sb": true}
	if len(got) != 2 {
		t.Fatalf("대상 수가 다르다: %v", got)
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("예상 밖 대상: %q", g)
		}
	}
}

// TestRewriteLinksHandlesNotionInlineHrefs는 본문 인라인 링크의 노션 경로를
// 바꾸는지 본다. 하위 페이지 블록과 달리 /p/ 접두사가 없고 하이픈도 없다.
func TestRewriteLinksHandlesNotionInlineHrefs(t *testing.T) {
	slugs := SlugIndex(map[string]string{"3afca6f5-eb55-4276-98fb-9b7c2fab863c": "확률"})
	body := "[확률 문서](/3afca6f5eb55427698fb9b7c2fab863c)"

	got, res := RewriteLinks(body, slugs)

	if got != "[확률 문서](/확률)" {
		t.Errorf("got %q", got)
	}
	if res.InlineLinks != 1 {
		t.Errorf("인라인 링크 교체 수가 다르다: %d", res.InlineLinks)
	}
	if res.PageLinks != 0 {
		t.Errorf("/p/ 교체로 잘못 셌다: %d", res.PageLinks)
	}
}

// TestRewriteLinksKeepsFragment는 페이지 안의 특정 블록을 가리키는 #조각을
// 유지하는지 본다.
func TestRewriteLinksKeepsFragment(t *testing.T) {
	slugs := SlugIndex(map[string]string{"8ef5464f-7a44-41c5-9850-f9804ff9cf2f": "vector-space"})
	body := "[벡터공간](/8ef5464f7a4441c59850f9804ff9cf2f#ad28eca3505f4c6599ed14d872c5c302)"

	got, _ := RewriteLinks(body, slugs)

	want := "[벡터공간](/vector-space#ad28eca3505f4c6599ed14d872c5c302)"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// TestSlugIndexAcceptsBothIDForms는 하이픈 있는 형태와 없는 형태 둘 다
// 같은 slug로 찾히는지 본다.
func TestSlugIndexAcceptsBothIDForms(t *testing.T) {
	idx := SlugIndex(map[string]string{"abc-123-def": "s"})
	if idx["abc-123-def"] != "s" {
		t.Error("하이픈 있는 형태를 못 찾는다")
	}
	if idx["abc123def"] != "s" {
		t.Error("하이픈 없는 형태를 못 찾는다")
	}
}

// TestSlugLinkTargetsSkipsRemainingPageLinks는 /p/ 링크를 slug 대상으로
// 잘못 세지 않는지 본다. 따로 집계하는 항목이라 여기서 또 세면 중복 보고가 된다.
func TestSlugLinkTargetsSkipsRemainingPageLinks(t *testing.T) {
	got := SlugLinkTargets("[A](/sa) [DB](/p/unresolved-db) ![](/img/abc)")
	if len(got) != 1 || got[0] != "sa" {
		t.Errorf("got %v, want [sa]", got)
	}
}

// TestSlugLinkTargetsStripsFragment는 #조각을 떼고 경로만 보는지 확인한다.
func TestSlugLinkTargetsStripsFragment(t *testing.T) {
	got := SlugLinkTargets("[A](/sa#block-id)")
	if len(got) != 1 || got[0] != "sa" {
		t.Errorf("got %v, want [sa]", got)
	}
}

// TestRewriteLinksIgnoresNonNotionPaths는 32자리 16진수가 아닌 경로를
// 건드리지 않는지 본다.
func TestRewriteLinksIgnoresNonNotionPaths(t *testing.T) {
	slugs := SlugIndex(map[string]string{"abc-123": "s"})
	body := "[홈](/) [소개](/about) [태그](/tags/go)"

	got, res := RewriteLinks(body, slugs)

	if got != body {
		t.Errorf("일반 경로가 바뀌었다:\ngot  %q\nwant %q", got, body)
	}
	if res.Replaced() != 0 {
		t.Errorf("교체가 일어났다: %d", res.Replaced())
	}
}

// TestSlugLinkTargetsSkipsUnresolvedNotionLinks는 못 바꾼 노션 인라인 링크를
// "깨진 slug"로 잘못 보고하지 않는지 본다. 이미 미해결로 따로 세고 있어서,
// 여기서 또 세면 의도적으로 남긴 것과 진짜 버그가 구분되지 않는다.
func TestSlugLinkTargetsSkipsUnresolvedNotionLinks(t *testing.T) {
	body := "[A](/sa) [못 찾음](/131ca460a4d94e98ae819460ac72bd50) [조각](/b4bfb788aff34b8eac1d3fe6104cd8c7#5f70789e160b407d94feb46bc6ae50aa)"
	got := SlugLinkTargets(body)
	if len(got) != 1 || got[0] != "sa" {
		t.Errorf("got %v, want [sa]", got)
	}
}
