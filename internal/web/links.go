package web

import "html/template"

// 분류 화면 아래에 붙이는 바깥 링크다.
//
// **글 내용이 아니라 화면 장치라 코드에 둔다.** 표지 글 본문(DB)에 적으면
// `import -db`가 다음 이관에서 덮어써서 사라진다 — 갈래 카드(deck.go)를 코드에
// 두는 이유와 같다.
//
// 지금은 `소개` 하나다. 자기소개를 읽은 사람이 다음으로 갈 곳은 이 아카이브의
// 다른 분류가 아니라 개인 페이지라서, 본문 바로 아래에 그 길을 둔다.

// SiteLink는 카테고리 화면에 그릴 바깥 링크 한 줄이다.
// 본문 안의 외부 링크 카드(markdown/extlink.go)와 같은 모양을 쓴다.
type SiteLink struct {
	// Title은 카드 첫 줄이다. Host는 그 아래 작은 글씨로 어디로 가는지 알려준다.
	Title string
	Host  string
	URL   string
	// I18n은 preferences.js의 고정 사전 키다. 언어를 바꾸면 Title이 이 키로
	// 갈린다. 사전에 없는 키를 적으면 한국어 그대로 남는다.
	I18n string
	Icon template.HTML
}

// globeIcon은 사이드바의 Pages 링크와 같은 그림이다. 같은 곳으로 가는 길이
// 두 자리에 있으니 모양도 같아야 한 곳임을 안다.
var globeIcon = template.HTML(`<svg viewBox="0 0 16 16" aria-hidden="true">` +
	`<path d="M8 0a8 8 0 1 0 0 16A8 8 0 0 0 8 0zm5.9 7H11.6c-.1-1.8-.5-3.4-1.1-4.5A6.5 6.5 0 0 1 13.9 7zM8 1.5c.8 0 1.8 2 2 5.5H6c.2-3.5 1.2-5.5 2-5.5zM2.1 7a6.5 6.5 0 0 1 3.4-4.5C4.9 3.6 4.5 5.2 4.4 7H2.1zm0 2h2.3c.1 1.8.5 3.4 1.1 4.5A6.5 6.5 0 0 1 2.1 9zM8 14.5c-.8 0-1.8-2-2-5.5h4c-.2 3.5-1.2 5.5-2 5.5zm2.5-1a10.9 10.9 0 0 0 1.1-4.5h2.3a6.5 6.5 0 0 1-3.4 4.5z"/>` +
	`</svg>`)

// categoryLinks는 카테고리 slug → 그 화면에 붙일 바깥 링크다.
var categoryLinks = map[string][]SiteLink{
	"intro": {{
		Title: "최인렬의 개인 페이지",
		Host:  "inryeolchoi.github.io",
		URL:   "https://inryeolchoi.github.io",
		I18n:  "personalSite",
		Icon:  globeIcon,
	}},
}

// linksFor는 그 분류에 붙일 바깥 링크를 돌려준다. 없으면 nil이라 템플릿이
// 아무것도 그리지 않는다.
func linksFor(slug string) []SiteLink { return categoryLinks[slug] }
