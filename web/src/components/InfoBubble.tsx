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
export function InfoBubble({ text }: { text: string }) {
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
        aria-label={text}
        aria-describedby={open ? tooltipId : undefined}
        tabIndex={0}
        onMouseEnter={show}
        onMouseLeave={hide}
        onFocus={show}
        onBlur={hide}
        className="inline-flex h-[15px] w-[15px] flex-none cursor-help items-center justify-center text-carbon-textMuted opacity-80 hover:opacity-100 focus:opacity-100 focus:outline-none"
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
            style={{
              position: "fixed",
              left: pos.left,
              top: pos.top,
              transform: "translateX(-50%)",
              pointerEvents: "none",
            }}
            className="z-50 w-64 max-w-[80vw] rounded-lg bg-carbon-surface3 px-2.5 py-1.5 text-xs leading-relaxed text-carbon-text shadow-lg"
          >
            {text}
          </div>,
          document.body
        )}
    </>
  );
}
