// What the status head says: one of a few fixed sentences while nothing runs,
// and the run it follows while something does.

import { ENTRY_KIND, isEntryKind, type EntryKind } from "../../lib/backupEntry";
import { offsiteRunProgress, type ProgressMap, type ProgressState } from "../../lib/progress";
import type { Tally } from "../../lib/protectionTally";
import type { Section } from "./protectionChecks";

export type Headline = "needs" | "waiting" | "partly" | "notes" | "allGood";

/**
 * headlineOf picks the sentence. A warning is never rounded up: open notes
 * and a section with a check that is not green keep the head off "everything
 * backed up".
 */
export function headlineOf({
  needs,
  tally,
  sections,
  notes,
}: {
  needs: number;
  tally: Tally;
  sections: Section[];
  /** Open findings that are only notes. */
  notes: number;
}): Headline {
  if (needs > 0) return "needs";
  if (tally.ok === 0) return "waiting";
  if (tally.ok < tally.total) return "partly";
  if (notes > 0 || sections.some((s) => s.tone === "warn" || s.tone === "fail")) return "notes";
  return "allGood";
}

export const STATIONS = ["stop", "read", "start", "copy"] as const;

export type Station = (typeof STATIONS)[number];

// Containers and VMs stop for their backup and start again after it. The
// other kinds are read while they run.
function stops(kind: EntryKind | null): boolean {
  return kind === "container" || kind === "vm";
}

/**
 * stationOf is where a run stands among the four stations. The server reports
 * a percentage, whether the backup is written and the off-site keys, but no
 * named phases, so this is the one place that derives them.
 */
export function stationOf(kind: EntryKind | null, state: ProgressState): Station {
  if (state.phase === "replicate") return "copy";
  if (state.committed) return "start";
  if (!stops(kind) || state.percent > 0 || state.stage !== undefined) return "read";
  return "stop";
}

export interface LiveRun {
  /** The kind of the entry being backed up, null for an off-site copy. */
  kind: EntryKind | null;
  /** The entry's name, "" for a kind with one entry. For an off-site copy
   *  the domain it copies. */
  name: string;
  percent: number;
  station: Station;
  /** Whether the entry passes the stop and start stations at all. */
  stops: boolean;
  /** An off-site copy counting through its snapshots. */
  step?: { index: number; total: number };
}

function liveBackup(key: string, state: ProgressState): LiveRun {
  const [head, ...rest] = key.split(":");
  const kind = isEntryKind(head) ? head : null;
  return {
    kind,
    name: kind && ENTRY_KIND[kind].keyedBy === "kind" ? "" : rest.join(":"),
    percent: state.percent,
    station: stationOf(kind, state),
    stops: stops(kind),
  };
}

function liveCopy(key: string, state: ProgressState): LiveRun {
  const overall = offsiteRunProgress(state);
  return {
    kind: null,
    name: key.slice("offsite:".length),
    percent: overall?.percent ?? state.percent,
    station: "copy",
    stops: false,
    step: overall ? { index: overall.index, total: overall.total } : undefined,
  };
}

/**
 * liveRunOf is the run the head follows: a backup under way, else an off-site
 * copy. Restores and maintenance leave the head alone.
 */
export function liveRunOf(progress: ProgressMap): LiveRun | null {
  const live = Object.entries(progress).filter(([, state]) => state.active && !state.finished);
  const backup = live.find(([key, state]) => state.phase === "backup" && !key.startsWith("offsite:"));
  if (backup) return liveBackup(...backup);
  const copy = live.find(([key, state]) => state.phase === "replicate" && key.startsWith("offsite:"));
  return copy ? liveCopy(...copy) : null;
}
