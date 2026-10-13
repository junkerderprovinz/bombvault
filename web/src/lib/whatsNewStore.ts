// Which of the release's changes this browser has seen. It is kept per
// release, so the next one starts with all of its dots, and per browser: in the
// look that syncs through the server one person's click would hide a dot for
// everybody.

import { useMemo, useSyncExternalStore } from "react";
import { CHANGES, WHATS_NEW_RELEASE, type Change } from "./whatsNew";

const PREFIX = "bv-whats-new@";
const KEY = PREFIX + WHATS_NEW_RELEASE;

interface Stored {
  seen: string[];
  /** Changes whose page, when it was last open, drew neither them nor a way
   *  to them. */
  absent: string[];
}

function readRaw(): string {
  try {
    return localStorage.getItem(KEY) ?? "";
  } catch {
    return "";
  }
}

function ids(list: unknown): string[] {
  return Array.isArray(list) ? list.filter((id): id is string => typeof id === "string") : [];
}

// Nothing is new to a browser that never saw an older release, so with nothing
// stored every change counts as seen.
function parse(raw: string): Stored {
  try {
    const stored: unknown = raw ? JSON.parse(raw) : null;
    if (stored && typeof stored === "object") {
      const { seen, absent } = stored as Record<string, unknown>;
      return { seen: ids(seen), absent: ids(absent) };
    }
  } catch {
    // Anything else in the slot counts as nothing stored.
  }
  return { seen: CHANGES.map((change) => change.id), absent: [] };
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
 * from one older than the list's release makes every change new, once: a
 * later patch of the same release leaves what was seen as it is. The
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
    // Without storage there is nothing to clear and nothing to mark.
  }
  write({ seen: [], absent: [] });
}

/** markSeen counts changes as seen, or as new again. Either way they are
 *  looked for afresh. */
export function markSeen(ids: string[], seen = true): void {
  const stored = parse(readRaw());
  const rest = stored.seen.filter((id) => !ids.includes(id));
  write({ ...stored, seen: seen ? [...rest, ...ids] : rest, absent: stored.absent.filter((id) => !ids.includes(id)) });
}

/**
 * markAbsent notes that the page of these changes showed neither them nor a
 * way to them, or that it shows them again. No dot leads to an absent change,
 * since the way would end at nothing.
 */
export function markAbsent(ids: string[], absent = true): void {
  const stored = parse(readRaw());
  const rest = stored.absent.filter((id) => !ids.includes(id));
  write({ ...stored, absent: absent ? [...rest, ...ids] : rest });
}

export interface WhatsNew {
  seen: ReadonlySet<string>;
  absent: ReadonlySet<string>;
  /** The changes nobody has clicked away yet. */
  unseen: Change[];
}

export function useWhatsNew(): WhatsNew {
  const raw = useSyncExternalStore(subscribe, readRaw);
  return useMemo(() => {
    const { seen, absent } = parse(raw);
    const known = new Set(seen);
    return { seen: known, absent: new Set(absent), unseen: CHANGES.filter((change) => !known.has(change.id)) };
  }, [raw]);
}

/** The two tabs of the window: the release notes, or the list of changes. */
export type NewsTab = "notes" | "changes";

// One window for the whole app, opened by an update and by the Version card's
// button, so the two cannot show different things.
let openTab: NewsTab | null = null;
const windowListeners = new Set<() => void>();

/** openWhatsNew opens the window on a tab, or closes it with null. */
export function openWhatsNew(tab: NewsTab | null): void {
  openTab = tab;
  for (const listener of windowListeners) listener();
}

/** The tab the window is open on, or null while it is closed. */
export function useWhatsNewOpen(): NewsTab | null {
  return useSyncExternalStore(
    (listener) => {
      windowListeners.add(listener);
      return () => windowListeners.delete(listener);
    },
    () => openTab,
  );
}
