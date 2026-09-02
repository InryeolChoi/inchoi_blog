package curation

// BodyEdits는 본문에서 지우거나 바꿀 줄의 **최종 표**다.
//
// # 왜 묶음을 나눠서 합치나
//
// 한 글의 손질이 수백 줄이 되는 경우가 생겼다(다항분포는 빈 절 셋을 통째로
// 채운다). 그런 것을 기본 표에 그대로 넣으면 다른 줄 하나를 고칠 때마다 그
// 덩어리를 스크롤해서 지나가야 한다. 글별로 파일을 나누고 여기서 합친다 —
// `BodyAppends`·`PostMetadataEdits`와 같은 꼴이다.
//
// **순서가 곧 적용 순서다.** `ApplyBodyEdits`가 이 슬라이스를 위에서부터
// 훑고 `replaceLine`/`removeLine`은 **첫 번째 것만** 건드리므로, 같은 글
// 안에서 같은 줄을 여러 번 고치는 표는 제 차례를 지켜야 한다.
var BodyEdits = concatBodyEdits(
	baseBodyEdits,
	multinomialEdits,
	discreteMathEdits,
	networkEdits,
	databaseTOCEdits,
	graphAlgorithmEdits,
)

// concatBodyEdits는 묶음들을 순서대로 이어 붙인다.
//
// **여기서는 (글, 줄) 중복을 막지 않는다.** `BodyAppends`와 달리 같은 줄을
// 일부러 두 번 고치는 것이 정상적인 쓰임이기 때문이다 — 똑같이 생긴 줄이
// 둘일 때 앞엣것을 먼저 바꿔 뒤엣것을 첫 번째로 만드는 수를 쓴다
// (multinomial.go 참고).
func concatBodyEdits(groups ...[]BodyEdit) []BodyEdit {
	var out []BodyEdit
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}
