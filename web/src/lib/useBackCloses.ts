import { useEffect, useId, useRef } from "react";

// An open sheet or dialog holds a history entry of its own above the page, so
// Back (the phone's key, the browser's button) closes it before it leaves the
// page. The entry carries the ids of the layers open at that point; a popstate
// that lands where a layer's id is missing means Back went past that layer.

interface Layer {
  id: string;
  close: () => void;
}

const KEY = "bvLayers";

const open: Layer[] = [];
/** Layers waiting for our own history.go to land before they push. */
let waiting: Layer[] = [];
/** history.go calls of ours whose popstate has not arrived yet. */
let returning = 0;
/** Layers closed in this task whose entries are still to be dropped. */
let leaving: string[] = [];
let listening = false;

function stackHere(): string[] {
  const state = window.history.state as Record<string, unknown> | null;
  const ids = state?.[KEY];
  return Array.isArray(ids) ? (ids as string[]) : [];
}

function push(layer: Layer) {
  const state = (window.history.state ?? {}) as Record<string, unknown>;
  window.history.pushState({ ...state, [KEY]: [...stackHere(), layer.id] }, "");
}

function onPop() {
  if (returning > 0) {
    returning--;
    if (returning === 0) {
      const next = waiting;
      waiting = [];
      next.forEach(push);
    }
    return;
  }
  const here = stackHere();
  for (let i = open.length - 1; i >= 0; i--) {
    const layer = open[i];
    if (here.includes(layer.id)) continue;
    open.splice(i, 1);
    layer.close();
  }
}

function hold(layer: Layer) {
  if (!listening) {
    window.addEventListener("popstate", onPop);
    listening = true;
  }
  open.push(layer);
  // Pushing now would put the entry above one that is about to be dropped.
  if (returning > 0 || leaving.length > 0) waiting.push(layer);
  else push(layer);
}

function release(layer: Layer) {
  const at = open.indexOf(layer);
  if (at === -1) return;
  open.splice(at, 1);
  const queued = waiting.indexOf(layer);
  if (queued !== -1) {
    waiting.splice(queued, 1);
    return;
  }
  leaving.push(layer.id);
  if (leaving.length > 1) return;
  // Several layers closing together leave in one step, and a navigation the
  // close led to (a link inside the sheet) has already replaced the entry,
  // which then stays where it is.
  queueMicrotask(() => {
    const here = stackHere();
    const steps = leaving.filter((id) => here.includes(id)).length;
    leaving = [];
    if (steps === 0) {
      if (returning === 0) {
        const next = waiting;
        waiting = [];
        next.forEach(push);
      }
      return;
    }
    returning++;
    window.history.go(-steps);
  });
}

/** useBackCloses lets Back close a sheet or dialog while it is open. */
export function useBackCloses(isOpen: boolean, onClose: () => void) {
  const id = useId();
  const closeRef = useRef(onClose);
  closeRef.current = onClose;
  useEffect(() => {
    if (!isOpen) return;
    const layer = { id, close: () => closeRef.current() };
    hold(layer);
    return () => release(layer);
  }, [isOpen, id]);
}
