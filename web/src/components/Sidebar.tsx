import { NavLink } from "react-router-dom";
import { useState, useRef, useEffect } from "react";
import { type Settings } from "../lib/api";
import { useT } from "../lib/i18n";
import { getTheme, toggleTheme } from "../lib/theme";
import { useAdvanced } from "../lib/advanced";

interface SidebarProps {
  settings: Settings | null;
}

interface NavItem {
  to: string;
  label: string;
  icon: React.ReactNode;
}

// Simple inline SVG icons (monochrome, 20×20)
function IconDashboard() {
  return (
    <svg width="22" height="22" viewBox="0 0 20 20" fill="none" className="shrink-0">
      <rect x="2" y="2" width="7" height="7" rx="1.5" fill="currentColor" />
      <rect x="11" y="2" width="7" height="7" rx="1.5" fill="currentColor" opacity=".6" />
      <rect x="2" y="11" width="7" height="7" rx="1.5" fill="currentColor" opacity=".6" />
      <rect x="11" y="11" width="7" height="7" rx="1.5" fill="currentColor" opacity=".4" />
    </svg>
  );
}

// Docker whale mark — Simple Icons path, scaled to 20×20 viewport
function IconContainers() {
  return (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M13.983 11.078h2.119a.186.186 0 0 0 .186-.185V9.006a.186.186 0 0 0-.186-.186h-2.119a.185.185 0 0 0-.185.185v1.888c0 .102.083.185.185.185m-2.954-5.43h2.118a.186.186 0 0 0 .186-.186V3.574a.186.186 0 0 0-.186-.185h-2.118a.185.185 0 0 0-.185.185v1.888c0 .103.082.185.185.185m0 2.716h2.118a.187.187 0 0 0 .186-.186V6.29a.186.186 0 0 0-.186-.185h-2.118a.185.185 0 0 0-.185.185v1.887c0 .102.082.185.185.185m-2.93 0h2.12a.186.186 0 0 0 .184-.186V6.29a.185.185 0 0 0-.185-.185H8.1a.185.185 0 0 0-.185.185v1.887c0 .102.083.185.185.185m-2.964 0h2.119a.186.186 0 0 0 .185-.186V6.29a.185.185 0 0 0-.185-.185H5.136a.186.186 0 0 0-.186.185v1.887c0 .102.084.185.186.185m5.893 2.715h2.118a.186.186 0 0 0 .186-.185V9.006a.186.186 0 0 0-.186-.186h-2.118a.185.185 0 0 0-.185.185v1.888c0 .102.082.185.185.185m-2.93 0h2.12a.185.185 0 0 0 .184-.185V9.006a.185.185 0 0 0-.184-.186h-2.12a.185.185 0 0 0-.184.185v1.888c0 .102.083.185.185.185m-2.964 0h2.119a.185.185 0 0 0 .185-.185V9.006a.185.185 0 0 0-.184-.186h-2.12a.186.186 0 0 0-.186.185v1.888c0 .102.084.185.186.185m-2.92 0h2.12a.185.185 0 0 0 .184-.185V9.006a.185.185 0 0 0-.184-.186h-2.12a.185.185 0 0 0-.185.185v1.888c0 .102.083.185.185.185M23.763 9.89c-.065-.051-.672-.51-1.954-.51-.338.001-.676.03-1.01.087-.248-1.7-1.653-2.53-1.716-2.566l-.344-.199-.226.327c-.284.438-.49.922-.612 1.43-.23.97-.09 1.882.403 2.661-.595.332-1.55.413-1.744.42H.751a.751.751 0 0 0-.75.75c-.007 1.73.425 3.43 1.25 4.977.892 1.679 2.22 2.922 3.836 3.592 1.973.799 5.146.985 7.325.985 1.815.001 3.626-.19 5.392-.573 2.483-.556 4.649-1.932 6.2-3.967a15.024 15.024 0 0 0 2.203-5.09c.048-.165.087-.336.122-.512.054-.234.086-.473.095-.714a4.81 4.81 0 0 0-.66-2.352" />
    </svg>
  );
}

// Desktop/monitor icon — matches Unraid's "VMs" tab glyph (screen + stand + base)
function IconVM() {
  return (
    <svg width="22" height="22" viewBox="0 0 20 20" fill="none" className="shrink-0" aria-hidden="true">
      <rect x="2" y="3" width="16" height="10" rx="1.5" stroke="currentColor" strokeWidth="1.5" />
      <path d="M7 17h6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <path d="M10 13v4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

function IconFlash() {
  return (
    <svg width="22" height="22" viewBox="0 0 20 20" fill="none" className="shrink-0">
      <path d="M11 2L4 11h6l-1 7 7-9h-6l1-7z" fill="currentColor" />
    </svg>
  );
}

// Sliders/tuner glyph for the Config self-backup tab — settings-like, but
// deliberately distinct from the Settings cog below so the two never read alike.
function IconConfig() {
  return (
    <svg width="22" height="22" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" className="shrink-0" aria-hidden="true">
      <path d="M3 6h9M15 6h2M3 14h2M8 14h9" strokeLinecap="round" />
      <circle cx="13.5" cy="6" r="2" fill="var(--sidebar-surface, transparent)" />
      <circle cx="6.5" cy="14" r="2" fill="var(--sidebar-surface, transparent)" />
    </svg>
  );
}

function IconSettings() {
  // Standard 8-tooth cog/gear — conventional settings symbol
  return (
    <svg width="22" height="22" viewBox="0 0 20 20" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 0 1-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 0 1 .947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 0 1 2.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 0 1 2.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 0 1 .947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 0 1-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 0 1-2.287-.947zM10 13a3 3 0 1 0 0-6 3 3 0 0 0 0 6z"
      />
    </svg>
  );
}

// Circular "restore" arrow — a recovery/roll-back glyph for the Recovery tab.
// 20×20 viewBox + strokeWidth 1.5 to match the sibling stroked nav icons (was a
// 16×16 viewBox at 1.4, which rendered a visibly heavier stroke at 22px).
function IconRecovery() {
  return (
    <svg width="22" height="22" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" className="shrink-0" aria-hidden="true">
      <path d="M10 3.125a6.875 6.875 0 1 0 6.5 4.625" strokeLinecap="round" />
      <path d="M16.875 2.5v4H12.875" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

const navBase =
  "flex items-center gap-3 px-3.5 py-2.5 rounded-lg text-[15px] font-medium transition-colors duration-150 select-none";
const navActive =
  "bg-accent text-accentContrast";
const navInactive =
  "text-[var(--sidebar-text)] hover:bg-carbon-hover hover:text-carbon-text";

function NavItem({ to, label, icon }: NavItem) {
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        `${navBase} ${isActive ? navActive : navInactive}`
      }
    >
      {icon}
      <span>{label}</span>
    </NavLink>
  );
}

// ---------------------------------------------------------------------------
// SidebarControls — theme toggle + language switcher in the sidebar footer
// ---------------------------------------------------------------------------

function Flag({ code }: { code: string }) {
  return (
    <span
      className={`fi fi-${code}`}
      style={{ width: "1.25em", height: "1em", display: "inline-block", flexShrink: 0 }}
    />
  );
}

function SidebarControls() {
  const { t, lang, setLanguage, languages } = useT();
  const [theme, setThemeState] = useState(getTheme);
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  const current = languages.find((l) => l.code === lang) ?? languages[0];

  function handleToggleTheme() {
    const next = toggleTheme();
    setThemeState(next);
  }

  // Close on outside click
  useEffect(() => {
    if (!open) return;
    function handler(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
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
    <div className="flex flex-col gap-1">
      {/* Language picker — flag + name, dropdown opens upward */}
      <div className="relative" ref={ref}>
        <button
          aria-label={`${t("language.label")}: ${current.label}`}
          title={`${t("language.label")}: ${current.label}`}
          aria-haspopup="listbox"
          aria-expanded={open}
          onClick={() => setOpen((v) => !v)}
          className={`${navBase} ${navInactive} w-full`}
        >
          <Flag code={current.flag} />
          <span>{current.label}</span>
        </button>
        {open && (
          <div
            role="listbox"
            aria-label={t("language.label")}
            className="absolute left-0 bottom-full mb-1 z-50 w-48 max-h-60 overflow-y-auto rounded-xl border border-carbon-border bg-carbon-surface shadow-lg"
            style={{ scrollbarColor: "var(--carbon-border) transparent" }}
          >
            {languages.map((l) => (
              <button
                key={l.code}
                role="option"
                aria-selected={l.code === lang}
                onClick={() => { setLanguage(l.code); setOpen(false); }}
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

      {/* Dark / Light mode — icon + current-mode label */}
      <button
        onClick={handleToggleTheme}
        title={t("theme.toggle")}
        className={`${navBase} ${navInactive} w-full`}
      >
        {theme === "dark" ? (
          <svg width="22" height="22" viewBox="0 0 20 20" fill="none" className="shrink-0">
            <path
              d="M17.5 12.5A7.5 7.5 0 017.5 2.5a7.5 7.5 0 100 15 7.5 7.5 0 0010-5z"
              stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round"
            />
          </svg>
        ) : (
          <svg width="22" height="22" viewBox="0 0 20 20" fill="none" className="shrink-0">
            <circle cx="10" cy="10" r="4" stroke="currentColor" strokeWidth="1.5" />
            <path
              d="M10 2v2M10 16v2M2 10h2M16 10h2M4.93 4.93l1.41 1.41M13.66 13.66l1.41 1.41M4.93 15.07l1.41-1.41M13.66 6.34l1.41-1.41"
              stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"
            />
          </svg>
        )}
        <span>{theme === "dark" ? t("theme.dark") : t("theme.light")}</span>
      </button>
    </div>
  );
}

export function Sidebar({ settings }: SidebarProps) {
  const { t } = useT();
  const { advanced, setAdvanced } = useAdvanced();
  const vmsEnabled = settings?.vmsEnabled ?? false;
  const flashEnabled = settings?.flashEnabled ?? false;
  const configEnabled = settings?.configEnabled ?? false;

  return (
    <aside className="flex flex-col w-56 shrink-0 h-full bg-carbon-sidebar">
      {/* Logo + wordmark → Dashboard. Frameless SVG on the darker sidebar so the
          logo stands out; clicking anywhere on it returns to the Dashboard.
          Borderless throughout — the darker sidebar tone alone separates the rail. */}
      <NavLink
        to="/dashboard"
        aria-label={t("nav.dashboard")}
        className="flex items-center gap-2.5 px-4 py-5 hover:opacity-90 transition-opacity"
      >
        <img
          src="/logo.svg"
          alt="BombVault"
          className="h-16 w-16 object-contain shrink-0"
        />
        <span className="text-carbon-text font-bold text-xl tracking-tight leading-none whitespace-nowrap">
          BombVault
        </span>
      </NavLink>

      {/* Navigation */}
      <nav className="flex flex-col gap-1 p-3 flex-1">
        <NavItem
          to="/dashboard"
          label={t("nav.dashboard")}
          icon={<IconDashboard />}
        />
        {/* Always visible: disaster recovery is a core, non-expert flow. */}
        <NavItem
          to="/recovery"
          label={t("nav.recovery")}
          icon={<IconRecovery />}
        />
        <NavItem
          to="/containers"
          label={t("nav.containers")}
          icon={<IconContainers />}
        />
        {/* VMs / Flash tabs appear only once their domain is enabled. */}
        {vmsEnabled && (
          <NavItem to="/vms" label={t("nav.vms")} icon={<IconVM />} />
        )}
        {flashEnabled && (
          <NavItem to="/flash" label={t("nav.flash")} icon={<IconFlash />} />
        )}
        {/* Config self-backup tab appears only once its domain is enabled. */}
        {configEnabled && (
          <NavItem to="/config" label={t("nav.config")} icon={<IconConfig />} />
        )}
      </nav>

      {/* Bottom group: language, then dark/light, then advanced, then settings */}
      <div className="flex flex-col gap-1 p-3">
        <SidebarControls />
        {/* Simple / Advanced mode — a visible 2-segment switch above Settings;
            reveals expert controls across the app (per-browser preference).
            Mirrors the SourceToggle segmented-control pattern. */}
        <div className="inline-flex rounded-lg border border-carbon-border overflow-hidden w-full">
          <button
            type="button"
            onClick={() => setAdvanced(false)}
            className={`flex-1 px-3 py-1.5 text-sm transition-colors ${
              !advanced
                ? "bg-accent text-accentContrast"
                : "text-carbon-textSub hover:text-carbon-text"
            }`}
          >
            {t("mode.simple")}
          </button>
          <button
            type="button"
            onClick={() => setAdvanced(true)}
            className={`flex-1 px-3 py-1.5 text-sm transition-colors ${
              advanced
                ? "bg-accent text-accentContrast"
                : "text-carbon-textSub hover:text-carbon-text"
            }`}
          >
            {t("mode.advanced")}
          </button>
        </div>
        <span className="px-1 text-[11px] leading-snug text-carbon-textMuted">
          {t("mode.hint")}
        </span>
        <NavItem
          to="/settings"
          label={t("nav.settings")}
          icon={<IconSettings />}
        />
      </div>
    </aside>
  );
}
