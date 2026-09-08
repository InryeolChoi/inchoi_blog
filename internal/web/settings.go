package web

import (
	"database/sql"
	"fmt"
	"html/template"
	"strconv"
)

// 홈 표제지의 문구다. 예전에는 `templates/home.html`에 박혀 있었다.
//
// **DB가 정본이라는 전제를 홈에도 적용한 것이다.** 문장 하나를 고치려고 코드를
// 고쳐 배포하는 것은 "글은 웹 UI에서 직접 쓰고 고친다"와 어긋난다 — 홈에 서는
// 문장도 결국 글이다. 표는 `settings`(migrations/008)이고 admin의 환경설정
// 화면에서 고친다.

// HomeText는 홈 표제지의 네 조각이다.
//
// 제목을 두 줄로 나눠 든다. 아랫줄에만 **형광 노랑 블록**이 깔리는데
// (`.home-hero h1 span`), 한 칸에 담아 `<br>`을 글자로 받으면 그 태그가
// 화면에 그대로 나오거나(이스케이프) 사람이 HTML을 쓰게 된다. 둘 다 나쁘다 —
// 본문 렌더러가 `html.WithUnsafe()`라 임의 태그를 받는 자리를 늘리지 않는다.
type HomeText struct {
	// Kicker는 제목 위의 눈썹줄이다.
	Kicker string
	// TitleTop은 제목 첫 줄, TitleMark는 노랑 블록이 깔리는 둘째 줄이다.
	// TitleMark가 비면 제목은 한 줄이다.
	TitleTop  string
	TitleMark string
	Lead      string
	// Body는 표제지 아래에 자유롭게 쓰는 본문이다. **마크다운이고 글과 같은
	// 렌더러로 그린다** — 수식도 코드도 목록도 글에서 되는 것은 여기서도 된다.
	//
	// 홈에 무엇을 둘지는 사람이 정할 일이지 코드가 정할 일이 아니다. 예전에는
	// 문구 네 줄만 고칠 수 있어서, 그 밖의 것을 넣으려면 템플릿을 고쳐
	// 배포해야 했다.
	Body template.HTML
	// RecentLimit은 홈에 세울 최근 글 수다. 0이면 그 절을 아예 안 그린다.
	RecentLimit int
	// Custom은 **사람이 하나라도 고쳤는지**다. 참이면 눈썹줄에 `data-i18n`을
	// 안 붙인다 — 그 속성이 있으면 preferences.js의 고정 사전이 언어를 바꿀 때
	// textContent를 통째로 갈아치워서, 방금 적은 문장이 조용히 사라진다.
	Custom bool
}

// 기본값은 코드에 둔다. **DB에 미리 넣지 않는다** — 행이 없다는 것이 곧
// "아무도 안 고쳤다"라서, 그동안은 코드에서 문장을 다듬을 수 있다.
// 미리 INSERT해 두면 아무도 고친 적 없는 문장이 그 순간 DB에 굳는다.
var defaultHomeText = HomeText{
	Kicker:    "기술 블로그 겸 학습 아카이브",
	TitleTop:  "배운 것을 연결하고,",
	TitleMark: "만든 것으로 남깁니다.",
	Lead:      "개발, 컴퓨터과학, 데이터와 수학을 오가며 쌓은 기록입니다. 정답보다 이해의 과정과, 다시 찾아갈 길을 남깁니다.",
}

// HomeKeys는 홈 문구의 설정 키다. admin이 무엇을 고칠 수 있는지 알아야 해서
// 내보낸다 — **두 곳에 이름을 적으면 언젠가 갈라진다.**
var HomeKeys = []string{
	"home.kicker", "home.title_top", "home.title_mark", "home.lead",
	// 자유 본문과 최근 글 수. 위 넷과 달리 **기본값이 빈 값**이다 — 홈에
	// 무엇을 더 둘지는 아무도 안 정한 상태가 정상이라, 그때는 표제지만 선다.
	"home.body", "home.recent",
}

// recentDefault는 최근 글 설정이 없을 때 세울 개수다. 표제지가 주인공이라
// 목록이 길어지면 그 자리를 뺏는다.
const recentDefault = 6

// DefaultHomeText는 admin이 폼의 자리표시자로 쓸 기본 문구다.
func DefaultHomeText() map[string]string {
	return map[string]string{
		"home.kicker":     defaultHomeText.Kicker,
		"home.title_top":  defaultHomeText.TitleTop,
		"home.title_mark": defaultHomeText.TitleMark,
		"home.lead":       defaultHomeText.Lead,
		"home.body":       "",
		"home.recent":     strconv.Itoa(recentDefault),
	}
}

// Settings는 표에 저장된 값을 전부 돌려준다. 없는 키는 빠진다.
func Settings(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, fmt.Errorf("설정 조회: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("설정 스캔: %w", err)
		}
		out[k] = v
	}
	return out, rows.Err()
}

// homeText는 홈 표제지에 그릴 문구다. 저장된 값이 없으면 기본값이다.
//
// **빈 문자열은 "안 정했다"로 읽는다.** 표에서 지우는 것과 빈 값을 저장하는
// 것을 사람이 구별해 쓰기 어렵고, 홈에 빈 제목이 서는 쪽이 훨씬 나쁘다.
// 다만 `TitleMark`만은 예외다 — 거기를 비우는 것은 "제목을 한 줄로"라는
// 뜻이라 실제로 쓸 일이 있다.
//
// **자유 본문은 원문 그대로 함께 돌려준다.** 그리는 것은 핸들러의 일이다 —
// store는 마크다운 렌더러를 들고 있지 않고, 들게 하면 읽기 전용 조회 자리에
// 렌더링이 섞인다.
func (s *store) homeText() (HomeText, string, error) {
	saved, err := Settings(s.db)
	if err != nil {
		return defaultHomeText, "", err
	}
	out := defaultHomeText
	if v, ok := saved["home.kicker"]; ok && v != "" {
		out.Kicker, out.Custom = v, true
	}
	if v, ok := saved["home.title_top"]; ok && v != "" {
		out.TitleTop, out.Custom = v, true
	}
	if v, ok := saved["home.title_mark"]; ok {
		out.TitleMark, out.Custom = v, true
	}
	if v, ok := saved["home.lead"]; ok && v != "" {
		out.Lead, out.Custom = v, true
	}
	// **최근 글 수는 0도 뜻이 있다.** "그 절을 안 그린다"라서, 빈 값(안 정함)과
	// 구별해야 한다 — 위의 문구들과 반대다.
	out.RecentLimit = recentDefault
	if v, ok := saved["home.recent"]; ok {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			out.RecentLimit = n
		}
	}
	return out, saved["home.body"], nil
}
