import { NavLink, useNavigate } from "react-router-dom";
import { useState, useRef, useEffect, type CSSProperties } from "react";
import { logout, type Settings } from "../lib/api";
import { Badge, type BadgeTone } from "./Badge";
import { useAnomalySummary } from "../lib/useAnomalies";
import { useT } from "../lib/i18n";
import { useAdvanced } from "../lib/advanced";
import { hueVars } from "../lib/appearance";

import {
  IconContainers,
  IconVM,
  IconFiles,
  IconFleet,
  IconDashboard,
  IconRecovery,
  IconFlash,
  IconConfig,
  IconViewSimple,
  IconViewAdvanced,
  IconGear,
  IconAnomalies,
} from "./navGlyphs";
import { IconSignOut } from "./glyphs";
import { useLabelMode } from "../lib/useLabelMode";
import { hidesLabel, labelWidth } from "../lib/controls";
import { useTipBubble } from "../lib/useTipBubble";

// Many files import the glyphs from this module, so they are re-exported here.
export {
  IconContainers,
  IconVM,
  IconFiles,
  IconReceiver,
  IconFleet,
  IconFolder,
  IconLocal,
  IconCloud,
  IconDatabase,
  IconAdd,
  IconDownload,
  IconBackupNow,
  IconRestore,
  IconPower,
  IconLive,
  IconTrash,
  IconPencil,
  IconCheckCircle,
  IconSync,
  IconGear,
  IconClose,
  IconCopy,
  IconDashboard,
  IconRecovery,
  IconFlash,
  IconConfig,
  IconViewSimple,
  IconViewAdvanced,
} from "./navGlyphs";

interface SidebarProps {
  settings: Settings | null;
  /** Whether a login password is set; the sign-out row appears only then. */
  authEnabled: boolean;
}

interface NavItem {
  to: string;
  label: string;
  icon: React.ReactNode;
  /** The row's rainbow position, assigned by the caller in render order. */
  hueIndex: number;
  /** A figure beside the label, such as the open anomalies. It carries its
   *  own name, because the bare number says nothing on its own. */
  count?: { value: number; tone: BadgeTone; label: string };
}

// Easter egg: idle, then wobble, then boom.
type EggState = "idle" | "wobble" | "boom";

// On boom the logo breaks into an N×N grid of tiles, each painting its slice of
// the logo through background-position and flying outward with some spin.
// Computed at module load so re-renders during the boom do not re-randomise it.
const FRAG_N = 6;
const FRAG_TILES = Array.from({ length: FRAG_N * FRAG_N }, (_, i) => {
  const row = Math.floor(i / FRAG_N);
  const col = i % FRAG_N;
  const mid = (FRAG_N - 1) / 2;
  const vx = col - mid;
  const vy = row - mid;
  const spread = 15 + Math.random() * 13;
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

// Soft radial puffs that overlap into the fire and smoke cloud.
const BOOM_CLOUD = [
  { cx: "-6px", cy: "-4px", delay: "0ms", hot: true },
  { cx: "16px", cy: "-8px", delay: "40ms", hot: true },
  { cx: "-18px", cy: "6px", delay: "70ms", hot: false },
  { cx: "10px", cy: "14px", delay: "110ms", hot: false },
  { cx: "0px", cy: "-16px", delay: "150ms", hot: false },
];

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

// `transition` rather than `transition-colors` so the hover and press transforms
// animate too. Every transform is motion-safe, so reduced motion gets colour
// feedback only.
const navBase =
  "glim-nav-row flex items-center gap-3 px-3.5 rounded-control text-[15px] font-medium transition duration-150 select-none motion-safe:active:scale-[var(--motion-press-scale)]";
const navActive =
  "bg-accent text-accentContrast";
// translate-x is physical, so the hover nudge would point away from the content
// once the rail sits on the right; `rtl:` negates it. The `!` wins over the base
// rule whatever order Tailwind emits them in.
const navInactive =
  "text-(--sidebar-text) hover:bg-carbon-hover hover:text-carbon-text motion-safe:hover:translate-x-0.5 motion-safe:hover:rtl:-translate-x-0.5!";

// NavItem is one destination row. On the current route glim-active switches the
// icon tint off, because the filled badge already shows the hue.
function NavItem({ to, label, icon, hueIndex, count }: NavItem) {
  // The rail has its own label axis: reducing it to glyphs changes the layout,
  // not just the density.
  const labelMode = useLabelMode("sidebar");
  const showLabel = !hidesLabel(labelMode);
  const showIcon = labelMode !== "text";
  // Reactive centres the glyph and slides the name back in on hover. The rail
  // keeps its full width in this mode, so nothing reflows.
  const reactive = labelMode === "reactive";
  // A row whose text is hidden names itself in the tooltip bubble. Reactive
  // mode needs none, since hovering brings the word back.
  const tooltip = useTipBubble(showLabel || reactive ? undefined : label);
  return (
    <>
      <NavLink
        to={to}
        ref={tooltip.ref}
        aria-describedby={tooltip.describedBy}
        {...tooltip.handlers}
        className={({ isActive }) =>
          `${navBase} ${showLabel ? "" : "justify-center"}${reactive ? " glim-reactive" : ""} glim-hue glim-hue-icon ${isActive ? `${navActive} glim-active` : navInactive}`
        }
        style={
          {
            ...(hueVars(hueIndex) as CSSProperties),
            ...(reactive ? { "--reactive-chars": labelWidth(label) } : {}),
          } as CSSProperties
        }
      >
        {showIcon && icon}
        {/* Hidden, never removed: without it the link has no accessible name.
            `sr-only` is absolutely positioned, so it leaves no gap beside the
            centred glyph. */}
        <span className={showLabel ? undefined : reactive ? "glim-label-reactive" : "sr-only"}>{label}</span>
        {count && count.value > 0 && (
          <Badge tone={count.tone} size="small" shape="pill" ariaLabel={count.label} className="ms-auto">
            {count.value}
          </Badge>
        )}
      </NavLink>
      {tooltip.bubble}
    </>
  );
}

// SidebarSignOut is the footer's sign-out row. Signing out clears this
// browser's cookie and reloads, which brings the login screen back.
function SidebarSignOut({ hueIndex }: { hueIndex: number }) {
  const { t } = useT();
  const labelMode = useLabelMode("sidebar");
  const showLabel = !hidesLabel(labelMode);
  const showIcon = labelMode !== "text";
  const reactive = labelMode === "reactive";
  const label = t("auth.logout");
  const tooltip = useTipBubble(showLabel || reactive ? undefined : label);

  async function signOut() {
    await logout().catch(() => undefined);
    const g = globalThis as unknown as { location: { reload(): void } };
    g.location.reload();
  }

  return (
    <>
      <button
        ref={tooltip.ref}
        onClick={() => void signOut()}
        aria-describedby={tooltip.describedBy}
        {...tooltip.handlers}
        // Hued like the nav rows it sits among. glim-nav-idle only decides when
        // the glyph takes the colour, not which colour it is.
        className={`${navBase} ${showLabel ? "" : "justify-center"}${reactive ? " glim-reactive" : ""} glim-hue glim-hue-icon glim-nav-idle ${navInactive} w-full`}
        style={
          {
            ...(hueVars(hueIndex) as CSSProperties),
            ...(reactive ? { "--reactive-chars": labelWidth(label) } : {}),
          } as CSSProperties
        }
      >
        {showIcon && <IconSignOut />}
        <span className={showLabel ? undefined : reactive ? "glim-label-reactive" : "sr-only"}>{label}</span>
      </button>
      {tooltip.bubble}
    </>
  );
}

// Flag is the country flag glyph used by the language picker in Settings.
export function Flag({ code }: { code: string }) {
  return (
    <span
      className={`fi fi-${code}`}
      style={{ width: "1.25em", height: "1em", display: "inline-block", flexShrink: 0 }}
    />
  );
}

// SidebarControls is the Simple/Advanced view toggle in the footer.
function SidebarControls({ hueIndex }: { hueIndex: number }) {
  const { t } = useT();
  const { advanced, setAdvanced } = useAdvanced();
  const labelMode = useLabelMode("sidebar");
  const showLabel = !hidesLabel(labelMode);
  const showIcon = labelMode !== "text";
  const reactive = labelMode === "reactive";
  const view = advanced ? t("mode.advancedView") : t("mode.simpleView");
  const tooltip = useTipBubble(showLabel || reactive ? undefined : view);

  return (
    <div className="flex flex-col gap-1">
      {/* The label shows the current view; a click flips it. */}
      <button
        ref={tooltip.ref}
        onClick={() => setAdvanced(!advanced)}
        aria-pressed={advanced}
        aria-describedby={tooltip.describedBy}
        {...tooltip.handlers}
        className={`${navBase} ${showLabel ? "" : "justify-center"}${reactive ? " glim-reactive" : ""} glim-hue glim-hue-icon glim-nav-idle ${navInactive} w-full`}
        style={
          {
            ...(hueVars(hueIndex) as CSSProperties),
            ...(reactive ? { "--reactive-chars": labelWidth(view) } : {}),
          } as CSSProperties
        }
      >
        {/* One glyph per view: in glyph mode the icon is all that tells the
            two apart. */}
        {showIcon && (advanced ? <IconViewAdvanced /> : <IconViewSimple />)}
        {/* Hidden, never removed: the toggle keeps its accessible name. */}
        <span className={showLabel ? undefined : reactive ? "glim-label-reactive" : "sr-only"}>{view}</span>
      </button>
      {tooltip.bubble}
    </div>
  );
}

export function Sidebar({ settings, authEnabled }: SidebarProps) {
  const { t } = useT();
  const navigate = useNavigate();
  const vmsEnabled = settings?.vmsEnabled ?? false;
  const flashEnabled = settings?.flashEnabled ?? false;
  const configEnabled = settings?.configEnabled ?? false;
  const filesEnabled = settings?.filesEnabled ?? false;
  const receiverEnabled = settings?.receiverEnabled ?? false;
  const fleetEnabled = settings?.fleetEnabled ?? false;
  const pullEnabled = settings?.pullEnabled ?? false;
  const anomaliesEnabled = settings?.anomalyEnabled ?? false;
  const { summary } = useAnomalySummary();
  const loudAnomalies = summary ? summary.open.critical + summary.open.warning : 0;

  // The logo row follows the rail's label mode too: with labels hidden the mark
  // centres on the glyph column and the wordmark goes. The button's aria-label
  // names it in every mode.
  const railMode = useLabelMode("sidebar");
  const railLabels = !hidesLabel(railMode);
  const railReactive = railMode === "reactive";
  const railNarrow = railMode === "glyph";
  // One size for all four boxes the mark is drawn in, or the shatter grid and
  // the image end up in differently sized boxes.
  const markBox = railNarrow ? "h-12 w-12" : "h-16 w-16";


  // Easter egg: hold the logo and it wobbles, explodes and comes back. A short
  // click still goes to the Dashboard; once the hold has fired, the click that
  // follows is swallowed.
  const [eggState, setEggState] = useState<EggState>("idle");
  const holdRef = useRef<number | null>(null);
  const seqRef = useRef<number[]>([]);
  const firedRef = useRef(false);

  function startHold() {
    if (eggState !== "idle") return;
    firedRef.current = false;
    if (holdRef.current !== null) window.clearTimeout(holdRef.current);
    holdRef.current = window.setTimeout(() => {
      holdRef.current = null;
      firedRef.current = true;
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

  // Releasing or leaving before the hold fires cancels it, so the click
  // navigates normally. A sequence that already started plays out.
  function cancelHold() {
    if (holdRef.current !== null) {
      window.clearTimeout(holdRef.current);
      holdRef.current = null;
    }
  }

  function handleLogoClick() {
    if (firedRef.current) return;
    navigate("/dashboard");
  }

  useEffect(() => {
    return () => {
      if (holdRef.current !== null) window.clearTimeout(holdRef.current);
      for (const id of seqRef.current) window.clearTimeout(id);
    };
  }, []);

  const eggClass =
    eggState === "wobble" ? "glim-egg-wobble" : eggState === "boom" ? "glim-egg-boom" : "glim-logo-idle";

  return (
    // Only glyph mode narrows the rail. Reactive cannot: its labels slide back
    // inside the rows and the active row keeps its label, which a narrow rail
    // would clip. A window shorter than the rows scrolls the rail rather than
    // cutting off the bottom group with Settings.
    <aside
      className={`flex flex-col ${railNarrow ? "w-(--rail-narrow)" : "w-56"} shrink-0 h-full overflow-x-hidden overflow-y-auto rounded-card bg-carbon-sidebar`}
      style={{ scrollbarWidth: "thin", scrollbarColor: "var(--carbon-border) transparent" }}
    >
      {/* A button rather than a link, so a click and a long press can be told
          apart. The two images switch with the theme through `dark:`. */}
      <button
        type="button"
        aria-label={t("nav.dashboard")}
        onClick={handleLogoClick}
        onPointerDown={startHold}
        onPointerUp={cancelHold}
        onPointerLeave={cancelHold}
        onPointerCancel={cancelHold}
        onContextMenu={(e) => e.preventDefault()}
        className={`glim-logo-btn flex items-center ${railLabels ? "gap-2.5 px-4 text-start" : "justify-center px-0"}${railReactive ? " glim-reactive" : ""} py-5 w-full cursor-pointer select-none hover:opacity-90 transition-opacity`}
      >
        {/* `--egg-mark` passes the mark size to the shatter tiles, whose
            background is sized to the whole mark. */}
        <span
          className={`relative inline-flex ${markBox} shrink-0 items-center justify-center`}
          style={{ "--egg-mark": railNarrow ? "48px" : "64px" } as CSSProperties}
        >
          <span className={`glim-logo-mark flex ${markBox} items-center justify-center ${eggClass}`}>
            <img
              src="/logo.svg"
              alt="BombVault"
              draggable={false}
              className={`glim-logo-img ${markBox} object-contain shrink-0 block dark:hidden`}
            />
            <img
              src="/logo-light.svg"
              alt="BombVault"
              draggable={false}
              className={`glim-logo-img ${markBox} object-contain shrink-0 hidden dark:block`}
            />
            {/* At boom the CSS hides the <img> and the tiles take its place. */}
            {eggState === "boom" && (
              <span className="glim-frag-grid" aria-hidden="true">
                {FRAG_TILES.map((f, i) => (
                  <span
                    key={i}
                    className="glim-frag"
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
            <span className="glim-boom-fx" aria-hidden="true">
              {BOOM_CLOUD.map((c, i) => (
                <span
                  key={`c${i}`}
                  className={`glim-cloud ${c.hot ? "glim-cloud--hot" : "glim-cloud--smoke"}`}
                  style={{ "--cx": c.cx, "--cy": c.cy, "--delay": c.delay } as React.CSSProperties}
                />
              ))}
              {BOOM_PARTICLES.map((p, i) => (
                <span
                  key={`p${i}`}
                  className="glim-particle"
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
        {/* Removed rather than `sr-only`, unlike the other labels in the rail:
            the button's aria-label is its name, and a hidden copy would make a
            screen reader say "Dashboard BombVault". Reactive keeps it in the
            DOM to slide it back on hover, with aria-hidden for the same
            reason. */}
        {(railLabels || railReactive) && (
          <span
            aria-hidden={railReactive || undefined}
            className={`text-carbon-text font-bold text-xl tracking-tight leading-none whitespace-nowrap${
              railReactive ? " glim-label-reactive" : ""
            }`}
          >
            BombVault
          </span>
        )}
      </button>

      {/* Every row of the rail, the footer included, takes the next palette
          position in render order. A domain that is off short-circuits before
          nextHue() runs, so hidden tabs leave no gap and the rows before them
          keep their colour. */}
      {(() => {
        let hueSeq = 0;
        const nextHue = () => hueSeq++;
        return (
          <>
            <nav className="flex flex-col gap-1 p-3 flex-1">
              <NavItem
                to="/dashboard"
                label={t("nav.dashboard")}
                icon={<IconDashboard />}
                hueIndex={nextHue()}
              />
              {/* Right under Dashboard: a finding that holds old backups back
                  has to be reachable whatever order the dashboard cards are
                  in. Detection is a feature switch, not a domain, so the row
                  goes when it is off; the Settings card still links to the
                  page for what was found earlier. */}
              {anomaliesEnabled && (
                <NavItem
                  to="/anomalies"
                  label={t("nav.anomalies")}
                  icon={<IconAnomalies />}
                  hueIndex={nextHue()}
                  count={{
                    value: loudAnomalies,
                    tone: summary && summary.open.critical > 0 ? "fail" : "warn",
                    label: t("anomaly.navCountAria", loudAnomalies),
                  }}
                />
              )}
              {/* Always visible: disaster recovery is a core, non-expert flow. */}
              <NavItem
                to="/recovery"
                label={t("nav.recovery")}
                icon={<IconRecovery />}
                hueIndex={nextHue()}
              />
              <NavItem
                to="/containers"
                label={t("nav.containers")}
                icon={<IconContainers />}
                hueIndex={nextHue()}
              />
              {vmsEnabled && (
                <NavItem to="/vms" label={t("nav.vms")} icon={<IconVM />} hueIndex={nextHue()} />
              )}
              {flashEnabled && (
                <NavItem to="/flash" label={t("nav.flash")} icon={<IconFlash />} hueIndex={nextHue()} />
              )}
              {filesEnabled && (
                <NavItem to="/files" label={t("nav.files")} icon={<IconFiles />} hueIndex={nextHue()} />
              )}
              {configEnabled && (
                <NavItem to="/config" label={t("nav.config")} icon={<IconConfig />} hueIndex={nextHue()} />
              )}
              {/* Receiver, Fleet and Pull share one row. The Instances page
                  shows only the tabs whose setting is on. */}
              {(receiverEnabled || fleetEnabled || pullEnabled) && (
                <NavItem
                  to="/instances"
                  label={t("instances.title")}
                  icon={<IconFleet />}
                  hueIndex={nextHue()}
                />
              )}
            </nav>

            <div className="flex flex-col gap-1 p-3">
              {authEnabled && <SidebarSignOut hueIndex={nextHue()} />}
              <SidebarControls hueIndex={nextHue()} />
              <NavItem
                to="/settings"
                label={t("nav.settings")}
                icon={<IconGear />}
                hueIndex={nextHue()}
              />
            </div>
          </>
        );
      })()}
    </aside>
  );
}
