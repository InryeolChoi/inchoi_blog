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
	"time"
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
	{SourceName: "핸즈온 머신러닝 2", ToSlug: "머신러닝", Why: "데이터 & 수리를 세 갈래로 가르면서 옮겼다"},
	{SourceName: "자연어처리 (1) : BERT와 GPT", ToSlug: "머신러닝", Why: "데이터 & 수리를 세 갈래로 가르면서 옮겼다"},

	// 웹 프로그래밍 77편을 서버 & API / 클라이언트 & UI 두 갈래로 갈랐다.
	// 배운 순서가 Django → Spring이고 HTML/CSS·React·로그인·웹소켓은 곁다리로
	// 붙어 있어서, 한 갈래 안에서는 무엇이 줄기인지 보이지 않았다.
	{SourceName: "Django", ToSlug: "서버-api", Why: "웹 프로그래밍을 두 갈래로 가르면서 옮겼다"},
	{SourceName: "Spring", ToSlug: "서버-api", Why: "웹 프로그래밍을 두 갈래로 가르면서 옮겼다"},
	{SourceName: "Node.js", ToSlug: "서버-api", Why: "웹 프로그래밍을 두 갈래로 가르면서 옮겼다"},
	{SourceName: "Javascript", ToSlug: "클라이언트-ui", Why: "웹 프로그래밍을 두 갈래로 가르면서 옮겼다"},
	{SourceName: "React", ToSlug: "클라이언트-ui", Why: "웹 프로그래밍을 두 갈래로 가르면서 옮겼다"},
	// 모바일은 개발 직속 5편짜리 갈래였다. Swift/UIKit도 결국 화면을 그리는
	// 쪽이라 클라이언트 밑으로 내린다 — 갈래 이름을 "프론트엔드"가 아니라
	// "클라이언트 & UI"로 둔 이유가 이것이다.
	{SourceName: "모바일 프로그래밍", ToSlug: "클라이언트-ui", Why: "웹 프론트와 모바일은 둘 다 화면을 그리는 쪽이라 한 갈래로 묶었다"},
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

// PostMoves는 사람이 정한 글 이동이다. 왜 옮기는지는 묶음마다 적어둔다.
var PostMoves = []PostMove{
	// 웹 프로그래밍 직속 글 열둘. 분류가 없어지므로 갈 곳을 정해준다.
	//
	// **입구 글 다섯은 같은 이름의 하위 분류로 내려보내고 그 분류의 표지로 삼는다**
	// (Covers). école 42에서 쓴 방법과 같다 — 안 그러면 `서버 & API` 화면에
	// `Django` 글과 `Django` 분류가 나란히 서서 같은 곳으로 가는 길이 두 벌이 된다.
	{NotionPageID: "6c8d3a3f-d398-4029-ab5c-92388eb194d1", ToSlug: "django", Title: "Django"},
	{NotionPageID: "4c00e9ea-5f92-4a36-b851-cd1c306aac52", ToSlug: "spring", Title: "Spring"},
	{NotionPageID: "b253d89d-0d82-459d-94a7-27a7dad05036", ToSlug: "node-js", Title: "Node.js"},
	{NotionPageID: "6f267d09-77fe-4cd9-aef5-2bdb47ba49de", ToSlug: "javascript", Title: "Javascript"},
	{NotionPageID: "4d9af5a1-cd10-419b-8c89-17a8b40a93f7", ToSlug: "react", Title: "React"},

	// 나머지 일곱은 제 분류가 없다. 성격대로 두 갈래에 직접 붙인다.
	// 로그인·인증과 웹소켓은 서버가 하는 일이고, HTML & CSS는 화면 쪽이다.
	{NotionPageID: "544a27a7-2c8e-42ff-9b37-59f86ebbdc69", ToSlug: "서버-api", Title: "Rest 프레임워크"},
	{NotionPageID: "9713e08a-f32b-4953-a827-9a55c241b667", ToSlug: "서버-api", Title: "쿠키와 세션"},
	{NotionPageID: "b1d6e4c4-d5ef-4f95-abc4-9bd87d471f94", ToSlug: "서버-api", Title: "인증과 인가"},
	{NotionPageID: "902c6a9a-059e-4fe8-ada0-e0b28152f780", ToSlug: "서버-api", Title: "OAuth 인증"},
	{NotionPageID: "2c7583ba-e6c0-41b7-9c4e-5d827f2a587d", ToSlug: "서버-api", Title: "2FA 인증"},
	{NotionPageID: "c329efdd-d626-43dc-a779-e292ecdec402", ToSlug: "서버-api", Title: "웹소켓이란?"},
	{NotionPageID: "1e7119a0-53e2-4267-877b-b64f4fd434a3", ToSlug: "클라이언트-ui", Title: "HTML & CSS"},

	// 자기소개 한 편은 `소개` 분류에 직접 붙인다.
	//
	// 노션 최상위 페이지 `최인렬 (Inryeol Choi)`가 categorize를 거치면 같은 이름의
	// 카테고리가 되는데, 그 밑에 남는 글은 이 자기소개 한 편뿐이다. 그러면 `소개`를
	// 눌렀을 때 본문이 펴지고 그 아래 "하위 분류"에 같은 글 한 건짜리 갈래가 또
	// 나온다 — 눌러도 같은 글로 되돌아오는 길이다. 글을 `소개`에 직접 붙이면
	// 표지 글이라 목록에서도 빠지고, 빈 껍데기가 된 카테고리는 DropCategories가 지운다.
	{NotionPageID: "1080901b-87f1-80d2-811a-eba467c2c160", ToSlug: "intro", Title: "최인렬 (Inryeol Choi)"},

	// 아래 6건은 노션에서 자기소개 페이지 밑 "프로젝트" 목록에 있던 껍데기다.
	// 본문이 0바이트고, 실제 내용은 école 42 밑의 별도 글에 있다. 그래도 지우지
	// 않는다(이 프로젝트는 글을 지우지 않는다). 과제 목록 링크 모음 성격이라
	// 프로젝트 최상위에 직접 붙인다.
	{NotionPageID: "f4847474-a809-47a4-a9a7-7db997b66bf1", ToSlug: "project", Title: "FT_IRC"},
	{NotionPageID: "ddd3c9db-d3e4-40f7-8878-8da7f5c9d1fd", ToSlug: "project", Title: "Inception"},
	{NotionPageID: "1080901b-87f1-80d6-82e2-c81d4d9b4401", ToSlug: "project", Title: "MiniShell"},
	{NotionPageID: "1080901b-87f1-800c-b1d5-f98d79010552", ToSlug: "project", Title: "PhiloSopher"},
	{NotionPageID: "5fd73f8a-648d-4b15-88be-c3612e0c3262", ToSlug: "project", Title: "Where42"},
	{NotionPageID: "e210fa33-6030-430e-8a7b-58d1026b1ba7", ToSlug: "project", Title: "심심조각"},

	// école 42 표지에는 아래 17편이 하위 분류의 입구처럼 적혀 있지만, 경로만 보면
	// école 42에 직접 붙는 글이라 같은 이름의 하위 분류와 화면에서 두 벌로 보였다.
	// 각 글을 제 하위 분류로 옮겨 표지로 쓴다(Covers 참고).
	{NotionPageID: "c7bcee75-28c5-4945-b5c3-f8e24e79e5e7", ToSlug: "c", Title: "C"},
	{NotionPageID: "5c84aeb4-c3d9-4341-a66b-acc71487be94", ToSlug: "cpp-part-1", Title: "Cpp part.1"},
	{NotionPageID: "0016e85a-614f-426c-ae62-f46427a7b719", ToSlug: "netpractice", Title: "Netpractice"},
	{NotionPageID: "5b239f6c-32bb-4a4a-921b-00db117abc3d", ToSlug: "shell", Title: "Shell"},
	{NotionPageID: "c4e6e521-2c48-460b-91d8-54d8452f2096", ToSlug: "born2beroot", Title: "born2beroot"},
	{NotionPageID: "92ab7921-feb3-4604-b9ce-171a3b8a4629", ToSlug: "cub3d", Title: "cub3D"},
	{NotionPageID: "90a0bf6c-e299-4158-8e13-d367d96298e5", ToSlug: "exam02", Title: "exam02"},
	{NotionPageID: "96b38479-74a3-4983-a9d9-c5530a9b94c8", ToSlug: "exam03", Title: "exam03"},
	{NotionPageID: "f6ceedec-0c40-4a30-8b97-05c05868e6f7", ToSlug: "fdf-fil-de-fer", Title: "fdf (fil de fer)"},
	{NotionPageID: "97c6e452-91e6-4c19-9769-802b6be9a982", ToSlug: "ft-irc-server", Title: "ft_irc (server)"},
	{NotionPageID: "62d53e5d-77fb-4a4e-b789-9a59ef3a70e4", ToSlug: "ft-printf", Title: "ft_printf"},
	{NotionPageID: "38315c40-5daf-4387-bae3-bde27387b43f", ToSlug: "get-next-line", Title: "get_next_line"},
	{NotionPageID: "4786e824-c744-4c32-8886-729e8e8c9bc6", ToSlug: "inception", Title: "inception"},
	{NotionPageID: "dcbbd10e-b50a-4a1d-8a4a-237426aa7249", ToSlug: "libft", Title: "libft"},
	{NotionPageID: "518ea6e1-306c-4cfa-8b3e-6ac182c16e14", ToSlug: "minishell", Title: "minishell"},
	{NotionPageID: "cdf4ecb8-ecdb-4cda-9dbc-d74722213449", ToSlug: "philosopher", Title: "philosopher"},
	{NotionPageID: "250631b1-6ac8-4e58-8dc2-eec4ecaca254", ToSlug: "pipex", Title: "pipex"},

	// 노션에서 `최적화이론 > 수업 : 통수 & 선계`에 섞여 있던 글 16편이다. 그중
	// 선형대수 11편만 선형대수로 옮기고, 선형계획·심플렉스·쌍대정리·라그랑주는
	// 최적화이론에 둔다. 최적화 표지의 옛 수업 묶음 링크 자체는 BodyEdits로 뺀다.
	//
	// `수학적 최적화`에는 같은 제목의 별도 draft가 일부 있지만 대부분 본문이
	// 0바이트인 껍데기다(노션에서 목차만 만들고 내용을 안 쓴 것). 내용이 있는 이
	// 선형대수 글은 선형대수로 옮기고, 내용 있는 최적화 글과 별도 draft는 모두
	// 최적화이론에 둔다 — 이 프로젝트는 글을 지우지 않는다.
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
	{NotionPageID: "404c96b3-e53c-4edb-88ee-8ef0f717ce79", ToSlug: "최적화이론", Title: "라그랑주 승수법"},
	{NotionPageID: "ad882859-b0f2-41d9-9552-c7c14cf0b559", ToSlug: "최적화이론", Title: "선형계획법"},
	{NotionPageID: "ff7a1343-68d1-4465-9df0-ea48a0a2565b", ToSlug: "최적화이론", Title: "심플렉스 알고리즘"},
	{NotionPageID: "e96b9abf-d1de-4790-8656-7ba4a57c4d89", ToSlug: "최적화이론", Title: "쌍대정리: 선형"},
	{NotionPageID: "eedb3add-e5e1-4b8a-a1d3-41bc80e00162", ToSlug: "최적화이론", Title: "심플렉스와 n분위수"},

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
	{NotionPageID: "79157a77-bc76-4b48-9aaa-370c95fc9658", ToSlug: "수리통계-응용", Title: "데이터분석 기초 : R언어"},
	{NotionPageID: "1ab1ecd4-d96b-4f1c-8c67-3e68cadca1c6", ToSlug: "수리통계-응용", Title: "데이터분석 기초 : 파이썬"},
	{NotionPageID: "226b7998-bd88-4892-88aa-1227dc89b5f0", ToSlug: "머신러닝-기초이론", Title: "핸즈온 머신러닝 2"},
	{NotionPageID: "86071a86-939b-49f0-8b73-b8a96a04afc1", ToSlug: "머신러닝", Title: "머신러닝 & 딥러닝"},
}

// PostMetadataEdit는 노션 원본과 다르게 공개 아카이브에서 쓸 제목·작성일·순서다.
// notion_page_id가 멱등 키라 다시 이관해도 같은 글에 적용된다. `OriginalTitle`은
// 표를 잘못 적었거나 덤프 제목이 바뀌었을 때 조용히 엉뚱한 글을 고치지 않게 한다.
type PostMetadataEdit struct {
	NotionPageID  string
	OriginalTitle string
	Title         string
	// OriginalCreatedAt가 비어 있으면 노션 원본 작성일을 그대로 둔다 —
	// **순서만 사람이 정한 글**이 이 경우다(다변량분석 : 코드 12편).
	OriginalCreatedAt string // YYYY-MM-DD, UTC. 비면 원본 유지
	SortOrder         int
	// Group은 이 글이 속한 수동 묶음의 이름이다. **표에 손으로 적지 않고
	// concatMetadataEdits가 찍는다** — 묶음마다 스물몇 줄에 같은 값을 반복하면
	// 언젠가 한 줄이 어긋난다.
	//
	// sort_order는 묶음 **안에서만** 0부터 세므로, 이 이름이 다르지 않으면
	// 두 묶음의 0번이 같은 자리를 다투는 것으로 보인다(cmd/sortorder의 중복 검사).
	Group string
}

// linearAlgebraMetadataEdits의 선형대수 순서는 일반적인 교과서 흐름을 따른다:
// 벡터 → 행렬/소거 → 벡터공간 → 선형변환 → 역행렬 → 내적 → 고유값 → SVD.
// 최적화 주제는 넣지 않는다. 날짜는 사용자의 요청대로 2022년 3~8월 안에서 순서가
// 뒤집히지 않게 임의로 배정했다.
var linearAlgebraMetadataEdits = []PostMetadataEdit{
	{NotionPageID: "f69e826a-2112-4dbe-861d-8b61e0c136a5", OriginalTitle: "벡터", Title: "1. 벡터", OriginalCreatedAt: "2022-03-12", SortOrder: 0},
	{NotionPageID: "7555c053-d6d0-404c-9282-43b737a4c063", OriginalTitle: "행렬", Title: "2. 행렬", OriginalCreatedAt: "2022-03-26", SortOrder: 1},
	{NotionPageID: "c76070b0-c71d-45ba-82d7-a542f7dbab89", OriginalTitle: "피봇연산", Title: "3. 피봇연산", OriginalCreatedAt: "2022-04-09", SortOrder: 2},
	{NotionPageID: "8ef5464f-7a44-41c5-9850-f9804ff9cf2f", OriginalTitle: "벡터공간", Title: "4. 벡터공간", OriginalCreatedAt: "2022-04-23", SortOrder: 3},
	{NotionPageID: "2275909f-ed86-40c7-ba47-198768145ded", OriginalTitle: "선형변환", Title: "5. 선형변환", OriginalCreatedAt: "2022-05-07", SortOrder: 4},
	{NotionPageID: "4e2f7f05-8d33-46bb-8285-833973dd7747", OriginalTitle: "행렬식과 역행렬", Title: "6. 행렬식과 역행렬", OriginalCreatedAt: "2022-05-21", SortOrder: 5},
	{NotionPageID: "9e966d0e-e821-46e5-b5d4-64aae905748d", OriginalTitle: "피봇연산과 역행렬", Title: "7. 피봇연산과 역행렬", OriginalCreatedAt: "2022-06-04", SortOrder: 6},
	{NotionPageID: "c520daf4-6db6-44cd-9767-d0f0c560caa2", OriginalTitle: "내적공간", Title: "8. 내적공간", OriginalCreatedAt: "2022-06-18", SortOrder: 7},
	{NotionPageID: "2cb48833-6e6e-4b48-9f74-2855ef3c63db", OriginalTitle: "고유값과 대각화", Title: "9. 고유값과 대각화", OriginalCreatedAt: "2022-07-02", SortOrder: 8},
	{NotionPageID: "5ef93000-4852-4af7-84cf-37c78b9b3774", OriginalTitle: "피봇연산과 대각화", Title: "10. 피봇연산과 대각화", OriginalCreatedAt: "2022-07-16", SortOrder: 9},
	{NotionPageID: "f77c0e4e-dd54-4a4a-9534-e9f8ca1846a8", OriginalTitle: "특이값 분해", Title: "11. 특이값 분해", OriginalCreatedAt: "2022-07-30", SortOrder: 10},
}

// multivariateCodeOrderEdits는 `다변량분석 : 코드` 인라인 데이터베이스 12편의
// **화면 순서만** 노션 원본과 같게 고정한다. 제목과 작성일은 원본 그대로다 —
// Title은 OriginalTitle과 같고 OriginalCreatedAt는 비워서 노션 날짜를 유지한다.
//
// 필요한 이유: 이 목록은 표지 글 본문의 인라인 DB 링크가 펼치는데, 그 행의
// sort_order는 created_time 순위라 신뢰도가 낮고(주성분(1)이 인자(1)보다 늦게
// 만들어졌다), 제목에 앞 번호도 없어서 sortPosts의 자연 정렬이 가나다순으로
// 세워 버린다. 사람이 정한 순서가 곧 노션 화면 순서다.
//
// 주의: `정준분석 : 코드 `의 끝 공백은 노션 원본 제목 그대로다. 지우면 대조에
// 실패해 import가 멈춘다.
var multivariateCodeOrderEdits = []PostMetadataEdit{
	{NotionPageID: "ec0b947d-bbf6-499f-8645-b5fe394d10bc", OriginalTitle: "다변량분석?", Title: "다변량분석?", SortOrder: 0},
	{NotionPageID: "394c5783-db1f-42b9-9476-a539bcbde6a4", OriginalTitle: "확률분포와 자료행렬", Title: "확률분포와 자료행렬", SortOrder: 1},
	{NotionPageID: "71436e58-812a-4685-ad88-4f4a37a24536", OriginalTitle: "주성분분석 : 코드 (1)", Title: "주성분분석 : 코드 (1)", SortOrder: 2},
	{NotionPageID: "8216bed7-2ce5-4555-8466-e321e8fb849c", OriginalTitle: "주성분분석 : 코드 (2)", Title: "주성분분석 : 코드 (2)", SortOrder: 3},
	{NotionPageID: "0a86612d-5f9e-4724-b2e1-373fd4a8d013", OriginalTitle: "인자분석 : 코드 (1)", Title: "인자분석 : 코드 (1)", SortOrder: 4},
	{NotionPageID: "bf3bee60-ca47-4f8e-b426-a82e989db0cd", OriginalTitle: "인자분석 : 코드 (2)", Title: "인자분석 : 코드 (2)", SortOrder: 5},
	{NotionPageID: "2dba98b9-d19c-4975-980b-1eeab3b8478f", OriginalTitle: "인자분석 : 코드 (3)", Title: "인자분석 : 코드 (3)", SortOrder: 6},
	{NotionPageID: "214d7452-4645-4cdf-8deb-8df6dfa7e74a", OriginalTitle: "인자분석 : 코드 (4)", Title: "인자분석 : 코드 (4)", SortOrder: 7},
	{NotionPageID: "bf5789a3-3a6b-48b9-b674-b8dd0838a27d", OriginalTitle: "정준분석 : 코드 ", Title: "정준분석 : 코드 ", SortOrder: 8},
	{NotionPageID: "765af940-662e-46a8-bbc7-4772ef58189c", OriginalTitle: "대응분석 : 코드", Title: "대응분석 : 코드", SortOrder: 9},
	{NotionPageID: "88ce9e1d-247f-4f58-86ce-1c30c01e27fd", OriginalTitle: "군집분석 : 코드", Title: "군집분석 : 코드", SortOrder: 10},
	{NotionPageID: "0cf2df56-37f2-4acc-ac3c-9a37513275de", OriginalTitle: "판별분석 : 코드", Title: "판별분석 : 코드", SortOrder: 11},
}

// PostMetadataEdits는 사람이 제목·작성일·순서(또는 순서만)를 정한 글 전체다.
// **import가 DB에 넣기 직전에 적용하고 sortorder(-only all)가 같은 표로 덮는다.**
// 웹은 이 표를 모른다 — DB의 sort_order를 읽을 뿐이라, 표를 고친 뒤에는
// 파이프라인을 다시 돌려야 화면이 바뀐다.
var PostMetadataEdits = concatMetadataEdits(
	metadataGroup{"linear-algebra", linearAlgebraMetadataEdits},
	metadataGroup{"multivariate-code", multivariateCodeOrderEdits},
)

// metadataGroup은 이름 하나와 그 묶음의 글들이다.
type metadataGroup struct {
	name  string
	edits []PostMetadataEdit
}

// concatMetadataEdits는 묶음들을 합치면서 각 글에 묶음 이름을 찍는다.
func concatMetadataEdits(groups ...metadataGroup) []PostMetadataEdit {
	var out []PostMetadataEdit
	for _, g := range groups {
		for _, e := range g.edits {
			e.Group = g.name
			out = append(out, e)
		}
	}
	return out
}

// PostTitleEdit는 작성일과 순서는 그대로 두고 제목만 바꾸는 예외다.
// 노션 원본 제목을 덮어쓰므로 import를 다시 돌려도 같은 제목을 유지한다.
type PostTitleEdit struct {
	NotionPageID  string
	OriginalTitle string
	Title         string
}

// PostTitleEdits의 네 건은 수리통계2 참고자료 제목 끝에 붙은 불필요한 `(1)`을 뺀다.
var PostTitleEdits = []PostTitleEdit{
	{NotionPageID: "df333507-a679-4beb-b165-285ae3bf42fc", OriginalTitle: "분포별 가능도함수 (1)", Title: "분포별 가능도함수"},
	{NotionPageID: "805825dd-084e-4612-9465-a2054a0d2004", OriginalTitle: "확률함수와 커널 (1)", Title: "확률함수와 커널"},
	{NotionPageID: "d9fe0a39-89cd-48b6-84ee-0efaa78cf67b", OriginalTitle: "수리통계2 - 과제 (1)", Title: "수리통계2 - 과제"},
	{NotionPageID: "1f3d0731-e367-4d0d-8239-94d92d6d02d5", OriginalTitle: "수리통계2 - 시험 (1)", Title: "수리통계2 - 시험"},
}

// PostTitleByID는 제목만 바꾸는 예외를 notion_page_id로 찾는다.
func PostTitleByID() map[string]PostTitleEdit {
	out := make(map[string]PostTitleEdit, len(PostTitleEdits))
	for _, edit := range PostTitleEdits {
		out[edit.NotionPageID] = edit
	}
	return out
}

// PostMetadataByID는 수동 메타데이터를 notion_page_id로 찾는다.
func PostMetadataByID() map[string]PostMetadataEdit {
	out := make(map[string]PostMetadataEdit, len(PostMetadataEdits))
	for _, edit := range PostMetadataEdits {
		out[edit.NotionPageID] = edit
	}
	return out
}

// ApplyPostMetadata는 import가 DB에 넣기 직전에 수동 제목·날짜·순서를 적용한다.
// 적용 대상이 아니면 받은 값을 그대로 돌려준다.
func ApplyPostMetadata(pageID, title string, createdAt *time.Time) (string, *time.Time, *int, error) {
	edit, ok := PostMetadataByID()[pageID]
	if !ok {
		if titleEdit, titleOK := PostTitleByID()[pageID]; titleOK {
			if title != titleEdit.OriginalTitle && title != titleEdit.Title {
				return "", nil, nil, fmt.Errorf("%s: 제목 표의 원본 제목 %q와 실제 제목 %q가 다르다",
					pageID, titleEdit.OriginalTitle, title)
			}
			return titleEdit.Title, createdAt, nil, nil
		}
		return title, createdAt, nil, nil
	}
	if title != edit.OriginalTitle && title != edit.Title {
		return "", nil, nil, fmt.Errorf("%s: 메타데이터 표의 원본 제목 %q와 실제 제목 %q가 다르다",
			pageID, edit.OriginalTitle, title)
	}
	order := edit.SortOrder
	if edit.OriginalCreatedAt == "" {
		// 순서만 사람이 정한 글이다. 작성일은 노션 원본을 그대로 둔다.
		return edit.Title, createdAt, &order, nil
	}
	date, err := time.Parse("2006-01-02", edit.OriginalCreatedAt)
	if err != nil {
		return "", nil, nil, fmt.Errorf("%s: 수동 작성일 %q: %w", pageID, edit.OriginalCreatedAt, err)
	}
	return edit.Title, &date, &order, nil
}

// StatusEdit는 사람이 status를 직접 정한 글이다.
//
// **status는 원래 덤프 분석이 정한다.** `notion-page-status.csv`가 블록 수를 보고
// 5개 미만이면 draft, 아니면 unlisted를 준다. 그 어림짐작이 틀리는 자리가 있다 —
// **본문은 짧지만 다른 글을 묶는 마디**가 그렇다. 공개 서버가 draft를 가리면
// 그런 마디가 사라지면서 밑에 매달린 글이 통째로 평평해진다.
//
// 여기 적으면 CSV 위에 얹는다. DB를 손으로 고치면 다음 `import -db`가 CSV 값으로
// 되돌리므로 안 된다 — BodyEdits와 같은 성질이다.
type StatusEdit struct {
	// NotionPageID는 status를 정할 글이다. posts의 멱등 키다.
	NotionPageID string
	// Status는 draft/unlisted/published 중 하나다.
	Status string
	Title  string // 사람이 읽으라고 적어두는 것. 대조에 쓰지 않는다.
	Why    string
}

var StatusEdits = []StatusEdit{
	// 지금은 비어 있다. 여기 있던 빅데이터 분석기사의 두 마디(개념정리·실전문제)는
	// 커리어 분류를 통째로 없애면서 DropPosts로 갔다.
}

// ApplyStatus는 CSV가 준 status 위에 사람이 정한 값을 얹는다.
func ApplyStatus(pageID, status string) string {
	for _, e := range StatusEdits {
		if e.NotionPageID == pageID {
			return e.Status
		}
	}
	return status
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
		SourceName: "최인렬 (Inryeol Choi)",
		Why: "사람이 만든 `소개` 분류와 이름만 다른 같은 층이다. 자기소개 글은 " +
			"PostMoves로 소개에 직접 붙였고, 프로젝트 글 여섯 건은 이미 /project로 " +
			"올라가 남는 것이 없다",
	},
	{
		SourceName: "프로젝트",
		Why:        "/project 밑에 같은 이름이라 프로젝트 > 프로젝트로 겹친다. 글은 PostMoves로 위로 올렸다",
	},
	{
		SourceName: "웹 프로그래밍",
		Why: "서버 & API와 클라이언트 & UI 두 갈래가 이 자리를 대신한다. 하위 분류 " +
			"다섯은 Moves로 두 갈래에 나눠 붙였고, 직속 글 열셋은 PostMoves로 " +
			"내려보내거나(입구 글은 같은 이름 분류의 표지로) 표지 한 편만 뺐다",
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
	{
		SourceName: "취업 준비",
		Why:        "커리어 분류를 통째로 없앤다. 글은 DropPosts로 뺐다",
	},
	{
		SourceName: "빅데이터 분석기사",
		Why:        "커리어 분류를 통째로 없앤다. 글은 DropPosts로 뺐다",
	},
	{
		SourceName: "기술면접 : 예상",
		Why:        "커리어 분류를 통째로 없앤다. 글은 DropPosts로 뺐다",
	},
	{
		SourceName: "취업박람회",
		Why:        "커리어 분류를 통째로 없앤다. 글은 DropPosts로 뺐다",
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

	// 웹 프로그래밍의 입구 글 다섯. PostMoves로 같은 이름의 하위 분류에 내려보내고
	// 그 분류의 표지로 삼는다. 들어가면 입구 본문과 세부 글을 한 화면에서 본다.
	{Slug: "django", NotionPageID: "6c8d3a3f-d398-4029-ab5c-92388eb194d1", Why: "Django 분류를 누르면 목록보다 소개가 먼저 보여야 한다"},
	{Slug: "spring", NotionPageID: "4c00e9ea-5f92-4a36-b851-cd1c306aac52", Why: "Spring 분류를 누르면 목록보다 소개가 먼저 보여야 한다"},
	{Slug: "node-js", NotionPageID: "b253d89d-0d82-459d-94a7-27a7dad05036", Why: "Node.js 분류를 누르면 목록보다 소개가 먼저 보여야 한다"},
	{Slug: "javascript", NotionPageID: "6f267d09-77fe-4cd9-aef5-2bdb47ba49de", Why: "Javascript 분류를 누르면 목록보다 소개가 먼저 보여야 한다"},
	{Slug: "react", NotionPageID: "4d9af5a1-cd10-419b-8c89-17a8b40a93f7", Why: "React 분류를 누르면 목록보다 소개가 먼저 보여야 한다"},

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
	{Slug: "머신러닝-기초이론", NotionPageID: "226b7998-bd88-4892-88aa-1227dc89b5f0", Why: "핸즈온 머신러닝 2 분류를 누르면 목록보다 소개가 먼저 보여야 한다"},

	// école 42 표지의 원형별 목차와 같은 이름의 하위 분류가 따로 보여 중복됐다.
	// 입구 글을 해당 분류의 표지로 삼으면 부모 목차는 분류로 바로 들어가고,
	// 하위 분류에서는 소개 본문과 세부 글을 한 화면에서 볼 수 있다.
	{Slug: "c", NotionPageID: "c7bcee75-28c5-4945-b5c3-f8e24e79e5e7", Why: "école 42의 C 입구 글을 C 분류 표지로 쓴다"},
	{Slug: "cpp-part-1", NotionPageID: "5c84aeb4-c3d9-4341-a66b-acc71487be94", Why: "école 42의 Cpp part.1 입구 글을 해당 분류 표지로 쓴다"},
	{Slug: "netpractice", NotionPageID: "0016e85a-614f-426c-ae62-f46427a7b719", Why: "école 42의 Netpractice 입구 글을 해당 분류 표지로 쓴다"},
	{Slug: "shell", NotionPageID: "5b239f6c-32bb-4a4a-921b-00db117abc3d", Why: "école 42의 Shell 입구 글을 해당 분류 표지로 쓴다"},
	{Slug: "born2beroot", NotionPageID: "c4e6e521-2c48-460b-91d8-54d8452f2096", Why: "école 42의 born2beroot 입구 글을 해당 분류 표지로 쓴다"},
	{Slug: "cub3d", NotionPageID: "92ab7921-feb3-4604-b9ce-171a3b8a4629", Why: "école 42의 cub3D 입구 글을 해당 분류 표지로 쓴다"},
	{Slug: "exam02", NotionPageID: "90a0bf6c-e299-4158-8e13-d367d96298e5", Why: "école 42의 exam02 입구 글을 해당 분류 표지로 쓴다"},
	{Slug: "exam03", NotionPageID: "96b38479-74a3-4983-a9d9-c5530a9b94c8", Why: "école 42의 exam03 입구 글을 해당 분류 표지로 쓴다"},
	{Slug: "fdf-fil-de-fer", NotionPageID: "f6ceedec-0c40-4a30-8b97-05c05868e6f7", Why: "école 42의 fdf 입구 글을 해당 분류 표지로 쓴다"},
	{Slug: "ft-irc-server", NotionPageID: "97c6e452-91e6-4c19-9769-802b6be9a982", Why: "école 42의 ft_irc 입구 글을 해당 분류 표지로 쓴다"},
	{Slug: "ft-printf", NotionPageID: "62d53e5d-77fb-4a4e-b789-9a59ef3a70e4", Why: "école 42의 ft_printf 입구 글을 해당 분류 표지로 쓴다"},
	{Slug: "get-next-line", NotionPageID: "38315c40-5daf-4387-bae3-bde27387b43f", Why: "école 42의 get_next_line 입구 글을 해당 분류 표지로 쓴다"},
	{Slug: "inception", NotionPageID: "4786e824-c744-4c32-8886-729e8e8c9bc6", Why: "école 42의 inception 입구 글을 해당 분류 표지로 쓴다"},
	{Slug: "libft", NotionPageID: "dcbbd10e-b50a-4a1d-8a4a-237426aa7249", Why: "école 42의 libft 입구 글을 해당 분류 표지로 쓴다"},
	{Slug: "minishell", NotionPageID: "518ea6e1-306c-4cfa-8b3e-6ac182c16e14", Why: "école 42의 minishell 입구 글을 해당 분류 표지로 쓴다"},
	{Slug: "philosopher", NotionPageID: "cdf4ecb8-ecdb-4cda-9dbc-d74722213449", Why: "école 42의 philosopher 입구 글을 해당 분류 표지로 쓴다"},
	{Slug: "pipex", NotionPageID: "250631b1-6ac8-4e58-8dc2-eec4ecaca254", Why: "école 42의 pipex 입구 글을 해당 분류 표지로 쓴다"},
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

	// `R언어 : 시각화` 인라인 데이터베이스의 행 다섯 건. 전부 본문이 0바이트인
	// draft다. 같은 내용을 실제로 쓴 글은 한 층 위의 `기본 시각화1`(고수준 함수),
	// `기본 시각화2`(저수준 함수), `그래프 꾸미기`에 있어서 목록에 두 벌로 보였다.
	// 다섯을 다 빼면 그 데이터베이스에 남는 행이 없어지므로 R 표지의 묶음 링크도
	// BodyEdits로 함께 없앤다.
	{NotionPageID: "d8902603-ff1f-4100-875f-e05b676dc864", Title: "ggplot", Why: "R언어 : 시각화 밑의 빈 껍데기(0바이트 draft)"},
	{NotionPageID: "b8085cd9-6538-4bc8-9a05-e6321289b175", Title: "고수준 함수", Why: "R언어 : 시각화 밑의 빈 껍데기(0바이트 draft). 알맹이는 기본 시각화1에 있다"},
	{NotionPageID: "63a80b68-62c5-45af-8efb-f9a2169732a6", Title: "저수준 함수", Why: "R언어 : 시각화 밑의 빈 껍데기(0바이트 draft). 알맹이는 기본 시각화2에 있다"},
	{NotionPageID: "5ea820b2-d90b-4e85-bca1-c1e34049f428", Title: "그래프 그리기", Why: "R언어 : 시각화 밑의 빈 껍데기(0바이트 draft)"},
	{NotionPageID: "2ad16805-6163-4314-a553-971ee01038c5", Title: "그래프 꾸미기", Why: "R언어 : 시각화 밑의 빈 껍데기(0바이트 draft). 같은 제목의 알맹이가 한 층 위에 따로 있다"},

	// `파이썬 : 데이터 시각화` 인라인 데이터베이스의 제목 없는 행 세 건.
	// 제목도 본문도 없어서 목록에서 `(제목 없음)`으로만 보이고 눌러도 볼 것이 없다.
	// 같은 데이터베이스의 엑셀 글 네 건은 내용이 있어 그대로 둔다.
	// 웹 프로그래밍 표지 글. 본문이 기초 이론 / 프레임워크 / 로그인 / 웹소켓
	// 링크 목록과 내용 없는 `## 실전 프로젝트` 제목뿐이라, 그 목차가 하던 일을
	// 서버 & API와 클라이언트 & UI 두 갈래가 그대로 대신한다.
	{NotionPageID: "9e9d8b9b-cfe9-46cd-9b1c-9207e79dde46", Title: "웹 프로그래밍", Why: "두 갈래로 가르면서 이 목차 글이 할 일이 없어졌다"},

	{NotionPageID: "02718d05-11b3-4c7a-9b29-f874cd586a98", Title: "(제목 없음)", Why: "파이썬 : 데이터 시각화 밑의 빈 페이지다"},
	{NotionPageID: "540f7a7e-b96b-4713-9bc6-03f755a86583", Title: "(제목 없음)", Why: "파이썬 : 데이터 시각화 밑의 빈 페이지다"},
	{NotionPageID: "c2c5b971-7a1a-4eee-a026-94a5bda068a2", Title: "(제목 없음)", Why: "파이썬 : 데이터 시각화 밑의 빈 페이지다"},

	// 커리어 50편. 분류를 통째로 없애기로 했다(취업 준비 1, 빅데이터
	// 분석기사 42, 기술면접 : 예상 6, 취업박람회 1). 노션 덤프에는 그대로
	// 남아 있으므로 되돌리려면 이 묶음을 지우면 된다.
	{
		NotionPageID: "72bccec8-bdd2-4e23-92ad-8d017c7f4ebf",
		Title:        "기본질문 / 인성질문",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "fcbf7afd-019e-440a-b81a-c412e6aaf127",
		Title:        "질문 : Docker and Cloud",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "37acc14d-b91b-4da3-a883-09a1f26985cc",
		Title:        "질문 : FT_IRC & Network",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "558ca531-1859-4a13-82a2-e870fe6e1238",
		Title:        "질문 : NLP with BERT and GPT",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "1150901b-87f1-8045-ac4d-e3b8245254fa",
		Title:        "질문 : 심심조각",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "4c1fa3c8-3809-42ce-961d-67128a2d2d46",
		Title:        "질문 : 필로소퍼 / 미니쉘",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "e26d0dde-3713-4a9d-830f-03bd3ff703d8",
		Title:        "(제목 없음)",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "e5c22877-d326-4bbb-85f8-e5dd77746338",
		Title:        "(제목 없음)",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "006faf85-e3c8-4454-b870-1e46183f5685",
		Title:        "1-1 : 빅데이터의 개념 및 활용",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "d489f2e0-062a-4136-a650-a4ec36bc225e",
		Title:        "1-1. 데이터 정제",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "5785cbb2-c7bf-4d7b-94c4-c981ad78655f",
		Title:        "1-1. 분석모형 평가",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "c158c39f-310b-4930-9708-bc1fd27346a1",
		Title:        "1-1. 분석절차 수립",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "4518ab2e-913e-41be-9266-19c0bf7aed88",
		Title:        "1-2 : 빅데이터 기술 및 제도",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "2491504a-f8ea-48d0-b695-61675cb79dd9",
		Title:        "1-2. 분석 변수 처리",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "5b86e6f7-dc71-4459-b221-597a9550933c",
		Title:        "1-2. 분석모형 개선",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "ab114ab2-c170-43f2-bbae-94728d076829",
		Title:        "1-2. 분석환경 구축",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "324b5cfc-891f-44a3-a87f-48223d36a3da",
		Title:        "2-1 : 분석방안 수립  ",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "c14ff59b-3afd-4e2c-aa6a-00882b48ca20",
		Title:        "2-1. 데이터 탐색의 기초",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "2ef8da5b-adb5-4c2b-afae-00f7d4bcaf11",
		Title:        "2-1. 분석결과 해석",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "29bf03af-2eb0-4183-8c15-fea550ef7185",
		Title:        "2-1. 분석기법",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "3df9447a-5fea-484d-9cb5-e105981fa2c4",
		Title:        "2-2 : 분석작업 계획 수립",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "18c5652d-9a36-4cc1-ba8a-1f639e11b661",
		Title:        "2-2. 고급 데이터 탐색",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "ac07f8e4-7a39-4e31-bacb-1d811ccf80df",
		Title:        "2-2. 고급 분석기법",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "6b93ac1e-834d-4b93-88d0-2bf01a791e81",
		Title:        "2-2. 분석결과 시각화",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "4ef6b461-19d6-4a1e-abaa-feb7a538c198",
		Title:        "2-3. 분석결과 활용",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "227a751c-d030-4925-b8b2-2047cb459f7a",
		Title:        "3-1 : 데이터 수집 및 저장",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "e1df6b62-dbeb-412b-8af4-def773643dca",
		Title:        "3-1. 기술통계",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "fedb6543-b712-49dc-aae9-075b12f130a3",
		Title:        "3-2 : 데이터 적재 및 저장",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "622704eb-ebe0-4640-b9ef-350fe2017c03",
		Title:        "3-2. 추론통계",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "e6fdf9a4-1884-49ce-8c0c-cf01721e1bb0",
		Title:        "[Ch1] 데이터 전처리",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "62f18a53-f71e-4a3f-9586-e66b3aa12300",
		Title:        "[Ch1] 분석 모형 설계",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "60c2343e-c340-4f56-a22a-c01d3600a11f",
		Title:        "[Ch1] 빅데이터의 이해",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "6e7f4758-c5c8-4fa9-889f-3da16623b1e0",
		Title:        "[Ch2] 데이터 분석 계획",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "8a9bc879-bc85-4234-9241-40f2ff943d53",
		Title:        "[Ch2] 데이터 탐색",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "ece9a902-068c-41eb-a390-21fc53fd78e4",
		Title:        "[Ch2] 분석기법 적용",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "a31318fb-7ad9-4c8e-a1d2-79c50f97bb6a",
		Title:        "[Ch3] 데이터 수집 및 저장 계획",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "684ab7ff-6284-468c-9d1b-7d8001ec3256",
		Title:        "[Ch3] 통계기법 이해",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "250c89b8-b6c1-4353-ba0a-f68ef44f6e07",
		Title:        "개념정리",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "a2fde93b-dbaf-49b3-8364-d61e84410195",
		Title:        "기출문제 4회",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "89d46c33-3bc9-44d2-8682-09f4a1d33e1a",
		Title:        "기출문제 5회",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "1f11f1c3-ae29-4e5b-a352-b9c323e4ba89",
		Title:        "기출문제 6회",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "613e1e48-8c37-4fb1-9229-18e46168d1e0",
		Title:        "기출문제 7회",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "a2f4b890-1f50-4390-acfc-871e612dce31",
		Title:        "빅데이터 분석기사",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "236908fd-251a-4f13-8520-d01eaa04e1f1",
		Title:        "실전문제",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "742c3a5e-d35c-449f-81f3-e7540de1dd75",
		Title:        "암기사항 (1과목)",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "d934873b-1bc1-4936-940f-6ee5c8c027d1",
		Title:        "암기사항 (2과목)",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "08c2ce51-1c61-4928-bc0f-124327a5a59d",
		Title:        "암기사항 (3과목)",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "792c4300-7446-4b1e-b18b-c1337a0e8225",
		Title:        "암기사항 (4과목)",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "9f697d2b-c63e-49f2-ac01-658c0af22a5c",
		Title:        "취업 준비",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
	{
		NotionPageID: "00dca451-0f12-4d78-9304-8f7f30d3b7a9",
		Title:        "AWS 특강",
		Why:          "커리어 분류를 통째로 없앤다. 더 이상 둘 자리가 아니다",
	},
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

// BodyEdit는 사람이 본문에서 지우거나 바꾸기로 한 줄이다.
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
	// Remove는 지우거나 바꿀 원본 줄이다. 변환 결과에 **그 줄 전체가 정확히**
	// 있어야 한다.
	// 조각이 아니라 줄 단위인 이유: 문장 중간을 지우면 앞뒤가 어떻게 이어지는지
	// 표만 보고는 알 수 없다.
	Remove string
	// Replace가 비어 있으면 Remove 줄을 지우고, 값이 있으면 그 줄로 바꾼다.
	Replace string
	Title   string // 사람이 읽으라고 적어두는 것. 대조에 쓰지 않는다.
	Why     string
}

var BodyEdits = []BodyEdit{
	{
		NotionPageID: "660e3d79-427d-40f7-b98a-6f8be0a5f787",
		Remove:       "[수업 : 통수 & 선계](/p/5f629e77-097e-40c4-a7d8-bbaa492c782f)",
		Title:        "최적화이론",
		Why:          "옛 수업 묶음은 없애고 선형대수 11편과 최적화 5편을 각 분류에서 직접 노출한다",
	},
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
		NotionPageID: "bd74e69e-56fa-4d0b-a4f5-4239749a8566",
		Remove:       "[R언어 : 시각화](/p/54d12411-e657-465e-b236-7a5e41b9e3eb)",
		Title:        "R",
		Why: "그 데이터베이스의 행이던 다섯 건을 DropPosts로 뺐다. " +
			"이제 눌러도 빈 목록이라 링크째 없앤다",
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
	{
		NotionPageID: "226b7998-bd88-4892-88aa-1227dc89b5f0",
		Remove:       "- 통계와 수학에 관한 지식이 많이 필요하다. 모르면 [여기, 저기](/0d24cebd0bc1496b99b64885cb5be2a6) [클릭!](/5d2a5e480d854fc594d3280cbeef87ee)",
		Replace:      "- 통계와 수학에 관한 지식이 많이 필요하다. 모르면 [클릭!](/5d2a5e480d854fc594d3280cbeef87ee)",
		Title:        "핸즈온 머신러닝 2",
		Why:          "404가 나는 여기, 저기 링크를 없애고 정상적인 데이터 & 수리 링크 하나만 남긴다",
	},
	{
		NotionPageID: "59d18904-4ed5-4f0a-ba61-17ec86d9fc7b",
		Remove:       "[분포별 가능도함수 (1)](/p/df333507-a679-4beb-b165-285ae3bf42fc)",
		Replace:      "[분포별 가능도함수](/p/df333507-a679-4beb-b165-285ae3bf42fc)",
		Title:        "수리통계2",
		Why:          "참고자료 제목 끝의 불필요한 (1)을 뺀다",
	},
	{
		NotionPageID: "59d18904-4ed5-4f0a-ba61-17ec86d9fc7b",
		Remove:       "[확률함수와 커널 (1)](/p/805825dd-084e-4612-9465-a2054a0d2004)",
		Replace:      "[확률함수와 커널](/p/805825dd-084e-4612-9465-a2054a0d2004)",
		Title:        "수리통계2",
		Why:          "참고자료 제목 끝의 불필요한 (1)을 뺀다",
	},
	{
		NotionPageID: "59d18904-4ed5-4f0a-ba61-17ec86d9fc7b",
		Remove:       "[수리통계2 - 과제 (1)](/p/d9fe0a39-89cd-48b6-84ee-0efaa78cf67b)",
		Replace:      "[수리통계2 - 과제](/p/d9fe0a39-89cd-48b6-84ee-0efaa78cf67b)",
		Title:        "수리통계2",
		Why:          "참고자료 제목 끝의 불필요한 (1)을 뺀다",
	},
	{
		NotionPageID: "59d18904-4ed5-4f0a-ba61-17ec86d9fc7b",
		Remove:       "[수리통계2 - 시험 (1)](/p/1f3d0731-e367-4d0d-8239-94d92d6d02d5)",
		Replace:      "[수리통계2 - 시험](/p/1f3d0731-e367-4d0d-8239-94d92d6d02d5)",
		Title:        "수리통계2",
		Why:          "참고자료 제목 끝의 불필요한 (1)을 뺀다",
	},
	{
		NotionPageID: "5d2a5e48-0d85-4fc5-94d3-280cbeef87ee",
		Remove:       "[빅데이터 분석기사](/p/a2f4b890-1f50-4390-acfc-871e612dce31)",
		Title:        "수학 & 통계",
		Why: "커리어 분류를 통째로 없애면서 그 표지 글도 뺐다. 남겨두면 대상이 " +
			"posts에 없는 slug가 되어 눌러도 404이고, 렌더러가 그걸 노션 인라인 " +
			"데이터베이스로 알아봐 엉뚱한 목록을 펼 수 있다",
	},

	// Mermaid 설명 글의 예제 넷. 글쓴이가 노션에서 **인라인 코드 하나에 여러
	// 줄을 넣고** 그 안에 `<div class="mermaid">`를 적어뒀다. 그 시절 블로그의
	// mermaid 렌더러가 그 div를 찾아 그렸던 것이라 원본에서는 뜻이 있었지만,
	// 여기서는 그냥 깨진다 — CommonMark의 코드 스팬은 이 모양으로 짝이 안 맞고,
	// 결국 백틱 하나가 글자로 남고 도형 소스가 문단으로 흘러나온다. 화면에서
	// 실제로 그렇게 보였다.
	//
	// **울타리 코드 블록으로 바꾼다.** 여는 줄과 닫는 줄만 갈아끼우면 사이의
	// 소스는 손대지 않아도 되고, ```mermaid는 이 사이트가 다이어그램으로
	// 그리는 형태다(static/mermaid.js). 줄 단위로만 고친다는 원칙에 그대로 맞는다.
	//
	// **같은 줄이 두 번 나와서 항목도 두 벌이다.** replaceLine은 첫 번째 것만
	// 바꾸므로, 두 번 적으면 앞엣것이 바뀐 뒤 뒤엣것이 걸린다.
	{
		NotionPageID: "8f4d7fa8-c2ee-41f8-910c-63df6c33b5ec",
		Remove:       "`<div class=\"mermaid\">",
		Replace:      "```mermaid",
		Title:        "Mermaid",
		Why:          "인라인 코드에 담긴 예제라 백틱이 글자로 새어 나왔다. 울타리 코드 블록이라야 다이어그램으로 그려진다",
	},
	{
		NotionPageID: "8f4d7fa8-c2ee-41f8-910c-63df6c33b5ec",
		Remove:       "</div>`",
		Replace:      "```",
		Title:        "Mermaid",
		Why:          "위 예제를 닫는 줄",
	},
	{
		NotionPageID: "8f4d7fa8-c2ee-41f8-910c-63df6c33b5ec",
		Remove:       "`<div class=\"mermaid\">",
		Replace:      "```mermaid",
		Title:        "Mermaid",
		Why:          "둘째 예제. 같은 줄이 두 번 나오므로 항목도 두 벌이다",
	},
	{
		NotionPageID: "8f4d7fa8-c2ee-41f8-910c-63df6c33b5ec",
		Remove:       "</div>`",
		Replace:      "```",
		Title:        "Mermaid",
		Why:          "둘째 예제를 닫는 줄",
	},
}

// ApplyBodyEdits는 한 페이지의 변환 결과에 BodyEdits와 BodyAppends를 적용한다.
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
		var next string
		var ok bool
		if e.Replace == "" {
			next, ok = removeLine(out, e.Remove)
		} else {
			next, ok = replaceLine(out, e.Remove, e.Replace)
		}
		if !ok {
			return "", fmt.Errorf("본문에서 고칠 줄을 못 찾았다 (%s %q): %q",
				e.NotionPageID, e.Title, e.Remove)
		}
		out = next
	}
	for _, e := range BodyAppends {
		if e.NotionPageID != pageID {
			continue
		}
		next, err := appendBody(out, e.Marker, e.Markdown)
		if err != nil {
			return "", fmt.Errorf("본문에 덧붙일 내용을 적용하지 못했다 (%s %q): %w",
				e.NotionPageID, e.Title, err)
		}
		out = next
	}
	return out, nil
}

// appendBody는 본문 끝에 마크다운을 한 번만 덧붙인다. 같은 결과에 다시 적용해도
// 중복되지 않아야 하고, marker만 있는데 내용이 다르면 사람이 고쳐둔 것을 덮지 않고
// 에러로 멈춘다.
func appendBody(body, marker, markdown string) (string, error) {
	appendix := strings.TrimSpace(markdown)
	if marker == "" || appendix == "" {
		return "", fmt.Errorf("marker와 markdown은 비어 있을 수 없다")
	}
	if strings.Contains(body, marker) {
		if strings.HasSuffix(strings.TrimSpace(body), appendix) {
			return body, nil
		}
		return "", fmt.Errorf("marker %q가 이미 있지만 덧붙일 내용과 다르다", marker)
	}
	base := strings.TrimRight(body, "\n")
	if base == "" {
		return appendix + "\n", nil
	}
	return base + "\n\n" + appendix + "\n", nil
}

// replaceLine은 줄 하나를 정확히 찾아 바꾼다. 첫 번째 것만 바꾼다.
func replaceLine(body, target, replacement string) (string, bool) {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if strings.TrimRight(line, " \t") != target {
			continue
		}
		lines[i] = replacement
		return strings.Join(lines, "\n"), true
	}
	return body, false
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
	out := make(map[string]bool, len(BodyEdits)+len(BodyAppends))
	for _, e := range BodyEdits {
		out[e.NotionPageID] = true
	}
	for _, e := range BodyAppends {
		out[e.NotionPageID] = true
	}
	return out
}
