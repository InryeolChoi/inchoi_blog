package web

import "testing"

func TestEcoleBundlesSplitsExamsFromProjects(t *testing.T) {
	children := []Category{
		{Name: "libft", Slug: "libft", PostCount: 3},
		{Name: "exam02", Slug: "exam02", PostCount: 2},
	}
	posts := []PostSummary{
		{Title: "push_swap", Slug: "a"},
		{Title: "exam05", Slug: "b"},
	}
	got := bundlesFor("école-42", children, posts, "/project/x")
	if len(got) != 2 {
		t.Fatalf("상자 2개를 바랐는데 %d개다", len(got))
	}
	if got[0].Name != "프로젝트" || got[1].Name != "Exam" {
		t.Fatalf("이름과 순서가 다르다: %q, %q", got[0].Name, got[1].Name)
	}

	// **분류와 글이 한 상자에 섞여야 한다.** 갈린 것이 바로 그 두 목록이었다.
	names := func(b Bundle) []string {
		var out []string
		for _, it := range b.Items {
			out = append(out, it.Name)
		}
		return out
	}
	if want := []string{"libft", "push_swap"}; !eqStrings(names(got[0]), want) {
		t.Errorf("프로젝트 = %v, 바란 것 %v", names(got[0]), want)
	}
	if want := []string{"exam02", "exam05"}; !eqStrings(names(got[1]), want) {
		t.Errorf("Exam = %v, 바란 것 %v", names(got[1]), want)
	}

	// 분류로 가는 줄만 글 수를 찍는다. 글 한 편에는 셀 것이 없다.
	if got[0].Items[0].Count != 3 || got[0].Items[1].Count != 0 {
		t.Errorf("글 수가 다르다: %+v", got[0].Items)
	}
	if got[0].Items[0].URL != "/project/x/libft" || got[0].Items[1].URL != "/p/a" {
		t.Errorf("주소가 다르다: %+v", got[0].Items)
	}
}

// 한쪽이 비면 가른 뜻이 없으므로 평소 목록으로 돌아간다.
func TestEcoleBundlesGivesUpWhenABoxWouldBeEmpty(t *testing.T) {
	only := []Category{{Name: "libft", Slug: "libft"}}
	if got := bundlesFor("école-42", only, nil, "/x"); got != nil {
		t.Errorf("시험이 하나도 없으면 nil이어야 하는데 %v", got)
	}
	exams := []Category{{Name: "exam02", Slug: "exam02"}}
	if got := bundlesFor("école-42", exams, nil, "/x"); got != nil {
		t.Errorf("프로젝트가 하나도 없으면 nil이어야 하는데 %v", got)
	}
}

// 중첩된 글이 오면 이 묶음이 다룰 모양이 아니라 통째로 물러난다.
func TestEcoleBundlesGivesUpOnNestedPosts(t *testing.T) {
	children := []Category{{Name: "exam02", Slug: "exam02"}}
	posts := []PostSummary{{Title: "libft", Slug: "a", Children: []PostSummary{{Title: "자식"}}}}
	if got := bundlesFor("école-42", children, posts, "/x"); got != nil {
		t.Errorf("중첩이 있으면 nil이어야 하는데 %v", got)
	}
}

// 표에 없는 분류는 예전 목록 그대로다.
func TestBundlesOnlyForListedCategories(t *testing.T) {
	if got := bundlesFor("libft", []Category{{Name: "exam02"}}, []PostSummary{{Title: "a"}}, "/x"); got != nil {
		t.Errorf("표에 없는 분류에는 묶음이 없어야 하는데 %v", got)
	}
}

// `exam`으로 시작한다고 다 시험이 아니다. 뒤가 숫자뿐일 때만이다.
func TestIsExamName(t *testing.T) {
	for _, s := range []string{"exam02", "exam06", "EXAM2"} {
		if !isExamName(s) {
			t.Errorf("%q는 시험이다", s)
		}
	}
	for _, s := range []string{"example", "exam", "exam 정리", "libft", "examen"} {
		if isExamName(s) {
			t.Errorf("%q는 시험이 아니다", s)
		}
	}
}

func eqStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
