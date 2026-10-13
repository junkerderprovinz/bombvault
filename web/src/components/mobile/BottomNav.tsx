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
// The slots come from the navigation registry (lib/navModel.ts), which the
// sidebar renders from as well: Overview, Backups, Restore and Settings, in
// the sidebar's order. The fifth slot is the More trigger, which opens the
// MoreSheet with every other phone entry.
//
// The More trigger is always there. Its sheet always lists the storage
// locations, so there is no empty sheet to hide it for. It announces itself as
// a disclosure that opens a dialog and reads as active while the current page
// belongs to one of the sheet's entries, so someone on such a page still sees
// where they are.
//
// Label axis: the bar has its own axis in the control label engine
// ("bottombar", lib/controls.ts); the same four modes as buttons/sidebar/
// tabs, consumed here the way Sidebar consumes "sidebar": hiding modes gate
// the caption under the glyph while the label survives as the slot's
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
// Overview carries the count of what needs the person, in grey: the bar keeps
// colour for the tab that is current.
//
// While a critical or warning finding is open, a dot in its tone sits on the
// More trigger's glyph: the Anomalies row and its count live in the sheet, and
// without the dot a phone would hide them one tap away. The count itself is
// the sheet's, so the dot stays out of the accessibility tree.
//
// Tap-on-active: tapping the already-active destination scrolls the scroller
// back to the top instead of navigating; and the navigation is actively
// suppressed (preventDefault before react-router's own handler, see
// tapDestination), not merely followed by a scroll, so the tap never stacks a
// duplicate history entry. The scroller is Layout's <main id="bv-main">, so
// the mechanism is passed down from Layout as a prop and nothing here queries
// the DOM for it.
import { useState, type CSSProperties, type MouseEvent } from "react";
import { NavLink, useLocation } from "react-router-dom";
import type { Settings } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { barDestinations, moreDestinations, onDestination, type NavDestination } from "../../lib/navModel";
import { useNavCounts, type NavCount as Count } from "../../lib/navCounts";
import { useNavPins } from "../../lib/navPins";
import { hidesLabel, labelWidth } from "../../lib/controls";
import { hueVars } from "../../lib/appearance";
import { useLabelMode } from "../../lib/useLabelMode";
import { useLogoBursting } from "../../lib/useLogoHold";
import { useTipBubble } from "../../lib/useTipBubble";
import { IconMore } from "../navGlyphs";
import { NavCount } from "../NavCount";
import { MoreSheet } from "./MoreSheet";

export interface BottomNavProps {
  /** Already-loaded settings, owned by Layout; the chrome performs no
   *  fetches of its own; both surfaces derive from the one registry
   *  synchronously (no loading state exists). */
  settings: Settings | null;
  /** Whether a login password exists. Gates the sign-out row inside the
   *  sheet, exactly like the desktop Sidebar footer. */
  authEnabled: boolean;
  /** The page came through the relay; the sheet then says how the phone is
   *  signed in in place of the sign-out row. */
  relayed?: boolean;
  /** Layout's tap-on-active scroll (the scroller is Layout's
   *  <main id="bv-main">); passed on to the sheet's rows too. */
  scrollMainToTop: () => void;
}

// One bar slot, destination or More trigger alike: the 24px glyph box over
// the caption, and the full 56px (h-14) row as the touch target; comfortably
// over the 44px floor. Each slot takes the width its caption needs and keeps
// at least the 44px floor, so the bar spends its width where the words are.
// The press scale reads the motion-engine token, so the app's motion levels
// govern it (--motion-press-scale is 1 at "off": no scale at all there).
const slotBase =
  "glim-bar-slot flex min-w-11 flex-col items-center justify-center gap-1 text-caption motion-safe:active:scale-[var(--motion-press-scale)]";

// The caption is already at the smallest size of the type scale, so when the
// captions together are wider than the bar, a long word goes onto a second
// line, hyphenated where the language allows it. Two lines at this leading
// still fit under the glyph in the 56px row.
const caption = "max-w-full text-center leading-[1.1] hyphens-auto wrap-anywhere";

// barVars places a slot in the bar: its rainbow hue, and its turn and first
// direction in the shake the overview's logo sends through the bar when it
// goes off, the way the rail's rows take it on the desktop.
function barVars(i: number): CSSProperties {
  return { ...hueVars(i), "--rail-i": i, "--rail-dir": i % 2 ? -1 : 1 } as CSSProperties;
}

export function BottomNav({ settings, authEnabled, relayed = false, scrollMainToTop }: BottomNavProps) {
  const { t } = useT();
  const location = useLocation();
  // The sheet belongs to the page it was opened on, so any way of leaving
  // that page closes it.
  const [moreOpenOn, setMoreOpenOn] = useState<string | null>(null);
  const moreOpen = moreOpenOn === location.pathname;
  const pins = useNavPins(settings);
  const slots = barDestinations(settings, pins);
  const counts = useNavCounts();
  // The bar's own label axis (see the header): hiding modes gate the caption,
  // reactive pairs with index.css's coarse-pointer at-rest reveal.
  const labelMode = useLabelMode("bottombar");
  const showLabel = !hidesLabel(labelMode);
  const reactive = labelMode === "reactive";
  // The trigger is filled for the pages its sheet leads to, including the
  // pages below them, which is what NavLink does for the slots.
  const moreActive = moreDestinations(settings, pins).some((d) => onDestination(location.pathname, d));
  // The More trigger's bubble: worded only in hiding modes, the same rule
  // the sidebar's rows follow (reactive mode brings the word back on hover).
  const moreTip = useTipBubble(showLabel || reactive ? undefined : t("nav.more"));
  const anomalyDot = (settings?.anomalyEnabled ?? false) && counts.anomalies.value > 0;
  const quaking = useLogoBursting();

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
          not a line: no border or ring on the card. Below 400px the card
          keeps half the gutter to leave the captions more room. */}
      <div
        className={`mt-2 ml-[max(1rem,var(--safe-area-left))] mr-[max(1rem,var(--safe-area-right))] max-[399px]:ml-[max(0.5rem,var(--safe-area-left))] max-[399px]:mr-[max(0.5rem,var(--safe-area-right))] mb-2 rounded-card bg-carbon-sidebar ${quaking ? "glim-rail-quake" : ""}`}
      >
        {/* h-14 static height: svh semantics by construction; a fixed-height
            row never resizes when the dynamic viewport does (browser chrome,
            keyboard), which is the property that keeps the bar visible: static
            chrome sizes against svh semantics, in every label mode. The
            slots are auto grid columns: each grows towards its caption's
            one-line width from an equal share of the room, so when the room
            runs short only the longest caption goes onto a second line. */}
        <div className="grid h-14 grid-flow-col">
          {slots.map((d, i) => (
            <BarSlot
              key={d.to}
              destination={d}
              hueIndex={i}
              count={d.count && counts[d.count]}
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
            data-new-way="more"
            onClick={() => setMoreOpenOn(location.pathname)}
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
                ...barVars(slots.length),
                ...(reactive && !showLabel ? { "--reactive-chars": labelWidth(t("nav.more")) } : {}),
              } as CSSProperties
            }
          >
            {/* The glyph box is the 24px slot-icon box (the 16px generated
                glyph scales up to it, the same scaling the rail applies). */}
            <span className="relative flex h-6 w-6 items-center justify-center rounded-control [&_svg]:h-6 [&_svg]:w-6">
              <IconMore />
              {anomalyDot && (
                <span
                  data-testid="more-anomaly-dot"
                  aria-hidden="true"
                  className={`absolute -end-0.5 -top-0.5 h-2 w-2 rounded-full ${counts.anomalies.tone === "fail" ? "bg-statusFailSolid" : "bg-statusWarnSolid"}`}
                />
              )}
            </span>
            <span
              className={showLabel ? caption : reactive ? "max-w-full truncate glim-label-reactive" : "sr-only"}
            >
              {t("nav.more")}
            </span>
          </button>
          {moreTip.bubble}
        </div>
      </div>
      <MoreSheet
        open={moreOpen}
        onClose={() => setMoreOpenOn(null)}
        settings={settings}
        authEnabled={authEnabled}
        relayed={relayed}
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
  count,
  labelMode,
  onTap,
}: {
  destination: NavDestination;
  hueIndex: number;
  count?: Count;
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
  const counted = count && count.value > 0 ? count : undefined;
  // An aria-label replaces what the slot contains, the figure included, so a
  // slot without a caption says both.
  const hiddenName = counted ? `${label}, ${counted.label}` : label;
  const here = onDestination(useLocation().pathname, destination);
  return (
    <>
      <NavLink
        to={destination.to}
        onClick={onTap}
        ref={tooltip.ref}
        aria-label={showLabel ? undefined : hiddenName}
        aria-describedby={tooltip.describedBy}
        {...tooltip.handlers}
        className={({ isActive }) =>
          `${slotBase} rounded-control ${reactive && !showLabel ? "glim-reactive" : ""} glim-hue glim-hue-icon ${isActive || here ? "bg-accent text-accentContrast glim-active" : "text-carbon-textMuted"}`
        }
        style={
          {
            ...barVars(hueIndex),
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
                changed, which is what keeps the narrow-viewport no-overflow
                asserts and the 44px touch floor true by construction. */}
            <span className="relative flex h-6 w-6 items-center justify-center rounded-control [&_svg]:h-6 [&_svg]:w-6">
              <Icon />
              {counted && (
                <NavCount count={counted} quiet size="corner" className="absolute -end-2.5 -top-1" />
              )}
            </span>
            {/* 400 rest, 600 active; the caption's two sanctioned weights. */}
            <span
              className={`${isActive || here ? "font-semibold" : ""} ${showLabel ? caption : reactive ? "max-w-full truncate glim-label-reactive" : "sr-only"}`}
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
