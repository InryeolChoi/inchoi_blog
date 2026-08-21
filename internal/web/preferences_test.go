package web

import (
	"net/http"
	"strings"
	"testing"
)

func TestPreferencesControlsAreOnEveryPage(t *testing.T) {
	body := get(t, testServer(t), "/dev/language").Body.String()
	for _, want := range []string{
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

func TestPreferencesOpenStateMorphsAtBottomRight(t *testing.T) {
	body := get(t, testServer(t), "/dev/language").Body.String()
	for _, want := range []string{
		`right: max(1rem, env(safe-area-inset-right))`,
		`.prefs-toggle[aria-expanded="true"] {`,
		`width: 4.4rem; border-radius: 1rem`,
		`.prefs-toggle[aria-expanded="true"] .prefs-glyph span:nth-child(1)`,
		`transform-origin: right bottom`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("설정 버튼 위치/모핑 CSS에 %q가 없다", want)
		}
	}
}
