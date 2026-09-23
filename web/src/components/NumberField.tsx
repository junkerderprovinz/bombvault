// A number input with its steppers inside the field. The browser's spinner is
// an OS widget on its own background that only `accent-color` reaches, so
// `glim-num` hides it and two arrows without a background of their own take its
// place, with padding that keeps the digits clear of them. Reference:
// glimstone reference/numberField.ts and the `.glim-num-*` rules in
// reference/tokens.css.
//
// The props are a native <input>'s, so a call site swaps `<input type="number"`
// for `<NumberField` and keeps its own onChange, clamping and debouncing
// included. stepUp() fires no event, so the steppers replay the new value
// through setValueLikeAUser for that onChange to run.

import { useCallback, useEffect, useRef, useState, type InputHTMLAttributes } from "react";

export type NumberFieldProps = Omit<InputHTMLAttributes<HTMLInputElement>, "type"> & {
  /** Classes for the positioned wrapper, for callers that size their own row. */
  wrapperClassName?: string;
};

/** Whether a step in this direction would still land inside min/max. */
function canStep(el: HTMLInputElement | null, direction: 1 | -1): boolean {
  if (!el) return true;
  const step = Number(el.step) || 1;
  const raw = el.value === "" ? Number(el.min) || 0 : Number(el.value);
  if (!Number.isFinite(raw)) return true;
  const next = raw + direction * step;
  const min = el.min === "" ? -Infinity : Number(el.min);
  const max = el.max === "" ? Infinity : Number(el.max);
  return direction > 0 ? next <= max : next >= min;
}

/**
 * Sets the value so that React treats the following input event as an edit.
 * React compares against the last value it saw on the node, and assigning
 * `el.value` updates that record too, so onChange would never fire. The
 * prototype's setter goes around the instance property React installed.
 */
function setValueLikeAUser(el: HTMLInputElement, next: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
  if (setter) setter.call(el, next);
  else el.value = next;
  el.dispatchEvent(new Event("input", { bubbles: true }));
}

export function NumberField({ className = "", wrapperClassName = "", ...rest }: NumberFieldProps) {
  const ref = useRef<HTMLInputElement>(null);
  // State rather than a value computed during render: it reads the DOM node's
  // min, max and step, which do not exist on the first render.
  const [ends, setEnds] = useState({ up: true, down: true });

  const sync = useCallback(() => {
    setEnds({ up: canStep(ref.current, 1), down: canStep(ref.current, -1) });
  }, []);

  useEffect(sync, [sync, rest.value]);

  // stepUp and stepDown keep min, max and step in the markup, and the browser
  // does the clamping.
  const step = (direction: 1 | -1) => {
    const el = ref.current;
    if (!el || el.disabled || el.readOnly) return;
    const before = el.value;
    if (direction > 0) el.stepUp();
    else el.stepDown();
    const after = el.value;
    if (after === before) return;
    // React did not see stepUp's write. Replay it so the call site's onChange
    // runs.
    setValueLikeAUser(el, before);
    setValueLikeAUser(el, after);
    sync();
  };

  // The wheel steps the value only while the field has focus, so scrolling the
  // page past a field never changes it. The listener is non-passive because
  // preventDefault has to stop the page scrolling the field away, and React's
  // onWheel is passive. Wheel up is more; trackpads report fractional deltas,
  // so only the sign counts.
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const onWheel = (e: WheelEvent) => {
      if (el.disabled || el.readOnly) return;
      if (el.ownerDocument.activeElement !== el) return;
      if (e.deltaY === 0) return;
      e.preventDefault();
      step(e.deltaY < 0 ? 1 : -1);
    };
    el.addEventListener("wheel", onWheel, { passive: false });
    return () => el.removeEventListener("wheel", onWheel);
    // `step` is new every render but only uses the ref and `sync`, which are
    // stable, so the listener is attached once.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sync]);

  // The native spinner's small solid triangle, with corners rounded by a
  // round-joined stroke in the same colour. The path is inset by half the
  // stroke width so the painted shape keeps the size of the sharp one.
  const Arrow = ({ up }: { up: boolean }) => (
    <svg viewBox="0 0 10 6" width="10" height="6" aria-hidden="true">
      {/* bv-convention-exception: user-message-is-translated -- SVG path data,
          not prose; nothing to translate. */}
      <path
        d={up ? "M5 1.7 L8.6 4.6 L1.4 4.6 Z" : "M5 4.3 L1.4 1.4 L8.6 1.4 Z"}
        fill="currentColor"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinejoin="round"
      />
    </svg>
  );

  return (
    // Sized by its content: in a labelled field's column flex container a
    // stretched wrapper would put the arrows at the far end of the row, away
    // from the input. The input therefore needs a definite width.
    <span className={`relative block w-fit max-w-full self-start justify-self-start ${wrapperClassName}`}>
      <input
        {...rest}
        ref={ref}
        type="number"
        // pe-8 keeps the digits clear of the arrows.
        className={`glim-num pe-8 ${className}`}
      />
      {/* Hidden and unfocusable: the input already exposes the value, the
          range and the arrow keys. */}
      <span
        aria-hidden="true"
        className="pointer-events-none absolute inset-y-px end-1.5 flex flex-col justify-center gap-px"
      >
        {([1, -1] as const).map((direction) => {
          const enabled = direction > 0 ? ends.up : ends.down;
          return (
            <button
              key={direction}
              type="button"
              tabIndex={-1}
              disabled={rest.disabled || rest.readOnly || !enabled}
              // No title: nobody reaches these by keyboard or screen reader, so
              // it would only paint the OS tooltip the icon-badge rule forbids.
              // No background, so the arrows stay part of the field.
              className="pointer-events-auto flex h-[11px] w-[14px] items-center justify-center border-0 bg-transparent p-0 text-carbon-textMuted transition-colors hover:text-carbon-text disabled:opacity-35 disabled:hover:text-carbon-textMuted focus:outline-none"
              // Keep the caret in the field: a mousedown here would move focus
              // and a field that saves on blur would fire on every click.
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => step(direction)}
            >
              <Arrow up={direction > 0} />
            </button>
          );
        })}
      </span>
    </span>
  );
}
