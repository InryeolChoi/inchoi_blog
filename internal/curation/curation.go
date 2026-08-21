// Package curation은 사람이 손으로 정한 분류 결정을 담는다.
//
// cmd/categorize는 original_path가 알려주는 것만 안다. 노션에서 어디에 뒀느냐와
// 블로그에서 어디에 두고 싶으냐가 다를 때가 있고, 그 차이를 여기 적어둔다.
//
// 두 도구가 같은 표를 봐야 한다:
//   - cmd/regroup이 이대로 DB를 고친다.
//   - cmd/categorize는 여기 있는 것을 자기 계획과 다르다고 트집잡지 않는다.
//     (categorize는 이미 있는 카테고리의 parent_id를 덮지 않지만, 검증은 따로다.)
//
// 표에서 빼면 다음 categorize 실행이 "경로와 다르다"고 실패한다. 되돌리려면
// regroup으로 제자리에 옮긴 뒤에 뺀다.
package curation

import (
	"fmt"
	"strings"
)

// Move는 노션 2단계 카테고리를 경로가 정한 곳이 아닌 다른 분류 밑으로 옮긴 것이다.
type Move struct {
	// SourceName은 옮길 카테고리의 source_name이다.
	// 경로에서 온 이름이라 사람이 이름을 바꿔도 그대로다.
	SourceName string
	// ToSlug는 새 부모 카테고리의 slug다. 사람이 만든 최상위나 중간 분류를 가리킨다.
	ToSlug string
	Why    string
}

// 아래 열둘은 노션 최상위 "수학 & 통계" 밑에 한 덩어리로 있던 것을 사람이
// 이론·응용·머신러닝 세 갈래로 가른 결과다. 갈래는 노션에 없는 층이라
// `ToSlug`가 최상위가 아니라 **사람이 둔 중간 층**을 가리킨다.
//
// regroup이 이대로 옮기고, categorize는 이걸 "경로와 다르다"고 트집잡지 않는다.
var Moves = []Move{
	{SourceName: "선형대수", ToSlug: "수리통계-이론", Why: "데이터 & 수리를 세 갈래로 가르면서 옮겼다"},
	{SourceName: "최적화이론", ToSlug: "수리통계-이론", Why: "데이터 & 수리를 세 갈래로 가르면서 옮겼다"},
	{SourceName: "수리통계1", ToSlug: "수리통계-이론", Why: "데이터 & 수리를 세 갈래로 가르면서 옮겼다"},
	{SourceName: "수리통계2", ToSlug: "수리통계-이론", Why: "데이터 & 수리를 세 갈래로 가르면서 옮겼다"},
	{SourceName: "확률과정론", ToSlug: "수리통계-이론", Why: "데이터 & 수리를 세 갈래로 가르면서 옮겼다"},
	{SourceName: "이산수학", ToSlug: "수리통계-이론", Why: "데이터 & 수리를 세 갈래로 가르면서 옮겼다"},
	{SourceName: "탐색적 자료분석", ToSlug: "수리통계-응용", Why: "데이터 & 수리를 세 갈래로 가르면서 옮겼다"},
	{SourceName: "회귀분석", ToSlug: "수리통계-응용", Why: "데이터 & 수리를 세 갈래로 가르면서 옮겼다"},
	{SourceName: "다변량분석", ToSlug: "수리통계-응용", Why: "데이터 & 수리를 세 갈래로 가르면서 옮겼다"},
	{SourceName: "빅데이터 분석기사", ToSlug: "career", Why: "자격증 학습 기록이라 수리/통계보다 커리어에 두기로 했다"},
	{SourceName: "핸즈온 머신러닝 2", ToSlug: "머신러닝", Why: "데이터 & 수리를 세 갈래로 가르면서 옮겼다"},
	{SourceName: "자연어처리 (1) : BERT와 GPT", ToSlug: "머신러닝", Why: "데이터 & 수리를 세 갈래로 가르면서 옮겼다"},
}

// PostMove는 글 하나를 경로가 정한 곳이 아닌 다른 카테고리에 붙이는 것이다.
//
// categorize는 이걸 경로 처리보다 먼저 본다. 그래서 옮긴 글의 경로는 카테고리를
// 만드는 데 아예 쓰이지 않는다. 어떤 카테고리의 글이 전부 옮겨지면 그 카테고리는
// 계획에서 사라지고, DropCategories가 남은 빈 행을 지운다.
type PostMove struct {
	// NotionPageID는 옮길 글이다. posts의 멱등 키다.
	NotionPageID string
	// ToSlug는 붙일 카테고리의 slug다. 어느 층이든 상관없다.
	ToSlug string
	Title  string // 사람이 읽으라고 적어두는 것. 대조에 쓰지 않는다.
}

// PostMoves는 사람이 정한 글 이동이다.
//
// 아래 6건은 노션에서 자기소개 페이지 밑 "프로젝트" 목록에 있던 껍데기다.
// 본문이 0바이트고, 실제 내용은 école 42 밑의 별도 글에 있다. 그래도 지우지
// 않는다(이 프로젝트는 글을 지우지 않는다). 과제 목록 링크 모음 성격이라
// 프로젝트 최상위에 직접 붙인다.
var PostMoves = []PostMove{
	{NotionPageID: "f4847474-a809-47a4-a9a7-7db997b66bf1", ToSlug: "project", Title: "FT_IRC"},
	{NotionPageID: "ddd3c9db-d3e4-40f7-8878-8da7f5c9d1fd", ToSlug: "project", Title: "Inception"},
	{NotionPageID: "1080901b-87f1-80d6-82e2-c81d4d9b4401", ToSlug: "project", Title: "MiniShell"},
	{NotionPageID: "1080901b-87f1-800c-b1d5-f98d79010552", ToSlug: "project", Title: "PhiloSopher"},
	{NotionPageID: "5fd73f8a-648d-4b15-88be-c3612e0c3262", ToSlug: "project", Title: "Where42"},
	{NotionPageID: "e210fa33-6030-430e-8a7b-58d1026b1ba7", ToSlug: "project", Title: "심심조각"},

	// 노션에서 `최적화이론 > 수업 : 통수 & 선계`에 있던 글이다. 그 수업이 통계수학과
	// 선형계획을 같이 다뤄서 선형대수 내용까지 최적화이론 밑에 들어가 있었다.
	// 최적화(라그랑주·심플렉스·쌍대정리)는 그대로 두고 선형대수만 옮긴다.
	//
	// 선형대수 카테고리에는 같은 제목의 글이 이미 있지만 **전부 본문이 0바이트인
	// 껍데기다**(노션에서 목차만 만들고 내용을 안 쓴 것). 내용이 있는 쪽이 이쪽이라
	// 옮기면 그 분류가 비로소 알맹이를 갖는다. 껍데기는 지우지 않고 그대로 둔다 —
	// 이 프로젝트는 글을 지우지 않는다.
	{NotionPageID: "2cb48833-6e6e-4b48-9f74-2855ef3c63db", ToSlug: "선형대수", Title: "고유값과 대각화"},
	{NotionPageID: "c520daf4-6db6-44cd-9767-d0f0c560caa2", ToSlug: "선형대수", Title: "내적공간"},
	{NotionPageID: "f69e826a-2112-4dbe-861d-8b61e0c136a5", ToSlug: "선형대수", Title: "벡터"},
	{NotionPageID: "8ef5464f-7a44-41c5-9850-f9804ff9cf2f", ToSlug: "선형대수", Title: "벡터공간"},
	{NotionPageID: "2275909f-ed86-40c7-ba47-198768145ded", ToSlug: "선형대수", Title: "선형변환"},
	{NotionPageID: "f77c0e4e-dd54-4a4a-9534-e9f8ca1846a8", ToSlug: "선형대수", Title: "특이값 분해"},
	{NotionPageID: "c76070b0-c71d-45ba-82d7-a542f7dbab89", ToSlug: "선형대수", Title: "피봇연산"},
	{NotionPageID: "5ef93000-4852-4af7-84cf-37c78b9b3774", ToSlug: "선형대수", Title: "피봇연산과 대각화"},
	{NotionPageID: "9e966d0e-e821-46e5-b5d4-64aae905748d", ToSlug: "선형대수", Title: "피봇연산과 역행렬"},
	{NotionPageID: "7555c053-d6d0-404c-9282-43b737a4c063", ToSlug: "선형대수", Title: "행렬"},
	{NotionPageID: "4e2f7f05-8d33-46bb-8285-833973dd7747", ToSlug: "선형대수", Title: "행렬식과 역행렬"},

	// 아래 열다섯 건은 노션 2단계 페이지의 **목차 글**이다. 사람이 "데이터 & 수리"를
	// 세 갈래로 가르면서 이 글들이 붙어 있던 중간층(`수학 & 통계`,
	// `머신러닝 & 딥러닝`)이 없어졌다. 각자 제 분류로 내려보낸다 — 분류를 누르면
	// 목록이 아니라 그 소개가 먼저 펴지게 하려는 것이다(Covers 참고).
	//
	// `데이터분석 기초 : R언어`와 `파이썬`은 대응하는 분류가 없어서 응용 갈래에
	// 직접 붙인다.
	{NotionPageID: "5d2a5e48-0d85-4fc5-94d3-280cbeef87ee", ToSlug: "data-math", Title: "수학 & 통계"},
	{NotionPageID: "ad1ef256-4567-4b9f-b57e-6f16486d0606", ToSlug: "선형대수", Title: "선형대수"},
	{NotionPageID: "660e3d79-427d-40f7-b98a-6f8be0a5f787", ToSlug: "최적화이론", Title: "최적화이론"},
	{NotionPageID: "0b6791d6-adfe-48d8-a91d-54e620d708c3", ToSlug: "이산수학", Title: "이산수학"},
	{NotionPageID: "7032b31f-d9ff-4365-83dc-b77ad8382d24", ToSlug: "수리통계1", Title: "수리통계1"},
	{NotionPageID: "59d18904-4ed5-4f0a-ba61-17ec86d9fc7b", ToSlug: "수리통계2", Title: "수리통계2"},
	{NotionPageID: "3280fbb3-019d-47dd-bc94-f1e433c99efc", ToSlug: "확률과정론", Title: "확률과정론"},
	{NotionPageID: "ab031d71-5474-4a03-bc3f-124391842de7", ToSlug: "탐색적-자료분석", Title: "탐색적 자료분석"},
	{NotionPageID: "7bb6bd36-9ad9-4ef8-8b2f-827080fad9f3", ToSlug: "회귀분석", Title: "회귀분석"},
	{NotionPageID: "cb02b6b8-6f29-479d-a791-25f1cd748b78", ToSlug: "다변량분석", Title: "다변량분석"},
	{NotionPageID: "a2f4b890-1f50-4390-acfc-871e612dce31", ToSlug: "빅데이터-분석기사", Title: "빅데이터 분석기사"},
	{NotionPageID: "79157a77-bc76-4b48-9aaa-370c95fc9658", ToSlug: "수리통계-응용", Title: "데이터분석 기초 : R언어"},
	{NotionPageID: "1ab1ecd4-d96b-4f1c-8c67-3e68cadca1c6", ToSlug: "수리통계-응용", Title: "데이터분석 기초 : 파이썬"},
	{NotionPageID: "226b7998-bd88-4892-88aa-1227dc89b5f0", ToSlug: "머신러닝-기초이론", Title: "핸즈온 머신러닝 2"},
	{NotionPageID: "86071a86-939b-49f0-8b73-b8a96a04afc1", ToSlug: "머신러닝", Title: "머신러닝 & 딥러닝"},
}

// DropCategory는 사람이 없애기로 한 카테고리다.
//
// 이 프로젝트는 글을 지우지 않지만 카테고리는 분류일 뿐이라 지울 수 있다.
// regroup이 **글도 자식도 없을 때만** 지운다. 그렇지 않으면 에러로 멈춘다 —
// 딸린 것을 조용히 잃지 않기 위해서다.
//
// **자식이 먼저다.** 표에 적힌 순서대로 지우므로, 부모를 자식보다 먼저 적으면
// "하위 분류가 남아 있다"고 멈춘다.
type DropCategory struct {
	SourceName string
	Why        string
}

var DropCategories = []DropCategory{
	{
		SourceName: "프로젝트",
		Why:        "/project 밑에 같은 이름이라 프로젝트 > 프로젝트로 겹친다. 글은 PostMoves로 위로 올렸다",
	},
	{
		SourceName: "시스템 프로그래밍",
		Why:        "컴퓨터 시스템 분류 전체를 내용 부족으로 없애기로 했다. 글은 DropPosts로 뺐다",
	},
	{
		SourceName: "컴퓨터 구조 : 이론",
		Why:        "컴퓨터 시스템 분류 전체를 내용 부족으로 없애기로 했다. 세 글 모두 빈 페이지라 DropPosts로 뺐다",
	},
	{
		SourceName: "컴퓨터 시스템",
		Why:        "내용이 너무 적어 CS 이론의 독립 갈래로 남기지 않기로 했다. 하위 분류와 글을 먼저 뺐다",
	},
	{
		SourceName: "머신러닝 : 기초이론",
		Why:        "글 열 건이 핸즈온 머신러닝 2와 겹쳐서 뺐다(DropPosts). 그 자리를 핸즈온이 대신한다",
	},
	{
		SourceName: "수학 & 통계",
		Why: "사람이 데이터 & 수리를 이론·응용·머신러닝 세 갈래로 갈라서 이 중간층이 " +
			"할 일이 없어졌다. 밑에 있던 분류는 세 갈래로 나눠 붙였고 직접 붙어 있던 " +
			"목차 글은 PostMoves로 제 분류에 내려보냈다",
	},
	{
		SourceName: "머신러닝 & 딥러닝",
		Why:        "머신러닝 갈래가 그 자리를 대신한다. 밑에 있던 것은 전부 옮겼다",
	},
	{
		SourceName: "전주시 데이터 분석",
		Why:        "전주 데이터분석 밑의 하위 분류. 같이 없앤다",
	},
	{
		SourceName: "전주 데이터분석",
		Why:        "프로젝트 전체를 블로그에 남기지 않기로 했다. 글은 DropPosts로 뺐다",
	},
}

// Cover는 사람이 만든 분류에 붙인 표지 글이다.
//
// categorize는 노션 최상위 19개에만 표지를 이어준다(경로가 자기 제목뿐인 글).
// 그 위에 얹은 8개는 source_name이 NULL이라 categorize가 아예 안 건드리는 층이다.
//
// categories.cover_post_id는 UNIQUE다. 한 글은 한 카테고리의 표지만 될 수 있어서,
// 여기 적는 건 표지를 "새로 만드는" 게 아니라 "옮기는" 것이다. regroup이 옛 자리를
// 먼저 비운다.
type Cover struct {
	// Slug는 표지를 붙일 카테고리 slug다.
	Slug string
	// NotionPageID는 표지로 쓸 글이다. posts의 멱등 키라 slug보다 안정적이다.
	NotionPageID string
	Why          string
}

var Covers = []Cover{
	{
		Slug: "intro", NotionPageID: "1080901b-87f1-80d2-811a-eba467c2c160",
		Why: "소개를 누르면 목록이 아니라 자기소개가 바로 나와야 한다",
	},

	// 아래 열셋은 노션 2단계 페이지의 목차 글이다. 중간층이 없어지면서 각자 제
	// 분류로 내려왔고(PostMoves), 그 분류의 표지로 삼는다.
	{Slug: "data-math", NotionPageID: "5d2a5e48-0d85-4fc5-94d3-280cbeef87ee", Why: "수학 & 통계 분류를 누르면 목록보다 소개가 먼저 보여야 한다"},
	{Slug: "머신러닝", NotionPageID: "86071a86-939b-49f0-8b73-b8a96a04afc1", Why: "머신러닝 & 딥러닝 분류를 누르면 목록보다 소개가 먼저 보여야 한다"},
	{Slug: "선형대수", NotionPageID: "ad1ef256-4567-4b9f-b57e-6f16486d0606", Why: "선형대수 분류를 누르면 목록보다 소개가 먼저 보여야 한다"},
	{Slug: "최적화이론", NotionPageID: "660e3d79-427d-40f7-b98a-6f8be0a5f787", Why: "최적화이론 분류를 누르면 목록보다 소개가 먼저 보여야 한다"},
	{Slug: "이산수학", NotionPageID: "0b6791d6-adfe-48d8-a91d-54e620d708c3", Why: "이산수학 분류를 누르면 목록보다 소개가 먼저 보여야 한다"},
	{Slug: "수리통계1", NotionPageID: "7032b31f-d9ff-4365-83dc-b77ad8382d24", Why: "수리통계1 분류를 누르면 목록보다 소개가 먼저 보여야 한다"},
	{Slug: "수리통계2", NotionPageID: "59d18904-4ed5-4f0a-ba61-17ec86d9fc7b", Why: "수리통계2 분류를 누르면 목록보다 소개가 먼저 보여야 한다"},
	{Slug: "확률과정론", NotionPageID: "3280fbb3-019d-47dd-bc94-f1e433c99efc", Why: "확률과정론 분류를 누르면 목록보다 소개가 먼저 보여야 한다"},
	{Slug: "탐색적-자료분석", NotionPageID: "ab031d71-5474-4a03-bc3f-124391842de7", Why: "탐색적 자료분석 분류를 누르면 목록보다 소개가 먼저 보여야 한다"},
	{Slug: "회귀분석", NotionPageID: "7bb6bd36-9ad9-4ef8-8b2f-827080fad9f3", Why: "회귀분석 분류를 누르면 목록보다 소개가 먼저 보여야 한다"},
	{Slug: "다변량분석", NotionPageID: "cb02b6b8-6f29-479d-a791-25f1cd748b78", Why: "다변량분석 분류를 누르면 목록보다 소개가 먼저 보여야 한다"},
	{Slug: "빅데이터-분석기사", NotionPageID: "a2f4b890-1f50-4390-acfc-871e612dce31", Why: "빅데이터 분석기사 분류를 누르면 목록보다 소개가 먼저 보여야 한다"},
	{Slug: "머신러닝-기초이론", NotionPageID: "226b7998-bd88-4892-88aa-1227dc89b5f0", Why: "핸즈온 머신러닝 2 분류를 누르면 목록보다 소개가 먼저 보여야 한다"},
}

// MovedSourceNames는 사람이 옮긴 카테고리의 source_name 집합이다.
func MovedSourceNames() map[string]bool {
	out := make(map[string]bool, len(Moves))
	for _, m := range Moves {
		out[m.SourceName] = true
	}
	return out
}

// CoverPageIDs는 사람이 표지로 지정한 글의 notion_page_id 집합이다.
func CoverPageIDs() map[string]bool {
	out := make(map[string]bool, len(Covers))
	for _, c := range Covers {
		out[c.NotionPageID] = true
	}
	return out
}

// PostMoveBySlug는 옮길 글 → 목표 카테고리 slug 대응이다.
func PostMoveBySlug() map[string]string {
	out := make(map[string]string, len(PostMoves))
	for _, m := range PostMoves {
		out[m.NotionPageID] = m.ToSlug
	}
	return out
}

// ── 본문에서 덜어낸 것 ──────────────────────────────────────────────────

// DropPost는 사람이 이관하지 않기로 한 글이다.
//
// **이 프로젝트는 원칙적으로 글을 지우지 않는다.** 공개 여부는 삭제가 아니라
// status로 가린다. 여기 적는 것은 그 원칙의 예외이고, 그래서 이유를 함께 적는다.
//
// **DB에서 행만 지우면 안 된다.** 덤프에 있는 한 다음 `import -db`가 다시 넣는다.
// 여기 적어두면 변환 단계에서 아예 건너뛰므로 몇 번을 다시 이관해도 안 돌아온다.
type DropPost struct {
	// NotionPageID는 안 넣을 글이다. posts의 멱등 키다.
	NotionPageID string
	Title        string // 사람이 읽으라고 적어두는 것. 대조에 쓰지 않는다.
	Why          string
}

var DropPosts = []DropPost{
	{
		NotionPageID: "1678930f-1003-4482-81be-699992b904bc",
		Title:        "(제목 없음)",
		Why: "회귀분석 > 회귀분석 : 코드 밑에 제목도 본문도 없이 남은 빈 페이지다. " +
			"목록에서 자리만 차지하고 눌러도 볼 것이 없다",
	},

	// 아래 열 건은 `머신러닝 : 기초이론` 카테고리에 있던 글이다. 다섯은 본문이
	// 0바이트고 나머지도 78~852자인데, 그 내용이 `핸즈온 머신러닝 2`(29편)와
	// 겹친다. 그 자리를 핸즈온이 대신하기로 해서 뺀다.
	{
		NotionPageID: "0f2d8cfe-d06d-4319-8413-3844000a8c23",
		Title:        "나이브 베이즈",
		Why:          "머신러닝 : 기초이론에 있던 글. 절반이 빈 글이고 나머지도 핸즈온 머신러닝 2와 내용이 겹친다",
	},
	{
		NotionPageID: "83f17a2c-04a6-4665-ac26-ea5a344b2f1a",
		Title:        "SVM",
		Why:          "머신러닝 : 기초이론에 있던 글. 절반이 빈 글이고 나머지도 핸즈온 머신러닝 2와 내용이 겹친다",
	},
	{
		NotionPageID: "4e464ba1-26cb-4374-b3d1-565f1b9d7802",
		Title:        "신경망",
		Why:          "머신러닝 : 기초이론에 있던 글. 절반이 빈 글이고 나머지도 핸즈온 머신러닝 2와 내용이 겹친다",
	},
	{
		NotionPageID: "5453a9cc-b816-4bdc-875f-53ae9c164eba",
		Title:        "결정 트리",
		Why:          "머신러닝 : 기초이론에 있던 글. 절반이 빈 글이고 나머지도 핸즈온 머신러닝 2와 내용이 겹친다",
	},
	{
		NotionPageID: "cbda5963-ead6-4591-8928-9ec58822aed6",
		Title:        "앙상블 학습",
		Why:          "머신러닝 : 기초이론에 있던 글. 절반이 빈 글이고 나머지도 핸즈온 머신러닝 2와 내용이 겹친다",
	},
	{
		NotionPageID: "bc1c20f0-3945-450c-b855-8d9323df8e82",
		Title:        "KNN",
		Why:          "머신러닝 : 기초이론에 있던 글. 절반이 빈 글이고 나머지도 핸즈온 머신러닝 2와 내용이 겹친다",
	},
	{
		NotionPageID: "fb869234-0a1d-46f8-9418-c137644612c8",
		Title:        "분류모델 평가",
		Why:          "머신러닝 : 기초이론에 있던 글. 절반이 빈 글이고 나머지도 핸즈온 머신러닝 2와 내용이 겹친다",
	},
	{
		NotionPageID: "d0250380-964c-4123-b73c-347129499957",
		Title:        "k-means",
		Why:          "머신러닝 : 기초이론에 있던 글. 절반이 빈 글이고 나머지도 핸즈온 머신러닝 2와 내용이 겹친다",
	},
	{
		NotionPageID: "88c63e84-59bc-4a26-8f30-21ffe0b307a9",
		Title:        "회귀학습",
		Why:          "머신러닝 : 기초이론에 있던 글. 절반이 빈 글이고 나머지도 핸즈온 머신러닝 2와 내용이 겹친다",
	},
	{
		NotionPageID: "6a1f9122-3ed0-4860-8f7b-fb28edc8e478",
		Title:        "예측모델 평가",
		Why:          "머신러닝 : 기초이론에 있던 글. 절반이 빈 글이고 나머지도 핸즈온 머신러닝 2와 내용이 겹친다",
	},

	// 전주 데이터분석 프로젝트 전체. 분류까지 통째로 없앤다.
	{
		NotionPageID: "6e1fa7b1-7fbe-4071-96ed-83e866015d63",
		Title:        "전주 데이터분석",
		Why:          "전주 데이터분석 프로젝트. 블로그에 남기지 않기로 했다",
	},
	{
		NotionPageID: "1069fbc9-488d-49d3-ae11-cd26a3bcb00f",
		Title:        "데이터가 필요해요!",
		Why:          "전주 데이터분석 프로젝트. 블로그에 남기지 않기로 했다",
	},
	{
		NotionPageID: "7f180bda-3097-443b-8a34-b8aec0d9e684",
		Title:        "중간 점검 - 방향 확인하기",
		Why:          "전주 데이터분석 프로젝트. 블로그에 남기지 않기로 했다",
	},
	{
		NotionPageID: "7f550d90-029e-43ae-957c-45ebe62d565b",
		Title:        "분석 배경",
		Why:          "전주 데이터분석 프로젝트. 블로그에 남기지 않기로 했다",
	},
	{
		NotionPageID: "7fc6dc5d-ae7a-4f03-97b2-214e54f6b72d",
		Title:        "어떤 모델이 좋을까요~?",
		Why:          "전주 데이터분석 프로젝트. 블로그에 남기지 않기로 했다",
	},
	{
		NotionPageID: "eef0901f-9d98-45dd-a47b-09c60dcf8c32",
		Title:        "가설을 설정해 봅시다!",
		Why:          "전주 데이터분석 프로젝트. 블로그에 남기지 않기로 했다",
	},
	{
		NotionPageID: "ffc452d1-8681-471f-802b-2f6361257996",
		Title:        "결론!",
		Why:          "전주 데이터분석 프로젝트. 블로그에 남기지 않기로 했다",
	},

	// 선형대수 카테고리의 draft 일곱 건. 노션에서 목차만 만들고 내용을 안 쓴
	// 자리라 본문이 0바이트고, 들어오는 링크도 없다.
	{
		NotionPageID: "8e4b2948-c366-491c-9dd0-1028c95b7a68",
		Title:        "고유값과 대각화",
		Why:          "선형대수 : 이론 밑의 빈 껍데기(0바이트 draft). 같은 제목의 알맹이가 최적화이론에서 옮겨와 있어 목록에 두 벌로 보였다",
	},
	{
		NotionPageID: "8dcfb67d-88f8-4722-8564-9fa418d3ab8b",
		Title:        "내적공간",
		Why:          "선형대수 : 이론 밑의 빈 껍데기(0바이트 draft). 같은 제목의 알맹이가 최적화이론에서 옮겨와 있어 목록에 두 벌로 보였다",
	},
	{
		NotionPageID: "5d08e0e6-64c1-477a-9553-ab62c790a817",
		Title:        "벡터공간",
		Why:          "선형대수 : 이론 밑의 빈 껍데기(0바이트 draft). 같은 제목의 알맹이가 최적화이론에서 옮겨와 있어 목록에 두 벌로 보였다",
	},
	{
		NotionPageID: "85e0d2ed-36ce-4470-a6c5-fcbb63edb984",
		Title:        "선형변환",
		Why:          "선형대수 : 이론 밑의 빈 껍데기(0바이트 draft). 같은 제목의 알맹이가 최적화이론에서 옮겨와 있어 목록에 두 벌로 보였다",
	},
	{
		NotionPageID: "8c662a18-7c27-420c-91ad-3f08dfda5ea5",
		Title:        "연립방정식과 행렬",
		Why:          "선형대수 : 이론 밑의 빈 껍데기(0바이트 draft). 같은 제목의 알맹이가 최적화이론에서 옮겨와 있어 목록에 두 벌로 보였다",
	},
	{
		NotionPageID: "da5db854-62ab-4549-8e18-bef999251c10",
		Title:        "표준형",
		Why:          "선형대수 : 이론 밑의 빈 껍데기(0바이트 draft). 같은 제목의 알맹이가 최적화이론에서 옮겨와 있어 목록에 두 벌로 보였다",
	},
	{
		NotionPageID: "c3e17917-ff6c-4a06-80a9-f1e618354549",
		Title:        "행렬식",
		Why:          "선형대수 : 이론 밑의 빈 껍데기(0바이트 draft). 같은 제목의 알맹이가 최적화이론에서 옮겨와 있어 목록에 두 벌로 보였다",
	},

	// 핸즈온 머신러닝 2 표지 끝의 한 건짜리 `연습문제 2` 절. 표지 본문에서
	// 제목과 링크도 BodyEdits로 함께 덜어내므로 상세 글도 이관하지 않는다.
	{
		NotionPageID: "358b2929-84e3-406f-8575-0e19534153d0",
		Title:        "교차검증과 과대적합",
		Why:          "핸즈온 머신러닝 2 끝의 한 건짜리 연습문제 2 절을 없애기로 했다",
	},

	// `컴퓨터 시스템` 전체. 표지 한 편과 시스템 프로그래밍 13편,
	// 컴퓨터 구조의 빈 페이지 3편뿐이라 독립 갈래를 통째로 없앤다.
	{NotionPageID: "e849a962-3039-446d-b187-0bad30806c94", Title: "컴퓨터 시스템", Why: "내용이 너무 적어 컴퓨터 시스템 분류 전체를 없애기로 했다"},
	{NotionPageID: "ddbdbae0-05c1-4448-bca6-9077d49fe917", Title: "공유 메모리", Why: "컴퓨터 시스템 분류 전체를 없애기로 했다"},
	{NotionPageID: "acc837f3-86ef-48f9-ae92-194a89ac3830", Title: "멀티 프로세스", Why: "컴퓨터 시스템 분류 전체를 없애기로 했다"},
	{NotionPageID: "cde2478b-cf91-4f8c-9c2c-cc4d539f8d69", Title: "메모리 매핑", Why: "컴퓨터 시스템 분류 전체를 없애기로 했다"},
	{NotionPageID: "42763dc8-dc5c-4788-8d44-2a0084f5e23e", Title: "세마포어", Why: "컴퓨터 시스템 분류 전체를 없애기로 했다"},
	{NotionPageID: "2ee4aa56-d58d-4594-8038-81e051a82c51", Title: "시그널", Why: "컴퓨터 시스템 분류 전체를 없애기로 했다"},
	{NotionPageID: "ba54d5a3-31ac-4637-a55e-420f113498a0", Title: "시스템 정보 확인", Why: "컴퓨터 시스템 분류 전체를 없애기로 했다"},
	{NotionPageID: "f5c628f7-8450-447a-960e-09a01edb7a49", Title: "시스템 프로그래밍이란?", Why: "컴퓨터 시스템 분류 전체를 없애기로 했다"},
	{NotionPageID: "7b1b64ba-bd37-4202-b38a-c17afe380a29", Title: "파이프", Why: "컴퓨터 시스템 분류 전체를 없애기로 했다"},
	{NotionPageID: "3f6a30ce-bf2b-457a-8ee9-7a4d98ff1659", Title: "파일 입출력", Why: "컴퓨터 시스템 분류 전체를 없애기로 했다"},
	{NotionPageID: "db5009e7-55a7-4614-aff9-22a09faaec15", Title: "파일과 디렉토리", Why: "컴퓨터 시스템 분류 전체를 없애기로 했다"},
	{NotionPageID: "df8aed14-9c58-4e35-a68f-2ad82f70ed74", Title: "파일이란?", Why: "컴퓨터 시스템 분류 전체를 없애기로 했다"},
	{NotionPageID: "730fefc8-33ec-4021-87c1-5282904c05c7", Title: "표준 입출력", Why: "컴퓨터 시스템 분류 전체를 없애기로 했다"},
	{NotionPageID: "d46052e4-26b7-4004-a0e8-6bcb91f8feda", Title: "프로세스 : 개요", Why: "컴퓨터 시스템 분류 전체를 없애기로 했다"},
	{NotionPageID: "2c224d59-221f-4fdf-8428-998ff9c8df0d", Title: "(제목 없음)", Why: "컴퓨터 구조 : 이론 밑의 빈 페이지다"},
	{NotionPageID: "b9a67d80-4be7-473f-a715-cd0b6310b520", Title: "(제목 없음)", Why: "컴퓨터 구조 : 이론 밑의 빈 페이지다"},
	{NotionPageID: "dfd45512-9cf2-4e24-be50-2759963c7141", Title: "(제목 없음)", Why: "컴퓨터 구조 : 이론 밑의 빈 페이지다"},
}

// Dropped는 이 글을 이관에서 빼야 하는지다.
func Dropped(pageID string) bool {
	for _, d := range DropPosts {
		if d.NotionPageID == pageID {
			return true
		}
	}
	return false
}

// DropImage는 덤프에는 남아 있지만 블로그 DB에는 넣지 않을 이미지다.
// 본문 참조만 지우면 `/img/{sha256}`를 아는 경우 파일은 계속 열리므로,
// 공개하지 않기로 한 개인 사진은 BLOB도 함께 제외한다.
type DropImage struct {
	SHA256 string
	Why    string
}

var DropImages = []DropImage{
	{
		SHA256: "0f9f83dcd63eb36d2bbc1c616342d8a8d2edfc29b6ba318debc159bcbf336128",
		Why:    "자기소개에서 증명사진을 공개하지 않기로 했다. BodyEdits가 본문 참조도 지운다",
	},
}

func DroppedImage(sha256 string) bool {
	for _, image := range DropImages {
		if image.SHA256 == sha256 {
			return true
		}
	}
	return false
}

// BodyEdit는 사람이 본문에서 덜어내기로 한 줄이다.
//
// 위의 표들과 달리 이건 분류가 아니라 **본문**을 고친다. 그래서 보는 쪽도
// 다르다 — cmd/import가 변환 직후에 적용한다(categorize·regroup이 아니다).
//
// **이관 시점에 적용하는 이유.** DB의 body를 손으로 고쳐두면 `import -db`를
// 다시 돌릴 때 변환 결과가 덮어써서 되살아난다. relink 재작성이 날아가는 것과
// 같은 성질이다. 여기 적어두면 몇 번을 다시 이관해도 같은 결과가 나온다.
//
// 덤프는 고정이라 변환 결과도 고정이다. 그래서 Remove가 안 맞으면 그건
// 표가 낡았거나 변환기가 바뀐 것이다 — 조용히 넘어가지 않고 에러를 낸다.
type BodyEdit struct {
	// NotionPageID는 고칠 글이다. posts의 멱등 키라 slug보다 안정적이다.
	NotionPageID string
	// Remove는 지울 줄이다. 변환 결과에 **그 줄 전체가 정확히** 있어야 한다.
	// 조각이 아니라 줄 단위인 이유: 문장 중간을 지우면 앞뒤가 어떻게 이어지는지
	// 표만 보고는 알 수 없다.
	Remove string
	Title  string // 사람이 읽으라고 적어두는 것. 대조에 쓰지 않는다.
	Why    string
}

var BodyEdits = []BodyEdit{
	{
		NotionPageID: "1080901b-87f1-80d2-811a-eba467c2c160",
		Remove:       "![](/img/0f9f83dcd63eb36d2bbc1c616342d8a8d2edfc29b6ba318debc159bcbf336128)",
		Title:        "최인렬 (Inryeol Choi)",
		Why:          "블로그 자기소개에서 증명사진을 노출하지 않기로 했다",
	},
	{
		NotionPageID: "ad1ef256-4567-4b9f-b57e-6f16486d0606",
		Remove:       "[Untitled](/p/1e8accad-7dac-4eb2-8da7-383a404b3ee5)",
		Title:        "선형대수",
		Why:          "이름 없는 인라인 데이터베이스라 눌러도 404다. 목록 맨 위에서 자리만 차지한다",
	},
	{
		NotionPageID: "ad1ef256-4567-4b9f-b57e-6f16486d0606",
		Remove:       "[선형대수 : 이론](/p/22206acc-5b0e-445c-8e09-52ef3a41cf4a)",
		Title:        "선형대수",
		Why: "그 데이터베이스의 행이던 글 일곱 건을 DropPosts로 뺐다. " +
			"이제 눌러도 빈 목록이라 링크째 없앤다",
	},
	{
		NotionPageID: "1080901b-87f1-80d2-811a-eba467c2c160",
		Remove:       "[프로젝트](/p/fd9d12dc-83de-4424-9428-0f26582130bc)",
		Title:        "최인렬 (Inryeol Choi)",
		Why: "자기소개 끝의 인라인 데이터베이스 링크다. 홈이 이 글을 통째로 " +
			"펴는데, 프로젝트로 가는 길은 사이드바에 이미 있어서 자리만 차지한다",
	},
	{
		NotionPageID: "226b7998-bd88-4892-88aa-1227dc89b5f0",
		Remove:       "## 연습문제 2",
		Title:        "핸즈온 머신러닝 2",
		Why:          "끝의 한 건짜리 연습문제 2 절을 없애기로 했다",
	},
	{
		NotionPageID: "226b7998-bd88-4892-88aa-1227dc89b5f0",
		Remove:       "[교차검증과 과대적합](/p/358b2929-84e3-406f-8575-0e19534153d0)",
		Title:        "핸즈온 머신러닝 2",
		Why:          "연습문제 2 절과 그 안의 유일한 링크를 함께 없앤다",
	},
}

// ApplyBodyEdits는 한 페이지의 변환 결과에서 BodyEdits에 적힌 줄을 덜어낸다.
//
// 지운 줄 자리에 빈 줄이 겹치면 하나로 줄인다. 안 그러면 마크다운에 빈 줄이
// 둘 남아 문단 사이가 벌어진다.
//
// 적힌 줄을 못 찾으면 에러다. 표가 낡았는데 조용히 넘어가면, 지웠다고 믿는
// 것이 본문에 그대로 남아 있게 된다.
func ApplyBodyEdits(pageID, body string) (string, error) {
	out := body
	for _, e := range BodyEdits {
		if e.NotionPageID != pageID {
			continue
		}
		next, ok := removeLine(out, e.Remove)
		if !ok {
			return "", fmt.Errorf("본문에서 지울 줄을 못 찾았다 (%s %q): %q",
				e.NotionPageID, e.Title, e.Remove)
		}
		out = next
	}
	return out, nil
}

// removeLine은 그 줄 하나를 지운다. 첫 번째 것만 지운다.
func removeLine(body, target string) (string, bool) {
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		if strings.TrimRight(l, " \t") != target {
			continue
		}
		lines = append(lines[:i:i], lines[i+1:]...)
		// 지운 자리에서 빈 줄이 겹치면 하나로 줄인다.
		if i > 0 && i < len(lines) && lines[i-1] == "" && lines[i] == "" {
			lines = append(lines[:i:i], lines[i+1:]...)
		}
		// 첫 줄을 지웠다면 그 뒤의 문단 구분용 빈 줄도 필요 없다.
		if i == 0 && len(lines) > 0 && lines[0] == "" {
			lines = lines[1:]
		}
		return strings.Join(lines, "\n"), true
	}
	return body, false
}

// BodyEditPageIDs는 본문을 고칠 글의 notion_page_id 집합이다.
// 표가 낡았는지 보려고 cmd/import가 쓴다.
func BodyEditPageIDs() map[string]bool {
	out := make(map[string]bool, len(BodyEdits))
	for _, e := range BodyEdits {
		out[e.NotionPageID] = true
	}
	return out
}
