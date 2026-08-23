package web

import (
	"database/sql"
	"strings"
	"testing"
	"time"
)

func titlesOf(in []PostSummary) string {
	out := make([]string, len(in))
	for i, p := range in {
		out[i] = p.Title
	}
	return strings.Join(out, " | ")
}

func summaries(titles ...string) []PostSummary {
	out := make([]PostSummary, len(titles))
	for i, t := range titles {
		out[i] = PostSummary{ID: int64(i + 1), Title: t, SortOrder: i}
	}
	return out
}

// TestSortPostsOrdersByLeadingNumber는 제목 앞 번호대로 세우는지 본다.
// sort_order가 created_time에서 나와서 "10."이 "2."보다 앞에 오는 목록이 있다.
func TestSortPostsOrdersByLeadingNumber(t *testing.T) {
	in := summaries("1. 인텔리제이", "10. 무중단 배포", "2. 테스트 코드", "9. 자동 배포화")

	sortPosts(in)

	want := "1. 인텔리제이 | 2. 테스트 코드 | 9. 자동 배포화 | 10. 무중단 배포"
	if got := titlesOf(in); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// TestSortPostsPushesUnnumberedToBack은 번호 없는 글을 번호 있는 것들 사이에
// 끼우지 않고 뒤로 보내는지 본다. 어디에 끼워야 할지 알 방법이 없다.
func TestSortPostsPushesUnnumberedToBack(t *testing.T) {
	in := summaries("과제1", "1. 확률", "연습문제", "2. 확률분포", "중간 & 기말")

	sortPosts(in)

	want := "1. 확률 | 2. 확률분포 | 과제1 | 연습문제 | 중간 & 기말"
	if got := titlesOf(in); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// TestSortPostsSortsTailNaturally는 앞 번호가 없는 글들을 제목 자연 정렬로
// 세우는지 본다. 예전에는 들어온 순서(sort_order)를 그대로 뒀는데, 그 값이
// 노션 작성 시각이라 제목에 번호가 적힌 시리즈가 서로 엇갈려 보였다.
func TestSortPostsSortsTailNaturally(t *testing.T) {
	in := summaries(
		"practice problem 1", "2022년 탐자 1차 시험", "practice problem 2", "연습문제",
		"2022년 탐자 2차 시험", "2022년 탐자 3차 시험", "practice problem 3")

	sortPosts(in)

	want := "2022년 탐자 1차 시험 | 2022년 탐자 2차 시험 | 2022년 탐자 3차 시험 | " +
		"practice problem 1 | practice problem 2 | practice problem 3 | 연습문제"
	if got := titlesOf(in); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// TestNaturalCompareReadsNumbersAsNumbers는 제목 속 숫자를 값으로 견주는지 본다.
// 글자로 견주면 "10"이 "2"보다 앞에 온다.
func TestNaturalCompareReadsNumbersAsNumbers(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"인자분석 : 코드 (2)", "인자분석 : 코드 (10)", -1},
		{"practice problem 10", "practice problem 9", 1},
		{"exam 007", "exam 7", 0}, // 앞의 0은 값에 영향이 없다
		{"2022년 탐자", "practice", -1},
		{"같다", "같다", 0},
		{"같", "같다", -1}, // 앞이 같으면 짧은 쪽이 앞이다
	}
	for _, c := range cases {
		got := naturalCompare(c.a, c.b)
		if (got < 0) != (c.want < 0) || (got > 0) != (c.want > 0) {
			t.Errorf("naturalCompare(%q, %q) = %d, want 부호 %d", c.a, c.b, got, c.want)
		}
	}
}

// TestSortPostsKeepsNumberedPrefixRule은 앞 번호 규칙이 그대로인지 본다.
// 자연 정렬은 **번호 없는 뒤쪽에만** 걸린다 — 앞 번호는 사람이 직접 붙인 것이라
// 여전히 더 믿을 만하고, 번호 없는 글을 그 사이에 끼우지도 않는다.
func TestSortPostsKeepsNumberedPrefixRule(t *testing.T) {
	in := summaries("과제1", "1. 확률", "42 : Cpp 모듈", "2. 확률분포")

	sortPosts(in)

	want := "1. 확률 | 2. 확률분포 | 42 : Cpp 모듈 | 과제1"
	if got := titlesOf(in); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// TestPostNumberNeedsSeparator는 앞자리 숫자를 아무거나 번호로 읽지 않는지 본다.
// "2022년 탐자 1차 시험" 같은 제목이 실제로 있다.
func TestPostNumberNeedsSeparator(t *testing.T) {
	for _, title := range []string{"2022년 탐자 1차 시험", "1주차 정리", "3장 연습문제", "42 : Cpp 모듈"} {
		if n, ok := postNumber(title); ok {
			t.Errorf("%q를 번호 %d로 읽었다", title, n)
		}
	}
	for title, want := range map[string]int{
		"1. 파일 다루기": 1, "10. 데이터 재구조화": 10, "0. 튜토리얼": 0, "3) 세 번째": 3,
	} {
		n, ok := postNumber(title)
		if !ok || n != want {
			t.Errorf("%q → %d, %v (want %d)", title, n, ok, want)
		}
	}
}

// TestSortPostsAppliesToNestedChildren은 중첩된 하위 글에도 정렬이 걸리는지 본다.
// nestPosts가 들어온 순서를 형제 순서로 쓰므로 평평할 때 정렬해두면 된다.
func TestSortPostsAppliesToNestedChildren(t *testing.T) {
	flat := []PostSummary{
		{ID: 1, Title: "부모"},
		{ID: 2, Title: "10. 열", ParentID: sql.NullInt64{Int64: 1, Valid: true}},
		{ID: 3, Title: "2. 둘", ParentID: sql.NullInt64{Int64: 1, Valid: true}},
	}
	sortPosts(flat)
	nested := nestPosts(flat)

	if len(nested) != 1 || len(nested[0].Children) != 2 {
		t.Fatalf("모양이 다르다: %+v", nested)
	}
	if got := titlesOf(nested[0].Children); got != "2. 둘 | 10. 열" {
		t.Errorf("got %s", got)
	}
}

// TestPostSummaryDate는 목록에 찍을 날짜를 만드는지 본다. 값이 없으면 빈
// 문자열이라 템플릿의 {{with}}에서 저절로 빠진다.
func TestPostSummaryDate(t *testing.T) {
	p := PostSummary{CreatedAt: sql.NullTime{
		Time: time.Date(2022, 8, 27, 11, 51, 0, 0, time.UTC), Valid: true}}
	if got := p.Date(); got != "2022-08-27" {
		t.Errorf("got %q", got)
	}
	if got := (PostSummary{}).Date(); got != "" {
		t.Errorf("값이 없는데 %q", got)
	}
}
