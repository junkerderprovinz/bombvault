// A stack restore runs on the server, one member after the other, and its
// answer carries no results. This hook starts one and follows it: whether a
// member is being restored comes from the progress stream, and how each one
// ended from the run history.

import { useEffect, useRef, useState } from "react";
import { listRuns, restoreStack, type DbDataCoverage, type OkEnvelope, type Run } from "./api";
import type { TranslationKey } from "./i18n";
import { useProgress } from "./progress";
import { isRuntimeRefusal } from "./runReason";

/**
 * stackRestoreQuestion asks whether to restore a stack. A member whose data
 * folder is copied while the stack runs comes back as files that may not
 * start, so the question names it.
 */
export function stackRestoreQuestion(
  members: { name: string; dbDataCoverage?: DbDataCoverage }[],
  t: (key: TranslationKey, n?: number) => string,
  lang: string
): string {
  const live = members.filter((m) => m.dbDataCoverage === "live").map((m) => m.name);
  if (live.length === 0) return t("stack.restoreConfirm");
  const names = new Intl.ListFormat(lang, { style: "long", type: "conjunction" }).format(live);
  return `${t("stack.restoreConfirm")} ${t("dbdump.stackRestoreWarn", live.length).replace("{names}", names)}`;
}

// How long no member may be active before the restore counts as finished.
// Longer than a member's progress lingers plus the gap before the next one
// starts, so the state does not flip between two members.
const DONE_GRACE_MS = 8000;

const RUNS_POLL_MS = 2000;

export interface StackRestoreRequest {
  source: string;
  startInOrder: boolean;
  /** Where the project folder comes from when the source does not hold it. */
  stackDirSource?: string;
}

export interface StackRestore {
  /** True from the accepted start until no member has been active for a while. */
  started: boolean;
  finished: boolean;
  /** The newest restore of each member since the start, by container name. */
  outcomes: ReadonlyMap<string, Run>;
  /** The members Docker could not recreate for a GPU or runtime this host lacks. */
  runtimeRefused: string[];
  start: (req: StackRestoreRequest) => Promise<OkEnvelope>;
}

export function useStackRestore(project: string, names: string[]): StackRestore {
  const [started, setStarted] = useState(false);
  const [finished, setFinished] = useState(false);
  const [outcomes, setOutcomes] = useState<ReadonlyMap<string, Run>>(new Map());
  const [runtimeRefused, setRuntimeRefused] = useState<string[]>([]);
  // The runs that existed before the start. Only newer ones are this restore's.
  const [watchFrom, setWatchFrom] = useState<Set<string> | null>(null);

  const progress = useProgress();
  const memberActive =
    started &&
    names.some((name) => {
      const p = progress[`container:${name}`];
      return !!p && p.active && p.phase === "restore";
    });
  const sawActive = useRef(false);
  useEffect(() => {
    if (!started) return;
    if (memberActive) {
      sawActive.current = true;
      return;
    }
    if (!sawActive.current) return;
    const timer = setTimeout(() => {
      sawActive.current = false;
      // Reset, or a later restore of one member alone would look like this
      // stack's restore still running.
      setStarted(false);
      setFinished(true);
    }, DONE_GRACE_MS);
    return () => clearTimeout(timer);
  }, [started, memberActive]);

  const memberKey = names.join(",");
  useEffect(() => {
    if (!watchFrom) return;
    const members = new Set(memberKey.split(","));
    let alive = true;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      const res = await listRuns().catch(() => null);
      if (!alive) return;
      // Newest first, so the first run of a member is its latest.
      const latest = new Map<string, Run>();
      for (const r of res?.runs ?? []) {
        if (r.kind === "restore" && r.domain === "container" && members.has(r.target) && !watchFrom.has(r.id) && !latest.has(r.target)) {
          latest.set(r.target, r);
        }
      }
      setOutcomes(latest);
      const done = [...latest.values()].filter((r) => r.status !== "running");
      if (done.length === members.size || done.some((r) => r.status === "cancelled") || finished) {
        setRuntimeRefused(done.filter((r) => isRuntimeRefusal(r.error)).map((r) => r.target));
        setWatchFrom(null);
        return;
      }
      timer = setTimeout(() => void poll(), RUNS_POLL_MS);
    };
    timer = setTimeout(() => void poll(), RUNS_POLL_MS);
    return () => {
      alive = false;
      clearTimeout(timer);
    };
  }, [watchFrom, finished, memberKey]);

  async function start(req: StackRestoreRequest): Promise<OkEnvelope> {
    setRuntimeRefused([]);
    setOutcomes(new Map());
    setStarted(false);
    setFinished(false);
    sawActive.current = false;
    const before = await listRuns().catch(() => null);
    const res = await restoreStack(project, req.startInOrder, true, req.source, req.stackDirSource);
    if (res.ok) {
      setStarted(true);
      setWatchFrom(new Set((before?.runs ?? []).map((r) => r.id)));
    }
    return res;
  }

  return { started, finished, outcomes, runtimeRefused, start };
}
