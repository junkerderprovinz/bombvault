// ---------------------------------------------------------------------------
// Live backup/restore progress — a single shared SSE connection.
//
// The backend streams Server-Sent Events on GET /api/progress (same-origin,
// cookies flow automatically). Each message body is one JSON object:
//
//   { "key": "container:plex", "phase": "backup", "percent": 42.5, "active": true }
//
// Some keys (currently: off-site replication, "offsite:<domain>" — see
// issue #159) also carry a "startedAt" (Unix SECONDS, omitted when unset) so a
// consumer can render a live elapsed duration for the whole run, PLUS —
// whenever a live per-snapshot signal is available — "snapshotIndex" (1-based)
// and "snapshotTotal" (a best-effort candidate count; restic itself never
// reports a whole-run total across snapshots — see restic.Copy's doc comment)
// alongside a REAL "percent" scoped to that one snapshot's own pack-copy
// progress (not a fabrication: restic copy genuinely prints this, once
// RESTIC_PROGRESS_FPS is wired up the same way backup/restore already get
// it — a first cut of this feature concluded otherwise).
//
// We keep ONE module-level EventSource for the whole app and ref-count its
// subscribers so multiple cards/rows don't each open their own connection.
// `useProgress()` returns a map keyed by the event `key` field; consumers index
// it (e.g. `progress["container:plex"]`).
// ---------------------------------------------------------------------------

import { useEffect, useState } from "react";

export type ProgressPhase = "backup" | "restore" | "replicate" | "maintenance";

export interface ProgressState {
  phase: ProgressPhase;
  percent: number;
  active: boolean;
  // Browser timestamp (Date.now()) of the last event seen for this key. Used to
  // age out an entry whose terminal SSE frame was lost (network blip, restic
  // crash, a reconnect where the clear never ran) so a stuck active:true entry
  // can't disable every bulk button app-wide until a reload. See STALE_MS.
  lastSeen: number;
  // Epoch SECONDS (backend's time.Now().Unix(), not Date.now()'s ms) the
  // operation began, repeated on every event for this key — including the
  // terminal one. Undefined for a key whose events don't carry it — treat
  // that as "unknown", never as 0/epoch (see lib/reltime.ts's elapsedSince,
  // which enforces this). Lets a consumer render a live elapsed duration
  // (issue #159) for the whole run.
  startedAt?: number;
  // Set only for off-site replication ("offsite:<domain>", phase
  // "replicate") once a live per-snapshot signal is available — see
  // restic.Copy's doc comment for the whole story: restic copy DOES print a
  // real, parseable per-snapshot pack-copy percentage, it just needed the
  // same RESTIC_PROGRESS_FPS wiring backup/restore already had. `percent` is
  // then scoped to the CURRENT snapshot (snapshotIndex, 1-based) of an
  // estimated snapshotTotal (a best-effort candidate count the backend
  // computes; restic itself never reports a whole-run total across
  // snapshots). Both undefined whenever no such signal is available yet
  // (e.g. restic is still walking the source tree) or for every other phase.
  snapshotIndex?: number;
  snapshotTotal?: number;
}

export type ProgressMap = Record<string, ProgressState>;

// ---------------------------------------------------------------------------
// Off-site run-level progress (issue #159)
// ---------------------------------------------------------------------------

/** One honest, run-level view of an in-flight off-site replication. */
export interface OffsiteRunProgress {
  /** Whole-run completion, 0..99 (see offsiteRunProgress for the 99 cap). */
  percent: number;
  /** 1-based index of the snapshot restic is copying right now. */
  index: number;
  /** Best-effort count of snapshots this run set out to copy. */
  total: number;
}

/**
 * offsiteRunProgress derives ONE run-level completion percentage for an
 * "offsite:<domain>" progress state, or null when no honest one exists.
 *
 * Issue #159 shipped the raw wire values straight to two surfaces, rendered as
 * "snapshot 15 of 126 (55%)". Neither number was miscalculated — they measure
 * genuinely different things, both correctly:
 *   - index/total is the SNAPSHOT count: restic is on the 15th of an estimated
 *     126 snapshots that still need copying.
 *   - percent is the PACK count WITHIN snapshot 15, which restarts at 0 for
 *     every snapshot (restic copy has no whole-run total; see
 *     internal/progress/progress.go's Event doc comment).
 * Side by side, with the percentage in parentheses right after the fraction,
 * every reader parses it as "15/126 = 55%" — and it self-contradicts, since
 * real overall progress there is ~12%. Over a one-hour run the percentage also
 * sawtoothed 0→100 once per snapshot while "of 126" crawled, which reads as
 * simply broken.
 *
 * So the two are COMBINED here instead of being shown side by side: the packs
 * done inside the current snapshot are the fractional part of the snapshots
 * done overall. The resulting number now AGREES with the "15 of 126" beside it
 * (~12% either way) instead of fighting it — which is the whole point.
 *
 * Honesty constraints, in order of how much they matter:
 *   - A total of 0/undefined is "the backend could not estimate" (see
 *     api.progBeginCopySink, which publishes that unknown rather than
 *     fabricating one) — there is no denominator, so this returns null and the
 *     caller falls back to its duration-only text. Dividing by a made-up total
 *     would read ~99% for a run that had barely started.
 *   - `total` is still only an estimate, and snapshots differ wildly in size,
 *     so this is snapshot-COUNT progress, not byte progress. Surfaces that show
 *     it say so (OffsiteIndicator's info bubble).
 *   - Capped at 99 while a run is live: the copy is not the whole job (off-site
 *     retention, unlock and the run record all follow), and a bar parked at
 *     100% for minutes is exactly the "it's stuck" impression #159 was about.
 *     Reaching a real 100% is the terminal event's job — it carries no
 *     snapshotIndex, so this returns null for it anyway.
 */
export function offsiteRunProgress(state: ProgressState | undefined): OffsiteRunProgress | null {
  const index = state?.snapshotIndex;
  const percent = state?.percent;
  const total = state?.snapshotTotal;
  if (typeof index !== "number" || !Number.isFinite(index) || index < 1) return null;
  if (typeof percent !== "number" || !Number.isFinite(percent)) return null;
  if (typeof total !== "number" || !Number.isFinite(total) || total < 1) return null;
  // The live index is ground truth; the total is a guess. If the guess
  // undercounted, widen it rather than render "snapshot 3 of 2".
  const wideTotal = Math.max(total, index);
  const withinSnapshot = Math.max(0, Math.min(100, percent)) / 100;
  const overall = ((index - 1 + withinSnapshot) / wideTotal) * 100;
  return {
    percent: Math.min(99, Math.max(0, Math.round(overall))),
    index,
    total: wideTotal,
  };
}

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
  const entry: ProgressState = {
    phase: ev.phase,
    percent: ev.percent,
    active: true,
    lastSeen: Date.now(),
    startedAt: ev.startedAt,
    snapshotIndex: ev.snapshotIndex,
    snapshotTotal: ev.snapshotTotal,
  };

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
      // tracker refuses a backup while a replication runs). "maintenance"
      // (prune/verify/drill/tamper/flash-ZIP export) must ALSO stay distinct —
      // it must not masquerade as a backup, but it is deliberately excluded
      // from anyActive() below (see there). Anything else unknown collapses to
      // "backup".
      phase:
        ev.phase === "restore"
          ? "restore"
          : ev.phase === "replicate"
            ? "replicate"
            : ev.phase === "maintenance"
              ? "maintenance"
              : "backup",
      percent: typeof ev.percent === "number" ? ev.percent : 0,
      active: !!ev.active,
      startedAt: typeof ev.startedAt === "number" ? ev.startedAt : undefined,
      snapshotIndex: typeof ev.snapshotIndex === "number" ? ev.snapshotIndex : undefined,
      snapshotTotal: typeof ev.snapshotTotal === "number" ? ev.snapshotTotal : undefined,
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
 *
 * Deliberately excludes "maintenance" (prune/verify — and, since #109,
 * drill/tamper/flash-ZIP export, which publish the same phase): every caller
 * (BackupButton, Containers/VMs/Flash/Files pages, RestorePanel, RestoreAction)
 * uses this to busy-guard repo-writing backup/restore/replicate starts. A
 * running maintenance op is not that kind of conflict (it takes the per-domain
 * lock itself and reports "busy" cleanly), so it must not disable the bulk
 * start buttons app-wide.
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
