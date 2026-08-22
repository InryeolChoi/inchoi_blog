package main

import "testing"

func TestApplyManualOrdersOverridesNotionOrder(t *testing.T) {
	const vectorSpace = "8ef5464f-7a44-41c5-9850-f9804ff9cf2f"
	in := []assignment{
		{pageID: vectorSpace, title: "벡터공간", order: 99, src: srcCreated, tied: true, groupID: "notion-db"},
		{pageID: "untouched", title: "그대로", order: 2, src: srcChildPage, groupID: "parent"},
	}
	out := applyManualOrders(in)
	if got := out[0]; got.title != "4. 벡터공간" || got.order != 3 || got.src != srcManual || got.tied {
		t.Errorf("수동 순서가 적용되지 않았다: %+v", got)
	}
	if got := out[1]; got != in[1] {
		t.Errorf("대상 아닌 글이 바뀌었다: got %+v want %+v", got, in[1])
	}
	if in[0].order != 99 {
		t.Error("입력 슬라이스를 직접 바꿨다")
	}
}

func TestApplyManualOrdersMarksFragmentedSourceGroup(t *testing.T) {
	const vectorSpace = "8ef5464f-7a44-41c5-9850-f9804ff9cf2f"
	out := applyManualOrders([]assignment{
		{pageID: vectorSpace, groupID: "mixed-notion-db", src: srcCreated},
		{pageID: "optimization-post", groupID: "mixed-notion-db", src: srcCreated, order: 12},
		{pageID: "other", groupID: "other-db", src: srcCreated},
	})
	if !out[1].skipGap {
		t.Error("수동 형제가 빠진 원본 묶음을 빈틈 검사 대상에서 빼지 않았다")
	}
	if out[2].skipGap {
		t.Error("관계없는 묶음까지 빈틈 검사를 건너뛴다")
	}
}
