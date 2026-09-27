import type { CSSProperties, ReactNode } from "react";
import { InfoBubble } from "./InfoBubble";

// HintSlot puts a control's (i) inside the control's box but not inside its
// <button>. Nested, it would be an interactive element inside another, a press
// on it would press the button, and a disabled button takes no hover, which
// would silence the explanation in the state that most needs it. Laid over a
// space the control keeps free at its end, the (i) keeps its own tab stop and
// its full strength while the control is dimmed.

/** The empty end of a control that HintSlot lays its (i) over, as wide as the
 *  (i). The control renders it as its last child. */
export function HintSpace() {
  return <span aria-hidden="true" className="w-[15px] flex-none" />;
}

export function HintSlot({
  tip,
  ink,
  end,
  hue,
  children,
}: {
  tip: string;
  /** The control's text colour. The (i) is a sibling of the control, so it
   *  cannot inherit it. */
  ink: string;
  /** The control's inline end padding, which the (i) sits inside. */
  end: string;
  /** The control's palette variables, so the (i) and its focus ring take the
   *  same position as the control. */
  hue?: CSSProperties;
  children: ReactNode;
}) {
  return (
    // w-fit keeps a flex column from stretching the box, which would carry the
    // (i) away from the control to the far edge of the row.
    <span className={`relative inline-flex w-fit max-w-full${hue ? " glim-hue" : ""}`} style={hue}>
      {children}
      <span
        className={`pointer-events-none absolute inset-y-0 flex items-center ${ink}`}
        style={{ insetInlineEnd: end }}
      >
        <span className="pointer-events-auto inline-flex">
          <InfoBubble tip={tip} onAccent />
        </span>
      </span>
    </span>
  );
}
