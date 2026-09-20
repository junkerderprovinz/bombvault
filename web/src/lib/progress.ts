// Live backup and restore progress over one shared SSE connection.
//
// GET /api/progress streams one JSON object per message:
//
//   { "key": "container:plex", "phase": "backup", "percent": 42.5, "active": true }
//
// Off-site replication keys ("offsite:<domain>") also carry "startedAt" (Unix
// seconds) and, once restic reports per-snapshot progress, "snapshotIndex" and
// "snapshotTotal". "percent" is then the current snapshot's own pack-copy
// progress, since restic has no whole-run total (see restic.Copy).
//
// One module-level EventSource serves the whole app and is ref-counted, so
// cards and rows do not each open a connection. useProgress() returns a map
// keyed by the event's `key`.

import { useEffect, useState } from "react";

export type ProgressPhase = "backup" | "restore" | "replicate" | "maintenance";

export interface ProgressState {
  phase: ProgressPhase;
  percent: number;
  active: boolean;
  // Browser time (Date.now()) of the last event for this key. anyActive uses it
  // to age out an entry whose terminal frame was lost (network blip, restic
  // crash, a reconnect where the clear never ran), which would otherwise
  // disable every bulk button until a reload. See STALE_MS.
  lastSeen: number;
  // Epoch seconds (the backend's time.Now().Unix(), not milliseconds) when the
  // operation began, repeated on every event for the key including the
  // terminal one. Undefined means unknown, never 0 (see reltime.ts's
  // elapsedSince).
  startedAt?: number;
  // Set only for off-site replication once restic reports per-snapshot
  // progress: `percent` is then scoped to snapshot snapshotIndex (1-based) of
  // an estimated snapshotTotal. Undefined while restic is still walking the
  // source tree and for every other phase.
  snapshotIndex?: number;
  snapshotTotal?: number;
  // A step inside the phase that has no percentage of its own: a database dump
  // streams straight into the repository, so `bytes` is all there is to show
  // and `percent` stays 0.
  stage?: ProgressStage;
  bytes?: number;
}

/** The steps that report bytes instead of a percentage. */
export type ProgressStage = "dbdump" | "dbdumpsave" | "dbimport";

const STAGES: ProgressStage[] = ["dbdump", "dbdumpsave", "dbimport"];

export type ProgressMap = Record<string, ProgressState>;

/** Run-level view of an in-flight off-site replication. */
export interface OffsiteRunProgress {
  /** Whole-run completion, 0..99 (see offsiteRunProgress for the 99 cap). */
  percent: number;
  /** 1-based index of the snapshot restic is copying right now. */
  index: number;
  /** Best-effort count of snapshots this run set out to copy. */
  total: number;
}

/**
 * offsiteRunProgress derives one run-level completion percentage for an
 * "offsite:<domain>" progress state, or null when the state does not support
 * one.
 *
 * The wire carries two different measures: index/total counts snapshots, and
 * percent counts packs within the current snapshot, restarting at 0 for each
 * one. Shown side by side ("snapshot 15 of 126 (55%)") they read as a
 * contradiction, and the percentage sawtooths once per snapshot. Here the packs
 * done inside the current snapshot become the fractional part of the snapshots
 * done, so the result agrees with "15 of 126" (about 12%).
 *
 * A total of 0 or undefined means the backend could not estimate one
 * (api.progBeginCopySink), so this returns null rather than divide by a guess.
 * The total is still an estimate and snapshots vary in size, so this is
 * snapshot-count progress, not byte progress. It is capped at 99 while the run
 * is live because retention, unlock and the run record still follow the copy;
 * the terminal event carries no snapshotIndex and returns null anyway.
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
type ProgressFrame = Omit<ProgressState, "lastSeen"> & { key: string };

// How long an inactive (completed) entry lingers so the bar can visibly reach
// 100% before it fades out, then gets dropped from the map entirely.
const COMPLETE_LINGER_MS = 800;

// anyActive treats an entry with no event for this long as finished: its
// terminal frame was almost certainly lost. restic streams progress at
// RESTIC_PROGRESS_FPS=3 (every 0.33s or so) while a run is live, so 15s
// without a frame means it has stopped without racing a slow tick.
export const STALE_MS = 15000;

let current: ProgressMap = {};
const listeners = new Set<(map: ProgressMap) => void>();
const dropTimers = new Map<string, ReturnType<typeof setTimeout>>();

let source: EventSource | null = null;
let refCount = 0;

function emit(): void {
  for (const listener of listeners) listener(current);
}

function applyEvent(ev: ProgressFrame): void {
  // An existing drop timer for this key is stale once a fresh event arrives.
  const pending = dropTimers.get(ev.key);
  if (pending) {
    clearTimeout(pending);
    dropTimers.delete(ev.key);
  }

  // The reported percent is kept as is (100 on success, 0 on failure) rather
  // than forced to 100, or a failed backup would flash a full bar. Consumers
  // render the bar only while `active` is true, so the entry stays active
  // through the linger and is dropped afterwards.
  const entry: ProgressState = {
    phase: ev.phase,
    percent: ev.percent,
    active: true,
    lastSeen: Date.now(),
    startedAt: ev.startedAt,
    snapshotIndex: ev.snapshotIndex,
    snapshotTotal: ev.snapshotTotal,
    stage: ev.stage,
    bytes: ev.bytes,
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

/**
 * parseProgressFrame narrows one SSE payload to what this client understands,
 * or returns null for a line it cannot place. A stage from a newer backend is
 * dropped rather than carried, so nothing renders a step it has no words for.
 */
export function parseProgressFrame(data: string): ProgressFrame | null {
  let parsed: unknown;
  try {
    parsed = JSON.parse(data);
  } catch {
    return null;
  }
  if (!parsed || typeof parsed !== "object" || typeof (parsed as ProgressFrame).key !== "string") {
    return null;
  }
  const ev = parsed as ProgressFrame;
  return {
    key: ev.key,
    // "replicate" stays distinct so anyActive can word the busy hint (a
    // backup is refused while a replication runs). "maintenance" (prune,
    // verify, drill, tamper check, flash ZIP export) stays distinct so it is
    // not mistaken for a backup; anyActive ignores it. Anything else counts
    // as "backup".
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
    stage: ev.stage && STAGES.includes(ev.stage) ? ev.stage : undefined,
    bytes: typeof ev.bytes === "number" ? ev.bytes : undefined,
  };
}

function handleMessage(e: MessageEvent<string>): void {
  const frame = parseProgressFrame(e.data);
  if (frame) applyEvent(frame);
}

function openSource(): void {
  if (source) return;
  source = new EventSource("/api/progress");
  source.onmessage = handleMessage;
  // No onerror teardown: EventSource reconnects by itself, and closeSource
  // runs on the last unsubscribe.
}

function closeSource(): void {
  if (source) {
    source.close();
    source = null;
  }
  // Drop cached state and pending linger timers so a later remount starts clean
  // and is repopulated by the backend's snapshot replay on reconnect. Without
  // this, a backup that finished while the page was unmounted would reappear as
  // a frozen bar (no live stream and no completion event to clear it).
  for (const timer of dropTimers.values()) clearTimeout(timer);
  dropTimers.clear();
  current = {};
}

/**
 * anyActive reports whether any tracked target is running a backup, restore or
 * replication, and returns the first phase found so the caller can word its
 * busy hint ("a restore is running" rather than "a backup is running"). Callers
 * use it to disable the buttons that start repo-writing work.
 *
 * "maintenance" is left out: it takes the per-domain lock itself and reports
 * busy cleanly, so it must not disable the start buttons app-wide.
 */
export function anyActive(
  map: Record<string, { phase: string; active: boolean; lastSeen?: number; stage?: ProgressStage }>
): { active: boolean; phase?: string; stage?: ProgressStage } {
  const now = Date.now();
  for (const k of Object.keys(map)) {
    const e = map[k];
    // An entry silent for longer than STALE_MS lost its terminal frame and must
    // not lock the bulk buttons forever. lastSeen is optional only so looser
    // shapes can be passed in; applyEvent always sets it.
    const stale = e.lastSeen !== undefined && now - e.lastSeen > STALE_MS;
    if (e.active && !stale && (e.phase === "backup" || e.phase === "restore" || e.phase === "replicate")) {
      return { active: true, phase: e.phase, stage: e.stage };
    }
  }
  return { active: false };
}

/** The busy hints, from the most precise step to the phase around it. */
export type BusyPhraseKey =
  | "dbdump.busyDumping"
  | "dbdump.busySaving"
  | "dbdump.busyImporting"
  | "common.restoreRunning"
  | "common.replicateRunning"
  | "common.backupRunning";

/**
 * busyPhraseKey maps an anyActive() phase to the i18n key for the busy hint, so
 * every hint (bulk bars, per-item buttons) words it the same way, including the
 * off-site "replication is running" case. A stage names the step inside the
 * phase, so an hour of "a backup is running" reads as the dump it is.
 */
export function busyPhraseKey(phase?: string, stage?: ProgressStage): BusyPhraseKey {
  if (stage === "dbdump") return "dbdump.busyDumping";
  if (stage === "dbdumpsave") return "dbdump.busySaving";
  if (stage === "dbimport") return "dbdump.busyImporting";
  if (phase === "restore") return "common.restoreRunning";
  if (phase === "replicate") return "common.replicateRunning";
  return "common.backupRunning";
}

/**
 * Subscribes to the shared progress stream. Returns a map of every active (and
 * just-completed) target keyed by its `key`, e.g. "container:plex", "vm:win11"
 * or "flash".
 */
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
