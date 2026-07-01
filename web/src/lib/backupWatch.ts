// ---------------------------------------------------------------------------
// Fire-and-watch hook for single backups AND restores.
//
// Single "Back up now" — and, since issue #24, every restore — actions are
// ASYNC on the server: the POST returns immediately ({ok:true, started:true})
// and the work runs in a detached goroutine, so it survives the request
// connection dying — including the cases that bit users: backing up the
// reverse-proxy container the UI runs through severs the fetch, and a
// multi-hour restore-to-folder holds the request open until the browser/proxy
// drops it (which used to cancel the context and kill restic mid-restore). The
// button must therefore NOT await the whole run; it fires, then WATCHES for
// completion and reads the recorded run to learn the real outcome. It must
// never show "Failed to fetch" for work the server actually runs.
//
// Watching uses two signals, belt-and-suspenders:
//   1. the shared SSE progress store (useProgress) — the live bar; when the
//      entry for this key was seen and then clears, the run finished.
//   2. polling listRuns() — both as the outcome lookup (success vs failure,
//      snapshot id, error text) and as a fallback when SSE reports nothing
//      (a very fast run that completed before we subscribed, or a dropped
//      stream). We snapshot this target's existing run ids BEFORE firing and
//      treat the newest run that did NOT exist then as ours — correlating by a
//      NEW run, never by comparing the client clock to the server's startedAt
//      (clock skew made that match the wrong run or hang until timeout).
// ---------------------------------------------------------------------------

import { useCallback, useEffect, useRef, useState } from "react";
import { listRuns, type Run } from "./api";
import { useProgress } from "./progress";

export type BackupWatchState =
  | { phase: "idle" }
  | { phase: "pending" }
  | { phase: "success"; snapshotId?: string }
  | { phase: "error"; message: string };

/** The run kind being watched (matches the recorded run's `kind` field). */
export type WatchKind = "backup" | "restore";

/** How long success stays shown before auto-clearing (matches the old UX). */
const SUCCESS_CLEAR_MS = 4000;
/** Poll the runs list at this cadence while watching for completion. */
const POLL_INTERVAL_MS = 2000;
/** Give up watching after this long and report the last known run state. */
const WATCH_TIMEOUT_MS = 13 * 60 * 60 * 1000; // beyond the server's 12h backup/restore cap
/** Retry a busy fire (shared single-flight guard still releasing) this long. */
const BUSY_RETRY_MS = 30 * 1000;

/** A function that POSTs the start request (backup or restore). */
export type StartBackupFn = () => Promise<{ ok: boolean; error?: string; started?: boolean }>;

/** Picks the newest run for this target out of the runs list. */
export type RunMatcher = (run: Run) => boolean;

interface UseBackupWatchArgs {
  /** The progress key BombVault publishes for this run (e.g. "container:plex"). */
  progressKey: string;
  /** Fires the start request (backupNow / restore / restoreVM / …). */
  start: StartBackupFn;
  /** True for the run that belongs to this target (domain + name). */
  matchRun: RunMatcher;
  /** Which run kind to watch. Defaults to "backup" (backward-compatible). */
  kind?: WatchKind;
  /** Called once on successful completion so the caller can refresh its list. */
  onDone?: () => void;
}

/**
 * Drives one "Back up now" / "Restore" button: call `fire()` on click. Returns
 * the current display state. Determines success vs failure from the recorded
 * run, never from the (now fire-and-forget) POST response.
 */
export function useBackupWatch({ progressKey, start, matchRun, kind = "backup", onDone }: UseBackupWatchArgs) {
  const [state, setState] = useState<BackupWatchState>({ phase: "idle" });
  const progress = useProgress();
  const entry = progress[progressKey];

  // Watch bookkeeping kept in refs so the polling effect doesn't re-subscribe.
  const watching = useRef(false);
  const sawProgress = useRef(false);
  // Run ids that already existed for this target the moment we fired. The new
  // run is whichever matching run is NOT in this set — correlation by identity,
  // not by clock. null = the pre-fire snapshot failed; seeded lazily on first poll.
  const baselineIds = useRef<Set<string> | null>(null);
  const matchRef = useRef(matchRun);
  const kindRef = useRef(kind);
  const onDoneRef = useRef(onDone);
  matchRef.current = matchRun;
  kindRef.current = kind;
  onDoneRef.current = onDone;

  const finish = useCallback((next: BackupWatchState) => {
    watching.current = false;
    sawProgress.current = false;
    setState(next);
    if (next.phase === "success") {
      onDoneRef.current?.();
      setTimeout(() => setState({ phase: "idle" }), SUCCESS_CLEAR_MS);
    }
  }, []);

  // Look up the outcome from the recorded runs. Returns true once a terminal
  // (success/failed) run for this target — one that did not exist when we fired
  // — is found.
  const resolveFromRuns = useCallback(async (): Promise<boolean> => {
    try {
      const res = await listRuns();
      if (!res.ok || !res.runs) return false;
      const mine = (r: Run) => r.kind === kindRef.current && matchRef.current(r);
      // If the pre-fire snapshot failed, seed the baseline from the current runs
      // now and wait for the NEXT one — never resolve against a pre-existing run.
      if (baselineIds.current === null) {
        baselineIds.current = new Set(res.runs.filter(mine).map((r) => r.id));
        return false;
      }
      const base = baselineIds.current;
      // Runs come newest-first; the newest matching run absent at fire time is ours.
      const run = res.runs.find((r) => mine(r) && !base.has(r.id));
      if (!run) return false;
      if (run.status === "success") {
        finish({ phase: "success", snapshotId: run.snapshotId || undefined });
        return true;
      }
      if (run.status === "failed") {
        finish({
          phase: "error",
          message: run.error || (kindRef.current === "restore" ? "Restore failed" : "Backup failed"),
        });
        return true;
      }
      return false; // still running
    } catch {
      return false; // transient network error — keep polling
    }
  }, [finish]);

  // Record that the live progress entry appeared so we know to look for its
  // disappearance as the completion edge. Runs on every progress map change.
  useEffect(() => {
    if (!watching.current) return;
    if (entry && entry.active) {
      sawProgress.current = true;
    }
  }, [entry]);

  // Drive a single fire+watch cycle. Polls runs on an interval until a terminal
  // run is found, the watch times out, or the component unmounts.
  const fire = useCallback(async () => {
    if (watching.current) return;
    setState({ phase: "pending" });
    // Snapshot this target's existing run ids BEFORE firing, so the watch can
    // pick out the run we are about to start by identity. If this fails, leave
    // the baseline null — resolveFromRuns seeds it lazily on the first poll.
    baselineIds.current = null;
    try {
      const before = await listRuns();
      if (before.ok && before.runs) {
        baselineIds.current = new Set(
          before.runs.filter((r) => r.kind === kindRef.current && matchRef.current(r)).map((r) => r.id)
        );
      }
    } catch {
      // ignore — lazy seeding covers it.
    }
    let res: Awaited<ReturnType<StartBackupFn>>;
    try {
      res = await start();
    } catch (err) {
      setState({ phase: "error", message: err instanceof Error ? err.message : "Network error" });
      return;
    }
    if (!res.ok) {
      setState({
        phase: "error",
        message: res.error ?? (kindRef.current === "restore" ? "Restore failed" : "Backup failed"),
      });
      return;
    }
    // Server accepted the job and is now running it detached.
    sawProgress.current = false;
    watching.current = true;

    const startedAt = Date.now();
    const poll = async () => {
      if (!watching.current) return;
      const done = await resolveFromRuns();
      if (done || !watching.current) return;
      if (Date.now() - startedAt > WATCH_TIMEOUT_MS) {
        finish({
          phase: "error",
          message: `Timed out waiting for the ${kindRef.current} to finish`,
        });
        return;
      }
      setTimeout(() => void poll(), POLL_INTERVAL_MS);
    };
    // Kick the first poll soon; a very fast run may already be recorded.
    setTimeout(() => void poll(), 600);
  }, [start, resolveFromRuns, finish]);

  // Clear a lingering success/error banner (e.g. when the user changes the
  // selection a stale result would misdescribe). No-op while a watch runs —
  // the in-flight state must stay visible.
  const reset = useCallback(() => {
    if (!watching.current) setState({ phase: "idle" });
  }, []);

  // Stop watching if the button unmounts mid-run (the server keeps going).
  useEffect(() => {
    return () => {
      watching.current = false;
    };
  }, []);

  return { state, fire, reset, isPending: state.phase === "pending" };
}

// ---------------------------------------------------------------------------
// fireAndWaitRun — the non-hook variant for BULK loops.
//
// Bulk "back up / restore selected" actions run their targets one after
// another. The starts are async and share the server's single-flight guard, so
// firing them in a tight loop would make every call after the first hit
// "already running". This helper fires ONE run (retrying briefly while the
// previous target's guard is still releasing — the guard clears just after the
// run goes terminal, not the instant we observe it), then waits for the NEW
// recorded run to reach a terminal state before returning. Correlates by a new
// run id, never by the client clock (skew matched the wrong or last run).
// ---------------------------------------------------------------------------

export async function fireAndWaitRun(opts: {
  kind: WatchKind;
  matchRun: RunMatcher;
  start: StartBackupFn;
}): Promise<{ ok: boolean; error?: string }> {
  const mine = (r: Run) => r.kind === opts.kind && opts.matchRun(r);
  let baseline = new Set<string>();
  try {
    const before = await listRuns();
    baseline = new Set((before.runs ?? []).filter(mine).map((r) => r.id));
  } catch {
    // ignore — fall back to the first terminal run for this target.
  }
  // Fire, retrying only while the guard is still busy from the previous target.
  const fireDeadline = Date.now() + BUSY_RETRY_MS;
  for (;;) {
    let res: Awaited<ReturnType<StartBackupFn>>;
    try {
      res = await opts.start();
    } catch (err) {
      return { ok: false, error: err instanceof Error ? err.message : "Network error" };
    }
    if (res.ok) break;
    const busy = (res.error ?? "").toLowerCase().includes("already running");
    if (!busy || Date.now() > fireDeadline) return { ok: false, error: res.error };
    await new Promise((r) => setTimeout(r, 1000));
  }
  // Poll the recorded runs until this target's NEW run reaches a terminal state.
  const deadline = Date.now() + WATCH_TIMEOUT_MS;
  for (;;) {
    await new Promise((r) => setTimeout(r, POLL_INTERVAL_MS));
    try {
      const runs = await listRuns();
      const run = runs.runs?.find((r) => mine(r) && !baseline.has(r.id));
      if (run && run.status === "success") return { ok: true };
      if (run && run.status === "failed") return { ok: false, error: run.error };
    } catch {
      // transient — keep polling
    }
    if (Date.now() > deadline) return { ok: false, error: `Timed out waiting for the ${opts.kind} to finish` };
  }
}
