import type { MouseEvent } from "react";
import { IconCheck } from "../navGlyphs";

/** The round mark by itself, for a row that is the control. */
export function SelectDot({ checked, onAccent = false, className = "" }: { checked: boolean; onAccent?: boolean; className?: string }) {
  const ground = onAccent
    ? checked
      ? "bg-accentContrast text-accentText"
      : "bg-black/20 text-transparent"
    : checked
      ? "bg-accent text-accentContrast"
      : "bg-carbon-surface3 text-transparent";
  return (
    <span
      aria-hidden="true"
      className={`flex h-[18px] w-[18px] flex-none items-center justify-center rounded-pill transition-colors [&_svg]:h-2.5 [&_svg]:w-2.5 ${ground} ${className}`}
    >
      <IconCheck />
    </span>
  );
}

/**
 * SelectMark is the mark that picks a row, or every row where it stands in
 * the bar above the list. To a screen reader it is a checkbox; to the eye it
 * is part of the row and not a form field.
 */
export function SelectMark({
  checked,
  label,
  onChange,
  onAccent = false,
  className = "",
}: {
  checked: boolean;
  label: string;
  onChange: (next: boolean) => void;
  /** Standing on a filled accent surface, which needs the colours turned. */
  onAccent?: boolean;
  className?: string;
}) {
  function toggle(e: MouseEvent) {
    e.stopPropagation();
    onChange(!checked);
  }

  return (
    <button
      type="button"
      role="checkbox"
      aria-checked={checked}
      aria-label={label}
      onClick={toggle}
      className={`group -m-2 flex flex-none cursor-pointer items-center justify-center rounded-pill p-2 focus-visible:outline-none ${className}`}
    >
      <SelectDot
        checked={checked}
        onAccent={onAccent}
        className="group-focus-visible:outline-solid group-focus-visible:outline-2 group-focus-visible:outline-offset-2 group-focus-visible:outline-(--focus-ring)"
      />
    </button>
  );
}
