package web

import (
	"html/template"
	"net/url"
	"strings"
)

// 분류 하나를 "아이콘 카드 묶음"으로 펼쳐 보여준다.
//
// 목록 한 줄짜리 링크로는 그 갈래가 무엇을 담고 있는지 알 수 없다. 갈래가 몇 개
// 안 되고 성격이 뚜렷한 자리에서는 카드가 낫다. **아무 데나 쓰지 않는다** — 지금은
// `데이터 & 수리`에서 시작해 갈래가 분명한 최상위 분류로 넓혔다.
//
// **글 내용이 아니라 화면 장치다.** 그래서 표지 글 본문(DB)이 아니라 여기 코드에
// 둔다. 본문에 넣으면 `import -db`가 덮어써서 사라진다.

const (
	projectSlug  = "project"
	languageSlug = "language"
)

// deckSource는 한 분류의 카드 묶음을 만드는 방법이다.
//
// 카드가 필요한 분류마다 재료가 다르다. 대부분은 하위 분류만 있으면 되지만,
// Language는 원본 경로에서 다시 묶은 언어 갈래가, 프로젝트는 표지 글 본문이
// 더 있어야 한다. 그 차이를 핸들러의 if 문으로 늘어놓는 대신 방법을 분류마다
// 하나씩 두고 slug로 찾는다. 새 카드 묶음은 아래 표에 한 줄이면 된다.
//
// 카드를 만들 수 없으면 error가 아니라 nil을 돌려준다. 그러면 화면이 평소
// 목록으로 돌아간다 — 반쪽짜리 카드 묶음보다 그쪽이 길을 잃지 않는다.
// error는 DB 조회가 실패했을 때만이다.
type deckSource interface {
	cards(s *Server, basePath string, children []Category) ([]DeckCard, error)
}

// deckSources는 카드로 펼칠 분류의 slug다. **아무 데나 쓰지 않는다** —
// 갈래가 몇 개 안 되고 성격이 뚜렷한 자리에서만 목록보다 카드가 낫다.
var deckSources = map[string]deckSource{
	"data-math":  childDeck{},
	"algorithm":  childDeck{},
	"cs-theory":  childDeck{},
	"dev":        childDeck{},
	languageSlug: languageDeck{},
	projectSlug:  projectDeck{},
}

// deckFor는 카드로 펼칠 분류면 카드 목록을, 아니면 nil을 돌려준다.
func (s *Server) deckFor(slug, basePath string, children []Category) ([]DeckCard, error) {
	source, ok := deckSources[slug]
	if !ok || len(children) == 0 {
		return nil, nil
	}
	return source.cards(s, basePath, children)
}

// childBySlug는 하위 분류 하나를 slug로 찾는다. 카드 묶음마다 "이 분류가
// 있어야 성립한다"는 재료가 있어서 세 곳이 같은 일을 한다.
func childBySlug(children []Category, slug string) (Category, bool) {
	for _, child := range children {
		if child.Slug == slug {
			return child, true
		}
	}
	return Category{}, false
}

// cardArt는 카드 하나의 그림과 한 줄 설명이다. 키는 하위 분류의 slug다.
type cardArt struct {
	Blurb string
	Icon  template.HTML
}

var languageBlurbs = map[string]string{
	"C":          "메모리와 포인터를 직접 다루며 프로그램의 바닥을 익힌 기록.",
	"C++":        "객체와 템플릿, 표준 라이브러리로 C 위에 구조를 세운 기록.",
	"Java":       "객체지향 문법부터 컬렉션과 JVM 생태계까지.",
	"Python":     "간결한 문법과 데이터 처리, 자동화에 쓴 파이썬 기록.",
	"R":          "통계 계산과 데이터 분석을 중심으로 정리한 R 기록.",
	"TypeScript": "자바스크립트에 타입을 더해 웹 코드를 단단하게 만든 기록.",
}

var languageIcon = template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
	<path d="M17 14L7 24l10 10M31 14l10 10-10 10M27 10l-6 28"/>
</svg>`)

// languageDeckFor는 `프로그래밍 언어` 한 장짜리 카드를 원본 경로에 남은
// C·Java·Python 같은 실제 언어 갈래로 바꾼다. 마크업 분류는 별도 카드로
// 남겨 Language 안의 다른 콘텐츠로 가는 길도 잃지 않는다.
func languageDeckFor(basePath string, children []Category, branches []LanguageBranch) []DeckCard {
	if len(branches) == 0 {
		return nil
	}
	cards := make([]DeckCard, 0, len(branches)+1)
	for _, branch := range branches {
		blurb, ok := languageBlurbs[branch.Name]
		if !ok {
			continue
		}
		cards = append(cards, DeckCard{
			Name: branch.Name, URL: "/p/" + url.PathEscape(branch.Slug), Count: branch.Count,
			Blurb: blurb, Icon: languageIcon, Native: true,
		})
	}
	if child, ok := childBySlug(children, "마크업-스타일링-표현식"); ok {
		art := cardArtBySlug[child.Slug]
		cards = append(cards, DeckCard{
			Name: child.Name, URL: basePath + "/" + url.PathEscape(child.Slug), Count: child.PostCount,
			Blurb: art.Blurb, Icon: art.Icon, Native: true,
		})
	}
	return cards
}

// 아이콘은 인라인 SVG다. 파일로 두지 않는 이유: 카드마다 하나씩이라 요청을 늘릴
// 값어치가 없고, currentColor를 쓰면 색을 카드가 정한다.
var cardArtBySlug = map[string]cardArt{
	"project-école-42": {
		Blurb: "42 서울에서 통과한 과제와 시험, 구현하며 남긴 기록.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<path d="M8 12h13v12H8zM27 12h13v24H27zM8 30h13v6H8z"/>
			<path d="M21 18h6M14 24v6"/>
		</svg>`),
	},
	"project-where42": {
		Blurb: "카뎃 활동을 한눈에 보는 서비스. 화면 구조를 따라가며 남긴 코드 분석.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<path d="M9 34c4-8 8-12 15-12s11 4 15 12"/>
			<circle cx="24" cy="14" r="6"/>
			<path d="M8 39h32M34 10l6 4-6 4"/>
		</svg>`),
	},
	"project-심심조각": {
		Blurb: "AI, 다이어리, 리포트까지. 기록을 조각처럼 모으는 서비스의 코드 분석.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<path d="M10 10h12v12H10zM26 10h12v12H26zM10 26h12v12H10z"/>
			<path d="M32 27v10M27 32h10"/>
		</svg>`),
	},
	"프로그래밍-언어": {
		Blurb: "R, Python, Java, C, C++, TypeScript, Swift. 언어마다의 문법과 버릇.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<path d="M17 14L7 24l10 10M31 14l10 10-10 10M27 10l-6 28"/>
		</svg>`),
	},
	"마크업-스타일링-표현식": {
		Blurb: "HTML과 CSS, 정규식과 표현식. 구조를 적고 모양과 패턴을 다루는 법.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<path d="M8 12h32v24H8zM8 19h32"/>
			<path d="M14 27l4 3-4 3M23 33h10"/>
		</svg>`),
	},
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
	"알고리즘-이론": {
		Blurb: "정렬과 탐색, 그래프와 동적계획법이 왜 그렇게 도는지.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<circle cx="24" cy="10" r="4"/><circle cx="12" cy="30" r="4"/><circle cx="36" cy="30" r="4"/>
			<circle cx="24" cy="42" r="3"/>
			<path d="M21 13l-6 13M27 13l6 13M14 33l8 7M34 33l-8 7"/>
		</svg>`),
	},
	"알고리즘-실전": {
		Blurb: "백준과 프로그래머스에서 실제로 푼 것들.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<rect x="7" y="9" width="34" height="30" rx="3"/>
			<path d="M14 20l5 4-5 4"/><path d="M24 28h10"/>
		</svg>`),
	},
	"운영체제": {
		Blurb: "프로세스와 메모리, 파일 시스템. 컴퓨터가 자기를 굴리는 방법.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<rect x="14" y="14" width="20" height="20" rx="3"/>
			<rect x="20" y="20" width="8" height="8" rx="1"/>
			<path d="M20 8v6M28 8v6M20 34v6M28 34v6M8 20h6M8 28h6M34 20h6M34 28h6"/>
		</svg>`),
	},
	"네트워크": {
		Blurb: "계층과 프로토콜. 패킷이 어디를 거쳐 어떻게 도착하는지.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<circle cx="24" cy="24" r="15"/>
			<path d="M9 24h30"/><path d="M24 9c5 5 5 25 0 30M24 9c-5 5-5 25 0 30"/>
		</svg>`),
	},
	"데이터베이스": {
		Blurb: "SQL과 설계, 인덱싱과 트랜잭션.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<ellipse cx="24" cy="12" rx="14" ry="5"/>
			<path d="M10 12v10c0 2.8 6.3 5 14 5s14-2.2 14-5V12"/>
			<path d="M10 22v10c0 2.8 6.3 5 14 5s14-2.2 14-5V22"/>
		</svg>`),
	},
	"컴퓨터-시스템": {
		Blurb: "비트에서 프로그램까지. 하드웨어에 가까운 쪽.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<rect x="9" y="12" width="30" height="24" rx="3"/>
			<path d="M9 20h8v8H9M39 20h-8M31 28h8"/>
			<circle cx="24" cy="24" r="3"/>
		</svg>`),
	},
	"가상화기술": {
		Blurb: "도커와 컨테이너. 격리된 환경을 만들어 쓰는 일.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<rect x="8" y="26" width="10" height="10" rx="1.5"/>
			<rect x="20" y="26" width="10" height="10" rx="1.5"/>
			<rect x="20" y="14" width="10" height="10" rx="1.5"/>
			<path d="M32 31h8"/>
		</svg>`),
	},
	"language": {
		Blurb: "C부터 파이썬까지. 언어마다의 문법과 버릇.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<path d="M17 14L7 24l10 10"/><path d="M31 14l10 10-10 10"/>
			<path d="M27 10l-6 28"/>
		</svg>`),
	},
	"웹-프로그래밍": {
		Blurb: "브라우저와 서버 사이. 프런트와 백엔드 양쪽.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<rect x="7" y="10" width="34" height="28" rx="3"/>
			<path d="M7 19h34"/>
			<circle cx="13" cy="14.5" r="1.4" fill="currentColor" stroke="none"/>
			<circle cx="18" cy="14.5" r="1.4" fill="currentColor" stroke="none"/>
		</svg>`),
	},
	"리눅스-쉘": {
		Blurb: "명령줄에서 하는 일. 스크립트와 시스템 관리.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<rect x="7" y="10" width="34" height="28" rx="3"/>
			<path d="M14 20l6 5-6 5"/><path d="M24 30h10"/>
		</svg>`),
	},
	"모바일-프로그래밍": {
		Blurb: "손에 들고 쓰는 화면을 만드는 쪽.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<rect x="15" y="6" width="18" height="36" rx="4"/>
			<path d="M21 11h6"/><circle cx="24" cy="36" r="1.8" fill="currentColor" stroke="none"/>
		</svg>`),
	},
	"tooling": {
		Blurb: "git과 개발 도구. 코드 밖에서 손이 가는 것들.",
		Icon: template.HTML(`<svg viewBox="0 0 48 48" aria-hidden="true">
			<path d="M31 9a9 9 0 0 0-11 11L9 31a3.5 3.5 0 0 0 5 5l11-11A9 9 0 0 0 36 14l-5 5-4-4 5-5z"/>
		</svg>`),
	},
}

// DeckCard는 템플릿이 그릴 카드 하나다.
type DeckCard struct {
	Name   string
	URL    string
	Count  int
	Blurb  string
	Icon   template.HTML
	Native bool // JS 기울기 없이 링크와 CSS만으로 동작하는 카드
}

// childDeck은 하위 분류를 그대로 한 장씩 펴는 기본 방식이다.
//
// **그림이 없는 하위 분류가 하나라도 있으면 통째로 포기한다.** 반은 카드고 반은
// 목록이면 어느 쪽도 아니게 된다. 하위 분류가 늘면 cardArtBySlug에 그림을
// 더해주면 된다.
type childDeck struct{}

func (childDeck) cards(_ *Server, basePath string, children []Category) ([]DeckCard, error) {
	cards := make([]DeckCard, 0, len(children))
	for _, c := range children {
		art, ok := cardArtBySlug[c.Slug]
		if !ok {
			return nil, nil
		}
		cards = append(cards, DeckCard{
			Name:   c.Name,
			URL:    basePath + "/" + url.PathEscape(c.Slug),
			Count:  c.PostCount,
			Blurb:  art.Blurb,
			Icon:   art.Icon,
			Native: false,
		})
	}
	return cards, nil
}

// languageDeck은 평평한 `프로그래밍 언어` 한 장을 실제 언어 갈래로 바꾼다.
// 그 하위 분류가 없으면 카드를 만들지 않는다.
type languageDeck struct{}

func (languageDeck) cards(s *Server, basePath string, children []Category) ([]DeckCard, error) {
	child, ok := childBySlug(children, "프로그래밍-언어")
	if !ok {
		return nil, nil
	}
	branches, err := s.store.LanguageBranches(child.ID)
	if err != nil {
		return nil, err
	}
	return languageDeckFor(basePath, children, branches), nil
}

// projectDeck은 `Projects` 표지 글 본문에서 where42·심심조각 두 절을 읽어야
// 성립한다. 그 분류나 표지 글이 없으면 카드를 만들지 않는다.
type projectDeck struct{}

func (projectDeck) cards(s *Server, basePath string, children []Category) ([]DeckCard, error) {
	child, ok := childBySlug(children, "projects")
	if !ok || child.CoverPostSlug == "" {
		return nil, nil
	}
	cover, err := s.store.PostBySlug(child.CoverPostSlug)
	if err != nil {
		return nil, err
	}
	if cover == nil {
		return nil, nil
	}
	return projectDeckFor(basePath, children, cover.Body), nil
}

// projectDeckFor는 프로젝트 첫 화면을 세 갈래로 정리한다.
//
// école 42는 이미 하위 카테고리라 그쪽으로 바로 보낸다. where42와 심심조각은
// 노션의 `Projects` 표지 글 한 편 안에 두 절로 함께 들어 있으므로, DB 본문을
// 다시 만들지 않고 그 절의 앵커로 보낸다. 링크 수도 같은 본문에서 세어 코드의
// 숫자가 정본보다 앞서거나 뒤처지지 않게 한다.
//
// 필요한 분류나 두 절이 없으면 nil이다. 반쪽짜리 카드 묶음 대신 평소 목록으로
// 돌아가는 편이 탐색 경로를 잃지 않는다.
func projectDeckFor(basePath string, children []Category, projectsBody string) []DeckCard {
	ecole, hasEcole := childBySlug(children, "école-42")
	projects, hasProjects := childBySlug(children, "projects")
	whereCount, pieceCount := projectSectionCounts(projectsBody)
	if !hasEcole || !hasProjects || whereCount == 0 || pieceCount == 0 {
		return nil
	}

	makeCard := func(name, target string, count int, artKey string) DeckCard {
		art := cardArtBySlug[artKey]
		return DeckCard{Name: name, URL: target, Count: count, Blurb: art.Blurb, Icon: art.Icon}
	}
	projectsURL := basePath + "/" + url.PathEscape(projects.Slug)
	return []DeckCard{
		makeCard("école 42", basePath+"/"+url.PathEscape(ecole.Slug), ecole.PostCount, "project-école-42"),
		makeCard("where42", projectsURL+"#where42", whereCount, "project-where42"),
		makeCard("심심조각", projectsURL+"#심심조각", pieceCount, "project-심심조각"),
	}
}

// projectSectionCounts는 Projects 표지 글의 두 최상위 절에서 글 링크를 센다.
// 변환된 HTML이 아니라 DB의 마크다운을 보는 이유는 이 숫자 역시 DB를 따라야
// 하기 때문이다. 제목 단계는 중요하지 않고 제목 글자만 정확히 맞춘다.
func projectSectionCounts(body string) (where42, piece int) {
	section := ""
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			switch {
			case strings.EqualFold(heading, "where42"):
				section = "where42"
			case heading == "심심조각":
				section = "심심조각"
			default:
				section = ""
			}
			continue
		}
		links := strings.Count(line, "](/p/")
		switch section {
		case "where42":
			where42 += links
		case "심심조각":
			piece += links
		}
	}
	return where42, piece
}
