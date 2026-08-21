// 화면 언어와 테마를 기기 안에 저장한다. 공통 UI는 고정 사전으로 바꾸고,
// 본문과 제목은 브라우저의 온디바이스 Translator API로 번역한다. 글을 외부
// 서버로 보내지 않으며, 코드와 수식은 번역 대상에서 뺀다.
(function () {
  "use strict";

  var root = document.documentElement;
  var toggle = document.getElementById("prefs-toggle");
  var panel = document.getElementById("prefs-panel");
  var translationStatus = document.getElementById("translation-status");
  if (!toggle || !panel) return;

  var messages = {
    ko: {
      skip: "본문으로 건너뛰기", openNav: "분류 열기", notes: "노트 {count}편",
      categories: "분류", home: "홈", location: "현재 위치",
      footer: "읽기 전용 미리보기 · status 구분 없이 전부 노출 (뱃지는 draft만)",
      settings: "화면 설정", language: "언어", theme: "테마",
      system: "시스템", light: "화이트", dark: "다크", openSettings: "화면 설정 열기",
      closeSettings: "화면 설정 닫기", subcategories: "하위 분류", posts: "글",
      childPosts: "하위 글",
      toc: "목차", noteMeta: "노트 {count}편 · 분류 {categories}개",
      postCount: "글 {count}건", noCategories: "카테고리가 없다.",
      translating: "본문을 기기에서 번역하는 중…",
      translationUnavailable: "이 브라우저는 기기 내 본문 번역을 지원하지 않습니다."
    },
    en: {
      skip: "Skip to content", openNav: "Open categories", notes: "{count} notes",
      categories: "Categories", home: "Home", location: "Current location",
      footer: "Read-only preview · all statuses shown (drafts are labeled)",
      settings: "Display settings", language: "Language", theme: "Theme",
      system: "System", light: "Light", dark: "Dark", openSettings: "Open display settings",
      closeSettings: "Close display settings", subcategories: "Subcategories", posts: "Posts",
      childPosts: "Child posts",
      toc: "Contents", noteMeta: "{count} notes · {categories} categories",
      postCount: "{count} posts", noCategories: "No categories.",
      translating: "Translating the post on this device…",
      translationUnavailable: "This browser does not support on-device post translation."
    },
    es: {
      skip: "Ir al contenido", openNav: "Abrir categorías", notes: "{count} notas",
      categories: "Categorías", home: "Inicio", location: "Ubicación actual",
      footer: "Vista de solo lectura · se muestran todos los estados (los borradores llevan etiqueta)",
      settings: "Ajustes de pantalla", language: "Idioma", theme: "Tema",
      system: "Sistema", light: "Claro", dark: "Oscuro", openSettings: "Abrir ajustes de pantalla",
      closeSettings: "Cerrar ajustes de pantalla", subcategories: "Subcategorías", posts: "Artículos",
      childPosts: "Artículos relacionados",
      toc: "Contenido", noteMeta: "{count} notas · {categories} categorías",
      postCount: "{count} artículos", noCategories: "No hay categorías.",
      translating: "Traduciendo el artículo en este dispositivo…",
      translationUnavailable: "Este navegador no admite la traducción local del artículo."
    }
  };

  // 이 제목들은 기존 서버 테스트가 정확한 HTML을 확인하므로 템플릿 마크업은
  // 그대로 두고, 브라우저에서 번역 키만 붙인다.
  var sectionKeys = { "하위 분류": "subcategories", "글": "posts", "하위 글": "childPosts" };
  document.querySelectorAll(".section-title").forEach(function (heading) {
    var key = sectionKeys[heading.textContent.trim()];
    if (key) heading.dataset.i18n = key;
  });

  var originalText = [];
  var translationRun = 0;

  // 번역된 UI 문구, 코드, 수식, SVG와 설정창은 원문 번역 대상이 아니다.
  // 나머지 텍스트 노드는 처음 표시된 한국어를 보존해 언어를 몇 번 바꿔도
  // 번역문을 다시 번역하지 않게 한다.
  document.querySelectorAll(".sidebar, .main").forEach(function (area) {
    var walker = document.createTreeWalker(area, NodeFilter.SHOW_TEXT);
    var node;
    while ((node = walker.nextNode())) {
      var parent = node.parentElement;
      if (!parent || !/[가-힣]/.test(node.nodeValue || "")) continue;
      if (parent.closest("script, style, pre, code, kbd, samp, math, svg, [data-i18n], [data-i18n-aria-label], .prefs-panel")) continue;
      originalText.push({ node: node, text: node.nodeValue });
    }
  });

  function restoreOriginalText() {
    originalText.forEach(function (item) { item.node.nodeValue = item.text; });
  }

  async function makeTranslator(target) {
    var api = window.Translator;
    if (api && typeof api.create === "function") {
      if (typeof api.availability === "function") {
        var state = await api.availability({ sourceLanguage: "ko", targetLanguage: target });
        if (state === "unavailable") return null;
      }
      return api.create({ sourceLanguage: "ko", targetLanguage: target });
    }
    // 초기 Chrome 구현도 함께 받는다. 둘 다 기기 안에서 모델을 실행한다.
    if (window.ai && window.ai.translator && typeof window.ai.translator.create === "function") {
      return window.ai.translator.create({ sourceLanguage: "ko", targetLanguage: target });
    }
    return null;
  }

  async function translateOriginalText(lang) {
    var run = ++translationRun;
    restoreOriginalText();
    if (translationStatus) translationStatus.textContent = "";
    if (lang === "ko" || originalText.length === 0) return;

    if (translationStatus) translationStatus.textContent = messages[lang].translating;
    var translator;
    try { translator = await makeTranslator(lang); } catch (_) { translator = null; }
    if (run !== translationRun) return;
    if (!translator) {
      if (translationStatus) translationStatus.textContent = messages[lang].translationUnavailable;
      return;
    }

    for (var i = 0; i < originalText.length; i++) {
      if (run !== translationRun) return;
      var item = originalText[i];
      var match = item.text.match(/^(\s*)([\s\S]*?)(\s*)$/);
      if (!match || !match[2]) continue;
      try {
        var translated = await translator.translate(match[2]);
        if (run === translationRun) item.node.nodeValue = match[1] + translated + match[3];
      } catch (_) {
        if (translationStatus) translationStatus.textContent = messages[lang].translationUnavailable;
        return;
      }
    }
    if (translationStatus && run === translationRun) translationStatus.textContent = "";
    if (typeof translator.destroy === "function") translator.destroy();
  }

  function saved(key) {
    try { return localStorage.getItem(key); } catch (_) { return null; }
  }

  function save(key, value) {
    try { localStorage.setItem(key, value); } catch (_) {}
  }

  function format(text, el) {
    return text.replace(/\{(\w+)\}/g, function (_, key) {
      return el.dataset[key] || "";
    });
  }

  function applyLanguage(lang) {
    if (!messages[lang]) lang = "ko";
    root.lang = lang;
    document.querySelectorAll("[data-i18n]").forEach(function (el) {
      var text = messages[lang][el.dataset.i18n];
      if (text) el.textContent = format(text, el);
    });
    document.querySelectorAll("[data-i18n-aria-label]").forEach(function (el) {
      var text = messages[lang][el.dataset.i18nAriaLabel];
      if (text) el.setAttribute("aria-label", text);
    });
    document.querySelectorAll("[data-language]").forEach(function (button) {
      button.setAttribute("aria-pressed", button.dataset.language === lang ? "true" : "false");
    });
    save("blog-language", lang);
    syncToggleLabel();
    translateOriginalText(lang);
  }

  function applyTheme(theme) {
    if (theme !== "light" && theme !== "dark" && theme !== "system") theme = "system";
    var resolved = theme === "system"
      ? (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light")
      : theme;
    root.dataset.theme = resolved;
    root.dataset.themeChoice = theme;
    document.querySelectorAll("[data-theme-choice]").forEach(function (button) {
      button.setAttribute("aria-pressed", button.dataset.themeChoice === theme ? "true" : "false");
    });
    save("blog-theme", theme);
  }

  function isOpen() { return panel.dataset.open === "true"; }

  function syncToggleLabel() {
    var lang = messages[root.lang] ? root.lang : "ko";
    var label = toggle.querySelector("[data-i18n='openSettings']");
    if (label) label.textContent = messages[lang][isOpen() ? "closeSettings" : "openSettings"];
  }

  function setOpen(open, restoreFocus) {
    panel.dataset.open = open ? "true" : "false";
    panel.setAttribute("aria-hidden", open ? "false" : "true");
    toggle.setAttribute("aria-expanded", open ? "true" : "false");
    syncToggleLabel();
    if (open) {
      var selected = panel.querySelector("[aria-pressed='true']");
      if (selected) selected.focus();
    } else if (restoreFocus) {
      toggle.focus();
    }
  }

  toggle.addEventListener("click", function () { setOpen(!isOpen(), false); });
  panel.addEventListener("click", function (event) {
    var language = event.target.closest("[data-language]");
    var theme = event.target.closest("[data-theme-choice]");
    if (language) applyLanguage(language.dataset.language);
    if (theme) applyTheme(theme.dataset.themeChoice);
  });
  document.addEventListener("pointerdown", function (event) {
    if (isOpen() && !panel.contains(event.target) && !toggle.contains(event.target)) setOpen(false, false);
  });
  document.addEventListener("keydown", function (event) {
    if (event.key === "Escape" && isOpen()) setOpen(false, true);
  });

  var systemTheme = window.matchMedia("(prefers-color-scheme: dark)");
  systemTheme.addEventListener("change", function () {
    if ((saved("blog-theme") || "system") === "system") applyTheme("system");
  });

  var initialTheme = saved("blog-theme") || "system";
  var initialLanguage = saved("blog-language") || root.lang;
  applyTheme(initialTheme);
  applyLanguage(initialLanguage);
})();
