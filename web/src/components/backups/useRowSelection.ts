import { useCallback, useEffect, useRef, useState } from "react";
import { rangeIds } from "../../lib/backupList";

export interface RowSelection {
  picked: ReadonlySet<string>;
  /** Rows are being picked: at least one is, or the mode was asked for. */
  selecting: boolean;
  allPicked: boolean;
  /** Takes a click on a row and answers whether it picked rows. A plain click
   *  outside the mode does not, and opens the entry instead. */
  click: (e: { shiftKey: boolean; ctrlKey: boolean; metaKey: boolean }, id: string) => boolean;
  set: (id: string, on: boolean) => void;
  setAll: (on: boolean) => void;
  /** Starts picking, with a row or with none yet. */
  begin: (id?: string) => void;
  clear: () => void;
}

function typing(): boolean {
  const el = document.activeElement;
  return el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement || (el instanceof HTMLElement && el.isContentEditable);
}

// A window, a confirmation or a menu above the list takes the keys itself.
function covered(): boolean {
  return document.querySelector('[role="dialog"], [role="alertdialog"], [role="menu"]') !== null;
}

/**
 * useRowSelection picks rows as a file manager does: Ctrl+click adds one,
 * Shift+click the range from the row picked last, Ctrl+A all of them, and
 * Escape lets go. `order` is the ids of the rows in view, in their order. A
 * row that leaves the view leaves the selection, so no action reaches what
 * cannot be seen.
 */
export function useRowSelection(order: string[]): RowSelection {
  const [picked, setPicked] = useState<ReadonlySet<string>>(new Set());
  const [mode, setMode] = useState(false);
  const anchor = useRef<string | null>(null);
  const orderRef = useRef(order);
  orderRef.current = order;

  const inView = order.join("\u0000");
  useEffect(() => {
    setPicked((prev) => {
      const kept = orderRef.current.filter((id) => prev.has(id));
      return kept.length === prev.size ? prev : new Set(kept);
    });
  }, [inView]);

  const set = useCallback((id: string, on: boolean) => {
    anchor.current = id;
    setPicked((prev) => {
      const next = new Set(prev);
      if (on) next.add(id);
      else next.delete(id);
      return next;
    });
  }, []);

  const setAll = useCallback((on: boolean) => setPicked(new Set(on ? orderRef.current : [])), []);

  const clear = useCallback(() => {
    anchor.current = null;
    setMode(false);
    setPicked(new Set());
  }, []);

  const begin = useCallback(
    (id?: string) => {
      setMode(true);
      if (id !== undefined) set(id, true);
    },
    [set]
  );

  const selecting = mode || picked.size > 0;

  const click = useCallback(
    (e: { shiftKey: boolean; ctrlKey: boolean; metaKey: boolean }, id: string) => {
      if (e.shiftKey) {
        const range = rangeIds(orderRef.current, anchor.current, id);
        setPicked((prev) => new Set([...prev, ...range]));
        return true;
      }
      if (!e.ctrlKey && !e.metaKey && !selecting) return false;
      set(id, !picked.has(id));
      return true;
    },
    [picked, selecting, set]
  );

  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (typing() || covered()) return;
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "a" && orderRef.current.length > 0) {
        e.preventDefault();
        setAll(true);
      } else if (e.key === "Escape" && selecting) {
        clear();
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [selecting, setAll, clear]);

  const allPicked = order.length > 0 && order.every((id) => picked.has(id));
  return { picked, selecting, allPicked, click, set, setAll, begin, clear };
}
