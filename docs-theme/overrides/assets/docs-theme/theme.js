// Wires the rail controls of partials/dt-sidebar-controls.html: the language
// menu and the dark/light row, which drives Material's own palette inputs so
// the choice is stored the way Material stores it.
(function () {
  "use strict";

  var WORDS = {
    en: ["Dark", "Light"], de: ["Dunkel", "Hell"], ar: ["داكن", "فاتح"],
    cs: ["Tmavý", "Světlý"], da: ["Mørkt", "Lyst"], el: ["Σκοτεινό", "Φωτεινό"],
    es: ["Oscuro", "Claro"], fi: ["Tumma", "Vaalea"], fr: ["Sombre", "Clair"],
    he: ["כהה", "בהיר"], hu: ["Sötét", "Világos"], it: ["Scuro", "Chiaro"],
    ja: ["ダーク", "ライト"], ko: ["어둡게", "밝게"], nl: ["Donker", "Licht"],
    no: ["Mørkt", "Lyst"], pl: ["Ciemny", "Jasny"], pt: ["Escuro", "Claro"],
    ro: ["Întunecat", "Luminos"], ru: ["Тёмная", "Светлая"], sv: ["Mörkt", "Ljust"],
    th: ["มืด", "สว่าง"], tr: ["Koyu", "Açık"], uk: ["Темна", "Світла"],
    vi: ["Tối", "Sáng"], zh: ["深色", "浅色"]
  };

  // A locale is a language, a flag is a country; these are the ones where the
  // two codes differ.
  var FLAG = {
    en: "gb", ar: "sa", cs: "cz", da: "dk", el: "gr", he: "il", ja: "jp",
    ko: "kr", sv: "se", uk: "ua", vi: "vn", zh: "cn"
  };

  var MOON = '<svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true"><path d="M17.5 12.5A7.5 7.5 0 017.5 2.5a7.5 7.5 0 100 15 7.5 7.5 0 0010-5z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/></svg>';
  var SUN = '<svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true"><circle cx="10" cy="10" r="4" stroke="currentColor" stroke-width="1.5"/><path d="M10 2v2M10 16v2M2 10h2M16 10h2M4.93 4.93l1.41 1.41M13.66 13.66l1.41 1.41M4.93 15.07l1.41-1.41M13.66 6.34l1.41-1.41" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>';

  function locale() {
    return (document.documentElement.lang || "en").toLowerCase().split("-")[0];
  }

  function isDark() {
    return document.body.getAttribute("data-md-color-scheme") === "slate";
  }

  function paintMark(el, lang, flags) {
    if (!el) return;
    if (flags) {
      el.className = "dt-ctl-mark fi fi-" + (FLAG[lang] || lang);
      el.textContent = "";
    } else {
      el.className = "dt-ctl-mark dt-ctl-code";
      el.textContent = lang.toUpperCase();
    }
  }

  function syncTheme() {
    var words = WORDS[locale()] || WORDS.en;
    document.querySelectorAll("[data-dt-theme-toggle]").forEach(function (btn) {
      btn.querySelector(".dt-theme-icon").innerHTML = isDark() ? MOON : SUN;
      btn.querySelector("[data-dt-theme-label]").textContent = isDark() ? words[0] : words[1];
    });
  }

  function toggleTheme() {
    var want = isDark() ? "default" : "slate";
    var input = document.querySelector('input[name="__palette"][data-md-color-scheme="' + want + '"]');
    if (input) input.click();
    window.setTimeout(syncTheme, 0);
  }

  function init(root) {
    if (root.dataset.dtReady) return;
    root.dataset.dtReady = "1";
    var flags = root.getAttribute("data-dt-flags") === "on";
    var here = locale();

    var toggle = root.querySelector("[data-dt-lang-toggle]");
    var menu = root.querySelector(".dt-lang-menu");
    if (toggle && menu) {
      root.querySelectorAll(".dt-lang-opt").forEach(function (opt) {
        var lang = opt.getAttribute("data-dt-lang");
        paintMark(opt.querySelector(".dt-ctl-mark"), lang, flags);
        if (lang === here) {
          opt.setAttribute("aria-selected", "true");
          root.querySelector("[data-dt-current-label]").textContent =
            opt.querySelector(".dt-ctl-label").textContent;
        }
      });
      paintMark(root.querySelector("[data-dt-current-mark]"), here, flags);

      var setOpen = function (open) {
        menu.hidden = !open;
        toggle.setAttribute("aria-expanded", open ? "true" : "false");
        root.classList.toggle("dt-open", open);
      };
      toggle.addEventListener("click", function (e) {
        e.stopPropagation();
        setOpen(menu.hidden);
      });
      document.addEventListener("click", function (e) {
        if (!root.contains(e.target)) setOpen(false);
      });
      document.addEventListener("keydown", function (e) {
        if (e.key === "Escape") setOpen(false);
      });
    }

    root.querySelector("[data-dt-theme-toggle]").addEventListener("click", toggleTheme);
  }

  function initAll() {
    document.querySelectorAll(".dt-rail-controls").forEach(init);
    syncTheme();
  }

  // navigation.instant swaps the page without a load event, and document$
  // fires after each swap.
  if (window.document$ && window.document$.subscribe) {
    window.document$.subscribe(initAll);
  } else if (document.readyState !== "loading") {
    initAll();
  } else {
    document.addEventListener("DOMContentLoaded", initAll);
  }
})();
