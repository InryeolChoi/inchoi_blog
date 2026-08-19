package web

import (
	"strings"
	"testing"
)

// 사이드바는 모든 페이지에 나온다. 어느 한 페이지에서만 나오면 탐색이 끊긴다.
func TestSidebarOnEveryPage(t *testing.T) {
	h := testServer(t)
	for _, path := range []string{"/", "/dev", "/dev/language", "/dev/language/python", "/p/list-post"} {
		side := sideOf(t, get(t, h, path).Body.String())
		if !strings.Contains(side, `href="/dev"`) {
			t.Errorf("%s의 사이드바에 최상위 분류가 없다:\n%s", path, side)
		}
	}
}

// 최상위는 늘 보이고 그 아래는 접혀 있다. 지금 보고 있는 곳으로 가는 길만 펼친다.
func TestSidebarOpensOnlyCurrentTrail(t *testing.T) {
	h := testServer(t)

	// 최상위 목록에서는 아무것도 펼치지 않는다.
	side := sideOf(t, get(t, h, "/").Body.String())
	if strings.Contains(side, "is-open") {
		t.Errorf("홈에서 분류가 펼쳐져 있다:\n%s", side)
	}

	// 3단계로 들어가면 그 위 두 단계가 펼쳐져 있어야 자기 자리가 보인다.
	side = sideOf(t, get(t, h, "/dev/language/python").Body.String())
	if n := strings.Count(side, "is-open"); n != 2 {
		t.Errorf("펼쳐진 마디가 %d개다(dev, language 둘을 기대):\n%s", n, side)
	}
	if !strings.Contains(side, `href="/dev/language/python" aria-current="page"`) {
		t.Errorf("현재 위치 표시가 없다:\n%s", side)
	}
}

// 글을 열면 그 글이 속한 분류가 펼쳐져 있어야 한다.
func TestSidebarMarksPostCategory(t *testing.T) {
	side := sideOf(t, get(t, testServer(t), "/p/list-post").Body.String())
	if !strings.Contains(side, `href="/dev/language/python" aria-current="page"`) {
		t.Errorf("글이 속한 분류가 표시되지 않았다:\n%s", side)
	}
}

// 자식이 없는 분류에는 펼침 단추를 두지 않는다. 눌러도 아무 일이 없는 단추다.
func TestSidebarNoTwistWithoutChildren(t *testing.T) {
	side := sideOf(t, get(t, testServer(t), "/").Body.String())
	i := strings.Index(side, `href="/dev/tools"`)
	if i < 0 {
		t.Fatalf("tools 링크가 없다:\n%s", side)
	}
	// tools 링크 바로 앞의 행에 단추가 있으면 안 된다.
	row := side[max(0, i-220):i]
	if strings.Contains(row, "nav-twist\"") {
		t.Errorf("자식 없는 분류에 펼침 단추가 있다:\n%s", row)
	}
}

// 사이드바 하단의 바깥 링크. rel=noreferrer가 빠지면 우리 주소가 상대 로그에 남는다.
func TestSidebarExternalLinks(t *testing.T) {
	side := sideOf(t, get(t, testServer(t), "/").Body.String())
	for _, want := range []string{
		`href="https://github.com/InryeolChoi" rel="noreferrer"`,
		`href="https://inryeolchoi.github.io" rel="noreferrer"`,
	} {
		if !strings.Contains(side, want) {
			t.Errorf("%q가 없다:\n%s", want, side)
		}
	}
}

// markNav는 깊이에 상한이 있다. categories 트리거가 3단계를 지키지만,
// 사이드바는 모든 페이지에 그려져서 여기서 도는 것이 곧 전체 장애다.
func TestMarkNavHandlesEmptyTree(t *testing.T) {
	markNav(nil, nil, 0) // 죽지만 않으면 된다
	tree := []NavCategory{{ID: 1, Children: []NavCategory{{ID: 2}}}}
	markNav(tree, map[int64]bool{1: true}, 2)
	if !tree[0].Open {
		t.Error("부모가 안 펼쳐졌다")
	}
	if !tree[0].Children[0].Active {
		t.Error("자식에 현재 위치 표시가 없다")
	}
	if tree[0].Children[0].Open {
		t.Error("자식이 없는데 펼침 표시가 붙었다")
	}
}
