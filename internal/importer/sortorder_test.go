package importer

import (
	"reflect"
	"testing"
)

func TestDenseRank(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []int
	}{
		{"전부 다름", []string{"B", "A", "C"}, []int{1, 0, 2}},
		{"동률 있음", []string{"B", "A", "A", "C"}, []int{1, 0, 0, 2}},
		{"전부 동률", []string{"A", "A", "A"}, []int{0, 0, 0}},
		{"하나뿐", []string{"A"}, []int{0}},
		{"빈 입력", []string{}, []int{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DenseRank(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DenseRank(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestDenseRankIsContiguous는 순위가 0부터 빈틈없이 이어지는지 본다.
// 동률 뒤에 건너뛰는 순위(0,0,2)를 주면 나중에 목록을 이어붙일 때 틈이 생긴다.
func TestDenseRankIsContiguous(t *testing.T) {
	got := DenseRank([]string{"A", "A", "B", "B", "B", "C"})
	want := []int{0, 0, 1, 1, 1, 2}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestDenseRankAllTiedIsAllZero는 전부 같은 시각이면 모두 0이 되는지 본다.
// "순서를 전혀 못 정함"이 0으로 드러나야 나중에 손볼 대상을 찾을 수 있다.
func TestDenseRankAllTiedIsAllZero(t *testing.T) {
	got := DenseRank([]string{"2024-01-17T08:19", "2024-01-17T08:19", "2024-01-17T08:19"})
	for i, v := range got {
		if v != 0 {
			t.Errorf("[%d] = %d, 전부 0이어야 한다", i, v)
		}
	}
}

// TestDenseRankOrdersRealTimestamps는 노션 시각 문자열이 사전순 비교로도
// 시간순이 되는지 본다(ISO8601이라 그렇다).
func TestDenseRankOrdersRealTimestamps(t *testing.T) {
	times := []string{
		"2023-08-19T16:36:00.000Z",
		"2023-08-16T07:47:00.000Z",
		"2023-08-19T08:42:00.000Z",
	}
	got := DenseRank(times)
	want := []int{2, 0, 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestTiedGroups(t *testing.T) {
	tests := []struct {
		in                []string
		wantTied, wantAll int
	}{
		{[]string{"A", "B", "C"}, 0, 3},
		{[]string{"A", "A", "B"}, 1, 3},
		{[]string{"A", "A", "A"}, 3, 3},
		{[]string{"A"}, 0, 0},
	}
	for _, tt := range tests {
		tied, all := TiedGroups(tt.in)
		if tied != tt.wantTied || all != tt.wantAll {
			t.Errorf("TiedGroups(%v) = (%d, %d), want (%d, %d)",
				tt.in, tied, all, tt.wantTied, tt.wantAll)
		}
	}
}

func TestAllTied(t *testing.T) {
	tests := []struct {
		in   []string
		want bool
	}{
		{[]string{"A", "A", "A"}, true},
		{[]string{"A", "A", "B"}, false},
		{[]string{"A"}, false}, // 하나뿐이면 순서 문제가 없다
		{[]string{}, false},
	}
	for _, tt := range tests {
		if got := AllTied(tt.in); got != tt.want {
			t.Errorf("AllTied(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
