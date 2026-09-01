// A number field with our own steppers inside it ([411]).
//
// GlimStone, "Never a native number spinner either": `<input type="number">` is
// fine and keeps everything worth keeping — min/max/step, the arrow keys, a
// phone's numeric keypad — but the two arrows the browser paints beside it come
// from the OS widget set, on their own background, and the only property that
// reaches them is `accent-color`. jdp reported it here in the same words he
// used one app earlier: "jetzt haben sie einen dunklen hintergrund. der soll
// weg".
//
// The rule has a second half that is easy to drop, and dropping it is what made
// the first replacement (in that other app) wrong: the complaint was never
// "there are arrows". It was that they arrive on their own dark ground AND
// crowd the value. So a stepper is PART OF THE FIELD, not a control beside it —
// inside the field's own box, no background of its own, only the ink changing
// on hover, with enough inline padding that the digits never run underneath.
//
// Reference implementation and CSS: glimstone reference/numberField.ts and the
// `.glim-num-*` rules in reference/tokens.css, both added in 1.7.0. This is the
// React shape of that, not a second design.
//
// ---------------------------------------------------------------------------
// Why the props are a native <input>'s props, verbatim
// ---------------------------------------------------------------------------
// Fifteen call sites across six files, and every one of them carries its own
// clamping inside onChange — Math.max(1, parseInt(...)), an isNaN guard, a
// server-matching 5..3600 range, a debounce keyed on the field name. A tidier
// `onValueChange(n: number)` signature would have meant rewriting all fifteen,
// at the end of a long session, on the page that holds somebody's SMTP port and
// their retention counts. The prize for that is a slightly nicer prop name.
//
// So this takes `onChange` exactly as the native element does, and the swap at
// each site is `<input` -> `<NumberField` with `type="number"` dropped. Nothing
// else moves.
//
// Which leaves one real problem: `stepUp()` writes the DOM and fires NO event,
// so on a controlled React input the new number would appear for a frame and
// snap straight back to the old prop. Setting `el.value` directly does not help
// either — React tracks the last value it wrote on the node, sees no change,
// and swallows the event. The way through is React's own tracker: write through
// the prototype's value setter (which the tracker does not see) and then
// dispatch a real `input` event, so React concludes a person typed it and calls
// onChange with an ordinary event. Every call site's existing handler then runs
// unchanged, clamping and debouncing included.

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
 * Set an input's value the way a person would, as far as React can tell.
 *
 * React stores the last value it rendered on the node itself and compares
 * against it before dispatching onChange; assigning `el.value` updates that
 * store as a side effect, so the comparison finds nothing changed and the event
 * never reaches the handler. Writing through the PROTOTYPE's setter bypasses
 * the instance property React installed, leaving its record stale — which is
 * exactly what makes the following event look like a real edit.
 */
function setValueLikeAUser(el: HTMLInputElement, next: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
  if (setter) setter.call(el, next);
  else el.value = next; // no prototype setter: nothing to bypass
  el.dispatchEvent(new Event("input", { bubbles: true }));
}

export function NumberField({ className = "", wrapperClassName = "", ...rest }: NumberFieldProps) {
  const ref = useRef<HTMLInputElement>(null);
  // Held in state rather than computed during render: the answer depends on the
  // DOM node's own min/max/step, which do not exist on the first pass.
  const [ends, setEnds] = useState({ up: true, down: true });

  const sync = useCallback(() => {
    setEnds({ up: canStep(ref.current, 1), down: canStep(ref.current, -1) });
  }, []);

  useEffect(sync, [sync, rest.value]);

  /**
   * Step through the input's OWN stepUp/stepDown so min/max/step live in the
   * markup and nowhere else, and the browser's clamping applies for free.
   */
  const step = (direction: 1 | -1) => {
    const el = ref.current;
    if (!el || el.disabled || el.readOnly) return;
    const before = el.value;
    if (direction > 0) el.stepUp();
    else el.stepDown();
    const after = el.value;
    if (after === before) return; // already at the end; nothing to report
    // stepUp wrote the DOM directly and React saw nothing. Put the value back
    // through the tracker so the call site's own onChange runs.
    setValueLikeAUser(el, before);
    setValueLikeAUser(el, after);
    sync();
  };

  const Arrow = ({ up }: { up: boolean }) => (
    <svg viewBox="0 0 10 6" width="10" height="6" aria-hidden="true">
      {/* bv-convention-exception: user-message-is-translated -- SVG path data,
          not prose. "M1 5 L5 1 L9 5" is a chevron; there is nothing here for a
          translator to translate. */}
      <path
        d={up ? "M1 5 L5 1 L9 5" : "M1 1 L5 5 L9 1"}
        fill="none"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );

  return (
    <span className={`relative block ${wrapperClassName}`}>
      <input
        {...rest}
        ref={ref}
        type="number"
        // bv-num strips the native spinner; pe-8 is the rule's second half —
        // room for the arrows so the digits never run underneath them.
        className={`bv-num pe-8 ${className}`}
      />
      {/* aria-hidden and not focusable: the input already carries the value,
          the range and the arrow keys. A screen reader meeting these would
          hear a third control changing the same number for no reason. */}
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
              // NO title, and not an oversight. These carry aria-hidden and
              // tabIndex={-1}, so no keyboard or screen-reader user ever
              // reaches them — which leaves `title` doing only the one thing
              // the repo's icon-badge rule forbids it for: painting an OS
              // balloon. A chevron inside a number field beside its own label
              // is not a control anybody has to be told about.
              // No background, ever. Giving these a surface is what turned the
              // first attempt at this into three objects standing in a row.
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
