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
} from "./navGlyphs";

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



function IconFlash() {
  return (
    <svg width="22" height="22" viewBox="0 0 20 20" fill="none" className="shrink-0">
      <path d="M11 2L4 11h6l-1 7 7-9h-6l1-7z" fill="currentColor" />
    </svg>
  );
}


// Padlock glyph for the Config self-backup tab — REDRAWN (GlimStone
// follow-up round, jdp's live review, emphatic: "Selbstbackup-Tab: bitte ein
// anderes Glyph verwenden, das sieht nicht gut aus"). The old sliders/tuner
// glyph (two tracks + two knobs) is what actually looked wrong, and the
// live screenshot this round pulled from the real deployed container showed
// exactly why: its "knobs" were `fill="var(--sidebar-surface, transparent)"`
// cutout circles, and `--sidebar-surface` is never actually DEFINED anywhere
// in index.css — the fallback `transparent` is all it ever resolved to, in
// EVERY theme and EVERY nav state (idle sidebar ground, hovered, and the
// solid `bg-accent` selected background alike). A cutout that always shows
// literally nothing is, by construction, invisible against any background
// whatsoever — confirmed live: the two knobs never rendered at all, in
// either state, leaving only the two flat bars, which read as a plain "="
// sign, not "sliders." This wasn't a tuning problem the same shape could be
// nudged out of; the whole "knob" concept needed dropping, not adjusting —
// hence a full redesign rather than a path-data patch, per this round's own
// instructions.
//
// New concept: "Selbst-Backup" is BombVault backing up its OWN settings/
// config, not a domain it protects — a padlock ("this instance's own state,
// kept secure") reads unambiguously at a glance and needs no internal detail
// at all to be recognisable, sidestepping the whole "small internal feature
// disappears at 16px" failure class above. Deliberately NOT a shield: this
// app's own Settings-page tab strip already has a shield+checkmark glyph
// for "Integrität" (IconTabIntegrity, Settings.tsx) — reusing that
// silhouette for a DIFFERENT concept elsewhere in the same app would be a
// fresh legibility problem of exactly the kind this round exists to fix, not
// a solution to it. Deliberately NOT a gear-with-arrow either, per this
// glyph's own long-standing "distinct from the Settings cog below" rule —
// even a small secondary gear risks reading as "a second settings icon" two
// rows away from the real one.
//
// Two plain filled shapes, no stroke, no cutout, no colour-on-colour overlap
// (this file's own IconCopy comment names the exact two failure modes this
// avoids): a ring-band shackle (the SAME two-concentric-arcs "thin filled
// shape" technique IconRecovery below already uses for its own circular
// arrow, just closed into a full loop instead of an open sweep) sitting
// directly above a solid rounded-rect body — the shackle's own two straight
// legs run flush into the body's top edge with zero gap, so the two pieces
// read as one continuous padlock silhouette (exactly how a plain-colour
// emoji/icon padlock always draws this — the outline alone carries the
// whole shape, no internal keyhole needed or attempted).
function IconConfig() {
  return (
    <svg width="22" height="22" viewBox="0 0 20 20" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M6.6 9V6.4a3.4 3.4 0 1 1 6.8 0V9h-1.8V6.4a1.6 1.6 0 1 0-3.2 0V9Z" />
      <rect x="4.2" y="9" width="11.6" height="8.6" rx="1.8" />
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
// FILLED (design-language.md "Icon glyphs"): the old glyph was two open
// strokes (a big arc + an L-shaped arrowhead) — a "line glyph" per rule 219,
// which needs real geometry, not a thicker stroke. Redrawn as one filled
// ring segment (rule 220's "thin filled shape" technique — two concentric
// arcs, outer r=7.4 / inner r=5.6, closed into a band) sweeping 270°
// clockwise from the right (3 o'clock) up to the top (12 o'clock), plus a
// solid filled triangular arrowhead capping the top end and pointing back
// along the sweep — same "mostly a circle, gap + arrowhead at one end"
// reading as the old stroke version, just filled. Same construction reused
// at the smaller 16×16 icon-only-badge scale for this file's own IconRestore
// below — the two read as the same visual family.
//   NOT shared any more with Settings.tsx's "reset to default" glyph
// (IconResetArrow, formerly IconResetSwirl): that one deliberately diverged
// to a bolder ring/arrowhead ratio for its own harder legibility case (16px,
// beside 8 competing colour swatches) — see IconResetArrow's own header
// comment in Settings.tsx. This icon and IconRestore below are unaffected
// and keep their original, already-correct-for-their-own-context proportions.
function IconRecovery() {
  return (
    <svg width="22" height="22" viewBox="0 0 20 20" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M17.4 10A7.4 7.4 0 1 1 10 2.6L10 4.4A5.6 5.6 0 1 0 15.6 10Z" />
      <path d="M10 1.6 5.4 3.5 10 5.4Z" />
    </svg>
  );
}








// IconSave (a second, plain save-disk glyph that used to sit beside
// IconBackupNow's then-arrowed disk) REMOVED — its only callers were
// Containers.tsx's four disclosure-panel
// editors (FoldersEditor/StopContainersEditor/ExcludesEditor/
// HooksEditor), each of which used it purely for an explicit "Speichern"
// icon-badge button. Live-save conversion (jdp, live review: "Brauchen wir
// die Speichern-Buttons in den Aufklappcards überhaupt? Es soll doch immer
// live speichern.") removed every one of those buttons — each editor now
// auto-saves instead (see HooksEditor's own header comment in Containers.tsx
// for the full writeup) — leaving this glyph with zero remaining call sites,
// so it goes too rather than lingering as dead code. (It was also the
// specific glyph jdp flagged as illegible at render size — moot now that
// nothing renders it, but if a genuine "explicit save/commit" button ever
// returns to this app, reuse IconBackupNow above rather than reviving this
// one — that glyph has since been redrawn to the proportions that actually
// survive 16px, which this one never had.)


// VM backup-method pair (jdp, live review: "die Methode für den VM-Backup
// (Live und graceful) bitte in quadratische Badges mit Glyph umformen") — the
// two glyphs the VMs page's method Selector paints as icon-only segments, at
// this file's own 16×16 icon-only-badge scale with the same solid,
// `currentColor`-only, no-`stroke` construction as everything else here
// (design-language.md's icon-glyph rule).
//
// They are a deliberate PAIR and have to stay apart at 16px, because they sit
// side by side in one 2-segment Selector where only the ACTIVE segment carries
// a fill — once colour is spent on the badge background rather than the glyph
// (the icon-only-badge rule), the silhouette is the only thing left telling
// the two states apart. Hence one round shape against one angular one, rather
// than two members of the same family: a "play" triangle beside a power ring
// would read at this size as two variants of one control, not as two
// different methods.










// Stacked-layers glyph for the Simple/Advanced view toggle — "more layers = more
// controls". Deliberately distinct from IconConfig (sliders) and IconSettings (cog).
// FILLED (design-language.md "Icon glyphs"): the top diamond was ALREADY a
// closed shape under its old stroke, so per rule 218 it flips directly to
// `fill="currentColor"` with the same path data. The two lower "V" folds were
// open line glyphs (rule 219) — each redrawn as a thin filled ribbon (two
// parallel V-paths closed into one band, rule 220's thin-filled-shape
// technique) with a small gap left above it so the three layers still read
// as separate, not fused into one blob.
function IconLayers() {
  return (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M12 2 2 7l10 5 10-5-10-5Z" />
      <path d="M2 12.6 12 16.6 22 12.6 22 13.9 12 17.9 2 13.9Z" />
      <path d="M2 18.5 12 22.5 22 18.5 22 19.8 12 23.8 2 19.8Z" />
    </svg>
  );
}

// `transition` (not just `transition-colors`) so the transform-based hover/press
// micro-interactions below animate too; all transforms are motion-safe-gated so
// reduced-motion users get colour-only feedback (Item 7a/7d).
const navBase =
  "flex items-center gap-3 px-3.5 py-2.5 rounded-control text-[15px] font-medium transition duration-150 select-none motion-safe:active:scale-[.97]";
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
  return (
    <NavLink
      to={to}
      className={({ isActive }) =>
        `${navBase} glim-hue glim-hue-icon ${isActive ? `${navActive} glim-active` : navInactive}`
      }
      style={hueVars(rainbowAt(hueIndex)) as CSSProperties}
    >
      {icon}
      <span>{label}</span>
    </NavLink>
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

  return (
    <div className="flex flex-col gap-1">
      {/* Simple / Advanced view — a single-click toggle (same height, hover,
          press feedback as every other nav-rail row). The label shows the
          CURRENT view; a click flips it. Replaces the old segmented switch +
          hint (Item 4). The dark/light theme row that used to sit above this
          one moved into Settings' General tab (ThemeCard) — see this
          function's own header comment. */}
      <button
        onClick={() => setAdvanced(!advanced)}
        title={advanced ? t("mode.advancedView") : t("mode.simpleView")}
        aria-pressed={advanced}
        // `bv-nav-idle` stated explicitly here, not inherited from
        // `navInactive` any more (GlimStone follow-up round, rainbow
        // reversal — see `navInactive`'s own comment above): this toggle is
        // a genuine set-of-one, not a member of the now-hued nav-destination
        // list, so it keeps the old flat-accent hover-reveal marker on its
        // own call site.
        className={`${navBase} bv-nav-idle ${navInactive} w-full`}
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
  const filesEnabled = settings?.filesEnabled ?? false;
  const receiverEnabled = settings?.receiverEnabled ?? false;
  const fleetEnabled = settings?.fleetEnabled ?? false;

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
        className="bv-logo-btn flex items-center gap-2.5 px-4 py-5 w-full text-start cursor-pointer select-none hover:opacity-90 transition-opacity"
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
        <span className="text-carbon-text font-bold text-xl tracking-tight leading-none whitespace-nowrap">
          BombVault
        </span>
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
                icon={<IconSettings />}
                hueIndex={nextHue()}
              />
            </div>
          </>
        );
      })()}
    </aside>
  );
}
