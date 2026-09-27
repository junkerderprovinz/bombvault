import { useEffect, useRef } from "react";

const AUTOSAVE_DEBOUNCE_MS = 800;

/** useDebouncedSave runs the last call after the delay and drops the ones
 *  before it, one timer per component instance. `cancel` lets an immediate
 *  save retire a pending one, which would otherwise fire afterwards and write
 *  the older value back. */
export function useDebouncedSave(delayMs: number = AUTOSAVE_DEBOUNCE_MS) {
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (timer.current) clearTimeout(timer.current);
    };
  }, []);

  function debouncedSave(run: () => void) {
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(run, delayMs);
  }

  function cancel() {
    if (timer.current) {
      clearTimeout(timer.current);
      timer.current = null;
    }
  }

  return { debouncedSave, cancel };
}
