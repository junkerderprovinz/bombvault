import { NavLink, useNavigate } from "react-router-dom";
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

// Easter-egg state machine (Item 6): idle → wobble (shake) → boom (explode).
type EggState = "idle" | "wobble" | "boom";

// Fragment shatter grid (Item 6). On boom the logo breaks into an N×N grid of tiles,
// each painting its OWN slice of the current logo (via --egg-logo + a per-tile
// background-position, so at rest they reassemble the whole mark) and flying outward
// from the centre with spin + a little gravity. Corner tiles point at the corners;
// magnitude is randomised per tile. Pre-computed once at module load so the pattern
// stays stable across the re-renders the boom triggers (no re-randomising mid-boom).
const FRAG_N = 6; // 6×6 = 36 fragments
const FRAG_TILES = Array.from({ length: FRAG_N * FRAG_N }, (_, i) => {
  const row = Math.floor(i / FRAG_N);
  const col = i % FRAG_N;
  const mid = (FRAG_N - 1) / 2;
  const vx = col - mid; // outward direction from centre
  const vy = row - mid;
  const spread = 15 + Math.random() * 13; // per-unit magnitude, randomised
  const dx = Math.round(vx * spread + (Math.random() - 0.5) * 12);
  const dy = Math.round(vy * spread + (Math.random() - 0.5) * 12);
  return {
    left: `${(col * 100) / FRAG_N}%`,
    top: `${(row * 100) / FRAG_N}%`,
    size: `${100 / FRAG_N}%`,
    bgPos: `${(col / (FRAG_N - 1)) * 100}% ${(row / (FRAG_N - 1)) * 100}%`,
    dx: `${dx}px`,
    dy: `${dy}px`,
    rot: `${Math.round((Math.random() - 0.5) * 560)}deg`,
    delay: `${Math.round(Math.random() * 90)}ms`,
  };
});

// Overlapping soft radial puffs that build the billowing fire→smoke explosion cloud.
const BOOM_CLOUD = [
  { cx: "-6px", cy: "-4px", delay: "0ms", hot: true },
  { cx: "16px", cy: "-8px", delay: "40ms", hot: true },
  { cx: "-18px", cy: "6px", delay: "70ms", hot: false },
  { cx: "10px", cy: "14px", delay: "110ms", hot: false },
  { cx: "0px", cy: "-16px", delay: "150ms", hot: false },
];

// Flying sparks — alternating hot yellow / orange, radial from the centre, staggered.
// Kept module-level so the array stays stable across renders (no re-randomising).
const BOOM_PARTICLES = Array.from({ length: 14 }, (_, i) => {
  const angle = (Math.PI * 2 * i) / 14 + (i % 2) * 0.22;
  const dist = 34 + (i % 3) * 12;
  return {
    tx: `${Math.round(Math.cos(angle) * dist)}px`,
    ty: `${Math.round(Math.sin(angle) * dist)}px`,
    spark: i % 2 === 0 ? "#fff57c" : "#f68e32",
    delay: `${Math.round((i % 4) * 18)}ms`,
  };
});

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

// Stacked-layers glyph for the Simple/Advanced view toggle — "more layers = more
// controls". Deliberately distinct from IconConfig (sliders) and IconSettings (cog).
function IconLayers() {
  return (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinejoin="round" strokeLinecap="round" className="shrink-0" aria-hidden="true">
      <path d="M12 2 2 7l10 5 10-5-10-5Z" />
      <path d="m2 17 10 5 10-5" />
      <path d="m2 12 10 5 10-5" />
    </svg>
  );
}

// `transition` (not just `transition-colors`) so the transform-based hover/press
// micro-interactions below animate too; all transforms are motion-safe-gated so
// reduced-motion users get colour-only feedback (Item 7a/7d).
const navBase =
  "flex items-center gap-3 px-3.5 py-2.5 rounded-lg text-[15px] font-medium transition duration-150 select-none motion-safe:active:scale-[.97]";
const navActive =
  "bg-accent text-accentContrast";
const navInactive =
  "text-[var(--sidebar-text)] hover:bg-carbon-hover hover:text-carbon-text motion-safe:hover:translate-x-0.5";

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
  const { advanced, setAdvanced } = useAdvanced();
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

      {/* Simple / Advanced view — a single-click toggle that mirrors the theme row
          above (same height, hover, press feedback). The label shows the CURRENT
          view; a click flips it. Replaces the old segmented switch + hint (Item 4). */}
      <button
        onClick={() => setAdvanced(!advanced)}
        title={advanced ? t("mode.advancedView") : t("mode.simpleView")}
        aria-pressed={advanced}
        className={`${navBase} ${navInactive} w-full`}
      >
        <IconLayers />
        <span>{advanced ? t("mode.advancedView") : t("mode.simpleView")}</span>
      </button>
    </div>
  );
}

export function Sidebar({ settings }: SidebarProps) {
  const { t } = useT();
  const navigate = useNavigate();
  const vmsEnabled = settings?.vmsEnabled ?? false;
  const flashEnabled = settings?.flashEnabled ?? false;
  const configEnabled = settings?.configEnabled ?? false;

  // Easter egg (Item 6): press-and-hold the logo → it wobbles, then explodes,
  // then reappears. A short click still navigates to the Dashboard; once the
  // hold has fired the egg, the trailing click is suppressed.
  const [eggState, setEggState] = useState<EggState>("idle");
  const holdRef = useRef<number | null>(null); // 500ms pre-fire hold timer
  const seqRef = useRef<number[]>([]);         // wobble→boom→idle sequence timers
  const firedRef = useRef(false);              // did the hold fire the egg?

  function startHold() {
    if (eggState !== "idle") return; // ignore new presses while an egg is playing
    firedRef.current = false;
    if (holdRef.current !== null) window.clearTimeout(holdRef.current);
    holdRef.current = window.setTimeout(() => {
      holdRef.current = null;
      firedRef.current = true; // the click that follows the release must not navigate
      setEggState("wobble");
      const toBoom = window.setTimeout(() => {
        setEggState("boom");
        const toIdle = window.setTimeout(() => {
          setEggState("idle");
          firedRef.current = false;
        }, 1400);
        seqRef.current.push(toIdle);
      }, 900);
      seqRef.current.push(toBoom);
    }, 500);
  }

  // Release/leave before the hold fires → cancel so the click navigates normally.
  // If the egg already fired we leave the sequence running to play out.
  function cancelHold() {
    if (holdRef.current !== null) {
      window.clearTimeout(holdRef.current);
      holdRef.current = null;
    }
  }

  function handleLogoClick() {
    if (firedRef.current) return; // the hold fired the egg → swallow the navigation
    navigate("/dashboard");
  }

  // Clear any pending timers on unmount.
  useEffect(() => {
    return () => {
      if (holdRef.current !== null) window.clearTimeout(holdRef.current);
      for (const id of seqRef.current) window.clearTimeout(id);
    };
  }, []);

  const eggClass =
    eggState === "wobble" ? "bv-egg-wobble" : eggState === "boom" ? "bv-egg-boom" : "bv-logo-idle";

  return (
    <aside className="flex flex-col w-56 shrink-0 h-full bg-carbon-sidebar">
      {/* Logo + wordmark → Dashboard. Two theme-specific marks auto-switch via the
          `dark:` variant (dark mark on the light surface, light mark on the dark
          surface). A short click navigates to the Dashboard; press-and-hold fires
          the easter egg (Item 6). It's a button (not a link) so click vs. long-press
          is fully under our control. */}
      <button
        type="button"
        aria-label={t("nav.dashboard")}
        onClick={handleLogoClick}
        onPointerDown={startHold}
        onPointerUp={cancelHold}
        onPointerLeave={cancelHold}
        onPointerCancel={cancelHold}
        onContextMenu={(e) => e.preventDefault()}
        className="bv-logo-btn flex items-center gap-2.5 px-4 py-5 w-full text-left cursor-pointer select-none hover:opacity-90 transition-opacity"
      >
        <span className="relative inline-flex h-16 w-16 shrink-0 items-center justify-center">
          <span className={`bv-logo-mark flex h-16 w-16 items-center justify-center ${eggClass}`}>
            <img
              src="/logo.svg"
              alt="BombVault"
              draggable={false}
              className="bv-logo-img h-16 w-16 object-contain shrink-0 block dark:hidden"
            />
            <img
              src="/logo-light.svg"
              alt="BombVault"
              draggable={false}
              className="bv-logo-img h-16 w-16 object-contain shrink-0 hidden dark:block"
            />
            {/* At boom the <img> is hidden (CSS) and the mark shatters into flying
                tiles, each showing its own slice of the current logo. */}
            {eggState === "boom" && (
              <span className="bv-frag-grid" aria-hidden="true">
                {FRAG_TILES.map((f, i) => (
                  <span
                    key={i}
                    className="bv-frag"
                    style={
                      {
                        left: f.left,
                        top: f.top,
                        width: f.size,
                        height: f.size,
                        backgroundPosition: f.bgPos,
                        "--dx": f.dx,
                        "--dy": f.dy,
                        "--rot": f.rot,
                        "--delay": f.delay,
                      } as React.CSSProperties
                    }
                  />
                ))}
              </span>
            )}
          </span>
          {eggState === "boom" && (
            <span className="bv-boom-fx" aria-hidden="true">
              {/* Billowing fire→smoke cloud behind the flying fragments. */}
              {BOOM_CLOUD.map((c, i) => (
                <span
                  key={`c${i}`}
                  className={`bv-cloud ${c.hot ? "bv-cloud--hot" : "bv-cloud--smoke"}`}
                  style={{ "--cx": c.cx, "--cy": c.cy, "--delay": c.delay } as React.CSSProperties}
                />
              ))}
              {/* Flying sparks for extra energy. */}
              {BOOM_PARTICLES.map((p, i) => (
                <span
                  key={`p${i}`}
                  className="bv-particle"
                  style={
                    {
                      "--tx": p.tx,
                      "--ty": p.ty,
                      "--spark": p.spark,
                      "--delay": p.delay,
                    } as React.CSSProperties
                  }
                />
              ))}
            </span>
          )}
        </span>
        <span className="font-brand text-carbon-text text-xl tracking-tight leading-none whitespace-nowrap">
          BombVault
        </span>
      </button>

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

      {/* Bottom group: language, dark/light and the Simple/Advanced view toggle
          (all in SidebarControls), then Settings. */}
      <div className="flex flex-col gap-1 p-3">
        <SidebarControls />
        <NavItem
          to="/settings"
          label={t("nav.settings")}
          icon={<IconSettings />}
        />
      </div>
    </aside>
  );
}
