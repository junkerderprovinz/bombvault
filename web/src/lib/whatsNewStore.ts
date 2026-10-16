// Which changes this browser has seen, and the release it came from at the
// last update. It is kept per browser: in the look that syncs through the
// server one person's click would hide a dot for everybody.

import { useMemo, useSyncExternalStore } from "react";
import { CHANGES, type Change } from "./whatsNew";

const KEY = "bv-whats-new";
// Older versions kept one slot per release, named after it. The first write takes it over.
const LEGACY = "bv-whats-new@";

interface Stored {
  seen: string[];
  /** Changes whose page, when it was last open, drew neither them nor a way
   *  to them. */
  absent: string[];
  /** The release this browser saw before the last update, as x.y.z. */
  from?: string;
}

function readRaw(): string {
  try {
    const raw = localStorage.getItem(KEY);
    if (raw !== null) return raw;
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (key?.startsWith(LEGACY)) return localStorage.getItem(key) ?? "";
    }
  } catch {
    // Without storage nothing is stored.
  }
  return "";
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
      const { seen, absent, from } = stored as Record<string, unknown>;
      return { seen: ids(seen), absent: ids(absent), ...(typeof from === "string" ? { from } : {}) };
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
    for (let i = localStorage.length - 1; i >= 0; i--) {
      const key = localStorage.key(i);
      if (key?.startsWith(LEGACY)) localStorage.removeItem(key);
    }
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
export function releaseOf(version: string): string | null {
  return /\d+\.\d+\.\d+/.exec(version)?.[0] ?? null;
}

/** newer tells whether release a came after release b, both as x.y.z. */
export function newer(a: string, b: string): boolean {
  const [x, y] = [a.split(".").map(Number), b.split(".").map(Number)];
  for (let i = 0; i < 3; i++) if (x[i] !== y[i]) return x[i] > y[i];
  return false;
}

/**
 * beginWhatsNew is told the release this browser saw before an update. Every
 * change of a later release is new to it, and what it had seen or not of the
 * releases before stays as it was.
 */
export function beginWhatsNew(previous: string): void {
  const from = releaseOf(previous);
  if (!from) return;
  const stored = parse(readRaw());
  const before = new Set(CHANGES.filter((change) => !newer(change.since, from)).map((change) => change.id));
  write({ seen: stored.seen.filter((id) => before.has(id)), absent: stored.absent, from });
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
  /** The release this browser saw before the last update, or null. */
  from: string | null;
}

export function useWhatsNew(): WhatsNew {
  const raw = useSyncExternalStore(subscribe, readRaw);
  return useMemo(() => {
    const { seen, absent, from } = parse(raw);
    const known = new Set(seen);
    return { seen: known, absent: new Set(absent), unseen: CHANGES.filter((change) => !known.has(change.id)), from: from ?? null };
  }, [raw]);
}

// One window for the whole app, opened by an update and by the Version card's
// button, so the two cannot show different things.
let open = false;
const windowListeners = new Set<() => void>();

export function openWhatsNew(next: boolean): void {
  open = next;
  for (const listener of windowListeners) listener();
}

export function useWhatsNewOpen(): boolean {
  return useSyncExternalStore(
    (listener) => {
      windowListeners.add(listener);
      return () => windowListeners.delete(listener);
    },
    () => open,
  );
}
