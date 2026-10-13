import type { MouseEvent } from "react";
import { IconCheck } from "../navGlyphs";

/** The round mark by itself, for a row that is the control. A mixed mark
 *  stands for a folder of which only some entries are picked. An offered
 *  mark is the empty ring in the row's colour that the pointer brings up
 *  before anything is picked. */
export function SelectDot({
  checked,
  mixed = false,
  onAccent = false,
  offered = false,
  className = "",
}: {
  checked: boolean;
  mixed?: boolean;
  onAccent?: boolean;
  offered?: boolean;
  className?: string;
}) {
  const on = checked || mixed;
  const ground = onAccent
    ? on
      ? "bg-accentContrast text-accentText"
      : "bg-black/20 text-transparent"
    : on
      ? "bg-accent text-accentContrast"
      : offered
        ? "bg-carbon-hover text-transparent shadow-[inset_0_0_0_2px_var(--accent)] group-hover/mark:text-(--accent)"
        : "bg-carbon-surface3 text-transparent";
  return (
    <span
      aria-hidden="true"
      className={`flex h-[18px] w-[18px] flex-none items-center justify-center rounded-full transition-colors [&_svg]:h-2.5 [&_svg]:w-2.5 ${ground} ${className}`}
    >
      {mixed && !checked ? <span className="h-0.5 w-2 rounded-full bg-current" /> : <IconCheck />}
    </span>
  );
}

/**
 * SelectMark is the mark that picks a row, or every row where it stands in
 * the bar above the list. To a screen reader it is a checkbox; to the eye it
 * is a disc in every shape, part of the row and not a form field.
 */
export function SelectMark({
  checked,
  label,
  onChange,
  onAccent = false,
  offered = false,
  bare = false,
  className = "",
}: {
  checked: boolean;
  label: string;
  onChange: (next: boolean) => void;
  /** Standing on a filled accent surface, which needs the colours turned. */
  onAccent?: boolean;
  /** Shown under the pointer before anything is picked; see SelectDot. */
  offered?: boolean;
  /** Without the room around the disc that makes it easier to hit, for a
   *  mark that lies on something else that takes clicks. */
  bare?: boolean;
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
      className={`group/mark flex flex-none cursor-pointer items-center justify-center rounded-pill focus-visible:outline-none ${bare ? "" : "-m-2 p-2"} ${className}`}
    >
      <SelectDot
        checked={checked}
        onAccent={onAccent}
        offered={offered}
        className="group-focus-visible/mark:outline-solid group-focus-visible/mark:outline-2 group-focus-visible/mark:outline-offset-2 group-focus-visible/mark:outline-(--focus-ring)"
      />
    </button>
  );
}
