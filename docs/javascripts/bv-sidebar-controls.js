// Wires the app-style sidebar-footer controls injected by
// overrides/partials/bv-sidebar-controls.html: a language picker (navigates to
// the same page in the chosen locale) and a dark/light toggle (drives Material's
// own palette so the choice persists). Faithful to web/src/components/Sidebar.tsx.
//
// navigation.instant is disabled (see mkdocs.yml), so every page is a fresh load
// and DOMContentLoaded init is enough; we still subscribe to document$ when it
// exists so this keeps working if instant nav is ever re-enabled.
(function () {
  "use strict";

  // Current-mode words per locale, copied verbatim from the app locales
  // (theme.dark / theme.light) so the docs read exactly like BombVault.
  var WORDS = {
    en: { d: "Dark", l: "Light" },
    de: { d: "Dunkel", l: "Hell" },
    ar: { d: "داكن", l: "فاتح" },
    cs: { d: "Tmavý", l: "Světlý" },
    da: { d: "Mørkt", l: "Lyst" },
    el: { d: "Σκοτεινό", l: "Φωτεινό" },
    es: { d: "Oscuro", l: "Claro" },
    fi: { d: "Tumma", l: "Vaalea" },
    fr: { d: "Sombre", l: "Clair" },
    he: { d: "כהה", l: "בהיר" },
    hu: { d: "Sötét", l: "Világos" },
    it: { d: "Scuro", l: "Chiaro" },
    ja: { d: "ダーク", l: "ライト" },
    ko: { d: "어둡게", l: "밝게" },
    nl: { d: "Donker", l: "Licht" },
    no: { d: "Mørkt", l: "Lyst" },
    pl: { d: "Ciemny", l: "Jasny" },
    pt: { d: "Escuro", l: "Claro" },
    ro: { d: "Întunecat", l: "Luminos" },
    ru: { d: "Тёмная", l: "Светлая" },
    sv: { d: "Mörkt", l: "Ljust" },
    th: { d: "มืด", l: "สว่าง" },
    tr: { d: "Koyu", l: "Açık" },
    uk: { d: "Темна", l: "Світла" },
    vi: { d: "Tối", l: "Sáng" },
    zh: { d: "深色", l: "浅色" },
  };

  // Paint a flag span with flag-icons (fi fi-XX), the same library the app uses,
  // so flags render as images on every platform (emoji flags do not on Windows).
  function setFlag(el, code) {
    if (el) el.className = "bv-ctl-flag fi fi-" + code;
  }

  function currentLocale() {
    var l = (document.documentElement.getAttribute("lang") || "en").toLowerCase();
    return l.split("-")[0];
  }

  function words(loc) {
    return WORDS[loc] || WORDS.en;
  }

  function schemeIsDark() {
    return document.body.getAttribute("data-md-color-scheme") === "slate";
  }

  // moon (shown in dark mode) / sun (shown in light mode), matching the app's
  // sidebar theme-row icons.
  function themeIcon(dark) {
    return dark
      ? '<svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true"><path d="M17.5 12.5A7.5 7.5 0 017.5 2.5a7.5 7.5 0 100 15 7.5 7.5 0 0010-5z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/></svg>'
      : '<svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true"><circle cx="10" cy="10" r="4" stroke="currentColor" stroke-width="1.5"/><path d="M10 2v2M10 16v2M2 10h2M16 10h2M4.93 4.93l1.41 1.41M13.66 13.66l1.41 1.41M4.93 15.07l1.41-1.41M13.66 6.34l1.41-1.41" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>';
  }

  function syncThemeRow(root) {
    var dark = schemeIsDark();
    var w = words(currentLocale());
    (root || document).querySelectorAll(".bv-theme").forEach(function (btn) {
      var icon = btn.querySelector(".bv-theme-icon");
      var label = btn.querySelector("[data-bv-theme-label]");
      if (icon) icon.innerHTML = themeIcon(dark);
      if (label) label.textContent = dark ? w.d : w.l;
    });
  }

  function toggleTheme() {
    // Click Material's palette radio for the OTHER scheme; Material applies it and
    // persists to localStorage. slate = dark, default = light.
    var want = schemeIsDark() ? "default" : "slate";
    var input = document.querySelector(
      'input[name="__palette"][data-md-color-scheme="' + want + '"]'
    );
    if (input) input.click();
    // Material flips body[data-md-color-scheme] on click; read it back next tick.
    window.setTimeout(function () {
      syncThemeRow();
    }, 0);
  }

  function switchLocale(target, controls) {
    var def = controls.getAttribute("data-bv-default") || "en";
    var base = controls.getAttribute("data-bv-base") || ".";
    var siteRoot = new URL(base.replace(/\/?$/, "/"), window.location.href);
    var rel = window.location.href.slice(siteRoot.href.length).replace(/[?#].*$/, "");
    var cur = currentLocale();
    if (cur !== def) {
      var pfx = cur + "/";
      if (rel.toLowerCase().indexOf(pfx) === 0) rel = rel.slice(pfx.length);
    }
    window.location.href = siteRoot.href + (target === def ? "" : target + "/") + rel;
  }

  function initControls(controls) {
    if (controls.__bvInit) return;
    controls.__bvInit = true;

    var loc = currentLocale();
    var def = controls.getAttribute("data-bv-default") || "en";

    // Current flag + label from the matching option (fall back to the default).
    var opt =
      controls.querySelector('.bv-lang-opt[data-bv-lang="' + loc + '"]') ||
      controls.querySelector('.bv-lang-opt[data-bv-lang="' + def + '"]');
    var curFlag = controls.querySelector("[data-bv-current-flag]");
    var curLabel = controls.querySelector("[data-bv-current-label]");
    if (opt) {
      if (curFlag) setFlag(curFlag, opt.getAttribute("data-bv-flag"));
      if (curLabel) {
        var lbl = opt.querySelector(".bv-ctl-label");
        if (lbl) curLabel.textContent = lbl.textContent;
      }
    }

    // Fill each option's flag, mark the current one, wire navigation.
    controls.querySelectorAll(".bv-lang-opt").forEach(function (o) {
      setFlag(o.querySelector(".bv-ctl-flag"), o.getAttribute("data-bv-flag"));
      o.setAttribute("aria-selected", o.getAttribute("data-bv-lang") === loc ? "true" : "false");
      o.addEventListener("click", function () {
        switchLocale(o.getAttribute("data-bv-lang"), controls);
      });
    });

    // Language menu open / close.
    var toggle = controls.querySelector("[data-bv-lang-toggle]");
    var menu = controls.querySelector(".bv-lang-menu");
    function setOpen(open) {
      if (!menu || !toggle) return;
      menu.hidden = !open;
      toggle.setAttribute("aria-expanded", open ? "true" : "false");
      controls.classList.toggle("bv-open", open);
    }
    if (toggle) {
      toggle.addEventListener("click", function (e) {
        e.stopPropagation();
        setOpen(!!menu.hidden);
      });
    }
    document.addEventListener("click", function (e) {
      if (!controls.contains(e.target)) setOpen(false);
    });
    document.addEventListener("keydown", function (e) {
      if (e.key === "Escape") setOpen(false);
    });

    // Dark / light row.
    var themeBtn = controls.querySelector("[data-bv-theme-toggle]");
    if (themeBtn) themeBtn.addEventListener("click", toggleTheme);
    syncThemeRow(controls);
  }

  function initAll() {
    document.querySelectorAll(".bv-sidebar-controls").forEach(initControls);
    syncThemeRow();
  }

  if (window.document$ && typeof window.document$.subscribe === "function") {
    window.document$.subscribe(initAll);
  } else if (document.readyState !== "loading") {
    initAll();
  } else {
    document.addEventListener("DOMContentLoaded", initAll);
  }
})();
