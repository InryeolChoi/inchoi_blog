package curation

// projectsCoverEdits는 옛 `Projects` 표지 글에서 목차 두 벌을 걷는다.
//
// 그 글은 절 두 개(`# where42`, `# 심심조각`) 아래에 글 링크 아홉을 늘어놓은
// 색인이었다. 두 절이 각자 제 분류가 되면서(`cmd/regroup`의 subs) 그 색인이
// 하는 일을 분류 두 개가 그대로 대신하므로, 남겨두면 같은 목록이 두 벌이 된다.
//
// **남는 것은 심심조각 서버를 옮기는 메모 하나다.** LightSail 컨테이너 안의
// AI·Nginx·스프링이라 where42(Xcode 앱)의 것일 수 없다 — 그래서 이 글이
// 심심조각의 표지가 된다(`Covers`).
//
// `> 코드 분석`은 두 절에 한 번씩 나온다. `removeLine`이 첫 번째 것만
// 지우므로 두 번 적는다 — 표의 차례가 곧 의미다.
var projectsCoverEdits = projectsCoverLines()

func projectsCoverLines() []BodyEdit {
	const page = "10d0901b-87f1-80b2-881f-effe3c29eff5"
	lines := []string{
		"# where42",
		"> 코드 분석",
		"[코드분석 (1) : 기본](/p/a82c5952-ce31-4670-a56e-7fbcccb5199e)",
		"[코드분석 (2) : View](/p/1050901b-87f1-805a-8138-c284ec864dbc)",
		"[코드분석 (3) : View](/p/1050901b-87f1-809c-bdad-ecf1a4fc9ece)",
		"# 심심조각",
		"> 코드 분석",
		"[코드분석 (1)](/p/10e0901b-87f1-800d-a76a-c7aea7486da0)",
		"[코드분석 (2) : AI](/p/10f0901b-87f1-804b-8d57-ed2c843a1475)",
		"[코드분석 (3) : 다이어리](/p/10f0901b-87f1-80bf-b23b-d5d6ede71715)",
		"[코드분석 (4) : 레포트](/p/1130901b-87f1-8072-abc4-c96c2cda3df1)",
		"[코드분석 (5) : 사용자](/p/1130901b-87f1-80d4-9e49-d27e6297547f)",
		"[코드분석 (6) : filter](/p/1140901b-87f1-8093-a12a-e364f769afbc)",
	}
	out := make([]BodyEdit, 0, len(lines))
	for _, line := range lines {
		out = append(out, BodyEdit{
			NotionPageID: page,
			Remove:       line,
			Title:        "Projects",
			Why:          "where42와 심심조각이 제 분류를 갖는다. 표지의 색인은 그 목록과 두 벌이 된다",
		})
	}
	return out
}
