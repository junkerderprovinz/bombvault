import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";

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
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ left: number; top: number } | null>(null);
  const iconRef = useRef<HTMLSpanElement>(null);
  const tooltipId = useId();

  function show() {
    const r = iconRef.current?.getBoundingClientRect();
    if (!r) return;
    setPos({ left: r.left + r.width / 2, top: r.bottom + 6 });
    setOpen(true);
  }
  function hide() {
    setOpen(false);
  }

  useEffect(() => {
    if (!open) return;
    const onScroll = () => hide();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") hide();
    };
    window.addEventListener("scroll", onScroll, true);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("scroll", onScroll, true);
      window.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <>
      <span
        ref={iconRef}
        aria-label={tip}
        aria-describedby={open ? tooltipId : undefined}
        tabIndex={0}
        onMouseEnter={show}
        onMouseLeave={hide}
        onFocus={show}
        onBlur={hide}
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
      {open &&
        pos &&
        createPortal(
          <div
            role="tooltip"
            id={tooltipId}
            style={{ left: pos.left, top: pos.top }}
            className="glim-bubble glim-fade"
          >
            {tip}
          </div>,
          document.body
        )}
    </>
  );
}
