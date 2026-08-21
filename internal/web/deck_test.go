package web

import "testing"

func TestProjectDeckUsesThreeDBBackedBranches(t *testing.T) {
	children := []Category{
		{Name: "Projects", Slug: "projects", PostCount: 10},
		{Name: "école 42", Slug: "école-42", PostCount: 324},
	}
	body := `# where42

[기본](/p/where-basic)
[View](/p/where-view)

# 심심조각

[AI](/p/piece-ai)
[다이어리](/p/piece-diary)
[리포트](/p/piece-report)
`

	cards := projectDeckFor("/project", children, body)
	if len(cards) != 3 {
		t.Fatalf("카드가 %d개다, 3개여야 한다", len(cards))
	}
	wants := []struct {
		name  string
		url   string
		count int
	}{
		{"école 42", "/project/%C3%A9cole-42", 324},
		{"where42", "/project/projects#where42", 2},
		{"심심조각", "/project/projects#심심조각", 3},
	}
	for i, want := range wants {
		got := cards[i]
		if got.Name != want.name || got.URL != want.url || got.Count != want.count {
			t.Errorf("카드 %d = %#v, want name=%q url=%q count=%d", i, got, want.name, want.url, want.count)
		}
		if got.Blurb == "" || got.Icon == "" {
			t.Errorf("카드 %q의 설명이나 아이콘이 비었다", got.Name)
		}
	}
}

func TestProjectDeckFallsBackWhenASectionIsMissing(t *testing.T) {
	children := []Category{{Slug: "projects"}, {Slug: "école-42"}}
	if got := projectDeckFor("/project", children, "# where42\n\n[글](/p/one)\n"); got != nil {
		t.Fatalf("심심조각 절이 없는데 카드 묶음을 만들었다: %#v", got)
	}
}

func TestLanguageDeckUsesActualLanguagePosts(t *testing.T) {
	children := []Category{
		{Name: "마크업 / 스타일링 / 표현식", Slug: "마크업-스타일링-표현식", PostCount: 12},
		{Name: "프로그래밍 언어", Slug: "프로그래밍-언어", PostCount: 191},
	}
	branches := []LanguageBranch{
		{Name: "C", Slug: "c-root", Count: 17},
		{Name: "Python", Slug: "python-root", Count: 28},
	}
	cards := languageDeckFor("/dev/language", children, branches)
	if len(cards) != 3 {
		t.Fatalf("Language 카드가 %d개다, 언어 2개와 마크업 1개여야 한다", len(cards))
	}
	if cards[0].Name != "C" || cards[0].URL != "/p/c-root" || cards[0].Count != 17 {
		t.Errorf("C 카드가 원본 글을 가리키지 않는다: %#v", cards[0])
	}
	if cards[1].Name != "Python" || cards[1].URL != "/p/python-root" || cards[1].Count != 28 {
		t.Errorf("Python 카드가 원본 글을 가리키지 않는다: %#v", cards[1])
	}
	if cards[2].Name == "프로그래밍 언어" {
		t.Errorf("프로그래밍 언어라는 뭉뚱그린 카드가 남았다: %#v", cards[2])
	}
	for _, card := range cards {
		if !card.Native {
			t.Errorf("Language 카드 %q가 포인터 JS 대상으로 남았다", card.Name)
		}
		if card.Blurb == "" || card.Icon == "" {
			t.Errorf("Language 카드 %q의 설명이나 아이콘이 비었다", card.Name)
		}
	}
}
