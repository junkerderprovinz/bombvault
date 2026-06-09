import { getTheme, toggleTheme } from "../lib/theme";
import { useState, useRef, useEffect } from "react";
import { useT } from "../lib/i18n";

// ---------------------------------------------------------------------------
// Icons
// ---------------------------------------------------------------------------

function IconSun() {
  return (
    <svg width="18" height="18" viewBox="0 0 20 20" fill="none">
      <circle cx="10" cy="10" r="4" stroke="currentColor" strokeWidth="1.5" />
      <path
        d="M10 2v2M10 16v2M2 10h2M16 10h2M4.93 4.93l1.41 1.41M13.66 13.66l1.41 1.41M4.93 15.07l1.41-1.41M13.66 6.34l1.41-1.41"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
      />
    </svg>
  );
}

function IconMoon() {
  return (
    <svg width="18" height="18" viewBox="0 0 20 20" fill="none">
      <path
        d="M17.5 12.5A7.5 7.5 0 017.5 2.5a7.5 7.5 0 100 15 7.5 7.5 0 0010-5z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
    </svg>
  );
}

// ---------------------------------------------------------------------------
// Flag helper — renders a flag-icons span for the given ISO 3166-1 alpha-2 code
// ---------------------------------------------------------------------------

function Flag({ code }: { code: string }) {
  return (
    <span
      className={`fi fi-${code}`}
      style={{ width: "1.25em", height: "1em", display: "inline-block", flexShrink: 0 }}
    />
  );
}

// ---------------------------------------------------------------------------
// LanguageSwitcher
// ---------------------------------------------------------------------------

function LanguageSwitcher() {
  const { lang, setLanguage, languages, t } = useT();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const current = languages.find((l) => l.code === lang) ?? languages[0];

  // Close on outside click
  useEffect(() => {
    if (!open) return;
    function handler(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open]);

  // Close on Escape
  useEffect(() => {
    if (!open) return;
    function handler(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [open]);

  return (
    <div className="relative" ref={ref}>
      {/* Trigger button — shows only the current flag; name is in aria-label/title */}
      <button
        aria-label={`${t("language.label")}: ${current.label}`}
        title={`${t("language.label")}: ${current.label}`}
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
        className="flex items-center px-2 py-1.5 rounded-lg text-xs font-medium text-carbon-textSub border border-carbon-border bg-carbon-surface hover:bg-carbon-hover hover:text-carbon-text transition-colors"
      >
        <Flag code={current.flag} />
      </button>

      {/* Dropdown */}
      {open && (
        <div
          role="listbox"
          aria-label={t("language.label")}
          className="absolute right-0 top-full mt-1 z-50 w-48 max-h-72 overflow-y-auto rounded-xl border border-carbon-border bg-carbon-surface shadow-lg"
          style={{ scrollbarColor: "var(--carbon-border) transparent" }}
        >
          {languages.map((l) => (
            <button
              key={l.code}
              role="option"
              aria-selected={l.code === lang}
              onClick={() => {
                setLanguage(l.code);
                setOpen(false);
              }}
              className={`flex items-center gap-2.5 w-full px-3 py-2 text-sm text-left transition-colors ${
                l.code === lang
                  ? "bg-carbon-surface3 text-carbon-text"
                  : "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
              }`}
            >
              <Flag code={l.flag} />
              <span>{l.label}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// TopBar
// ---------------------------------------------------------------------------

export function TopBar() {
  const { t } = useT();
  const [theme, setThemeState] = useState(getTheme);

  function handleToggleTheme() {
    const next = toggleTheme();
    setThemeState(next);
  }

  return (
    <header className="flex items-center justify-end gap-2 px-4 py-2.5 bg-carbon-surface border-b border-carbon-border shrink-0">
      {/* Flag-based language switcher */}
      <LanguageSwitcher />

      {/* Theme toggle */}
      <button
        onClick={handleToggleTheme}
        title={t("theme.toggle")}
        className="p-1.5 rounded-lg text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors"
      >
        {theme === "dark" ? <IconSun /> : <IconMoon />}
      </button>
    </header>
  );
}
