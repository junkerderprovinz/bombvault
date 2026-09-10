import { useId, useRef, useState, type ReactNode } from "react";
import { DropdownListbox } from "./DropdownListbox";
import { stepIndex } from "../lib/selectScroll";

// ---------------------------------------------------------------------------
// SelectField — the app's own replacement for a native <select> (#3425).
//
// GlimStone rule 18: a native control gets replaced, not persuaded. An open
// native <select> is drawn by the operating system, so no rule in this house
// reaches inside it: not the shape engine, not the palette, not the type scale.
// "Closed, it looks on-brand" is a half-finished rebuild and reads as one.
//
// This app already had the PANEL (DropdownListbox, portalled, positioned,
// dismissed, keyboard-navigable) and used it twice. What it did not have was
// the whole CONTROL, so every other picker stayed native: converting one meant
// hand-rolling a trigger, an open/close state, a wheel handler and a list of
// option buttons, which is 30 lines of markup nobody writes for a filter bar.
// That is why twenty-one of them were still native. This is the missing half,
// so a call site is a field again rather than a construction kit.
//
// What it owns, and what makes it more than markup:
//
//   1. THE WIDTH DOES NOT MOVE. A native select is as wide as its widest
//      option; a button is as wide as its current label, so a plain conversion
//      makes the control resize whenever the value changes, and a row of them
//      reflows on every pick. Every label is rendered stacked in one grid cell
//      and all but the current one are `invisible`, so the intrinsic width is
//      the widest label in the CURRENT language and stays put.
//   2. THE ARROW IS THE HOUSE'S. Rule 14: whoever sets `appearance: none` owes
//      the arrow. It is the same solid rounded triangle NumberField's steppers
//      draw, not a chevron from somewhere else.
//   3. THE WHEEL WORKS ON THE CLOSED CONTROL (rule 14's addendum), clamped at
//      both ends and skipping disabled options.
//
// It deliberately does NOT invent a size taxonomy: the trigger takes the same
// `className` its <select> carried, so a filter bar stays small and a settings
// field stays field-sized, and this component never becomes the place where
// those two drift apart.
// ---------------------------------------------------------------------------

export interface SelectOption<T extends string> {
  value: T;
  label: string;
  /** Optional mark before the label, shown in the trigger and in the list. */
  glyph?: ReactNode;
  disabled?: boolean;
}

/** The house's downward arrow: NumberField's own stepper triangle, which is a
 *  filled shape with rounded corners rather than a two-stroke chevron (an icon
 *  set is filled shapes, and a chevron is the one outline in the interface). */
function Arrow() {
  return (
    <svg viewBox="0 0 10 6" width="10" height="6" aria-hidden="true" className="shrink-0 opacity-70">
      {/* bv-convention-exception: user-message-is-translated -- SVG path data,
          not prose. */}
      <path d="M5 4.3 L1.4 1.4 L8.6 1.4 Z" fill="currentColor" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" />
    </svg>
  );
}

export function SelectField<T extends string>({
  value,
  onChange,
  options,
  label,
  disabled = false,
  className = "",
  id,
}: {
  value: T;
  onChange: (next: T) => void;
  options: SelectOption<T>[];
  /** The control's accessible name — what a `<label>` or `aria-label` said on
   *  the `<select>` this replaces. A picker without one announces as an
   *  unlabelled button, which is the one thing this conversion must not cost. */
  label: string;
  disabled?: boolean;
  /** The trigger's own look, taken over verbatim from the field it replaces. */
  className?: string;
  /** Forwarded to the trigger, for a visible `<label htmlFor>` that already
   *  points at this control. */
  id?: string;
}) {
  const [open, setOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const listId = useId();
  const current = options.find((o) => o.value === value);

  // A wheel notch moves to the next option that can actually be picked, so a
  // disabled entry is stepped OVER rather than landed on and silently ignored.
  function step(delta: 1 | -1) {
    if (disabled || options.length < 2) return;
    let at = options.findIndex((o) => o.value === value);
    if (at < 0) at = 0;
    let next = at;
    for (;;) {
      const candidate = stepIndex(options.length, next, delta);
      if (candidate === next) return; // clamped at an end
      next = candidate;
      if (!options[next]?.disabled) break;
    }
    const picked = options[next];
    if (picked && picked.value !== value) onChange(picked.value);
  }

  return (
    <>
      <button
        ref={triggerRef}
        id={id}
        type="button"
        disabled={disabled}
        // The ARIA select-only combobox: a trigger that owns a listbox, which
        // is what a <select> announces as. Replacing a native control is
        // exactly where an app quietly changes what a screen reader says about
        // it, and "button" would have.
        role="combobox"
        aria-label={label}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listId : undefined}
        onClick={() => setOpen((v) => !v)}
        className={`inline-flex items-center gap-2 text-start ${className}`}
      >
        {current?.glyph}
        {/* One grid cell, every label stacked in it: the button is as wide as
            the widest option, exactly like the select it replaces, and picking
            a shorter value does not shrink it under the pointer. */}
        <span className="grid min-w-0 flex-1">
          {options.map((o) => (
            <span
              key={o.value}
              aria-hidden={o.value !== value}
              className={`col-start-1 row-start-1 truncate ${o.value === value ? "" : "invisible"}`}
            >
              {o.label}
            </span>
          ))}
        </span>
        <Arrow />
      </button>
      <DropdownListbox
        open={open}
        onClose={() => setOpen(false)}
        triggerRef={triggerRef}
        label={label}
        wheelStep={step}
      >
        {options.map((o) => (
          <button
            key={o.value}
            type="button"
            role="option"
            aria-selected={o.value === value}
            disabled={o.disabled}
            onClick={() => {
              onChange(o.value);
              setOpen(false);
            }}
            className={`flex w-full items-center gap-2.5 px-3 py-2 text-start text-sm transition-colors disabled:opacity-50 ${
              o.value === value
                ? "bg-carbon-surface3 text-carbon-text"
                : "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
            }`}
          >
            {o.glyph}
            <span className="truncate">{o.label}</span>
          </button>
        ))}
      </DropdownListbox>
    </>
  );
}
