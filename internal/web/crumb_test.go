package web

import "testing"

// 경로는 카테고리 사슬 뒤에 상위 글 사슬을 잇는다. 그런데 **분류의 표지 글은
// 그 분류와 이름이 같은 경우가 많다**(수리통계1·2, 탐색적 자료분석, 확률과정론
// — 61편). 그대로 이으면 `… / 수리통계2 / 수리통계2`가 된다.
func TestCrumbsDropAncestorRepeatingCategoryName(t *testing.T) {
	trail := []Category{{Name: "데이터 & 수리", Slug: "data-math"}, {Name: "수리통계2", Slug: "수리통계2"}}
	got, _ := crumbs(trail)
	ancestors := []PostSummary{{Title: "수리통계2", Slug: "cover"}, {Title: "1. 통계량", Slug: "stat"}}

	out := appendPostCrumbs(got, ancestors)
	var names []string
	for _, c := range out {
		names = append(names, c.Name)
	}
	if len(names) != 3 {
		t.Fatalf("경로가 %v다 (want 3칸)", names)
	}
	if names[1] != "수리통계2" || names[2] != "1. 통계량" {
		t.Errorf("경로가 %v다", names)
	}
}

// 이름이 다른 조상은 그대로 이어야 한다. 빼면 그 글이 어디쯤인지 알 수 없다.
func TestCrumbsKeepDifferentAncestors(t *testing.T) {
	trail := []Category{{Name: "네트워크", Slug: "네트워크"}}
	got, _ := crumbs(trail)
	out := appendPostCrumbs(got, []PostSummary{{Title: "HTTP 완벽 가이드", Slug: "http"}})

	if len(out) != 2 || out[1].Name != "HTTP 완벽 가이드" {
		t.Errorf("이름이 다른 조상을 뺐다: %+v", out)
	}
	if out[1].URL != "/p/http" {
		t.Errorf("조상 링크가 %q다", out[1].URL)
	}
}
