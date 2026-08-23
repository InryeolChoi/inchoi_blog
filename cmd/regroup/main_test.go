package main

import (
	"reflect"
	"testing"

	"github.com/inryeol/blog/internal/curation"
)

func TestCuratedCategoryOrdersAndCareerMove(t *testing.T) {
	wantApplied := []string{"탐색적 자료분석", "회귀분석", "다변량분석"}
	wantCareer := []string{"취업 준비", "빅데이터 분석기사"}
	wantDev := []string{"Language", "리눅스 & 쉘", "소프트스킬"}
	wantServer := []string{"Django", "Spring", "Node.js"}
	wantClient := []string{"Javascript", "React", "모바일 프로그래밍"}

	var gotApplied, gotCareer, gotDev, gotServer, gotClient []string
	for _, group := range groups {
		if group.slug == "career" {
			gotCareer = group.members
		}
		if group.slug == "dev" {
			gotDev = group.members
		}
		for _, sub := range group.subs {
			switch sub.slug {
			case "수리통계-응용":
				gotApplied = sub.members
			case "서버-api":
				gotServer = sub.members
			case "클라이언트-ui":
				gotClient = sub.members
			}
		}
	}
	if !reflect.DeepEqual(gotApplied, wantApplied) {
		t.Fatalf("수리/통계: 응용 순서 = %v, 원한 값 %v", gotApplied, wantApplied)
	}
	if !reflect.DeepEqual(gotCareer, wantCareer) {
		t.Fatalf("커리어 순서 = %v, 원한 값 %v", gotCareer, wantCareer)
	}
	if !reflect.DeepEqual(gotDev, wantDev) {
		t.Fatalf("개발 순서 = %v, 원한 값 %v", gotDev, wantDev)
	}
	// 웹 프로그래밍 77편을 배운 순서(Django → Spring)와 성격대로 두 갈래로 갈랐다.
	// 3단계가 끝이라 웹 프로그래밍 밑에 한 층을 더 두는 길은 막혀 있다.
	if !reflect.DeepEqual(gotServer, wantServer) {
		t.Fatalf("서버 & API 순서 = %v, 원한 값 %v", gotServer, wantServer)
	}
	if !reflect.DeepEqual(gotClient, wantClient) {
		t.Fatalf("클라이언트 & UI 순서 = %v, 원한 값 %v", gotClient, wantClient)
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

// 소개 분류에는 하위 분류를 두지 않는다.
//
// 노션 최상위 `최인렬 (Inryeol Choi)`가 그대로 남으면 소개 화면에 "하위 분류 >
// 최인렬 (Inryeol Choi) 글 1건"이 생기는데, 그 갈래에는 화면 위쪽에 이미 펼친
// 자기소개 한 편뿐이라 눌러도 제자리로 돌아온다. 글은 PostMoves로 소개에 직접
// 붙이고 빈 껍데기가 된 카테고리는 DropCategories가 지운다.
func TestIntroHasNoNotionLayerBeneathIt(t *testing.T) {
	const notionTop = "최인렬 (Inryeol Choi)"
	const introPost = "1080901b-87f1-80d2-811a-eba467c2c160"

	for _, group := range groups {
		if group.slug != "intro" {
			continue
		}
		if len(group.members) != 0 || len(group.subs) != 0 {
			t.Errorf("소개 밑에 층이 남았다: members=%v subs=%v", group.members, group.subs)
		}
	}

	if slug := curation.PostMoveBySlug()[introPost]; slug != "intro" {
		t.Errorf("자기소개 글이 붙는 곳 = %q, 원한 값 %q", slug, "intro")
	}

	dropped := false
	for _, drop := range curation.DropCategories {
		if drop.SourceName == notionTop {
			dropped = true
		}
	}
	if !dropped {
		t.Errorf("%q를 DropCategories에 적지 않았다. 다음 categorize가 되살린다", notionTop)
	}
}
