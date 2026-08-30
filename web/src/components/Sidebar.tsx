import { NavLink, useNavigate } from "react-router-dom";
import { useState, useRef, useEffect, type CSSProperties } from "react";
import { type Settings } from "../lib/api";
import { useT } from "../lib/i18n";
import { useAdvanced } from "../lib/advanced";
import { hueVars, rainbowAt } from "../lib/appearance";
import { useRainbow } from "../lib/useRainbow";

import {
  IconContainers,
  IconVM,
  IconFiles,
  IconReceiver,
  IconFleet,
  IconDashboard,
  IconRecovery,
  IconFlash,
  IconConfig,
  IconViewSimple,
  IconViewAdvanced,
  IconGear,
} from "./navGlyphs";
import { useLabelMode } from "../lib/useLabelMode";
import { hidesLabel } from "../lib/controls";
import { useTipBubble } from "../lib/useTipBubble";

// Navigation and domain glyphs now come from Streamline's free Core Solid set
// (#178, [202]) rather than being drawn here, so the whole interface reads as
// one icon family. They are re-exported under their existing names, because
// dozens of files import IconTrash and friends FROM this file and there is no
// value in touching all of them for a change none of them can see.
export {
  IconContainers,
  IconVM,
  IconFiles,
  IconReceiver,
  IconFleet,
  IconFolder,
  IconCloud,
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
}

interface NavItem {
  to: string;
  label: string;
  icon: React.ReactNode;
  /** This item's own stable rainbow position — see this file's own header
   *  comment (the rainbow-reversal round) and NavItem's own doc comment
   *  below for how the CALLER resolves this via `nextHue()`. */
  hueIndex: number;
}

// Rainbow-hued nav rail (GlimStone follow-up round, REVERSING the decision
// below — jdp, live-review: "Die ganzen Tabs in der Sidebar sind wieder
// nicht im Regenbogenmodus bzw in der Farbengine."). jdp is this design
// exception's own owner (five escalations deep, this app's own standing
// convention for a self-authored aesthetic call) and has now explicitly
// overridden it: every visible destination below — including Settings in
// the footer group — gets its own stable rainbow-hue position, via the
// EXACT `hueVars(rainbowAt(index))` / `.glim-hue`/`.glim-hue-icon`
// mechanism the Settings tab strip's own Selector segments already use
// (Selector.tsx) — not a second, bespoke nav-only mechanism.
//
// The two objections the original decision raised (preserved below,
// unedited) are answered, not ignored:
//
//  1. "Colour buys nothing here" — overridden outright by jdp's explicit
//     ask; colour is no longer optional-because-unneeded, it's requested.
//  2. "Colour could not be stable here" — true only against a STATIC,
//     hand-written literal index (0 for Dashboard, 1 for Recovery, ...),
//     which really would drift the moment a domain toggle hid one of the
//     conditional tabs (VMs/Flash/Files/Config/Receiver/Fleet). The fix is
//     the SAME nextHue()-during-actual-render approach Dashboard.tsx's own
//     `visibleBlocks`/`hueSeq` counter and Settings.tsx's own per-tab
//     `hueSeq`/`nextHue()` Card sequence already use for THEIR OWN
//     conditionally-rendered sets (see either file's own `hueSeq`/`nextHue`
//     comment): a plain `let hueSeq = 0`, reset fresh on every render,
//     incremented once per NavItem call made DIRECTLY in the order this
//     file's own JSX below actually evaluates it. A
//     `{vmsEnabled && <NavItem hueIndex={nextHue()} .../>}` gate
//     SHORT-CIRCUITS before ever calling `nextHue()` when the domain is
//     off, so a hidden tab never burns a slot — every VISIBLE item's
//     position (and with it its colour) is exactly its own rank among the
//     tabs actually on screen that render, immune to which OTHER
//     conditional tabs happen to be hidden or shown around it. The wrap
//     past RAINBOW's 8 colours (lib/appearance.ts) at full config (10
//     entries, Settings included) is the same accepted tradeoff
//     Containers.tsx/VMs.tsx's own row lists already make past their own
//     8th row — see the ORIGINAL comment below, which raised the wrap on
//     its own but never called it disqualifying by itself either.
//
// KnightLoader's Sidebar.tsx (hued nav off 6 hard-coded destinations) is now
// a real precedent this file actually follows, not one it distinguishes
// itself from: the "off a FIXED destination count" gap the original comment
// drew between the two apps closes the moment this file's own hue position
// is read off the VISIBLE list's own render-time rank instead of a
// hard-coded literal.
//
// ORIGINAL DECISION (GlimStone form-engine Phase 2, Task 2 — decided in the
// spec-compliance review of the first attempt, which had wired every
// NavItem to a palette position; reaffirmed once more in a later live-review
// round below before THIS round reversed it), preserved verbatim for
// history:
//
//  1. Colour buys nothing here. Every destination already carries a
//     permanent, always-visible identity in its label + icon, so rainbow's
//     actual job ("tell apart several members of a set that otherwise look
//     alike") has nothing to do on a rail where no two entries look alike in
//     the first place.
//  2. Colour could not be stable here even if it were wanted. The visible set
//     is user-configured — 4 to 10 entries, depending on which domains are on
//     (vmsEnabled etc.) — so a destination's position, and with it its
//     colour, moves whenever a domain is toggled in Settings; at full config
//     the 10 entries also wrap RAINBOW's 8 colours (lib/appearance.ts) and
//     two of them repeat. A nav rail is the one surface where a colour would
//     have to be LEARNED to be worth anything, and a learned colour that
//     moves is worse than no colour at all.
//
// The wrap on its own is NOT the disqualifier: a container/VM list wraps the
// same way past its eighth row and keeps rainbow on purpose (see
// Containers.tsx/VMs.tsx) — there the colour only has to separate rows that
// are on screen together, and a repeat lands a full palette apart, never
// beside its twin. KnightLoader's Sidebar.tsx does hue its nav, but off 6
// hard-coded destinations, so neither point above applies to it; that
// precedent doesn't transfer to a rail whose length changes.
//
// STILL not rainbowed (GlimStone follow-up round, live-review — the
// tab-strip 3-state colour rule, jdp: "...Hauptabs in der Sidebar..."): this
// decision and its two counts above stood unchanged for that round too — see
// `navInactive`'s own comment below for the flat-accent, single-token
// mechanism that round built, since REPLACED by this round's
// `.glim-hue`/`.glim-hue-icon` (still true of SidebarControls' own
// Simple/Advanced toggle, which stays on the old flat-accent mechanism — see
// `navInactive`'s own comment below for why that one control is different).

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

// `transition` (not just `transition-colors`) so the transform-based hover/press
// micro-interactions below animate too; all transforms are motion-safe-gated so
// reduced-motion users get colour-only feedback (Item 7a/7d).
const navBase =
  "bv-nav-row flex items-center gap-3 px-3.5 py-2.5 rounded-control text-[15px] font-medium transition duration-150 select-none motion-safe:active:scale-[.97]";
const navActive =
  "bg-accent text-accentContrast";
// `rtl:-translate-x-0.5!` — physical `translate-x`, same trap as the Toggle
// thumb (RTL sweep, form-engine Phase 2 Task 6 follow-up fix): a positive
// nudge always moves right on screen, which reaches toward the main content
// area only when the sidebar sits on the LEFT (LTR). Negating it under `rtl:`
// keeps the nudge pointed at the content instead of away from it once the
// sidebar sits on the right. `!` beats the base rule regardless of Tailwind's
// generated declaration order, same reasoning as the Toggle thumb fix.
//
// `bv-nav-idle` DROPPED from this shared string (GlimStone follow-up round,
// rainbow reversal — see this file's own header comment): NavItem below no
// longer reads the bespoke flat-accent `.bv-nav-idle svg` CSS rule at all —
// it now carries `.glim-hue`/`.glim-hue-icon` instead, the exact classes any
// other hue-enabled Selector segment carries (Selector.tsx), so it picks up
// index.css's EXISTING generic `.glim-hue-icon` rule (no new CSS needed for
// NavItem itself) — the identical idle/hover/selected 3-state machine this
// rail always had, just reading this item's own `--item-hue` instead of the
// single flat `--accent`.
//   `bv-nav-idle` itself is NOT deleted from index.css — SidebarControls'
// own Simple/Advanced view toggle below is a genuine set-of-ONE, not a list
// of same-type destinations competing for a stable rainbow position (the
// colour engine's own "anything that is the only one of its kind on the
// page keeps the single accent" exclusion, glimstone/docs/design-language.md
// "Rainbow — the accent, plural") — it keeps the marker directly on its own
// call site below instead of inheriting it from this shared string.
const navInactive =
  "text-(--sidebar-text) hover:bg-carbon-hover hover:text-carbon-text motion-safe:hover:translate-x-0.5 motion-safe:hover:rtl:-translate-x-0.5!";

// hueIndex (GlimStone follow-up round, rainbow reversal — see this file's
// own header comment): resolved by the CALLER's `nextHue()` counter at the
// exact synchronous point each NavItem below is actually rendered — never
// computed inside this component itself, the same "caller resolves the
// position, callee just paints it" split Settings.tsx's own `hueIndex` Card
// prop and Dashboard.tsx's own per-block `hueIndex` props already use.
// `glim-hue`/`glim-hue-icon` ride unconditionally on every NavItem
// regardless of active state (exactly Selector.tsx's own SelectorTab
// convention, `hue ? "glim-hue glim-hue-icon" : ""` applied to every
// segment) — `glim-active` is the one extra marker that differs between the
// two branches, added only alongside `navActive` on the active route, so
// index.css's existing `.glim-hue-icon:not(.glim-active)` guard correctly
// excludes the filled/selected item: it already gets its icon colour for
// free via `text-accentContrast` currentColor inheritance off navActive's
// own `bg-accent text-accentContrast`, not from `.glim-hue-icon`'s own
// colour declaration, so painting this item's hue on top of a badge already
// filled with that same hue never happens.
function NavItem({ to, label, icon, hueIndex }: NavItem) {
  // #178: the sidebar has its own axis, deliberately separate from buttons and
  // tabs, because reducing THIS rail to glyphs is a layout decision rather
  // than a density preference.
  const labelMode = useLabelMode("sidebar");
  const showLabel = !hidesLabel(labelMode);
  const showIcon = labelMode !== "text";
  // Reactive: the glyph sits centred like in glyph mode, and hovering the row
  // slides its name back in. Nothing reflows, because the rail is a fixed
  // 224px and the row was always that wide.
  const reactive = labelMode === "reactive";
  // A rail row with no visible text needs to say what it is on hover, in the
  // app's own bubble — the same ruling that took the native `title` balloon
  // off Button. Eleven unnamed pictures is a worse glyph mode than a slightly
  // denser one. Nothing is passed while the label is visible: the row already
  // says what it is, and a tooltip repeating it is noise.
  // No bubble in reactive mode: hovering already brings the word back, and a
  // tooltip saying the same thing would land on top of the animation doing it.
  const tooltip = useTipBubble(showLabel || reactive ? undefined : label);
  return (
    <>
      <NavLink
        to={to}
        ref={tooltip.ref}
        aria-describedby={tooltip.describedBy}
        {...tooltip.handlers}
        // `justify-center` — jdp's ruling on the rail-width question: the rail
        // KEEPS its full 224px in glyph mode and the glyphs centre in it,
        // rather than the rail narrowing to fit them. (The narrow-rail
        // alternative was tried live at 6rem and needs a second, smaller logo
        // to go with it — a separate job, not a width value.) Without this the
        // glyphs sat hard left with 178px of empty rail beside each one.
        className={({ isActive }) =>
          `${navBase} ${showLabel ? "" : "justify-center"}${reactive ? " bv-reactive" : ""} glim-hue glim-hue-icon ${isActive ? `${navActive} glim-active` : navInactive}`
        }
        style={hueVars(rainbowAt(hueIndex)) as CSSProperties}
      >
        {showIcon && icon}
        {/* Never removed, only hidden: a nav row whose text is gone entirely
            has no accessible name, and this rail is how the app is navigated.
            `sr-only` is `position: absolute`, so the hidden span is not a flex
            item and `gap-3` does not leave a phantom gap beside the centred
            glyph. */}
        <span className={showLabel ? undefined : reactive ? "bv-label-reactive" : "sr-only"}>{label}</span>
      </NavLink>
      {tooltip.bubble}
    </>
  );
}

// ---------------------------------------------------------------------------
// SidebarControls — Simple/Advanced view toggle in the sidebar footer. Both
// the language switcher (flag + name, dropdown opening upward — GlimStone
// follow-up pass, live-review point 9) and the dark/light theme toggle
// (GlimStone follow-up pass, later live-review round) that used to live here
// have MOVED into their own Cards in Settings' General tab — jdp asked for
// each as a literal move ("verschieb den Sprachschalter... auch als eigene
// card ins allgemein setting", then the same for the theme toggle), never a
// duplicate — so this footer no longer renders either of them. See
// Settings.tsx's LanguageCard/ThemeCard for where they live now, and
// Sidebar.language.dom.test.tsx for the regression guard against either one
// reappearing here.
// ---------------------------------------------------------------------------

// Exported: Settings.tsx's own Language Card (GlimStone follow-up pass,
// live-review point 9) reuses this exact flag glyph for its relocated
// language picker — same "small shared piece imported straight from
// Sidebar.tsx" precedent this file's own IconContainers/IconVM/IconFiles/
// IconReceiver/IconFleet already established for Containers.tsx/VMs.tsx/
// Files.tsx/Receiver.tsx/Fleet.tsx.
export function Flag({ code }: { code: string }) {
  return (
    <span
      className={`fi fi-${code}`}
      style={{ width: "1.25em", height: "1em", display: "inline-block", flexShrink: 0 }}
    />
  );
}

function SidebarControls() {
  const { t } = useT();
  const { advanced, setAdvanced } = useAdvanced();
  // #178: this row lives IN the rail, so it follows the rail's own axis. It
  // was the one row that did not, which showed up immediately in glyph mode:
  // five icons and one full-width label, and the rail could not narrow.
  const labelMode = useLabelMode("sidebar");
  const showLabel = !hidesLabel(labelMode);
  const reactive = labelMode === "reactive";
  const view = advanced ? t("mode.advancedView") : t("mode.simpleView");
  // Same rule as NavItem's: the row explains itself in the real bubble, and
  // only while its text is hidden. It used to carry the name in a native
  // `title` in EVERY mode — the balloon on a row whose words are already
  // printed right there, which is the pattern this round removed everywhere
  // else.
  const tooltip = useTipBubble(showLabel || reactive ? undefined : view);

  return (
    <div className="flex flex-col gap-1">
      {/* Simple / Advanced view — a single-click toggle (same height, hover,
          press feedback as every other nav-rail row). The label shows the
          CURRENT view; a click flips it. Replaces the old segmented switch +
          hint (Item 4). The dark/light theme row that used to sit above this
          one moved into Settings' General tab (ThemeCard) — see this
          function's own header comment. */}
      <button
        ref={tooltip.ref}
        onClick={() => setAdvanced(!advanced)}
        aria-pressed={advanced}
        aria-describedby={tooltip.describedBy}
        {...tooltip.handlers}
        // `bv-nav-idle` stated explicitly here, not inherited from
        // `navInactive` any more (GlimStone follow-up round, rainbow
        // reversal — see `navInactive`'s own comment above): this toggle is
        // a genuine set-of-one, not a member of the now-hued nav-destination
        // list, so it keeps the old flat-accent hover-reveal marker on its
        // own call site.
        //
        // `justify-center` in glyph mode for the same reason as NavItem's —
        // this row sits in the same column and has to centre with it.
        className={`${navBase} ${showLabel ? "" : "justify-center"}${reactive ? " bv-reactive" : ""} bv-nav-idle ${navInactive} w-full`}
      >
        {/* One glyph per state, not one for both (jdp): the row shows the view
            it is CURRENTLY in, so a single symbol left the two states looking
            identical — in glyph mode, where the words are gone, that made the
            row unreadable. Sparse layout for simple, dense one for advanced. */}
        {advanced ? <IconViewAdvanced /> : <IconViewSimple />}
        {/* Hidden, never removed: the toggle keeps its accessible name. */}
        <span className={showLabel ? undefined : reactive ? "bv-label-reactive" : "sr-only"}>{view}</span>
      </button>
      {tooltip.bubble}
    </div>
  );
}

export function Sidebar({ settings }: SidebarProps) {
  const { t } = useT();
  const navigate = useNavigate();
  const vmsEnabled = settings?.vmsEnabled ?? false;
  const flashEnabled = settings?.flashEnabled ?? false;
  const configEnabled = settings?.configEnabled ?? false;
  const filesEnabled = settings?.filesEnabled ?? false;
  const receiverEnabled = settings?.receiverEnabled ?? false;
  const fleetEnabled = settings?.fleetEnabled ?? false;

  // #178: the logo row lives IN the rail, so it follows the rail's own axis
  // like every other row does. jdp's call after seeing the centred glyph
  // column live: the mark centres and the wordmark goes, rather than a
  // left-aligned 64px mark and a word sitting on top of eleven centred
  // glyphs. Measured before the change: mark centre x=48 against a glyph
  // column at x=112, so the two were visibly off the same axis.
  //
  // Nothing is lost by hiding the word: this button already carries
  // `aria-label={t("nav.dashboard")}`, so its accessible name never depended
  // on the wordmark being painted.
  const railMode = useLabelMode("sidebar");
  const railLabels = !hidesLabel(railMode);
  // The wordmark comes back on hover too, so the header behaves like the
  // rows beneath it rather than being the one thing that stays mute.
  const railReactive = railMode === "reactive";

  // Subscribed, not read (GlimStone follow-up round, rainbow reversal — see
  // this file's own header comment): registers this component for a
  // re-render on any rainbow change (on/off/reactive/rotate/palette edit),
  // the same lib/useRainbow.ts contract every other hue-enabled consumer
  // (Selector.tsx, ContainerRow/VMRow/FileSetRow) already subscribes
  // through — called once here, not once per NavItem, matching that hook's
  // own "once per list, not once per row" doc.
  useRainbow();

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
        className={`bv-logo-btn flex items-center ${railLabels ? "gap-2.5 px-4 text-start" : "justify-center px-0"}${railReactive ? " bv-reactive" : ""} py-5 w-full cursor-pointer select-none hover:opacity-90 transition-opacity`}
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
        {/* Removed from the DOM rather than hidden with `sr-only`, unlike every
            other label in this rail: this one is decoration, not a name. The
            button's own `aria-label` above is its accessible name in every
            mode, so an `sr-only` copy would only make a screen reader read
            "Dashboard BombVault".
            Reactive keeps it in the DOM so it can slide back on hover, and
            `aria-hidden` is what stops that from reintroducing exactly the
            double-announcement the removal above avoids. */}
        {(railLabels || railReactive) && (
          <span
            aria-hidden={railReactive || undefined}
            className={`text-carbon-text font-bold text-xl tracking-tight leading-none whitespace-nowrap${
              railReactive ? " bv-label-reactive" : ""
            }`}
          >
            BombVault
          </span>
        )}
      </button>

      {/* hueSeq/nextHue (GlimStone follow-up round, rainbow reversal — see
          this file's own header comment): the SAME page-wide running-counter
          pattern as Dashboard.tsx's own `visibleBlocks`/`hueSeq` and
          Settings.tsx's own per-tab `hueSeq`/`nextHue()` — a plain `let`,
          reset to 0 fresh on every render (nothing here needs to survive
          between renders), incremented once per NavItem call made DIRECTLY
          in the order the JSX below actually evaluates it, spanning BOTH
          groups (the main <nav> list AND the Settings NavItem in the footer
          group further down) — jdp's own ask was "die ganzen Tabs in der
          Sidebar," ALL of them, and Settings is the same NavItem component
          rendering the same kind of destination, just placed in its own
          flex group below the Simple/Advanced spacer for LAYOUT reasons
          only, not a different species of nav element. A
          `{flag && <NavItem hueIndex={nextHue()} .../>}` gate
          short-circuits BEFORE evaluating `nextHue()` when the domain is
          off, so a hidden conditional tab never burns a slot — hiding or
          showing VMs/Flash/Files/Config/Receiver/Fleet only ever
          shifts the POSITIONS of tabs after it in this list, never the
          identity-to-position mapping of the ones before it, and never
          leaves a gap in the sequence. */}
      {(() => {
        let hueSeq = 0;
        const nextHue = () => hueSeq++;
        return (
          <>
            {/* Navigation */}
            <nav className="flex flex-col gap-1 p-3 flex-1">
              <NavItem
                to="/dashboard"
                label={t("nav.dashboard")}
                icon={<IconDashboard />}
                hueIndex={nextHue()}
              />
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
              {/* VMs / Flash / Files tabs appear only once their domain is enabled. */}
              {vmsEnabled && (
                <NavItem to="/vms" label={t("nav.vms")} icon={<IconVM />} hueIndex={nextHue()} />
              )}
              {flashEnabled && (
                <NavItem to="/flash" label={t("nav.flash")} icon={<IconFlash />} hueIndex={nextHue()} />
              )}
              {filesEnabled && (
                <NavItem to="/files" label={t("nav.files")} icon={<IconFiles />} hueIndex={nextHue()} />
              )}
              {/* Config self-backup tab appears only once its domain is enabled. */}
              {configEnabled && (
                <NavItem to="/config" label={t("nav.config")} icon={<IconConfig />} hueIndex={nextHue()} />
              )}
              {/* Receiver dashboard appears only once its domain is enabled. */}
              {receiverEnabled && (
                <NavItem to="/receiver" label={t("nav.receiver")} icon={<IconReceiver />} hueIndex={nextHue()} />
              )}
              {/* Fleet view appears only once its domain is enabled. */}
              {fleetEnabled && (
                <NavItem to="/fleet" label={t("nav.fleet")} icon={<IconFleet />} hueIndex={nextHue()} />
              )}
            </nav>

            {/* Bottom group: the Simple/Advanced view toggle (SidebarControls,
                its own set-of-one, never hued — see its own call site's
                comment), then Settings. Language and theme both moved out —
                see SidebarControls' own header comment. Settings continues
                THIS SAME nextHue() sequence rather than starting its own —
                see this block's own opening comment. */}
            <div className="flex flex-col gap-1 p-3">
              <SidebarControls />
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
