// ---------------------------------------------------------------------------
// Toggle — the one shared switch control (GlimStone form-engine Task 4).
//
// Replaces three previously-separate copies of the same track+thumb button:
// IncludeToggle's inline switch, Settings.tsx's ToggleRow switch (reused by
// Config.tsx and Recovery.tsx), and OffsiteTargetsSection's inline fourth
// copy. All three drift risk is now in one place.
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
  /** Suppress the visible caption — text survives as aria-label. Use when a
   *  caller already renders this same text elsewhere (a Card title, a row's
   *  own label block), so the caption never appears twice. */
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
        className={`relative inline-flex h-5 w-9 shrink-0 items-center rounded-pill transition-colors focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-statusInfoSolid disabled:opacity-50 ${
          checked ? "bg-accent" : "bg-carbon-surface3"
        }`}
      >
        <span
          className={`inline-block h-3.5 w-3.5 rounded-full bg-carbon-background transition-transform ${
            checked ? "translate-x-[18px]" : "translate-x-[3px]"
          }`}
        />
      </button>
    </span>
  );
}
