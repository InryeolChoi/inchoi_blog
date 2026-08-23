package web

import (
	"database/sql"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Category는 목록에 보여줄 카테고리 한 줄이다.
type Category struct {
	ID       int64
	Name     string
	Slug     string
	ParentID sql.NullInt64
	// PostCount는 이 카테고리와 그 아래 전체에 붙은 글 수다.
	PostCount int
	// CoverPostSlug는 이 카테고리의 표지 글이다. 없으면 빈 문자열이다.
	// 카테고리를 열면 이 글의 본문을 목록 위에 그대로 펼쳐 보여준다.
	CoverPostSlug string
}

// PostSummary는 목록에 보여줄 글 한 줄이다.
type PostSummary struct {
	ID        int64
	ParentID  sql.NullInt64
	Slug      string
	Title     string
	Status    string
	SortOrder int
	// CreatedAt은 노션 원본의 작성일이다(이관 시점이 아니다).
	// sort_order가 이 값에서 나온 목록이 많아서, 순서가 이상해 보일 때
	// 근거를 눈으로 확인할 수 있게 목록에 같이 찍는다.
	CreatedAt sql.NullTime
	// Children은 이 글에 달린 하위 글이다. nestPosts가 채운다.
	Children []PostSummary
	// Hidden은 이 글이 남에게 안 보이는 글(draft)이라는 표시다.
	//
	// **목록 조회는 애초에 이런 글을 안 돌려준다.** 이 자리가 필요한 곳은
	// 본문 링크를 풀 때뿐이다 — 링크 대상이 "숨긴 글"인지 "아예 없는 것"인지
	// 구별해야 하기 때문이다. inline.go 참고.
	Hidden bool
}

// Date는 목록에 찍을 날짜다. 값이 없으면 빈 문자열이라 템플릿에서 저절로 빠진다.
func (p PostSummary) Date() string {
	if !p.CreatedAt.Valid {
		return ""
	}
	return p.CreatedAt.Time.Format("2006-01-02")
}

// Post는 글 상세다.
type Post struct {
	ID           int64
	ParentID     sql.NullInt64
	Slug         string
	Title        string
	Body         string
	Status       string
	OriginalPath sql.NullString
	CreatedAt    sql.NullTime
	// Trail은 최상위부터 이 글이 속한 카테고리까지의 경로다.
	Trail []Category
	// Ancestors는 위에서부터 이 글의 부모까지의 글 사슬이다.
	Ancestors []PostSummary
	// Children은 이 글의 직속 하위 글이다.
	Children []PostSummary
}

// Image는 이미지 한 장이다.
type Image struct {
	Data []byte
	MIME string
}

// LanguageBranch는 `Language > 프로그래밍 언어` 안의 언어 한 갈래다.
// URL은 그 언어의 표지 글로 가고 Count는 같은 언어 아래 전체 글 수다.
type LanguageBranch struct {
	Name  string
	Slug  string
	Count int
}

// store는 DB 조회를 모아둔 것이다.
//
// **draft를 가리는 판정은 여기 한 곳에만 둔다.** 핸들러마다 챙기게 하면 새 조회를
// 더할 때마다 구멍이 하나씩 생긴다 — 사이드바를 render가 한 번에 채우는 것과 같은
// 이유다. showDrafts가 참이면(로컬 `-drafts`) 예전처럼 전부 보인다.
type store struct {
	db         *sql.DB
	showDrafts bool
}

// hidden은 "이 글은 남에게 안 보인다"를 판정하는 SQL 조각이다.
// alias는 posts 테이블의 별칭이다. 보이는 것만 원하면 notHidden을 쓴다.
func (s *store) notHidden(alias string) string {
	if s.showDrafts {
		return "1=1"
	}
	return alias + `.status <> 'draft'`
}

// subtreePostCount는 카테고리와 그 하위 전체의 **보이는** 글 수를 세는 SQL 조각이다.
// 카테고리는 3단계까지라 재귀 CTE로 아래를 훑는다.
//
// **숫자와 실제로 보이는 목록이 어긋나면 숫자가 거짓말이 된다.** 사이드바의
// `노트 N편`도 이 값을 최상위끼리 더한 것이라 같이 따라온다.
func (s *store) subtreePostCount() string {
	return `
	(SELECT count(*) FROM posts p WHERE ` + s.notHidden("p") + ` AND p.category_id IN (
		WITH RECURSIVE sub(id) AS (
			SELECT c2.id FROM categories c2 WHERE c2.id = c.id
			UNION ALL
			SELECT k.id FROM categories k JOIN sub ON k.parent_id = sub.id
		) SELECT id FROM sub
	))`
}

// visibleCategory는 화면에 내보낼 분류인지 판정하는 SQL 조각이다.
//
// **하위까지 보이는 글이 0인 노션 유래 분류는 감춘다.** draft를 가리면
// `알고리즘 : 기초 (1)`처럼 통째로 비는 마디가 생기는데, 목록에 `0`으로 남겨두면
// 눌러도 빈 화면인 막다른 길이 된다. 그런 마디는 덤프가 만든 것이지 사람이
// 세운 것이 아니다.
//
// **사람이 만든 분류(`source_name IS NULL`)는 비어도 남긴다.** `라이프`가
// 그렇다 — 앞으로 채울 자리로 일부러 만든 선반이라, 비었다고 치우면 그
// 약속까지 지우는 셈이다. 이 구분은 schema에 이미 있는 것을 그대로 쓴다.
func (s *store) visibleCategory() string {
	return `(c.source_name IS NULL OR ` + s.subtreePostCount() + ` > 0)`
}

// coverSlug는 카테고리의 표지 글 slug를 가져오는 SQL 조각이다.
//
// **표지가 숨긴 글이면 빈 값을 준다.** 표지는 본문을 목록 위에 통째로 펼치므로,
// 안 거르면 draft 본문이 그대로 공개된다. 빈 값이면 핸들러가 표지 없는 분류로
// 다루어 평소 목록을 그린다. 현재 pipex·Shell 두 곳이 여기 해당한다.
func (s *store) coverSlug() string {
	return `(SELECT p.slug FROM posts p WHERE p.id = c.cover_post_id AND ` +
		s.notHidden("p") + `)`
}

func (s *store) scanCategories(rows *sql.Rows) ([]Category, error) {
	defer rows.Close()
	var out []Category
	for rows.Next() {
		var c Category
		var cover sql.NullString
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.ParentID, &c.PostCount, &cover); err != nil {
			return nil, fmt.Errorf("카테고리 스캔: %w", err)
		}
		c.CoverPostSlug = cover.String
		out = append(out, c)
	}
	return out, rows.Err()
}

// TopCategories는 최상위 카테고리를 sort_order 순으로 돌려준다.
func (s *store) TopCategories() ([]Category, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.name, c.slug, c.parent_id, ` + s.subtreePostCount() + `, ` + s.coverSlug() + `
		FROM categories c
		WHERE c.parent_id IS NULL AND ` + s.visibleCategory() + `
		ORDER BY c.sort_order, c.name`)
	if err != nil {
		return nil, fmt.Errorf("최상위 카테고리 조회: %w", err)
	}
	return s.scanCategories(rows)
}

// ChildCategories는 주어진 카테고리의 바로 아래 자식들을 돌려준다.
func (s *store) ChildCategories(parentID int64) ([]Category, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.name, c.slug, c.parent_id, `+s.subtreePostCount()+`, `+s.coverSlug()+`
		FROM categories c
		WHERE c.parent_id = ? AND `+s.visibleCategory()+`
		ORDER BY c.sort_order, c.name`, parentID)
	if err != nil {
		return nil, fmt.Errorf("하위 카테고리 조회: %w", err)
	}
	return s.scanCategories(rows)
}

// CategoryBySlug는 slug로 카테고리를 찾는다. 부모까지 맞는지 함께 확인해서
// /a/b 경로의 b가 정말 a의 자식일 때만 찾히게 한다.
func (s *store) CategoryBySlug(slug string, parentID sql.NullInt64) (*Category, error) {
	q := `
		SELECT c.id, c.name, c.slug, c.parent_id, ` + s.subtreePostCount() + `, ` + s.coverSlug() + `
		FROM categories c
		WHERE c.slug = ? AND c.parent_id IS NULL`
	args := []any{slug}
	if parentID.Valid {
		q = `
		SELECT c.id, c.name, c.slug, c.parent_id, ` + s.subtreePostCount() + `, ` + s.coverSlug() + `
		FROM categories c
		WHERE c.slug = ? AND c.parent_id = ?`
		args = append(args, parentID.Int64)
	}

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("카테고리 조회(%s): %w", slug, err)
	}
	cats, err := s.scanCategories(rows)
	if err != nil {
		return nil, err
	}
	if len(cats) == 0 {
		return nil, nil
	}
	return &cats[0], nil
}

// postColumns는 목록 한 줄에 필요한 컬럼이다.
const postColumns = `p.id, p.parent_id, p.slug, p.title, p.status, p.sort_order,
	       p.original_created_at`

func scanPostSummaries(rows *sql.Rows) ([]PostSummary, error) {
	defer rows.Close()
	var out []PostSummary
	for rows.Next() {
		var p PostSummary
		if err := rows.Scan(&p.ID, &p.ParentID, &p.Slug, &p.Title, &p.Status,
			&p.SortOrder, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("글 스캔: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PostsInCategory는 카테고리에 직접 붙은 글을 sort_order 순으로 돌려준다.
// 상단에 본문을 이미 펼친 표지 글은 목록에서 빼고, 하위 카테고리의 글도 포함하지 않는다.
//
// parent_id로 계층을 되살려 중첩된 형태로 돌려준다. 노션 계층이 카테고리 3단계보다
// 깊었던 글들은 같은 카테고리 안에서 서로 부모-자식이라, 그냥 나열하면 한 층으로
// 평평해진다.
func (s *store) PostsInCategory(categoryID int64) ([]PostSummary, error) {
	rows, err := s.db.Query(`
		SELECT `+postColumns+`
		FROM posts p
		JOIN categories c ON c.id = p.category_id
		WHERE p.category_id = ? AND `+s.notHidden("p")+`
		  AND (c.cover_post_id IS NULL OR p.id <> c.cover_post_id)
		ORDER BY p.sort_order, p.title`, categoryID)
	if err != nil {
		return nil, fmt.Errorf("카테고리 글 조회: %w", err)
	}
	flat, err := scanPostSummaries(rows)
	if err != nil {
		return nil, err
	}
	sortPosts(flat)
	return nestPosts(flat), nil
}

// LanguageBranches는 평평한 `프로그래밍 언어` 카테고리를 original_path의
// 세 번째 칸(C, C++, Java …)으로 다시 묶는다. 카테고리 트리는 3단계가 끝이라
// 언어별 갈래가 categories에는 없지만, 원래 노션 경로에는 그대로 남아 있다.
//
// 내용 없는 표지 글만 있는 언어는 보여주지 않는다. 눌렀는데 빈 화면으로 가는
// Swift 카드가 실제로 그 경우다.
func (s *store) LanguageBranches(categoryID int64) ([]LanguageBranch, error) {
	rows, err := s.db.Query(`
		SELECT p.slug, p.body, p.original_path
		FROM posts p
		WHERE p.category_id = ? AND `+s.notHidden("p")+`
		ORDER BY p.sort_order, p.title`, categoryID)
	if err != nil {
		return nil, fmt.Errorf("언어별 글 조회: %w", err)
	}
	defer rows.Close()

	type branchState struct {
		rootSlug string
		rootBody string
		count    int
	}
	branches := map[string]*branchState{}
	for rows.Next() {
		var slug, body string
		var path sql.NullString
		if err := rows.Scan(&slug, &body, &path); err != nil {
			return nil, fmt.Errorf("언어별 글 스캔: %w", err)
		}
		parts := strings.Split(path.String, " > ")
		if len(parts) < 3 || parts[0] != "Language" || parts[1] != "프로그래밍 언어" {
			continue
		}
		name := parts[2]
		state := branches[name]
		if state == nil {
			state = &branchState{}
			branches[name] = state
		}
		state.count++
		if len(parts) == 3 {
			state.rootSlug = slug
			state.rootBody = body
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	order := []string{"C", "C++", "Java", "Python", "R", "TypeScript", "Swift"}
	out := make([]LanguageBranch, 0, len(order))
	for _, name := range order {
		state := branches[name]
		if state == nil || state.rootSlug == "" || strings.TrimSpace(state.rootBody) == "" {
			continue
		}
		out = append(out, LanguageBranch{Name: name, Slug: state.rootSlug, Count: state.count})
	}
	return out, nil
}

// leadingNumber는 제목 맨 앞의 번호를 잡는다. "10. 다중공선성" → 10.
//
// 구분자(. 또는 ))와 공백을 반드시 요구한다. 안 그러면 "1주차 정리"나
// "2022년 탐자 1차 시험" 같은 제목의 앞자리를 번호로 착각한다.
var leadingNumber = regexp.MustCompile(`^\s*(\d{1,4})\s*[.)]\s+`)

// postNumber는 제목 앞 번호를 읽는다. 없으면 두 번째 값이 false다.
func postNumber(title string) (int, bool) {
	m := leadingNumber.FindStringSubmatch(title)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// naturalCompare는 제목 둘을 자연 정렬로 견준다. a<b면 음수다.
//
// 제목을 **숫자 덩어리와 글자 덩어리로 갈라** 앞에서부터 맞춰 본다. 숫자끼리는
// 값으로, 글자끼리는 글자로 견주고, 숫자와 글자가 만나면 숫자가 앞이다.
//
// 그냥 글자로 견주면 `practice problem 10`이 `practice problem 2`보다 앞에 온다.
// 번호가 제목 **끝이나 중간**에 붙은 시리즈가 그래서 엇갈려 보였다 —
// `postNumber`는 제목 맨 앞의 번호만 읽으므로 이런 제목은 전부 "번호 없음"으로
// 들어와 sort_order 순, 즉 노션 작성 시각 순으로 흩어졌다.
func naturalCompare(a, b string) int {
	ar, br := []rune(a), []rune(b)
	i, j := 0, 0
	for i < len(ar) && j < len(br) {
		ad, bd := isDigit(ar[i]), isDigit(br[j])
		switch {
		case ad && bd:
			// 숫자 덩어리는 값으로 견준다. 앞의 0은 값에 영향이 없으므로
			// 길이가 아니라 자릿수를 세어 비교한다.
			ai, av := scanNumber(ar, i)
			bj, bv := scanNumber(br, j)
			if av != bv {
				if av < bv {
					return -1
				}
				return 1
			}
			i, j = ai, bj
		case ad != bd:
			// 숫자가 글자보다 앞이다. `2022년 …`이 `practice …`보다 앞에 온다.
			if ad {
				return -1
			}
			return 1
		case ar[i] != br[j]:
			if ar[i] < br[j] {
				return -1
			}
			return 1
		default:
			i, j = i+1, j+1
		}
	}
	// 앞이 같으면 짧은 쪽이 앞이다.
	switch {
	case len(ar)-i < len(br)-j:
		return -1
	case len(ar)-i > len(br)-j:
		return 1
	}
	return 0
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

// scanNumber는 i부터 이어지는 숫자 덩어리를 읽어 끝 위치와 값을 준다.
// 값이 int를 넘칠 만큼 긴 숫자는 더 세지 않고 큰 값으로 둔다 — 제목에 그런
// 숫자가 있으면 순서가 아니라 글자다.
func scanNumber(rs []rune, i int) (int, int) {
	v := 0
	for i < len(rs) && isDigit(rs[i]) {
		if v < 1<<40 {
			v = v*10 + int(rs[i]-'0')
		}
		i++
	}
	return i, v
}

// sortPosts는 목록을 제목 앞 번호대로 다시 세운다.
//
// **sort_order를 믿을 수 없어서 필요하다.** 데이터베이스 행의 sort_order는
// created_time 순위에서 나왔는데(노션이 DB 뷰의 화면 순서를 API로 안 준다),
// created_time은 분 단위라 한 번에 만든 글이 같은 값을 갖는다. 그래서
// "10. 무중단 배포"가 "2. 테스트 코드"보다 앞에 오는 목록이 실제로 있다.
// 제목 앞 번호는 사람이 직접 붙인 것이라 이보다 훨씬 믿을 만하다.
//
// 번호가 없는 글은 **번호 있는 글들 사이에 끼우지 않고 뒤로 보낸다.** 어느
// 번호 옆에 놓아야 하는지 알 방법이 없어서다.
//
// 그 뒤쪽은 **제목 자연 정렬**로 세운다(naturalCompare). 예전에는 들어온 순서
// (sort_order, title)를 그대로 썼는데, 그 sort_order가 created_time 순위라
// `practice problem 1 / 2022년 탐자 1차 / practice problem 2 / 연습문제 …`처럼
// 시리즈가 서로 엇갈려 보였다. 제목에 이미 번호가 적혀 있는데 그걸 안 읽고
// 작성 시각을 따르는 것이라, 없는 순서를 지어내는 것과는 반대쪽 문제였다.
func sortPosts(in []PostSummary) {
	sort.SliceStable(in, func(i, j int) bool {
		ni, oki := postNumber(in[i].Title)
		nj, okj := postNumber(in[j].Title)
		if oki != okj {
			return oki
		}
		if oki && ni != nj {
			return ni < nj
		}
		return naturalCompare(in[i].Title, in[j].Title) < 0
	})
}

// nestPosts는 평평한 목록을 parent_id로 묶는다. 들어온 순서를 형제 순서로 그대로 쓴다.
//
// 뿌리는 parent_id가 NULL인 글과, 부모가 이 목록 밖에 있는 글이다. 후자가 흔하다.
// 카테고리의 최상단 글들은 부모가 그 카테고리의 표지 글(보통 한 단계 위 카테고리에
// 붙어 있다)이기 때문이다.
func nestPosts(flat []PostSummary) []PostSummary {
	if len(flat) == 0 {
		return nil
	}

	idx := make(map[int64]int, len(flat))
	for i, p := range flat {
		idx[p.ID] = i
	}

	kids := make([][]int, len(flat))
	var roots []int
	for i, p := range flat {
		if p.ParentID.Valid {
			if j, ok := idx[p.ParentID.Int64]; ok && j != i {
				kids[j] = append(kids[j], i)
				continue
			}
		}
		roots = append(roots, i)
	}

	// placed는 두 가지 일을 한다: 아래에서 빠진 글을 찾는 것과, parent_id가 도는
	// 데이터에서 무한히 내려가지 않게 막는 것이다. 이미 그린 글은 자식으로 다시
	// 그리지 않는다.
	placed := make([]bool, len(flat))
	var build func(i int) PostSummary
	build = func(i int) PostSummary {
		placed[i] = true
		n := flat[i]
		n.Children = nil
		for _, k := range kids[i] {
			if placed[k] {
				continue
			}
			n.Children = append(n.Children, build(k))
		}
		return n
	}

	out := make([]PostSummary, 0, len(roots))
	for _, i := range roots {
		out = append(out, build(i))
	}

	// parent_id가 서로를 가리키며 도는 글은 어느 뿌리에서도 닿지 않는다. 그런 게
	// 생기더라도 목록에서 조용히 사라지면 안 되므로 남은 것을 뿌리로 붙인다.
	for i := range flat {
		if !placed[i] {
			out = append(out, build(i))
		}
	}
	return out
}

// ChildPosts는 글의 직속 하위 글을 sort_order 순으로 돌려준다.
// 카테고리가 달라도 포함한다. 계층은 카테고리와 별개다.
func (s *store) ChildPosts(postID int64) ([]PostSummary, error) {
	rows, err := s.db.Query(`
		SELECT `+postColumns+`
		FROM posts p
		LEFT JOIN categories c ON c.id = p.category_id
		WHERE p.parent_id = ? AND `+s.notHidden("p")+`
		ORDER BY p.sort_order, p.title`, postID)
	if err != nil {
		return nil, fmt.Errorf("하위 글 조회: %w", err)
	}
	out, err := scanPostSummaries(rows)
	if err != nil {
		return nil, err
	}
	sortPosts(out)
	return out, nil
}

// PostAncestors는 위에서부터 이 글의 부모까지를 돌려준다. 자기 자신은 뺀다.
//
// depth에 상한을 둔다. posts.parent_id에는 categories와 달리 깊이를 지키는
// 트리거가 없어서, 사슬이 도는 데이터가 들어오면 재귀 CTE가 끝나지 않는다.
func (s *store) PostAncestors(postID int64) ([]PostSummary, error) {
	rows, err := s.db.Query(`
		WITH RECURSIVE up(id, depth) AS (
			SELECT parent_id, 0 FROM posts WHERE id = ? AND parent_id IS NOT NULL
			UNION ALL
			SELECT p.parent_id, up.depth + 1
			FROM posts p JOIN up ON p.id = up.id
			WHERE p.parent_id IS NOT NULL AND up.depth < 32
		)
		SELECT `+postColumns+`, up.depth
		FROM up JOIN posts p ON p.id = up.id
		LEFT JOIN categories c ON c.id = p.category_id
		WHERE `+s.notHidden("p")+`
		ORDER BY up.depth DESC`, postID)
	if err != nil {
		return nil, fmt.Errorf("상위 글 조회: %w", err)
	}
	defer rows.Close()

	var out []PostSummary
	for rows.Next() {
		var p PostSummary
		var depth int
		if err := rows.Scan(&p.ID, &p.ParentID, &p.Slug, &p.Title, &p.Status,
			&p.SortOrder, &p.CreatedAt, &depth); err != nil {
			return nil, fmt.Errorf("상위 글 스캔: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PostBySlug는 글 하나를 가져온다. 없으면 nil을 돌려준다.
func (s *store) PostBySlug(slug string) (*Post, error) {
	var p Post
	var categoryID sql.NullInt64
	err := s.db.QueryRow(`
		SELECT id, parent_id, slug, title, body, status, original_path, original_created_at, category_id
		FROM posts p WHERE p.slug = ? AND `+s.notHidden("p")+``, slug).
		Scan(&p.ID, &p.ParentID, &p.Slug, &p.Title, &p.Body, &p.Status,
			&p.OriginalPath, &p.CreatedAt, &categoryID)
	switch {
	case err == sql.ErrNoRows:
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("글 조회(%s): %w", slug, err)
	}

	if categoryID.Valid {
		trail, err := s.CategoryTrail(categoryID.Int64)
		if err != nil {
			return nil, err
		}
		p.Trail = trail
	}

	if p.Ancestors, err = s.PostAncestors(p.ID); err != nil {
		return nil, err
	}
	if p.Children, err = s.ChildPosts(p.ID); err != nil {
		return nil, err
	}
	return &p, nil
}

// CategoryTrail은 최상위부터 주어진 카테고리까지의 경로를 돌려준다.
func (s *store) CategoryTrail(categoryID int64) ([]Category, error) {
	rows, err := s.db.Query(`
		WITH RECURSIVE up(id, name, slug, parent_id, depth) AS (
			SELECT id, name, slug, parent_id, 0 FROM categories WHERE id = ?
			UNION ALL
			SELECT c.id, c.name, c.slug, c.parent_id, up.depth + 1
			FROM categories c JOIN up ON c.id = up.parent_id
		)
		SELECT id, name, slug, parent_id FROM up ORDER BY depth DESC`, categoryID)
	if err != nil {
		return nil, fmt.Errorf("카테고리 경로 조회: %w", err)
	}
	defer rows.Close()

	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.ParentID); err != nil {
			return nil, fmt.Errorf("카테고리 경로 스캔: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ImageBySHA256은 이미지 바이트와 MIME 타입을 가져온다. 없으면 nil을 돌려준다.
func (s *store) ImageBySHA256(sha string) (*Image, error) {
	var img Image
	err := s.db.QueryRow(`SELECT data, mime FROM images WHERE sha256 = ?`, sha).
		Scan(&img.Data, &img.MIME)
	switch {
	case err == sql.ErrNoRows:
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("이미지 조회(%s): %w", sha, err)
	}
	return &img, nil
}

// PostSummariesBySlug는 주어진 slug들의 목록 한 줄을 돌려준다. posts에 없는
// slug는 빠진다. 그래서 결과에 없다는 것이 곧 "가리킬 글이 없는 죽은 링크"라는
// 뜻이다.
//
// **제목만으로는 모자란다.** 자리표시자를 진짜 제목으로 바꾸는 데는 제목이면
// 되지만, 낱개 링크를 묶어 만드는 상자(groupLinkRuns)는 인라인 데이터베이스를
// 펼친 상자와 같은 모양이어야 한다 — 거기에는 작성일과 draft 뱃지가 찍힌다.
// 본문에서 뽑아낸 링크는 slug와 글자밖에 없으므로 나머지를 여기서 가져온다.
//
// **여기만은 숨긴 글도 돌려준다.** 대신 Hidden 표시를 단다. 그냥 빼버리면
// resolveBody가 "posts에 없는 slug = 노션 데이터베이스"로 오해해서 **엉뚱한
// 목록을 펼친다.** 세 상태를 구별해야 한다: 보인다 / 숨겼다 / 아예 없다.
func (s *store) PostSummariesBySlug(slugs []string) (map[string]PostSummary, error) {
	if len(slugs) == 0 {
		return map[string]PostSummary{}, nil
	}
	args := make([]any, len(slugs))
	for i, sl := range slugs {
		args[i] = sl
	}
	q := `SELECT ` + postColumns + `
		FROM posts p
		WHERE p.slug IN (?` + strings.Repeat(",?", len(slugs)-1) + `)`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("글 요약 조회: %w", err)
	}
	list, err := scanPostSummaries(rows)
	if err != nil {
		return nil, fmt.Errorf("글 요약 스캔: %w", err)
	}
	out := make(map[string]PostSummary, len(list))
	for _, p := range list {
		p.Hidden = !s.showDrafts && p.Status == "draft"
		out[p.Slug] = p
	}
	return out, nil
}

// InlineDBGroups는 ownerPath 밑으로 "한 층 더" 들어가 있는 글들을 그 중간 이름별로
// 묶어 돌려준다. 인라인 데이터베이스의 행이 여기 해당한다.
//
// 노션 데이터베이스는 posts에 행이 없어서 id로는 그 자식을 찾을 길이 없다.
// 남아 있는 유일한 끈이 original_path다. 데이터베이스로 한 층 들어간 글의 경로는
// `{주인 글 경로} > {데이터베이스 이름} > {글 제목}` 꼴이라, 가운데 칸이 곧
// 데이터베이스 이름이다.
//
// **바로 아래 한 칸인 이름은 뺀다.** 그건 실제로 존재하는 글(child_page)이지
// 데이터베이스가 아니다. 이걸 안 빼면 손자 글이 엉뚱한 링크 밑으로 딸려간다.
func (s *store) InlineDBGroups(ownerPath string) (map[string][]PostSummary, error) {
	prefix := ownerPath + " > "
	// SQLite의 substr/length는 글자 단위다. 경로에 한글이 많아 바이트 길이를
	// 주면 어긋난다. LIKE를 안 쓰는 이유는 경로에 %와 _가 든 글이 120건이라서다.
	rows, err := s.db.Query(`
		SELECT `+postColumns+`, p.original_path
		FROM posts p
		LEFT JOIN categories c ON c.id = p.category_id
		WHERE p.original_path IS NOT NULL AND substr(p.original_path, 1, ?) = ?
		ORDER BY p.sort_order, p.title`,
		utf8.RuneCountInString(prefix), prefix)
	if err != nil {
		return nil, fmt.Errorf("인라인 데이터베이스 조회: %w", err)
	}
	defer rows.Close()

	direct := map[string]bool{}
	grouped := map[string][]PostSummary{}
	for rows.Next() {
		var p PostSummary
		var path string
		if err := rows.Scan(&p.ID, &p.ParentID, &p.Slug, &p.Title, &p.Status,
			&p.SortOrder, &p.CreatedAt, &path); err != nil {
			return nil, fmt.Errorf("인라인 데이터베이스 스캔: %w", err)
		}
		rest := strings.Split(strings.TrimPrefix(path, prefix), " > ")
		switch len(rest) {
		case 1:
			direct[rest[0]] = true
		case 2:
			grouped[rest[0]] = append(grouped[rest[0]], p)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for name := range direct {
		delete(grouped, name)
	}
	for name, list := range grouped {
		// **숨긴 행을 빼되 이름은 남긴다.** 행이 하나도 안 남은 목록과 애초에
		// 없는 목록은 다르게 다뤄야 한다 — 앞의 것은 링크를 풀어 글자로 남기고,
		// 뒤의 것은 손대지 않는다(짝을 못 찾은 죽은 링크다). 값이 빈 슬라이스면
		// 조회하는 쪽에서 ok는 참이고 len이 0이라 그 둘이 구별된다.
		visible := make([]PostSummary, 0, len(list))
		for _, p := range list {
			if !s.showDrafts && p.Status == "draft" {
				continue
			}
			visible = append(visible, p)
		}
		sortPosts(visible)
		grouped[name] = nestPosts(visible)
		if grouped[name] == nil {
			grouped[name] = []PostSummary{}
		}
	}
	return grouped, nil
}

// CategorySubtreePostSlugs는 카테고리와 그 아래 전체에 붙은 글의 slug를 돌려준다.
//
// 카테고리 페이지에서 "하위 분류" 목록이 표지 글 본문과 겹치는지 볼 때 쓴다.
// 이름이 아니라 실제 글로 따진다 — 같은 이름이라도 분류 쪽에 더 많은 글이
// 달려 있으면 그건 겹친 게 아니다.
func (s *store) CategorySubtreePostSlugs(categoryID int64) (map[string]bool, error) {
	rows, err := s.db.Query(`
		WITH RECURSIVE sub(id) AS (
			SELECT id FROM categories WHERE id = ?
			UNION ALL
			SELECT k.id FROM categories k JOIN sub ON k.parent_id = sub.id
		)
		SELECT p.slug FROM posts p
		 WHERE `+s.notHidden("p")+` AND p.category_id IN (SELECT id FROM sub)`, categoryID)
	if err != nil {
		return nil, fmt.Errorf("분류 하위 글 조회: %w", err)
	}
	defer rows.Close()

	out := map[string]bool{}
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, fmt.Errorf("분류 하위 글 스캔: %w", err)
		}
		out[slug] = true
	}
	return out, rows.Err()
}

// NavCategory는 사이드바 트리의 한 마디다.
type NavCategory struct {
	ID        int64
	Name      string
	Slug      string
	PostCount int
	// Path는 최상위부터 이어 붙인 주소다. 카테고리 라우트가 3단계 경로를
	// 받으므로 조상 slug를 모두 알아야 링크를 만들 수 있다.
	Path     string
	Children []NavCategory
	// Open은 이 마디의 자식을 펼쳐둘지다. Active는 지금 보고 있는 곳인지다.
	// 둘 다 markNav가 채운다 — nav.go 참고.
	Open   bool
	Active bool
}

// NavTree는 카테고리 전체를 사이드바용 트리로 돌려준다.
//
// 사이드바는 **모든 페이지**에 나오므로 한 번의 조회로 끝내야 한다. 93개뿐이라
// 전부 읽어 Go에서 묶는다 — 페이지마다 단계별로 세 번 조회하는 것보다 낫다.
//
// 순서는 목록 페이지와 같은 규칙(sort_order, name)이다. 두 곳이 다르면 같은
// 분류가 화면마다 다른 자리에 있게 된다.
func (s *store) NavTree() ([]NavCategory, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.name, c.slug, c.parent_id, ` + s.subtreePostCount() + `
		FROM categories c
		WHERE ` + s.visibleCategory() + `
		ORDER BY c.sort_order, c.name`)
	if err != nil {
		return nil, fmt.Errorf("사이드바 카테고리 조회: %w", err)
	}
	defer rows.Close()

	type row struct {
		cat    NavCategory
		parent sql.NullInt64
	}
	var all []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.cat.ID, &r.cat.Name, &r.cat.Slug, &r.parent, &r.cat.PostCount); err != nil {
			return nil, fmt.Errorf("사이드바 카테고리 스캔: %w", err)
		}
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 부모 → 자식 목록. 조회 순서를 그대로 두므로 형제 순서가 유지된다.
	kids := map[int64][]int{}
	var roots []int
	for i, r := range all {
		if r.parent.Valid {
			kids[r.parent.Int64] = append(kids[r.parent.Int64], i)
		} else {
			roots = append(roots, i)
		}
	}

	// categories에는 깊이를 지키는 트리거가 있지만(3단계), 여기서도 방어한다.
	// 사이드바는 모든 페이지에 그려지므로 여기서 도는 것이 곧 전체 장애다.
	seen := make([]bool, len(all))
	var build func(i int, prefix string, depth int) NavCategory
	build = func(i int, prefix string, depth int) NavCategory {
		seen[i] = true
		c := all[i].cat
		c.Path = prefix + "/" + url.PathEscape(c.Slug)
		if depth < 8 {
			for _, k := range kids[c.ID] {
				if seen[k] {
					continue
				}
				c.Children = append(c.Children, build(k, c.Path, depth+1))
			}
		}
		return c
	}

	out := make([]NavCategory, 0, len(roots))
	for _, i := range roots {
		out = append(out, build(i, "", 0))
	}
	return out, nil
}

// CoverPostSlugOf는 카테고리 slug로 그 분류의 표지 글 slug를 찾는다.
// 최상위 카테고리만 본다. 없으면 빈 문자열이다.
func (s *store) CoverPostSlugOf(catSlug string) (string, error) {
	var slug sql.NullString
	err := s.db.QueryRow(`
		SELECT `+s.coverSlug()+`
		FROM categories c WHERE c.slug = ? AND c.parent_id IS NULL`, catSlug).Scan(&slug)
	switch {
	case err == sql.ErrNoRows:
		return "", nil
	case err != nil:
		return "", fmt.Errorf("표지 글 조회(%s): %w", catSlug, err)
	}
	return slug.String, nil
}
