package curation

import "fmt"

// GitHub 저장소에서 옮겨오는 글.
//
// # 노션 이관과 무엇이 다른가
//
// 노션은 **덤프가 고정**이라 변환 결과도 고정이었다. GitHub은 저장소가 계속
// 바뀌므로 "다시 돌리면 같은 결과"가 아니라 **"다시 돌리면 지금의 저장소와
// 같아진다"**가 목표다. 그래서 멱등 키가 `notion_page_id`가 아니라
// `source_ref`(출처 경로)다 — 파일이 그대로면 같은 글을 갱신한다.
//
// # 왜 파일 하나하나를 표에 적나
//
// 글머리 제목을 그대로 쓰면 "3. 람다식 써보기!"처럼 파일 안의 사정이 그대로
// 화면에 나오고, 파일 이름을 쓰면 "chapter3"이 된다. **둘 다 읽는 사람을 위한
// 이름이 아니다.** 몇 개 안 되므로 사람이 정해서 적는다 — 순서도 여기서 정한다.
//
// 표에 없는 파일은 **가져오지 않는다.** 저장소에 파일이 늘어도 조용히 딸려
// 들어오지 않고, 사람이 여기 적어야 들어온다.

// GitHubDoc은 옮겨올 파일 하나다.
type GitHubDoc struct {
	// Path는 저장소 안의 경로다. 이것이 멱등 키의 몸통이다.
	Path string
	// Title은 화면에 나갈 제목이다. 파일 안의 첫 제목을 쓰지 않는다.
	Title string
	// SortOrder는 형제 사이 순서다. 표에 적힌 것이 곧 사람이 정한 순서라
	// `sort_order_manual`이 켜진다(migrations/005).
	SortOrder int
	// Status는 이 글의 공개 상태다. 비면 "unlisted"다.
	Status string
}

// GitHubSource는 저장소 하나에서 옮겨오는 묶음이다.
type GitHubSource struct {
	// Repo는 `소유자/저장소`다.
	Repo string
	// Ref는 가져올 가지나 커밋이다. 비면 기본 가지다.
	Ref string
	// CategorySlug는 이 묶음이 붙을 분류다. **이미 있어야 한다** — 분류를
	// 만드는 것은 categorize·regroup의 일이고, 여기서 또 만들면 두 곳이
	// 같은 트리를 손대게 된다.
	CategorySlug string
	// OriginalPath는 `original_path`에 적을 값이다. 노션 경로 자리를 쓰는
	// 이유는 웹이 그 값으로 언어 갈래를 나누기 때문이다(web.LanguageBranches) —
	// 여기 `Language > 프로그래밍 언어 > Java`를 적어야 Java 갈래에 함께 선다.
	// 뒤에 글 제목이 붙는다.
	OriginalPath string
	// Docs는 옮겨올 파일들이다.
	Docs []GitHubDoc
}

// Ref는 이 글의 멱등 키다. `posts.source_ref`에 그대로 들어간다.
func (s GitHubSource) SourceRef(d GitHubDoc) string {
	return fmt.Sprintf("github:%s@%s", s.Repo, d.Path)
}

// GitHubSources는 옮겨오기로 정한 저장소다.
//
// 지금은 `모던 자바 인 액션` 정리 하나뿐이다. 블로그의 Java 글은 **기초**
// (자바의 정석 계열)라 겹치지 않는다 — 겹치는 `람다식`은 블로그 쪽이 본문
// 0바이트짜리 빈 자리이고, 이쪽이 8KB짜리 실제 글이다.
var GitHubSources = []GitHubSource{
	{
		Repo:         "InryeolChoi/Java_Modern",
		CategorySlug: "프로그래밍-언어",
		OriginalPath: "Language > 프로그래밍 언어 > Java > 모던 자바",
		Docs: []GitHubDoc{
			{Path: "src/main/theory/chapter1.md", Title: "모던 자바 1. 모던 자바란", SortOrder: 0},
			{Path: "src/main/theory/chapter2.md", Title: "모던 자바 2. 동작 파라미터화", SortOrder: 1},
			{Path: "src/main/theory/chapter3.md", Title: "모던 자바 3. 람다식", SortOrder: 2},
			{Path: "src/main/theory/chapter4.md", Title: "모던 자바 4. 스트림 소개", SortOrder: 3},
			{Path: "src/main/theory/chapter5.md", Title: "모던 자바 5. 스트림 활용", SortOrder: 4},
			{Path: "src/main/theory/chapter6.md", Title: "모던 자바 6. 스트림으로 데이터 수집", SortOrder: 5},
			{Path: "src/main/theory/chapter7.md", Title: "모던 자바 7. 병렬 데이터 처리와 성능", SortOrder: 6},
			{Path: "src/main/theory/chapter8.md", Title: "모던 자바 8. 컬렉션 API 개선", SortOrder: 7},
			{Path: "src/main/theory/chapter9.md", Title: "모던 자바 9. 리팩터링·테스팅·디버깅", SortOrder: 8},
			{Path: "src/main/theory/chapter11.md", Title: "모던 자바 11. null 대신 Optional", SortOrder: 9},
			{Path: "src/main/theory/chapter13.md", Title: "모던 자바 13. 디폴트 메서드", SortOrder: 10},
			{Path: "src/main/theory/chapter15.md", Title: "모던 자바 15. CompletableFuture와 리액티브의 기초", SortOrder: 11},
			{Path: "src/main/theory/chapter16.md", Title: "모던 자바 16. CompletableFuture : 비동기 조합", SortOrder: 12},
			{Path: "src/main/theory/chapter17.md", Title: "모던 자바 17. 리액티브 프로그래밍", SortOrder: 13},
			{Path: "src/main/theory/chapter18.md", Title: "모던 자바 18. 함수형 관점으로 생각하기", SortOrder: 14},
			{Path: "src/main/theory/chapter19.md", Title: "모던 자바 19. 함수형 프로그래밍 기법", SortOrder: 15},
		},
	},
	// 데이터베이스: `Database System Concepts` 정리. **이미지가 있는 첫 묶음이다** —
	// 그래서 importgh에 그림 처리를 더했다(cmd/importgh/images.go).
	{
		Repo:         "InryeolChoi/Database-System-Concepts",
		CategorySlug: "데이터베이스",
		OriginalPath: "데이터베이스",
		Docs: []GitHubDoc{
			{Path: "PART3/chapter8.md", Title: "복합 데이터 타입", SortOrder: 0},
		},
	},
}
