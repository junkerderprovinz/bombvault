import { IconCheck } from "../navGlyphs";
import { Selector } from "../Selector";

/**
 * StepBar names the steps of a window and shows which one is open. A step
 * that was reached can be gone back to, one that was not is locked.
 */
export function StepBar({
  steps,
  current,
  reach,
  label,
  onGo,
}: {
  steps: string[];
  /** The open step, counted from 1. */
  current: number;
  /** The furthest step that may be opened. */
  reach: number;
  label: string;
  onGo: (step: number) => void;
}) {
  return (
    <Selector
      items={steps.map((name, i) => ({
        id: String(i + 1),
        label: name,
        icon:
          i + 1 < current ? (
            <IconCheck />
          ) : (
            <span aria-hidden="true" className="tabular-nums">
              {i + 1}
            </span>
          ),
        disabled: i + 1 > reach,
      }))}
      label={label}
      select="one"
      activation="manual"
      fit
      hue={false}
      active={String(current)}
      onChange={(id) => onGo(Number(id))}
    />
  );
}
