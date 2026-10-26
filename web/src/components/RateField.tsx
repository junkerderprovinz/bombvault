// A number field for a rate in KiB/s with the same rate in MB/s and Mbit/s in
// front of it, because providers quote bandwidth in bits. The two are siblings
// so the row's control box lines them up and wraps them like its other
// children.
import { useId } from "react";
import { rateHint } from "../lib/rate";
import { NumberField, type NumberFieldProps } from "./NumberField";

export function RateField(props: NumberFieldProps) {
  const hintId = useId();
  const hint = rateHint(Number(props.value));
  return (
    <>
      {hint && (
        <span id={hintId} className="whitespace-nowrap text-xs tabular-nums text-carbon-textMuted">
          {hint}
        </span>
      )}
      <NumberField {...props} aria-describedby={hint ? hintId : undefined} />
    </>
  );
}
