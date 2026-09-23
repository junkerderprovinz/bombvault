// Fire-and-watch for single backups and restores. The POST returns at once
// ({ok:true, started:true}) and the work runs detached on the server, so it
// survives the request dying: backing up the reverse-proxy container the UI
// runs through cuts the fetch, and a multi-hour restore to a folder outlives
// any browser or proxy timeout. The button therefore fires, then watches for
// completion and reads the recorded run for the outcome, and never reports
// "Failed to fetch" for work the server is doing.
//
// It watches two signals:
//   1. the SSE progress store (useProgress): when this key's entry was seen
//      and then clears, the run has finished;
//   2. polling listRuns(), for the outcome (status, snapshot id, error) and as
//      a fallback when SSE reports nothing (a run that finished before we
//      subscribed, or a dropped stream). The target's run ids are recorded
//      before firing and the newest run not among them is ours; comparing the
//      client clock with the server's startedAt picks the wrong run under
//      clock skew.

import { useCallback, useEffect, useRef, useState, type MutableRefObject } from "react";
import { listRuns, type Run } from "./api";
import { useProgress } from "./progress";
import { useT } from "./i18n";

/** The translate function, same alias every page in this app uses. */
type T = ReturnType<typeof useT>["t"];

export type BackupWatchState =
  | { phase: "idle" }
  | { phase: "pending" }
  | { phase: "success"; snapshotId?: string }
  // A cancelled restore: neutral and sticky, no error banner.
  | { phase: "cancelled" }
  // The container was removed from the host but is still a target, so the run
  // was skipped. Neutral, and the watch ends instead of running into its timeout.
  | { phase: "skipped" }
  | { phase: "error"; message: string };

/** The run kind being watched (matches the recorded run's `kind` field). */
export type WatchKind = "backup" | "restore";

/** How long a backup success stays shown. A restore result is sticky; see
 *  finish(). */
const SUCCESS_CLEAR_MS = 4000;
/** Poll the runs list at this cadence while watching for completion. */
const POLL_INTERVAL_MS = 2000;
/** Give up watching after this long, just beyond the server's hard cap on a
 *  detached run (12h backups, 48h restores). */
const WATCH_TIMEOUT_BACKUP_MS = 13 * 60 * 60 * 1000;
const WATCH_TIMEOUT_RESTORE_MS = 49 * 60 * 60 * 1000;
/** Once the live progress entry has vanished, allow this many successful run
 * polls to still surface the recorded run before falling back to a generic
 * success (some flows record no run; see the fallback in fire()'s poll). */
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
  /** Which run kind to watch. Defaults to "backup". */
  kind?: WatchKind;
  /** Called once on successful completion so the caller can refresh its list. */
  onDone?: () => void;
  /**
   * Set by a paired cancel button once its cancel POST succeeds. A cancelled
   * restore that records no run (a file or to-folder restore on a container
   * without a target) would otherwise end in the no-run success fallback and
   * show a green "Restored". Reset when a new run starts.
   */
  cancelledRef?: MutableRefObject<boolean>;
}

/**
 * useBackupWatch drives one "Back up now" or "Restore" button: call `fire()`
 * on click. Success or failure comes from the recorded run, never from the
 * POST response.
 */
export function useBackupWatch({ progressKey, start, matchRun, kind = "backup", onDone, cancelledRef }: UseBackupWatchArgs) {
  const [state, setState] = useState<BackupWatchState>({ phase: "idle" });
  const { t } = useT();
  const progress = useProgress();
  const entry = progress[progressKey];

  // Watch bookkeeping lives in refs so the polling effect does not re-subscribe.
  const watching = useRef(false);
  const sawProgress = useRef(false);
  // True once the progress entry was seen active and then cleared: the work
  // is done and only the outcome lookup remains.
  const progressVanished = useRef(false);
  // Clean run polls since then that found no matching run (see fire()).
  const pollsSinceVanished = useRef(0);
  // Run ids the target already had when we fired; our run is the matching one
  // not in this set. null means the pre-fire lookup failed and the first poll
  // seeds it.
  const baselineIds = useRef<Set<string> | null>(null);
  const matchRef = useRef(matchRun);
  const kindRef = useRef(kind);
  const onDoneRef = useRef(onDone);
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
      // A restore is rare and destructive, so its success stays until
      // reset(), which the pages call when the selection or destination
      // changes.
      if (kindRef.current !== "restore") {
        setTimeout(() => setState({ phase: "idle" }), SUCCESS_CLEAR_MS);
      }
    }
  }, []);

  // Looks up the outcome in the recorded runs. "resolved": a terminal run that
  // did not exist when we fired was found, and the state is set. "no-run": the
  // lookup worked but found no new matching run. "inconclusive": the lookup
  // failed, the baseline was being seeded, or the run is still going.
  const resolveFromRuns = useCallback(async (): Promise<"resolved" | "no-run" | "inconclusive"> => {
    try {
      const res = await listRuns();
      if (!res.ok || !res.runs) return "inconclusive";
      const mine = (r: Run) => r.kind === kindRef.current && matchRef.current(r);
      // The pre-fire lookup failed: seed the baseline now and resolve on a
      // later poll, never against a run that already existed.
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
          message:
            run.error ||
            (kindRef.current === "restore" ? t("common.restoreFailed") : t("common.backupFailed")),
        });
        return "resolved";
      }
      if (run.status === "cancelled") {
        finish({ phase: "cancelled" });
        return "resolved";
      }
      if (run.status === "skipped") {
        // The container is gone from the host. End the watch instead of
        // polling into the 13h timeout.
        finish({ phase: "skipped" });
        return "resolved";
      }
      return "inconclusive"; // still running
    } catch {
      return "inconclusive"; // transient network error, keep polling
    }
  }, [finish, t]);

  // The progress entry disappearing after it was seen active is the
  // completion edge.
  useEffect(() => {
    if (!watching.current) return;
    if (entry && entry.active) {
      sawProgress.current = true;
    } else if (sawProgress.current) {
      progressVanished.current = true;
    }
  }, [entry]);

  // Drive a single fire+watch cycle. Polls runs on an interval until a terminal
  // run is found, the watch times out, or the component unmounts.
  const fire = useCallback(async () => {
    if (watching.current) return;
    setState({ phase: "pending" });
    // Clear a cancel left over from the previous run.
    if (cancelledRefRef.current) cancelledRefRef.current.current = false;
    // Record the target's existing run ids before firing. If that fails the
    // baseline stays null and resolveFromRuns seeds it on the first poll.
    baselineIds.current = null;
    try {
      const before = await listRuns();
      if (before.ok && before.runs) {
        baselineIds.current = new Set(
          before.runs.filter((r) => r.kind === kindRef.current && matchRef.current(r)).map((r) => r.id)
        );
      }
    } catch {
      // the first poll seeds the baseline
    }
    let res: Awaited<ReturnType<StartBackupFn>>;
    try {
      res = await start();
    } catch (err) {
      setState({ phase: "error", message: err instanceof Error ? err.message : t("common.networkError") });
      return;
    }
    if (!res.ok) {
      setState({
        phase: "error",
        message:
          res.error ?? (kindRef.current === "restore" ? t("common.restoreFailed") : t("common.backupFailed")),
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
      // Some flows record no run at all (a file restore or restore-to on a
      // container without a target row), so polling alone would never end.
      // Once the progress entry was seen and has cleared, a few clean polls
      // without a run end the watch as a success. A run that does turn up
      // resolves above first.
      if (progressVanished.current && outcome === "no-run") {
        pollsSinceVanished.current += 1;
        if (pollsSinceVanished.current >= RUNLESS_GRACE_POLLS) {
          // A cancel without a recorded run must not end as a success.
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
  }, [start, resolveFromRuns, finish, t]);

  // Clears a finished result, e.g. when the selection changes and the result
  // would describe the wrong thing. Does nothing while a watch runs.
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

/**
 * fireAndWaitRun is the non-hook variant for bulk loops, which back up or
 * restore their targets one after another. It fires one run, retrying while
 * the previous target still holds the server's single-flight guard, then
 * waits for the new run to finish. The guard is released only when the
 * previous target's whole backup call returns, including its inline off-site
 * replication, so the retry shares the full watch deadline; a shorter budget
 * would drop a batch item without any run recorded for it. Like the hook, it
 * matches the run by a new id, not by the clock.
 */
export async function fireAndWaitRun(opts: {
  kind: WatchKind;
  matchRun: RunMatcher;
  start: StartBackupFn;
  /** Required: every caller toasts the failure text as is, so it has to be
   *  translated. */
  t: T;
}): Promise<{ ok: boolean; error?: string }> {
  const mine = (r: Run) => r.kind === opts.kind && opts.matchRun(r);
  let baseline = new Set<string>();
  try {
    const before = await listRuns();
    baseline = new Set((before.runs ?? []).filter(mine).map((r) => r.id));
  } catch {
    // without a baseline, the first terminal run for this target counts
  }
  const deadline = Date.now() + watchTimeoutMs(opts.kind);
  for (;;) {
    let res: Awaited<ReturnType<StartBackupFn>>;
    try {
      res = await opts.start();
    } catch (err) {
      return { ok: false, error: err instanceof Error ? err.message : opts.t("common.networkError") };
    }
    if (res.ok) break;
    const busy = (res.error ?? "").toLowerCase().includes("already running");
    if (!busy || Date.now() > deadline) return { ok: false, error: res.error };
    await new Promise((r) => setTimeout(r, 1000));
  }
  // Poll the recorded runs until this target's new run reaches a terminal state.
  for (;;) {
    await new Promise((r) => setTimeout(r, POLL_INTERVAL_MS));
    try {
      const runs = await listRuns();
      const run = runs.runs?.find((r) => mine(r) && !baseline.has(r.id));
      if (run && run.status === "success") return { ok: true };
      if (run && run.status === "skipped") return { ok: true }; // a removed target is skipped, not failed
      if (run && run.status === "failed") return { ok: false, error: run.error };
    } catch {
      // transient, keep polling
    }
    if (Date.now() > deadline) return { ok: false, error: `Timed out waiting for the ${opts.kind} to finish` };
  }
}
