import type { InputHTMLAttributes } from "react";

// ---------------------------------------------------------------------------
// RevealInput — the GlimStone "reveal eye" affordance (form-engine Task 6).
//
// Every secret/token field in the app (login password, the cloud-credential
// secrets, the fleet/receiver/recovery paste-a-key fields, the show-once
// tokens) renders through this ONE component instead of a bare
// <input type="password">, so the eye can't drift site-to-site the way
// Toggle/Badge's duplicate predecessors did.
//
// Pure, hookless function component on purpose — same shape as Toggle.tsx/
// Badge.tsx (props in, an element tree out) — so it stays unit-testable by
// calling it directly with props, no renderer/jsdom needed (this repo's test
// suite is `environment: "node"`, zero DOM-rendering infra — see
// Toggle.test.ts's header comment). The show/hide STATE lives in the
// `useReveal` hook (lib/useReveal.ts) instead of inside this component for
// exactly that reason: a stateful `useState` here would make RevealInput
// un-callable as a plain function outside React's renderer. A call site
// does `const reveal = useReveal(); <RevealInput {...reveal} .../>`.
//
// Design-language contract (docs/design-language.md, "The reveal eye"):
//   - Bare icon inside the field's own trailing padding, not a chrome button
//     beside it. BombVault serves its own SPA (web/index.html mounts #root
//     directly — no foreign host DOM, confirmed against the Dockerfile/
//     README: single Go binary serving its own embedded React app on its
//     own port, never injected into another app's page), so nothing repaints
//     a plain <button> with host chrome. This renders a real <button>, not
//     the `<span role="button" tabindex="0">` workaround the spec reserves
//     for a page embedded in a foreign host UI's global button styling.
//   - Neutral colour, never the accent: `text-carbon-textMuted` + an
//     opacity-based hover/focus step, the exact treatment InfoBubble.tsx
//     uses for its own (i) icon (same precedent, "furniture, not activity").
//   - Doesn't change the field's width: `wrapperClassName` takes over
//     whatever layout classes (`w-full`, `flex-1 min-w-0`, …) the bare
//     <input> used to carry at each call site; the input itself always gets
//     an unconditional `w-full` so it fills that wrapper exactly like the
//     original bare input filled ITS parent — swapping <input> for
//     <RevealInput> never shrinks or grows the field's own footprint.
//   - Reserves trailing room with `pe-8!`. Several call sites build their
//     className from a shared `inputCls`/`offsiteInput` const also reused by
//     OTHER, non-secret fields in the same file/function; appending a plain
//     `pe-8` after that constant in the className string is NOT guaranteed
//     to win the cascade against the constant's own `px-*` (Tailwind's
//     generated utility order is not the order classes appear in the
//     `class` attribute). The `!` important modifier pins the override
//     without having to fork or string-edit the shared constant.
//   - `pe-8`/`end-2` (LOGICAL, writing-mode-aware Tailwind utilities), not
//     `pr-8`/`right-2` — the eye sits on the field's TRAILING edge, which is
//     the right in LTR (English, German, ...) but the LEFT in RTL (Arabic,
//     Hebrew — both shipped locales here, see lib/i18n.ts's isRtl). Physical
//     `right-*` would pin the eye to the same visual side in both directions,
//     landing on the RTL reader's leading edge instead. `dir` flips on
//     document.documentElement per the active locale (lib/i18n.ts), so the
//     logical utilities resolve correctly with no JS/conditional needed.
// ---------------------------------------------------------------------------

export interface RevealInputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "type"> {
  /** Current reveal state — from useReveal(). */
  visible: boolean;
  /** Flips `visible` — from useReveal(). */
  onToggleVisible: () => void;
  /** Accessible name while hidden (the eye's action is "show"). From
   *  useReveal(), which already resolves it via t("common.showValue"). */
  showLabel: string;
  /** Accessible name while visible (the eye's action is "hide"). From
   *  useReveal(), via t("common.hideValue"). */
  hideLabel: string;
  /** Classes for the outer positioning wrapper — pass through whatever
   *  layout classes (`w-full`, `flex-1 min-w-0`, …) used to sit on the bare
   *  <input> here, not on `className`, so the field's footprint in its own
   *  row/grid/flex parent is unchanged by adopting the eye. */
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
        className={`w-full pe-8!${className ? ` ${className}` : ""}`}
      />
      <button
        type="button"
        onClick={onToggleVisible}
        aria-label={visible ? hideLabel : showLabel}
        aria-pressed={visible}
        className="absolute end-2 top-1/2 -translate-y-1/2 inline-flex h-[15px] w-[15px] items-center justify-center rounded-pill text-carbon-textMuted opacity-80 hover:opacity-100 focus-visible:opacity-100 focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--focus-ring)"
      >
        {visible ? (
          // Slashed eye: the same open-eye glyph, dimmed, struck through —
          // never mirrored/rotated for RTL (design-language.md's RTL section:
          // a symmetric icon with no inherent reading direction, like the
          // reveal eye it names explicitly, never gets mirrored).
          <svg viewBox="0 0 16 16" width="15" height="15" fill="none" aria-hidden="true">
            <path
              d="M1 8C1 8 3.8 3.6 8 3.6S15 8 15 8 12.2 12.4 8 12.4 1 8 1 8Z"
              stroke="currentColor"
              strokeWidth="1.3"
              strokeLinejoin="round"
              opacity="0.55"
            />
            <circle cx="8" cy="8" r="2.1" stroke="currentColor" strokeWidth="1.3" opacity="0.55" />
            <path d="M2.3 2.3L13.7 13.7" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" />
          </svg>
        ) : (
          <svg viewBox="0 0 16 16" width="15" height="15" fill="none" aria-hidden="true">
            <path
              d="M1 8C1 8 3.8 3.6 8 3.6S15 8 15 8 12.2 12.4 8 12.4 1 8 1 8Z"
              stroke="currentColor"
              strokeWidth="1.3"
              strokeLinejoin="round"
            />
            <circle cx="8" cy="8" r="2.1" stroke="currentColor" strokeWidth="1.3" />
          </svg>
        )}
      </button>
    </div>
  );
}
