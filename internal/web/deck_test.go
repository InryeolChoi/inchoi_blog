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

func TestBranchDeckUsesActualPostsInHumanOrder(t *testing.T) {
	// 돌아오는 순서는 정해져 있지 않다(store.PathBranches). 카드가 설
	// 순서는 사람이 정한 order가 정한다는 것을 뒤집힌 입력으로 확인한다.
	branches := []PathBranch{
		{Name: "Python", Slug: "python-root", Count: 28},
		{Name: "Rust", Slug: "rust-root", Count: 9},
		{Name: "C", Slug: "c-root", Count: 17},
	}
	cards := branchDeck(branches, languageOrder, languageArt)
	if len(cards) != 2 {
		t.Fatalf("카드가 %d장이다, 표에 있는 C와 Python 둘이어야 한다: %#v", len(cards), cards)
	}
	if cards[0].Name != "C" || cards[0].URL != "/p/c-root" || cards[0].Count != 17 {
		t.Errorf("C 카드가 원본 글을 가리키지 않는다: %#v", cards[0])
	}
	if cards[1].Name != "Python" || cards[1].URL != "/p/python-root" || cards[1].Count != 28 {
		t.Errorf("Python 카드가 원본 글을 가리키지 않는다: %#v", cards[1])
	}
	for _, card := range cards {
		if card.Name == "Rust" {
			t.Errorf("표에 없는 갈래가 설명 없는 카드로 섰다: %#v", card)
		}
		if !card.Native {
			t.Errorf("갈래 카드 %q가 포인터 JS 대상으로 남았다", card.Name)
		}
		if card.Blurb == "" || card.Icon == "" {
			t.Errorf("갈래 카드 %q의 설명이나 아이콘이 비었다", card.Name)
		}
	}
	if got := branchDeck(nil, languageOrder, languageArt); got != nil {
		t.Errorf("갈래가 하나도 없는데 카드 묶음을 만들었다: %#v", got)
	}
}

// TestBranchOrdersHaveArt는 카드 순서를 적어둔 표와 글자를 적어둔 표가
// 갈라지지 않는지 본다. order에만 있는 이름은 **조용히** 카드가 되지 않는다.
func TestBranchOrdersHaveArt(t *testing.T) {
	for _, deck := range []struct {
		name  string
		order []string
		art   map[string]cardArt
	}{
		{"language", languageOrder, languageArt},
		{"markup", markupOrder, markupArt},
	} {
		for _, name := range deck.order {
			if _, ok := deck.art[name]; !ok {
				t.Errorf("%s: %q가 순서표에만 있고 그림표에 없다", deck.name, name)
			}
		}
	}
}

// TestChildDeckSlugsHaveArt는 카드로 펼치는 분류의 하위 slug에 그림이 다 있는지 본다.
//
// **하나라도 비면 그 분류는 카드를 통째로 포기하고 목록으로 돌아간다**
// (childDeck.cards). 조용히 일어나는 일이라 화면을 열어보기 전에는 모른다.
// 갈래를 옮기거나 이름을 바꾸면 slug가 달라지므로 여기서 잡는다.
func TestChildDeckSlugsHaveArt(t *testing.T) {
	// deckSources는 slug만 알고 그 자식은 DB가 정한다. 카드가 걸린 분류의
	// 자식 slug를 여기 적어두고 그림이 있는지만 확인한다.
	want := map[string][]string{
		"dev":       {"language", "리눅스-쉘", "tooling", "서버-api", "클라이언트-ui"},
		"data-math": {"수리통계-이론", "수리통계-응용", "머신러닝"},
		"algorithm": {"알고리즘-이론", "알고리즘-실전"},
		"cs-theory": {"운영체제", "네트워크", "데이터베이스", "가상화기술"},
	}
	for parent, kids := range want {
		if _, ok := deckSources[parent]; !ok {
			t.Errorf("%s가 더 이상 카드 분류가 아니다", parent)
			continue
		}
		for _, slug := range kids {
			art, ok := cardArtBySlug[slug]
			if !ok || art.Blurb == "" || art.Icon == "" {
				t.Errorf("%s > %s에 카드 그림이 없다. 그러면 %s가 목록으로 돌아간다",
					parent, slug, parent)
			}
		}
	}
}
