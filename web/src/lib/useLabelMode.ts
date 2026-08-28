import { useEffect, useState } from "react";
import { getLabelMode, type ControlAxis, type LabelMode } from "./controls";

// ---------------------------------------------------------------------------
// useLabelMode (#178) — the current label mode for one axis, kept in step
// across every control on the page.
//
// The same shape useCloudCredSets uses for a value that is EDITED in one place
// (the Settings card) and CONSUMED in many (every button, every nav row, every
// tab strip), which are mounted at the same time: a window event rather than a
// per-component copy, so choosing a mode updates the whole page at once
// instead of only after a reload.
//
// The initial value is read synchronously from localStorage, so the very first
// paint is already in the right mode and nothing flashes.
// ---------------------------------------------------------------------------

const LABEL_MODE_CHANGED = "bv:label-mode-changed";

/** Announce that a label mode changed, so every mounted control re-reads it. */
export function labelModeChanged(): void {
  window.dispatchEvent(new Event(LABEL_MODE_CHANGED));
}

export function useLabelMode(axis: ControlAxis): LabelMode {
  const [mode, setMode] = useState<LabelMode>(() => getLabelMode(axis));
  useEffect(() => {
    const reread = () => setMode(getLabelMode(axis));
    window.addEventListener(LABEL_MODE_CHANGED, reread);
    // "storage" fires when ANOTHER tab changes it, which is worth following:
    // two tabs of the same app disagreeing about their own chrome is the kind
    // of thing that reads as a bug rather than as two independent windows.
    window.addEventListener("storage", reread);
    return () => {
      window.removeEventListener(LABEL_MODE_CHANGED, reread);
      window.removeEventListener("storage", reread);
    };
  }, [axis]);
  return mode;
}
