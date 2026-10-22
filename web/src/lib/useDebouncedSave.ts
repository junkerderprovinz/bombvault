import { useEffect, useRef } from "react";

const AUTOSAVE_DEBOUNCE_MS = 800;

/** useDebouncedSave runs the last call after the delay and drops the ones
 *  before it, one timer per component instance. A call still waiting when
 *  the component goes away runs then, so an edit made just before leaving is
 *  saved. `cancel` lets an immediate save retire a pending one, which would
 *  otherwise fire afterwards and write the older value back. */
export function useDebouncedSave(delayMs: number = AUTOSAVE_DEBOUNCE_MS) {
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pending = useRef<(() => void) | null>(null);

  useEffect(() => {
    return () => {
      if (timer.current) clearTimeout(timer.current);
      timer.current = null;
      const run = pending.current;
      pending.current = null;
      run?.();
    };
  }, []);

  function debouncedSave(run: () => void) {
    if (timer.current) clearTimeout(timer.current);
    pending.current = run;
    timer.current = setTimeout(() => {
      timer.current = null;
      pending.current = null;
      run();
    }, delayMs);
  }

  function cancel() {
    if (timer.current) clearTimeout(timer.current);
    timer.current = null;
    pending.current = null;
  }

  return { debouncedSave, cancel };
}
