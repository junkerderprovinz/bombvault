// Which of the release's changes this browser has seen, and whether their dots
// show. It is kept per release, so the next one starts with all of its dots,
// and per browser: in the look that syncs through the server one person's
// click would hide a dot for everybody.

import { useMemo, useSyncExternalStore } from "react";
import { CHANGES, WHATS_NEW_RELEASE, type Change } from "./whatsNew";

const PREFIX = "bv-whats-new@";
const KEY = PREFIX + WHATS_NEW_RELEASE;

interface Stored {
  /** The dots show. They start hidden: nothing is new to a browser that
   *  never saw an older release. */
  on: boolean;
  seen: string[];
}

function readRaw(): string {
  try {
    return localStorage.getItem(KEY) ?? "";
  } catch {
    return "";
  }
}

function parse(raw: string): Stored {
  try {
    const stored: unknown = raw ? JSON.parse(raw) : null;
    if (stored && typeof stored === "object") {
      const { on, seen } = stored as Record<string, unknown>;
      return { on: on === true, seen: Array.isArray(seen) ? seen.filter((id): id is string => typeof id === "string") : [] };
    }
  } catch {
    // Anything else in the slot counts as nothing stored.
  }
  return { on: false, seen: [] };
}

const listeners = new Set<() => void>();

function write(next: Stored): void {
  try {
    localStorage.setItem(KEY, JSON.stringify(next));
  } catch {
    return;
  }
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  window.addEventListener("storage", listener);
  return () => {
    listeners.delete(listener);
    window.removeEventListener("storage", listener);
  };
}

/** The x.y.z of a version such as "v9.9.1+main.fcc0544", or null without one. */
function numbers(version: string): number[] | null {
  const m = /(\d+)\.(\d+)\.(\d+)/.exec(version);
  return m ? [Number(m[1]), Number(m[2]), Number(m[3])] : null;
}

function older(a: number[], b: number[]): boolean {
  for (let i = 0; i < 3; i++) if (a[i] !== b[i]) return a[i] < b[i];
  return false;
}

/**
 * beginWhatsNew is told the release this browser saw before an update. Coming
 * from one older than the list's release switches the dots on, once: a later
 * patch of the same release leaves what was seen and hidden as it is. The
 * stores of earlier releases go, since their dots never show again.
 */
export function beginWhatsNew(previous: string): void {
  const [from, release] = [numbers(previous), numbers(WHATS_NEW_RELEASE)];
  if (!from || !release || !older(from, release) || readRaw()) return;
  try {
    for (let i = localStorage.length - 1; i >= 0; i--) {
      const key = localStorage.key(i);
      if (key?.startsWith(PREFIX) && key !== KEY) localStorage.removeItem(key);
    }
  } catch {
    // Without storage there is nothing to clear and nothing to switch on.
  }
  write({ on: true, seen: [] });
}

/** markSeen counts changes as seen, or as new again. */
export function markSeen(ids: string[], seen = true): void {
  const stored = parse(readRaw());
  const rest = stored.seen.filter((id) => !ids.includes(id));
  write({ ...stored, seen: seen ? [...rest, ...ids] : rest });
}

export function showDots(on: boolean): void {
  write({ ...parse(readRaw()), on });
}

export interface WhatsNew {
  on: boolean;
  seen: ReadonlySet<string>;
  /** The changes nobody has clicked away yet. */
  unseen: Change[];
}

export function useWhatsNew(): WhatsNew {
  const raw = useSyncExternalStore(subscribe, readRaw);
  return useMemo(() => {
    const { on, seen } = parse(raw);
    const known = new Set(seen);
    return { on, seen: known, unseen: CHANGES.filter((change) => !known.has(change.id)) };
  }, [raw]);
}
