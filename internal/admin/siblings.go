package admin

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/inryeol/blog/internal/web"
)

// 편집기의 `형제 순서` 패널.
//
// **목록을 admin이 제 눈으로 세지 않는다.** 화면에 서는 순서를 아는 것은 공개
// 쪽이고(web.SiblingOrder), 여기서 다시 세면 두 답이 언젠가 갈라진다 —
// 미리보기가 렌더러·CSS·CDN 태그를 공개 쪽에서 그대로 가져다 쓰는 것과 같은
// 이유다. 여기가 하는 일은 그 답을 JSON으로 넘기고, 사람이 옮긴 결과를 쓰는
// 것뿐이다.

// handleSiblings는 이 글이 화면에서 서 있는 목록을 화면 순서대로 준다.
func (s *Server) handleSiblings(w http.ResponseWriter, r *http.Request) {
	list, err := web.SiblingOrder(s.store.db, r.PathValue("slug"))
	if err != nil {
		log.Printf("admin 형제 목록 조회 실패: %v", err)
		writeErr(w, http.StatusInternalServerError, "이 글이 어느 목록에 서는지 알아내지 못했다")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// applyOrder는 사람이 옮긴 순서를 그대로 쓴다. 목록의 i번째 글에 sort_order = i,
// sort_order_manual = 1을 적는다.
//
// # 왜 목록 전원에게 쓰나
//
// **한 글만 표시해서는 옮긴 대로 안 선다.** 화면은 사람이 정한 글을 맨 앞에
// 세우고 나머지는 제목으로 다시 세우므로(web.sortPosts), 한 편만 켜면 그
// 한 편이 맨 위로 갈 뿐 나머지 순서는 그대로다. 목록이 통째로 사람 것이어야
// 적어둔 차례가 곧 화면의 차례가 된다.
//
// # 왜 slug로 받나
//
// 사람이 화면에서 알아볼 수 있는 값이어야 잘못 보냈을 때 눈에 띈다 —
// 부모 글을 id가 아니라 slug로 주고받는 것과 같은 이유다.
//
// **저장과 같은 트랜잭션에서 돈다.** 글은 저장됐는데 순서만 안 바뀌는 절반의
// 결과를 남기지 않는다. 그리고 **글 저장보다 뒤에 돈다** — 패널을 썼으면
// 거기서 정한 순서가 숫자 칸을 이긴다.
func applyOrder(tx *sql.Tx, oldSlug, newSlug string, slugs []string, now time.Time) error {
	if len(slugs) == 0 {
		return nil
	}

	// slug를 바꾸면서 순서도 옮겼으면, 패널이 들고 있던 이름은 **옛 slug**다.
	// 여기서 새 이름으로 갈아끼운다 — 안 그러면 방금 이름을 바꿨다는 이유로
	// 저장 전체가 "그런 글이 없다"로 실패한다.
	list := make([]string, len(slugs))
	copy(list, slugs)
	if oldSlug != newSlug {
		for i, slug := range list {
			if slug == oldSlug {
				list[i] = newSlug
			}
		}
	}

	seen := make(map[string]bool, len(list))
	for _, slug := range list {
		if seen[slug] {
			return bad("순서 목록에 %q가 두 번 있다", slug)
		}
		seen[slug] = true
	}
	// **지금 고치는 글이 그 목록에 있어야 한다.** 없으면 이 요청은 남의 목록을
	// 재배열하는 것이고, 그건 이 패널이 하는 일이 아니다.
	if !seen[newSlug] {
		return bad("순서 목록에 지금 고치는 글(%s)이 없다", newSlug)
	}

	up, err := tx.Prepare(`
		UPDATE posts SET sort_order = ?, sort_order_manual = 1, updated_at = ?
		WHERE slug = ?`)
	if err != nil {
		return err
	}
	defer up.Close()

	for i, slug := range list {
		res, err := up.Exec(i, now, slug)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		// **못 찾으면 멈춘다.** 조용히 넘어가면 화면에서 옮긴 것과 DB에 남은
		// 것이 달라지고, 그 차이는 다음에 목록을 열어야 보인다.
		if n == 0 {
			return bad("순서를 정할 글 %q를 찾지 못했다", slug)
		}
	}
	return nil
}
