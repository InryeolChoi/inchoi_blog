package importer

import (
	"reflect"
	"testing"
)

// TestPathAncestorsDropsPageTitle은 경로의 마지막 요소(글 제목 자신)를
// 빼는지 본다. 안 빼면 글이 자기 제목과 같은 이름의 카테고리에 들어간다.
func TestPathAncestorsDropsPageTitle(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"운영체제 > part 4 : 메모리 관리 > 공룡책 9장 > 3. 페이징",
			[]string{"운영체제", "part 4 : 메모리 관리", "공룡책 9장"}},
		{"école 42 > Netpractice", []string{"école 42"}},
		{"Netpractice", nil},
		{"", nil},
		{"   ", nil},
	}
	for _, tt := range tests {
		got := PathAncestors(tt.path)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("PathAncestors(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// TestPathAncestorsHandlesExtraSpaces는 실제 덤프에 있는
// "pipex >  pipex : 진행 상황" 처럼 공백이 두 번 들어간 경우를 본다.
func TestPathAncestorsHandlesExtraSpaces(t *testing.T) {
	got := PathAncestors("école 42 > pipex >  pipex : 진행 상황 > 리다이렉션과 파이프")
	want := []string{"école 42", "pipex", "pipex : 진행 상황"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Part 2", "part-2"},
		{"운영체제", "운영체제"},
		{"수학 & 통계", "수학-통계"},
		{"part 4 : 메모리 관리", "part-4-메모리-관리"},
		{"école 42", "école-42"},
		{"공룡책 9장 : 메인 메모리", "공룡책-9장-메인-메모리"},
		{"알고리즘: 실전", "알고리즘-실전"},
		{"  앞뒤 공백  ", "앞뒤-공백"},
		{"연속   공백", "연속-공백"},
		{"Language", "language"},
		{"머신러닝 & 딥러닝", "머신러닝-딥러닝"},
	}
	for _, tt := range tests {
		if got := Slugify(tt.in); got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestSlugifyNeverStartsOrEndsWithHyphen은 앞뒤에 하이픈이 남지 않는지 본다.
func TestSlugifyNeverStartsOrEndsWithHyphen(t *testing.T) {
	for _, in := range []string{": 앞에 기호", "뒤에 기호 :", "-- 하이픈 --", "(괄호)"} {
		got := Slugify(in)
		if got == "" {
			continue
		}
		if got[0] == '-' || got[len(got)-1] == '-' {
			t.Errorf("Slugify(%q) = %q — 앞뒤에 하이픈이 남았다", in, got)
		}
	}
}

func TestAssignCategory(t *testing.T) {
	tests := []struct {
		path         string
		want1, want2 string
		leaf         string
	}{
		{"운영체제 > part 4 > 공룡책 9장 > 3. 페이징", "운영체제", "part 4", "part 4"},
		{"école 42 > Netpractice", "école 42", "", "école 42"},
		{"Netpractice", "", "", ""},
	}
	for _, tt := range tests {
		got := AssignCategory("p1", tt.path)
		if got.Level1 != tt.want1 || got.Level2 != tt.want2 {
			t.Errorf("AssignCategory(%q) = (%q, %q), want (%q, %q)",
				tt.path, got.Level1, got.Level2, tt.want1, tt.want2)
		}
		if got.Leaf() != tt.leaf {
			t.Errorf("AssignCategory(%q).Leaf() = %q, want %q", tt.path, got.Leaf(), tt.leaf)
		}
	}
}

// TestAssignCategoryIgnoresDeeperAncestors는 카테고리가 2단계까지만
// 만들어지는지 본다. 스키마의 트리거가 3단계를 막는다.
func TestAssignCategoryIgnoresDeeperAncestors(t *testing.T) {
	got := AssignCategory("p1", "A > B > C > D > E > 제목")
	if got.Level1 != "A" || got.Level2 != "B" {
		t.Errorf("3단계 이후를 가져왔다: %+v", got)
	}
}
