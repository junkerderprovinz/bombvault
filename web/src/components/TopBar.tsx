import { getTheme, toggleTheme } from "../lib/theme";
import { useState } from "react";
import type { useT } from "../lib/i18n";

type T = ReturnType<typeof useT>["t"];
type LangCode = "en" | "de";

interface TopBarProps {
  t: T;
  lang: LangCode;
  setLanguage: (code: LangCode) => void;
  supportedLangs: LangCode[];
  langNames: Record<LangCode, string>;
}

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

export function TopBar({
  t,
  lang,
  setLanguage,
  supportedLangs,
  langNames,
}: TopBarProps) {
  const [theme, setThemeState] = useState(getTheme);

  function handleToggleTheme() {
    const next = toggleTheme();
    setThemeState(next);
  }

  return (
    <header className="flex items-center justify-end gap-2 px-4 py-2.5 bg-carbon-surface border-b border-carbon-border shrink-0">
      {/* Language switcher */}
      <div className="flex items-center gap-1">
        <span className="text-xs text-carbon-textMuted mr-1">{t("language.label")}:</span>
        <div className="flex rounded-lg overflow-hidden border border-carbon-border">
          {supportedLangs.map((code) => (
            <button
              key={code}
              onClick={() => setLanguage(code)}
              className={`px-2 py-1 text-xs font-medium transition-colors ${
                lang === code
                  ? "bg-carbon-surface3 text-carbon-text"
                  : "bg-carbon-surface text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
              }`}
              title={langNames[code]}
            >
              {langNames[code]}
            </button>
          ))}
        </div>
      </div>

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
