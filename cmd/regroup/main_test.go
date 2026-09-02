package main

import (
	"database/sql"
	"reflect"
	"testing"

	"github.com/inryeol/blog/internal/curation"
)

func TestCuratedCategoryOrdersAndCareerMove(t *testing.T) {
	wantApplied := []string{"탐색적 자료분석", "회귀분석", "다변량분석"}
	wantDev := []string{"Language", "리눅스 & 쉘", "소프트스킬"}
	wantServer := []string{"Django", "Spring", "Node.js"}
	wantClient := []string{"Javascript", "React", "모바일 프로그래밍"}

	var gotApplied, gotDev, gotServer, gotClient []string
	for _, group := range groups {
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

	// 커리어는 통째로 없앴다. 배정표에도 이동 규칙에도 남아 있으면 안 된다 —
	// 남으면 regroup이 지운 분류를 다음 실행이 되살린다.
	for _, group := range groups {
		if group.slug == "career" {
			t.Fatal("없앤 커리어 묶음이 배정표에 남아 있다")
		}
	}
	for _, move := range curation.Moves {
		if move.ToSlug == "career" {
			t.Fatalf("없앤 커리어로 보내는 이동 규칙이 남아 있다: %q", move.SourceName)
		}
	}
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

// moveTarget은 **2단계 분류만** 부모로 받아준다.
//
// 최상위는 groups/subs 쪽에서 이미 잡히고, 3단계 밑에 매달면 4단계가 된다.
// 이 판정이 느슨해지면 002의 트리거가 대신 막아주지만, 그때 나오는 말은
// "트리거가 거부했다"라 무엇이 잘못됐는지 알려주지 않는다.
func TestMoveTargetTakesOnlyMidLevelCategories(t *testing.T) {
	top := &category{id: 1, slug: "dev"}
	mid := &category{id: 2, slug: "tooling", parentID: sql.NullInt64{Int64: 1, Valid: true}}
	leaf := &category{id: 3, slug: "git", parentID: sql.NullInt64{Int64: 2, Valid: true}}
	cat := catalog{
		bySlug: map[string]*category{"dev": top, "tooling": mid, "git": leaf},
		byID:   map[int64]*category{1: top, 2: mid, 3: leaf},
	}

	for _, tc := range []struct {
		slug string
		want bool
	}{
		{"tooling", true},
		{"dev", false}, // 최상위
		{"git", false}, // 3단계 — 그 밑은 4단계다
		{"없다", false},  // 오타
	} {
		if _, ok := cat.moveTarget(tc.slug); ok != tc.want {
			t.Errorf("moveTarget(%q) = %v, want %v", tc.slug, ok, tc.want)
		}
	}
}

// 클라우드가 실제로 tooling을 가리키는지 본다. 이 줄은 노션에서 온 2단계
// 분류를 부모로 쓰는 **첫 자리**라, 지워지면 moveTarget이 쓰이는 곳이 없어진다.
func TestCloudMovesUnderTooling(t *testing.T) {
	for _, mv := range curation.Moves {
		if mv.SourceName == "클라우드 탐구생활" {
			if mv.ToSlug != "tooling" {
				t.Errorf("클라우드가 %q로 간다", mv.ToSlug)
			}
			return
		}
	}
	t.Error("클라우드 탐구생활 이동이 표에 없다")
}
