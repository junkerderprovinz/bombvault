// ---------------------------------------------------------------------------
// Fire-and-watch single-backup hook.
//
// Single "Back up now" actions are ASYNC on the server: the POST returns
// immediately ({ok:true, started:true}) and the backup runs in a detached
// goroutine, so it survives the request connection dying — including the case
// that bit a user, where backing up the reverse-proxy container the UI runs
// through severs this very fetch while the backup keeps going. The button must
// therefore NOT await the whole backup; it fires, then WATCHES for completion
// and reads the recorded run to learn the real outcome. It must never show
// "Failed to fetch" for a backup the server actually runs.
//
// Watching uses two signals, belt-and-suspenders:
//   1. the shared SSE progress store (useProgress) — the live bar; when the
//      entry for this key was seen and then clears, the backup finished.
//   2. polling listRuns() — both as the outcome lookup (success vs failure,
//      snapshot id, error text) and as a fallback when SSE reports nothing
//      (a very fast backup that completed before we subscribed, or a dropped
//      stream). The newest matching backup run started at/after the fire time
//      is the authority on the result.
// ---------------------------------------------------------------------------

import { useCallback, useEffect, useRef, useState } from "react";
import { listRuns, type Run } from "./api";
import { useProgress } from "./progress";

export type BackupWatchState =
  | { phase: "idle" }
  | { phase: "pending" }
  | { phase: "success"; snapshotId?: string }
  | { phase: "error"; message: string };

/** How long success stays shown before auto-clearing (matches the old UX). */
const SUCCESS_CLEAR_MS = 4000;
/** Poll the runs list at this cadence while watching for completion. */
const POLL_INTERVAL_MS = 2000;
/** Give up watching after this long and report the last known run state. */
const WATCH_TIMEOUT_MS = 13 * 60 * 60 * 1000; // beyond the server's 12h backup cap
/** Slack subtracted from the fire time when matching a run (clock skew). */
const FIRE_SLACK_SEC = 5;

/** A function that POSTs the single-backup start request. */
export type StartBackupFn = () => Promise<{ ok: boolean; error?: string; started?: boolean }>;

/** Picks the newest backup run for this target out of the runs list. */
export type RunMatcher = (run: Run) => boolean;

interface UseBackupWatchArgs {
  /** The progress key BombVault publishes for this backup (e.g. "container:plex"). */
  progressKey: string;
  /** Fires the start request (backupNow / backupVMNow / backupFlashNow). */
  start: StartBackupFn;
  /** True for the backup run that belongs to this target (domain + name). */
  matchRun: RunMatcher;
  /** Called once on successful completion so the caller can refresh its list. */
  onDone?: () => void;
}

/**
 * Drives one "Back up now" button: call `fire()` on click. Returns the current
 * display state. Determines success vs failure from the recorded run, never from
 * the (now fire-and-forget) POST response.
 */
export function useBackupWatch({ progressKey, start, matchRun, onDone }: UseBackupWatchArgs) {
  const [state, setState] = useState<BackupWatchState>({ phase: "idle" });
  const progress = useProgress();
  const entry = progress[progressKey];

  // Watch bookkeeping kept in refs so the polling effect doesn't re-subscribe.
  const watching = useRef(false);
  const sawProgress = useRef(false);
  const fireTime = useRef(0);
  const matchRef = useRef(matchRun);
  const onDoneRef = useRef(onDone);
  matchRef.current = matchRun;
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
  // (success/failed) run for this target — started at/after the fire — is found.
  const resolveFromRuns = useCallback(async (): Promise<boolean> => {
    try {
      const res = await listRuns();
      if (!res.ok || !res.runs) return false;
      const since = fireTime.current - FIRE_SLACK_SEC;
      // Runs come newest-first; the first match at/after the fire is ours.
      const run = res.runs.find(
        (r) => r.kind === "backup" && r.startedAt >= since && matchRef.current(r)
      );
      if (!run) return false;
      if (run.status === "success") {
        finish({ phase: "success", snapshotId: run.snapshotId || undefined });
        return true;
      }
      if (run.status === "failed") {
        finish({ phase: "error", message: run.error || "Backup failed" });
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
    let res: Awaited<ReturnType<StartBackupFn>>;
    try {
      res = await start();
    } catch (err) {
      setState({ phase: "error", message: err instanceof Error ? err.message : "Network error" });
      return;
    }
    if (!res.ok) {
      setState({ phase: "error", message: res.error ?? "Backup failed" });
      return;
    }
    // Server accepted the job and is now running it detached.
    fireTime.current = Math.floor(Date.now() / 1000);
    sawProgress.current = false;
    watching.current = true;

    const startedAt = Date.now();
    const poll = async () => {
      if (!watching.current) return;
      const done = await resolveFromRuns();
      if (done || !watching.current) return;
      if (Date.now() - startedAt > WATCH_TIMEOUT_MS) {
        finish({ phase: "error", message: "Timed out waiting for the backup to finish" });
        return;
      }
      setTimeout(() => void poll(), POLL_INTERVAL_MS);
    };
    // Kick the first poll soon; a very fast backup may already be recorded.
    setTimeout(() => void poll(), 600);
  }, [start, resolveFromRuns, finish]);

  // Stop watching if the button unmounts mid-backup (the server keeps going).
  useEffect(() => {
    return () => {
      watching.current = false;
    };
  }, []);

  return { state, fire, isPending: state.phase === "pending" };
}
