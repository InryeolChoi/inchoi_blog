package web

import (
	"database/sql"
	"net/http"
	"strings"
	"testing"
	"time"
)

// 복사 버튼은 **서버가 찍지 않는다.** 클립보드는 스크립트 없이 쓸 수 없어서,
// 미리 찍어두면 스크립트가 꺼진 브라우저에 눌러도 아무 일이 없는 죽은 버튼이
// 남는다. 그래서 여기서 확인하는 것은 "버튼이 있는가"가 아니라
// **"버튼을 만들 스크립트를 필요한 페이지에만 실어 보내는가"**다.

func copyTestDB(t *testing.T) *sql.DB {
	t.Helper()

	sqlDB := testDB(t)
	exec := execer(t, sqlDB)
	exec(`INSERT INTO categories (id, name, slug, sort_order) VALUES (1, '개발', 'dev', 0)`)

	now := time.Now().UTC()
	post := func(id int, slug, title, body string) {
		exec(`INSERT INTO posts (id, slug, title, body, status, source, category_id, sort_order, created_at, updated_at)
		      VALUES (?, ?, ?, ?, 'unlisted', 'notion', 1, 0, ?, ?)`,
			id, slug, title, body, now, now)
	}
	post(1, "with-code", "코드 있는 글", "앞말\n\n```python\nx = 1\n```\n")
	// `text`는 "색칠하지 말라"는 뜻이라 highlight.js를 안 받는다. 그래도 복사할
	// 코드는 그대로 있으므로 copy.js는 실려야 한다.
	post(2, "text-code", "text 코드 글", "앞말\n\n```text\n그냥 글\n```\n")
	post(3, "no-code", "코드 없는 글", "코드가 없는 본문이다.")

	return sqlDB
}

func TestCopyScriptOnlyOnPagesWithCodeBlocks(t *testing.T) {
	h := handlerFor(t, copyTestDB(t))

	for _, tc := range []struct {
		path string
		want bool
	}{
		{"/p/with-code", true},
		{"/p/text-code", true},
		{"/p/no-code", false},
		{"/dev", false},
		{"/", false},
	} {
		body := get(t, h, tc.path).Body.String()
		if got := strings.Contains(body, `src="/static/copy.js"`); got != tc.want {
			t.Errorf("%s: copy.js 실림=%v, want %v", tc.path, got, tc.want)
		}
	}

	// 두 판정이 갈리는 자리를 화면에서도 확인한다. text만 있는 글은
	// highlight.js를 안 받지만 copy.js는 받는다.
	body := get(t, h, "/p/text-code").Body.String()
	if strings.Contains(body, "highlight-init.js") {
		t.Error("text만 있는 글에 highlight.js가 실렸다")
	}
	if !strings.Contains(body, `src="/static/copy.js"`) {
		t.Error("text만 있는 글에 copy.js가 안 실렸다")
	}
}

func TestCopyScriptRunsBeforePreferences(t *testing.T) {
	// defer 스크립트끼리는 순서가 지켜진다. copy.js가 먼저 돌아야 버튼이
	// 이미 있을 때 preferences.js의 사전이 aria-label을 세 언어로 바꾼다.
	body := get(t, handlerFor(t, copyTestDB(t)), "/p/with-code").Body.String()
	copyAt := strings.Index(body, `src="/static/copy.js"`)
	prefsAt := strings.Index(body, `src="/static/preferences.js"`)
	if copyAt < 0 || prefsAt < 0 {
		t.Fatal("두 스크립트 중 하나가 없다")
	}
	if copyAt > prefsAt {
		t.Error("copy.js가 preferences.js 뒤에 있다. 버튼 라벨이 한국어로 남는다")
	}
}

func TestCopyScriptIsServed(t *testing.T) {
	rec := get(t, testServer(t), "/static/copy.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("copy.js 상태 코드 %d", rec.Code)
	}
	src := rec.Body.String()

	// 보안 컨텍스트가 아니면 버튼을 아예 만들지 않는다.
	if !strings.Contains(src, "navigator.clipboard") {
		t.Error("클립보드 확인이 없다")
	}
	// 껍데기가 없는 <pre>도 감싸고 나서 단다. pre 안에 두면 가로 스크롤에
	// 버튼이 딸려 흘러간다.
	if !strings.Contains(src, `shell.className = "codeblock"`) {
		t.Error("껍데기 없는 pre를 감싸는 코드가 없다")
	}
	// admin 미리보기가 다시 부를 수 있어야 한다.
	if !strings.Contains(src, "window.blogCopyButtons") {
		t.Error("admin이 다시 부를 함수가 열려 있지 않다")
	}
	if !strings.Contains(src, `btn.type = "button"`) {
		t.Error("type=button이 아니다. 폼 안에 들어가면 제출 버튼이 된다")
	}
}

// TestCopyButtonLabelsAreInEveryDictionary는 버튼 글자가 세 언어에 다 있는지
// 본다. 키가 코드에만 있고 화면에는 속성값으로 나가서, 빠뜨려도 조용히
// 한국어로 남는다 — 오류 화면 글자를 같은 이유로 지키고 있다.
func TestCopyButtonLabelsAreInEveryDictionary(t *testing.T) {
	dict, err := staticFS.ReadFile("static/preferences.js")
	if err != nil {
		t.Fatalf("preferences.js 읽기: %v", err)
	}
	src, err := staticFS.ReadFile("static/copy.js")
	if err != nil {
		t.Fatalf("copy.js 읽기: %v", err)
	}

	for _, key := range []string{"copyCode", "copyCodeDone", "copyCodeFailed"} {
		if got := strings.Count(string(dict), key+":"); got != 3 {
			t.Errorf("사전의 %q가 %d곳이다. 세 언어에 다 있어야 한다", key, got)
		}
		if !strings.Contains(string(src), `"`+key+`"`) {
			t.Errorf("copy.js가 %q 키를 쓰지 않는다", key)
		}
	}
	// aria-label은 textContent가 아니라 속성이다. 사전이 그 길로 바꿔야 한다.
	if !strings.Contains(string(src), "i18nAriaLabel") {
		t.Error("aria-label을 사전이 바꿀 수 있게 달지 않았다")
	}
	if !strings.Contains(string(dict), "window.blogLabel") {
		t.Error("나중에 생긴 요소에 사전을 입히는 길이 없다")
	}
}

// TestCopyButtonCSSAnchorsToShellNotPre는 375px에서 버튼이 코드와 함께
// 흘러가지 않는 근거를 지킨다.
//
// `article pre`에는 overflow-x: auto가 걸려 있다. 버튼을 pre 안에 absolute로
// 두면 스크롤 컨테이너 안이라 긴 줄을 밀 때 같이 밀린다. 기준은 스크롤하지
// 않는 껍데기(.codeblock)여야 한다.
func TestCopyButtonCSSAnchorsToShellNotPre(t *testing.T) {
	css, err := SiteCSS()
	if err != nil {
		t.Fatalf("SiteCSS: %v", err)
	}
	for _, want := range []string{
		".codeblock { position: relative;",
		".copy {",
		"position: absolute; top: 0; right: 0;",
		`.copy[data-state="done"] .copy-ico-done`,
		`.codeblock[data-copy="on"] .lang`,
	} {
		if !strings.Contains(string(css), want) {
			t.Errorf("복사 버튼 CSS에 %q가 없다", want)
		}
	}
	if strings.Contains(string(css), ".codeblock pre .copy") ||
		strings.Contains(string(css), "article pre .copy") {
		t.Error("버튼을 pre 안에 두는 규칙이 있다")
	}
}
