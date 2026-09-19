import { useId, useRef, useState, type ReactNode, type Ref } from "react";
import { DropdownListbox } from "./DropdownListbox";
import { mergeRefs } from "../lib/mergeRefs";
import { stepIndex } from "../lib/selectScroll";

// SelectField replaces a native <select>, whose open list is drawn by the
// operating system and cannot be styled. The trigger takes the className the
// select had, so each call site keeps its size, and the mouse wheel steps the
// closed control.

export interface SelectOption<T extends string> {
  value: T;
  label: string;
  /** Optional mark before the label, shown in the trigger and in the list. */
  glyph?: ReactNode;
  disabled?: boolean;
}

/** The rounded filled triangle NumberField's steppers use. */
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
  ref,
}: {
  value: T;
  onChange: (next: T) => void;
  options: SelectOption<T>[];
  /** Accessible name, what the replaced select's label or aria-label said. */
  label: string;
  disabled?: boolean;
  /** Classes for the trigger. */
  className?: string;
  /** Set on the trigger, for a visible <label htmlFor>. */
  id?: string;
  /** The trigger's own DOM node, for a caller that opens this field already
   *  focused (LinkEntryPicker does, the moment the picker itself opens). */
  ref?: Ref<HTMLButtonElement>;
}) {
  const [open, setOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const listId = useId();
  const current = options.find((o) => o.value === value);

  // A wheel notch skips disabled options instead of landing on one.
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
        ref={mergeRefs(ref, triggerRef)}
        id={id}
        type="button"
        disabled={disabled}
        // The ARIA select-only combobox, so screen readers announce it the
        // way they announce a <select>.
        role="combobox"
        aria-label={label}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listId : undefined}
        onClick={() => setOpen((v) => !v)}
        className={`inline-flex items-center gap-2 text-start ${className}`}
      >
        {current?.glyph}
        {/* Every label is stacked in one grid cell, so the button is as wide
            as the widest option and does not shrink when a shorter one is
            picked. */}
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
