import { IconCheck } from "./navGlyphs";

// Checkbox is the box for the places a box stands instead of a switch (rule
// 39), such as "Don't ask again" under a question. Every setting is a Toggle.
export function Checkbox({
  checked,
  onChange,
  label,
  disabled,
  className = "",
}: {
  checked: boolean;
  /** Called with the flipped value when the box is activated. */
  onChange: (next: boolean) => void;
  /** Shown beside the box and its accessible name. */
  label: string;
  disabled?: boolean;
  className?: string;
}) {
  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={`glim-checkbox ${className}`.trim()}
    >
      <span className="glim-checkbox-box" aria-hidden="true">
        <IconCheck />
      </span>
      <span>{label}</span>
    </button>
  );
}
