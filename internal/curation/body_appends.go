package curation

// BodyAppends는 본문 끝에 덧붙일 마크다운의 **최종 표**다.
//
// # 왜 묶음을 나눠서 합치나
//
// 해답 하나가 수백 줄이라 한 파일에 다 넣으면 다른 표를 고칠 때마다 그
// 덩어리를 스크롤해서 지나가야 한다. 글별로 파일을 나누고 여기서 합친다 —
// `PostMetadataEdits`가 `concatMetadataEdits`로 묶음을 합치는 것과 같은 꼴이다.
//
// **순서가 곧 덧붙는 순서다.** `ApplyBodyEdits`가 이 슬라이스를 위에서부터
// 훑으며 `appendBody`를 부르므로, 같은 글의 항목은 본문에 놓일 순서대로 적는다.
var BodyAppends = concatBodyAppends(
	probabilityProcessAppends,
	mathstat2ExamAppends,
)

// concatBodyAppends는 묶음들을 순서대로 이어 붙인다.
//
// **같은 (글, marker) 짝이 두 번 나오면 멈춘다.** 그러면 `appendBody`가 두
// 번째 것을 "marker가 이미 있는데 내용이 다르다"로 잡아 이관 전체가 실패하는데,
// 그때 나오는 말로는 표가 겹쳤다는 것을 알 수 없다. 여기서 먼저 본다.
func concatBodyAppends(groups ...[]BodyAppend) []BodyAppend {
	var out []BodyAppend
	seen := map[[2]string]bool{}
	for _, g := range groups {
		for _, a := range g {
			key := [2]string{a.NotionPageID, a.Marker}
			if seen[key] {
				panic("curation: 같은 글에 같은 marker가 두 번 있다: " + a.NotionPageID + " " + a.Marker)
			}
			seen[key] = true
			out = append(out, a)
		}
	}
	return out
}
