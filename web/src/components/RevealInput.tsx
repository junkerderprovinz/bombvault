import type { InputHTMLAttributes } from "react";

import { IconEye, IconEyeOff } from "./glyphs";

// RevealInput is the password field with a show/hide eye that every secret in
// the app renders through. It holds no state (useReveal does), so tests can
// call it as a plain function.
//
// The input is always dir="ltr": keys and tokens are technical data and must
// not be reordered on an RTL page.
//
// The eye and the padding reserved for it both use physical properties behind
// the rtl: variant. A logical pe-8 would resolve against the input's own
// forced ltr, and end-2 against the nearest dir ancestor, which is not always
// the page (OffsiteWizard puts this field inside a dir="ltr" label). rtl:
// matches on the page, so both halves land on the same side.
//
// The padding utilities carry ! because callers pass shared class strings with
// their own px-*, and Tailwind's output order, not the class order, decides
// which one wins. rtl:right-auto! needs it for the same reason against right-2.
//
// Under a coarse pointer the 16px eye is too small to hit, so its button takes
// the field's full height and a 44px strip at the end, and the padding widens
// to match. The glyph stays the same size.

export interface RevealInputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "type"> {
  /** visible, onToggleVisible, showLabel and hideLabel come from useReveal(). */
  visible: boolean;
  onToggleVisible: () => void;
  /** Accessible name of the eye while the value is hidden. */
  showLabel: string;
  /** Accessible name of the eye while the value is shown. */
  hideLabel: string;
  /** Layout classes such as w-full or flex-1 min-w-0. They go on the wrapper,
   *  and the input always fills it. */
  wrapperClassName?: string;
}

export function RevealInput({
  visible,
  onToggleVisible,
  showLabel,
  hideLabel,
  wrapperClassName,
  className,
  ...rest
}: RevealInputProps) {
  return (
    <div className={`relative${wrapperClassName ? ` ${wrapperClassName}` : ""}`}>
      <input
        {...rest}
        type={visible ? "text" : "password"}
        dir="ltr"
        className={`w-full pr-8! rtl:pr-0! rtl:pl-8! pointer-coarse:pr-11! rtl:pointer-coarse:pr-0! rtl:pointer-coarse:pl-11! text-start${className ? ` ${className}` : ""}`}
      />
      {/* bv-convention-exception: one-icon-badge-size: the reveal eye is a bare
          glyph inside the field, never a badge, so it keeps the glyph's own size. */}
      <button
        type="button"
        onClick={onToggleVisible}
        aria-label={visible ? hideLabel : showLabel}
        aria-pressed={visible}
        className="absolute right-2 rtl:right-auto! rtl:left-2 top-1/2 -translate-y-1/2 inline-flex h-4 w-4 pointer-coarse:h-full pointer-coarse:w-11 pointer-coarse:right-0 rtl:pointer-coarse:left-0! items-center justify-center rounded-pill text-carbon-textMuted opacity-80 hover:opacity-100 focus-visible:opacity-100 focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--focus-ring)"
      >
        {/* The eye is symmetric, so it is not mirrored under RTL. */}
        {visible ? <IconEyeOff /> : <IconEye />}
      </button>
    </div>
  );
}
