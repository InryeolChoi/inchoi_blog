package main

import (
	"reflect"
	"testing"

	"github.com/inryeol/blog/internal/curation"
)

func TestDataMathAppliedOrderAndCareerMove(t *testing.T) {
	wantApplied := []string{"탐색적 자료분석", "회귀분석", "다변량분석"}
	wantCareer := []string{"취업 준비", "빅데이터 분석기사"}

	var gotApplied, gotCareer []string
	for _, group := range groups {
		if group.slug == "career" {
			gotCareer = group.members
		}
		for _, sub := range group.subs {
			if sub.slug == "수리통계-응용" {
				gotApplied = sub.members
			}
		}
	}
	if !reflect.DeepEqual(gotApplied, wantApplied) {
		t.Fatalf("수리/통계: 응용 순서 = %v, 원한 값 %v", gotApplied, wantApplied)
	}
	if !reflect.DeepEqual(gotCareer, wantCareer) {
		t.Fatalf("커리어 순서 = %v, 원한 값 %v", gotCareer, wantCareer)
	}

	for _, move := range curation.Moves {
		if move.SourceName == "빅데이터 분석기사" {
			if move.ToSlug != "career" {
				t.Fatalf("빅데이터 분석기사 이동 대상 = %q, 원한 값 career", move.ToSlug)
			}
			return
		}
	}
	t.Fatal("빅데이터 분석기사 이동 규칙이 없다")
}
