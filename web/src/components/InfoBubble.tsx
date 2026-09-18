import { useEffect, useRef } from "react";
import { useTipBubble } from "../lib/useTipBubble";
import { useIsCoarsePointer } from "../lib/useMediaQuery";

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
// Tap path: hover and focus are both dead on a phone, so
// below the coarse-pointer axis — capability, NOT width (the same axis
// separation every touch surface follows); useTipBubble's documented manual show()/hide() API is the
// sanctioned migration entry, exactly because the hook keeps owning content,
// positioning and dismissal — a tap now opens the bubble through the same
// hook state as hover. The tap-open is deliberately OPEN-ONLY, not a toggle:
// on Android a tap fires focus BEFORE click, and focus has already run the
// hook's onFocus=show() by the time click arrives, so a click-toggling on
// "shown" would re-hide the bubble the same gesture just opened. Dismissal
// on touch comes from the paths that already exist plus one added here:
// outside-tap (the pointerdown effect below — pointer-events:none on the
// bubble means the tap lands on whatever is underneath, which is exactly
// "outside"), Escape and scroll (the hook's own listeners), and blur where
// the browser bothers to move focus (iOS does not blur on taps into plain
// content, which is why the effect exists at all). Re-tap on the (i) itself
// deliberately does NOT close — it re-opens an already-open bubble, which is
// the same no-op it looks like; the (i) span is not a popover with a
// dismissal layer, and the hook has no toggle to borrow without re-deriving
// one (the anti-pattern this whole arrangement avoids).
//
// The 44px touch target rides the SAME coarse-pointer gate (a >=44px box is
// a touch-floor concern, not a width concern — a coarse-pointer tablet at
// any width gets the finger-sized target), swapping the 15px inline box for
// an h-11/w-11 one, icon centred inside.
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
  // Destructured once so the effects below can list exactly what they read
  // (the hook returns a fresh object literal every render; naming the pieces
  // keeps the dependency arrays truthful).
  const { ref: hookRef, handlers, describedBy, bubble, show, hide } = useTipBubble(tip);
  const coarse = useIsCoarsePointer();
  // The hook's own ref is a callback that only stores the element internally;
  // the outside-tap dismissal below needs to read the trigger element too, so
  // a local ref rides along in the same callback.
  const spanRef = useRef<HTMLElement | null>(null);
  const setSpanRef = (el: HTMLElement | null) => {
    spanRef.current = el;
    hookRef(el);
  };

  // While the bubble is showing on a coarse pointer, a tap anywhere outside
  // the (i) closes it. pointerdown (not click) because it fires before the
  // browser synthesizes the rest of the tap sequence, and because the bubble
  // itself is pointer-events:none — the tap lands on the content underneath,
  // which is the definition of "outside" here. pointerdown on the (i) or its
  // glyph is "inside" and does nothing (see the tap-open note above for why
  // re-tap is not a close).
  const tapOpen = describedBy !== undefined;
  useEffect(() => {
    if (!coarse || !tapOpen) return;
    function onPointerDown(e: PointerEvent) {
      const el = spanRef.current;
      if (el && e.target instanceof Node && !el.contains(e.target)) hide();
    }
    document.addEventListener("pointerdown", onPointerDown);
    return () => document.removeEventListener("pointerdown", onPointerDown);
  }, [coarse, tapOpen, hide]);

  return (
    <>
      {/* bv-convention-exception: one-icon-badge-size -- the 44px box below is
          the mandated touch tap target (the same
          exception the touch chevron and the sheet close button carry), not a
          square icon badge — it is an inline (i) trigger, not a Badge, and the
          15px glyph stays 15px inside it. */}
      <span
        ref={setSpanRef}
        aria-label={tip}
        aria-describedby={describedBy}
        tabIndex={0}
        {...handlers}
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
        onClick={(e) => {
          // The <label>-forwarding fix documented above — stays on every path.
          e.preventDefault();
          // Tap-open (coarse pointer only — desktop clicks keep the no-op
          // default and the hover/focus handlers own everything): OPEN-ONLY,
          // never a toggle, for the Android focus-before-click ordering
          // documented in the header. iOS Safari does not focus a plain
          // tabindex span on tap, which is why click — not focus alone — has
          // to carry the open here.
          if (coarse && describedBy === undefined) show();
        }}
        className={`inline-flex ${
          // Touch floor (44px) on coarse pointers, the original 15px box
          // otherwise — desktop call sites keep their exact footprint.
          coarse ? "h-11 w-11" : "h-[15px] w-[15px]"
        } flex-none cursor-help items-center justify-center rounded-pill focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--focus-ring) ${
          onAccent ? "text-current" : "text-carbon-textMuted opacity-80 hover:opacity-100 focus:opacity-100"
        }`}
      >
        <svg viewBox="0 0 16 16" width="15" height="15" fill="none" aria-hidden="true">
          <circle cx="8" cy="8" r="7" stroke="currentColor" strokeWidth="1.3" />
          <circle cx="8" cy="4.6" r="0.9" fill="currentColor" />
          <path d="M8 7v4.4" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" />
        </svg>
      </span>
      {bubble}
    </>
  );
}
