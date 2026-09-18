// ---------------------------------------------------------------------------
// BottomNav — the mobile bottom bar.
//
// Below the 48rem breakpoint this is the desktop Sidebar's counterpart, and
// Layout mounts exactly one of the two (THE ONE chrome switch). The bar is a
// NORMAL-FLOW flex sibling of the `bv-main` scroller — never `position:
// fixed`: a fixed bar is an overlay the scroller ignores, which is exactly
// how content ends up hidden under chrome on a resizing viewport. As a
// shrink-0 sibling, the browser's own layout reserves the bar's height and
// the scroller ends above it by construction, in both orientations and under
// the Android keyboard's viewport resize.
//
// Slots are NEVER hand-typed: every destination slot is derived from the ONE
// nav registry (lib/navModel.ts) the desktop Sidebar reads — same order, same
// settings gates — so bar and Sidebar cannot drift apart about which
// destinations exist. The registry's `bar` members fill the destination
// slots; the fifth slot is the More trigger, which opens the MoreSheet (inside
// the BottomSheet primitive).
//
// EMPTINESS rule: the More trigger renders only while the sheet it opens
// would have content — at least one enabled non-bar destination, or a
// sign-out row (authEnabled). An empty sheet never exists; when there is
// nothing to show, the bar degrades to its destination slots alone.
//
// Label axis: the bar has its OWN axis in the control label engine
// ("bottombar", lib/controls.ts) — the same four modes as buttons/sidebar/
// tabs, consumed here the way Sidebar consumes "sidebar": hiding modes gate
// the 11px caption under the glyph while the label survives as the slot's
// aria-label and native title, never as nothing. "reactive" pairs with the
// coarse-pointer at-rest reveal rules in index.css (a tap fires before any
// hover state can exist on a touch screen, so a reactive bar slot reads as
// text at rest under coarse pointers and reveals on hover/focus on fine
// ones). The bar's own height is the static h-14 in EVERY mode — the height
// never follows the label mode, which is what keeps the shell's chrome
// geometry stable across the whole settings surface.
//
// Active/rest language: the active slot is FILLED — the whole slot carries
// the accent edge to edge, and every mark on it (caption and glyph alike)
// takes the ink paired to that fill via ordinary currentColor inheritance;
// resting slots stay the muted text token with no fill. Accent-coloured text
// on the bar ground reads as a link, not as a selection, which is why the
// earlier coloured-label treatment was replaced — filled is how this language
// says "selected" (anchored on .glim-coin-tile.glim-active in index.css).
// Status colors never touch controls; tokens only, no hex.
//
// Tap-on-active: tapping the ALREADY-active destination scrolls the scroller
// back to the top instead of navigating — and the navigation is actively
// SUPPRESSED (preventDefault before react-router's own handler, see
// tapDestination), not merely followed by a scroll, so the tap never stacks a
// duplicate history entry. The scroller is Layout's <main id="bv-main">, so
// the mechanism is passed down from Layout as a prop and nothing here queries
// the DOM for it.
// ---------------------------------------------------------------------------
import { useState, type CSSProperties, type MouseEvent } from "react";
import { NavLink, useLocation } from "react-router-dom";
import type { Settings } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { barDestinations, moreDestinations, type NavDestination } from "../../lib/navModel";
import { hidesLabel, labelWidth } from "../../lib/controls";
import { useLabelMode } from "../../lib/useLabelMode";
import { IconEllipsis } from "../navGlyphs";
import { MoreSheet } from "./MoreSheet";

export interface BottomNavProps {
  /** Already-loaded settings, owned by Layout — the chrome performs no
   *  fetches of its own; both surfaces derive from the ONE registry
   *  synchronously (no loading state exists). */
  settings: Settings | null;
  /** Whether a login password exists. Drives the More trigger's emptiness
   *  rule here, and the sign-out row inside the sheet. */
  authEnabled: boolean;
  /** Layout's tap-on-active scroll (the scroller is Layout's
   *  <main id="bv-main">) — passed on to the sheet's rows too. */
  scrollMainToTop: () => void;
}

// One bar slot, destination or More trigger alike: the 24px glyph box over
// the 11px caption, `flex-1` so the slots split the bar's width equally, and
// the full 56px (h-14) row as the touch target — comfortably over the 44px
// floor. min-w-0 + truncate keep de/fr labels single-line at 320px.
const slotBase =
  "flex min-w-0 flex-1 flex-col items-center justify-center gap-1 text-caption motion-safe:active:scale-[.97]";

export function BottomNav({ settings, authEnabled, scrollMainToTop }: BottomNavProps) {
  const { t } = useT();
  const location = useLocation();
  const [moreOpen, setMoreOpen] = useState(false);
  // The ONE registry, filtered to enabled bottom-bar destinations — the same
  // derivation the Sidebar's order guarantee rides on.
  const slots = barDestinations(settings);
  const hasMore = moreDestinations(settings).length > 0 || authEnabled;
  // The bar's own label axis (see the header): hiding modes gate the caption,
  // reactive pairs with index.css's coarse-pointer at-rest reveal.
  const labelMode = useLabelMode("bottombar");
  const showLabel = !hidesLabel(labelMode);
  const reactive = labelMode === "reactive";

  // NavLink's onClick fires BEFORE react-router's own Link handler, and Link
  // checks event.defaultPrevented before navigating (verified against the
  // installed react-router: `if (onClick) onClick(event); if
  // (!event.defaultPrevented) internalOnClick(event);`). So the current
  // location still equals the slot's route exactly when this tap is a
  // tap-on-active, and preventDefault() there suppresses the navigation —
  // without it every tap on the already-active slot ALSO pushes a duplicate
  // history entry (react-router has no same-location dedup), and the first
  // Android back-gesture appears to do nothing while the second leaves the
  // app: the broken-back-button read this contract exists to prevent.
  const tapDestination = (e: MouseEvent, to: string) => {
    if (location.pathname === to) {
      e.preventDefault();
      scrollMainToTop();
    }
  };

  return (
    <nav
      data-testid="bottom-nav"
      // Landmark name: the bar is the mobile counterpart of the desktop
      // Sidebar's <nav>, and a landmark needs an accessible name even though
      // exactly one of the two mounts at a time. Pinned by a source assert in
      // app/mobileShellSource.test.ts.
      aria-label={t("nav.mobileNavigation")}
      // Normal flow (see header): shrink-0 keeps the bar at its own height
      // while `main` flexes; the hairline rides the bar's TOP edge (border
      // token, never a shadow on an edge that must read as a surface
      // boundary), and the safe-area padding sits below the content so the
      // slots themselves stay a full h-14 on notched devices. The surface is
      // the SAME token the desktop rail reads — one nav-surface token.
      className="shrink-0 border-t border-carbon-border bg-carbon-sidebar pb-[var(--safe-area-bottom)]"
    >
      {/* h-14 static height: svh semantics by construction — a fixed-height
          row never resizes when the DYNAMIC viewport does (browser chrome,
          keyboard), which is the property that keeps the bar visible: static
          chrome sizes against svh semantics, in every label mode. */}
      <div className="flex h-14">
        {slots.map((d) => (
          <BarSlot key={d.to} destination={d} labelMode={labelMode} onTap={(e) => tapDestination(e, d.to)} />
        ))}
        {hasMore && (
          <button
            type="button"
            onClick={() => setMoreOpen(true)}
            title={showLabel ? undefined : t("nav.more")}
            aria-label={showLabel ? undefined : t("nav.more")}
            className={`${slotBase} ${reactive && !showLabel ? "glim-reactive" : ""} text-carbon-textMuted`}
          >
            {/* The glyph box is the 24px slot-icon box (the 16px generated
                glyph scales up to it, the same scaling the rail applies). */}
            <span className="flex h-6 w-6 items-center justify-center rounded-control [&_svg]:h-6 [&_svg]:w-6">
              <IconEllipsis />
            </span>
            <span
              className={`max-w-full truncate ${showLabel ? "" : reactive ? "glim-label-reactive" : "sr-only"}`}
              style={reactive && !showLabel ? ({ "--reactive-chars": labelWidth(t("nav.more")) } as CSSProperties) : undefined}
            >
              {t("nav.more")}
            </span>
          </button>
        )}
      </div>
      <MoreSheet
        open={moreOpen}
        onClose={() => setMoreOpen(false)}
        settings={settings}
        authEnabled={authEnabled}
        scrollMainToTop={scrollMainToTop}
      />
    </nav>
  );
}

// One destination slot. NavLink drives the active language off the route
// (native aria-current), the same className-by-isActive shape the desktop
// rail's NavItem uses — no parallel active-state bookkeeping to keep in sync.
// The label mode comes from the bar's own "bottombar" axis: hiding modes keep
// the caption in the DOM but out of view (sr-only) — or reveal-on-hover in
// reactive mode — while aria-label + title carry the slot's name and hover
// bubble, so a glyph-mode bar never becomes eleven unnamed pictures.
function BarSlot({
  destination,
  labelMode,
  onTap,
}: {
  destination: NavDestination;
  labelMode: ReturnType<typeof useLabelMode>;
  onTap: (e: MouseEvent) => void;
}) {
  const { t } = useT();
  const Icon = destination.icon;
  const label = t(destination.labelKey);
  const showLabel = !hidesLabel(labelMode);
  const reactive = labelMode === "reactive";
  return (
    <NavLink
      to={destination.to}
      onClick={onTap}
      title={showLabel ? undefined : label}
      aria-label={showLabel ? undefined : label}
      className={({ isActive }) =>
        `${slotBase} rounded-control ${reactive && !showLabel ? "glim-reactive" : ""} ${isActive ? "bg-accent text-accentContrast" : "text-carbon-textMuted"}`
      }
      style={reactive && !showLabel ? ({ "--reactive-chars": labelWidth(label) } as CSSProperties) : undefined}
    >
      {({ isActive }) => (
        <>
          {/* The fill is the SLOT's, edge to edge — glyph and caption sit ON
              the accent fill and both take the ink it was paired with
              (text-accentContrast): glyphs draw fill="currentColor" per
              navGlyphs' contract, so no svg utility is needed. Anchored on
              .glim-coin-tile.glim-active / .glim-hue-icon in index.css —
              filled is how this language says "this one is selected". The
              fill is paint-only: no padding, height, width or gap token
              changed, which is what keeps the narrow-viewport single-line /
              no-overflow asserts and the 44px touch floor true by
              construction. */}
          <span className="flex h-6 w-6 items-center justify-center rounded-control [&_svg]:h-6 [&_svg]:w-6">
            <Icon />
          </span>
          {/* 400 rest, 600 active — the caption's two sanctioned weights. */}
          <span
            className={`max-w-full truncate ${isActive ? "font-semibold" : ""} ${showLabel ? "" : reactive ? "glim-label-reactive" : "sr-only"}`}
          >
            {label}
          </span>
        </>
      )}
    </NavLink>
  );
}
