// ---------------------------------------------------------------------------
// activityLog — pure data layer for the dashboard "activity log": a flat,
// scrollable, docker-logs-style list of timestamped lines (NO zones), merged
// from three sources:
//
//   1. Finished runs (GET /api/runs via listRuns) — one line per completed
//      backup/restore/update/prune/verify/offsite/drill/tamper/export,
//      ordered by finish time.
//   2. Currently-active SSE progress keys (useProgress()) — live tail lines,
//      always rendered at the very bottom ("now").
//   3. The soonest scheduled fire (GET /api/schedule/next) — a trailing idle
//      "next up" line, shown only while nothing is active.
//
// `buildLogLines` is the single pure entry point: given plain data (no
// React, no fetch, no Date.now() reached for internally) it returns the
// ordered, deduped `LogLine[]` the component renders. Keeping it pure makes
// the merge/dedupe/ordering logic reasoned-about and unit-testable without a
// live i18n context, SSE connection or clock.
// ---------------------------------------------------------------------------

import type { Run, ScheduleNext } from "./api";
import type { ProgressMap, ProgressState } from "./progress";
import { offsiteRunProgress, STALE_MS } from "./progress";
import { elapsedSince, formatClockTime, formatDuration } from "./reltime";
import { RUN_REASONS } from "./runReason";

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

/** Visual/semantic bucket for a line's glyph + colour (see ActivityLog.tsx). */
export type LogStatus = "running" | "success" | "failed" | "offsite" | "info";

/** The domain a line belongs to, for the domain quick-filter. "everything" is
 *  the "Backup Everything" pseudo-domain (a sequential pass over the other
 *  five, see store.EverythingTargetID / runTargetMaps on the backend). "" when
 *  a finished run's target could not be resolved (e.g. a deleted item). */
export type LogDomain = "containers" | "vms" | "flash" | "config" | "files" | "everything" | "";

/** The operation kind, for the type quick-filter. "update" is a real kind
 *  (the post-backup image-update run) that deliberately has no dedicated
 *  filter chip (see ActivityLog.tsx) but still carries a kind for search.
 *  "drill" (local restore-verification drill), "drdrill" (off-site DR restore
 *  check — a distinct kind so the two drill families are tellable apart; rows
 *  recorded before the split stay "drill"), "tamper" (off-site tamper test)
 *  and "export" (flash ZIP export) are persisted run kinds since the
 *  everything-in-the-log wave. */
export type LogKind = "backup" | "restore" | "prune" | "verify" | "offsite" | "update" | "drill" | "drdrill" | "tamper" | "export" | "";

export interface LogLine {
  /** Stable React key. */
  id: string;
  /** Epoch ms used for ordering. Finished runs: finishedAt (fallback
   *  startedAt). Live lines: the progress entry's lastSeen. Idle: `now`. */
  atMs: number;
  status: LogStatus;
  /** Fully rendered, already-localized message (no timestamp/glyph). */
  text: string;
  domain: LogDomain;
  kind: LogKind;
  /** True for a currently-active tail line (updates in place). */
  live: boolean;
  /** True only for the trailing idle "next up"/"nothing yet" line, which
   *  carries no domain/kind of its own (nothing has run/is scheduled to a
   *  specific item yet) — exempts it from the domain/type quick-filters in
   *  filterLogLines so an active filter chip can't hide it. */
  idle?: boolean;
}

/**
 * Resolves a translation key (optionally with `{placeholder}` params) to its
 * localized, interpolated string. Injected so `buildLogLines` stays pure and
 * framework-free — the real implementation (ActivityLog.tsx) closes over
 * `useT()`'s `t`; a test can pass a trivial stub instead.
 */
export type ResolveName = (key: string, params?: Record<string, string>) => string;

/**
 * A run's reason, in the reader's language where it is one of ours ([377]).
 *
 * Every "…failed: {error}" and "…skipped: {error}" line below fills that
 * placeholder from runs.error, so before this the log read half-translated:
 * "MinIO-Backup übersprungen: container no longer exists on the host", a German
 * sentence finished in English. Measured on jdp's dashboard, where three
 * definitions produced exactly that line twelve times over.
 *
 * Translating HERE rather than in each of the thirteen call sites keeps the fix
 * in one place, and keeps buildLogLines pure: RUN_REASONS maps our sentences to
 * keys, and resolveName is already the injected way this module turns a key
 * into text.
 *
 * A message from restic, rclone or Docker is passed through untouched, which is
 * the correct outcome for all of them.
 */
function reasonText(raw: string | undefined, resolveName: ResolveName): string {
  if (!raw) return "";
  const key = RUN_REASONS[raw.trim()];
  return key ? resolveName(key) : raw;
}

// ---------------------------------------------------------------------------
// Domain / job literal → translation key
// ---------------------------------------------------------------------------

const DOMAIN_KEYS: Record<string, string> = {
  containers: "activityLog.domainContainers",
  vms: "activityLog.domainVMs",
  flash: "activityLog.domainFlash",
  config: "activityLog.domainConfig",
  files: "activityLog.domainFiles",
  everything: "activityLog.domainEverything",
};

const JOB_KEYS: Record<string, string> = {
  backup: "activityLog.jobBackup",
  offsite: "activityLog.jobOffsite",
  drill: "activityLog.jobDrill",
  tamper: "activityLog.jobTamper",
  digest: "activityLog.jobDigest",
  watchdog: "activityLog.jobWatchdog",
  // Both were missing, so both fell through to the raw literal: the scheduler
  // emits job "receiver" and (since the fleet sweep learned its own name) job
  // "fleet". An unmapped job renders as the bare English identifier.
  receiver: "activityLog.jobReceiver",
  fleet: "activityLog.jobFleet",
};

/** Translates a domain literal ("containers"/"vms"/"flash"/"config"/"files");
 *  an unknown literal (should not happen) falls back to the raw string. */
function domainLabel(resolveName: ResolveName, domain: string): string {
  const key = DOMAIN_KEYS[domain];
  return key ? resolveName(key) : domain;
}

/** Translates a schedule job literal ("backup"/"offsite"/"drill"/"tamper"/
 *  "digest"/"watchdog"/"receiver"/"fleet"); an unknown literal falls back to the
 *  raw string, which is why an unmapped job shows up as bare English. */
function jobLabel(resolveName: ResolveName, job: string): string {
  const key = JOB_KEYS[job];
  return key ? resolveName(key) : job;
}

/**
 * normalizeDomain maps the singular item-domain vocabulary used by
 * runView.Domain / progress keys ("container"/"vm") to the plural domain
 * literal used everywhere else (filter chips, prune/verify domains):
 * "container"→"containers", "vm"→"vms". "files"/"flash"/"config"/"" pass
 * through unchanged (already canonical or empty/unresolved).
 */
function normalizeDomain(domain: string): LogDomain {
  if (domain === "container") return "containers";
  if (domain === "vm") return "vms";
  if (
    domain === "containers" ||
    domain === "vms" ||
    domain === "flash" ||
    domain === "config" ||
    domain === "files" ||
    domain === "everything"
  ) {
    return domain;
  }
  return "";
}

// ---------------------------------------------------------------------------
// Small pure formatters
// ---------------------------------------------------------------------------

/** Clamp + round a percent to a display-safe 0..100 integer. */
function displayPercent(percent: number): number {
  if (!Number.isFinite(percent)) return 0;
  return Math.round(Math.max(0, Math.min(100, percent)));
}

/** Binary (1024) byte formatter, one decimal — mirrors Dashboard's humanBytes
 *  so the activity log reads the same way the storage/backups cards do. */
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

// ---------------------------------------------------------------------------
// Progress key parsing
// ---------------------------------------------------------------------------

type ParsedKey =
  | { scope: "item"; domain: "container" | "vm" | "files" | "flash" | "config"; name: string }
  | { scope: "batch"; domain: string }
  | { scope: "offsite" | "prune" | "verify" | "drill" | "drdrill" | "tamper" | "export"; domain: string };

/**
 * parseProgressKey decodes a live SSE progress key into what it refers to.
 * See web/src/lib/progress.ts for the wire key shapes this must track:
 * "container:<name>", "vm:<name>", "flash", "config", "files:<set>",
 * "batch:containers", "batch:files", "offsite:<domain>", "prune:<domain>",
 * "verify:<domain>", "drill:<domain>" (local subset drill), "drdrill:<domain>"
 * (off-site DR restore check), "tamper:<domain>", "export:flash"
 * (#109 — drills/tamper tests/the flash-ZIP export publish live pairs too).
 * Every "<name>"/"<set>" suffix is ALREADY the human name (the backend
 * publishes "container:" + containerName, "files:" + set.Name, etc. — see
 * internal/api/service.go), so no id→name lookup is needed here.
 * Returns null for an unrecognized key shape (defensive; should not happen).
 */
function parseProgressKey(key: string): ParsedKey | null {
  if (key === "flash") return { scope: "item", domain: "flash", name: "flash" };
  if (key === "config") return { scope: "item", domain: "config", name: "config" };
  if (key.startsWith("container:")) return { scope: "item", domain: "container", name: key.slice("container:".length) };
  if (key.startsWith("vm:")) return { scope: "item", domain: "vm", name: key.slice("vm:".length) };
  if (key.startsWith("files:")) return { scope: "item", domain: "files", name: key.slice("files:".length) };
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
 * itemDisplayName resolves an item-scope key's display name. Real
 * container/VM/file-set names are shown verbatim (they are proper nouns, not
 * translatable); the two singleton domains ("flash"/"config" keys with no
 * suffix) get their translated domain label instead. Disambiguated by
 * `parsed.domain` (which key prefix matched), not by the name string, so a
 * container coincidentally named "flash" is never mistaken for the flash
 * singleton (its key would be "container:flash", domain "container").
 */
function itemDisplayName(resolveName: ResolveName, parsed: Extract<ParsedKey, { scope: "item" }>): string {
  if (parsed.domain === "flash") return domainLabel(resolveName, "flash");
  if (parsed.domain === "config") return domainLabel(resolveName, "config");
  return parsed.name;
}

// ---------------------------------------------------------------------------
// Live lines
// ---------------------------------------------------------------------------

interface LiveResult {
  lines: LogLine[];
  /** Signatures of currently-active operations, used to suppress the
   *  finished-run line that would otherwise briefly double up with it. */
  signatures: Set<string>;
}

// itemSignature builds the dedupe key for an item-scope operation (backup/
// restore of one container/VM/file-set/flash/config), so a finished run's
// history line can be suppressed while its live tail line is still showing.
//
// Flash and config are domain-wide singletons, and their two callers disagree
// on a display name: the live tail resolves the TRANSLATED domain label (see
// itemDisplayName — e.g. "Flash"), while the finished run's `target` is the
// backend's hard-coded English name (handlers.go: "Unraid flash"/"App
// configuration" — see internal/api/handlers.go handleRuns). Keying on that
// name would never match, so the two signatures agree by domain alone for
// singleton domains — that's already unambiguous since there is exactly one
// flash item and one config item. Containers/vms/files still key on their
// real (untranslated, stable) item name, since a domain can have many.
function itemSignature(kind: string, domain: LogDomain, name: string): string {
  if (domain === "flash" || domain === "config") return `item|${kind}|${domain}`;
  return `item|${kind}|${domain}|${name}`;
}

function domainOpSignature(kind: string, domain: string): string {
  return `domain|${kind}|${domain}`;
}

/** Live-line text template per domain-scoped operation scope ("Pruning —
 *  {domain} …" etc.). The export line deliberately takes no {domain} — the
 *  flash-ZIP export is flash-only, so its text names flash itself. */
const DOMAIN_OP_RUNNING_KEYS: Record<"prune" | "verify" | "drill" | "drdrill" | "tamper" | "export", string> = {
  prune: "activityLog.linePruneRunning",
  verify: "activityLog.lineVerifyRunning",
  drill: "activityLog.lineDrillRunning",
  drdrill: "activityLog.lineDRDrillRunning",
  tamper: "activityLog.lineTamperRunning",
  export: "activityLog.lineExportRunning",
};

/**
 * offsiteLiveLineText picks the honest live-line text for an "offsite:<domain>"
 * progress state (issue #159), mirroring OffsiteIndicator's offsiteStatusText
 * tiering exactly (see that function's doc comment for the full reasoning):
 * a RUN-LEVEL percentage when one can honestly be derived ("… {percent}%
 * overall (snapshot {index} of {total})"), else the plain elapsed-duration
 * text, else the bare "running" text. Each tier has its own "WithDuration"
 * sibling key so a live percentage never has to drop the duration.
 *
 * The percentage comes from progress.ts's shared offsiteRunProgress so this
 * line and OffsiteIndicator cannot drift apart on the arithmetic — see that
 * function for why a raw per-snapshot percentage next to "k of N" is the
 * defect being fixed here, not the feature.
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
 * buildLiveLines renders the live SSE progress keys as tail lines. `now` gates
 * staleness (STALE_MS) — deliberately coarse-tick-tolerant, since a lagging
 * `now` only delays noticing a lost terminal frame by a bit. `liveNow`
 * (defaults to `now` for callers that don't need finer granularity — e.g.
 * every existing test) is used ONLY for the off-site line's elapsedSince
 * computation: ActivityLog.tsx ticks `now` at a coarse 60s cadence (its idle
 * "next up" countdown doesn't need better), which used to ALSO starve the
 * off-site duration — for the run's first ~60s, `now` could sit BEHIND
 * `startedAt` (captured before the run began), making elapsedSince go
 * negative → "" (blank), then jump straight to a large value once `now`
 * finally ticked. `liveNow` is a separate, faster-ticking clock the caller
 * only runs while a live line is on screen.
 */
function buildLiveLines(progressMap: ProgressMap, resolveName: ResolveName, now: number, liveNow: number = now): LiveResult {
  const lines: LogLine[] = [];
  const signatures = new Set<string>();

  for (const key of Object.keys(progressMap)) {
    const state = progressMap[key];
    // A terminal SSE frame lost in transit can leave active:true stuck
    // forever (see progress.ts STALE_MS/anyActive) — treat a stale entry as
    // no longer live so it can't wedge a "running…" line in place forever.
    const stale = now - state.lastSeen > STALE_MS;
    if (!state.active || stale) continue;

    const parsed = parseProgressKey(key);
    if (!parsed) continue; // unrecognized key shape — skip defensively

    if (parsed.scope === "item") {
      const name = itemDisplayName(resolveName, parsed);
      const domain = normalizeDomain(parsed.domain);
      const kind: LogKind = state.phase === "restore" ? "restore" : "backup";
      const pct = displayPercent(state.percent);
      const text =
        kind === "restore"
          ? resolveName("activityLog.lineRestoringItem", { name, percent: String(pct) })
          : resolveName("activityLog.lineBackingUpItem", { name, percent: String(pct) });
      signatures.add(itemSignature(kind, domain, name));
      lines.push({ id: `live:${key}`, atMs: state.lastSeen, status: "running", text, domain, kind, live: true });
      continue;
    }

    if (parsed.scope === "batch") {
      const domain = normalizeDomain(parsed.domain);
      const pct = displayPercent(state.percent);
      const text = resolveName("activityLog.lineBackingUpBatch", {
        domain: domainLabel(resolveName, domain),
        percent: String(pct),
      });
      // No Run row is ever attributed to a "batch:*" key itself (each member
      // item gets its own backup run) — nothing to dedupe against.
      lines.push({ id: `live:${key}`, atMs: state.lastSeen, status: "running", text, domain, kind: "backup", live: true });
      continue;
    }

    if (parsed.scope === "offsite") {
      const domain = normalizeDomain(parsed.domain);
      // Issue #159: restic copy DOES print a real per-snapshot percentage
      // (see restic.Copy's doc comment) — offsiteLiveLineText shows it once
      // available, falling back to the honest elapsed-duration signal (from
      // the backend-stamped startedAt), computed against `liveNow` (not
      // `now` — see buildLiveLines' doc comment for why that distinction
      // matters here) so it never goes negative for this component's first
      // ~60s.
      const duration = elapsedSince(state.startedAt, liveNow);
      const text = offsiteLiveLineText(resolveName, domain, state, duration);
      // Off-site replication now DOES write a Run row (kind="offsite" on the
      // domain target) — register the domain-op signature so the finished-run
      // line can't briefly double up with this live tail line.
      signatures.add(domainOpSignature("offsite", domain));
      lines.push({ id: `live:${key}`, atMs: state.lastSeen, status: "offsite", text, domain, kind: "offsite", live: true });
      continue;
    }

    // "prune" | "verify" | "drill" | "drdrill" | "tamper" | "export" —
    // domain-scoped ops.
    // Same dedupe mechanics as offsite above: each of these records a Run row
    // on the reserved domain target when it finishes (recordDomainRun /
    // StartRun+FinishRun), so registering the domain-op signature lets the
    // finished-run line supersede this live tail line without doubling up.
    const domain = normalizeDomain(parsed.domain);
    const text = resolveName(DOMAIN_OP_RUNNING_KEYS[parsed.scope], { domain: domainLabel(resolveName, domain) });
    signatures.add(domainOpSignature(parsed.scope, domain));
    lines.push({ id: `live:${key}`, atMs: state.lastSeen, status: "running", text, domain, kind: parsed.scope, live: true });
  }

  return { lines, signatures };
}

// ---------------------------------------------------------------------------
// Finished-run lines
// ---------------------------------------------------------------------------

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
    // "skipped" = the test ran but produced no verdict (non-REST off-site,
    // transport error, inconclusive probe) — a neutral info line carrying the
    // backend's reason, never a red.
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

  // "backup" (and any future/unexpected kind falls back to the same shape).
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

/** Narrows a raw Run.kind string to the known LogKind set; an unexpected
 *  future kind falls back to "" rather than a bogus filter value. */
function asLogKind(kind: string): LogKind {
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

/** Kinds recorded against the reserved DOMAIN target id (see the backend's
 *  domainRunTargetID): their targetId IS the domain literal (or the flash/
 *  config singleton id), never a resolvable item id. */
function isDomainOpKind(kind: string): boolean {
  return kind === "prune" || kind === "verify" || kind === "offsite" || kind === "drill" || kind === "drdrill" || kind === "tamper" || kind === "export";
}

function buildHistoryLines(runs: Run[], resolveName: ResolveName, liveSignatures: Set<string>): LogLine[] {
  const lines: LogLine[] = [];
  for (const run of runs) {
    // Only COMPLETED runs come from history — an in-flight run is represented
    // by its live progress line instead (see the module doc comment).
    if (run.finishedAt == null) continue;

    const isDomainOp = isDomainOpKind(run.kind);
    const domain: LogDomain = isDomainOp ? normalizeDomain(run.targetId) : normalizeDomain(run.domain);
    const name = run.target;

    const signature = isDomainOp ? domainOpSignature(run.kind, domain) : itemSignature(run.kind, domain, name);
    if (liveSignatures.has(signature)) continue; // superseded by its live tail line

    const { status, text } = finishedLineText(resolveName, run, domain, name);
    lines.push({ id: `run:${run.id}`, atMs: run.finishedAt * 1000, status, text, domain, kind: asLogKind(run.kind), live: false });
  }
  return lines;
}

// ---------------------------------------------------------------------------
// Idle "next up" line
// ---------------------------------------------------------------------------

function buildIdleLine(scheduleNext: ScheduleNext[], resolveName: ResolveName, now: number, hasHistory: boolean): LogLine | null {
  const next = scheduleNext[0];
  if (!next) {
    // No live lines AND no history AND nothing scheduled — truly empty.
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

// ---------------------------------------------------------------------------
// buildLogLines — the pure merge/dedupe/order entry point
// ---------------------------------------------------------------------------

/**
 * Merges finished runs, live progress and the next scheduled fire into one
 * ordered, deduped `LogLine[]` — oldest first, live lines always last (they
 * are "now"), with a trailing idle line only when nothing is currently
 * active. Pure: given the same inputs it always returns the same output.
 *
 * `liveNow` (defaults to `now`) is an optional finer-grained clock used ONLY
 * for the off-site live line's elapsed-duration computation — see
 * buildLiveLines' doc comment for why it needs to tick faster than `now`
 * does in the real component.
 */
export function buildLogLines(
  runs: Run[],
  progressMap: ProgressMap,
  scheduleNext: ScheduleNext[],
  resolveName: ResolveName,
  now: number,
  liveNow: number = now
): LogLine[] {
  const { lines: liveLines, signatures } = buildLiveLines(progressMap, resolveName, now, liveNow);
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

// ---------------------------------------------------------------------------
// Per-line date formatting (#104 — the log now spans many days since
// everything-in-the-log, so a time-only stamp is ambiguous and must also be
// date-searchable)
// ---------------------------------------------------------------------------

/**
 * formatLogDate renders a line's `atMs` as a locale-aware short date — day/
 * month in the ORDER the locale reads it (e.g. "23.07." for de, "24/07" for
 * pt-PT, "07/23" for en-US) — via Intl.DateTimeFormat. `locale` is normally
 * OMITTED: `undefined` runs the SAME default-locale negotiation every other
 * date in the app uses (formatTs's plain toLocaleString), which is the fix
 * for issue #108 — navigator.language can disagree with the browser's
 * formatting default (e.g. a macOS "en-US" UI language with a Portuguese
 * region), which made the log the ONLY place showing US order. Tests pass
 * an explicit locale for deterministic assertions. Exported so
 * ActivityLog.tsx can pair it with reltime.ts's formatClockTime for the
 * leftmost per-line stamp; filterLogLines below builds the identical string
 * into its search haystack so typing that same date filters correctly.
 * Deliberately only the date is locale-ordered — the time stays
 * formatClockTime's fixed 24-hour face, for the same reason that helper
 * gives (a stable, unambiguous clock, not a locale-varying 12/24-hour one).
 */
export function formatLogDate(atMs: number, locale?: string): string {
  return new Intl.DateTimeFormat(locale, { day: "2-digit", month: "2-digit" }).format(new Date(atMs));
}

/**
 * isoDateOf renders a line's `atMs` as its local-calendar-day ISO date
 * (YYYY-MM-DD), independent of the app's display language — lets
 * filterLogLines match a typed ISO date (e.g. "2026-07-23") no matter which
 * language is active.
 */
function isoDateOf(atMs: number): string {
  const d = new Date(atMs);
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

// ---------------------------------------------------------------------------
// filterLogLines — the client-side "docker logs | grep" filter
// ---------------------------------------------------------------------------

/** Domain quick-filter value ("all" plus every LogDomain except ""). */
export type LogFilterDomain = "all" | "containers" | "vms" | "flash" | "config" | "files" | "everything";

/** Type quick-filter value ("all" plus the operation kinds the filter bar
 *  offers — deliberately NOT including "update", which has no chip). "drill"
 *  and "drdrill" are separate filter values (local subset check vs off-site DR
 *  restore check); DR rows recorded before the kind split stay "drill" and so
 *  keep matching the drill filter. */
export type LogFilterKind = "all" | "backup" | "restore" | "prune" | "verify" | "offsite" | "drill" | "drdrill" | "tamper" | "export";

export interface LogFilter {
  domain: LogFilterDomain;
  kind: LogFilterKind;
  /** Free-text, case-insensitive substring match against the line's message,
   *  its ISO date (YYYY-MM-DD) and its locale-short date (#104). */
  text: string;
  /** Explicit date locale for the localized-date match (#104). Normally
   *  OMITTED — undefined runs the browser's default-locale negotiation,
   *  matching exactly what formatLogDate displays (#108). Tests pass an
   *  explicit locale for deterministic assertions. */
  lang?: string;
  /** Optional exact-day filter (ISO YYYY-MM-DD), set by clicking a Dashboard
   *  heatmap cell: keeps only lines whose `atMs` falls on that LOCAL calendar
   *  day (the same local-day mapping the heatmap itself uses). Like the
   *  domain/kind quick-filters — and unlike the free-text search — the idle
   *  "next up" line is exempt, so the day chip can never hide the only line
   *  telling the user what's coming next. */
  day?: string;
}

/**
 * Narrows `lines` to those matching the domain/type quick-filters, the
 * optional heatmap day filter and the free-text search — a pure filter,
 * extracted from ActivityLog.tsx so it can be unit-tested independently of
 * any rendering. The free-text search matches against the line's message AND
 * its date (both the ISO form and the locale-short form the UI displays), so
 * typing a date narrows the log too.
 */
export function filterLogLines(lines: LogLine[], filter: LogFilter): LogLine[] {
  const q = filter.text.trim().toLowerCase();
  const lang = filter.lang; // undefined = browser-default locale, matching the display (#108)
  return lines.filter((l) => {
    // The idle line (`idle: true`) carries no domain/kind of its own — it is
    // exempt from the domain/type quick-filters so an active filter chip
    // never hides the only line telling the user what's coming next.
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
