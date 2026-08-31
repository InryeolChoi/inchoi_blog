package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// # 왜 지우기가 있는가
//
// 이 프로젝트는 원래 **글을 지우지 않는다.** 공개 여부는 삭제가 아니라 status로
// 가리고(`draft`), 노션에서 온 글을 빼는 것은 `curation.DropPosts`에 적는 일이다.
// 그 원칙은 그대로다.
//
// 여기가 필요한 이유는 **웹에서 잘못 만든 글**이다. 저장이 붙은 뒤로 오타 하나로
// 생긴 빈 글이 남을 수 있는데, 그걸 치우는 길이 DB를 직접 여는 것뿐이면
// "DB를 손으로 고치지 마라"는 원칙과 정면으로 부딪힌다.
//
// # 그래서 지우는 것보다 **말해주는 것**이 이 파일의 일이다
//
//	자식이 있다        → 아예 막는다 (되돌릴 수 없이 사슬이 끊긴다)
//	분류의 표지다      → 알려주고, 확인하면 표지를 잃는다
//	노션에서 왔다      → 알려준다. 다음 재이관이 **되살린다**
//	가리키는 링크가 있다 → 같은 트랜잭션에서 글자로 푼다

// deleteReq는 지우기 요청이다.
type deleteReq struct {
	// Rev는 저장과 같은 표다. 내가 목록에서 본 그 글이 맞는지 확인한다.
	Rev string `json:"rev"`
	// Force는 "알려준 것을 읽었다"는 뜻이다. **자식이 있는 경우는 이걸로도
	// 못 지운다** — 그건 알려주고 말고의 문제가 아니다.
	Force bool `json:"force"`
}

// PostRefs는 이 글을 지우면 무슨 일이 생기는지다.
type PostRefs struct {
	Slug string `json:"slug"`
	// Children은 이 글을 부모로 삼는 글이다. 하나라도 있으면 못 지운다.
	Children []string `json:"children"`
	// CoverOf는 이 글을 표지로 쓰는 분류다. 지우면 그 분류가 표지를 잃는다.
	CoverOf []string `json:"coverOf"`
	// LinkedFrom은 본문에서 이 글을 가리키는 글이다. 지울 때 링크를 글자로 푼다.
	LinkedFrom []string `json:"linkedFrom"`
	// Notion이면 다음 `import -db`가 이 글을 **되살린다.**
	Notion bool `json:"notion"`
	// Images는 이 글의 본문만 쓰는 그림 수다. 지워도 BLOB은 남는다.
	OrphanImages int `json:"orphanImages"`
}

// Blocked는 자식이 있어 아예 못 지우는 경우다.
func (r PostRefs) Blocked() bool { return len(r.Children) > 0 }

// NeedsForce는 사람이 한 번 더 읽고 확인해야 하는 경우다.
func (r PostRefs) NeedsForce() bool {
	return len(r.CoverOf) > 0 || len(r.LinkedFrom) > 0 || r.Notion
}

// linkRe는 이 글을 가리키는 마크다운 링크다. 글자 부분은 대괄호가 겹치지 않는
// 것만 본다 — `[![](img)](/p/x)` 같은 중첩까지 다루려 들면 정규식으로는 못 한다.
func linkRe(slug string) *regexp.Regexp {
	return regexp.MustCompile(`\[([^\[\]]*)\]\(/p/` + regexp.QuoteMeta(slug) + `\)`)
}

// postRefs는 지우기 전에 무엇이 걸리는지 모은다.
func (s *store) postRefs(slug string) (*PostRefs, error) {
	var id int64
	var source string
	err := s.db.QueryRow(`SELECT id, source FROM posts WHERE slug = ?`, slug).Scan(&id, &source)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	refs := &PostRefs{
		Slug: slug, Notion: source == "notion",
		Children: []string{}, CoverOf: []string{}, LinkedFrom: []string{},
	}

	collect := func(q string, args ...any) ([]string, error) {
		rows, err := s.db.Query(q, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []string{}
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, rows.Err()
	}

	if refs.Children, err = collect(
		`SELECT title FROM posts WHERE parent_id = ? ORDER BY sort_order, id`, id); err != nil {
		return nil, err
	}
	if refs.CoverOf, err = collect(
		`SELECT name FROM categories WHERE cover_post_id = ?`, id); err != nil {
		return nil, err
	}
	// 링크는 `](/p/{slug})` 한 형태다. relink가 모든 내부 링크를 그렇게 맞춰뒀다.
	if refs.LinkedFrom, err = collect(
		`SELECT title FROM posts WHERE id <> ? AND instr(body, ?) > 0 ORDER BY id`,
		id, "](/p/"+slug+")"); err != nil {
		return nil, err
	}
	return refs, nil
}

// deletePost는 글 하나를 지운다. **전부 한 트랜잭션이다.**
func (s *store) deletePost(slug string, req deleteReq, now time.Time) (*PostRefs, error) {
	refs, err := s.postRefs(slug)
	if err != nil {
		return nil, err
	}
	if refs == nil {
		return nil, errNoSuchPost
	}
	if refs.Blocked() {
		return refs, bad("하위 글 %d편이 이 글에 매달려 있다. 그것들을 먼저 옮기거나 지워라",
			len(refs.Children))
	}
	if refs.NeedsForce() && !req.Force {
		return refs, errNeedsConfirm
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var id int64
	var rev string
	err = tx.QueryRow(`SELECT id, cast(updated_at AS TEXT) FROM posts WHERE slug = ?`, slug).
		Scan(&id, &rev)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNoSuchPost
	}
	if err != nil {
		return nil, err
	}
	if req.Rev == "" || req.Rev != rev {
		return refs, errRevMismatch
	}

	// **가리키던 링크를 글자로 푼다.**
	//
	// 안 풀면 그 링크의 대상이 "posts에 없는 slug"가 되는데, 렌더링 쪽
	// resolveBody는 그걸 **노션 인라인 데이터베이스로 판정한다**(CLAUDE.md의
	// "posts에 없는 slug = 노션 데이터베이스"). 즉 지운 글로 가는 죽은 링크가
	// 남는 데서 그치지 않고 **엉뚱한 목록이 펼쳐질 수 있다.**
	//
	// 링크를 통째로 지우지 않고 글자만 남기는 것은 공개 쪽 unlinkHidden과 같다 —
	// 줄째 지우면 문장이 끊긴다.
	if err := unlinkTo(tx, id, slug, now); err != nil {
		return nil, err
	}

	if _, err := tx.Exec(`DELETE FROM posts WHERE id = ?`, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return refs, nil
}

// unlinkTo는 이 글을 가리키는 링크를 글자로 푼다.
//
// SQLite에는 정규식이 없어서 Go에서 고친다. 한 글을 가리키는 링크는 많아야
// 몇 편이라 통째로 읽어도 된다.
func unlinkTo(tx *sql.Tx, id int64, slug string, now time.Time) error {
	rows, err := tx.Query(`SELECT id, body FROM posts WHERE id <> ? AND instr(body, ?) > 0`,
		id, "](/p/"+slug+")")
	if err != nil {
		return err
	}
	type edit struct {
		id   int64
		body string
	}
	var edits []edit
	re := linkRe(slug)
	for rows.Next() {
		var e edit
		if err := rows.Scan(&e.id, &e.body); err != nil {
			rows.Close()
			return err
		}
		// $1은 링크 글자다. 비어 있으면(`[](/p/x)`) 아무것도 안 남는데,
		// 그건 원래 보이는 글자가 없던 링크라 맞다.
		e.body = re.ReplaceAllString(e.body, "$1")
		edits = append(edits, e)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, e := range edits {
		if _, err := tx.Exec(`UPDATE posts SET body = ?, updated_at = ? WHERE id = ?`,
			e.body, now, e.id); err != nil {
			return err
		}
		// **정규식이 못 잡은 것이 남았으면 멈춘다.** 중첩 대괄호처럼 이 정규식이
		// 다루지 못하는 모양이 있으면, 조용히 넘기면 죽은 링크가 남는다.
		if strings.Contains(e.body, "](/p/"+slug+")") {
			return bad("글 %d의 링크를 다 풀지 못했다. 그 글을 먼저 손보고 다시 지워라", e.id)
		}
	}
	return nil
}

// errNeedsConfirm은 "알려줄 것이 있으니 읽고 다시 오라"는 뜻이다.
var errNeedsConfirm = errors.New("지우면 잃는 것이 있다. 확인하고 다시 보내라")

// handleRefs는 지우기 전에 무엇이 걸리는지 알려준다.
//
// **지우기 버튼을 누르기 전에 화면이 이걸 먼저 묻는다.** 무엇을 잃는지 모른 채
// 확인 창의 "예"를 누르게 하지 않는다.
func (s *Server) handleRefs(w http.ResponseWriter, r *http.Request) {
	refs, err := s.store.postRefs(r.PathValue("slug"))
	if err != nil {
		log.Printf("admin 참조 조회 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "무엇이 걸리는지 알아내지 못했다")
		return
	}
	if refs == nil {
		writeErr(w, http.StatusNotFound, "그런 글이 없다")
		return
	}
	writeJSON(w, http.StatusOK, refs)
}

// handleDelete는 글을 지운다.
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<16))
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "요청이 너무 크다")
		return
	}
	var req deleteReq
	// 본문이 비어 있으면 rev가 없는 것이고, 그러면 아래에서 거절된다.
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			writeErr(w, http.StatusBadRequest, "JSON을 읽지 못했다: "+err.Error())
			return
		}
	}

	refs, err := s.store.deletePost(slug, req, time.Now().UTC())
	if err != nil {
		if errors.Is(err, errNeedsConfirm) {
			// 409에 **무엇을 잃는지 같이 실어 보낸다.** 화면이 그걸 그대로
			// 보여주고 사람이 읽은 뒤에 force로 다시 온다.
			writeJSON(w, http.StatusConflict, map[string]any{
				"error": errNeedsConfirm.Error(), "refs": refs,
			})
			return
		}
		writeSaveErr(w, r, err)
		return
	}
	log.Printf("admin 삭제: slug=%q 링크 푼 글 %d편 표지였던 분류 %d개",
		slug, len(refs.LinkedFrom), len(refs.CoverOf))
	writeJSON(w, http.StatusOK, map[string]any{"deleted": slug, "refs": refs})
}
