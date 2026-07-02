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

export type ProgressPhase = "backup" | "restore" | "replicate";

export interface ProgressState {
  phase: ProgressPhase;
  percent: number;
  active: boolean;
  // Browser timestamp (Date.now()) of the last event seen for this key. Used to
  // age out an entry whose terminal SSE frame was lost (network blip, restic
  // crash, a reconnect where the clear never ran) so a stuck active:true entry
  // can't disable every bulk button app-wide until a reload. See STALE_MS.
  lastSeen: number;
}

export type ProgressMap = Record<string, ProgressState>;

// Shape of a single SSE payload. lastSeen is stamped locally in applyEvent, so
// it is not part of the wire shape.
type ProgressEvent = Omit<ProgressState, "lastSeen"> & { key: string };

// How long an inactive (completed) entry lingers so the bar can visibly reach
// 100% before it fades out, then gets dropped from the map entirely.
const COMPLETE_LINGER_MS = 800;

// An entry with no event for this long is treated as NOT active by anyActive():
// its terminal frame was almost certainly lost. restic streams progress at
// RESTIC_PROGRESS_FPS=3 (~every 0.33s) while a run is live, so 15s without a
// frame comfortably means "no longer running" without racing a slow tick.
export const STALE_MS = 15000;

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
  const entry: ProgressState = { phase: ev.phase, percent: ev.percent, active: true, lastSeen: Date.now() };

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
      // Preserve the real domain: a "replicate" (off-site) phase must stay
      // distinct so anyActive can word the busy hint correctly (the activity
      // tracker refuses a backup while a replication runs). Anything unknown
      // collapses to "backup".
      phase: ev.phase === "restore" ? "restore" : ev.phase === "replicate" ? "replicate" : "backup",
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
  map: Record<string, { phase: string; active: boolean; lastSeen?: number }>
): { active: boolean; phase?: string } {
  const now = Date.now();
  for (const k of Object.keys(map)) {
    const e = map[k];
    // A live entry whose last event is older than STALE_MS lost its terminal
    // frame — treat it as no longer active so it can't lock the bulk buttons
    // forever. (lastSeen is always set by applyEvent; the optional type only
    // keeps this callable with looser shapes.)
    const stale = e.lastSeen !== undefined && now - e.lastSeen > STALE_MS;
    if (e.active && !stale && (e.phase === "backup" || e.phase === "restore" || e.phase === "replicate")) {
      return { active: true, phase: e.phase };
    }
  }
  return { active: false };
}

/**
 * busyPhraseKey maps an anyActive() phase to the i18n key for the "something is
 * running" hint, so every busy hint (bulk bars, per-item buttons) words it the
 * same way — including the off-site "replication is running" case.
 */
export function busyPhraseKey(
  phase?: string
): "common.restoreRunning" | "common.replicateRunning" | "common.backupRunning" {
  if (phase === "restore") return "common.restoreRunning";
  if (phase === "replicate") return "common.replicateRunning";
  return "common.backupRunning";
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
