// Whether a retention rule that keeps fewer backups asks before it is saved.
// The answer belongs to the person, so it is stored and synced with the other
// display preferences.

import { useSyncExternalStore } from "react";
import { ADOPTED_EVENT, save as saveDisplayPrefs } from "./displayPrefs";

const KEY = "bombvault.askKeepLess";

function readRaw(): string {
  try {
    return localStorage.getItem(KEY) ?? "";
  } catch {
    return "";
  }
}

/** readAskKeepLess is true until someone switches the question off. */
export function readAskKeepLess(): boolean {
  return readRaw() !== "off";
}

const listeners = new Set<() => void>();

export function setAskKeepLess(ask: boolean): void {
  try {
    localStorage.setItem(KEY, ask ? "on" : "off");
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

export function useAskKeepLess(): boolean {
  return useSyncExternalStore(subscribe, readRaw) !== "off";
}
