// The dashboard's activity log: one flat list of timestamped lines, merged
// from finished runs (listRuns), live SSE progress (useProgress) and the next
// scheduled fire (/api/schedule/next). buildLogLines takes plain data and the
// clock as arguments, so the merge, dedupe and ordering can be tested without
// React, i18n or a live stream.

import type { Run, ScheduleNext } from "./api";
import { dumpLeftRunning, dumpWasCancelled, importHadErrors } from "./dbdump";
import type { ProgressMap, ProgressStage, ProgressState } from "./progress";
import { offsiteRunProgress, STALE_MS } from "./progress";
import { elapsedSince, formatClockTime, formatDuration } from "./reltime";
import type { TranslationKey } from "./i18n";
import { isWarningNote, runReason, runReasonParts } from "./runReason";

/** Picks a line's glyph and colour in ActivityLog.tsx. */
export type LogStatus = "running" | "success" | "failed" | "offsite" | "info";

/** The domain a line belongs to, for the domain filter. "everything" is the
 *  Backup Everything pass over the others (store.EverythingTargetID on the
 *  backend). "" means a finished run's target could not be resolved, e.g. a
 *  deleted item. */
export type LogDomain = "containers" | "vms" | "flash" | "config" | "files" | "zfs" | "everything" | "";

/** The operation kind, for the type filter. "update" (the image update after a
 *  backup) has no filter chip but still carries a kind for search. "drill" is
 *  the local restore drill and "drdrill" the off-site DR check; rows recorded
 *  before the two were split stay "drill". */
export type LogKind = "backup" | "restore" | "prune" | "verify" | "offsite" | "update" | "drill" | "drdrill" | "tamper" | "export" | "dbdump" | "";

export interface LogLine {
  /** Stable React key. */
  id: string;
  /** Ordering key in epoch ms: finishedAt (else startedAt) for a finished run,
   *  the progress entry's lastSeen for a live line, `now` for the idle line. */
  atMs: number;
  status: LogStatus;
  /** Localized message, without timestamp or glyph. */
  text: string;
  domain: LogDomain;
  kind: LogKind;
  /** An active tail line that updates in place. */
  live: boolean;
  /** The trailing "next up" or "nothing yet" line. It has no domain or kind,
   *  so filterLogLines exempts it from the quick filters; otherwise an active
   *  filter chip could hide it. */
  idle?: boolean;
  /** A run that succeeded with a note worth acting on, coloured like the run
   *  history colours that note. */
  warn?: boolean;
}

/**
 * Turns a translation key and optional `{placeholder}` params into text. The
 * count picks the plural form of a key that offers several.
 * Injected so buildLogLines stays pure: ActivityLog.tsx passes useT()'s `t`,
 * tests pass a stub.
 */
export type ResolveName = (key: string, params?: Record<string, string>, count?: number) => string;

/** The `t` a run reason expects, fed from a ResolveName. */
function reasonT(resolveName: ResolveName) {
  return (key: TranslationKey, n?: number) => resolveName(key, undefined, n);
}

/**
 * reasonText translates a run's error when it is one of our own sentences, so a
 * "…failed: {error}" line does not start in German and end in English. A tool's
 * own message behind ours stays as it was stored, and a message from restic,
 * rclone or Docker passes through whole.
 */
function reasonText(raw: string | undefined, resolveName: ResolveName): string {
  if (!raw) return "";
  return runReason(raw, reasonT(resolveName));
}

const DOMAIN_KEYS: Record<string, string> = {
  containers: "activityLog.domainContainers",
  vms: "activityLog.domainVMs",
  flash: "activityLog.domainFlash",
  config: "activityLog.domainConfig",
  files: "activityLog.domainFiles",
  zfs: "activityLog.domainZFS",
  everything: "activityLog.domainEverything",
};

/** Translation keys for the job names the Go scheduler emits. A missing entry
 *  shows up as the bare English job name, and nothing in TypeScript can see
 *  internal/schedule/schedule.go, so activityLog.jobReach.test.ts reads the Go
 *  source and fails when a job has no entry here. */
export const JOB_KEYS: Record<string, string> = {
  backup: "activityLog.jobBackup",
  offsite: "activityLog.jobOffsite",
  drill: "activityLog.jobDrill",
  tamper: "activityLog.jobTamper",
  digest: "activityLog.jobDigest",
  watchdog: "activityLog.jobWatchdog",
  receiver: "activityLog.jobReceiver",
  fleet: "activityLog.jobFleet",
  pull: "activityLog.jobPull",
};

/** Translates a domain literal; an unknown one falls back to the raw string. */
export function domainLabel(resolveName: ResolveName, domain: string): string {
  const key = DOMAIN_KEYS[domain];
  return key ? resolveName(key) : domain;
}

/** Translates a schedule job name; an unknown one falls back to the raw string. */
function jobLabel(resolveName: ResolveName, job: string): string {
  const key = JOB_KEYS[job];
  return key ? resolveName(key) : job;
}

/** normalizeDomain maps the singular item domains of runs and progress keys
 *  ("container", "vm") to the plural form the filters use. Other known
 *  domains pass through; anything else becomes "". */
function normalizeDomain(domain: string): LogDomain {
  if (domain === "container") return "containers";
  if (domain === "vm") return "vms";
  if (
    domain === "containers" ||
    domain === "vms" ||
    domain === "flash" ||
    domain === "config" ||
    domain === "files" ||
    domain === "zfs" ||
    domain === "everything"
  ) {
    return domain;
  }
  return "";
}

/** Clamps and rounds a percentage to an integer in 0..100. */
function displayPercent(percent: number): number {
  if (!Number.isFinite(percent)) return 0;
  return Math.round(Math.max(0, Math.min(100, percent)));
}

/** Binary byte formatter with one decimal, matching Dashboard's humanBytes. */
function formatBytesShort(n: number): string {
  if (!n || n <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let v = n;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${i === 0 ? v : v.toFixed(1)} ${units[i]}`;
}

type ParsedKey =
  | { scope: "item"; domain: "container" | "vm" | "files" | "zfs" | "flash" | "config"; name: string }
  | { scope: "batch"; domain: string }
  | { scope: "offsite" | "prune" | "verify" | "drill" | "drdrill" | "tamper" | "export"; domain: string };

/**
 * parseProgressKey decodes a live SSE progress key; progress.ts documents the
 * shapes. An item key's suffix is already the display name the backend
 * published (internal/api/service.go), so no id lookup is needed. Returns
 * null for a shape it does not know.
 */
function parseProgressKey(key: string): ParsedKey | null {
  if (key === "flash") return { scope: "item", domain: "flash", name: "flash" };
  if (key === "config") return { scope: "item", domain: "config", name: "config" };
  if (key.startsWith("container:")) return { scope: "item", domain: "container", name: key.slice("container:".length) };
  if (key.startsWith("vm:")) return { scope: "item", domain: "vm", name: key.slice("vm:".length) };
  if (key.startsWith("files:")) return { scope: "item", domain: "files", name: key.slice("files:".length) };
  if (key.startsWith("zfs:")) return { scope: "item", domain: "zfs", name: key.slice("zfs:".length) };
  if (key.startsWith("batch:")) return { scope: "batch", domain: key.slice("batch:".length) };
  if (key.startsWith("offsite:")) return { scope: "offsite", domain: key.slice("offsite:".length) };
  if (key.startsWith("prune:")) return { scope: "prune", domain: key.slice("prune:".length) };
  if (key.startsWith("verify:")) return { scope: "verify", domain: key.slice("verify:".length) };
  if (key.startsWith("drill:")) return { scope: "drill", domain: key.slice("drill:".length) };
  if (key.startsWith("drdrill:")) return { scope: "drdrill", domain: key.slice("drdrill:".length) };
  if (key.startsWith("tamper:")) return { scope: "tamper", domain: key.slice("tamper:".length) };
  if (key.startsWith("export:")) return { scope: "export", domain: key.slice("export:".length) };
  return null;
}

/**
 * itemDisplayName resolves an item-scope key's display name. Container, VM,
 * file-set and dataset names are proper nouns and shown as is; the flash and
 * config singletons get their translated domain label. The check is on
 * `parsed.domain`, not on the name, so a container called "flash" stays a
 * container.
 */
function itemDisplayName(resolveName: ResolveName, parsed: Extract<ParsedKey, { scope: "item" }>): string {
  if (parsed.domain === "flash") return domainLabel(resolveName, "flash");
  if (parsed.domain === "config") return domainLabel(resolveName, "config");
  return parsed.name;
}

/** The live line of a step that counts bytes instead of a percentage. */
const STAGE_LINE_KEYS: Record<ProgressStage, string> = {
  dbdump: "activityLog.lineDumpingItem",
  dbdumpsave: "activityLog.lineSavingDumpItem",
  dbimport: "activityLog.lineImportingItem",
};

interface LiveResult {
  lines: LogLine[];
  /** Signatures of currently-active operations, used to suppress the
   *  finished-run line that would otherwise briefly double up with it. */
  signatures: Set<string>;
}

// itemSignature is the dedupe key for an item-scope backup or restore, so a
// finished run's history line stays hidden while its live line still shows.
// Flash and config key on the domain alone: the live line carries the
// translated domain label and the run row the backend's English name
// ("Unraid flash", "App configuration"), so the names never match, and each
// of those domains has exactly one item anyway.
function itemSignature(kind: string, domain: LogDomain, name: string): string {
  if (domain === "flash" || domain === "config") return `item|${kind}|${domain}`;
  return `item|${kind}|${domain}|${name}`;
}

function domainOpSignature(kind: string, domain: string): string {
  return `domain|${kind}|${domain}`;
}

/** Live-line text per domain-scoped operation. The export line takes no
 *  {domain}: the flash ZIP export is flash-only, so its text names flash. */
const DOMAIN_OP_RUNNING_KEYS: Record<"prune" | "verify" | "drill" | "drdrill" | "tamper" | "export", string> = {
  prune: "activityLog.linePruneRunning",
  verify: "activityLog.lineVerifyRunning",
  drill: "activityLog.lineDrillRunning",
  drdrill: "activityLog.lineDRDrillRunning",
  tamper: "activityLog.lineTamperRunning",
  export: "activityLog.lineExportRunning",
};

/**
 * offsiteLiveLineText picks the live-line text for an "offsite:<domain>"
 * progress state, in the same tiers as OffsiteIndicator's offsiteStatusText:
 * a run-level percentage when one can be derived ("{percent}% overall
 * (snapshot {index} of {total})"), otherwise the elapsed duration, otherwise
 * the bare running text. Each tier has a "WithDuration" key so a percentage
 * never has to drop the duration. The percentage comes from
 * offsiteRunProgress, so this line and OffsiteIndicator agree on it.
 */
function offsiteLiveLineText(resolveName: ResolveName, domain: LogDomain, state: ProgressState, duration: string): string {
  const domainText = domainLabel(resolveName, domain);
  const run = offsiteRunProgress(state);
  if (run) {
    const params = { domain: domainText, index: String(run.index), total: String(run.total), percent: String(run.percent), duration };
    return duration
      ? resolveName("activityLog.lineOffsiteRunningSnapshotPercentWithDuration", params)
      : resolveName("activityLog.lineOffsiteRunningSnapshotPercent", params);
  }
  return duration
    ? resolveName("activityLog.lineOffsiteRunningWithDuration", { domain: domainText, duration })
    : resolveName("activityLog.lineOffsiteRunning", { domain: domainText });
}

/**
 * buildLiveLines renders the active SSE progress keys as tail lines. `now`
 * decides staleness and may lag, since ActivityLog.tsx ticks it once a
 * minute. `liveNow` is a faster clock the caller runs while a live line is on
 * screen, used only for the off-site elapsed duration: on the minute tick
 * alone, `now` can sit behind the run's `startedAt` for its first minute and
 * the duration renders blank.
 */
function buildLiveLines(
  progressMap: ProgressMap,
  resolveName: ResolveName,
  now: number,
  liveNow: number = now,
  stillRunning: ReadonlySet<string> = new Set()
): LiveResult {
  const lines: LogLine[] = [];
  const signatures = new Set<string>();

  for (const key of Object.keys(progressMap)) {
    const state = progressMap[key];
    if (!state.active) continue;

    // A terminal SSE frame lost in transit can leave active:true stuck, so a
    // silent line has to be able to end. Silence alone is not enough, though:
    // restic goes quiet for minutes while it scans a large folder tree, and
    // lastSeen ages past STALE_MS on a perfectly healthy run. A stale line is
    // dropped only once the runs list, polled every 10s, stops reporting its
    // target as running.
    const stale = now - state.lastSeen > STALE_MS;
    // A null signature means no Run row is ever attributed to the key, so
    // staleness is the only signal there is for it.
    const keep = (signature: string | null) =>
      !stale || (signature !== null && stillRunning.has(signature));

    const parsed = parseProgressKey(key);
    if (!parsed) continue;

    if (parsed.scope === "item") {
      const name = itemDisplayName(resolveName, parsed);
      const domain = normalizeDomain(parsed.domain);
      // A dump, a dump save and an import each record a run of their own kind,
      // so the signature follows the stage rather than the phase around it.
      const runKind = state.stage ?? (state.phase === "restore" ? "restore" : "backup");
      const pct = displayPercent(state.percent);
      const text = state.stage
        ? resolveName(STAGE_LINE_KEYS[state.stage], { name, bytes: formatBytesShort(state.bytes ?? 0) })
        : runKind === "restore"
          ? resolveName("activityLog.lineRestoringItem", { name, percent: String(pct) })
          : resolveName("activityLog.lineBackingUpItem", { name, percent: String(pct) });
      const sig = itemSignature(runKind, domain, name);
      if (!keep(sig)) continue;
      signatures.add(sig);
      lines.push({ id: `live:${key}`, atMs: state.lastSeen, status: "running", text, domain, kind: asLogKind(runKind), live: true });
      continue;
    }

    if (parsed.scope === "batch") {
      const domain = normalizeDomain(parsed.domain);
      const pct = displayPercent(state.percent);
      const text = resolveName("activityLog.lineBackingUpBatch", {
        domain: domainLabel(resolveName, domain),
        percent: String(pct),
      });
      // Each member item records its own run and the batch key none, so there
      // is nothing to dedupe against and nothing that can vouch for it.
      if (!keep(null)) continue;
      lines.push({ id: `live:${key}`, atMs: state.lastSeen, status: "running", text, domain, kind: "backup", live: true });
      continue;
    }

    if (parsed.scope === "offsite") {
      const domain = normalizeDomain(parsed.domain);
      // Against liveNow, not now; see the doc comment.
      const duration = elapsedSince(state.startedAt, liveNow);
      const text = offsiteLiveLineText(resolveName, domain, state, duration);
      // Replication records a Run row on the domain target, so the signature
      // keeps its finished line from showing next to this one.
      const offsiteSig = domainOpSignature("offsite", domain);
      if (!keep(offsiteSig)) continue;
      signatures.add(offsiteSig);
      lines.push({ id: `live:${key}`, atMs: state.lastSeen, status: "offsite", text, domain, kind: "offsite", live: true });
      continue;
    }

    // prune, verify, drill, drdrill, tamper and export: domain-wide
    // operations that record a Run row on the domain target the same way.
    const domain = normalizeDomain(parsed.domain);
    const text = resolveName(DOMAIN_OP_RUNNING_KEYS[parsed.scope], { domain: domainLabel(resolveName, domain) });
    const opSig = domainOpSignature(parsed.scope, domain);
    if (!keep(opSig)) continue;
    signatures.add(opSig);
    lines.push({ id: `live:${key}`, atMs: state.lastSeen, status: "running", text, domain, kind: parsed.scope, live: true });
  }

  return { lines, signatures };
}

function finishedLineText(resolveName: ResolveName, run: Run, domain: LogDomain, name: string): {
  status: LogStatus;
  text: string;
} {
  const duration = formatDuration((run.finishedAt ?? run.startedAt) - run.startedAt);
  const domainText = domainLabel(resolveName, domain);

  if (run.kind === "prune") {
    return run.status === "success"
      ? { status: "success", text: resolveName("activityLog.linePruneSuccess", { domain: domainText }) }
      : run.status === "failed"
        ? { status: "failed", text: resolveName("activityLog.linePruneFailed", { domain: domainText, error: reasonText(run.error, resolveName) }) }
        : { status: "info", text: resolveName("activityLog.lineOther", { name: domainText, kind: run.kind, status: run.status }) };
  }

  if (run.kind === "verify") {
    return run.status === "success"
      ? { status: "success", text: resolveName("activityLog.lineVerifySuccess", { domain: domainText }) }
      : run.status === "failed"
        ? { status: "failed", text: resolveName("activityLog.lineVerifyFailed", { domain: domainText, error: reasonText(run.error, resolveName) }) }
        : { status: "info", text: resolveName("activityLog.lineOther", { name: domainText, kind: run.kind, status: run.status }) };
  }

  if (run.kind === "offsite") {
    return run.status === "success"
      ? { status: "offsite", text: resolveName("activityLog.lineOffsiteSuccess", { domain: domainText, duration }) }
      : run.status === "failed"
        ? { status: "failed", text: resolveName("activityLog.lineOffsiteFailed", { domain: domainText, error: reasonText(run.error, resolveName) }) }
        : { status: "info", text: resolveName("activityLog.lineOther", { name: domainText, kind: run.kind, status: run.status }) };
  }

  if (run.kind === "drill") {
    return run.status === "success"
      ? { status: "success", text: resolveName("activityLog.lineDrillSuccess", { domain: domainText }) }
      : run.status === "failed"
        ? { status: "failed", text: resolveName("activityLog.lineDrillFailed", { domain: domainText, error: reasonText(run.error, resolveName) }) }
        : { status: "info", text: resolveName("activityLog.lineOther", { name: domainText, kind: run.kind, status: run.status }) };
  }

  if (run.kind === "drdrill") {
    return run.status === "success"
      ? { status: "success", text: resolveName("activityLog.lineDRDrillSuccess", { domain: domainText }) }
      : run.status === "failed"
        ? { status: "failed", text: resolveName("activityLog.lineDRDrillFailed", { domain: domainText, error: reasonText(run.error, resolveName) }) }
        : { status: "info", text: resolveName("activityLog.lineOther", { name: domainText, kind: run.kind, status: run.status }) };
  }

  if (run.kind === "tamper") {
    // "skipped" means the test ran without a verdict (non-REST off-site,
    // transport error, inconclusive probe): a neutral line with the backend's
    // reason, not a failure.
    return run.status === "success"
      ? { status: "success", text: resolveName("activityLog.lineTamperSuccess", { domain: domainText }) }
      : run.status === "failed"
        ? { status: "failed", text: resolveName("activityLog.lineTamperFailed", { domain: domainText, error: reasonText(run.error, resolveName) }) }
        : run.status === "skipped"
          ? { status: "info", text: resolveName("activityLog.lineTamperSkipped", { domain: domainText, error: reasonText(run.error, resolveName) }) }
          : { status: "info", text: resolveName("activityLog.lineOther", { name: domainText, kind: run.kind, status: run.status }) };
  }

  if (run.kind === "export") {
    return run.status === "success"
      ? { status: "success", text: resolveName("activityLog.lineExportSuccess", { bytes: formatBytesShort(run.bytes), duration }) }
      : run.status === "failed"
        ? { status: "failed", text: resolveName("activityLog.lineExportFailed", { error: reasonText(run.error, resolveName) }) }
        : { status: "info", text: resolveName("activityLog.lineOther", { name: domainText, kind: run.kind, status: run.status }) };
  }

  if (run.kind === "dbdump") {
    // A dump that may still be running is worth a look even when it was cancelled.
    if (dumpWasCancelled(run.error) && !dumpLeftRunning(run.error)) {
      return { status: "info", text: resolveName("activityLog.lineDbDumpCancelled", { name }) };
    }
    if (run.status === "success") {
      const bytes = formatBytesShort(run.bytes);
      return run.error
        ? { status: "success", text: resolveName("activityLog.lineDbDumpNote", { name, bytes, duration, note: reasonText(run.error, resolveName) }) }
        : { status: "success", text: resolveName("activityLog.lineDbDumpSuccess", { name, bytes, duration }) };
    }
    return run.status === "failed"
      ? { status: "failed", text: resolveName("activityLog.lineDbDumpFailed", { name, error: reasonText(run.error, resolveName) }) }
      : { status: "info", text: resolveName("activityLog.lineOther", { name, kind: run.kind, status: run.status }) };
  }

  if (run.kind === "dbdumpsave") {
    return run.status === "success"
      ? { status: "success", text: resolveName("activityLog.lineDbDumpSaved", { name, bytes: formatBytesShort(run.bytes) }) }
      : run.status === "failed"
        ? { status: "failed", text: resolveName("activityLog.lineDbDumpSaveFailed", { name, error: reasonText(run.error, resolveName) }) }
        : run.status === "cancelled"
          ? { status: "info", text: resolveName("activityLog.lineDbDumpSaveCancelled", { name }) }
          : { status: "info", text: resolveName("activityLog.lineOther", { name, kind: run.kind, status: run.status }) };
  }

  if (run.kind === "dbimport") {
    if (run.status === "success") {
      if (importHadErrors(run.error)) {
        return { status: "success", text: resolveName("activityLog.lineDbImportedErrors", { name, note: reasonText(run.error, resolveName) }) };
      }
      const text = resolveName("activityLog.lineDbImported", { name });
      const { note } = runReasonParts(run.error, reasonT(resolveName));
      return { status: "success", text: note ? `${text}; ${note}` : text };
    }
    return run.status === "failed"
      ? { status: "failed", text: resolveName("activityLog.lineDbImportFailed", { name, error: reasonText(run.error, resolveName) }) }
      : { status: "info", text: resolveName("activityLog.lineOther", { name, kind: run.kind, status: run.status }) };
  }

  if (run.kind === "restore") {
    return run.status === "success"
      ? { status: "success", text: resolveName("activityLog.lineRestoreSuccess", { name, duration }) }
      : run.status === "failed"
        ? { status: "failed", text: resolveName("activityLog.lineRestoreFailed", { name, error: reasonText(run.error, resolveName) }) }
        : { status: "info", text: resolveName("activityLog.lineOther", { name, kind: run.kind, status: run.status }) };
  }

  if (run.kind === "update") {
    return run.status === "success"
      ? { status: "success", text: resolveName("activityLog.lineUpdateSuccess", { name, duration }) }
      : run.status === "failed"
        ? { status: "failed", text: resolveName("activityLog.lineUpdateFailed", { name, error: reasonText(run.error, resolveName) }) }
        : { status: "info", text: resolveName("activityLog.lineOther", { name, kind: run.kind, status: run.status }) };
  }

  // backup, and any kind this client does not know yet.
  if (run.status === "success") {
    return { status: "success", text: resolveName("activityLog.lineBackupSuccess", { name, bytes: formatBytesShort(run.bytes), duration }) };
  }
  if (run.status === "failed") {
    return { status: "failed", text: resolveName("activityLog.lineBackupFailed", { name, error: reasonText(run.error, resolveName) }) };
  }
  if (run.status === "skipped") {
    return { status: "info", text: resolveName("activityLog.lineBackupSkipped", { name, error: reasonText(run.error, resolveName) }) };
  }
  return { status: "info", text: resolveName("activityLog.lineOther", { name, kind: run.kind, status: run.status }) };
}

/** Narrows a raw Run.kind string to LogKind; an unknown kind becomes ""
 *  rather than a filter value nothing offers. Saving a dump to a folder and
 *  importing one are restore-side work, and that is the filter someone reaches
 *  for to find them. */
function asLogKind(kind: string): LogKind {
  if (kind === "dbdumpsave" || kind === "dbimport") return "restore";
  if (kind === "dbdump") return "dbdump";
  if (
    kind === "backup" ||
    kind === "restore" ||
    kind === "prune" ||
    kind === "verify" ||
    kind === "update" ||
    kind === "offsite" ||
    kind === "drill" ||
    kind === "drdrill" ||
    kind === "tamper" ||
    kind === "export"
  ) {
    return kind;
  }
  return "";
}

/** Kinds recorded against the reserved domain target (the backend's
 *  domainRunTargetID): their targetId is the domain literal or the flash or
 *  config singleton id, never an item id. */
function isDomainOpKind(kind: string): boolean {
  return kind === "prune" || kind === "verify" || kind === "offsite" || kind === "drill" || kind === "drdrill" || kind === "tamper" || kind === "export";
}

function buildHistoryLines(runs: Run[], resolveName: ResolveName, liveSignatures: Set<string>): LogLine[] {
  const lines: LogLine[] = [];
  for (const run of runs) {
    // A run still in flight shows as its live progress line instead.
    if (run.finishedAt == null) continue;

    const isDomainOp = isDomainOpKind(run.kind);
    const domain: LogDomain = isDomainOp ? normalizeDomain(run.targetId) : normalizeDomain(run.domain);
    const name = run.target;

    const signature = isDomainOp ? domainOpSignature(run.kind, domain) : itemSignature(run.kind, domain, name);
    if (liveSignatures.has(signature)) continue;

    const { status, text } = finishedLineText(resolveName, run, domain, name);
    const line: LogLine = { id: `run:${run.id}`, atMs: run.finishedAt * 1000, status, text, domain, kind: asLogKind(run.kind), live: false };
    if (run.status === "success" && isWarningNote(run.error)) line.warn = true;
    lines.push(line);
  }
  return lines;
}

function buildIdleLine(scheduleNext: ScheduleNext[], resolveName: ResolveName, now: number, hasHistory: boolean): LogLine | null {
  const next = scheduleNext[0];
  if (!next) {
    if (hasHistory) return null;
    return { id: "idle-empty", atMs: now, status: "info", text: resolveName("activityLog.lineEmpty"), domain: "", kind: "", live: false, idle: true };
  }

  const nextMs = new Date(next.next).getTime();
  const countdown = formatDuration(Math.max(0, Math.round((nextMs - now) / 1000)));
  const time = formatClockTime(nextMs / 1000, false);
  const job = jobLabel(resolveName, next.job);

  const text = next.domain
    ? resolveName("activityLog.lineNextWithDomain", {
        job,
        domain: domainLabel(resolveName, next.domain),
        time,
        countdown,
      })
    : resolveName("activityLog.lineNextNoDomain", { job, time, countdown });

  return { id: "idle-next", atMs: now, status: "info", text, domain: "", kind: "", live: false, idle: true };
}

/**
 * buildLogLines merges finished runs, live progress and the next scheduled
 * fire into one deduped list: history oldest first, then the live lines, and
 * an idle line at the end only when nothing is running. `liveNow` defaults to
 * `now`; buildLiveLines explains why the off-site line wants a faster clock.
 */
export function buildLogLines(
  runs: Run[],
  progressMap: ProgressMap,
  scheduleNext: ScheduleNext[],
  resolveName: ResolveName,
  now: number,
  liveNow: number = now
): LogLine[] {
  // The runs the backend still reports as running, keyed with the same
  // signatures buildHistoryLines uses, so a stale live line can ask whether
  // its run is still going.
  const stillRunning = new Set<string>();
  for (const run of runs) {
    if (run.status !== "running") continue;
    const isDomainOp = isDomainOpKind(run.kind);
    const domain: LogDomain = isDomainOp ? normalizeDomain(run.targetId) : normalizeDomain(run.domain);
    stillRunning.add(
      isDomainOp ? domainOpSignature(run.kind, domain) : itemSignature(run.kind, domain, run.target)
    );
  }

  const { lines: liveLines, signatures } = buildLiveLines(progressMap, resolveName, now, liveNow, stillRunning);
  const historyLines = buildHistoryLines(runs, resolveName, signatures);

  const orderedHistory = historyLines.slice().sort((a, b) => a.atMs - b.atMs);
  const orderedLive = liveLines.slice().sort((a, b) => a.atMs - b.atMs);

  const result = [...orderedHistory, ...orderedLive];

  if (orderedLive.length === 0) {
    const idle = buildIdleLine(scheduleNext, resolveName, now, orderedHistory.length > 0);
    if (idle) result.push(idle);
  }

  return result;
}

/**
 * formatLogDate renders `atMs` as a short day and month in the locale's own
 * order ("23.07." for de, "07/23" for en-US). The app leaves `locale`
 * undefined, which runs the same default-locale negotiation as every other
 * date here; navigator.language can disagree with the browser's formatting
 * locale (an en-US interface in a Portuguese region). Tests pass one
 * explicitly. filterLogLines searches the same string, so a typed date
 * matches what is shown. The time stays on formatClockTime's fixed 24-hour
 * face.
 */
export function formatLogDate(atMs: number, locale?: string): string {
  return new Intl.DateTimeFormat(locale, { day: "2-digit", month: "2-digit" }).format(new Date(atMs));
}

/** isoDateOf renders `atMs` as its local calendar day in YYYY-MM-DD, so a
 *  typed ISO date matches whatever the display language. */
function isoDateOf(atMs: number): string {
  const d = new Date(atMs);
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

/** Domain quick-filter value ("all" plus every LogDomain except ""). */
export type LogFilterDomain = "all" | "containers" | "vms" | "flash" | "config" | "files" | "zfs" | "everything";

/** Type quick-filter value: "all" plus the kinds the filter bar offers, which
 *  leaves out "update". "drill" and "drdrill" are separate values (local
 *  subset check and off-site DR restore check); DR rows recorded before the
 *  split say "drill" and keep matching the drill filter. */
export type LogFilterKind = "all" | "backup" | "restore" | "dbdump" | "prune" | "verify" | "offsite" | "drill" | "drdrill" | "tamper" | "export";

/** The filter bar's options as value and translation-key pairs. The activity
 *  log's bar and the error panel's both read them from here, so a new kind
 *  cannot reach one filter and miss the other. */
export const LOG_FILTER_DOMAINS: { value: LogFilterDomain; key: string }[] = [
  { value: "all", key: "activityLog.filterAllDomains" },
  { value: "containers", key: "activityLog.domainContainers" },
  { value: "vms", key: "activityLog.domainVMs" },
  { value: "flash", key: "activityLog.domainFlash" },
  { value: "config", key: "activityLog.domainConfig" },
  { value: "files", key: "activityLog.domainFiles" },
  { value: "zfs", key: "activityLog.domainZFS" },
  { value: "everything", key: "activityLog.domainEverything" },
];

export const LOG_FILTER_KINDS: { value: LogFilterKind; key: string }[] = [
  { value: "all", key: "activityLog.filterAllTypes" },
  { value: "backup", key: "activityLog.typeBackup" },
  { value: "restore", key: "activityLog.typeRestore" },
  { value: "dbdump", key: "run.kindDbDump" },
  { value: "prune", key: "activityLog.typePrune" },
  { value: "verify", key: "activityLog.typeVerify" },
  { value: "offsite", key: "activityLog.typeOffsite" },
  // drill and tamper reuse the job labels, drdrill the Run History kind label.
  { value: "drill", key: "activityLog.jobDrill" },
  { value: "drdrill", key: "run.kindDRDrill" },
  { value: "tamper", key: "activityLog.jobTamper" },
  { value: "export", key: "activityLog.typeExport" },
];

export interface LogFilter {
  domain: LogFilterDomain;
  kind: LogFilterKind;
  /** Case-insensitive substring match against the message, the ISO date and
   *  the short localized date. */
  text: string;
  /** Locale for the localized-date match. Leave it undefined to match what
   *  formatLogDate displays; tests pass one explicitly. */
  lang?: string;
  /** Exact day (ISO YYYY-MM-DD), set by clicking a Dashboard heatmap cell and
   *  matched on the local calendar day the way the heatmap maps it. Like the
   *  quick filters, it never hides the idle line, the one that says what runs
   *  next. */
  day?: string;
}

/**
 * filterLogLines narrows `lines` by the domain and type quick filters, the
 * heatmap day and the free-text search. The search also matches the line's
 * date, in ISO form and in the short form the UI shows, so typing a date
 * narrows the log too.
 */
export function filterLogLines(lines: LogLine[], filter: LogFilter): LogLine[] {
  const q = filter.text.trim().toLowerCase();
  const lang = filter.lang;
  return lines.filter((l) => {
    // The idle line has no domain or kind, so the quick filters skip it; an
    // active chip must not hide the line saying what runs next.
    if (!l.idle) {
      if (filter.domain !== "all" && l.domain !== filter.domain) return false;
      if (filter.kind !== "all" && l.kind !== filter.kind) return false;
      if (filter.day && isoDateOf(l.atMs) !== filter.day) return false;
    }
    if (q) {
      const haystack = `${l.text} ${isoDateOf(l.atMs)} ${formatLogDate(l.atMs, lang)}`.toLowerCase();
      if (!haystack.includes(q)) return false;
    }
    return true;
  });
}
