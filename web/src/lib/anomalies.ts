// The wording of a finding. The backend sends a metric, a few numbers and the
// ids behind them; which sentence they make, in which language and with which
// units, is decided here and nowhere else, so the same finding reads the same
// on the dashboard card, on the page and in a badge's bubble.
//
// Framework-free like forecast.ts: the translator is injected, so the tables
// are the only thing a test has to stand up.

import type {
  AnomalyItem,
  AnomalySeverity,
  AnomalyState,
  AnomalyView,
} from "./api";
import type { TranslationKey } from "./i18n";
import type { BadgeTone } from "../components/Badge";
import { dbDumpNameOf, isDbDumpIdentity } from "./dbdump";
import { humanBytes } from "./forecast";
import { isolateLtr } from "./ltrFragments";
import { formatMillis } from "./reltime";

/** Translates a key, and with a count picks the form that count needs. */
export type TranslateAnomaly = (key: TranslationKey, n?: number) => string;

/** Dispatched after an acknowledgement or an expectation, so every open list
 *  refetches without waiting for the next poll. */
export const ANOMALY_CHANGED_EVENT = "bv:anomalies-changed";

const SEVERITY_TONE: Record<AnomalySeverity, BadgeTone> = {
  critical: "fail",
  warning: "warn",
  info: "neutral",
};

export const ANOMALY_SEVERITY_LABEL: Record<AnomalySeverity, TranslationKey> = {
  critical: "anomaly.severity.critical",
  warning: "anomaly.severity.warning",
  info: "anomaly.severity.info",
};

export const ANOMALY_STATE_LABEL: Record<AnomalyState, TranslationKey> = {
  open: "anomaly.state.open",
  resolved: "anomaly.state.resolved",
  acknowledged: "anomaly.state.acknowledged",
  expected: "anomaly.state.expected",
};

/** The three presets. The empty setting, "follow the global one", is not a
 *  preset and carries the global name in its own label. */
export const ANOMALY_SENSITIVITY_LABEL: Record<string, TranslationKey> = {
  strict: "anomaly.sensitivity.strict",
  balanced: "anomaly.sensitivity.balanced",
  permissive: "anomaly.sensitivity.permissive",
};

/** The lowest severity that still sends a message. */
export const ANOMALY_NOTIFY_LABEL: Record<string, TranslationKey> = {
  critical: "anomaly.settings.notify.critical",
  warning: "anomaly.settings.notify.warning",
  info: "anomaly.settings.notify.info",
  off: "anomaly.settings.notify.off",
};

/** What a user can declare normal, one entry per rule and direction. */
export const ANOMALY_FAMILY_LABEL: Record<string, TranslationKey> = {
  new_data: "anomaly.family.newData",
  source_bytes_down: "anomaly.family.sourceBytesDown",
  source_bytes_up: "anomaly.family.sourceBytesUp",
  source_files_down: "anomaly.family.sourceFilesDown",
  duration: "anomaly.family.duration",
  dump_bytes_down: "anomaly.family.dumpBytesDown",
  dump_bytes_up: "anomaly.family.dumpBytesUp",
  dump_duration: "anomaly.family.dumpDuration",
};

// An item carries the singular domain of its own table ("container"), a drill
// scope the plural of the domain it ran for ("containers"), so both spellings
// resolve to one label.
export const ANOMALY_DOMAIN_LABEL: Record<string, TranslationKey> = {
  container: "dashboard.domainContainers",
  containers: "dashboard.domainContainers",
  vm: "dashboard.domainVMs",
  vms: "dashboard.domainVMs",
  files: "dashboard.domainFiles",
  zfs: "dashboard.domainZFS",
  flash: "dashboard.domainFlash",
  config: "dashboard.domainConfig",
};

export function anomalySeverityTone(severity: AnomalySeverity): BadgeTone {
  return SEVERITY_TONE[severity] ?? "neutral";
}

/**
 * anomalyDomainsLabel translates the backup types a finding covers. A capacity
 * finding belongs to a volume rather than to one type, and where none is named
 * the disk is described by what sits on it.
 */
export function anomalyDomainsLabel(domains: string, t: TranslateAnomaly): string {
  const labels = domains
    .split(",")
    .map((d) => ANOMALY_DOMAIN_LABEL[d.trim()])
    .filter((key): key is TranslationKey => !!key)
    .map((key) => t(key));
  return labels.length > 0 ? [...new Set(labels)].join(", ") : t("repos.title");
}

/** What a finding is about, for a list, a chip or a row heading. */
export function anomalyItemLabel(
  a: { name: string; domain: string; scopeKind: string; part: string },
  t: TranslateAnomaly
): string {
  if (a.scopeKind === "zfsds" && a.part) return a.part;
  if (a.scopeKind === "dump") return t("anomaly.dumpOf").replace("{name}", a.name);
  if (a.name) return a.name;
  return anomalyDomainsLabel(a.domain, t);
}

const SPAN_DAY_CAP = 14;

/**
 * anomalyTimeSpan renders a projection in days or, from a fortnight on, in
 * weeks. Intl applies the language's own plural rule, which a translated value
 * with a number in it could not.
 */
export function anomalyTimeSpan(days: number, t: TranslateAnomaly, locale: string): string {
  if (!Number.isFinite(days) || days < 1) return t("anomaly.days.lessThanOne");
  const weeks = days >= SPAN_DAY_CAP;
  const value = Math.round(weeks ? days / 7 : days);
  return new Intl.NumberFormat(locale, {
    style: "unit",
    unit: weeks ? "week" : "day",
    unitDisplay: "long",
  }).format(value);
}

export function anomalyErrorText(code: string | undefined, t: TranslateAnomaly): string {
  switch (code) {
    case "bad-filter":
      return t("anomaly.error.badFilter");
    case "bad-request":
      return t("anomaly.error.badRequest");
    case "not-found":
      return t("anomaly.error.notFound");
    default:
      return t("anomaly.error.generic");
  }
}

/** How far a series has come, as a badge caption. */
export function anomalyLearningText(
  t: TranslateAnomaly,
  samples: number,
  needed: number,
  noData: boolean
): string {
  if (noData) return t("anomaly.noData");
  if (samples >= needed) return t("anomaly.learningDone");
  return t("anomaly.learning")
    .replace("{n}", samples.toLocaleString())
    .replace("{needed}", needed.toLocaleString());
}

export function worstSeverity(open: Record<AnomalySeverity, number>): AnomalySeverity | null {
  if (open.critical > 0) return "critical";
  if (open.warning > 0) return "warning";
  if (open.info > 0) return "info";
  return null;
}

/** An item's open findings with those of its dump and its datasets, the set
 *  the item's own filter on the Anomalies page shows. */
export function itemOpenCounts(item: AnomalyItem): Record<AnomalySeverity, number> {
  const series = [item, ...(item.dump ? [item.dump] : []), ...item.datasets];
  return {
    critical: series.reduce((n, s) => n + s.open.critical, 0),
    warning: series.reduce((n, s) => n + s.open.warning, 0),
    info: series.reduce((n, s) => n + s.open.info, 0),
  };
}

/** The id findings use for a snapshot. An off-site copy has an id of its own
 *  and carries the local one as `original`. */
export function findingSnapshotId(snap: { id: string; original?: string }): string {
  return snap.original || snap.id;
}

/**
 * heldTagLabels names the items behind the identity tags a prune kept. A VM's
 * block disks carry tags of their own and read as the VM, and the flash drive
 * and the self-backup have no name beyond their backup type.
 */
export function heldTagLabels(tags: string[], t: TranslateAnomaly): string[] {
  const labels = tags.map((tag) => {
    if (isDbDumpIdentity(tag)) return t("anomaly.dumpOf").replace("{name}", dbDumpNameOf(tag));
    if (tag === "flash" || tag === "config") return t(ANOMALY_DOMAIN_LABEL[tag]);
    const name = tag.slice(tag.indexOf(":") + 1);
    return tag.startsWith("vm:") ? name.replace(/:zvol:.*$/, "") : name;
  });
  return [...new Set(labels)];
}

const SEVERITY_RANK: Record<AnomalySeverity, number> = { critical: 0, warning: 1, info: 2 };

/** Critical first, a finding whose cause is still there before one that went
 *  away, and the newest of those first. */
export function sortOpenAnomalies(list: AnomalyView[]): AnomalyView[] {
  return [...list].sort(
    (a, b) =>
      SEVERITY_RANK[a.severity] - SEVERITY_RANK[b.severity] ||
      Number(a.recoveredAt > 0) - Number(b.recoveredAt > 0) ||
      b.lastSeenAt - a.lastSeenAt
  );
}

function numberOf(details: AnomalyView["details"], key: string): number {
  const value = details[key];
  return typeof value === "number" ? value : 0;
}

function fill(text: string, params: Record<string, string>): string {
  let out = text;
  for (const [name, value] of Object.entries(params)) out = out.replace(`{${name}}`, value);
  return out;
}

// A figure goes into the sentence isolated, or right-to-left prose reorders it.
function bytes(n: number): string {
  return isolateLtr(humanBytes(n));
}

function count(n: number): string {
  return isolateLtr(Math.round(n).toLocaleString());
}

function levelParams(a: AnomalyView): Record<string, string> {
  return { current: bytes(a.observed), typical: bytes(a.expected) };
}

function countParams(a: AnomalyView): Record<string, string> {
  return { current: count(a.observed), typical: count(a.expected) };
}

function durationParams(a: AnomalyView): Record<string, string> {
  return {
    current: isolateLtr(formatMillis(a.observed)),
    typical: isolateLtr(formatMillis(a.expected)),
  };
}

function shrinkKey(details: AnomalyView["details"], collapse: TranslationKey, drain: TranslationKey,
  plain: TranslationKey): TranslationKey {
  if (details.collapse) return collapse;
  if (details.drain) return drain;
  return plain;
}

/**
 * anomalySentence is the one place a finding becomes a sentence. A metric the
 * tables have no wording for still says that something was found, because a
 * browser can hold an older page than the server it talks to.
 */
export function anomalySentence(a: AnomalyView, t: TranslateAnomaly, locale = "en"): string {
  const name = anomalyItemLabel(a, t);
  const d = a.details;

  switch (a.metric) {
    case "new_data":
      return fill(t("anomaly.sentence.newData"), {
        name,
        bytes: bytes(a.observed),
        typical: bytes(numberOf(d, "refBytes")),
      });
    case "new_data_rewrite":
      return fill(t(d.allFilesNew ? "anomaly.sentence.newDataRenamed" : "anomaly.sentence.newDataRewrite"), {
        name,
        bytes: bytes(a.observed),
        source: bytes(numberOf(d, "sourceBytes")),
      });
    case "new_data_full":
      return fill(t("anomaly.sentence.newDataFull"), { name, bytes: bytes(a.observed) });
    case "source_bytes_shrink":
      return fill(
        t(shrinkKey(d, "anomaly.sentence.sourceCollapse", "anomaly.sentence.sourceDrain",
          "anomaly.sentence.sourceShrink")),
        { name, ...levelParams(a) }
      );
    case "source_bytes_growth":
      return fill(t("anomaly.sentence.sourceGrowth"), { name, ...levelParams(a) });
    case "dump_bytes_shrink":
      return fill(
        t(d.collapse || d.drain ? "anomaly.sentence.dumpCollapse" : "anomaly.sentence.dumpShrink"),
        { name: a.name, ...levelParams(a) }
      );
    case "dump_bytes_growth":
      return fill(t("anomaly.sentence.dumpGrowth"), { name: a.name, ...levelParams(a) });
    case "source_files_shrink":
      return fill(
        t(shrinkKey(d, "anomaly.sentence.filesCollapse", "anomaly.sentence.filesCollapse",
          "anomaly.sentence.filesShrink")),
        { name, ...countParams(a) }
      );
    case "duration_slower":
      return fill(t("anomaly.sentence.duration"), { name, ...durationParams(a) });
    case "dump_duration_slower":
      return fill(t("anomaly.sentence.dumpDuration"), { name: a.name, ...durationParams(a) });
    case "failure_streak":
      return fill(t("anomaly.sentence.failureStreak"), { name, count: count(a.observed) });
    case "dump_failure_streak":
      return fill(t("anomaly.sentence.dumpFailureStreak"), {
        name: a.name,
        count: count(a.observed),
      });
    case "flaky":
    case "dump_flaky": {
      const key = a.metric === "flaky" ? "anomaly.sentence.flaky" : "anomaly.sentence.dumpFlaky";
      return fill(t(key), {
        name: a.metric === "flaky" ? name : a.name,
        failed: count(numberOf(d, "failed")),
        total: count(numberOf(d, "total")),
      });
    }
    case "drill_subset":
      return a.targetName
        ? fill(t("anomaly.sentence.drillTarget"), {
            domain: anomalyDomainsLabel(a.domain, t),
            target: a.targetName,
          })
        : fill(t("anomaly.sentence.drill"), {
            domain: anomalyDomainsLabel(a.domain, t),
            source: t(d.source === "offsite" ? "source.offsite" : "source.local"),
          });
    case "drill_dr":
      return a.targetName
        ? fill(t("anomaly.sentence.drillDrTarget"), {
            domain: anomalyDomainsLabel(a.domain, t),
            target: a.targetName,
          })
        : fill(t("anomaly.sentence.drillDr"), { domain: anomalyDomainsLabel(a.domain, t) });
    case "capacity_eta":
      return fill(t("anomaly.sentence.capacityEta"), {
        domains: anomalyDomainsLabel(a.domain, t),
        time: anomalyTimeSpan(a.observed, t, locale),
      });
    case "capacity_low":
      return fill(t("anomaly.sentence.capacityLow"), {
        domains: anomalyDomainsLabel(a.domain, t),
        free: bytes(numberOf(d, "freeBytes")),
        percent: count(a.observed * 100),
      });
    default:
      return fill(t("anomaly.sentence.unknown"), { name });
  }
}

/**
 * anomalyShortLine is a finding as one line under its item's heading: the
 * same facts as anomalySentence without the name the heading already shows. A
 * dump or a dataset is named by `series`, which the line shows in front.
 */
export function anomalyShortLine(
  a: AnomalyView,
  t: TranslateAnomaly,
  locale = "en"
): { series: string; text: string } {
  const d = a.details;
  const series = a.scopeKind === "dump" ? t("anomaly.items.dumpSeries") : a.scopeKind === "zfsds" ? a.part : "";
  const line = (key: TranslationKey, params: Record<string, string> = {}) => ({ series, text: fill(t(key), params) });

  switch (a.metric) {
    case "new_data":
      return line("anomaly.short.newData", { bytes: bytes(a.observed), typical: bytes(numberOf(d, "refBytes")) });
    case "new_data_rewrite":
      return line(d.allFilesNew ? "anomaly.short.newDataRenamed" : "anomaly.short.newDataRewrite", {
        bytes: bytes(a.observed),
        source: bytes(numberOf(d, "sourceBytes")),
      });
    case "new_data_full":
      return line("anomaly.short.newDataFull", { bytes: bytes(a.observed) });
    case "source_bytes_shrink":
      return line(
        shrinkKey(d, "anomaly.short.sourceCollapse", "anomaly.short.sourceDrain", "anomaly.short.shrink"),
        levelParams(a)
      );
    case "dump_bytes_shrink":
      return line(d.collapse || d.drain ? "anomaly.short.sourceCollapse" : "anomaly.short.shrink", levelParams(a));
    case "source_bytes_growth":
    case "dump_bytes_growth":
      return line("anomaly.short.growth", levelParams(a));
    case "source_files_shrink":
      return line(d.collapse || d.drain ? "anomaly.short.filesCollapse" : "anomaly.short.filesShrink", countParams(a));
    case "duration_slower":
    case "dump_duration_slower":
      return line("anomaly.short.duration", durationParams(a));
    case "failure_streak":
    case "dump_failure_streak":
      return line("anomaly.short.failureStreak", { count: count(a.observed) });
    case "flaky":
    case "dump_flaky":
      return line("anomaly.short.flaky", {
        failed: count(numberOf(d, "failed")),
        total: count(numberOf(d, "total")),
      });
    case "drill_subset":
      return line("anomaly.short.drill");
    case "drill_dr":
      return line("anomaly.short.drillDr");
    case "capacity_eta":
      return line("anomaly.short.capacityEta", { time: anomalyTimeSpan(a.observed, t, locale) });
    case "capacity_low":
      return line("anomaly.short.capacityLow", {
        free: bytes(numberOf(d, "freeBytes")),
        percent: count(a.observed * 100),
      });
    default:
      return line("anomaly.short.unknown");
  }
}

/** The item's own page, for the links out of a finding. */
const DOMAIN_PATH: Record<string, string> = {
  container: "/containers",
  containers: "/containers",
  vm: "/vms",
  vms: "/vms",
  files: "/files",
  zfs: "/zfs",
  flash: "/flash",
  config: "/config",
};

export function anomalyItemPath(a: AnomalyView): string | undefined {
  return DOMAIN_PATH[a.domain];
}

/**
 * anomalyRestorePath opens the item's page on the last good backup of a
 * finding about lost data. The item names the row on a page that lists many;
 * flash and config have no name and need none.
 */
export function anomalyRestorePath(a: AnomalyView): string | null {
  const itemPath = DOMAIN_PATH[a.domain];
  if (!itemPath || !a.lastGood) return null;
  const params = new URLSearchParams({ restore: a.lastGood.snapshotId, at: String(a.lastGood.at) });
  if (a.name) params.set("item", a.name);
  if (a.scopeKind === "zfsds") params.set("dataset", a.part);
  if (a.scopeKind === "dump") params.set("dump", "1");
  return `${itemPath}?${params.toString()}`;
}

const DRILL_METRICS = new Set(["drill_subset", "drill_dr"]);
const DURATION_METRICS = new Set(["duration_slower", "dump_duration_slower"]);
const COUNT_METRICS = new Set(["source_files_shrink", "failure_streak", "dump_failure_streak", "flaky", "dump_flaky"]);

// The statistics the detector stored, in the order they explain the finding.
const DETAIL_LABEL: [string, TranslationKey][] = [
  ["median", "anomaly.detail.median"],
  ["mad", "anomaly.detail.mad"],
  ["z", "anomaly.detail.z"],
  ["refBytes", "anomaly.detail.refBytes"],
  ["refRate", "anomaly.detail.refRate"],
  ["etaGrowthDays", "anomaly.detail.etaGrowth"],
  ["etaFreeDays", "anomaly.detail.etaFree"],
  ["slopePerDay", "anomaly.detail.slope"],
];

/** A metric's numbers in the unit the detector measured them in. */
function metricValue(a: AnomalyView, value: number, t: TranslateAnomaly, locale: string): string {
  if (a.metric === "capacity_eta") return anomalyTimeSpan(value, t, locale);
  if (DURATION_METRICS.has(a.metric)) return isolateLtr(formatMillis(value));
  if (COUNT_METRICS.has(a.metric)) return count(value);
  if (a.metric === "capacity_low") return isolateLtr(`${Math.round(value * 100)}%`);
  return bytes(value);
}

export type AnomalyFigure = [label: string, value: string];

/**
 * anomalyFigures splits a finding's numbers into the few its opened line
 * shows and the rest, which only someone checking the detector's arithmetic
 * wants to read.
 */
export function anomalyFigures(
  a: AnomalyView,
  t: TranslateAnomaly,
  locale: string,
  formatTime: (unix: number) => string
): { main: AnomalyFigure[]; more: AnomalyFigure[] } {
  const main: AnomalyFigure[] = [];
  const more: AnomalyFigure[] = [];
  if (DRILL_METRICS.has(a.metric)) {
    main.push([t("anomaly.detail.checksCompared"), count(a.samples)]);
  } else {
    main.push([t("anomaly.detail.observed"), metricValue(a, a.observed, t, locale)]);
    if (a.expected > 0) main.push([t("anomaly.detail.expected"), metricValue(a, a.expected, t, locale)]);
    if (a.threshold > 0) more.push([t("anomaly.detail.threshold"), metricValue(a, a.threshold, t, locale)]);
    more.push([
      t(a.scopeKind === "volume" ? "anomaly.detail.samplesDisk" : "anomaly.detail.samples"),
      count(a.samples),
    ]);
  }
  if (a.firstRunAt) {
    main.push([t("anomaly.detail.firstBackup"), formatTime(a.firstRunAt)]);
    more.push([t("anomaly.detail.firstSeen"), formatTime(a.firstSeenAt)]);
  } else {
    main.push([t("anomaly.detail.firstSeen"), formatTime(a.firstSeenAt)]);
  }
  const sensitivity = ANOMALY_SENSITIVITY_LABEL[a.sensitivity];
  if (sensitivity) more.push([t("anomaly.detail.sensitivity"), t(sensitivity)]);
  more.push([t("anomaly.detail.lastSeen"), formatTime(a.lastSeenAt)]);
  if (a.occurrences > 1) more.push([t("anomaly.detail.occurrences"), count(a.occurrences)]);
  for (const [key, labelKey] of DETAIL_LABEL) {
    const value = a.details[key];
    if (typeof value !== "number") continue;
    if (key === "z") more.push([t(labelKey), value.toFixed(1)]);
    // The detector measures a rate per second; an hour is the span a reader
    // can picture for a backup.
    else if (key === "refRate") more.push([t(labelKey), bytes(value * 3600)]);
    else if (key === "refBytes" || key === "slopePerDay") more.push([t(labelKey), bytes(value)]);
    else more.push([t(labelKey), metricValue(a, value, t, locale)]);
  }
  return { main, more };
}

/** The open findings one card shows: an item with its dump and datasets, one
 *  restore check series, or one disk. */
export interface AnomalyGroup {
  key: string;
  findings: AnomalyView[];
  worst: AnomalySeverity;
}

/** A finding of an item, its dump or its datasets carries the item's target
 *  id; a restore check or a disk has none and is a series of its own. */
export function anomalyGroupKey(a: AnomalyView): string {
  return a.targetId || `${a.scopeKind}:${a.scopeId}`;
}

/** Cards in the order a reader should get to them: the worst first, and among
 *  equals the one with the newest finding. */
export function groupAnomalies(list: AnomalyView[]): AnomalyGroup[] {
  const byKey = new Map<string, AnomalyView[]>();
  for (const a of sortOpenAnomalies(list)) {
    const key = anomalyGroupKey(a);
    byKey.set(key, [...(byKey.get(key) ?? []), a]);
  }
  const newest = (g: AnomalyGroup) => Math.max(...g.findings.map((a) => a.lastSeenAt));
  return [...byKey.entries()]
    .map(([key, findings]) => ({ key, findings, worst: findings[0].severity }))
    .sort((a, b) => SEVERITY_RANK[a.worst] - SEVERITY_RANK[b.worst] || newest(b) - newest(a));
}
