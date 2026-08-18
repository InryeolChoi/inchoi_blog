package importer

import (
	"strings"
	"testing"
)

// slugSet은 테스트에서 "실제로 존재하는 slug" 집합을 짧게 만든다.
func slugSet(names ...string) map[string]bool {
	out := map[string]bool{}
	for _, n := range names {
		out[n] = true
	}
	return out
}

func TestRewriteLinksReplacesPathOnly(t *testing.T) {
	slugs := map[string]string{"abc-123": "paging"}
	body := "앞 [3. 페이징 (paging)](/p/abc-123) 뒤"

	got, res := RewriteLinks(body, slugs, slugSet("paging"))

	want := "앞 [3. 페이징 (paging)](/p/paging) 뒤"
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

	got, res := RewriteLinks(body, slugs, slugSet("paging"))

	want := "[경로는 /p/abc-123 이다](/p/paging)"
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

	got, res := RewriteLinks(body, slugs, slugSet("something"))

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

	got, res := RewriteLinks(body, slugs, slugSet("known-slug"))

	if !strings.Contains(got, "[있는 글](/p/known-slug)") {
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

	got, res := RewriteLinks(body, slugs, slugSet("sa", "sb", "sc"))

	for _, want := range []string{"[A](/p/sa)", "[B](/p/sb)", "[C](/p/sc)"} {
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
	set := slugSet("sa")
	body := "[A](/p/a)"

	once, _ := RewriteLinks(body, slugs, set)
	twice, res := RewriteLinks(once, slugs, set)

	if once != twice {
		t.Errorf("두 번째 실행에서 또 바뀌었다: %q → %q", once, twice)
	}
	if res.Replaced() != 0 {
		t.Errorf("두 번째 실행에서 교체가 일어났다: %d", res.Replaced())
	}
}

// TestRewriteLinksIsIdempotentWhenSlugIsPageID는 slug가 노션 페이지 ID와 같을 때도
// 멱등한지 본다. 지금 DB가 이 상태다. 결과 형태(/p/)가 입력 형태와 같아서,
// 이미 바뀐 링크를 "또 바꿨다"고 세면 멱등성 검사가 거짓으로 실패한다.
func TestRewriteLinksIsIdempotentWhenSlugIsPageID(t *testing.T) {
	id := "81d8f957-a42e-4cb9-8aa3-8e3e86edaf92"
	slugs := SlugIndex(map[string]string{id: id})
	set := slugSet(id)
	body := "[카이제곱분포](/p/" + id + ")"

	got, res := RewriteLinks(body, slugs, set)

	if got != body {
		t.Errorf("이미 최종 형태인데 바뀌었다:\ngot  %q\nwant %q", got, body)
	}
	if res.Replaced() != 0 {
		t.Errorf("이미 최종 형태인데 교체로 셌다: %d", res.Replaced())
	}
	if res.UnresolvedCount() != 0 {
		t.Errorf("이미 최종 형태인데 미해결로 셌다: %+v", res.Unresolved)
	}
}

// TestRewriteLinksLeavesImagePathsAlone은 이미지 경로를 건드리지 않는지 본다.
func TestRewriteLinksLeavesImagePathsAlone(t *testing.T) {
	slugs := map[string]string{"a": "sa"}
	body := "![](/img/deadbeef) [A](/p/a)"

	got, _ := RewriteLinks(body, slugs, slugSet("sa"))

	if !strings.Contains(got, "![](/img/deadbeef)") {
		t.Errorf("이미지 경로가 바뀌었다: %q", got)
	}
}

// TestRewriteLinksUpgradesBareSlugLinks는 옛 relink가 접두사 없이 써둔 링크를
// /p/로 옮기는지 본다. 서버 라우트가 GET /p/{slug}뿐이라 그대로 두면 404다.
func TestRewriteLinksUpgradesBareSlugLinks(t *testing.T) {
	body := "[카이제곱분포](/sa) 그리고 [t분포](/sb)"

	got, res := RewriteLinks(body, map[string]string{}, slugSet("sa", "sb"))

	want := "[카이제곱분포](/p/sa) 그리고 [t분포](/p/sb)"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if res.BareLinks != 2 {
		t.Errorf("접두사 없는 링크 교체 수가 다르다: %d", res.BareLinks)
	}
	if res.PageLinks != 0 || res.InlineLinks != 0 {
		t.Errorf("다른 항목으로 잘못 셌다: %+v", res)
	}
}

// TestRewriteLinksBareLinkKeepsFragment는 접두사를 붙일 때 #조각을 살리는지 본다.
func TestRewriteLinksBareLinkKeepsFragment(t *testing.T) {
	body := "[벡터공간](/vector-space#ad28eca3505f4c6599ed14d872c5c302)"

	got, _ := RewriteLinks(body, map[string]string{}, slugSet("vector-space"))

	want := "[벡터공간](/p/vector-space#ad28eca3505f4c6599ed14d872c5c302)"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// TestRewriteLinksLeavesNonSlugBarePathsAlone은 slug가 아닌 절대경로를
// 건드리지 않는지 본다. 사이트의 다른 경로까지 /p/ 밑으로 끌고 가면 안 된다.
func TestRewriteLinksLeavesNonSlugBarePathsAlone(t *testing.T) {
	body := "[홈](/) [소개](/about) [태그](/tags/go) ![](/img/abc) [분류](/dev/language)"

	got, res := RewriteLinks(body, map[string]string{}, slugSet("sa"))

	if got != body {
		t.Errorf("slug가 아닌 경로가 바뀌었다:\ngot  %q\nwant %q", got, body)
	}
	if res.Replaced() != 0 {
		t.Errorf("교체가 일어났다: %d", res.Replaced())
	}
}

// TestRewriteLinksBareUpgradeIsIdempotent는 접두사를 두 번 붙이지 않는지 본다.
func TestRewriteLinksBareUpgradeIsIdempotent(t *testing.T) {
	set := slugSet("sa")
	body := "[A](/sa)"

	once, _ := RewriteLinks(body, map[string]string{}, set)
	twice, res := RewriteLinks(once, map[string]string{}, set)

	if once != "[A](/p/sa)" {
		t.Fatalf("첫 실행 결과가 다르다: %q", once)
	}
	if twice != once {
		t.Errorf("두 번째 실행에서 또 바뀌었다: %q → %q", once, twice)
	}
	if res.Replaced() != 0 {
		t.Errorf("두 번째 실행에서 교체가 일어났다: %d", res.Replaced())
	}
}

func TestCountPostLinks(t *testing.T) {
	set := slugSet("sa", "sb")
	body := "[A](/p/sa) [B](/p/sb) [DB](/p/unresolved-db) [외부](https://app.notion.com/p/x) [C](/sc)"
	if got := CountPostLinks(body, set); got != 2 {
		t.Errorf("CountPostLinks = %d, want 2", got)
	}
}

func TestCountNotionPageLinks(t *testing.T) {
	set := slugSet("sa")
	body := "[A](/p/sa) [DB](/p/unresolved-db) [외부](https://app.notion.com/p/x)"
	if got := CountNotionPageLinks(body, set); got != 1 {
		t.Errorf("CountNotionPageLinks = %d, want 1", got)
	}
}

// TestCountPostLinksSeesFragment는 #조각이 붙은 링크도 세는지 본다.
// 조각을 경로에 붙여 읽으면 slug와 안 맞아서 노션 형태로 오해한다.
func TestCountPostLinksSeesFragment(t *testing.T) {
	set := slugSet("sa")
	body := "[A](/p/sa#block-id)"
	if got := CountPostLinks(body, set); got != 1 {
		t.Errorf("CountPostLinks = %d, want 1", got)
	}
	if got := CountNotionPageLinks(body, set); got != 0 {
		t.Errorf("CountNotionPageLinks = %d, want 0", got)
	}
}

func TestValidateRewritten(t *testing.T) {
	set := slugSet("sa")
	if err := ValidateRewritten("[A](/p/sa)", set); err != nil {
		t.Errorf("깨끗한 본문인데 에러가 났다: %v", err)
	}
	if err := ValidateRewritten("[A](/sa)", set); err == nil {
		t.Error("접두사 없는 글 링크가 남았는데 통과했다")
	}
}

func TestBareSlugLinksSkipsImagesAndPostLinks(t *testing.T) {
	set := slugSet("sa", "sb")
	body := "![](/img/abc) [A](/p/sa) [B](/sb) [외부](https://example.com) [소개](/about)"
	got := BareSlugLinks(body, set)

	if len(got) != 1 || got[0] != "sb" {
		t.Errorf("got %v, want [sb]", got)
	}
}

// TestBareSlugLinksStripsFragment는 #조각을 떼고 경로만 보는지 확인한다.
func TestBareSlugLinksStripsFragment(t *testing.T) {
	got := BareSlugLinks("[A](/sa#block-id)", slugSet("sa"))
	if len(got) != 1 || got[0] != "sa" {
		t.Errorf("got %v, want [sa]", got)
	}
}

// TestBareSlugLinksSkipsUnresolvedNotionLinks는 못 바꾼 노션 인라인 링크를
// "접두사 없는 글 링크"로 잘못 보고하지 않는지 본다. 이미 미해결로 따로 세고
// 있어서, 여기서 또 세면 의도적으로 남긴 것과 진짜 버그가 구분되지 않는다.
func TestBareSlugLinksSkipsUnresolvedNotionLinks(t *testing.T) {
	body := "[A](/sa) [못 찾음](/131ca460a4d94e98ae819460ac72bd50) [조각](/b4bfb788aff34b8eac1d3fe6104cd8c7#5f70789e160b407d94feb46bc6ae50aa)"
	got := BareSlugLinks(body, slugSet("sa"))
	if len(got) != 1 || got[0] != "sa" {
		t.Errorf("got %v, want [sa]", got)
	}
}

// TestRewriteLinksHandlesNotionInlineHrefs는 본문 인라인 링크의 노션 경로를
// 바꾸는지 본다. 하위 페이지 블록과 달리 /p/ 접두사가 없고 하이픈도 없다.
func TestRewriteLinksHandlesNotionInlineHrefs(t *testing.T) {
	slugs := SlugIndex(map[string]string{"3afca6f5-eb55-4276-98fb-9b7c2fab863c": "확률"})
	body := "[확률 문서](/3afca6f5eb55427698fb9b7c2fab863c)"

	got, res := RewriteLinks(body, slugs, slugSet("확률"))

	if got != "[확률 문서](/p/확률)" {
		t.Errorf("got %q", got)
	}
	if res.InlineLinks != 1 {
		t.Errorf("인라인 링크 교체 수가 다르다: %d", res.InlineLinks)
	}
	if res.PageLinks != 0 || res.BareLinks != 0 {
		t.Errorf("다른 항목으로 잘못 셌다: %+v", res)
	}
}

// TestRewriteLinksKeepsFragment는 페이지 안의 특정 블록을 가리키는 #조각을
// 유지하는지 본다.
func TestRewriteLinksKeepsFragment(t *testing.T) {
	slugs := SlugIndex(map[string]string{"8ef5464f-7a44-41c5-9850-f9804ff9cf2f": "vector-space"})
	body := "[벡터공간](/8ef5464f7a4441c59850f9804ff9cf2f#ad28eca3505f4c6599ed14d872c5c302)"

	got, _ := RewriteLinks(body, slugs, slugSet("vector-space"))

	want := "[벡터공간](/p/vector-space#ad28eca3505f4c6599ed14d872c5c302)"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// TestRewriteLinksFragmentLinkIsIdempotent는 조각이 붙은 링크를 다시 돌려도
// 안 바뀌는지 본다. /p/ 패턴이 조각을 경로에 붙여 읽으면 slug를 못 알아보고
// 미해결로 센다.
func TestRewriteLinksFragmentLinkIsIdempotent(t *testing.T) {
	slugs := SlugIndex(map[string]string{"8ef5464f-7a44-41c5-9850-f9804ff9cf2f": "vector-space"})
	set := slugSet("vector-space")
	body := "[벡터공간](/8ef5464f7a4441c59850f9804ff9cf2f#ad28eca3505f4c6599ed14d872c5c302)"

	once, _ := RewriteLinks(body, slugs, set)
	twice, res := RewriteLinks(once, slugs, set)

	if twice != once {
		t.Errorf("두 번째 실행에서 또 바뀌었다: %q → %q", once, twice)
	}
	if res.Replaced() != 0 || res.UnresolvedCount() != 0 {
		t.Errorf("두 번째 실행이 조용하지 않다: %+v", res)
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

// TestRewriteLinksIgnoresNonNotionPaths는 32자리 16진수가 아닌 경로를
// 건드리지 않는지 본다.
func TestRewriteLinksIgnoresNonNotionPaths(t *testing.T) {
	slugs := SlugIndex(map[string]string{"abc-123": "s"})
	body := "[홈](/) [소개](/about) [태그](/tags/go)"

	got, res := RewriteLinks(body, slugs, slugSet("s"))

	if got != body {
		t.Errorf("일반 경로가 바뀌었다:\ngot  %q\nwant %q", got, body)
	}
	if res.Replaced() != 0 {
		t.Errorf("교체가 일어났다: %d", res.Replaced())
	}
}
