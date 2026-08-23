// ---------------------------------------------------------------------------
// Toggle — the one shared switch control (GlimStone form-engine Task 4).
//
// Replaces three previously-separate copies of the same track+thumb button:
// IncludeToggle's inline switch, Settings.tsx's ToggleRow switch (reused by
// Config.tsx and Recovery.tsx), and OffsiteTargetsSection's inline fourth
// copy. All three drift risk is now in one place.
//
// The focus ring is `--focus-ring`, NOT the `outline-statusInfoSolid` blue every
// one of those three copies carried. Two reasons, both only visible ACROSS tasks:
//   - Task 1's own plan note about `--status-info-solid` — whose hue was then
//     still deliberately unresolved (audit item 19, deferred to Phase 2) — was
//     "just don't add MORE dependencies on it." A new shared component is
//     precisely a new dependency, and the single point through which a later
//     hue change would reach every switch in the app at once. Phase 2 Task 7
//     has since resolved it: `--status-info-solid` no longer exists at all
//     (see index.css's TASK 7 comment for what every real call site became) —
//     this comment stays as the historical reason THIS component never took
//     the dependency in the first place, not a forward-looking warning anymore.
//   - `--focus-ring` is already what the other shared controls this branch added
//     or reworked use: RevealInput's eye, InfoBubble's icon, Toast's dismiss X.
//     A switch focusing blue while the reveal eye in the same form focuses amber
//     reads as two unrelated systems rather than one.
// `outline-offset-2` keeps the ring on the surrounding card surface rather than
// on the track's own fill — that surface is the background `--focus-ring`'s
// contrast was actually measured against (see index.css; ≥3:1 on both
// --carbon-surface and --carbon-surface2, in both themes).
//
// `hideLabel` is the GlimStone "Switches" contract verbatim: the text always
// survives as the control's accessible name (`aria-label`, set unconditionally
// so a switch is never nameless) — hideLabel only decides whether the eye
// ALSO sees it as visible text next to the track. There is no "indent" prop:
// per the design language's "flush, no indent" rule, indentation of a
// sub-switch is a caller-side layout concern (padding on the wrapping
// container), never something the switch itself renders.
// ---------------------------------------------------------------------------

export interface ToggleProps {
  /** Current on/off state. */
  checked: boolean;
  /** Called with the flipped value when the switch is activated. */
  onChange: (next: boolean) => void;
  /** Always used as the accessible name; shown as visible text unless hideLabel. */
  label: string;
  /** Suppress the visible caption — text survives as aria-label. Use ONLY
   *  when the CALLER, right there in its own layout, already draws this
   *  exact text as real visible content next to this switch (e.g. a
   *  hand-rolled row like Containers.tsx's UpdateAfterBackupRow, or a
   *  wrapping `<label>` + sibling `<span>` like VMs.tsx's VMIncludeToggle) —
   *  never to rely on a Card's title/hint further up the page to justify an
   *  otherwise-blank row. That specific reasoning ("the Card title already
   *  says it") is the exact anti-pattern jdp ruled out categorically after
   *  it got built and reversed three times on ToggleRow (design-language.md
   *  Toggles/Switches section) — ToggleRow has no `hideLabel` prop at all
   *  anymore as a result. This lower-level prop stays only for the genuinely
   *  different "the caller itself already prints the same text" shape. */
  hideLabel?: boolean;
  disabled?: boolean;
  /** Extra classes for the outer wrapper (e.g. row-alignment nudges). */
  className?: string;
}

export function Toggle({ checked, onChange, label, hideLabel = false, disabled, className }: ToggleProps) {
  return (
    <span className={`inline-flex items-center gap-2${className ? ` ${className}` : ""}`}>
      {!hideLabel && <span className="text-sm text-carbon-text">{label}</span>}
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        aria-label={label}
        disabled={disabled}
        onClick={() => onChange(!checked)}
        title={label}
        className={`relative inline-flex h-5 w-9 shrink-0 items-center rounded-pill transition-colors focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--focus-ring) disabled:opacity-50 ${
          checked ? "bg-accent" : "bg-carbon-surface3"
        }`}
      >
        <span
          // `translate-x` is a PHYSICAL transform — always a rightward pixel
          // shift for a positive value, regardless of `direction` (there is
          // no logical "translate toward the inline-end" primitive Tailwind
          // maps this onto). The track's flex layout DOES auto-mirror under
          // RTL (flex main-start follows `direction`), so the thumb's
          // untransformed rest position already flips to the right edge —
          // but the physical translate then pushes it EVEN FURTHER right on
          // top of that, landing the "on" state's thumb outside the track
          // entirely instead of at its mirrored left edge. `rtl:` negates the
          // sign so the shift lands on the correct side of the now-mirrored
          // base position either way (RTL sweep, form-engine Phase 2 Task 6
          // follow-up fix). The `!` on the override isn't stylistic — both
          // classes set the same single `translate` property with equal
          // selector specificity, so without it the winner would depend on
          // Tailwind's generated declaration order rather than being
          // guaranteed by the cascade.
          //
          // INVARIANT this relies on: the `rtl:` variant resolves against the
          // PAGE (its compiled selector is an inherited-`:lang()` list OR
          // `[dir=rtl] *`), while the flex base position it corrects resolves
          // against the nearest `dir` ANCESTOR. Those agree everywhere today,
          // but they would diverge if a call site ever nested a Toggle inside
          // its own `dir="ltr"`/`dir="rtl"` island — the base would mirror one
          // way and the sign flip the other, putting the thumb outside its
          // track again. RevealInput hit exactly that (see its header comment)
          // via OffsiteWizard's `<label dir="ltr">`; there the fix was to make
          // BOTH halves page-gated. Here there is nothing to make page-gated —
          // flex direction is not overridable per-property — so the rule is
          // simply: do not put a Toggle inside a dir-overriding container.
          // Pin only the technical TEXT with `dir="ltr"` (a `<span>` around
          // the literal), never a container that also holds layout.
          //
          // rounded-pill (shape engine follow-up, live-review point 2 —
          // "the knob should ALSO adapt, not stay a permanent circle"): reads
          // the exact same `--radius-pill` custom property the OUTER track
          // above already keys its own `rounded-pill` off, not a separate,
          // size-tuned token. Two reasons this is the right token rather than
          // `--radius-control` (index.css's other candidate, used by cards/
          // buttons/fields): (1) consistency — the track and its own thumb
          // reading the same variable is what makes them read as one control
          // reshaping together, not two independently-tuned pieces that
          // happen to move in the same direction; (2) the actual numbers only
          // work out right with --radius-pill. At shape="round", --radius-pill
          // is 9999px — on this 14px (h-3.5 w-3.5) box that still resolves to
          // a perfect circle (any radius ≥ half the box side does), matching
          // "round shape → circular knob (current, unchanged)" exactly.
          // --radius-control at "round" is only 0.625rem (10px), which on a
          // 14px box would render a rounded SQUARE, not a circle — it would
          // silently break the one shape this change must leave alone. At
          // "soft" (--radius-pill: 0.3125rem / 5px) this reads as a gently
          // rounded square knob; at "square" (--radius-pill: 0) it's sharp
          // corners — both exactly the ask.
          className={`inline-block h-3.5 w-3.5 rounded-pill bg-carbon-background transition-transform ${
            checked ? "translate-x-[18px] rtl:-translate-x-[18px]!" : "translate-x-[3px] rtl:-translate-x-[3px]!"
          }`}
        />
      </button>
    </span>
  );
}
