// ---------------------------------------------------------------------------
// Live backup/restore progress — a single shared SSE connection.
//
// The backend streams Server-Sent Events on GET /api/progress (same-origin,
// cookies flow automatically). Each message body is one JSON object:
//
//   { "key": "container:plex", "phase": "backup", "percent": 42.5, "active": true }
//
// We keep ONE module-level EventSource for the whole app and ref-count its
// subscribers so multiple cards/rows don't each open their own connection.
// `useProgress()` returns a map keyed by the event `key` field; consumers index
// it (e.g. `progress["container:plex"]`).
// ---------------------------------------------------------------------------

import { useEffect, useState } from "react";

export type ProgressPhase = "backup" | "restore";

export interface ProgressState {
  phase: ProgressPhase;
  percent: number;
  active: boolean;
}

export type ProgressMap = Record<string, ProgressState>;

// Shape of a single SSE payload.
interface ProgressEvent extends ProgressState {
  key: string;
}

// How long an inactive (completed) entry lingers so the bar can visibly reach
// 100% before it fades out, then gets dropped from the map entirely.
const COMPLETE_LINGER_MS = 800;

// ---------------------------------------------------------------------------
// Module-level shared state
// ---------------------------------------------------------------------------

let current: ProgressMap = {};
const listeners = new Set<(map: ProgressMap) => void>();
const dropTimers = new Map<string, ReturnType<typeof setTimeout>>();

let source: EventSource | null = null;
let refCount = 0;

function emit(): void {
  for (const listener of listeners) listener(current);
}

function applyEvent(ev: ProgressEvent): void {
  // An existing drop timer for this key is stale once a fresh event arrives.
  const pending = dropTimers.get(ev.key);
  if (pending) {
    clearTimeout(pending);
    dropTimers.delete(ev.key);
  }

  // Keep the bar visible for a brief linger after completion, then drop the
  // entry so it disappears. We mirror the REPORTED percent (the backend sends
  // 100 on success and 0 on failure) rather than forcing 100 — otherwise a
  // failed/cancelled backup would flash a full green bar. Consumers render the
  // bar only while `active` is true, so we hold `active` during the linger.
  const entry: ProgressState = { phase: ev.phase, percent: ev.percent, active: true };

  current = { ...current, [ev.key]: entry };
  emit();

  if (!ev.active) {
    const timer = setTimeout(() => {
      dropTimers.delete(ev.key);
      const next = { ...current };
      delete next[ev.key];
      current = next;
      emit();
    }, COMPLETE_LINGER_MS);
    dropTimers.set(ev.key, timer);
  }
}

function handleMessage(e: MessageEvent<string>): void {
  let parsed: unknown;
  try {
    parsed = JSON.parse(e.data);
  } catch {
    return; // ignore malformed lines
  }
  if (
    parsed &&
    typeof parsed === "object" &&
    typeof (parsed as ProgressEvent).key === "string"
  ) {
    const ev = parsed as ProgressEvent;
    applyEvent({
      key: ev.key,
      phase: ev.phase === "restore" ? "restore" : "backup",
      percent: typeof ev.percent === "number" ? ev.percent : 0,
      active: !!ev.active,
    });
  }
}

function openSource(): void {
  // EventSource reconnects on transient errors natively; guard against opening
  // a second connection.
  if (source) return;
  source = new EventSource("/api/progress");
  source.onmessage = handleMessage;
  // No onerror teardown: EventSource auto-reconnects. We only close on the last
  // unsubscribe (closeSource).
}

function closeSource(): void {
  if (source) {
    source.close();
    source = null;
  }
  // Drop cached state + pending linger timers so a later remount starts clean
  // and is repopulated by the backend's snapshot replay on reconnect. Without
  // this, a backup that finished while the page was unmounted would reappear as
  // a frozen bar (no live stream and no completion event to clear it).
  for (const timer of dropTimers.values()) clearTimeout(timer);
  dropTimers.clear();
  current = {};
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

/**
 * Subscribe to the shared progress stream. Returns a map of every active (and
 * just-completed) target keyed by its `key` (e.g. "container:plex", "vm:win11",
 * "flash"). Index it directly in the consumer.
 */
/**
 * anyActive reports whether ANY tracked target is currently running a backup,
 * restore, or replication — a broad "something is in flight" signal used to
 * disable start buttons and show a busy hint. Returns the first matching phase so
 * the caller can word the hint ("a restore is running" vs "a backup is running").
 */
export function anyActive(
  map: Record<string, { phase: string; active: boolean }>
): { active: boolean; phase?: string } {
  for (const k of Object.keys(map)) {
    const e = map[k];
    if (e.active && (e.phase === "backup" || e.phase === "restore" || e.phase === "replicate")) {
      return { active: true, phase: e.phase };
    }
  }
  return { active: false };
}

export function useProgress(): ProgressMap {
  const [map, setMap] = useState<ProgressMap>(current);

  useEffect(() => {
    const listener = (next: ProgressMap) => setMap(next);
    listeners.add(listener);

    refCount += 1;
    if (refCount === 1) openSource();

    // Sync immediately in case events arrived before this subscriber mounted.
    setMap(current);

    return () => {
      listeners.delete(listener);
      refCount -= 1;
      if (refCount === 0) closeSource();
    };
  }, []);

  return map;
}
