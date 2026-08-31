package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/inryeol/blog/internal/importer"
)

// saveMaxBytes는 저장으로 받을 요청의 최대 크기다. 지금 가장 긴 글이 약 40KB다.
const saveMaxBytes = 1 << 20 // 1MB

// slugMaxLen은 slug의 최대 길이다. 주소에 그대로 나가는 값이라 좁혀둔다.
const slugMaxLen = 120

// titleMaxLen은 제목의 최대 길이다(룬 기준).
const titleMaxLen = 300

// saveReq는 편집 폼이 보내는 것이다.
//
// **PUT은 부분 갱신이 아니라 통째로 바꾸기다.** 안 보낸 칸은 "그대로 두라"가
// 아니라 "비우라"로 읽는다. 부분 갱신으로 두면 "이 칸을 지우고 싶다"와
// "이 칸은 안 건드린다"를 JSON으로 구별할 수 없고, 그 모호함은 결국
// 사람이 지운 줄 안 값이 남는 쪽으로 끝난다.
//
// **published_at은 여기 없다.** 그건 사람이 적는 값이 아니라 status가 처음
// published가 되는 순간 서버가 찍는 값이다. 받아주면 두 곳에서 정해진다.
type saveReq struct {
	Slug   string `json:"slug"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Status string `json:"status"`

	// Rev는 불러올 때 받은 updated_at 글자다. 고칠 때만 쓴다.
	// **비어 있으면 거절한다** — 검사를 건너뛰는 쪽으로 틀리지 않는다.
	Rev string `json:"rev"`

	CategoryID *int64 `json:"categoryId"`
	// ParentSlug는 부모 글이다. id가 아니라 slug로 받는다 — 사람이 화면에서
	// 알아볼 수 있는 값이어야 잘못 붙였을 때 눈에 띈다.
	ParentSlug string `json:"parentSlug"`
	SortOrder  int    `json:"sortOrder"`

	// OriginalCreatedAt은 목록에 찍히는 날짜다. ""면 비운다.
	// "2026-08-31" 또는 RFC3339를 받는다.
	OriginalCreatedAt string `json:"originalCreatedAt"`
}

// badInput은 사람이 고칠 수 있는 잘못이다. 그대로 화면에 보여준다.
type badInput string

func (e badInput) Error() string { return string(e) }

func bad(format string, a ...any) error { return badInput(fmt.Sprintf(format, a...)) }

var (
	// errSlugTaken은 그 slug를 다른 글이 이미 쓰고 있다는 뜻이다.
	errSlugTaken = errors.New("그 slug를 쓰는 글이 이미 있다")
	// errRevMismatch는 내가 불러온 뒤에 누군가(다른 탭이) 이 글을 고쳤다는 뜻이다.
	errRevMismatch = errors.New("내가 연 뒤에 이 글이 바뀌었다. 새로 고쳐 다시 열어라")
	// errNoSuchPost는 고치려는 글이 없다는 뜻이다.
	errNoSuchPost = errors.New("그런 글이 없다")
)

// validSlug는 slug가 쓸 수 있는 형태인지 본다.
//
// **판정을 importer.Slugify에 맡긴다.** 규칙을 여기 다시 적으면 카테고리
// slug와 글 slug가 언젠가 갈라진다. 한 번 돌린 결과가 자기 자신과 같으면
// 그 규칙이 만들 수 있는 값이라는 뜻이다.
//
// 그래서 **한글이 그대로 남는다.** 스키마 주석은 "영문 소문자 + 하이픈"이라고
// 적혀 있지만, 카테고리 slug는 이미 한글을 쓰고 그 주소가 배포되어 있다
// (`/data-math/수리-통계-이론/선형대수`). 글만 로마자로 강제하면 같은 사이트
// 안에서 주소 규칙이 두 벌이 되고, 한국어 제목에서 slug를 만들 길이 없어진다.
func validSlug(s string) bool {
	return s != "" && len([]rune(s)) <= slugMaxLen && importer.Slugify(s) == s
}

// normalizeSave는 요청을 다듬고 사람이 고칠 수 있는 잘못을 잡는다.
func normalizeSave(req *saveReq) error {
	req.Slug = strings.TrimSpace(req.Slug)
	req.Title = strings.TrimSpace(req.Title)
	req.ParentSlug = strings.TrimSpace(req.ParentSlug)
	req.OriginalCreatedAt = strings.TrimSpace(req.OriginalCreatedAt)

	if req.Title == "" {
		return bad("제목이 비었다")
	}
	if len([]rune(req.Title)) > titleMaxLen {
		return bad("제목이 너무 길다 (%d자까지)", titleMaxLen)
	}
	if len(req.Body) > saveMaxBytes {
		return bad("본문이 너무 크다")
	}

	ok := false
	for _, s := range Statuses {
		if s == req.Status {
			ok = true
			break
		}
	}
	if !ok {
		return bad("status가 %q다. %s 중 하나여야 한다", req.Status, strings.Join(Statuses, ", "))
	}

	// slug를 안 적었으면 제목에서 만든다. 카테고리와 같은 규칙이라 한글이 남는다.
	if req.Slug == "" {
		req.Slug = importer.Slugify(req.Title)
	}
	if !validSlug(req.Slug) {
		return bad("slug %q를 쓸 수 없다. 소문자·숫자·한글과 하이픈만 되고 %d자까지다",
			req.Slug, slugMaxLen)
	}
	if req.SortOrder < 0 {
		return bad("sort_order는 0 이상이어야 한다")
	}
	return nil
}

// parseDate는 메타 패널이 보낸 날짜를 읽는다. ""면 NULL이다.
func parseDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			u := t.UTC()
			return &u, nil
		}
	}
	return nil, bad("날짜 %q를 읽지 못했다. 2026-08-31 꼴로 적는다", s)
}

// savePost는 글 하나를 만들거나 고친다. **전부 한 트랜잭션이다.**
//
// 중간에 실패하면 아무것도 안 들어간다. 반쯤 저장된 글을 손으로 정리하는 것보다
// 통째로 다시 하는 게 낫다 — cmd/import가 1,356편을 한 트랜잭션에 넣는 것과
// 같은 이유다.
// curSlug는 고칠 글을 찾는 지금의 slug다(만들 때는 안 쓴다). req.Slug는
// **바꾸고 싶은 slug**라 둘이 다를 수 있다 — 그 경우가 곧 slug 바꾸기다.
func (s *store) savePost(curSlug string, req saveReq, create bool, now time.Time) (out *PostDetail, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	origCreated, err := parseDate(req.OriginalCreatedAt)
	if err != nil {
		return nil, err
	}

	// ── 분류가 실제로 있는지
	if req.CategoryID != nil {
		var n int
		if err = tx.QueryRow(`SELECT count(*) FROM categories WHERE id = ?`, *req.CategoryID).Scan(&n); err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, bad("그런 분류가 없다 (id=%d)", *req.CategoryID)
		}
	}

	// ── 지금 상태를 집는다 (고칠 때만)
	var id int64
	var oldSlug, oldStatus, oldRev string
	var oldPublished sql.NullTime
	if !create {
		err = tx.QueryRow(`SELECT id, slug, status, cast(updated_at AS TEXT), published_at
		                   FROM posts WHERE slug = ?`, curSlug).
			Scan(&id, &oldSlug, &oldStatus, &oldRev, &oldPublished)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errNoSuchPost
		}
		if err != nil {
			return nil, err
		}
		// **rev가 비면 통과시키지 않는다.** 검사를 건너뛰는 쪽으로 틀리면
		// 그게 곧 조용한 덮어쓰기다.
		if req.Rev == "" || req.Rev != oldRev {
			return nil, errRevMismatch
		}
	}

	// ── 부모 글
	var parentID *int64
	if req.ParentSlug != "" {
		var pid int64
		err = tx.QueryRow(`SELECT id FROM posts WHERE slug = ?`, req.ParentSlug).Scan(&pid)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, bad("부모로 지정한 글이 없다: %q", req.ParentSlug)
		}
		if err != nil {
			return nil, err
		}
		if !create && pid == id {
			return nil, bad("자기 자신을 부모로 둘 수 없다")
		}
		if !create {
			if err = checkNoCycle(tx, id, pid); err != nil {
				return nil, err
			}
		}
		parentID = &pid
	}

	// ── published_at: status가 처음 published가 될 때만 찍는다
	var published *time.Time
	if oldPublished.Valid {
		t := oldPublished.Time
		published = &t
	}
	if req.Status == "published" && published == nil {
		t := now
		published = &t
	}

	if create {
		// original_created_at을 안 적었으면 지금으로 둔다. 비워두면 목록에
		// 날짜 없는 줄이 하나 생기는데, 새로 쓴 글에는 쓸 날짜가 실제로 있다.
		if origCreated == nil {
			t := now
			origCreated = &t
		}
		// **notion_page_id는 NULL이고 source는 native다.** deploy/upload-guard.sql이
		// 그 NULL로 "이관이 아니라 사람이 여기서 쓴 글"을 알아본다. 노션 id를
		// 채우면 그 글은 가드에 안 걸리고 다음 upload-db.sh에 조용히 사라진다.
		var res sql.Result
		res, err = tx.Exec(`
			INSERT INTO posts (slug, title, body, status, source, notion_page_id,
			                   category_id, parent_id, sort_order,
			                   original_created_at, published_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, 'native', NULL, ?, ?, ?, ?, ?, ?, ?)`,
			req.Slug, req.Title, req.Body, req.Status,
			req.CategoryID, parentID, req.SortOrder,
			origCreated, published, now, now)
		if err != nil {
			if isUniqueSlug(err) {
				return nil, errSlugTaken
			}
			return nil, err
		}
		if id, err = res.LastInsertId(); err != nil {
			return nil, err
		}
	} else {
		_, err = tx.Exec(`
			UPDATE posts SET slug = ?, title = ?, body = ?, status = ?,
			                 category_id = ?, parent_id = ?, sort_order = ?,
			                 original_created_at = ?, published_at = ?, updated_at = ?
			WHERE id = ?`,
			req.Slug, req.Title, req.Body, req.Status,
			req.CategoryID, parentID, req.SortOrder,
			origCreated, published, now, id)
		if err != nil {
			if isUniqueSlug(err) {
				return nil, errSlugTaken
			}
			return nil, err
		}
		// **slug를 바꾸면 그 글을 가리키는 본문 링크도 같이 바꾼다.**
		// 안 그러면 다른 글 몇 편이 조용히 404를 가리키게 된다. relink가
		// 모든 내부 링크를 `](/p/{slug})` 한 형태로 맞춰두었으므로 그것만 본다.
		if oldSlug != req.Slug {
			if err = rewriteLinks(tx, id, oldSlug, req.Slug, now); err != nil {
				return nil, err
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.postBySlug(req.Slug)
}

// checkNoCycle은 parent를 이 글의 부모로 두면 사슬이 도는지 본다.
//
// **posts.parent_id에는 categories와 달리 깊이·순환을 막는 트리거가 없다.**
// (마이그레이션 002는 categories만 지킨다.) 도는 사슬을 한 번 만들면
// 목록을 그리는 web의 nestPosts와 PostAncestors가 각자 방어해야 하고,
// cmd/postparent도 같은 검사를 자기 쪽에서 한다. 여기서 막는 게 가장 싸다.
func checkNoCycle(tx *sql.Tx, id, parent int64) error {
	seen := map[int64]bool{id: true}
	cur := parent
	for depth := 0; depth < 64; depth++ {
		if seen[cur] {
			return bad("부모로 두면 사슬이 돈다")
		}
		seen[cur] = true
		var next sql.NullInt64
		err := tx.QueryRow(`SELECT parent_id FROM posts WHERE id = ?`, cur).Scan(&next)
		if errors.Is(err, sql.ErrNoRows) || !next.Valid {
			return nil
		}
		if err != nil {
			return err
		}
		cur = next.Int64
	}
	return bad("부모 사슬이 너무 깊다")
}

// rewriteLinks는 옛 slug를 가리키던 본문 링크를 새 slug로 바꾼다.
//
// 형태가 `](/p/{slug})` 하나뿐이라 닫는 괄호까지 붙여 정확히 짝지을 수 있다.
// slug에는 `%`도 `_`도 들어갈 수 없으므로(Slugify가 걸러낸다) LIKE가 안전하다.
func rewriteLinks(tx *sql.Tx, id int64, oldSlug, newSlug string, now time.Time) error {
	from := "](/p/" + oldSlug + ")"
	to := "](/p/" + newSlug + ")"
	_, err := tx.Exec(`
		UPDATE posts SET body = replace(body, ?, ?), updated_at = ?
		WHERE id <> ? AND instr(body, ?) > 0`, from, to, now, id, from)
	return err
}

// isUniqueSlug는 UNIQUE 제약에 걸린 것인지 본다. 드라이버가 주는 글자를 보는
// 수밖에 없어서 넓게 잡되, posts.slug라는 것까지 확인한다.
func isUniqueSlug(err error) bool {
	s := err.Error()
	return strings.Contains(s, "UNIQUE") && strings.Contains(s, "posts.slug")
}

// handleSave는 글을 만들거나 고친다.
//
//	POST /api/admin/posts        새 글
//	PUT  /api/admin/posts/{slug} 고치기 — {slug}는 **지금의** slug다
//
// 성공하면 저장된 글을 그대로 돌려준다. 화면이 새 rev를 받아야 이어서 또
// 저장할 수 있다 — 안 주면 두 번째 저장이 늘 "그새 바뀌었다"로 거절당한다.
func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, saveMaxBytes))
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "본문이 너무 크다")
		return
	}
	var req saveReq
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "JSON을 읽지 못했다: "+err.Error())
		return
	}
	if err := normalizeSave(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	curSlug := r.PathValue("slug")
	create := curSlug == ""

	post, err := s.store.savePost(curSlug, req, create, time.Now().UTC())
	if err != nil {
		writeSaveErr(w, r, err)
		return
	}
	code := http.StatusOK
	if create {
		code = http.StatusCreated
		w.Header().Set("Location", "/admin/edit/"+post.Slug)
	}
	log.Printf("admin 저장: %s slug=%q status=%q 본문 %d바이트",
		map[bool]string{true: "새 글", false: "고침"}[create], post.Slug, post.Status, len(req.Body))
	writeJSON(w, code, post)
}

// writeSaveErr는 저장 실패를 알맞은 상태 코드로 바꾼다.
//
// **사람이 고칠 수 있는 잘못과 서버가 터진 것을 가른다.** 전부 500으로 주면
// 글 쓰는 사람은 자기가 뭘 잘못했는지 알 수 없고, 전부 400으로 주면 진짜
// 고장이 사용자 실수처럼 보인다.
func writeSaveErr(w http.ResponseWriter, r *http.Request, err error) {
	var bi badInput
	switch {
	case errors.As(err, &bi):
		writeErr(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, errNoSuchPost):
		writeErr(w, http.StatusNotFound, err.Error())
	case errors.Is(err, errSlugTaken), errors.Is(err, errRevMismatch):
		// 409다. 요청 자체는 멀쩡한데 지금 DB 상태와 부딪힌다.
		writeErr(w, http.StatusConflict, err.Error())
	default:
		log.Printf("admin 저장 실패: %s %s: %v", r.Method, r.URL.Path, err)
		writeErr(w, http.StatusInternalServerError, "저장하지 못했다")
	}
}

// handleCategories는 메타 패널의 분류 선택지를 준다.
func (s *Server) handleCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := s.store.categories()
	if err != nil {
		log.Printf("admin 분류 조회 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "분류를 가져오지 못했다")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": cats})
}
