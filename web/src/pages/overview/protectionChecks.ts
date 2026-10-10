// The four checks every section is measured by: a current backup on this
// server, a copy off the premises, delete protection there and a trial restore
// from there. The ring in the status head, the rows of the protection card and
// the window of one section all read them from here.

import type { BackupItem, DomainStatus } from "../../lib/api";
import { kindOfDomain, type EntryKind } from "../../lib/backupEntry";
import type { TranslationKey } from "../../lib/i18n";
import type { ProgressMap } from "../../lib/progress";
import { tallyEntries, type Tally } from "../../lib/protectionTally";

export type CheckTone = "ok" | "warn" | "fail" | "run" | "neutral";

export const CHECK_KEYS = ["local", "offsite", "lock", "drill"] as const;

export type CheckKey = (typeof CHECK_KEYS)[number];

export const CHECK_LABEL: Record<CheckKey, TranslationKey> = {
  local: "settings.pathMode.local",
  offsite: "source.offsite",
  lock: "storage.protected",
  drill: "overview.check.drill",
};

export interface Check {
  key: CheckKey;
  tone: CheckTone;
  say: TranslationKey;
  /** Unix seconds of what the check last saw, 0 where it saw nothing. */
  at: number;
}

type Verdict = [tone: CheckTone, say: TranslationKey, at?: number];

// VMs and the self-backup can have an off-site copy, but the server refuses
// to restore either into a sandbox.
const DRILL_DOMAINS = new Set(["containers", "flash", "files", "zfs"]);

export function canDrill(domain: string): boolean {
  return DRILL_DOMAINS.has(domain);
}

function localCheck(d: DomainStatus, tally: Tally): Verdict {
  if (tally.total === 0) return ["neutral", "overview.check.localNothing"];
  if (d.status === "never") return ["neutral", "overview.check.localNever"];
  if (d.status === "off") return ["neutral", "overview.check.localNoSchedule"];
  if (tally.entries.some((item) => item.lastRunStatus === "failed")) {
    return ["warn", "overview.check.localFailed", d.lastSuccess];
  }
  if (d.status === "overdue") return ["fail", "overview.check.localOverdue", d.lastSuccess];
  if (tally.none > 0 || tally.pending > 0) return ["warn", "overview.check.localPartly", d.lastSuccess];
  if (d.status === "warn") return ["warn", "overview.check.localDueSoon", d.lastSuccess];
  return ["ok", "overview.check.localCurrent", d.lastSuccess];
}

function offsiteCheck(d: DomainStatus, copying: boolean): Verdict {
  if (!d.offsiteConfigured) {
    return d.offPremisesCovered ? ["ok", "overview.check.offsiteCurrent"] : ["fail", "overview.check.offsiteNone"];
  }
  if (copying) return ["run", "overview.check.offsiteRunning"];
  if (d.replicationState === "overdue") return ["warn", "overview.check.offsiteOverdue", d.lastReplicationAt];
  if (d.replicationState === "paused") return ["warn", "overview.check.offsitePaused", d.lastReplicationAt];
  if (d.lastReplicationAt > 0 && d.replicationState !== "never") {
    return ["ok", "overview.check.offsiteCurrent", d.lastReplicationAt];
  }
  return ["neutral", "overview.check.offsiteNever"];
}

function lockCheck(d: DomainStatus): Verdict {
  if (!d.offsiteConfigured) return ["neutral", "overview.check.lockNeedsOffsite"];
  if (d.tamperState === "ok") return ["ok", "overview.check.lockChecked", d.lastTamperAt];
  if (d.tamperState === "failed") return ["fail", "overview.check.lockFailed", d.lastTamperAt];
  return ["warn", "overview.check.lockUnproven"];
}

function drillCheck(d: DomainStatus, drilling: boolean): Verdict {
  if (!d.offsiteConfigured) return ["neutral", "overview.check.drillNeedsOffsite"];
  if (!canDrill(d.domain)) return ["neutral", "overview.check.drillUnavailable"];
  if (drilling) return ["run", "overview.check.drillRunning"];
  if (d.lastDrDrillAt > 0) {
    if (!d.lastDrDrillOK) return ["fail", "overview.check.drillFailed", d.lastDrDrillAt];
    if (d.drillState === "overdue") return ["warn", "overview.check.drillOverdue", d.lastDrDrillAt];
    return ["ok", "overview.check.drillPassed", d.lastDrDrillAt];
  }
  return ["warn", d.offsiteDrillScheduled ? "overview.check.drillNever" : "overview.check.drillManual"];
}

export function domainChecks(
  d: DomainStatus,
  tally: Tally,
  live: { copying: boolean; drilling: boolean }
): Check[] {
  const verdicts = [localCheck(d, tally), offsiteCheck(d, live.copying), lockCheck(d), drillCheck(d, live.drilling)];
  return verdicts.map(([tone, say, at], i) => ({ key: CHECK_KEYS[i], tone, say, at: at ?? 0 }));
}

// The worst check decides. A section where nothing applies yet stays neutral
// instead of looking safe.
const TONE_RANK: CheckTone[] = ["fail", "warn", "run", "ok", "neutral"];

export function worstTone(checks: Check[]): CheckTone {
  return TONE_RANK.find((tone) => checks.some((c) => c.tone === tone)) ?? "neutral";
}

export const VERDICT: Record<CheckTone, TranslationKey> = {
  ok: "ransomware.protGreen",
  warn: "ransomware.protAmber",
  fail: "ransomware.protRed",
  run: "overview.verdict.checking",
  neutral: "overview.verdict.nothing",
};

/** One switched-on section with everything the overview says about it. */
export interface Section {
  kind: EntryKind;
  status: DomainStatus;
  tally: Tally;
  checks: Check[];
  tone: CheckTone;
}

/**
 * sectionsOf measures every switched-on section. `drilling` is the domain
 * whose trial restore was started from this page and has not come back yet.
 */
export function sectionsOf(
  domains: DomainStatus[],
  items: BackupItem[],
  progress: ProgressMap,
  drilling: string | null
): Section[] {
  return domains.flatMap((status) => {
    const kind = kindOfDomain(status.domain);
    if (!kind || !status.enabled) return [];
    const tally = tallyEntries(items, kind);
    const checks = domainChecks(status, tally, {
      copying: !!progress[`offsite:${status.domain}`]?.active,
      drilling: drilling === status.domain,
    });
    return [{ kind, status, tally, checks, tone: worstTone(checks) }];
  });
}
