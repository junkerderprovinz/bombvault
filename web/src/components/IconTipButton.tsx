import type { CSSProperties, ReactNode } from "react";
import { useTipBubble } from "../lib/useTipBubble";

// IconTipButton is an icon-only <button> whose required tip is both its
// accessible name and a `.glim-bubble` tooltip, in place of a native `title=`.
// Unlike Button it has no width stage, tone or label mode: it is a bare glyph
// in a box the caller sizes.
export function IconTipButton({
  tip,
  onClick,
  disabled,
  className,
  style,
  children,
  type = "button",
  ariaPressed,
  ariaExpanded,
}: {
  /** Tooltip text and the button's accessible name. */
  tip: string;
  onClick?: () => void;
  disabled?: boolean;
  className?: string;
  /** `aria-pressed` for a toggle that stays on between clicks. */
  ariaPressed?: boolean;
  /** `aria-expanded` for a trigger that opens a panel below it. */
  ariaExpanded?: boolean;
  /** For Badge's `tip` branch, which sets its `--item-hue*` properties
   *  inline. */
  style?: CSSProperties;
  children: ReactNode;
  type?: "button" | "submit";
}) {
  const tooltip = useTipBubble(tip, disabled);

  return (
    <>
      {tooltip.wrap(
        <button
          ref={tooltip.ref}
          type={type}
          onClick={onClick}
          disabled={disabled}
          aria-label={tip}
          aria-pressed={ariaPressed}
          aria-expanded={ariaExpanded}
          aria-describedby={tooltip.describedBy}
          {...tooltip.handlers}
          className={className}
          style={style}
        >
          {children}
        </button>,
      )}
      {tooltip.bubble}
    </>
  );
}
