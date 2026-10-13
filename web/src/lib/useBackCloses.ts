import { useEffect, useId, useRef } from "react";

// An open sheet or dialog holds a history entry of its own above the page, so
// Back (the phone's key, the browser's button) closes it before it leaves the
// page. The entry carries the ids of the layers open at that point; a popstate
// that lands where a layer's id is missing means Back went past that layer.

interface Layer {
  id: string;
  /** false keeps the layer open, and its entry is put back. */
  close: () => void | boolean;
}

const KEY = "bvLayers";

const open: Layer[] = [];
/** Layers waiting for our own history.go to land before they push. */
let waiting: Layer[] = [];
/** history.go calls of ours whose popstate has not arrived yet. */
let returning = 0;
/** Layers closed in this task whose entries are still to be dropped. */
let leaving: string[] = [];
/** What waits for the layer entries to be gone, see leaveLayers. */
let landed: (() => void)[] = [];
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

// Once our own steps back have landed, what waited for that runs first, so a
// URL it replaces is the entry below the layers and not a layer pushed next.
function settle() {
  const then = landed;
  landed = [];
  then.forEach((fn) => fn());
  if (returning > 0) return;
  const next = waiting;
  waiting = [];
  next.forEach(push);
}

function onPop() {
  if (returning > 0) {
    returning--;
    if (returning === 0) settle();
    return;
  }
  const here = stackHere();
  for (let i = open.length - 1; i >= 0; i--) {
    const layer = open[i];
    if (here.includes(layer.id)) continue;
    open.splice(i, 1);
    if (layer.close() === false) {
      open.splice(i, 0, layer);
      push(layer);
    }
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
      if (returning === 0) settle();
      return;
    }
    returning++;
    window.history.go(-steps);
  });
}

/**
 * leaveLayers drops the history entries of the open layers and then runs then
 * on the entry below them. A window that its URL opens closes through it, so
 * the URL it clears is its own entry and Back afterwards does not open it
 * again.
 */
export function leaveLayers(then: () => void) {
  if (returning > 0 || leaving.length > 0) {
    landed.push(() => leaveLayers(then));
    return;
  }
  const steps = stackHere().length;
  if (steps === 0) {
    then();
    return;
  }
  landed.push(then);
  returning++;
  window.history.go(-steps);
}

/** useBackCloses lets Back close a sheet or dialog while it is open. An
 *  onClose that returns false keeps it open, and Back stays with it. A way
 *  out of it to another page navigates with replace, so that page takes the
 *  layer's entry and Back from it skips the closed window. */
export function useBackCloses(isOpen: boolean, onClose: () => void | boolean) {
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
