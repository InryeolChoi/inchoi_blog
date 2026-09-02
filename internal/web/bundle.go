package web

import (
	"net/url"
	"strings"
)

// 하위 분류와 직속 글을 **사람이 정한 묶음**으로 다시 세운다.
//
// # 왜 필요한가
//
// école 42 화면은 같은 것을 두 목록으로 보여주고 있었다. 과제 열일곱 개는
// 글이 여러 편이라 `하위 분류`가 됐고, 글이 한 편뿐인 넷(exam04·exam05·
// push_swap·ft_transcendence)은 `글`로 남았다. 그래서 **exam02·exam03은 위
// 목록에, exam04·exam05는 아래 목록에** 있었다 — 한 시리즈가 두 상자로
// 갈린 것이라, 읽는 사람은 그 차이를 뜻으로 읽는다(위는 뭐고 아래는 뭔가?).
//
// 실제 차이는 "글이 여러 편이냐 한 편이냐"뿐인데 그건 읽는 사람이 알 바가
// 아니다. 알고 싶은 것은 **시험이냐 프로젝트냐**다.
//
// # 왜 코드에 두나
//
// 갈래 카드·외부 링크·`deferredSections`와 같은 이유다. 표지 본문(DB)에 적으면
// 다음 `import -db`가 덮어써서 사라지고, `internal/curation`에 적으면 웹이 그
// 패키지를 읽게 되어 "DB가 정본"이 깨진다. 이건 화면 장치다.
//
// # 못 만들면 nil이다
//
// 상자 하나가 비면 통째로 포기하고 평소 목록으로 돌아간다. 반쪽짜리 묶음보다
// 예전 목록이 낫다 — 갈래 카드가 그림 없는 분류에서 통째로 물러나는 것과 같다.

// Bundle은 이름표를 단 상자 하나다. 인라인 데이터베이스를 펼친 목록·`하위 분류`와
// 같은 머리띠 상자로 그린다.
type Bundle struct {
	Name  string
	Items []BundleItem
}

// BundleItem은 상자 안의 한 줄이다. 분류로 가는 줄과 글로 가는 줄이 섞인다.
type BundleItem struct {
	Name string
	URL  string
	// Count는 분류일 때의 글 수다. 글 한 편으로 가는 줄에는 셀 것이 없어서 0이고,
	// 템플릿이 0이면 안 찍는다.
	Count int
}

// bundleSource는 한 분류의 묶음을 만드는 방법이다.
type bundleSource func(children []Category, posts []PostSummary, basePath string) []Bundle

// bundleSources는 slug마다 하나씩이다. 여기 없는 분류는 예전 목록 그대로다.
var bundleSources = map[string]bundleSource{
	"école-42": ecoleBundles,
}

func bundlesFor(slug string, children []Category, posts []PostSummary, basePath string) []Bundle {
	make, ok := bundleSources[slug]
	if !ok {
		return nil
	}
	return make(children, posts, basePath)
}

// ecoleBundles는 école 42의 과제를 `Exam`과 `프로젝트`로 가른다.
//
// **이름으로 가른다.** 42의 시험 글은 `exam02`~`exam06`으로 번호까지 규칙적이라
// 여기서 표를 따로 둘 값이 없다. 규칙이 안 맞는 이름이 생기면 프로젝트 쪽으로
// 가므로, 조용히 사라지지 않는다.
func ecoleBundles(children []Category, posts []PostSummary, basePath string) []Bundle {
	exam := Bundle{Name: "Exam"}
	proj := Bundle{Name: "프로젝트"}
	put := func(name string, item BundleItem) {
		if isExamName(name) {
			exam.Items = append(exam.Items, item)
			return
		}
		proj.Items = append(proj.Items, item)
	}

	for _, c := range children {
		put(c.Name, BundleItem{
			Name:  c.Name,
			URL:   basePath + "/" + url.PathEscape(c.Slug),
			Count: c.PostCount,
		})
	}
	// **글은 평평하게 넣는다.** 여기 남은 것은 하위 분류가 못 된 한 편짜리라
	// 자식이 없다. 중첩이 생기면 그건 이 묶음이 다룰 모양이 아니므로 포기한다.
	for _, p := range posts {
		if len(p.Children) > 0 {
			return nil
		}
		put(p.Title, BundleItem{Name: p.Title, URL: "/p/" + url.PathEscape(p.Slug)})
	}

	// 한쪽이 비면 가른 뜻이 없다.
	if len(exam.Items) == 0 || len(proj.Items) == 0 {
		return nil
	}
	return []Bundle{proj, exam}
}

// isExamName은 `exam02`처럼 시험 글의 이름인지 본다. 앞에서 `exam`으로 시작하고
// 그 뒤가 숫자뿐일 때만이다 — `example`이나 `exam 정리`가 딸려 들어가지 않는다.
func isExamName(name string) bool {
	rest, ok := strings.CutPrefix(strings.ToLower(strings.TrimSpace(name)), "exam")
	if !ok || rest == "" {
		return false
	}
	for _, r := range rest {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
