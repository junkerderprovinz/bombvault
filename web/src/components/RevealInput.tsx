import type { InputHTMLAttributes } from "react";

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
        className={`w-full pr-8! rtl:pr-0! rtl:pl-8! text-start${className ? ` ${className}` : ""}`}
      />
      <button
        type="button"
        onClick={onToggleVisible}
        aria-label={visible ? hideLabel : showLabel}
        aria-pressed={visible}
        className="absolute right-2 rtl:right-auto! rtl:left-2 top-1/2 -translate-y-1/2 inline-flex h-[15px] w-[15px] items-center justify-center rounded-pill text-carbon-textMuted opacity-80 hover:opacity-100 focus-visible:opacity-100 focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--focus-ring)"
      >
        {visible ? (
          // The eye is symmetric, so it is not mirrored under RTL. The pupil is
          // cut out in the field's surface colour.
          <svg viewBox="0 0 16 16" width="15" height="15" fill="currentColor" aria-hidden="true">
            <g opacity="0.55">
              <path d="M1 8C1 8 3.8 3.6 8 3.6S15 8 15 8 12.2 12.4 8 12.4 1 8 1 8Z" />
              <circle cx="8" cy="8" r="2.1" fill="var(--carbon-surface2, transparent)" />
            </g>
            <rect x="-0.5" y="7.2" width="17" height="1.6" rx="0.8" transform="rotate(45 8 8)" />
          </svg>
        ) : (
          <svg viewBox="0 0 16 16" width="15" height="15" fill="currentColor" aria-hidden="true">
            <path d="M1 8C1 8 3.8 3.6 8 3.6S15 8 15 8 12.2 12.4 8 12.4 1 8 1 8Z" />
            <circle cx="8" cy="8" r="2.1" fill="var(--carbon-surface2, transparent)" />
          </svg>
        )}
      </button>
    </div>
  );
}
