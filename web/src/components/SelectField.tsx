import { useEffect, useId, useRef, type ReactNode, type Ref } from "react";
import * as select from "@zag-js/select";
import { normalizeProps, Portal, useMachine } from "@zag-js/react";
import { IconCheck } from "./navGlyphs";
import { mergeRefs } from "../lib/mergeRefs";
import { usePortalHue } from "../lib/portalHue";
import { enableWheelStep, stepIndex } from "../lib/selectScroll";

// SelectField replaces a native <select>, whose open list is drawn by the
// operating system and cannot be styled. Zag's select machine carries the
// behaviour GlimStone's Select contract asks for: typeahead, Home and End,
// aria-activedescendant and focus back on the trigger. The open list wears
// GlimStone's .glim-select-* rules; the trigger takes the className the select
// had, so each call site keeps its size and the filled look of the fields
// beside it, and the mouse wheel steps the closed control.

export interface SelectOption<T extends string> {
  value: T;
  label: string;
  /** Optional mark before the label, shown in the trigger and in the list. */
  glyph?: ReactNode;
  disabled?: boolean;
}

/** The rounded filled triangle NumberField's steppers use. */
export function SelectArrow() {
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
  wrap = false,
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
  /** Lets a label run onto a second line where the row is narrower than it,
   *  for a value whose end says as much as its start, such as a location. */
  wrap?: boolean;
  /** Set on the trigger, for a visible <label htmlFor>. */
  id?: string;
  /** The trigger's own DOM node, for a caller that opens this field already
   *  focused. */
  ref?: Ref<HTMLButtonElement>;
}) {
  const triggerRef = useRef<HTMLButtonElement>(null);
  const collection = select.collection({
    items: options,
    itemToValue: (o) => o.value,
    itemToString: (o) => o.label,
    isItemDisabled: (o) => !!o.disabled,
  });
  const service = useMachine(select.machine, {
    id: useId(),
    ids: id ? { trigger: id } : undefined,
    collection,
    value: [value],
    disabled,
    loopFocus: true,
    onValueChange: (d) => {
      const next = d.value[0];
      if (next !== undefined) onChange(next as T);
    },
    positioning: { placement: "bottom", gutter: 4 },
  });
  const api = select.connect(service, normalizeProps);
  // The list sits on <body>, outside the hued card the trigger stands in.
  const hue = usePortalHue(api.open, triggerRef);
  const triggerProps = api.getTriggerProps();
  const positioner = api.getPositionerProps();

  // Read through a ref, so the listener is attached once and still steps from
  // the value of the latest render.
  const stepRef = useRef<(delta: 1 | -1) => void>(() => {});
  stepRef.current = (delta) => {
    if (disabled || options.length < 2) return;
    let at = options.findIndex((o) => o.value === value);
    if (at < 0) at = 0;
    let next = at;
    // A wheel notch skips disabled options instead of landing on one.
    for (;;) {
      const candidate = stepIndex(options.length, next, delta);
      if (candidate === next) return; // clamped at an end
      next = candidate;
      if (!options[next]?.disabled) break;
    }
    const picked = options[next];
    if (picked && picked.value !== value) onChange(picked.value);
  };
  useEffect(() => {
    const trigger = triggerRef.current;
    if (!trigger) return;
    return enableWheelStep(trigger, (delta) => stepRef.current(delta));
  }, []);

  const current = options.find((o) => o.value === value);

  return (
    <>
      <button
        {...triggerProps}
        ref={mergeRefs(ref, triggerRef)}
        // The field has no visible label of its own, so the name is set here
        // rather than through Zag's label part, and the list is named only
        // while it exists.
        aria-labelledby={undefined}
        aria-label={label}
        aria-controls={api.open ? triggerProps["aria-controls"] : undefined}
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
              className={`col-start-1 row-start-1 ${wrap ? "wrap-break-word" : "truncate"} ${o.value === value ? "" : "invisible"}`}
            >
              {o.label}
            </span>
          ))}
        </span>
        <span {...api.getIndicatorProps()} className="glim-select-indicator">
          <SelectArrow />
        </span>
      </button>
      {/* Mounted only while open, like the list a native select draws, so a
          page holds no hidden copy of every label. Zag reaches the list a
          frame after it opens, by when it is there. */}
      {api.open && (
        <Portal>
          <div
            {...positioner}
            style={{ ...positioner.style, ...hue.style }}
            className={`z-50 ${hue.className}`}
          >
            <ul
              {...api.getContentProps()}
              aria-labelledby={undefined}
              aria-label={label}
              className="glim-select-content glim-fade w-max min-w-(--reference-width) max-w-[min(92vw,28rem)]"
            >
              {options.map((item) => (
                <li key={item.value} {...api.getItemProps({ item })} className="glim-select-item">
                  <span {...api.getItemTextProps({ item })} className="flex min-w-0 items-center gap-2.5">
                    {item.glyph}
                    <span className="truncate">{item.label}</span>
                  </span>
                  <span {...api.getItemIndicatorProps({ item })} className="glim-select-check">
                    <IconCheck />
                  </span>
                </li>
              ))}
            </ul>
          </div>
        </Portal>
      )}
    </>
  );
}
