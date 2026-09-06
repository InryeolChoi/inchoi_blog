package curation

// osStubEdits는 운영체제 `Part 1`·`Part 2` 표지에서 빈 stub으로 가는 안내
// 링크를 걷는다.
//
// `1장 연습문제`·`2장 연습문제`·`4장 연습문제`는 본문이 0바이트인 draft라
// `DropPosts`로 뺐다(2026-09-06). 표지 본문에 남겨두면 그 글이 posts에 없는
// slug가 되어, 렌더러가 노션 인라인 데이터베이스로 알아보고 엉뚱한 목록을
// 펼 수 있다 — 커리어를 뺄 때와 같은 자리다.
var osStubEdits = []BodyEdit{
	{
		NotionPageID: "33c90690-98c3-432f-ac34-2557d6df5747",
		Remove:       "[1장 연습문제](/p/1e5e9a22-76f7-4421-a4c4-bdacbefad57b)",
		Title:        "Part 1 : 운영체제란?",
		Why:          "가리키는 글이 DropPosts로 빠졌다. 본문이 0바이트인 draft였다",
	},
	{
		NotionPageID: "33c90690-98c3-432f-ac34-2557d6df5747",
		Remove:       "[2장 연습문제](/p/446f5151-2c25-450a-bcbd-62b08fb64bd4)",
		Title:        "Part 1 : 운영체제란?",
		Why:          "가리키는 글이 DropPosts로 빠졌다. 본문이 0바이트인 draft였다",
	},
	{
		// Part 2와 달리 이 절의 두 링크가 전부 빠져서 제목만 남는다.
		// 가상화기술의 `## Cloud`와 같은 자리라 제목째 없앤다.
		NotionPageID: "33c90690-98c3-432f-ac34-2557d6df5747",
		Remove:       "### 연습문제",
		Title:        "Part 1 : 운영체제란?",
		Why:          "그 밑 링크 둘을 모두 걷어 빈 절 제목만 남았다",
	},
	{
		NotionPageID: "aa769806-17eb-4db9-ade2-ab7a237e467c",
		Remove:       "[4장 연습문제](/p/1b0f487f-35fb-4818-bcc7-0abbabbc23ae)",
		Title:        "Part 2 : 프로세스 관리",
		Why:          "가리키는 글이 DropPosts로 빠졌다. 본문이 0바이트인 draft였다",
	},
}
