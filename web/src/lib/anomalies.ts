// The wording of a finding. The backend sends a metric, a few numbers and the
// ids behind them; which sentence they make, in which language and with which
// units, is decided here and nowhere else, so the same finding reads the same
// on the dashboard card, on the page and in a badge's bubble.
//
// Framework-free like forecast.ts: the translator is injected, so the tables
// are the only thing a test has to stand up.

import type {
  AnomalyDetector,
  AnomalyItem,
  AnomalySeverity,
  AnomalyState,
  AnomalyView,
} from "./api";
import type { TranslationKey } from "./i18n";
import type { BadgeTone } from "../components/Badge";
import { dbDumpNameOf, isDbDumpIdentity } from "./dbdump";
import { humanBytes } from "./forecast";
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

export const ANOMALY_DETECTOR_LABEL: Record<AnomalyDetector, TranslationKey> = {
  new_data: "anomaly.detector.newData",
  source: "anomaly.detector.source",
  duration: "anomaly.detector.duration",
  reliability: "anomaly.detector.reliability",
  integrity: "anomaly.detector.integrity",
  capacity: "anomaly.detector.capacity",
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

export function anomalyDetectorLabel(detector: AnomalyDetector, t: TranslateAnomaly): string {
  const key = ANOMALY_DETECTOR_LABEL[detector];
  return key ? t(key) : detector;
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

function levelParams(a: AnomalyView): Record<string, string> {
  return { current: humanBytes(a.observed), typical: humanBytes(a.expected) };
}

function countParams(a: AnomalyView): Record<string, string> {
  return {
    current: Math.round(a.observed).toLocaleString(),
    typical: Math.round(a.expected).toLocaleString(),
  };
}

function durationParams(a: AnomalyView): Record<string, string> {
  return {
    current: formatMillis(a.observed),
    typical: formatMillis(a.expected),
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
        bytes: humanBytes(a.observed),
        typical: humanBytes(numberOf(d, "refBytes")),
      });
    case "new_data_rewrite":
      return fill(t(d.allFilesNew ? "anomaly.sentence.newDataRenamed" : "anomaly.sentence.newDataRewrite"), {
        name,
        bytes: humanBytes(a.observed),
        source: humanBytes(numberOf(d, "sourceBytes")),
      });
    case "new_data_full":
      return fill(t("anomaly.sentence.newDataFull"), { name, bytes: humanBytes(a.observed) });
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
      return fill(t("anomaly.sentence.failureStreak"), { name, count: String(Math.round(a.observed)) });
    case "dump_failure_streak":
      return fill(t("anomaly.sentence.dumpFailureStreak"), {
        name: a.name,
        count: String(Math.round(a.observed)),
      });
    case "flaky":
    case "dump_flaky": {
      const key = a.metric === "flaky" ? "anomaly.sentence.flaky" : "anomaly.sentence.dumpFlaky";
      return fill(t(key), {
        name: a.metric === "flaky" ? name : a.name,
        failed: String(numberOf(d, "failed")),
        total: String(numberOf(d, "total")),
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
        free: humanBytes(numberOf(d, "freeBytes")),
        percent: String(Math.round(a.observed * 100)),
      });
    default:
      return fill(t("anomaly.sentence.unknown"), { name });
  }
}
