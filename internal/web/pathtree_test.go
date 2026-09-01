package web

import (
	"database/sql"
	"strings"
	"testing"
)

// 이 파일이 지키는 것은 하나다: **조용히 반만 묶지 않는다.**
//
// 경로로 층을 되살리는 것은 근거가 있을 때만 옳다. 근거가 흔들리는데도
// 억지로 묶으면 화면이 "이게 원래 구조"라고 거짓말을 하고, 읽는 사람은
// 그 말을 믿을 수밖에 없다. 그래서 pathTree는 **어긋나면 통째로 포기하고**
// 평소 목록으로 돌아간다 — 갈래 카드가 그림 없는 갈래 하나에 통째로
// 포기하는 것과 같은 규칙이다.

func cat(sourceName string) Category { return Category{SourceName: sourceName} }

func post(title, path string) PostSummary {
	return PostSummary{
		Title:        title,
		Slug:         title,
		OriginalPath: sql.NullString{String: path, Valid: path != ""},
	}
}

// 여러 언어가 뒤섞인 목록을 경로로 가른다. `프로그래밍 언어` 111편이 이 경우다.
func TestPathTreeSplitsByLostLevels(t *testing.T) {
	var posts []PostSummary
	for _, n := range []string{"구조체", "포인터", "배열", "변수", "함수", "매크로"} {
		posts = append(posts, post(n, "Language > 프로그래밍 언어 > C > "+n))
	}
	for _, n := range []string{"람다식", "스트림", "제너릭스", "상속", "다형성", "예외처리"} {
		posts = append(posts, post(n, "Language > 프로그래밍 언어 > Java > "+n))
	}

	nodes := pathTree(cat("프로그래밍 언어"), posts)
	if len(nodes) != 2 {
		t.Fatalf("마디가 %d개다. C와 Java 둘이어야 한다", len(nodes))
	}
	names := []string{nodes[0].Name(), nodes[1].Name()}
	if names[0] != "C" || names[1] != "Java" {
		t.Errorf("마디 이름이 %v다", names)
	}
	for _, n := range nodes {
		if len(n.Children) != 6 {
			t.Errorf("%s 아래가 %d편이다. 6편이어야 한다", n.Name(), len(n.Children))
		}
	}
}

// **근거가 흔들리면 통째로 포기한다.** 네 가지 거절 조건을 하나씩 어겨본다.
func TestPathTreeGivesUpRatherThanGuess(t *testing.T) {
	good := func() []PostSummary {
		var out []PostSummary
		for _, n := range []string{"a", "b", "c", "d", "e", "f"} {
			out = append(out, post("c-"+n, "Language > 프로그래밍 언어 > C > c-"+n))
		}
		for _, n := range []string{"a", "b", "c", "d", "e", "f"} {
			out = append(out, post("j-"+n, "Language > 프로그래밍 언어 > Java > j-"+n))
		}
		return out
	}
	if pathTree(cat("프로그래밍 언어"), good()) == nil {
		t.Fatal("멀쩡한 목록을 포기했다 — 아래 검사들이 뜻이 없어진다")
	}

	t.Run("사람이 만든 분류는 닻이 없다", func(t *testing.T) {
		if got := pathTree(cat(""), good()); got != nil {
			t.Errorf("source_name이 없는데 묶었다: %d마디", len(got))
		}
	})

	t.Run("짧은 목록은 안 묶는다", func(t *testing.T) {
		if got := pathTree(cat("프로그래밍 언어"), good()[:8]); got != nil {
			t.Errorf("%d편짜리를 묶었다. %d편부터여야 한다", 8, pathTreeMinPosts)
		}
	})

	t.Run("parent_id가 이미 층을 만든 목록", func(t *testing.T) {
		posts := good()
		posts[0].Children = []PostSummary{post("자식", "")}
		if got := pathTree(cat("프로그래밍 언어"), posts); got != nil {
			t.Errorf("계층이 두 벌이 됐다: %d마디", len(got))
		}
	})

	t.Run("한 편이라도 닻을 못 찾으면", func(t *testing.T) {
		posts := good()
		// curation이 다른 분류에서 데려온 글은 경로가 다르다(선형대수 11편).
		posts[3] = post("남의 글", "수학 & 통계 > 선형대수 > 남의 글")
		if got := pathTree(cat("프로그래밍 언어"), posts); got != nil {
			t.Errorf("경로가 다른 글이 섞였는데 묶었다: %d마디", len(got))
		}
	})

	t.Run("가르지 못하면 안 쓴다", func(t *testing.T) {
		// 노션에서도 원래 평평했던 곳. 마디가 글 수만큼 생겨 아무것도 안 준다.
		var flat []PostSummary
		for _, n := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"} {
			flat = append(flat, post(n, "Language > 프로그래밍 언어 > "+n+" > "+n))
		}
		if got := pathTree(cat("프로그래밍 언어"), flat); got != nil {
			t.Errorf("층이 목록을 못 줄이는데 썼다: %d마디", len(got))
		}
	})
}

// 모두가 같은 첫 칸을 가지면 그 칸은 아무것도 가르지 않는다. 벗겨내고 다음을 본다.
func TestPathTreePeelsUselessLevels(t *testing.T) {
	var posts []PostSummary
	for _, n := range []string{"a", "b", "c", "d", "e", "f"} {
		posts = append(posts, post("x-"+n, "CS 이론 > 운영체제 > 같은칸 > Part 1 > x-"+n))
	}
	for _, n := range []string{"a", "b", "c", "d", "e", "f"} {
		posts = append(posts, post("y-"+n, "CS 이론 > 운영체제 > 같은칸 > Part 2 > y-"+n))
	}
	nodes := pathTree(cat("운영체제"), posts)
	if len(nodes) != 2 {
		t.Fatalf("마디가 %d개다. 쓸모없는 칸을 벗기고 Part 1·2가 나와야 한다", len(nodes))
	}
	if nodes[0].Name() != "Part 1" || nodes[1].Name() != "Part 2" {
		t.Errorf("마디 이름이 %q, %q다", nodes[0].Name(), nodes[1].Name())
	}
}

// 마디 이름과 제목이 같은 글이 목록에 있으면 **그 글이 그 마디다**(링크).
// 없으면 posts에 행이 없는 인라인 데이터베이스라 이름표로만 남는다 —
// InlineDBGroups가 본문에서 하는 판정을 목록에서 하는 것이다.
func TestPathTreeNodeIsThePostWhenOneExists(t *testing.T) {
	var posts []PostSummary
	posts = append(posts, post("C", "Language > 프로그래밍 언어 > C"))
	for _, n := range []string{"a", "b", "c", "d", "e"} {
		posts = append(posts, post("c-"+n, "Language > 프로그래밍 언어 > C > c-"+n))
	}
	for _, n := range []string{"a", "b", "c", "d", "e", "f"} {
		posts = append(posts, post("j-"+n, "Language > 프로그래밍 언어 > Java > j-"+n))
	}

	nodes := pathTree(cat("프로그래밍 언어"), posts)
	var cNode, jNode *PostNode
	for i := range nodes {
		switch nodes[i].Name() {
		case "C":
			cNode = &nodes[i]
		case "Java":
			jNode = &nodes[i]
		}
	}
	if cNode == nil || jNode == nil {
		t.Fatalf("C와 Java 마디가 없다: %d개", len(nodes))
	}
	if cNode.Post == nil {
		t.Error("제목이 같은 글이 있는데 링크가 아니다")
	}
	if jNode.Post != nil {
		t.Error("posts에 행이 없는 마디가 링크가 됐다 — 눌러도 404다")
	}
}

// 화면이 실제로 이 층을 그리는지 본다. 함수만 맞고 템플릿이 안 쓰면 소용없다.
func TestCategoryPageRendersPathTree(t *testing.T) {
	body := get(t, testServer(t), "/dev/language").Body.String()
	if strings.Contains(body, "pathgroup") && !strings.Contains(body, "class=\"list\"") {
		t.Error("이름표만 있고 목록이 없다")
	}
}
