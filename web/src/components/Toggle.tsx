import { useEffect, useId, useRef, useState, type RefObject } from "react";
import { shake } from "../lib/shake";
import { useTipBubble } from "../lib/useTipBubble";

export interface ToggleProps {
  checked: boolean;
  /** Called with the flipped value when the switch is activated. */
  onChange: (next: boolean) => void;
  /** Always the accessible name; shown as visible text unless hideLabel. */
  label: string;
  /** Hides the visible caption. Only for callers that print the same text
   *  right next to the switch themselves; a card title further up does not
   *  count. */
  hideLabel?: boolean;
  disabled?: boolean;
  /** Why the switch cannot be flipped right now, such as OLED while Light is
   *  chosen. It keeps its state and its place in the tab order, its track
   *  dims, a press shakes it, and the reason is its bubble, or a line under
   *  the row on a touch screen. */
  refused?: string;
  /** Extra classes for the outer wrapper. */
  className?: string;
}

interface Refusal {
  frame: RefObject<HTMLSpanElement | null>;
  button: (el: HTMLElement | null) => void;
  handlers: ReturnType<typeof useTipBubble>["handlers"];
  describedBy: string;
  press: () => void;
}

// Toggle is the shared switch. The focus ring sits outside the track, on the
// card surface its contrast was measured against.
//
// The button is the tap target and the track is drawn inside it. Under a
// coarse pointer the button grows to the control height while the track keeps
// its size, so the larger target stays inside the switch's own box and cannot
// take taps from a neighbouring control.
export function Toggle(props: ToggleProps) {
  // The plain switch stays free of hooks, so its tests can call it as a function.
  if (props.refused) return <RefusedToggle {...props} refused={props.refused} />;
  return frame(props);
}

function RefusedToggle(props: ToggleProps & { refused: string }) {
  const frameEl = useRef<HTMLSpanElement>(null);
  const [shakes, setShakes] = useState(0);
  useEffect(() => (shakes > 0 ? shake(frameEl.current!) : undefined), [shakes]);
  const tip = useTipBubble(props.refused);
  const reason = useId();
  return (
    <>
      {frame(props, {
        frame: frameEl,
        button: tip.ref,
        handlers: tip.handlers,
        describedBy: [reason, tip.describedBy].filter(Boolean).join(" "),
        press: () => setShakes((n) => n + 1),
      })}
      <span id={reason} className="glim-toggle-refusal">
        {props.refused}
      </span>
      {tip.bubble}
    </>
  );
}

function frame({ checked, onChange, label, hideLabel = false, disabled, className }: ToggleProps, refusal?: Refusal) {
  return (
    <span ref={refusal?.frame} className={`inline-flex items-center gap-2 ${className ?? ""}`}>
      {!hideLabel && <span className="text-sm text-carbon-text">{label}</span>}
      <button
        {...refusal?.handlers}
        ref={refusal?.button}
        type="button"
        role="switch"
        aria-checked={checked}
        aria-label={label}
        aria-disabled={refusal ? true : undefined}
        aria-describedby={refusal?.describedBy}
        disabled={disabled}
        onClick={refusal ? refusal.press : () => onChange(!checked)}
        // No title: the label is always visible next to the switch, so a
        // tooltip would only repeat it.
        className="group inline-flex h-5 w-9 shrink-0 items-center pointer-coarse:h-(--btn-h) focus-visible:outline-none disabled:opacity-50"
      >
        <span
          className={`inline-flex h-5 w-9 items-center rounded-pill transition-colors group-focus-visible:outline-solid group-focus-visible:outline-2 group-focus-visible:outline-offset-2 group-focus-visible:outline-(--focus-ring) ${
            checked ? "bg-accent" : "bg-(--toggle-off)"
          }`}
        >
          <span
            // translate-x is physical, but the flex rest position already
            // mirrors in RTL, so rtl: flips the sign. The `!` settles the tie
            // between two classes setting the same property.
            //
            // rtl: follows the page while the flex layout follows the nearest
            // dir attribute, so a Toggle must not sit inside a container with
            // its own dir. Wrap only the text that needs dir="ltr".
            //
            // rounded-pill shares the track's token, so the knob is a circle in
            // the round shape and follows the track in the others.
            className={`inline-block h-3.5 w-3.5 rounded-pill bg-carbon-background transition-transform ${
              checked ? "translate-x-[18px] rtl:-translate-x-[18px]!" : "translate-x-[3px] rtl:-translate-x-[3px]!"
            }`}
          />
        </span>
      </button>
    </span>
  );
}
