import { useTipBubble } from "../lib/useTipBubble";

// InfoBubble is a neutral (i) icon that shows a short help text on hover and
// focus, so an explanation sits beside its label instead of in a permanent
// paragraph under the control. It is never accent-coloured, because the accent
// means active.
//
// `onAccent` is for an icon inside a heading badge on the accent fill: it
// inherits the badge's --accent-contrast ink and keeps full opacity.
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
        // Inside a <label>, a click would also focus the label's input and
        // close the tooltip. The forwarding is native, so only preventDefault
        // stops it; focus still lands here because browsers assign it on
        // mousedown.
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
