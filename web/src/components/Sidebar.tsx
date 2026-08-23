import { NavLink, useNavigate } from "react-router-dom";
import { useState, useRef, useEffect } from "react";
import { type Settings } from "../lib/api";
import { useT } from "../lib/i18n";
import { useAdvanced } from "../lib/advanced";

interface SidebarProps {
  settings: Settings | null;
}

interface NavItem {
  to: string;
  label: string;
  icon: React.ReactNode;
}

// Deliberately NOT rainbowed (GlimStone form-engine Phase 2, Task 2 — decided
// in the spec-compliance review of the first attempt, which had wired every
// NavItem to a palette position). Task 2's brief is explicitly "decide which
// candidates genuinely benefit"; the nav rail does not, on two counts:
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
// decision and its two counts above stand unchanged. The nav rail's own
// idle-colourless/hover-tints-icon/selected-fills-badge behaviour (see
// `navInactive`'s own `bv-nav-idle` marker and index.css's matching rule)
// reads the single FLAT --accent token, never .glim-hue/hueVars() — the flat
// accent is the same one colour for every destination regardless of how many
// are visible or in what order, so point 2 above (no stable position) never
// gets a chance to apply in the first place.

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

// Desktop/monitor icon — matches Unraid's "VMs" tab glyph (screen + stand + base)
export function IconVM() {
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

// Folder glyph for the Files (file-set backup) tab — stroked to match the
// sibling VM/Config/Recovery icons.
export function IconFiles() {
  return (
    <svg width="22" height="22" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" className="shrink-0" aria-hidden="true">
      <path d="M2.75 5.5A1.75 1.75 0 0 1 4.5 3.75h3.3c.47 0 .92.19 1.25.52l1.06 1.06c.14.14.33.22.53.22h4.86c.97 0 1.75.78 1.75 1.75v7.2a1.75 1.75 0 0 1-1.75 1.75h-11a1.75 1.75 0 0 1-1.75-1.75V5.5Z" strokeLinejoin="round" />
      <path d="M2.75 8.25h14.5" strokeLinecap="round" />
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

// Inbox / tray-with-down-arrow glyph for the Receiver tab — "off-site copies
// land here". Stroked to match the sibling VM/Files/Recovery nav icons.
export function IconReceiver() {
  return (
    <svg width="22" height="22" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" className="shrink-0" aria-hidden="true">
      <path d="M3 12.5 4.4 5.2A1.75 1.75 0 0 1 6.1 3.75h7.8a1.75 1.75 0 0 1 1.7 1.45L17 12.5v2.75a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 3 15.25V12.5Z" strokeLinejoin="round" />
      <path d="M3 12.5h3.5a1 1 0 0 1 1 1 1.5 1.5 0 0 0 1.5 1.5h2a1.5 1.5 0 0 0 1.5-1.5 1 1 0 0 1 1-1H17" strokeLinejoin="round" />
      <path d="M10 5.75v4.5M8 8.25l2 2 2-2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

// Connected-nodes glyph for the Fleet tab — "several boxes, watched from one
// place". Stroked to match the sibling VM/Files/Receiver nav icons.
export function IconFleet() {
  return (
    <svg width="22" height="22" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.5" className="shrink-0" aria-hidden="true">
      <circle cx="10" cy="4.5" r="2" />
      <circle cx="4" cy="15.5" r="2" />
      <circle cx="16" cy="15.5" r="2" />
      <path d="M10 6.5v3M8.6 11.2 5.4 13.8M11.4 11.2l3.2 2.6" strokeLinecap="round" strokeLinejoin="round" />
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

// Download glyph — a downward arrow over a tray, the conventional "save this
// to disk" symbol (Settings.tsx's own "Recovery-Kit herunterladen" icon-only
// button, same GlimStone follow-up round point as IconAdd above). One closed
// filled silhouette for the shaft+arrowhead (design-language.md: "a line
// glyph... needs real geometry" — no `stroke` anywhere), plus a separate
// filled bar for the tray, matching this file's own 16×16 icon-only-badge
// scale.
export function IconDownload() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" className="shrink-0" aria-hidden="true">
      <path d="M6.9 2.4h2.2v4.9h2.45L8 11.1 4.45 7.3H6.9V2.4Z" />
      <rect x="2.6" y="12.6" width="10.8" height="1.6" rx="0.6" />
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
// `bv-nav-idle` (GlimStone follow-up round, live-review — the tab-strip
// 3-state colour rule, jdp: "Beim Mouseover soll das Icon eingefärbt
// werden..."): a plain marker class, no styling of its own here — index.css's
// matching `.bv-nav-idle:hover svg`/`:focus-within svg` rule reads it to tint
// just the glyph with the flat `--accent` token on hover/focus, while idle
// (no marker match) and active (a different class entirely, navActive below,
// never carries this marker) both stay as they already were. See that CSS
// rule's own comment for why this reads the single flat accent rather than
// .glim-hue/hueVars() the way the Settings tab strip's icons do — this
// file's own header comment already explains why the nav rail can't own a
// stable rainbow position.
const navInactive =
  "bv-nav-idle text-(--sidebar-text) hover:bg-carbon-hover hover:text-carbon-text motion-safe:hover:translate-x-0.5 motion-safe:hover:rtl:-translate-x-0.5!";

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
  const filesEnabled = settings?.filesEnabled ?? false;
  const receiverEnabled = settings?.receiverEnabled ?? false;
  const fleetEnabled = settings?.fleetEnabled ?? false;

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
        {/* VMs / Flash / Files tabs appear only once their domain is enabled. */}
        {vmsEnabled && (
          <NavItem to="/vms" label={t("nav.vms")} icon={<IconVM />} />
        )}
        {flashEnabled && (
          <NavItem to="/flash" label={t("nav.flash")} icon={<IconFlash />} />
        )}
        {filesEnabled && (
          <NavItem to="/files" label={t("nav.files")} icon={<IconFiles />} />
        )}
        {/* Config self-backup tab appears only once its domain is enabled. */}
        {configEnabled && (
          <NavItem to="/config" label={t("nav.config")} icon={<IconConfig />} />
        )}
        {/* Receiver dashboard appears only once its domain is enabled. */}
        {receiverEnabled && (
          <NavItem to="/receiver" label={t("nav.receiver")} icon={<IconReceiver />} />
        )}
        {/* Fleet view appears only once its domain is enabled. */}
        {fleetEnabled && (
          <NavItem to="/fleet" label={t("nav.fleet")} icon={<IconFleet />} />
        )}
      </nav>

      {/* Bottom group: the Simple/Advanced view toggle (SidebarControls),
          then Settings. Language and theme both moved out — see
          SidebarControls' own header comment. */}
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
