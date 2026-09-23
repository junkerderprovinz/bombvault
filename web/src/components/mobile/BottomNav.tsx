// ---------------------------------------------------------------------------
// BottomNav; the mobile bottom bar.
//
// Below the 48rem breakpoint this is the desktop Sidebar's counterpart, and
// Layout mounts exactly one of the two (the one chrome switch). The bar is a
// normal-flow flex sibling of the `bv-main` scroller; never `position:
// fixed`: a fixed bar is an overlay the scroller ignores, which is exactly
// how content ends up hidden under chrome on a resizing viewport. As a
// shrink-0 sibling, the browser's own layout reserves the bar's height and
// the scroller ends above it by construction, in both orientations and under
// the Android keyboard's viewport resize.
//
// The bar reads as a card, in the same language as the desktop rail: the
// <nav> host is transparent and reserves the full bar height from the screen
// edge, and the surface itself is a rounded sidebar-token card inset from
// all four edges; horizontal insets are the fixed gutter widened by the
// left/right safe areas, and the bottom gutter sits above the bottom safe
// area. Separation from the page comes from shade alone (the app's ground
// token shows around the card); no border, ring or hairline anywhere on the
// bar, because surfaces in this app are separated by tint, never by lines.
//
// Slots are never hand-typed: every destination slot is derived from the one
// nav registry (lib/navModel.ts); same order, same settings gates. The
// desktop rail keeps its own hand-written JSX (its render-order hue counter
// and inline gates are the part the registry cannot own), so
// the two listings are held equal by test rather than by construction:
// Sidebar.navModel.dom.test.tsx fails the moment Sidebar and the registry
// disagree about which destinations exist. The registry's `bar` members fill
// the destination slots; the fifth slot is the More trigger, which opens the
// MoreSheet (inside the BottomSheet primitive).
//
// More trigger: always rendered under the breakpoint, never gated on the
// sheet's content. The sheet keeps content on every instance; the
// Simple/Advanced view toggle (and, with a password set, sign-out) render
// below the destination rows; so a content-based emptiness rule would only
// make the trigger flicker in and out across settings flips while the sheet
// it opens always has something to show. The trigger announces itself to
// assistive tech (haspopup/expanded, like any disclosure that opens a
// dialog) and reads as active while the current route lives on the More
// side of the registry (the gated tabs, Instances and Settings), so a user
// on one of those routes can still see where they are.
//
// Label axis: the bar has its own axis in the control label engine
// ("bottombar", lib/controls.ts); the same four modes as buttons/sidebar/
// tabs, consumed here the way Sidebar consumes "sidebar": hiding modes gate
// the 11px caption under the glyph while the label survives as the slot's
// aria-label and opens the tip bubble, never as nothing. "reactive" pairs with the
// coarse-pointer at-rest reveal rules in index.css (a tap fires before any
// hover state can exist on a touch screen, so a reactive bar slot reads as
// text at rest under coarse pointers and reveals on hover/focus on fine
// ones). The bar's own height is the static h-14 in every mode; the height
// never follows the label mode, which is what keeps the shell's chrome
// geometry stable across the whole settings surface.
//
// Colour engine: every slot carries `glim-hue` + `hueVars(i)`
// exactly like a Sidebar row; position i being the slot's own rank among
// the slots actually rendered, so hiding a gated slot shifts later slots
// the same way a hidden Sidebar tab does. The active slot adds
// `glim-active` and fills with the accent edge to edge, and every mark on
// it (caption and glyph alike) takes the ink paired to that fill via
// ordinary currentColor inheritance; resting slots stay tinted by their own
// hue. Accent-coloured text on the bar ground reads as a link, not as a
// selection; filled is how this language says "selected" (anchored on
// .glim-coin-tile.glim-active in index.css). Status colors never touch
// controls; tokens only, no hex.
//
// Tap-on-active: tapping the already-active destination scrolls the scroller
// back to the top instead of navigating; and the navigation is actively
// suppressed (preventDefault before react-router's own handler, see
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
import { hueVars } from "../../lib/appearance";
import { useLabelMode } from "../../lib/useLabelMode";
import { useTipBubble } from "../../lib/useTipBubble";
import { IconEllipsis } from "../navGlyphs";
import { MoreSheet } from "./MoreSheet";

export interface BottomNavProps {
  /** Already-loaded settings, owned by Layout; the chrome performs no
   *  fetches of its own; both surfaces derive from the one registry
   *  synchronously (no loading state exists). */
  settings: Settings | null;
  /** Whether a login password exists. Gates the sign-out row inside the
   *  sheet, exactly like the desktop Sidebar footer. */
  authEnabled: boolean;
  /** Layout's tap-on-active scroll (the scroller is Layout's
   *  <main id="bv-main">); passed on to the sheet's rows too. */
  scrollMainToTop: () => void;
}

// One bar slot, destination or More trigger alike: the 24px glyph box over
// the 11px caption, `flex-1` so the slots split the bar's width equally, and
// the full 56px (h-14) row as the touch target; comfortably over the 44px
// floor. min-w-0 + truncate keep de/fr labels single-line at 320px. The press
// scale reads the motion-engine token, so the app's motion levels govern it
// (--motion-press-scale is 1 at "off": no scale at all there).
const slotBase =
  "flex min-w-0 flex-1 flex-col items-center justify-center gap-1 text-caption motion-safe:active:scale-[var(--motion-press-scale)]";

export function BottomNav({ settings, authEnabled, scrollMainToTop }: BottomNavProps) {
  const { t } = useT();
  const location = useLocation();
  const [moreOpen, setMoreOpen] = useState(false);
  // The one registry, filtered to enabled bottom-bar destinations; the same
  // derivation the Sidebar's order guarantee rides on.
  const slots = barDestinations(settings);
  // The bar's own label axis (see the header): hiding modes gate the caption,
  // reactive pairs with index.css's coarse-pointer at-rest reveal.
  const labelMode = useLabelMode("bottombar");
  const showLabel = !hidesLabel(labelMode);
  const reactive = labelMode === "reactive";
  // The More trigger reads as active while the current route lives on the
  // More side of the registry (the non-bar destinations, Settings
  // included), the same "you can see where you are" contract the
  // destination slots get from NavLink's isActive. One lookup, no special
  // case: /settings is an always-enabled registry entry, so a literal
  // `pathname === "/settings"` disjunct beside the lookup was dead code that
  // could only ever drift from it.
  const moreActive = moreDestinations(settings).some((d) => d.to === location.pathname);
  // The More trigger's bubble: worded only in hiding modes, the same rule
  // the sidebar's rows follow (reactive mode brings the word back on hover).
  const moreTip = useTipBubble(showLabel || reactive ? undefined : t("nav.more"));

  // NavLink's onClick fires before react-router's own Link handler, and Link
  // checks event.defaultPrevented before navigating (verified against the
  // installed react-router: `if (onClick) onClick(event); if
  // (!event.defaultPrevented) internalOnClick(event);`). So the current
  // location still equals the slot's route exactly when this tap is a
  // tap-on-active, and preventDefault() there suppresses the navigation;
  // without it every tap on the already-active slot also pushes a duplicate
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
      // while `main` flexes. The host paints nothing; the app's ground token
      // (the shell root's bg-carbon-background) is what shows around the
      // card; and the bottom safe-area padding sits below the card so the
      // card itself lifts off the screen edge on notched devices while the
      // slots stay a full h-14.
      className="shrink-0 bg-transparent pb-[var(--safe-area-bottom)]"
    >
      {/* The bar-card (see the header): the same surface token and radius the
          desktop rail reads, inset from all four edges; each side takes the
          page gutter (1rem, the same floor the scroller's side paddings use)
          widened by its own safe area, so an edge never slides under a notch
          in landscape. The top margin is the card's own ground: the scroller
          above ends flush (its pb-0 contract), so without it the last card of
          every page would rest on the bar card's edge. Separation is shade,
          not a line: no border or ring on the card. */}
      <div className="mt-2 ml-[max(1rem,var(--safe-area-left))] mr-[max(1rem,var(--safe-area-right))] mb-2 rounded-card bg-carbon-sidebar">
        {/* h-14 static height: svh semantics by construction; a fixed-height
            row never resizes when the dynamic viewport does (browser chrome,
            keyboard), which is the property that keeps the bar visible: static
            chrome sizes against svh semantics, in every label mode. */}
        <div className="flex h-14">
          {slots.map((d, i) => (
            <BarSlot
              key={d.to}
              destination={d}
              hueIndex={i}
              labelMode={labelMode}
              onTap={(e) => tapDestination(e, d.to)}
            />
          ))}
          {/* The More trigger: a disclosure that opens a dialog, so it
              announces haspopup/expanded; its hue position follows the
              enabled destination slots, exactly like the rail's own counter.
              In hiding modes its name lives in the aria-label and opens the
              same tip bubble the sidebar's rows use; the native title is the
              mechanism useTipBubble exists to replace. */}
          <button
            type="button"
            ref={moreTip.ref}
            onClick={() => setMoreOpen(true)}
            aria-label={showLabel ? undefined : t("nav.more")}
            aria-describedby={moreTip.describedBy}
            {...moreTip.handlers}
            aria-haspopup="dialog"
            aria-expanded={moreOpen}
            // The destination slots get "where am I" from NavLink's
            // aria-current; this one is a disclosure rather than a link to the
            // page, so it says "true" instead of "page". Without it the fill
            // is the only signal, and a screen reader or forced-colors user
            // has none.
            aria-current={moreActive ? "true" : undefined}
            className={`${slotBase} rounded-control ${reactive && !showLabel ? "glim-reactive" : ""} glim-hue glim-hue-icon ${moreActive ? "bg-accent text-accentContrast glim-active" : "text-carbon-textMuted"}`}
            style={
              {
                ...(hueVars(slots.length) as CSSProperties),
                ...(reactive && !showLabel ? { "--reactive-chars": labelWidth(t("nav.more")) } : {}),
              } as CSSProperties
            }
          >
            {/* The glyph box is the 24px slot-icon box (the 16px generated
                glyph scales up to it, the same scaling the rail applies). */}
            <span className="flex h-6 w-6 items-center justify-center rounded-control [&_svg]:h-6 [&_svg]:w-6">
              <IconEllipsis />
            </span>
            <span
              className={`max-w-full truncate ${showLabel ? "" : reactive ? "glim-label-reactive" : "sr-only"}`}
            >
              {t("nav.more")}
            </span>
          </button>
          {moreTip.bubble}
        </div>
      </div>
      <MoreSheet
        open={moreOpen}
        onClose={() => setMoreOpen(false)}
        settings={settings}
        authEnabled={authEnabled}
        scrollMainToTop={scrollMainToTop}
        hueOffset={slots.length + 1}
      />
    </nav>
  );
}

// One destination slot. NavLink drives the active language off the route
// (native aria-current), the same className-by-isActive shape the desktop
// rail's NavItem uses; no parallel active-state bookkeeping to keep in sync.
// `glim-hue`/`glim-hue-icon` ride unconditionally with the slot's own
// `hueIndex` (the caller's render-order rank, Sidebar's nextHue() semantics),
// and `glim-active` joins only the filled branch so index.css's
// `.glim-hue-icon:not(.glim-active)` guard leaves the filled slot's ink to
// currentColor; the rail's exact convention. The label mode comes from the
// bar's own "bottombar" axis: hiding modes keep the caption in the DOM but
// out of view (sr-only); or reveal-on-hover in reactive mode; while the
// aria-label and the tip bubble carry the slot's name, so a glyph-mode
// bar never becomes eleven unnamed pictures.
function BarSlot({
  destination,
  hueIndex,
  labelMode,
  onTap,
}: {
  destination: NavDestination;
  hueIndex: number;
  labelMode: ReturnType<typeof useLabelMode>;
  onTap: (e: MouseEvent) => void;
}) {
  const { t } = useT();
  const Icon = destination.icon;
  const label = t(destination.labelKey);
  const showLabel = !hidesLabel(labelMode);
  const reactive = labelMode === "reactive";
  // A slot whose caption is hidden names itself in the tip bubble, the
  // sidebar row's own mechanism; reactive mode needs none, since hovering
  // brings the word back.
  const tooltip = useTipBubble(showLabel || reactive ? undefined : label);
  return (
    <>
      <NavLink
        to={destination.to}
        onClick={onTap}
        ref={tooltip.ref}
        aria-label={showLabel ? undefined : label}
        aria-describedby={tooltip.describedBy}
        {...tooltip.handlers}
        className={({ isActive }) =>
          `${slotBase} rounded-control ${reactive && !showLabel ? "glim-reactive" : ""} glim-hue glim-hue-icon ${isActive ? "bg-accent text-accentContrast glim-active" : "text-carbon-textMuted"}`
        }
        style={
          {
            ...(hueVars(hueIndex) as CSSProperties),
            ...(reactive && !showLabel ? { "--reactive-chars": labelWidth(label) } : {}),
          } as CSSProperties
        }
      >
        {({ isActive }) => (
          <>
            {/* The fill is the slot's, edge to edge; glyph and caption sit on
                the accent fill and both take the ink it was paired with
                (text-accentContrast): glyphs draw fill="currentColor" per
                navGlyphs' contract, so no svg utility is needed. Anchored on
                .glim-coin-tile.glim-active / .glim-hue-icon in index.css;
                filled is how this language says "this one is selected". The
                fill is paint-only: no padding, height, width or gap token
                changed, which is what keeps the narrow-viewport single-line /
                no-overflow asserts and the 44px touch floor true by
                construction. */}
            <span className="flex h-6 w-6 items-center justify-center rounded-control [&_svg]:h-6 [&_svg]:w-6">
              <Icon />
            </span>
            {/* 400 rest, 600 active; the caption's two sanctioned weights. */}
            <span
              className={`max-w-full truncate ${isActive ? "font-semibold" : ""} ${showLabel ? "" : reactive ? "glim-label-reactive" : "sr-only"}`}
            >
              {label}
            </span>
          </>
        )}
      </NavLink>
      {tooltip.bubble}
    </>
  );
}
