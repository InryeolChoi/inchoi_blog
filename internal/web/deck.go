package web

import (
	"html/template"
	"net/url"
)

// 분류 하나를 "아이콘 카드 묶음"으로 펼쳐 보여준다.
//
// 목록 한 줄짜리 링크로는 그 갈래가 무엇을 담고 있는지 알 수 없다. 갈래가 몇 개
// 안 되고 성격이 뚜렷한 자리에서는 카드가 낫다. **아무 데나 쓰지 않는다** — 지금은
// `데이터 & 수리` 한 곳뿐이고, 여기서 써보고 넓힐지 정한다.
//
// **글 내용이 아니라 화면 장치다.** 그래서 표지 글 본문(DB)이 아니라 여기 코드에
// 둔다. 본문에 넣으면 `import -db`가 덮어써서 사라진다.

// deckCategories는 카드로 펼칠 분류의 slug다. 그 분류의 **하위 분류**가 카드가 된다.
var deckCategories = map[string]bool{
	"data-math": true,
}

// cardArt는 카드 하나의 그림과 한 줄 설명이다. 키는 하위 분류의 slug다.
type cardArt struct {
	Blurb string
	Icon  template.HTML
}

// 아이콘은 인라인 SVG다. 파일로 두지 않는 이유: 카드마다 하나씩이라 요청을 늘릴
// 값어치가 없고, currentColor를 쓰면 색을 카드가 정한다.
var cardArtBySlug = map[string]cardArt{
	"수리통계-이론": {
		Blurb: "정의에서 시작해 증명으로 이어지는 것들. 선형대수와 최적화, 확률과 수리통계.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<path d="M8 40V8" /><path d="M8 40h32" />
			<path d="M8 34c8 0 10-20 16-20s8 14 16 14" />
			<circle cx="24" cy="14" r="2.5" fill="currentColor" stroke="none"/>
		</svg>`),
	},
	"수리통계-응용": {
		Blurb: "데이터를 앞에 두고 하는 일. 탐색과 회귀, 다변량 분석, 그리고 자격증 정리.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<path d="M8 40V8" /><path d="M8 40h32" />
			<rect x="14" y="24" width="6" height="12" rx="1"/>
			<rect x="24" y="16" width="6" height="20" rx="1"/>
			<rect x="34" y="28" width="6" height="8" rx="1"/>
		</svg>`),
	},
	"머신러닝": {
		Blurb: "모델을 학습시키는 쪽. 핸즈온으로 훑은 기초 이론과 자연어처리.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<circle cx="10" cy="24" r="3.5"/>
			<circle cx="26" cy="13" r="3.5"/><circle cx="26" cy="35" r="3.5"/>
			<circle cx="40" cy="24" r="3.5"/>
			<path d="M13 22l10-7M13 26l10 7M29 15l9 7M29 33l9-7"/>
		</svg>`),
	},
}

// DeckCard는 템플릿이 그릴 카드 하나다.
type DeckCard struct {
	Name  string
	URL   string
	Count int
	Blurb string
	Icon  template.HTML
}

// deckFor는 카드로 펼칠 분류면 카드 목록을, 아니면 nil을 돌려준다.
//
// **그림이 없는 하위 분류가 하나라도 있으면 통째로 포기한다.** 반은 카드고 반은
// 목록이면 어느 쪽도 아니게 된다. 하위 분류가 늘면 cardArtBySlug에 그림을
// 더해주면 된다.
func deckFor(slug, basePath string, children []Category) []DeckCard {
	if !deckCategories[slug] || len(children) == 0 {
		return nil
	}
	cards := make([]DeckCard, 0, len(children))
	for _, c := range children {
		art, ok := cardArtBySlug[c.Slug]
		if !ok {
			return nil
		}
		cards = append(cards, DeckCard{
			Name:  c.Name,
			URL:   basePath + "/" + url.PathEscape(c.Slug),
			Count: c.PostCount,
			Blurb: art.Blurb,
			Icon:  art.Icon,
		})
	}
	return cards
}
