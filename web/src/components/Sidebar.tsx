import { NavLink, useNavigate } from "react-router-dom";
import { useState, useRef, useEffect, type CSSProperties } from "react";
import { type Settings } from "../lib/api";
import { useT } from "../lib/i18n";
import { useAdvanced } from "../lib/advanced";
import { hueVars, rainbowAt } from "../lib/appearance";
import { useRainbow } from "../lib/useRainbow";

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

// Docker whale mark — Simple Icons path, scaled to 20×20 viewport
export function IconContainers() {
  return (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M13.983 11.078h2.119a.186.186 0 0 0 .186-.185V9.006a.186.186 0 0 0-.186-.186h-2.119a.185.185 0 0 0-.185.185v1.888c0 .102.083.185.185.185m-2.954-5.43h2.118a.186.186 0 0 0 .186-.186V3.574a.186.186 0 0 0-.186-.185h-2.118a.185.185 0 0 0-.185.185v1.888c0 .103.082.185.185.185m0 2.716h2.118a.187.187 0 0 0 .186-.186V6.29a.186.186 0 0 0-.186-.185h-2.118a.185.185 0 0 0-.185.185v1.887c0 .102.082.185.185.185m-2.93 0h2.12a.186.186 0 0 0 .184-.186V6.29a.185.185 0 0 0-.185-.185H8.1a.185.185 0 0 0-.185.185v1.887c0 .102.083.185.185.185m-2.964 0h2.119a.186.186 0 0 0 .185-.186V6.29a.185.185 0 0 0-.185-.185H5.136a.186.186 0 0 0-.186.185v1.887c0 .102.084.185.186.185m5.893 2.715h2.118a.186.186 0 0 0 .186-.185V9.006a.186.186 0 0 0-.186-.186h-2.118a.185.185 0 0 0-.185.185v1.888c0 .102.082.185.185.185m-2.93 0h2.12a.185.185 0 0 0 .184-.185V9.006a.185.185 0 0 0-.184-.186h-2.12a.185.185 0 0 0-.184.185v1.888c0 .102.083.185.185.185m-2.964 0h2.119a.185.185 0 0 0 .185-.185V9.006a.185.185 0 0 0-.184-.186h-2.12a.186.186 0 0 0-.186.185v1.888c0 .102.084.185.186.185m-2.92 0h2.12a.185.185 0 0 0 .184-.185V9.006a.185.185 0 0 0-.184-.186h-2.12a.185.185 0 0 0-.185.185v1.888c0 .102.083.185.185.185M23.763 9.89c-.065-.051-.672-.51-1.954-.51-.338.001-.676.03-1.01.087-.248-1.7-1.653-2.53-1.716-2.566l-.344-.199-.226.327c-.284.438-.49.922-.612 1.43-.23.97-.09 1.882.403 2.661-.595.332-1.55.413-1.744.42H.751a.751.751 0 0 0-.75.75c-.007 1.73.425 3.43 1.25 4.977.892 1.679 2.22 2.922 3.836 3.592 1.973.799 5.146.985 7.325.985 1.815.001 3.626-.19 5.392-.573 2.483-.556 4.649-1.932 6.2-3.967a15.024 15.024 0 0 0 2.203-5.09c.048-.165.087-.336.122-.512.054-.234.086-.473.095-.714a4.81 4.81 0 0 0-.66-2.352" />
    </svg>
  );
}

// Desktop/monitor icon — matches Unraid's "VMs" tab glyph (screen + stand + base).
// FILLED (design-language.md "Icon glyphs" — every glyph is a solid shape,
// `fill="currentColor"`, never a stroked outline): three solid pieces —
// screen (a filled rounded rect, same footprint as the old stroked rect —
// a filled monitor glyph shows the whole bezel solid, not a hollow "glass"
// cutout, matching how Material Symbols Filled's own desktop icon works),
// a thin filled neck and a thin filled base bar (rule 220's "a structural
// detail that has to stay a thin line... renders as a thin filled shape" —
// the neck/base were plain strokes before, now two small solid bars instead).
export function IconVM() {
  return (
    <svg width="22" height="22" viewBox="0 0 20 20" fill="currentColor" className="shrink-0" aria-hidden="true">
      <rect x="2" y="3" width="16" height="10" rx="1.5" />
      <rect x="9.2" y="13" width="1.6" height="3.3" rx="0.5" />
      <rect x="7" y="16.3" width="6" height="1.4" rx="0.7" />
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

// Folder glyph for the Files (file-set backup) tab — FILLED (design-language.md
// "Icon glyphs"): this exact silhouette is already a closed shape under its
// old stroke, so per rule 218 ("a closed silhouette flips directly... just
// needs stroke='none' fill='currentColor' and the same path data") it just
// needed the same treatment IconFolder below already got for the SAME path
// data at a smaller badge scale — reused verbatim here at the nav-rail size
// instead of drawing a second, competing folder shape. The old divider-crease
// line is dropped, same reasoning as IconFolder's own header comment: a
// filled solid folder doesn't carry it.
export function IconFiles() {
  return (
    <svg width="22" height="22" viewBox="0 0 20 20" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M2.75 5.5A1.75 1.75 0 0 1 4.5 3.75h3.3c.47 0 .92.19 1.25.52l1.06 1.06c.14.14.33.22.53.22h4.86c.97 0 1.75.78 1.75 1.75v7.2a1.75 1.75 0 0 1-1.75 1.75h-11a1.75 1.75 0 0 1-1.75-1.75V5.5Z" />
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

// Basket-with-down-arrow glyph for the Receiver tab — REDRAWN (GlimStone
// follow-up round, jdp's live review, alongside IconConfig above:
// "Der Glyph des Empfänger-Tabs ist auch nicht erkennbar was es sein soll").
// The old inbox/tray attempt already had the right IDEA (an arrow dropping
// into a tray — jdp's own task brief later independently suggested the same
// motif back) but the geometry didn't execute it: pulling the live-rendered
// coordinates, the arrow's shaft+arrowhead (y 5.75–12.2) sat almost entirely
// INSIDE the tray silhouette's own bounding box (y 3.75–15.25+), both filled
// with the identical `currentColor` — two same-colour opaque shapes with no
// gap between them simply fuse into one solid region (this file's own
// IconCopy comment names this same failure mode), which is exactly what the
// live screenshot confirmed: a single indistinct rounded blob, no visible
// arrow, no visible tray opening.
//
// Fix: the SAME two ingredients (arrow above, receptacle below), but with a
// real, deliberate vertical GAP between the arrowhead's tip and the
// receptacle's own top edge — nothing drawn in that band at all, so it
// reads as true empty space above whatever sits behind the icon (idle
// sidebar ground or the selected `bg-accent` fill alike), needing no CSS
// variable or cutout trick to stay correct in every state. The receptacle
// itself is now an open-top basket (a wider top edge tapering to a
// rounded-corner bottom, a genuine trapezoid) rather than the old fully
// closed tray outline — a plainer silhouette than "closed box", and visibly
// distinct from IconDownload below (16×16 icon-only-badge scale), which is a
// BARE arrow with no receptacle of any kind under it (jdp: "Der Downloadglyph
// soll nie einen waagrechten Strich haben. Nur der Pfeil."), so the two never
// read as the same symbol at a glance: this one is arrow-plus-basket, that one
// is arrow-only.
export function IconReceiver() {
  return (
    <svg width="22" height="22" viewBox="0 0 20 20" fill="currentColor" className="shrink-0" aria-hidden="true">
      <rect x="9.2" y="3" width="1.6" height="5.2" rx="0.6" />
      <path d="M6.8 8.2h6.4L10 11.8Z" />
      <path d="M4.2 13.5 15.8 13.5 14.2 17.8 5.8 17.8Z" />
    </svg>
  );
}

// Connected-nodes glyph for the Fleet tab — "several boxes, watched from one
// place". FILLED (design-language.md "Icon glyphs"): the three nodes are now
// solid filled dots (dropping the hollow-ring stroke look); the connecting
// lines (rule 220's "structural detail that has to stay thin" — same
// treatment as a slider's track) are now thin filled bars instead of
// strokes — the vertical one a plain rect, the two diagonals a `<rect>`
// rotated to each segment's own angle (same rotated-filled-rect technique
// IconClose already uses for its ×, just non-45° angles here), same
// endpoints as the old stroke paths.
export function IconFleet() {
  return (
    <svg width="22" height="22" viewBox="0 0 20 20" fill="currentColor" className="shrink-0" aria-hidden="true">
      <rect x="9.3" y="6.5" width="1.4" height="3" rx="0.5" />
      <rect x="4.55" y="11.8" width="4.9" height="1.4" rx="0.5" transform="rotate(140.9 7 12.5)" />
      <rect x="10.55" y="11.8" width="4.9" height="1.4" rx="0.5" transform="rotate(39.1 13 12.5)" />
      <circle cx="10" cy="4.5" r="2.2" />
      <circle cx="4" cy="15.5" r="2.2" />
      <circle cx="16" cy="15.5" r="2.2" />
    </svg>
  );
}

// Folder glyph (GlimStone follow-up round — Paths & Storage tab rework, point
// 1/2: FolderBrowser's icon-only "Durchsuchen" button and PathModeSwitch's
// icon-only "Local" segment both need a folder glyph, and this app already
// has exactly one — IconFiles above, "Folder glyph for the Files tab"). Same
// path data, verbatim, just exported at a smaller fixed size for inline use
// next to a text field instead of IconFiles' own fixed 22×22 nav-rail scale:
// a folder is the conventional "browse/local directory" glyph everywhere
// else (OS file pickers, IDEs, ...), so this reuses that established shape
// rather than inventing a second, visually competing one. Not given a size
// prop on IconFiles itself — its five nav-icon siblings (IconContainers/
// IconVM/../IconFleet) are ALL hardcoded to the sidebar's fixed scale with no
// size prop either, so parameterizing just this one would make it the only
// resizable icon in an otherwise fixed-size family.
//
// FILLED, not stroked (GlimStone follow-up round, live-review — icon-badge
// consistency pass): IconFiles above is a stroke outline to match ITS OWN
// sibling nav-rail icons (IconVM/IconConfig/IconRecovery/...), but this
// export is a SEPARATE glyph for a different context — an icon-only coloured
// badge (FolderBrowser's Browse button, PathModeSwitch's Local segment), not
// a nav-rail row sitting beside a permanent text label. jdp, live-review
// screenshot: the two icon-only badges read as visually inconsistent — one
// glyph a solid shape, the sibling Remote badge a thin outline — and a
// small enclosed shape drawn as a 1.5px outline at 16px doesn't read as
// "the same weight" as a genuinely filled sibling once both sit side by
// side. `fill="currentColor"`, no `stroke` — reuses the exact same
// silhouette path (the folder+tab outline, unchanged) just painted solid
// instead of traced; the second interior "crease" path (purely decorative on
// a stroked glyph) is dropped since a filled solid folder doesn't carry it.
export function IconFolder() {
  return (
    <svg width="16" height="16" viewBox="0 0 20 20" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M2.75 5.5A1.75 1.75 0 0 1 4.5 3.75h3.3c.47 0 .92.19 1.25.52l1.06 1.06c.14.14.33.22.53.22h4.86c.97 0 1.75.78 1.75 1.75v7.2a1.75 1.75 0 0 1-1.75 1.75h-11a1.75 1.75 0 0 1-1.75-1.75V5.5Z" />
    </svg>
  );
}

// Cloud glyph — same design as Settings.tsx's own IconTabOffsite ("A cloud —
// the remote/off-site replica target"), reused here at this file's shared
// icon-set scale for the same "remote" concept elsewhere (PathModeSwitch's
// icon-only "Remote" segment, GlimStone follow-up round point 2): both files
// intentionally draw the exact same shape rather than each inventing an
// independent cloud glyph. Settings.tsx's own tab icons stay local/private by
// their own header comment ("a different taxonomy than the sidebar's page
// destinations") — that comment is about TAB semantics, not about whether the
// raw glyph shape can be reused elsewhere, so this copies the path data here
// rather than exporting the tab-scoped original.
//
// FILLED, not stroked — this was the actual bug the live-review screenshot
// caught: this glyph and IconFolder above sit in the SAME icon-only-badge
// row (PathModeSwitch's Local/Remote pair) and must read as the same weight,
// but this one was still a 1.3px stroke outline while its sibling read as
// solid. Same fix as IconFolder's own comment above, applied to this glyph's
// existing closed cloud-silhouette path: `fill="currentColor"`, no `stroke`.
export function IconCloud() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M4.5 12.5A3 3 0 0 1 4 6.53 3.5 3.5 0 0 1 10.9 5.1 2.75 2.75 0 0 1 12.5 10.4v.1a2.25 2.25 0 0 1-2 2h-6Z" />
    </svg>
  );
}

// Plus glyph — the conventional "add a new one" symbol (Settings.tsx's own
// "Registry hinzufügen" icon-only button, GlimStone follow-up round: "beide
// sollen einen Glyph statt Text bekommen"). Two overlapping filled rounded
// rects, per design-language.md's own icon-glyph rule for a line-like shape
// ("a plus... needs real geometry, not a thicker stroke... two overlapping
// filled rects"), at this file's own 16×16 icon-only-badge scale (IconFolder/
// IconCloud above), not the 22×22 nav-rail scale.
export function IconAdd() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <rect x="6.8" y="2.5" width="2.4" height="11" rx="0.6" />
      <rect x="2.5" y="6.8" width="11" height="2.4" rx="0.6" />
    </svg>
  );
}

// Download glyph — REDRAWN (Containers.tsx Task 1, jdp live-review: "Der
// Downloadglyph soll nie einen waagrechten Strich haben. Nur der Pfeil.").
// Used to be a downward arrow over a separate filled tray bar (the
// conventional "arrow + tray" download silhouette); jdp flagged that
// horizontal bar itself as unwanted, at every one of this glyph's call sites
// (Settings.tsx's "Recovery-Kit herunterladen", Containers.tsx's Export
// button) — not a request to swap in a DIFFERENT tray shape, an explicit
// "never a horizontal bar under the arrow" rule. Fixed by deleting the tray
// `<rect>` outright and keeping only the shaft+arrowhead silhouette that
// already existed above it — one closed filled path (design-language.md: "a
// line glyph... needs real geometry" — no `stroke`), matching this file's own
// 16×16 icon-only-badge scale.
//
// RESCALED (jdp, live review: "Containertab, Containercard: der Glyph auf dem
// Downloadbadge (Export plain) ist zu klein."). Deleting the tray bar left the
// arrow exactly where it had been drawn — in the UPPER portion of a viewBox it
// used to share with that bar — and nothing re-derived its proportions
// afterwards, so it kept a footprint sized for "arrow plus tray" while
// rendering as "arrow alone". Measured (SVG getBBox on the live-rendered
// glyph, ink extent in the 16-unit viewBox, at this file's own 16px render
// size), old shape against the siblings it actually sits beside:
//   IconDownload   7.10 × 8.70 px   (44.4% × 54.4% of the box, centre y 42.2%)
//   IconBackupNow 12.00 × 12.00 px  (75.0% × 75.0%, centre 50/50)
//   IconCopy      12.00 × 12.00 px  (75.0% × 75.0%, centre 50/50)
//   IconRestore   12.00 × 12.90 px  (75.0% × 80.6%)
//   IconTrash     10.80 × 13.10 px  (67.5% × 81.9%)
//   IconAdd       11.00 × 11.00 px  (68.8% × 68.8%)
// So it was not only the smallest glyph in the set by a wide margin (its ink
// box covered 43% of IconBackupNow's, and its FILLED area roughly 24 px²
// against that glyph's ~91 px²) but also sat 1.25px high in its own box —
// which in a 32px badge reads as an arrow floating above centre.
//
// Redrawn to the same construction (one closed filled path, shaft + 45°
// arrowhead, no tray bar, no stroke) at the family's own proportions: ink now
// 12.00 × 12.00 px, 75% × 75%, centred exactly on 50/50 — the identical
// footprint IconBackupNow and IconCopy measure, so the Export badge and the
// Jetzt-sichern badge beside it in a Container card now carry the same optical
// weight. Every coordinate is a WHOLE unit of the 16-unit viewBox (6/10/2/8/
// 14/4), so at the 1:1 render size the shaft's long vertical edges land on
// real pixel boundaries and stay crisp instead of anti-aliasing to grey — the
// same crispness constraint IconBackupNow's own comment records, and the
// reason a slightly heavier half-unit variant (shaft 5 units wide, x 5.5→10.5)
// was rendered alongside this one and rejected: it fringed both shaft edges
// for no legibility gain.
//   Judged the way IconBackupNow's redraw was: every candidate rasterised at
// its TRUE 16×16 and the RASTER magnified ×14 nearest-neighbour, against the
// same treatment of IconBackupNow/IconTrash/IconRestore/IconCopy/IconAdd —
// never by re-rendering the vector large, which flatters every shape equally.
// Both call sites (Containers.tsx's Export badge, Settings.tsx's Recovery-Kit
// download badge) are `size="icon"` Badges that give the glyph a plain 32px
// square with no adjacent geometry to clear, so neither depended on the old
// small footprint.
export function IconDownload() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M6 2h4v6h4l-6 6-6-6h4V2Z" />
    </svg>
  );
}

// Backup-now glyph — REPLACES the old IconUpload (an upward arrow over a
// tray, the mirror of IconDownload's pre-fix shape above) as BackupButton's
// own icon (Containers.tsx Task 1, jdp live-review: "Für den Jetzt-sichern-
// Button bitte einen treffenderen Glyph" — an arrow mirroring the download
// glyph read as "the opposite of download," not specifically "an immediate,
// on-demand backup," and it shared the exact tray bar jdp separately flagged
// on IconDownload). A shield motif was considered and rejected — IconConfig
// above already carries this app's one padlock for a different "this
// instance's own state, kept secure" concept, and this file's own history
// (see IconConfig's comment) already ruled out reusing a shield/lock
// silhouette for a second, unrelated meaning nearby. Drawn as a floppy disk —
// the conventional, universally-recognised "save state" glyph (Material
// Symbols' filled `save` and virtually every desktop app's own Save icon draw
// the identical shape).
//
// REDRAWN a second time (jdp, live review: "Bitte ein anderes Symbol für den
// 'Jetzt sichern'-Badge, ein gut erkennbares Speichern-Symbol"). The previous
// take was a floppy with its label window cut into the shape of a DOWNWARD
// ARROW — and that is exactly what failed: at 16px the arrow's shaft and
// barbs come out around one device pixel wide each, so the punched arrow
// collapsed into an indistinct smudge inside an already-small body, and the
// glyph as a whole read as "a dark box with something in it".
//   Two things changed, both found by rasterising every candidate at its REAL
// 16×16 and magnifying that raster (not by re-rendering the vector large,
// which flatters every shape equally and is what let the previous version
// through):
//   1. The arrow is gone. jdp asked for a save symbol, not a save symbol plus
//      a direction; the "now/immediacy" half of the meaning is carried by the
//      button's own tip ("Jetzt sichern") and by its accent-filled active
//      badge, not by cramming a second motif into 16 pixels.
//   2. The body is a CLOSED square again. Intermediate drafts let the label
//      window run out through the bottom edge (as several stock save icons
//      do at larger sizes); at 16px that turns the silhouette into an arch
//      standing on two legs and stops reading as a disk at all. Both windows
//      are now fully enclosed holes — real ones, punched with
//      `fillRule="evenodd"` the way IconCheckCircle's ring is, so they show
//      whatever sits behind the glyph rather than a colour-matching fake —
//      with ~2px of body left on every side of them and a 2.5px gap between
//      the two, all landing on whole/half units of the 16-unit viewBox so the
//      edges stay crisp instead of anti-aliasing to grey.
// The one remaining flourish is the clipped top-right corner every real
// floppy has; it survives at 16px because it is a 2.5px diagonal, not a
// detail.
//   Still unmistakably distinct from its neighbours at a glance: IconDownload
// (a lone arrow, no body), IconRestore/IconRecovery (a circular sweep),
// IconCopy (two offset plain rounded squares, no windows), IconFolder,
// IconCloud, IconTrash, IconConfig (a padlock). Checked side by side with the
// Export badge it actually sits next to, at true size, in both themes.
export function IconBackupNow() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M2 3.2A1.2 1.2 0 0 1 3.2 2h8.3L14 4.5V12.8A1.2 1.2 0 0 1 12.8 14H3.2A1.2 1.2 0 0 1 2 12.8Z
           M5 3h6v3H5Z
           M4 8.5h8v4H4Z"
      />
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

// Circular "restore" arrow, at this file's own 16×16 icon-only-badge scale —
// the SAME filled-ring-segment + arrowhead construction as this file's own
// IconRecovery above (the Recovery nav-tab, 22×22) — extending that
// already-established "revert/restore" visual family to a second, exported
// call site (RestorePanel.tsx's per-snapshot "Wiederherstellen…" trigger,
// Containers.tsx icon-badge round) rather than inventing a competing
// "restore" glyph.
//   NOT shared any more with Settings.tsx's "reset to default" glyph
// (IconResetArrow, formerly IconResetSwirl, formerly identical path data to
// this icon): that one deliberately diverged to a bolder ring/arrowhead
// ratio for its own harder legibility case (16px, beside 8 competing colour
// swatches) — see IconResetArrow's own header comment in Settings.tsx. This
// icon keeps its original proportions, already correct for a plain single
// badge with no adjacent colour to compete against.
export function IconRestore() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M14 8A6 6 0 1 1 8 2L8 3.8A4.2 4.2 0 1 0 12.2 8Z" />
      <path d="M8 1.1 4.3 3 8 4.9Z" />
    </svg>
  );
}

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

// Power symbol — "graceful": the VM is shut down cleanly before the snapshot.
// Built as a true annular sector (ONE subpath: the outer arc taken the long
// way round, then the inner arc back) plus the interrupt bar, rather than a
// stroked circle — a `stroke` would not scale with the badge's own font-size
// and would break this file's fill-only construction.
export function IconPower() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M10.76 4.19A5.2 5.2 0 1 1 5.24 4.19L6.15 5.63A3.5 3.5 0 1 0 9.85 5.63Z" />
      <rect x="7.1" y="1.5" width="1.8" height="6.4" rx="0.9" />
    </svg>
  );
}

// Lightning bolt — "live": the VM keeps running and is snapshotted hot. The
// angular counterpart to IconPower's ring above; a single filled zigzag, the
// one shape that stays unambiguous at 16px with no internal detail to lose.
export function IconLive() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M9.7 1.3 4 9h3.3l-.9 5.7L12 6.8H8.8Z" />
    </svg>
  );
}

// Trash-can glyph — the conventional "remove this row" symbol (Settings.tsx's
// own standalone Registries Card, GlimStone follow-up round: "Wenn man eine
// Registry hinzufügt, soll der Entfernen-Button quadratisch sein mit
// Mülleimer-Icon"). Same solid, `currentColor`-only, no-`stroke` construction
// as IconAdd/IconDownload above (design-language.md's icon-glyph rule — "a
// line glyph... needs real geometry"), at this file's own 16×16
// icon-only-badge scale: a small handle bar, a wider lid bar, and a tapered
// body silhouette underneath, three filled shapes, no internal rib lines (a
// classic trash-can glyph's vertical ribs are normally cut out with a
// second, contrasting fill — not available here since design-language.md's
// icon-only-badge rule reserves colour for the BADGE background, never the
// glyph itself, so the body stays one plain silhouette).
export function IconTrash() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <rect x="6.2" y="1.3" width="3.6" height="1.15" rx="0.5" />
      <rect x="2.6" y="2.7" width="10.8" height="1.3" rx="0.5" />
      <path d="M3.8 4.3h8.4l-.8 9.2a1 1 0 0 1-1 .9H5.6a1 1 0 0 1-1-.9l-.8-9.2Z" />
    </svg>
  );
}

// Pencil glyph — the conventional "edit this" symbol (Files.tsx's per-file-set
// "Ordner-Set bearbeiten" button converting to a square icon badge, jdp:
// "Ordnertab: die Buttons 'Ordnerset bearbeiten' und 'Set entfernen' sollen
// quadratische Badges mit Glyphen sein.").
//
// NOT a new drawing. This app already had exactly one pencil — Dashboard.tsx's
// customize toggle, an inline `<svg>` on a 24-unit grid — and its path data is
// reproduced here VERBATIM, character for character, rather than a second
// pencil being invented next to it. Dashboard.tsx now renders this component
// instead of its own inline copy, so there is one pencil in the codebase, not
// two that can drift.
//
// What DID change is the framing, and only because the same defect this
// round's IconDownload fix is about would otherwise repeat here immediately.
// The Dashboard pencil's ink covers 63.0% × 58.8% of its own 24-unit viewBox,
// centred at (48.2%, 53.9%) — measured with getBBox, not estimated. Dropped
// straight into a 16px icon-badge slot that would render 10.07 × 9.41 px of
// ink, against a sibling family that measures 11–13 px in both axes
// (IconBackupNow 12.00 × 12.00, IconTrash 10.80 × 13.10, IconAdd 11.00 ×
// 11.00, IconCopy 12.00 × 12.00) — i.e. it would arrive as the new smallest
// glyph in the set, the exact complaint that produced this round.
//   Fixed WITHOUT touching a single path coordinate, by cropping the viewBox
// to the ink instead: `1.65 3.02 19.83 19.83` is the smallest square window
// centred on that measured ink box (centre 11.568, 12.936) that leaves the
// glyph filling 76.3% of it. Rendered at 16px that is 12.20 × 11.39 px of ink,
// centred exactly on 50/50 — squarely inside the family. Cropping the window
// rather than rescaling the path is what keeps the reuse honest: the `d`
// attribute is still Dashboard's own, and there is no transform stack or
// re-derived arithmetic to get wrong.
//   Rendered at true 16px and judged from a ×14 nearest-neighbour magnified
// RASTER beside IconTrash/IconAdd/IconCopy (the same method this file's
// IconBackupNow and IconDownload comments describe), and cross-checked against
// the Dashboard pencil's own pre-change 18px rendering: same silhouette, same
// optical weight (its ink grows 11.34 × 10.58 → 12.20 × 11.39 px there, a
// change of well under a pixel per axis).
export function IconPencil() {
  return (
    <svg width="16" height="16" viewBox="1.65 3.02 19.83 19.83" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M4 20h4L18.5 9.5a2.121 2.121 0 0 0-3-3L5 17v3z" />
    </svg>
  );
}

// Check-in-circle glyph — "Verbindung testen" (Settings.tsx's off-site
// TestConnectionButton, GlimStone follow-up round: "Können wir die Buttons in
// quadratische Badges mit Glyphen umwandeln?", jdp explicitly named this
// exact fallback — "ein Plug/Verbindungs-Icon (oder ein simples Häkchen im
// Kreis, falls ein Stecker bei kleiner Größe schwer sauber zu zeichnen ist)":
// a plug drawn at this file's own 16×16 icon-only-badge scale reads as an
// ambiguous blob (tried first, discarded — see this round's own icon-preview
// scratch check), while a filled check-in-a-circle reads unambiguously as
// "verified / test passed" at the same size, so this takes the named
// fallback rather than forcing the harder glyph. A proven, widely-used
// silhouette (Heroicons v1 solid `check-circle`, ported at this file's own
// 16×16 icon-only-badge scale — scaled ×0.8 from its native 20×20 grid,
// arithmetic re-derived and re-checked by rendering it standalone before
// wiring it in, not hand-guessed), not a fresh invention: a solid ring plus a
// checkmark cut as a true hole through it, which needs `fillRule="evenodd"`
// (NOT the file's usual plain default nonzero winding every other glyph here
// uses) — the ring subpath and the checkmark subpath wind the same direction
// in this source path, so nonzero fill unions them into one solid disc with
// no visible check at all (this exact failure was caught by the same
// render-before-wiring-in check, not left to be found live). One filled
// `currentColor` path, no `stroke` — the ring's "outline" look comes from
// carving an inner circle out of an outer one via evenodd, still real filled
// geometry per design-language.md's icon-glyph rule, not a stroke.
export function IconCheckCircle() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M8 14.4a6.4 6.4 0 1 0 0-12.8a6.4 6.4 0 0 0 0 12.8z m2.9656-7.4344a0.8 0.8 0 0 0-1.1312-1.1312L7.2 8.4688 6.1656 7.4344a0.8 0.8 0 0 0-1.1312 1.1312l1.6 1.6a0.8 0.8 0 0 0 1.1312 0l3.2-3.2z"
      />
    </svg>
  );
}

// Circular sync/refresh-arrows glyph — "Jetzt replizieren" (Settings.tsx's
// off-site ReplicateNowButton, same GlimStone follow-up round as
// IconCheckCircle above: "einen Sync-/Refresh-Pfeile-Glyph"). Ported from the
// conventional two-arrow circular-refresh silhouette (Material Design Icons'
// baseline "refresh" glyph, a single filled path — no stroke arcs, matching
// this file's icon-glyph rule the same way IconCheckCircle's ring does) at
// this file's own 16×16 icon-only-badge scale — scaled ×2/3 from its native
// 24×24 grid, every coordinate (including the Bézier control points, which
// scale linearly the same as plain line endpoints under uniform scaling)
// re-derived by hand and rendered standalone before wiring it in, same
// verification pass as IconCheckCircle above.
export function IconSync() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M11.77 4.23C10.8 3.27 9.47 2.67 8 2.67C5.05 2.67 2.67 5.05 2.67 8C2.67 10.95 5.05 13.33 8 13.33C10.49 13.33 12.56 11.63 13.15 9.33L11.77 9.33C11.22 10.89 9.74 12 8 12C5.79 12 4 10.21 4 8C4 5.79 5.79 4 8 4C9.11 4 10.09 4.46 10.81 5.19L8.67 7.33H13.33V2.67L11.77 4.23Z" />
    </svg>
  );
}

// Cog/gear glyph — "Einrichten" (Settings.tsx's off-site per-domain wizard
// toggle, same GlimStone follow-up round: "ein Zahnrad/Schraubenschlüssel-
// Glyph, wenn geschlossen"). NOT a fresh glyph: IconSettings above (this same
// file) already draws exactly this "conventional settings symbol", filled/
// currentColor/no-stroke already, the identical shape this task independently
// asked for — reused verbatim rather than inventing a second, visually
// competing cog, the same "shrink an existing path's own width/height attrs,
// keep its viewBox untouched" technique IconFolder/IconCloud above already
// established for reusing a bigger nav-rail glyph at this file's smaller
// icon-only-badge scale. IconSettings itself stays module-private (only used
// by the nav rail's own icon map) — this is a second, exported, small-scale
// instantiation of its path data, not a change to IconSettings or its call site.
export function IconGear() {
  return (
    <svg width="16" height="16" viewBox="0 0 20 20" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 0 1-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 0 1 .947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 0 1 2.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 0 1 2.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 0 1 .947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 0 1-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 0 1-2.287-.947zM10 13a3 3 0 1 0 0-6 3 3 0 0 0 0 6z"
      />
    </svg>
  );
}

// Close/X glyph — "Schließen", the SAME toggle's open-state icon (swapping
// with IconGear above the same way the button's own text used to swap
// between "Einrichten…"/"Schließen"). Built from IconAdd's own exact two-rect
// geometry above, just rotated ±45° instead of left axis-aligned: same
// `rx`/width/height/thickness as the plus, so Add's "+" and this "×" read as
// the same weight/family the way IconFolder/IconCloud's own fix made solid
// icon-badge siblings match each other — appropriate here too, since this
// glyph is literally the "cancel/undo the add-a-new-thing-below action"
// counterpart in the same row family (design-language.md's icon-glyph rule:
// a line-like shape "needs real geometry, not a thicker stroke... two
// overlapping filled rects", the exact technique IconAdd already uses).
export function IconClose() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <rect x="6.8" y="2.5" width="2.4" height="11" rx="0.6" transform="rotate(45 8 8)" />
      <rect x="6.8" y="2.5" width="2.4" height="11" rx="0.6" transform="rotate(-45 8 8)" />
    </svg>
  );
}

// Copy/clipboard glyph — "Kopieren" (Settings.tsx's own VMSSHCard, GlimStone
// follow-up round: "beide Kopieren-Buttons sollen ein quadratischer Badge mit
// Glyph sein" — both the public-key copy and the authorize-command copy
// button convert from a short-text button to this icon-only glyph). The
// conventional "two overlapping sheets" copy silhouette, built the SAME way
// IconAdd's "+" and IconClose's "×" already are — two plain filled
// `<rect>`s, nothing hollowed out or colour-cut. Deliberately NOT the
// evenodd "picture-frame ring behind a solid sheet" construction most icon
// sets actually use for this glyph (Material Symbols' filled `content_copy`,
// for one): that needs a real inner-rect subtraction, and this file's own
// IconConfig/IconReceiver history (the sliders' punched "knobs" and the
// tray's overlapping arrow, both GlimStone follow-up round, both fixed at
// their own call sites) already proved what goes wrong the moment a detail
// meant to read as a distinct shape ends up either (a) a `var(--sidebar-
// surface, transparent)` cutout with no real surface value behind it to
// show, invisible against ANY background by construction, or (b) painted in
// the exact same `currentColor` as a shape already sitting under it, so the
// two silhouettes fuse into one solid blob with no visible seam. Two plain
// overlapping RECTS sidesteps both failure modes entirely: the LATER rect
// (paint order, not z-index — SVG has none) simply covers whatever the
// earlier one drew underneath it, so the exposed L-shaped sliver of the back
// rect's own top-left corner is real, uncovered space with nothing special
// needed to keep it visible — no CSS variable, no evenodd, no assumption
// about what sits behind the icon at any given moment (idle sidebar ground
// vs. the solid `bg-accent` this row's own background becomes once selected
// — see NavItem's own `navActive`/`glim-hue-icon` comments for that same
// idle/selected split, which this glyph never has to reason about at all
// because it paints itself, not a hole into whatever the row's background
// happens to be).
export function IconCopy() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <rect x="2" y="2" width="9" height="9" rx="1.6" />
      <rect x="5" y="5" width="9" height="9" rx="1.6" />
    </svg>
  );
}

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
