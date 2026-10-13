import { NavLink, useLocation, useNavigate } from "react-router-dom";
import type { CSSProperties } from "react";
import { logout, type Settings } from "../lib/api";
import { useT } from "../lib/i18n";
import { hueVars } from "../lib/appearance";
import { onDestination, sidebarDestinations, type NavDestination } from "../lib/navModel";
import { useNavCounts, type NavCount as Count } from "../lib/navCounts";
import { useNavPins } from "../lib/navPins";

import { IconSignOut } from "./glyphs";
import { NavCount } from "./NavCount";
import { useLabelMode } from "../lib/useLabelMode";
import { hidesLabel, labelWidth } from "../lib/controls";
import { useTipBubble } from "../lib/useTipBubble";
import { useLogoHold } from "../lib/useLogoHold";

// Many files import the glyphs from this module, so they are re-exported here.
export {
  IconContainers,
  IconVM,
  IconFiles,
  IconZFS,
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
  IconCheck,
  IconSync,
  IconGear,
  IconClose,
  IconCopy,
  IconDashboard,
  IconRecovery,
  IconFlash,
  IconConfig,
} from "./navGlyphs";

interface SidebarProps {
  settings: Settings | null;
  /** Whether a login password is set; the sign-out row appears only then. */
  authEnabled: boolean;
}

interface NavItem {
  destination: NavDestination;
  /** The row's rainbow position: its rank among the rows the rail shows. */
  hueIndex: number;
  count?: Count;
}

// `transition` rather than `transition-colors` so the hover and press transforms
// animate too. Every transform is motion-safe, so reduced motion gets colour
// feedback only.
const navBase =
  "glim-nav-row flex items-center gap-3 px-3.5 rounded-pill text-sm font-medium transition duration-150 select-none motion-safe:active:scale-[var(--motion-press-scale)]";
const navActive =
  "bg-accent text-accentContrast";
// translate-x is physical, so the hover nudge would point away from the content
// once the rail sits on the right; `rtl:` negates it. The `!` wins over the base
// rule whatever order Tailwind emits them in.
const navInactive =
  "text-(--sidebar-text) hover:bg-carbon-hover hover:text-carbon-text motion-safe:hover:translate-x-0.5 motion-safe:hover:rtl:-translate-x-0.5!";

// railVars places a row in the rail: its rainbow hue, and its turn and first
// direction in the shock that runs down the rail when the logo goes off.
function railVars(i: number): CSSProperties {
  return { ...hueVars(i), "--rail-i": i, "--rail-dir": i % 2 ? -1 : 1 } as CSSProperties;
}

// NavItem is one destination row. On the current route glim-active switches the
// icon tint off, because the filled badge already shows the hue.
function NavItem({ destination, hueIndex, count }: NavItem) {
  const { t } = useT();
  const label = t(destination.labelKey);
  const Icon = destination.icon;
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
  const here = onDestination(useLocation().pathname, destination);
  return (
    <>
      <NavLink
        to={destination.to}
        ref={tooltip.ref}
        aria-describedby={tooltip.describedBy}
        {...tooltip.handlers}
        className={({ isActive }) =>
          `${navBase} relative ${showLabel ? "" : "justify-center"}${reactive ? " glim-reactive" : ""} glim-hue glim-hue-icon ${isActive || here ? `${navActive} glim-active` : navInactive}`
        }
        style={
          {
            ...railVars(hueIndex),
            ...(reactive ? { "--reactive-chars": labelWidth(label) } : {}),
          } as CSSProperties
        }
      >
        {showIcon && <Icon />}
        {/* Hidden, never removed: without it the link has no accessible name.
            `sr-only` is absolutely positioned, so it leaves no gap beside the
            centred glyph. */}
        <span className={showLabel ? undefined : reactive ? "glim-label-reactive" : "sr-only"}>{label}</span>
        {/* A row without a visible label centres its glyph, and a figure in
            the flow would push it aside. The reactive row keeps the figure at
            its end, the narrow rail hangs it on the corner. */}
        {count && count.value > 0 && (
          <NavCount
            count={count}
            size={showLabel || reactive ? "row" : "corner"}
            className={
              showLabel ? "ms-auto" : reactive ? "absolute end-3 top-1/2 -translate-y-1/2" : "absolute end-1 top-0.5"
            }
          />
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
            ...railVars(hueIndex),
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

export function Sidebar({ settings, authEnabled }: SidebarProps) {
  const { t } = useT();
  const navigate = useNavigate();
  const pins = useNavPins(settings);
  const entries = sidebarDestinations(settings, pins);
  const list = entries.filter((d) => !d.footer);
  const foot = entries.filter((d) => d.footer);
  const counts = useNavCounts();
  // Every row of the rail takes the palette position of its rank, sign out
  // included, so a hidden entry leaves no gap and the rows before it keep
  // their colour.
  const footHue = list.length + (authEnabled ? 1 : 0);

  // The brand block is GlimStone's: the mark centred above the name, smaller
  // and without the name in the narrow rail. The button's aria-label names it
  // in every mode.
  const railMode = useLabelMode("sidebar");
  const railLabels = !hidesLabel(railMode);
  const railReactive = railMode === "reactive";
  const railNarrow = railMode === "glyph";
  const markBox = railNarrow ? "h-11 w-11" : "h-26 w-26";

  // Held, the logo shakes harder and harder and then goes off; a short click
  // still goes to the overview.
  const hold = useLogoHold(() => navigate("/dashboard"));

  return (
    // Only glyph mode narrows the rail. Reactive cannot: its labels slide back
    // inside the rows and the active row keeps its label, which a narrow rail
    // would clip. A window shorter than the rows scrolls the rail rather than
    // cutting off the bottom group with Settings.
    <aside
      data-testid="desktop-sidebar"
      className={`flex flex-col ${railNarrow ? "w-(--rail-narrow)" : "w-56"} shrink-0 h-full overflow-x-hidden overflow-y-auto rounded-card bg-carbon-sidebar ${hold.phase === "gone" ? "glim-rail-quake" : ""}`}
      style={{ scrollbarWidth: "thin", scrollbarColor: "var(--carbon-border) transparent" }}
    >
      {/* A button rather than a link, so a click and a long press can be told
          apart. */}
      <button
        type="button"
        aria-label={t("nav.overview")}
        {...hold.handlers}
        className={`glim-logo-btn flex flex-col items-center gap-2 ${railNarrow ? "px-2 py-4" : "px-4 py-6"}${railReactive ? " glim-reactive" : ""} w-full cursor-pointer select-none hover:opacity-90 transition-opacity`}
      >
        <span className={`glim-logo-mark flex ${markBox} shrink-0 items-center justify-center ${hold.markClass}`}>
          <img
            ref={hold.markRef}
            src="/logo.svg"
            alt="BombVault"
            draggable={false}
            className={`glim-logo-img ${markBox} object-contain shrink-0 block`}
          />
        </span>
        {/* Removed rather than `sr-only`, unlike the other labels in the rail:
            the button's aria-label is its name, and a hidden copy would make a
            screen reader say "Overview BombVault". Reactive keeps it in the
            DOM to slide it back on hover, with aria-hidden for the same
            reason. */}
        {(railLabels || railReactive) && (
          <span
            aria-hidden={railReactive || undefined}
            className={`text-carbon-text font-bold text-xl tracking-tight leading-none whitespace-nowrap ${
              railReactive ? "glim-label-reactive" : ""
            }`}
          >
            BombVault
          </span>
        )}
      </button>

      <nav className="flex flex-col gap-1 p-3 flex-1">
        {list.map((d, i) => (
          <NavItem key={d.to} destination={d} hueIndex={i} count={d.count && counts[d.count]} />
        ))}
      </nav>

      <div className="flex flex-col gap-1 p-3">
        {authEnabled && <SidebarSignOut hueIndex={list.length} />}
        {foot.map((d, i) => (
          <NavItem key={d.to} destination={d} hueIndex={footHue + i} count={d.count && counts[d.count]} />
        ))}
      </div>
    </aside>
  );
}
