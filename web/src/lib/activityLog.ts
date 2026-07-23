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
import type { ProgressMap } from "./progress";
import { STALE_MS } from "./progress";
import { formatClockTime, formatDuration } from "./reltime";

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

/** Visual/semantic bucket for a line's glyph + colour (see ActivityLog.tsx). */
export type LogStatus = "running" | "success" | "failed" | "offsite" | "info";

/** The domain a line belongs to, for the domain quick-filter. "" when a
 *  finished run's target could not be resolved (e.g. a deleted item). */
export type LogDomain = "containers" | "vms" | "flash" | "config" | "files" | "";

/** The operation kind, for the type quick-filter. "update" is a real kind
 *  (the post-backup image-update run) that deliberately has no dedicated
 *  filter chip (see ActivityLog.tsx) but still carries a kind for search.
 *  "drill" (restore-verification drill), "tamper" (off-site tamper test) and
 *  "export" (flash ZIP export) are persisted run kinds since the
 *  everything-in-the-log wave. */
export type LogKind = "backup" | "restore" | "prune" | "verify" | "offsite" | "update" | "drill" | "tamper" | "export" | "";

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

// ---------------------------------------------------------------------------
// Domain / job literal → translation key
// ---------------------------------------------------------------------------

const DOMAIN_KEYS: Record<string, string> = {
  containers: "activityLog.domainContainers",
  vms: "activityLog.domainVMs",
  flash: "activityLog.domainFlash",
  config: "activityLog.domainConfig",
  files: "activityLog.domainFiles",
};

const JOB_KEYS: Record<string, string> = {
  backup: "activityLog.jobBackup",
  offsite: "activityLog.jobOffsite",
  drill: "activityLog.jobDrill",
  tamper: "activityLog.jobTamper",
  digest: "activityLog.jobDigest",
};

/** Translates a domain literal ("containers"/"vms"/"flash"/"config"/"files");
 *  an unknown literal (should not happen) falls back to the raw string. */
function domainLabel(resolveName: ResolveName, domain: string): string {
  const key = DOMAIN_KEYS[domain];
  return key ? resolveName(key) : domain;
}

/** Translates a schedule job literal ("backup"/"offsite"/"drill"/"tamper"/
 *  "digest"); an unknown literal falls back to the raw string. */
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
  if (domain === "containers" || domain === "vms" || domain === "flash" || domain === "config" || domain === "files") {
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
  | { scope: "offsite" | "prune" | "verify"; domain: string };

/**
 * parseProgressKey decodes a live SSE progress key into what it refers to.
 * See web/src/lib/progress.ts for the wire key shapes this must track:
 * "container:<name>", "vm:<name>", "flash", "config", "files:<set>",
 * "batch:containers", "batch:files", "offsite:<domain>", "prune:<domain>",
 * "verify:<domain>". Every "<name>"/"<set>" suffix is ALREADY the human name
 * (the backend publishes "container:" + containerName, "files:" + set.Name,
 * etc. — see internal/api/service.go), so no id→name lookup is needed here.
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

function buildLiveLines(progressMap: ProgressMap, resolveName: ResolveName, now: number): LiveResult {
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
      const text = resolveName("activityLog.lineOffsiteRunning", { domain: domainLabel(resolveName, domain) });
      // Off-site replication now DOES write a Run row (kind="offsite" on the
      // domain target) — register the domain-op signature so the finished-run
      // line can't briefly double up with this live tail line.
      signatures.add(domainOpSignature("offsite", domain));
      lines.push({ id: `live:${key}`, atMs: state.lastSeen, status: "offsite", text, domain, kind: "offsite", live: true });
      continue;
    }

    // "prune" | "verify"
    const domain = normalizeDomain(parsed.domain);
    const templateKey = parsed.scope === "prune" ? "activityLog.linePruneRunning" : "activityLog.lineVerifyRunning";
    const text = resolveName(templateKey, { domain: domainLabel(resolveName, domain) });
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
        ? { status: "failed", text: resolveName("activityLog.linePruneFailed", { domain: domainText, error: run.error }) }
        : { status: "info", text: resolveName("activityLog.lineOther", { name: domainText, kind: run.kind, status: run.status }) };
  }

  if (run.kind === "verify") {
    return run.status === "success"
      ? { status: "success", text: resolveName("activityLog.lineVerifySuccess", { domain: domainText }) }
      : run.status === "failed"
        ? { status: "failed", text: resolveName("activityLog.lineVerifyFailed", { domain: domainText, error: run.error }) }
        : { status: "info", text: resolveName("activityLog.lineOther", { name: domainText, kind: run.kind, status: run.status }) };
  }

  if (run.kind === "offsite") {
    return run.status === "success"
      ? { status: "offsite", text: resolveName("activityLog.lineOffsiteSuccess", { domain: domainText, duration }) }
      : run.status === "failed"
        ? { status: "failed", text: resolveName("activityLog.lineOffsiteFailed", { domain: domainText, error: run.error }) }
        : { status: "info", text: resolveName("activityLog.lineOther", { name: domainText, kind: run.kind, status: run.status }) };
  }

  if (run.kind === "drill") {
    return run.status === "success"
      ? { status: "success", text: resolveName("activityLog.lineDrillSuccess", { domain: domainText }) }
      : run.status === "failed"
        ? { status: "failed", text: resolveName("activityLog.lineDrillFailed", { domain: domainText, error: run.error }) }
        : { status: "info", text: resolveName("activityLog.lineOther", { name: domainText, kind: run.kind, status: run.status }) };
  }

  if (run.kind === "tamper") {
    return run.status === "success"
      ? { status: "success", text: resolveName("activityLog.lineTamperSuccess", { domain: domainText }) }
      : run.status === "failed"
        ? { status: "failed", text: resolveName("activityLog.lineTamperFailed", { domain: domainText, error: run.error }) }
        : { status: "info", text: resolveName("activityLog.lineOther", { name: domainText, kind: run.kind, status: run.status }) };
  }

  if (run.kind === "export") {
    return run.status === "success"
      ? { status: "success", text: resolveName("activityLog.lineExportSuccess", { bytes: formatBytesShort(run.bytes), duration }) }
      : run.status === "failed"
        ? { status: "failed", text: resolveName("activityLog.lineExportFailed", { error: run.error }) }
        : { status: "info", text: resolveName("activityLog.lineOther", { name: domainText, kind: run.kind, status: run.status }) };
  }

  if (run.kind === "restore") {
    return run.status === "success"
      ? { status: "success", text: resolveName("activityLog.lineRestoreSuccess", { name, duration }) }
      : run.status === "failed"
        ? { status: "failed", text: resolveName("activityLog.lineRestoreFailed", { name, error: run.error }) }
        : { status: "info", text: resolveName("activityLog.lineOther", { name, kind: run.kind, status: run.status }) };
  }

  if (run.kind === "update") {
    return run.status === "success"
      ? { status: "success", text: resolveName("activityLog.lineUpdateSuccess", { name, duration }) }
      : run.status === "failed"
        ? { status: "failed", text: resolveName("activityLog.lineUpdateFailed", { name, error: run.error }) }
        : { status: "info", text: resolveName("activityLog.lineOther", { name, kind: run.kind, status: run.status }) };
  }

  // "backup" (and any future/unexpected kind falls back to the same shape).
  if (run.status === "success") {
    return { status: "success", text: resolveName("activityLog.lineBackupSuccess", { name, bytes: formatBytesShort(run.bytes), duration }) };
  }
  if (run.status === "failed") {
    return { status: "failed", text: resolveName("activityLog.lineBackupFailed", { name, error: run.error }) };
  }
  if (run.status === "skipped") {
    return { status: "info", text: resolveName("activityLog.lineBackupSkipped", { name, error: run.error }) };
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
  return kind === "prune" || kind === "verify" || kind === "offsite" || kind === "drill" || kind === "tamper" || kind === "export";
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
 */
export function buildLogLines(
  runs: Run[],
  progressMap: ProgressMap,
  scheduleNext: ScheduleNext[],
  resolveName: ResolveName,
  now: number
): LogLine[] {
  const { lines: liveLines, signatures } = buildLiveLines(progressMap, resolveName, now);
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
// filterLogLines — the client-side "docker logs | grep" filter
// ---------------------------------------------------------------------------

/** Domain quick-filter value ("all" plus every LogDomain except ""). */
export type LogFilterDomain = "all" | "containers" | "vms" | "flash" | "config" | "files";

/** Type quick-filter value ("all" plus the operation kinds the filter bar
 *  offers — deliberately NOT including "update", which has no chip). */
export type LogFilterKind = "all" | "backup" | "restore" | "prune" | "verify" | "offsite" | "drill" | "tamper" | "export";

export interface LogFilter {
  domain: LogFilterDomain;
  kind: LogFilterKind;
  /** Free-text, case-insensitive substring match against the line's message. */
  text: string;
}

/**
 * Narrows `lines` to those matching the domain/type quick-filters and the
 * free-text search — a pure filter, extracted from ActivityLog.tsx so it can
 * be unit-tested independently of any rendering.
 */
export function filterLogLines(lines: LogLine[], filter: LogFilter): LogLine[] {
  const q = filter.text.trim().toLowerCase();
  return lines.filter((l) => {
    // The idle line (`idle: true`) carries no domain/kind of its own — it is
    // exempt from the domain/type quick-filters so an active filter chip
    // never hides the only line telling the user what's coming next.
    if (!l.idle) {
      if (filter.domain !== "all" && l.domain !== filter.domain) return false;
      if (filter.kind !== "all" && l.kind !== filter.kind) return false;
    }
    if (q && !l.text.toLowerCase().includes(q)) return false;
    return true;
  });
}
