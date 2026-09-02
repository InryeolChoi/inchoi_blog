package curation

// virtualizationCoverEdits는 `가상화기술` 표지에서 `## Cloud` 절을 걷는다.
//
// 두 가지가 한 번에 걸린 자리다.
//
//  1. **`Untitled` 링크는 눌러도 404다.** 가리키는 것이 posts에 행이 없는
//     인라인 데이터베이스라, 이름이 `Untitled`여서 경로와 짝지을 수도 없다
//     (CLAUDE.md "남은 일"의 열아홉 개 중 하나다). 죽은 링크를 노션 형태로
//     남기는 것은 relink의 판단이지만, **화면에 글자로 나오는 자리**는 다르다.
//  2. `클라우드 탐구생활`은 `tooling`으로 옮겼다(`Moves`). 절이 남아 있으면
//     떠난 분류의 목록을 이 화면이 계속 펼친다.
//
// 절 제목까지 함께 뺀다. 남기면 아무것도 없는 `## Cloud`만 서 있게 된다.
var virtualizationCoverEdits = []BodyEdit{
	{
		NotionPageID: "ec9aa848-92cf-41ed-b1c4-c55f8dc47be3",
		Remove:       "## Cloud",
		Title:        "가상화기술",
		Why:          "클라우드를 tooling으로 옮겼다. 빈 절 제목만 남는다",
	},
	{
		NotionPageID: "ec9aa848-92cf-41ed-b1c4-c55f8dc47be3",
		Remove:       "[클라우드 탐구생활](/p/c435202a-12db-4e64-b7e5-be4ff439a2df)",
		Title:        "가상화기술",
		Why:          "그 목록은 이제 tooling 밑의 제 분류가 보여준다",
	},
	{
		NotionPageID: "ec9aa848-92cf-41ed-b1c4-c55f8dc47be3",
		Remove:       "[Untitled](/p/73ec489b-b05d-4e76-ace2-81fe04003c8c)",
		Title:        "가상화기술",
		Why:          "이름 없는 인라인 데이터베이스라 짝지을 글이 없다. 눌러도 404인 링크가 화면에 글자로 남았다",
	},
}
