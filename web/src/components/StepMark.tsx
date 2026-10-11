import type { StepState } from "../lib/firstSteps";
import { IconCheck } from "./navGlyphs";

const TONE: Record<StepState, string> = {
  done: "bg-statusOkBg text-statusOk",
  next: "bg-accent text-accentContrast",
  open: "bg-carbon-surface2 text-carbon-textSub",
};

/** StepMark is the number in front of a step. A finished step shows a tick,
 *  and the step to do next is the only one in the accent. */
export function StepMark({ n, state }: { n: number; state: StepState }) {
  return (
    <span
      aria-hidden="true"
      data-state={state}
      className={`flex h-7 w-7 flex-none items-center justify-center rounded-pill text-sm font-semibold tabular-nums [&_svg]:h-3.5 [&_svg]:w-3.5 ${TONE[state]}`}
    >
      {state === "done" ? <IconCheck /> : n}
    </span>
  );
}
