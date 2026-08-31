package web

import (
	"net/http"
	"strings"
	"testing"
)

func TestPreferencesControlsAreOnEveryPage(t *testing.T) {
	body := get(t, testServer(t), "/dev/language").Body.String()
	for _, want := range []string{
		`id="prefs-shell"`,
		`id="prefs-toggle"`,
		`id="prefs-panel"`,
		`data-language="es"`,
		`data-theme-choice="system"`,
		`data-theme-choice="dark"`,
		`src="/static/preferences.js"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("화면 설정 UI에 %q가 없다", want)
		}
	}
}

func TestPreferencesScriptIsServed(t *testing.T) {
	rec := get(t, testServer(t), "/static/preferences.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("preferences.js 상태 코드 %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/javascript; charset=utf-8" {
		t.Errorf("Content-Type=%q", got)
	}
	if !strings.Contains(rec.Body.String(), `localStorage.setItem`) {
		t.Error("선택을 저장하는 코드가 없다")
	}
	if !strings.Contains(rec.Body.String(), `translator.translate`) {
		t.Error("렌더링된 본문을 번역하는 코드가 없다")
	}
	if !strings.Contains(rec.Body.String(), `prefers-color-scheme: dark`) ||
		!strings.Contains(rec.Body.String(), `addEventListener("change"`) {
		t.Error("시스템 테마 변경을 따라가는 코드가 없다")
	}
}

func TestPreferencesButtonMorphsIntoPanelAtBottomRight(t *testing.T) {
	body := get(t, testServer(t), "/dev/language").Body.String()
	for _, want := range []string{
		`right: max(1rem, env(safe-area-inset-right))`,
		`.prefs-shell[data-open="true"] {`,
		`width: min(19rem, calc(100vw - 2rem))`,
		`height: min(17.5rem, calc(100vh - 2rem))`,
		// **모서리는 각지다** (2026-09-01의 디자인 개편). 예전에는 원이
		// 둥근 사각형으로 자라는 모핑이었는데, 이 팔레트에는 둥근 모서리가
		// 없다 — 사각 버튼이 사각 판으로 자란다. 자라는 것 자체는 그대로다.
		`border: 2px solid var(--ink)`,
		`.prefs-toggle[aria-expanded="true"] {`,
		`.prefs-toggle[aria-expanded="true"] .prefs-glyph span:nth-child(1)`,
		`transform-origin: right bottom`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("설정 버튼 위치/모핑 CSS에 %q가 없다", want)
		}
	}
}

// TestBrandIsNamedByDictionaryNotMachineTranslated는 사이트 이름이 고정 사전으로만
// 바뀌는지 본다.
//
// **이름은 기계번역 대상이 아니다.** preferences.js는 `.sidebar`/`.main` 안의
// 한글 텍스트 노드를 온디바이스 Translator에 넘기는데, `[data-i18n]` 안은 뺀다.
// 브랜드에 키가 없으면 Chrome에서 이름이 통째로 번역돼 나간다 — 실제로 그랬다.
//
// 키를 `<a class="brand">`가 아니라 **양옆 글자에 따로** 다는 이유: 사전 적용이
// textContent를 통째로 갈아치우므로, 바깥에 달면 가운데 `<span class="dot">`이
// 사라진다.
func TestBrandIsNamedByDictionaryNotMachineTranslated(t *testing.T) {
	body := get(t, testServer(t), "/dev/language").Body.String()
	const want = `<span data-i18n="brandHead">열렬히</span>` +
		`<span class="dot">.</span>` +
		`<span data-i18n="brandTail">뛰기</span>`
	// 상단 헤더와 사이드바 두 곳에 같은 이름이 나온다.
	if got := strings.Count(body, want); got != 2 {
		t.Errorf("사전 키를 단 브랜드가 %d곳이다, 2곳이어야 한다", got)
	}

	dict, err := staticFS.ReadFile("static/preferences.js")
	if err != nil {
		t.Fatalf("preferences.js 읽기: %v", err)
	}
	for _, key := range []string{"brandHead", "brandTail"} {
		if strings.Count(string(dict), key+":") < 3 {
			t.Errorf("사전에 %q가 세 언어로 다 있지 않다", key)
		}
	}
}
