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

// TestSortPostsKeepsIncomingOrderOtherwise는 번호가 없거나 같으면 들어온 순서를
// 그대로 두는지 본다. 들어올 때 이미 sort_order, title 순이다.
func TestSortPostsKeepsIncomingOrderOtherwise(t *testing.T) {
	in := summaries("나중", "먼저", "2. 단순회귀 (1)", "2. 단순회귀 (2)")

	sortPosts(in)

	want := "2. 단순회귀 (1) | 2. 단순회귀 (2) | 나중 | 먼저"
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
