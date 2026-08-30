import { useTipBubble } from "../lib/useTipBubble";

// InfoBubble — a neutral (i) icon that reveals a short help text on hover AND
// focus (keyboard-accessible). House convention (matches CannonadeCommand's
// cc-info): explanations belong behind an inline (i) next to the label, never
// as permanent grey paragraph text under the control — that costs vertical
// space forever after being read once.
//
// Non-negotiables (see the "explanations-belong-in-an-info-bubble" note):
//   - NEVER the accent colour — the icon is furniture, accent means "active".
//   - Portal-rendered to <body>, positioned off the icon's own rect, so it is
//     never clipped by a card's overflow:hidden ancestor.
//   - Closes on scroll instead of drifting out of position.
//   - pointer-events:none on the bubble — it must never eat a click meant for
//     whatever is underneath it.
//   - The help text is also the icon's aria-label; Escape closes it.
//   - ALWAYS fully within the viewport — never clipped at any edge. Live bug
//     (jdp): the "Wiederherstellungskit"/recovery-kit bubble in Settings.tsx
//     overflowed the browser window and part of it couldn't be read. Root
//     cause: this component (and Selector.tsx's SelectorTab, which had the
//     identical copy-pasted positioning line) centred the bubble on the
//     trigger and always opened downward with NO viewport awareness at all —
//     a trigger near the right edge pushed the horizontally-centred bubble
//     half off-screen, and a trigger low on a tall page (or a long tip like
//     recovery.why's multi-sentence explanation, which wraps to a genuinely
//     tall box) pushed it past the bottom.
//
// All of the above — the open state, the viewport clamp, the close-on-scroll
// and Escape contract, the portal — now lives ONCE in lib/useTipBubble.tsx
// and this file is a trigger that calls it. Having to fix that clamp twice,
// in two files, from one bug report is precisely why: see the hook's header.
// What stayed here is what is actually this component's own: the (i)
// silhouette, its two colour treatments, and the <label> click fix below.
//
// `onAccent` (live-review follow-up: "the (i) icon is hard to see on a
// solid-accent section-title badge, especially a light/yellow accent").
// Settings.tsx's Card() nests an InfoBubble INSIDE its own tone="heading"
// Badge (`bg-accent text-accentContrast`) so the bubble rides along as part
// of the same floating notch — see Card's own header comment. That badge
// already computes --accent-contrast specifically to guarantee a legible
// ink colour on top of whatever accent/hue is active, so the icon can just
// INHERIT that via `currentColor` instead of carrying its own fixed neutral
// tone: the icon's SVG strokes/fill were already `currentColor` (never
// hard-coded), it was only the wrapping <span>'s own `text-carbon-textMuted`
// class that pinned the colour and blocked inheritance. `onAccent` drops
// that pin (`text-current`, i.e. explicitly inherit) and skips the idle
// opacity dip (kept at full strength rather than 80%, since a translucent
// icon sitting on a busy accent fill has less margin than one sitting on a
// plain card surface) — every OTHER call site (ToggleRow's caption, every
// plain-card Card body hint) omits this prop and keeps the exact neutral
// look it always had.
export function InfoBubble({ tip, onAccent = false }: { tip: string; onAccent?: boolean }) {
  const tooltip = useTipBubble(tip);

  return (
    <>
      <span
        ref={tooltip.ref}
        aria-label={tip}
        aria-describedby={tooltip.describedBy}
        tabIndex={0}
        {...tooltip.handlers}
        // Bugfix (found live while verifying a NEW label+InfoBubble call site
        // this same round, notify.healthchecks — but the gap turned out to
        // pre-exist at every one of this component's OTHER call sites that
        // already sit inside a <label> alongside their own input/select:
        // cloud.storageClass.label, flash.zipExport.keepN, drill.target, and
        // the two ["key", info] map-driven labels). A plain `<span>`, even
        // with `tabIndex`, is not one of the browser's natively-recognised
        // "interactive" exemptions from a <label>'s implicit click-forwarding
        // (unlike a real <button>/<a>/form control) — clicking this icon
        // therefore ALSO fired the ancestor label's default action, stealing
        // focus to its associated input/select, which immediately fired this
        // span's own `onBlur={hide}` and closed the tooltip a frame after it
        // opened. Verified live (getting `document.activeElement` after a
        // real, trusted click landed on the icon): the adjacent field, not
        // this span, ended up focused, and the bubble closed instantly.
        //   `stopPropagation()` alone does NOT fix this — verified live, it
        // changed nothing — because a <label>'s forwarding is native browser
        // behaviour keyed off the click event's target chain, not a JS
        // bubble-phase listener stopPropagation can intercept.
        // `preventDefault()` on click IS what blocks it (also verified live:
        // this span correctly keeps focus on itself and the tooltip opens
        // and stays open), without disabling anything this component
        // otherwise relies on — the span's own focus-on-click still happens
        // (browsers assign focus on mousedown, before "click" fires, so
        // preventDefault here doesn't undo it), and at every call site that
        // does NOT sit inside a <label> a plain <span> click has no default
        // action to prevent in the first place, so this is a no-op there.
        onClick={(e) => e.preventDefault()}
        className={`inline-flex h-[15px] w-[15px] flex-none cursor-help items-center justify-center rounded-pill focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--focus-ring) ${
          onAccent ? "text-current" : "text-carbon-textMuted opacity-80 hover:opacity-100 focus:opacity-100"
        }`}
      >
        <svg viewBox="0 0 16 16" width="15" height="15" fill="none" aria-hidden="true">
          <circle cx="8" cy="8" r="7" stroke="currentColor" strokeWidth="1.3" />
          <circle cx="8" cy="4.6" r="0.9" fill="currentColor" />
          <path d="M8 7v4.4" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" />
        </svg>
      </span>
      {tooltip.bubble}
    </>
  );
}
