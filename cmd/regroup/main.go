// regroup은 노션에서 온 최상위 카테고리들을 더 큰 분류 밑으로 묶는다.
//
//	dev > Language > 프로그래밍 언어
//	└ 새로 만드는 것   └ 노션 최상위    └ 노션 2단계 (그대로 딸려 내려감)
//
// 노션 워크스페이스의 최상위 구조는 블로그의 분류로 쓰기엔 너무 잘다. cmd/categorize가
// 경로에서 뽑아낸 19개 위에 사람이 정한 8개를 얹는다.
//
// 이 도구는 카테고리를 지우지 않는다. 이름과 slug는 그대로 두고 parent_id만 바꾼다
// (이름을 바꾸라고 지정한 것만 예외).
//
// 기본은 미리보기다. 실제로 DB를 고치려면 -apply를 준다.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/inryeol/blog/internal/curation"
	"github.com/inryeol/blog/internal/db"
)

const rule = "════════════════════════════════════════════════════════════════════════"

// group은 새로 만들 최상위 분류 하나다.
type group struct {
	slug string
	name string
	// members는 이 분류 밑으로 내릴 기존 카테고리 이름들이다.
	members []string
	// subs는 이 분류 밑에 **한 층을 더** 두는 경우다. members와 같이 쓰지 않는다.
	//
	// 노션 최상위 하나가 너무 많은 것을 담고 있어서 사람이 갈라주고 싶을 때 쓴다.
	// 깊이는 group(1) > sub(2) > member(3)으로 여전히 3단계다.
	subs []subgroup
}

// subgroup은 최상위 분류 밑에 사람이 하나 더 두는 층이다.
type subgroup struct {
	slug    string
	name    string
	members []string
}

// rename은 옮기면서 이름과 slug까지 바꾸는 경우다.
type rename struct {
	fromName string
	toName   string
	toSlug   string
}

// groups는 새 최상위 분류와 그 아래로 내릴 기존 카테고리다.
// 순서가 곧 sort_order다.
var groups = []group{
	// 소개 밑에는 노션에서 온 층을 두지 않는다. 자기소개 글 한 편이 곧 이 분류의
	// 표지라 하위 분류를 만들면 같은 글로 되돌아오는 갈래가 하나 생긴다.
	{slug: "intro", name: "소개", members: nil},
	{slug: "algorithm", name: "알고리즘", members: []string{
		"알고리즘: 이론", "알고리즘: 실전",
	}},
	{slug: "cs-theory", name: "CS 이론", members: []string{
		"운영체제", "네트워크", "데이터베이스", "가상화기술",
	}},
	// 개발은 members와 subs를 **함께** 쓰는 유일한 분류다. Language·리눅스 & 쉘·
	// tooling은 노션 최상위가 그대로 한 갈래인데, 웹 프로그래밍 77편만은 배운
	// 순서(Django → Spring)와 곁다리로 붙은 것들(HTML/CSS, React, 로그인, 웹소켓)이
	// 한 갈래에 뒤엉켜 있었다. 3단계가 끝이라 `웹 프로그래밍 > 백엔드 > Django`로는
	// 못 내려가므로, 웹 프로그래밍 자리를 두 갈래가 대신한다.
	//
	// 이름을 "프론트엔드"가 아니라 "클라이언트 & UI"로 둔 이유: 그 밑에 모바일
	// 프로그래밍(Swift/UIKit)이 들어간다. 웹만 가리키는 말이면 어긋난다.
	{slug: "dev", name: "개발", members: []string{
		"Language", "리눅스 & 쉘", "소프트스킬",
	}, subs: []subgroup{
		{slug: "서버-api", name: "서버 & API", members: []string{
			"Django", "Spring", "Node.js",
		}},
		{slug: "클라이언트-ui", name: "클라이언트 & UI", members: []string{
			"Javascript", "React", "모바일 프로그래밍",
		}},
	}},
	// 노션 최상위 "수학 & 통계" 하나에 이론·응용·머신러닝이 다 들어 있어서
	// 사람이 세 갈래로 갈랐다. 갈래 이름은 노션에 없는 것이라 source_name이 NULL이다.
	{slug: "data-math", name: "데이터 & 수리", subs: []subgroup{
		{slug: "수리통계-이론", name: "수리/통계: 이론", members: []string{
			"선형대수", "최적화이론", "수리통계1", "수리통계2", "확률과정론", "이산수학",
		}},
		{slug: "수리통계-응용", name: "수리/통계: 응용", members: []string{
			"탐색적 자료분석", "회귀분석", "다변량분석",
		}},
		{slug: "머신러닝", name: "머신러닝", members: []string{
			"핸즈온 머신러닝 2", "자연어처리 (1) : BERT와 GPT",
		}},
	}},
	{slug: "project", name: "프로젝트", members: []string{
		"école 42", "Projects",
	}},
	{slug: "life", name: "라이프", members: nil},
}

// topic은 사람이 **기존 분류 아래에** 새로 두는 갈래다.
//
// # 왜 subs로 안 되나
//
// `subs`는 최상위(group) 밑에만 층을 놓는다. 여기서 필요한 것은 그보다 한 칸
// 아래다 — `알고리즘 > 알고리즘: 실전 > 동적 계획법`. 3단계가 끝이므로 부모가
// 2단계일 때만 쓸 수 있고, `mustBeDepth`가 그것을 확인한다.
//
// # 왜 categorize가 못 만드나
//
// categorize는 `original_path`가 알려주는 것만 안다. 백준 문제의 경로는
// `알고리즘: 실전 > 백준 단계별로 풀기: 1번 ~ 9번 > 1978번: 소수 찾기`라
// **푼 순서**만 적혀 있고 무엇을 배웠나는 어디에도 없다. 사람이 정하는 층이라
// 여기 있는 것이 맞다 — `groups`·`subs`와 같은 성질이다.
type topic struct {
	// parentSlug는 이 갈래를 매달 기존 분류다. 반드시 2단계여야 한다.
	parentSlug string
	slug       string
	name       string
}

// topics는 사람이 정한 3단계 갈래다. 같은 부모 안에서는 적은 순서가 sort_order다.
//
// **백준 문제 104편을 알고리즘별로 다시 나눈 것이다**(2026-09-02). 노션에서 온
// `백준 단계별로 풀기 1~9 / 10~25 / 알고리즘 강의 / 기타 백준 문제`는 푼 순서라,
// "이분 탐색 문제를 어디서 봤더라"에 아무 답도 못 했다. 어느 문제가 어느 갈래인지는
// `curation.PostMoves`가 한 줄씩 적어둔다.
var topics = []topic{
	{parentSlug: "알고리즘-실전", slug: "bj-수학", name: "수학 & 정수론"},
	{parentSlug: "알고리즘-실전", slug: "bj-문자열", name: "문자열"},
	{parentSlug: "알고리즘-실전", slug: "bj-정렬", name: "정렬"},
	{parentSlug: "알고리즘-실전", slug: "bj-자료구조", name: "자료구조"},
	{parentSlug: "알고리즘-실전", slug: "bj-브루트포스", name: "브루트포스 & 백트래킹"},
	{parentSlug: "알고리즘-실전", slug: "bj-분할정복", name: "분할정복 & 재귀"},
	{parentSlug: "알고리즘-실전", slug: "bj-동적-계획법", name: "동적 계획법"},
	{parentSlug: "알고리즘-실전", slug: "bj-그래프-탐색", name: "그래프 탐색"},
	{parentSlug: "알고리즘-실전", slug: "bj-누적-합", name: "누적 합"},
	{parentSlug: "알고리즘-실전", slug: "bj-기하", name: "기하"},
}

// groupDrops는 **사람이 만든 최상위 분류를 없애는 것**이다.
//
// `curation.DropCategories`는 멱등 키가 source_name이라 노션에서 온 분류만
// 지울 수 있다 — groups가 만든 분류는 그 칸이 NULL이다. 그래서 배정표에서
// 줄만 지우면 그 분류가 "최상위인데 배정표에 없다"로 남아 계획 검사에 걸린다.
//
// **DropCategories와 같은 안전장치를 쓴다**: 글도 자식도 없을 때만 지우고,
// 남은 것이 전부 떠나기로 예정돼 있으면 다음 바퀴로 미룬다.
var groupDrops = []struct{ slug, name, why string }{
	{slug: "career", name: "커리어", why: "취업 준비와 빅데이터 분석기사를 통째로 빼면서 빈 껍데기가 됐다"},
}

// groupRenames는 **최상위 분류 자체의 slug를 바꾼 것**이다.
//
// groups는 slug를 멱등 키로 쓰기 때문에, 배정표에서 slug만 바꾸면 새 행이 하나
// 더 생기고 옛 행은 자식을 안은 채 남는다. 그래서 upsert보다 먼저 옮겨준다.
var groupRenames = []struct{ fromSlug, toSlug, toName string }{
	{fromSlug: "math-stat", toSlug: "data-math", toName: "데이터 & 수리"},
}

// renames는 옮기면서 이름도 바꾸는 카테고리다.
var renames = []rename{
	{fromName: "소프트스킬", toName: "tooling", toSlug: "tooling"},
	// 핸즈온 책 정리가 사실상 이 블로그의 머신러닝 기초 이론이다. 원래 그 이름을
	// 달고 있던 카테고리는 글이 열 편뿐이고 내용이 여기와 겹쳐서 없앴다.
	{fromName: "핸즈온 머신러닝 2", toName: "머신러닝: 기초이론", toSlug: "머신러닝-기초이론"},
	{fromName: "자연어처리 (1) : BERT와 GPT", toName: "자연어처리", toSlug: "자연어처리"},
	// `(2)`가 없어졌으므로 `(1)`이라는 번호도 뜻을 잃었다. 껍데기뿐이던
	// `네트워크 이론 (2)`는 DropPosts로 뺐다.
	{fromName: "네트워크 이론 (1)", toName: "네트워크 이론", toSlug: "네트워크-이론"},
}

// allScheduledToLeave는 이 카테고리에 남은 글과 하위 분류가 **전부** curation의
// 표에 따라 다른 곳으로 갈 예정인지 본다.
//
// 글은 PostMoves가, 하위 분류는 Moves가 데려간다. 하나라도 표에 없으면 false다 —
// 그건 갈 곳이 정해지지 않은 것이고, 지우면 조용히 잃는다.
// topicOrder는 같은 부모 안에서 이 갈래가 설 자리다. 표에 적은 순서를 그대로 쓴다 —
// 갈래 이름은 사람이 지은 것이라 가나다순도 알파벳순도 여기서 뜻이 없다.
//
// **노션에서 온 형제가 남아 있어도 그 뒤로 밀지 않는다.** 지금은 그 형제(옛 백준
// 묶음 넷)가 전부 DropCategories에 있어 이번 실행에서 사라지고, 남겨야 할 형제가
// 생기면 subs가 members 다음부터 세는 것처럼 여기도 그때 고칠 자리다.
func topicOrder(t topic) int {
	n := 0
	for _, o := range topics {
		if o.parentSlug != t.parentSlug {
			continue
		}
		if o.slug == t.slug {
			return n
		}
		n++
	}
	return n
}

func allScheduledToLeave(tx *sql.Tx, id int64) (bool, error) {
	moving := map[string]bool{}
	for _, mp := range curation.PostMoves {
		moving[mp.NotionPageID] = true
	}
	rows, err := tx.Query(`SELECT notion_page_id FROM posts WHERE category_id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("남은 글 조회(%d): %w", id, err)
	}
	defer rows.Close()
	for rows.Next() {
		var pageID sql.NullString
		if err := rows.Scan(&pageID); err != nil {
			return false, fmt.Errorf("남은 글 스캔(%d): %w", id, err)
		}
		if !pageID.Valid || !moving[pageID.String] {
			return false, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}

	// **지워질 자식도 떠나는 것이다.** 예전에는 옮겨지는 것(Moves)만 셌는데,
	// 그러면 자식이 전부 DropCategories에 있는 부모가 "예정에 없는 것이 남았다"로
	// 걸린다. 커리어를 통째로 뺄 때 `취업 준비`가 실제로 그랬다 — 자식 둘이
	// 같은 표에 적혀 있는데도 멈췄다.
	movingCat := map[string]bool{}
	for _, mv := range curation.Moves {
		movingCat[mv.SourceName] = true
	}
	for _, dc := range curation.DropCategories {
		movingCat[dc.SourceName] = true
	}
	kids, err := tx.Query(`SELECT source_name FROM categories WHERE parent_id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("남은 하위 분류 조회(%d): %w", id, err)
	}
	defer kids.Close()
	for kids.Next() {
		var source sql.NullString
		if err := kids.Scan(&source); err != nil {
			return false, fmt.Errorf("남은 하위 분류 스캔(%d): %w", id, err)
		}
		if !source.Valid || !movingCat[source.String] {
			return false, nil
		}
	}
	return true, kids.Err()
}

// category는 DB에 있는 카테고리 한 줄이다.
type category struct {
	id         int64
	name       string
	slug       string
	parentID   sql.NullInt64
	sourceName sql.NullString
	sortOrder  int
	children   int
	// grandchildren은 자식의 자식 수다. 이게 0보다 크면 서브트리 높이가 2라서
	// 새 최상위 밑으로 넣으면 4단계가 된다.
	grandchildren int
	posts         int
}

func main() {
	dbPath := flag.String("db", "blog.db", "SQLite 파일 경로")
	apply := flag.Bool("apply", false, "실제로 DB를 고친다. 없으면 미리보기만")
	flag.Parse()

	sqlDB, err := db.Open(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	cat, err := loadCategories(sqlDB)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := checkPlan(cat); err != nil {
		fmt.Fprintf(os.Stderr, "계획 검사 실패: %v\n", err)
		os.Exit(1)
	}
	printPlan(cat)

	if !*apply {
		fmt.Println("\n미리보기다. 실제로 고치려면 -apply 를 붙여 다시 실행해라.")
		return
	}

	if err := applyGroups(sqlDB, cat); err != nil {
		fmt.Fprintf(os.Stderr, "\n적용 실패: %v\n", err)
		os.Exit(1)
	}
	if err := verify(sqlDB); err != nil {
		fmt.Fprintf(os.Stderr, "\n검증 실패: %v\n", err)
		os.Exit(1)
	}
}

// catalog는 카테고리를 이름과 slug 두 가지로 찾을 수 있게 담는다.
type catalog struct {
	byName   map[string]*category
	bySlug   map[string]*category
	bySource map[string]*category // source_name = 경로에서 온 이름 (사람이 안 바꾼다)
}

// member는 배정표의 이름으로 실제 카테고리를 찾는다.
//
// 두 번 돌려도 같은 걸 찾아야 한다:
//   - 이름을 바꾼 뒤에는 옛 이름으로 못 찾으므로 바뀐 slug로도 찾아본다.
//   - 새 최상위 분류가 기존 카테고리와 이름이 겹칠 수 있다("수학 & 통계"가 그렇다).
//     이름 색인에서 새 분류를 빼두었으므로 원래 카테고리가 잡힌다.
func (c catalog) member(name string) (*category, bool) {
	// source_name이 가장 안정적이다. 사람이 이름을 바꿔도 그대로다.
	if cat, ok := c.bySource[name]; ok {
		return cat, true
	}
	if cat, ok := c.byName[name]; ok {
		return cat, true
	}
	for _, r := range renames {
		if r.fromName == name {
			if cat, ok := c.bySlug[r.toSlug]; ok {
				return cat, true
			}
		}
	}
	return nil, false
}

func loadCategories(sqlDB *sql.DB) (*catalog, error) {
	rows, err := sqlDB.Query(`
		SELECT c.id, c.name, c.slug, c.parent_id, c.source_name, c.sort_order,
		       (SELECT count(*) FROM categories k WHERE k.parent_id = c.id),
		       (SELECT count(*) FROM categories g
		          JOIN categories k ON g.parent_id = k.id
		         WHERE k.parent_id = c.id),
		       (SELECT count(*) FROM posts p WHERE p.category_id = c.id)
		FROM categories c`)
	if err != nil {
		return nil, fmt.Errorf("categories 조회: %w", err)
	}
	defer rows.Close()

	newSlugs := map[string]bool{}
	for _, g := range groups {
		newSlugs[g.slug] = true
		for _, sub := range g.subs {
			newSlugs[sub.slug] = true
		}
	}
	// 아직 옮기기 전이면 옛 slug로 남아 있다. 그것도 "우리가 만든 분류"다.
	for _, r := range groupRenames {
		newSlugs[r.fromSlug] = true
	}
	// 없애기로 한 최상위는 "배정표에 없다"고 나무랄 것이 아니라 지울 것이다.
	for _, d := range groupDrops {
		newSlugs[d.slug] = true
	}

	cat := &catalog{
		byName:   map[string]*category{},
		bySlug:   map[string]*category{},
		bySource: map[string]*category{},
	}
	for rows.Next() {
		var c category
		if err := rows.Scan(&c.id, &c.name, &c.slug, &c.parentID, &c.sourceName, &c.sortOrder,
			&c.children, &c.grandchildren, &c.posts); err != nil {
			return nil, fmt.Errorf("categories 스캔: %w", err)
		}
		row := c
		cat.bySlug[row.slug] = &row
		if row.sourceName.Valid {
			cat.bySource[row.sourceName.String] = &row
		}
		// 새 최상위 분류는 이름 색인에 넣지 않는다. 기존 카테고리와 이름이 겹치면
		// 배정표의 이름이 엉뚱하게 새 분류를 가리키게 된다.
		if !newSlugs[row.slug] {
			cat.byName[row.name] = &row
		}
	}
	return cat, rows.Err()
}

// checkPlan은 계획이 실제 DB와 맞는지 미리 본다.
// 없는 카테고리를 옮기려 하거나, 옮길 대상을 빠뜨렸으면 여기서 멈춘다.
func checkPlan(cat *catalog) error {
	planned := map[string]string{} // 기존 이름 → 새 부모 slug
	newSlugs := map[string]bool{}
	for _, r := range groupRenames {
		newSlugs[r.fromSlug] = true
	}
	for _, g := range groups {
		newSlugs[g.slug] = true
		for _, m := range g.members {
			if prev, dup := planned[m]; dup {
				return fmt.Errorf("%q가 %q와 %q 두 곳에 배정돼 있다", m, prev, g.slug)
			}
			planned[m] = g.slug
		}
		for _, sub := range g.subs {
			newSlugs[sub.slug] = true
			for _, m := range sub.members {
				if prev, dup := planned[m]; dup {
					return fmt.Errorf("%q가 %q와 %q 두 곳에 배정돼 있다", m, prev, sub.slug)
				}
				planned[m] = sub.slug
			}
		}
	}
	// 없앨 카테고리는 배정하지 않는다. 아래 "배정표에 없다" 검사에서 빼준다.
	dropping := map[string]bool{}
	for _, d := range curation.DropCategories {
		dropping[d.SourceName] = true
	}

	var missing []string
	for m := range planned {
		if _, ok := cat.member(m); !ok {
			missing = append(missing, m)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("배정표에 있는데 DB에 없는 카테고리: %s", strings.Join(missing, ", "))
	}

	// 지금 최상위인데 배정표에 없는 것이 있으면 알려준다. 조용히 두면
	// 새 구조 밖에 혼자 남는다.
	var unassigned []string
	for name, c := range cat.byName {
		if c.parentID.Valid || newSlugs[c.slug] {
			continue
		}
		if _, ok := planned[name]; !ok && !dropping[name] {
			unassigned = append(unassigned, name)
		}
	}
	if len(unassigned) > 0 {
		sort.Strings(unassigned)
		return fmt.Errorf("최상위인데 배정표에 없는 카테고리: %s", strings.Join(unassigned, ", "))
	}

	// 이름을 바꾸려는 대상이 실제로 있는지 (이미 바뀐 뒤면 새 slug로 찾힌다)
	for _, r := range renames {
		if _, ok := cat.member(r.fromName); !ok {
			return fmt.Errorf("이름을 바꾸려는 카테고리가 없다: %q", r.fromName)
		}
	}

	// 서브트리 높이가 1을 넘으면 3단계 안에 못 넣는다.
	// (새 최상위 + 이 카테고리 + 그 자식 = 3단계가 상한)
	// 트리거도 막지만, 여기서 먼저 걸러야 어느 카테고리가 문제인지 알 수 있다.
	var tooDeep []string
	for m := range planned {
		c, _ := cat.member(m)
		if c.grandchildren > 0 {
			tooDeep = append(tooDeep, fmt.Sprintf("%s(손자 %d개)", m, c.grandchildren))
		}
	}
	if len(tooDeep) > 0 {
		sort.Strings(tooDeep)
		return fmt.Errorf("손자까지 있어 3단계 안에 못 넣는 카테고리: %s", strings.Join(tooDeep, ", "))
	}

	// moves가 가리키는 양쪽이 다 있는지, 그리고 옮겨도 3단계를 넘지 않는지.
	for _, mv := range curation.Moves {
		c, ok := cat.bySource[mv.SourceName]
		if !ok {
			return fmt.Errorf("옮기려는 카테고리가 없다: source_name %q", mv.SourceName)
		}
		if !newSlugs[mv.ToSlug] {
			return fmt.Errorf("%q의 새 부모 %q가 우리가 만든 분류가 아니다", mv.SourceName, mv.ToSlug)
		}
		// DB에 아직 없어도 된다. newSlugs에 있으면 **이번 실행이 만드는 분류**라
		// 옮기기 전에 생긴다(1a·1b가 3보다 먼저다). 새 층을 처음 얹는 실행이
		// 이 검사에 걸려 멈추던 것을 고쳤다.
		if c.grandchildren > 0 {
			return fmt.Errorf("%q는 손자까지 있어 옮기면 4단계가 된다", mv.SourceName)
		}
	}

	// covers가 가리키는 카테고리가 있는지. 글은 DB를 봐야 알 수 있어 여기선 안 본다.
	//
	// **이번 실행이 만들 slug도 받아들인다.** `renames`가 이름과 slug를 함께
	// 바꾸는 경우, 표지가 가리키는 것은 바뀐 뒤의 slug다. 그런데 그 이름 변경은
	// 이 검사보다 **뒤에** 일어나므로 DB에는 아직 옛 slug만 있다. 여기서 막으면
	// 이름을 바꾸면서 표지를 지정하는 실행이 통째로 멈춘다 —
	// `Moves`의 `ToSlug`를 받아주는 것과 같은 이유다.
	willExist := map[string]bool{}
	for _, r := range renames {
		if _, ok := cat.byName[r.fromName]; ok {
			willExist[r.toSlug] = true
		}
	}
	for _, t := range topics {
		willExist[t.slug] = true
	}
	for _, cv := range curation.Covers {
		if _, ok := cat.bySlug[cv.Slug]; ok {
			continue
		}
		if willExist[cv.Slug] {
			continue
		}
		return fmt.Errorf("표지를 붙일 카테고리가 없다: slug %q", cv.Slug)
	}
	return nil
}

func printPlan(cat *catalog) {
	fmt.Println(rule)
	fmt.Println("카테고리 재구성 계획")
	fmt.Println(rule)

	renameBy := map[string]rename{}
	for _, r := range renames {
		renameBy[r.fromName] = r
	}

	totalMembers, totalChildren, totalPosts := 0, 0, 0
	fmt.Println()
	// 화면에 나갈 순서 그대로 찍는다: members가 먼저고 subs가 그 뒤다.
	line := func(m, prefix string) {
		c, _ := cat.member(m)
		totalMembers++
		totalChildren += c.children
		totalPosts += c.posts
		label := c.name
		if r, ok := renameBy[m]; ok && c.name != r.toName {
			label = fmt.Sprintf("%s → %s (slug: %s → %s)", m, r.toName, c.slug, r.toSlug)
		}
		fmt.Printf("%s%s   [자식 %d개, 직접 붙은 글 %d건]\n", prefix, label, c.children, c.posts)
	}
	for _, g := range groups {
		fmt.Printf("%s  (/%s)\n", g.name, g.slug)
		if len(g.members) == 0 && len(g.subs) == 0 {
			fmt.Println("    (아직 소속 카테고리 없음)")
			continue
		}
		for _, m := range g.members {
			line(m, "    └ ")
		}
		for _, sub := range g.subs {
			fmt.Printf("    ├ %s  (/%s)  [사람이 둔 층]\n", sub.name, sub.slug)
			for _, m := range sub.members {
				line(m, "    │   └ ")
			}
		}
	}

	fmt.Printf("\n새로 만들 최상위 : %d개\n", len(groups))
	fmt.Printf("2단계로 내릴 것  : %d개\n", totalMembers)
	fmt.Printf("3단계가 될 것    : %d개 (딸려 내려가는 하위 카테고리)\n", totalChildren)
	fmt.Printf("이름 변경        : %d개\n", len(renames))

	if len(curation.Moves) > 0 {
		fmt.Printf("\n■ 사람이 정한 카테고리 이동 %d건\n", len(curation.Moves))
		for _, mv := range curation.Moves {
			c := cat.bySource[mv.SourceName]
			from := "(최상위)"
			if c.parentID.Valid {
				for _, x := range cat.byName {
					if x.id == c.parentID.Int64 {
						from = x.name
						break
					}
				}
			}
			fmt.Printf("      %s : %s → /%s   [글 %d건]\n", c.name, from, mv.ToSlug, c.posts)
			fmt.Printf("          %s\n", mv.Why)
		}
	}

	if len(curation.Covers) > 0 {
		fmt.Printf("\n■ 사람이 만든 분류의 표지 글 %d건\n", len(curation.Covers))
		for _, cv := range curation.Covers {
			fmt.Printf("      /%s ← %s\n", cv.Slug, cv.NotionPageID)
			fmt.Printf("          %s\n", cv.Why)
		}
	}

	if len(curation.DropCategories) > 0 {
		fmt.Printf("\n■ 없앨 카테고리 %d개\n", len(curation.DropCategories))
		for _, dc := range curation.DropCategories {
			c, ok := cat.bySource[dc.SourceName]
			if !ok {
				fmt.Printf("      %s   (이미 없다)\n", dc.SourceName)
				continue
			}
			fmt.Printf("      %s   [글 %d건, 하위 분류 %d개]\n", c.name, c.posts, c.children)
			fmt.Printf("          %s\n", dc.Why)
			if c.posts > 0 || c.children > 0 {
				fmt.Println("          ← 딸린 게 있어 지우지 않는다. 먼저 categorize를 돌려라")
			}
		}
	}
}

func applyGroups(sqlDB *sql.DB, cat *catalog) error {
	tx, err := sqlDB.Begin()
	if err != nil {
		return fmt.Errorf("트랜잭션 시작: %w", err)
	}
	defer tx.Rollback()

	// 0) 최상위 분류의 slug를 옮긴다. groups는 slug가 멱등 키라, 먼저 옮기지 않으면
	//    옛 행이 자식을 안은 채 남고 새 행이 하나 더 생긴다.
	for _, r := range groupRenames {
		if _, err := tx.Exec(
			`UPDATE categories SET slug = ?, name = ? WHERE slug = ?`,
			r.toSlug, r.toName, r.fromSlug); err != nil {
			return fmt.Errorf("최상위 분류 slug 변경(%s → %s): %w", r.fromSlug, r.toSlug, err)
		}
	}

	// 1) 새 최상위 분류를 넣는다. slug가 멱등 키다.
	groupID := map[string]int64{}
	for i, g := range groups {
		_, err := tx.Exec(`
			INSERT INTO categories (parent_id, name, slug, sort_order)
			VALUES (NULL, ?, ?, ?)
			ON CONFLICT (slug) DO UPDATE SET
				name       = excluded.name,
				parent_id  = NULL,
				sort_order = excluded.sort_order`,
			g.name, g.slug, i)
		if err != nil {
			return fmt.Errorf("최상위 분류 %q: %w", g.slug, err)
		}
		var id int64
		if err := tx.QueryRow(`SELECT id FROM categories WHERE slug = ?`, g.slug).Scan(&id); err != nil {
			return fmt.Errorf("최상위 분류 id 조회(%s): %w", g.slug, err)
		}
		groupID[g.slug] = id
	}

	// 1b) 사람이 둔 중간 층(subs)을 넣는다. 최상위와 같은 방식이고 부모만 다르다.
	//
	// **sort_order는 members 다음부터 센다.** 둘 다 같은 분류의 자식이라, 0부터
	// 다시 세면 사이드바에서 members와 subs가 같은 자리를 다투게 된다. 개발이
	// 둘을 함께 쓰는 첫 분류다(members 3 + subs 2).
	subID := map[string]int64{}
	for _, g := range groups {
		for i, sub := range g.subs {
			_, err := tx.Exec(`
				INSERT INTO categories (parent_id, name, slug, sort_order)
				VALUES (?, ?, ?, ?)
				ON CONFLICT (slug) DO UPDATE SET
					name       = excluded.name,
					parent_id  = excluded.parent_id,
					sort_order = excluded.sort_order`,
				groupID[g.slug], sub.name, sub.slug, len(g.members)+i)
			if err != nil {
				return fmt.Errorf("중간 분류 %q: %w", sub.slug, err)
			}
			var id int64
			if err := tx.QueryRow(`SELECT id FROM categories WHERE slug = ?`, sub.slug).Scan(&id); err != nil {
				return fmt.Errorf("중간 분류 id 조회(%s): %w", sub.slug, err)
			}
			subID[sub.slug] = id
		}
	}

	// 1b2) 사람이 기존 분류 아래에 두는 갈래(topics)를 넣는다.
	//
	// **부모의 깊이를 여기서 확인한다.** 3단계가 끝이라 부모가 2단계여야 하는데,
	// 넘으면 002의 트리거가 막아준다 — 다만 그때 나오는 말은 "트리거가 거부했다"라
	// 무엇이 잘못됐는지 알려주지 않는다. 여기서 먼저 보고 이름으로 말한다.
	for _, t := range topics {
		var parentID int64
		var grandParent sql.NullInt64
		err := tx.QueryRow(
			`SELECT id, parent_id FROM categories WHERE slug = ?`, t.parentSlug).Scan(&parentID, &grandParent)
		if err == sql.ErrNoRows {
			return fmt.Errorf("갈래 %q의 부모 %q가 없다", t.slug, t.parentSlug)
		}
		if err != nil {
			return fmt.Errorf("갈래 부모 조회(%s): %w", t.parentSlug, err)
		}
		if !grandParent.Valid {
			return fmt.Errorf("갈래 %q의 부모 %q가 최상위다. 그 아래는 2단계라 갈래를 두면 3단계를 넘지 않지만, "+
				"이 표는 3단계 갈래를 위한 것이라 부모가 2단계여야 한다", t.slug, t.parentSlug)
		}
		if _, err := tx.Exec(`
			INSERT INTO categories (parent_id, name, slug, sort_order)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (slug) DO UPDATE SET
				name       = excluded.name,
				parent_id  = excluded.parent_id,
				sort_order = excluded.sort_order`,
			parentID, t.name, t.slug, topicOrder(t)); err != nil {
			return fmt.Errorf("갈래 %q: %w", t.slug, err)
		}
	}

	// 1c) 사람이 없애기로 한 카테고리를 **이름 변경보다 먼저** 지운다.
	//     없앨 카테고리가 쓰던 slug를 다른 카테고리가 물려받는 경우가 있어서,
	//     나중에 지우면 UNIQUE에 걸린다(핸즈온 머신러닝 2 → 머신러닝: 기초이론).
	// 글도 자식도 없을 때만 지운다.
	//    딸린 게 남아 있으면 조용히 잃지 않도록 에러로 멈춘다.
	dropped := 0
	for _, dc := range curation.DropCategories {
		var id int64
		switch err := tx.QueryRow(
			`SELECT id FROM categories WHERE source_name = ?`, dc.SourceName).Scan(&id); err {
		case nil:
		case sql.ErrNoRows:
			continue // 이미 지워졌다
		default:
			return fmt.Errorf("지울 카테고리 조회(%s): %w", dc.SourceName, err)
		}

		var posts, kids int
		if err := tx.QueryRow(
			`SELECT (SELECT count(*) FROM posts p WHERE p.category_id = c.id),
			        (SELECT count(*) FROM categories k WHERE k.parent_id = c.id)
			 FROM categories c WHERE c.id = ?`, id).Scan(&posts, &kids); err != nil {
			return fmt.Errorf("딸린 것 조회(%s): %w", dc.SourceName, err)
		}
		if posts > 0 || kids > 0 {
			// **떠나기로 예정된 것만 남았으면 이번엔 미룬다.** 새 층을 막 만든
			// 직후가 그렇다 — categorize가 글을 옮기려면 그 층이 있어야 하는데,
			// 그 층을 만드는 게 바로 이 실행이다. 여기서 에러를 내면 두 도구가
			// 서로를 기다리며 아무것도 못 한다. 한 바퀴 더 돌면 비고 지워진다.
			//
			// 예정에 없는 것이 하나라도 남아 있으면 예전대로 멈춘다. 조용히
			// 지워서 글을 잃는 것이 이 검사가 막으려는 일이다.
			leaving, err := allScheduledToLeave(tx, id)
			if err != nil {
				return err
			}
			if !leaving {
				return fmt.Errorf(
					"카테고리 %q를 지우려는데 글 %d건, 하위 분류 %d개가 남아 있다. "+
						"먼저 curation.PostMoves로 옮겨라", dc.SourceName, posts, kids)
			}
			fmt.Printf("카테고리 %q 삭제를 미뤘다 (글 %d건, 하위 분류 %d개가 아직 있다. "+
				"categorize를 돌린 뒤 다시 실행하면 지워진다)\n", dc.SourceName, posts, kids)
			continue
		}
		if _, err := tx.Exec(`DELETE FROM categories WHERE id = ?`, id); err != nil {
			return fmt.Errorf("카테고리 삭제(%s): %w", dc.SourceName, err)
		}
		dropped++
	}

	// 1d) 사람이 만든 최상위 중 없애기로 한 것을 지운다. 그 자식(취업 준비 등)은
	//     바로 위 1c가 이미 지웠으므로 이 시점에는 비어 있다.
	for _, gd := range groupDrops {
		var id int64
		switch err := tx.QueryRow(
			`SELECT id FROM categories WHERE slug = ?`, gd.slug).Scan(&id); err {
		case nil:
		case sql.ErrNoRows:
			continue // 이미 지워졌다
		default:
			return fmt.Errorf("지울 최상위 조회(%s): %w", gd.slug, err)
		}

		var posts, kids int
		if err := tx.QueryRow(
			`SELECT (SELECT count(*) FROM posts p WHERE p.category_id = c.id),
			        (SELECT count(*) FROM categories k WHERE k.parent_id = c.id)
			 FROM categories c WHERE c.id = ?`, id).Scan(&posts, &kids); err != nil {
			return fmt.Errorf("딸린 것 조회(%s): %w", gd.slug, err)
		}
		if posts > 0 || kids > 0 {
			leaving, err := allScheduledToLeave(tx, id)
			if err != nil {
				return err
			}
			if !leaving {
				return fmt.Errorf(
					"최상위 %q를 지우려는데 글 %d건, 하위 분류 %d개가 남아 있다. "+
						"먼저 curation으로 옮기거나 빼라", gd.name, posts, kids)
			}
			fmt.Printf("최상위 %q 삭제를 미뤘다 (글 %d건, 하위 분류 %d개가 아직 있다)\n",
				gd.name, posts, kids)
			continue
		}
		if _, err := tx.Exec(`DELETE FROM categories WHERE id = ?`, id); err != nil {
			return fmt.Errorf("최상위 삭제(%s): %w", gd.slug, err)
		}
		dropped++
	}

	// 2) 이름을 바꿀 것부터 처리한다. 옮긴 뒤에 하면 이름으로 못 찾는다.
	for _, r := range renames {
		c, ok := cat.member(r.fromName)
		if !ok {
			return fmt.Errorf("이름을 바꾸려는 카테고리가 없다: %q", r.fromName)
		}
		if c.name == r.toName && c.slug == r.toSlug {
			continue // 이미 바뀌어 있다
		}
		if _, err := tx.Exec(
			`UPDATE categories SET name = ?, slug = ? WHERE id = ?`, r.toName, r.toSlug, c.id); err != nil {
			return fmt.Errorf("이름 변경(%s → %s): %w", r.fromName, r.toName, err)
		}
	}

	// 3) 기존 카테고리를 새 분류 밑으로 옮긴다.
	//    트리거가 깊이를 검사하므로, 3단계를 넘기면 여기서 에러가 난다.
	moved := 0
	for _, g := range groups {
		for j, m := range g.members {
			c, ok := cat.member(m)
			if !ok {
				return fmt.Errorf("옮기려는 카테고리가 없다: %q", m)
			}
			if c.parentID.Valid && c.parentID.Int64 == groupID[g.slug] && c.sortOrder == j {
				continue // 이미 그 밑에 있다
			}
			if _, err := tx.Exec(
				`UPDATE categories SET parent_id = ?, sort_order = ? WHERE id = ?`,
				groupID[g.slug], j, c.id); err != nil {
				return fmt.Errorf("%q를 %q 밑으로 이동: %w", m, g.slug, err)
			}
			moved++
		}
		for _, sub := range g.subs {
			for j, m := range sub.members {
				c, ok := cat.member(m)
				if !ok {
					return fmt.Errorf("옮기려는 카테고리가 없다: %q", m)
				}
				if c.parentID.Valid && c.parentID.Int64 == subID[sub.slug] && c.sortOrder == j {
					continue
				}
				if _, err := tx.Exec(
					`UPDATE categories SET parent_id = ?, sort_order = ? WHERE id = ?`,
					subID[sub.slug], j, c.id); err != nil {
					return fmt.Errorf("%q를 %q 밑으로 이동: %w", m, sub.slug, err)
				}
				moved++
			}
		}
	}

	// 4) 사람이 정한 개별 이동. groups가 끝난 뒤에 해야 새 부모 id가 다 있다.
	handMoved := 0
	for _, mv := range curation.Moves {
		c, ok := cat.bySource[mv.SourceName]
		if !ok {
			return fmt.Errorf("옮기려는 카테고리가 없다: source_name %q", mv.SourceName)
		}
		parentID, ok := groupID[mv.ToSlug]
		if !ok {
			// 사람이 둔 중간 층도 부모가 될 수 있다.
			parentID, ok = subID[mv.ToSlug]
		}
		if !ok {
			return fmt.Errorf("%q의 새 부모 %q를 못 찾았다", mv.SourceName, mv.ToSlug)
		}
		if c.parentID.Valid && c.parentID.Int64 == parentID {
			continue // 이미 그 밑에 있다
		}
		if _, err := tx.Exec(
			`UPDATE categories SET parent_id = ? WHERE id = ?`, parentID, c.id); err != nil {
			return fmt.Errorf("%q를 %q 밑으로 이동: %w", mv.SourceName, mv.ToSlug, err)
		}
		handMoved++
	}

	// 5) 사람이 만든 분류의 표지 글. notion_page_id가 멱등 키다.
	coverSet := 0
	for _, cv := range curation.Covers {
		var postID int64
		switch err := tx.QueryRow(
			`SELECT id FROM posts WHERE notion_page_id = ?`, cv.NotionPageID).Scan(&postID); err {
		case nil:
		case sql.ErrNoRows:
			return fmt.Errorf("표지로 쓸 글이 없다: notion_page_id %s", cv.NotionPageID)
		default:
			return fmt.Errorf("표지 글 조회(%s): %w", cv.NotionPageID, err)
		}

		// cover_post_id는 UNIQUE다. 한 글은 한 카테고리의 표지만 될 수 있다.
		// 그 글이 노션 최상위 페이지라면 categorize가 이미 자기 카테고리에 붙여뒀다.
		// 새로 만드는 게 아니라 옮기는 것이므로 옛 자리를 먼저 비운다.
		if _, err := tx.Exec(
			`UPDATE categories SET cover_post_id = NULL
			 WHERE cover_post_id = ? AND slug <> ?`, postID, cv.Slug); err != nil {
			return fmt.Errorf("옛 표지 자리 비우기(%s): %w", cv.Slug, err)
		}

		res, err := tx.Exec(`
			UPDATE categories SET cover_post_id = ?
			WHERE slug = ? AND (cover_post_id IS NULL OR cover_post_id <> ?)`,
			postID, cv.Slug, postID)
		if err != nil {
			return fmt.Errorf("표지 지정(%s): %w", cv.Slug, err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			coverSet++
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("커밋: %w", err)
	}
	fmt.Printf("\n최상위 분류 %d개 확보, %d개 카테고리를 옮겼다.\n", len(groups), moved)
	fmt.Printf("사람이 정한 개별 이동 %d건, 표지 지정 %d건, 카테고리 삭제 %d개.\n",
		handMoved, coverSet, dropped)
	return nil
}

func verify(sqlDB *sql.DB) error {
	fmt.Println()
	fmt.Println(rule)
	fmt.Println("검증")
	fmt.Println(rule)

	depths := map[int]int{}
	rows, err := sqlDB.Query(`
		WITH RECURSIVE d(id, depth) AS (
			SELECT id, 1 FROM categories WHERE parent_id IS NULL
			UNION ALL
			SELECT c.id, d.depth + 1 FROM categories c JOIN d ON c.parent_id = d.id
		)
		SELECT depth, count(*) FROM d GROUP BY depth ORDER BY depth`)
	if err != nil {
		return fmt.Errorf("깊이 조회: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var depth, n int
		if err := rows.Scan(&depth, &n); err != nil {
			return fmt.Errorf("깊이 스캔: %w", err)
		}
		depths[depth] = n
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("깊이 조회: %w", err)
	}

	fmt.Printf("\n깊이별 카테고리: 1단계 %d, 2단계 %d, 3단계 %d\n",
		depths[1], depths[2], depths[3])

	if depths[1] != len(groups) {
		return fmt.Errorf("1단계가 %d개다. %d개여야 한다", depths[1], len(groups))
	}
	fmt.Printf("1단계가 계획한 %d개인가: 예 ✓\n", len(groups))

	for d := 4; d <= 10; d++ {
		if depths[d] > 0 {
			return fmt.Errorf("%d단계 카테고리가 %d개 있다", d, depths[d])
		}
	}
	fmt.Println("4단계 이상: 0개 ✓")

	// 트리에 들어가지 못한 카테고리가 없는지 (순환 등으로 재귀에서 빠진 것)
	var total, reached int
	if err := sqlDB.QueryRow(`SELECT count(*) FROM categories`).Scan(&total); err != nil {
		return fmt.Errorf("전체 수 조회: %w", err)
	}
	for _, n := range depths {
		reached += n
	}
	fmt.Printf("트리에 닿는 카테고리: %d / %d개%s\n", reached, total, mark(reached == total))
	if reached != total {
		return fmt.Errorf("트리에서 닿지 않는 카테고리가 있다 (순환 가능성)")
	}

	// 글이 여전히 유효한 카테고리를 가리키는지
	var badRef int
	err = sqlDB.QueryRow(`
		SELECT count(*) FROM posts
		WHERE category_id IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM categories c WHERE c.id = posts.category_id)`).Scan(&badRef)
	if err != nil {
		return fmt.Errorf("category_id 참조 검사: %w", err)
	}
	fmt.Printf("없는 카테고리를 가리키는 글: %d건%s\n", badRef, mark(badRef == 0))
	if badRef != 0 {
		return fmt.Errorf("category_id가 깨졌다")
	}

	// moves가 실제로 반영됐는지
	for _, mv := range curation.Moves {
		var parentSlug string
		err := sqlDB.QueryRow(`
			SELECT coalesce((SELECT p.slug FROM categories p WHERE p.id = c.parent_id), '')
			FROM categories c WHERE c.source_name = ?`, mv.SourceName).Scan(&parentSlug)
		if err != nil {
			return fmt.Errorf("이동 검증(%s): %w", mv.SourceName, err)
		}
		fmt.Printf("%s의 부모: /%s%s\n", mv.SourceName, parentSlug, mark(parentSlug == mv.ToSlug))
		if parentSlug != mv.ToSlug {
			return fmt.Errorf("%q가 /%s 밑에 있어야 하는데 /%s 밑에 있다", mv.SourceName, mv.ToSlug, parentSlug)
		}
	}

	// covers가 실제로 반영됐는지
	for _, cv := range curation.Covers {
		var ok bool
		err := sqlDB.QueryRow(`
			SELECT c.cover_post_id IS NOT NULL AND c.cover_post_id = p.id
			FROM categories c JOIN posts p ON p.notion_page_id = ?
			WHERE c.slug = ?`, cv.NotionPageID, cv.Slug).Scan(&ok)
		if err != nil {
			return fmt.Errorf("표지 검증(%s): %w", cv.Slug, err)
		}
		fmt.Printf("/%s의 표지 글 지정%s\n", cv.Slug, mark(ok))
		if !ok {
			return fmt.Errorf("/%s에 표지 글이 안 붙었다", cv.Slug)
		}
	}

	// 없애기로 한 카테고리가 실제로 사라졌는지.
	//
	// **미룬 것은 실패가 아니다.** 남은 것이 전부 떠나기로 예정돼 있으면 위에서
	// 지우지 않고 넘긴다(allScheduledToLeave). 그건 categorize를 한 번 더 돌리면
	// 풀리는 상태라, 여기서 멈추면 수렴할 길이 없다. 대신 아직 남았다고 찍는다.
	for _, dc := range curation.DropCategories {
		var id int64
		switch err := sqlDB.QueryRow(
			`SELECT id FROM categories WHERE source_name = ?`, dc.SourceName).Scan(&id); err {
		case sql.ErrNoRows:
			fmt.Printf("카테고리 %q 삭제됨%s\n", dc.SourceName, mark(true))
			continue
		case nil:
		default:
			return fmt.Errorf("삭제 검증(%s): %w", dc.SourceName, err)
		}

		tx, err := sqlDB.Begin()
		if err != nil {
			return fmt.Errorf("삭제 검증 트랜잭션(%s): %w", dc.SourceName, err)
		}
		leaving, err := allScheduledToLeave(tx, id)
		tx.Rollback()
		if err != nil {
			return err
		}
		if !leaving {
			fmt.Printf("카테고리 %q 삭제됨%s\n", dc.SourceName, mark(false))
			return fmt.Errorf("카테고리 %q가 아직 있다", dc.SourceName)
		}
		fmt.Printf("카테고리 %q 삭제 대기 (categorize를 돌린 뒤 다시 실행하면 지워진다)\n", dc.SourceName)
	}
	return nil
}

func mark(ok bool) string {
	if ok {
		return " ✓"
	}
	return " ✗"
}
