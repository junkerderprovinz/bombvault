import { useEffect, useState } from "react";
import { getLabelMode, type ControlAxis, type LabelMode } from "./controls";

// The label mode is edited in Settings and read by every button, nav row and
// tab strip on the page, so it travels by window event like useCloudCredSets.
// The initial value comes from localStorage synchronously, so the first paint
// is already right.

const LABEL_MODE_CHANGED = "bv:label-mode-changed";

export function labelModeChanged(): void {
  window.dispatchEvent(new Event(LABEL_MODE_CHANGED));
}

export function useLabelMode(axis: ControlAxis): LabelMode {
  const [mode, setMode] = useState<LabelMode>(() => getLabelMode(axis));
  useEffect(() => {
    const reread = () => setMode(getLabelMode(axis));
    window.addEventListener(LABEL_MODE_CHANGED, reread);
    // Follow changes made in another tab, too.
    window.addEventListener("storage", reread);
    return () => {
      window.removeEventListener(LABEL_MODE_CHANGED, reread);
      window.removeEventListener("storage", reread);
    };
  }, [axis]);
  return mode;
}
