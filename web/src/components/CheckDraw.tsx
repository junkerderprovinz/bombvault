// ---------------------------------------------------------------------------
// CheckDraw — GlimStone motion-engine, animation 5 (drawn-checkmark).
//
// Replaces the plain "✓" glyph a handful of busy→success indicators across
// this app render the instant an in-session action (verify/unlock/drill/
// tamper-test/import/backup) finishes successfully, with a checkmark that
// draws itself via SVG stroke-dashoffset instead of appearing all at once.
//
// `pathLength="1"` on the <path> makes the path's length exactly 1 unit
// REGARDLESS of its real on-screen geometry — a plain `strokeDasharray: 1` +
// animating `strokeDashoffset` from 1 (fully undrawn) to 0 (fully drawn)
// therefore always draws the WHOLE glyph, with no getTotalLength() call or
// per-shape magic number needed. `stroke="currentColor"` deliberately, not a
// hard-coded colour: every call site already wraps this in a
// `text-statusOk`-classed element for the "✓ " text it sits beside, and
// `currentColor` picks that up for free — this glyph is a STATUS colour
// (always "success green"), never a rainbow hue, matching every other
// state-colour indicator in this app (design-language.md's own rule 4: the
// four state hues are never rainbowed).
//
// The actual animation lives in index.css (`.bv-check-draw`, "Round 2, item
// 5" — see that rule's own comment, and the SAFE-DEFAULT rule right above
// the (prefers-reduced-motion: no-preference) block it lives inside, for
// why a reduced-motion viewer sees a fully-drawn checkmark on the very
// first frame instead of one stuck invisible). This component only ever
// renders the markup; it carries no animation logic and no "did this just
// change" state of its own — every call site already only renders it at the
// exact moment a piece of state transitions from busy to a FRESH success
// (never present at initial mount, never re-rendered while already "ok"),
// so React creating a brand-new SVG node IS the "did this just happen"
// signal, the same way glim-shake's own conditionally-rendered siblings
// already work.
// ---------------------------------------------------------------------------

/** A small drawn checkmark. Sized to sit inline with `text-sm`/`text-xs`
 *  body text (the sizes every call site today uses) without a wrapper. */
export function CheckDraw() {
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="inline-block shrink-0 align-[-1px]"
      aria-hidden="true"
    >
      <path pathLength="1" className="bv-check-draw" d="M3 8.5L6.5 12L13 4" />
    </svg>
  );
}
