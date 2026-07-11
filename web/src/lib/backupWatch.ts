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

import { useCallback, useEffect, useRef, useState, type MutableRefObject } from "react";
import { listRuns, type Run } from "./api";
import { useProgress } from "./progress";

export type BackupWatchState =
  | { phase: "idle" }
  | { phase: "pending" }
  | { phase: "success"; snapshotId?: string }
  // A user-cancelled restore is a NEUTRAL terminal (sticky, no red error banner):
  // the recorded run's status is "cancelled", distinct from a real failure.
  | { phase: "cancelled" }
  // The container was removed from the host but is still a target: the recorded
  // run's status is "skipped" — a NEUTRAL terminal (no false success, no red
  // error), so the watcher completes instead of spinning to timeout (#57).
  | { phase: "skipped" }
  | { phase: "error"; message: string };

/** The run kind being watched (matches the recorded run's `kind` field). */
export type WatchKind = "backup" | "restore";

/** How long a BACKUP success stays shown before auto-clearing (matches the old
 * UX). Restore banners are sticky — see finish(). */
const SUCCESS_CLEAR_MS = 4000;
/** Poll the runs list at this cadence while watching for completion. */
const POLL_INTERVAL_MS = 2000;
/** Give up watching after this long and report the last known run state — kept
 * just beyond the matching server-side hard cap (12h backups, 48h restores). */
const WATCH_TIMEOUT_BACKUP_MS = 13 * 60 * 60 * 1000;
const WATCH_TIMEOUT_RESTORE_MS = 49 * 60 * 60 * 1000;
/** Retry a busy fire (shared single-flight guard still releasing) this long. */
const BUSY_RETRY_MS = 30 * 1000;
/** Once the live progress entry has vanished, allow this many successful run
 * polls to still surface the recorded run before falling back to a generic
 * success (some flows record no run — see the fallback in fire()'s poll). */
const RUNLESS_GRACE_POLLS = 3;

/** The watch deadline for a run kind (matches the server's detached-run cap). */
function watchTimeoutMs(kind: WatchKind): number {
  return kind === "restore" ? WATCH_TIMEOUT_RESTORE_MS : WATCH_TIMEOUT_BACKUP_MS;
}

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
  /**
   * Set true by a paired cancel button when its cancel POST succeeds. The
   * no-run success fallback consults it so a cancelled restore that recorded NO
   * run (file/to-folder on a target-less container → progress entry just
   * vanishes) finishes "cancelled" (neutral) instead of flashing green
   * "Restored". Reset to false when a fresh run starts.
   */
  cancelledRef?: MutableRefObject<boolean>;
}

/**
 * Drives one "Back up now" / "Restore" button: call `fire()` on click. Returns
 * the current display state. Determines success vs failure from the recorded
 * run, never from the (now fire-and-forget) POST response.
 */
export function useBackupWatch({ progressKey, start, matchRun, kind = "backup", onDone, cancelledRef }: UseBackupWatchArgs) {
  const [state, setState] = useState<BackupWatchState>({ phase: "idle" });
  const progress = useProgress();
  const entry = progress[progressKey];

  // Watch bookkeeping kept in refs so the polling effect doesn't re-subscribe.
  const watching = useRef(false);
  const sawProgress = useRef(false);
  // True once the SSE progress entry was seen active AND then disappeared —
  // the server-side work has finished; only the outcome lookup remains.
  const progressVanished = useRef(false);
  // Successful run polls since the progress entry vanished that found NO
  // matching run — drives the no-run fallback (see fire()'s poll loop).
  const pollsSinceVanished = useRef(0);
  // Run ids that already existed for this target the moment we fired. The new
  // run is whichever matching run is NOT in this set — correlation by identity,
  // not by clock. null = the pre-fire snapshot failed; seeded lazily on first poll.
  const baselineIds = useRef<Set<string> | null>(null);
  const matchRef = useRef(matchRun);
  const kindRef = useRef(kind);
  const onDoneRef = useRef(onDone);
  // Mirror the (optional) cancelled flag ref the same way, so the poll closure
  // always reads the latest one without re-subscribing.
  const cancelledRefRef = useRef(cancelledRef);
  matchRef.current = matchRun;
  kindRef.current = kind;
  onDoneRef.current = onDone;
  cancelledRefRef.current = cancelledRef;

  const finish = useCallback((next: BackupWatchState) => {
    watching.current = false;
    sawProgress.current = false;
    progressVanished.current = false;
    pollsSinceVanished.current = 0;
    setState(next);
    if (next.phase === "success") {
      onDoneRef.current?.();
      // A restore's success banner is STICKY: a restore is a rare, destructive
      // action whose outcome the user must actually see, so the backup button's
      // ~4s auto-clear does not apply. It stays until reset() — which is wired
      // to selection/destination changes — puts the button back to idle.
      if (kindRef.current !== "restore") {
        setTimeout(() => setState({ phase: "idle" }), SUCCESS_CLEAR_MS);
      }
    }
  }, []);

  // Look up the outcome from the recorded runs. "resolved" once a terminal
  // (success/failed) run for this target — one that did not exist when we fired
  // — is found (state has been set); "no-run" when the lookup worked but no new
  // matching run exists (the candidate for the no-run fallback); "inconclusive"
  // when the lookup failed, was seeding, or the run is still in flight.
  const resolveFromRuns = useCallback(async (): Promise<"resolved" | "no-run" | "inconclusive"> => {
    try {
      const res = await listRuns();
      if (!res.ok || !res.runs) return "inconclusive";
      const mine = (r: Run) => r.kind === kindRef.current && matchRef.current(r);
      // If the pre-fire snapshot failed, seed the baseline from the current runs
      // now and wait for the NEXT one — never resolve against a pre-existing run.
      if (baselineIds.current === null) {
        baselineIds.current = new Set(res.runs.filter(mine).map((r) => r.id));
        return "inconclusive";
      }
      const base = baselineIds.current;
      // Runs come newest-first; the newest matching run absent at fire time is ours.
      const run = res.runs.find((r) => mine(r) && !base.has(r.id));
      if (!run) return "no-run";
      if (run.status === "success") {
        finish({ phase: "success", snapshotId: run.snapshotId || undefined });
        return "resolved";
      }
      if (run.status === "failed") {
        finish({
          phase: "error",
          message: run.error || (kindRef.current === "restore" ? "Restore failed" : "Backup failed"),
        });
        return "resolved";
      }
      if (run.status === "cancelled") {
        // Neutral terminal: the user cancelled — finish() leaves it sticky and
        // does NOT fire onDone or the red error banner (see the union comment).
        finish({ phase: "cancelled" });
        return "resolved";
      }
      if (run.status === "skipped") {
        // Neutral terminal: the container no longer exists, so the backup was
        // skipped (not failed). Complete the watch instead of polling to the 13h
        // timeout, and show neither a green success nor a red error (#57).
        finish({ phase: "skipped" });
        return "resolved";
      }
      return "inconclusive"; // still running
    } catch {
      return "inconclusive"; // transient network error — keep polling
    }
  }, [finish]);

  // Record that the live progress entry appeared so we know to look for its
  // disappearance as the completion edge. Runs on every progress map change.
  useEffect(() => {
    if (!watching.current) return;
    if (entry && entry.active) {
      sawProgress.current = true;
    } else if (sawProgress.current) {
      // The entry was seen active and has now cleared: the server-side work is
      // done, only the run lookup remains. From here the poll loop counts
      // successful-but-empty run polls toward the no-run fallback.
      progressVanished.current = true;
    }
  }, [entry]);

  // Drive a single fire+watch cycle. Polls runs on an interval until a terminal
  // run is found, the watch times out, or the component unmounts.
  const fire = useCallback(async () => {
    if (watching.current) return;
    setState({ phase: "pending" });
    // A fresh run starts uncancelled — clear any leftover flag from a prior one.
    if (cancelledRefRef.current) cancelledRefRef.current.current = false;
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
    progressVanished.current = false;
    pollsSinceVanished.current = 0;
    watching.current = true;

    const startedAt = Date.now();
    const poll = async () => {
      if (!watching.current) return;
      const outcome = await resolveFromRuns();
      if (outcome === "resolved" || !watching.current) return;
      // No-run fallback: some flows record no run at all (restore-files /
      // restore-to on a container without a target row), so run polling alone
      // would pend forever. Once the SSE progress entry was seen active and
      // then vanished (the work IS finished), a few clean polls that still
      // find no run end the watch with a generic success. A run that IS found
      // stays authoritative — it resolves above before this can trigger.
      if (progressVanished.current && outcome === "no-run") {
        pollsSinceVanished.current += 1;
        if (pollsSinceVanished.current >= RUNLESS_GRACE_POLLS) {
          // A cancel with no recorded run must NOT masquerade as success: if the
          // paired cancel button flagged a cancel, finish neutral-cancelled.
          finish(cancelledRefRef.current?.current ? { phase: "cancelled" } : { phase: "success" });
          return;
        }
      }
      if (Date.now() - startedAt > watchTimeoutMs(kindRef.current)) {
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
  const deadline = Date.now() + watchTimeoutMs(opts.kind);
  for (;;) {
    await new Promise((r) => setTimeout(r, POLL_INTERVAL_MS));
    try {
      const runs = await listRuns();
      const run = runs.runs?.find((r) => mine(r) && !baseline.has(r.id));
      if (run && run.status === "success") return { ok: true };
      if (run && run.status === "skipped") return { ok: true }; // removed target: a neutral skip, not a failure (#57)
      if (run && run.status === "failed") return { ok: false, error: run.error };
    } catch {
      // transient — keep polling
    }
    if (Date.now() > deadline) return { ok: false, error: `Timed out waiting for the ${opts.kind} to finish` };
  }
}
