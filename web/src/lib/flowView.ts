// Whether the storage locations of each section show as a picture or as plain
// switches. Some read the picture faster and others want the switches, so the
// choice is stored and synced with the other display preferences.

import { useSyncExternalStore } from "react";
import { ADOPTED_EVENT, save as saveDisplayPrefs } from "./displayPrefs";

export type FlowView = "graph" | "switches";

const KEY = "bombvault.flowView";

function readRaw(): string {
  try {
    return localStorage.getItem(KEY) ?? "";
  } catch {
    return "";
  }
}

const listeners = new Set<() => void>();

export function setFlowView(view: FlowView): void {
  try {
    localStorage.setItem(KEY, view);
  } catch {
    return;
  }
  saveDisplayPrefs();
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  window.addEventListener("storage", listener);
  window.addEventListener(ADOPTED_EVENT, listener);
  return () => {
    listeners.delete(listener);
    window.removeEventListener("storage", listener);
    window.removeEventListener(ADOPTED_EVENT, listener);
  };
}

export function useFlowView(): FlowView {
  return useSyncExternalStore(subscribe, readRaw) === "switches" ? "switches" : "graph";
}
