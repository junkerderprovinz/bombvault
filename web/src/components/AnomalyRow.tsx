// One finding, wherever it is shown: compact on the dashboard card, full on
// the Anomalies page. Both modes lead with the sentence, because a severity
// chip alone says that something is wrong without saying what.

import { useState } from "react";
import { Link } from "react-router-dom";
import { Badge } from "./Badge";
import { Button } from "./Button";
import { IconDisclosure } from "./IconDisclosure";
import { InfoBubble } from "./InfoBubble";
import type { AnomalyView } from "../lib/api";
import {
  ANOMALY_SENSITIVITY_LABEL,
  ANOMALY_SEVERITY_LABEL,
  ANOMALY_STATE_LABEL,
  anomalyErrorText,
  anomalyItemLabel,
  anomalySentence,
  anomalySeverityTone,
  anomalyTimeSpan,
  type TranslateAnomaly,
} from "../lib/anomalies";
import { humanBytes } from "../lib/forecast";
import { useT, type TranslationKey } from "../lib/i18n";
import { isolateLtr } from "../lib/ltrFragments";
import { formatMillis, formatTs, relativeTime } from "../lib/reltime";
import { useConfirm } from "../lib/useConfirm";
import { useToast } from "../lib/toast";

/** What an action answered: the coded refusal decides the message. */
export type AnomalyActionResult = { ok: boolean; code?: string };
export type AnomalyAction = (a: AnomalyView) => Promise<AnomalyActionResult>;

const DRILL_METRICS = new Set(["drill_subset", "drill_dr"]);
const DURATION_METRICS = new Set(["duration_slower", "dump_duration_slower"]);
const COUNT_METRICS = new Set([
  "source_files_shrink",
  "failure_streak",
  "dump_failure_streak",
  "flaky",
  "dump_flaky",
]);

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

/** The item's own page, for the link out of a finding. */
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

/** A metric's numbers in the unit the detector measured them in. Figures are
 *  isolated, so right-to-left prose keeps "4.0 GB" in one piece. */
function metricValue(a: AnomalyView, value: number, locale: string, t: TranslateAnomaly): string {
  if (a.metric === "capacity_eta") return anomalyTimeSpan(value, t, locale);
  if (DURATION_METRICS.has(a.metric)) return isolateLtr(formatMillis(value));
  if (COUNT_METRICS.has(a.metric)) return isolateLtr(Math.round(value).toLocaleString());
  if (a.metric === "capacity_low") return isolateLtr(`${Math.round(value * 100)}%`);
  return isolateLtr(humanBytes(value));
}

function when(unix: number): string {
  return isolateLtr(formatTs(unix));
}

export function AnomalyRow({
  a,
  t,
  compact = false,
  selectable = false,
  selected = false,
  onSelect,
  onAcknowledge,
  onExpected,
}: {
  a: AnomalyView;
  t: TranslateAnomaly;
  /** The dashboard card's mode: the sentence, the flags, the two actions. */
  compact?: boolean;
  selectable?: boolean;
  selected?: boolean;
  onSelect?: (id: string, selected: boolean) => void;
  onAcknowledge?: AnomalyAction;
  onExpected?: AnomalyAction;
}) {
  const { lang } = useT();
  const { confirm, confirmDialog } = useConfirm();
  const { push } = useToast();
  const [busy, setBusy] = useState(false);
  const [open, setOpen] = useState(false);

  const label = anomalyItemLabel(a, t);
  const isOpen = a.state === "open";
  const settled = a.state === "acknowledged" || a.state === "expected";

  async function run(action: AnomalyAction, confirmKey: TranslationKey) {
    if (a.retentionHeld) {
      const message = t("anomaly.releaseConfirm").replace("{name}", label);
      if (!(await confirm(message, { confirmKey }))) return;
    }
    setBusy(true);
    try {
      const res = await action(a);
      if (!res.ok) push(anomalyErrorText(res.code, t), "fail");
    } catch {
      push(anomalyErrorText(undefined, t), "fail");
    } finally {
      setBusy(false);
    }
  }

  const itemPath = DOMAIN_PATH[a.domain];
  let restorePath: string | null = null;
  if (itemPath && a.lastGood) {
    // The item names the row on a page that lists many; flash and config
    // have no name and need none.
    const params = new URLSearchParams({ restore: a.lastGood.snapshotId, at: String(a.lastGood.at) });
    if (a.name) params.set("item", a.name);
    if (a.scopeKind === "zfsds") params.set("dataset", a.part);
    if (a.scopeKind === "dump") params.set("dump", "1");
    restorePath = `${itemPath}?${params.toString()}`;
  }

  const details: [string, string][] = [];
  if (DRILL_METRICS.has(a.metric)) {
    details.push([t("anomaly.detail.checksCompared"), String(a.samples)]);
  } else {
    details.push([t("anomaly.detail.observed"), metricValue(a, a.observed, lang, t)]);
    if (a.expected > 0) details.push([t("anomaly.detail.expected"), metricValue(a, a.expected, lang, t)]);
    if (a.threshold > 0) {
      details.push([t("anomaly.detail.threshold"), metricValue(a, a.threshold, lang, t)]);
    }
    details.push([
      t(a.scopeKind === "volume" ? "anomaly.detail.samplesDisk" : "anomaly.detail.samples"),
      String(a.samples),
    ]);
  }
  const sensitivity = ANOMALY_SENSITIVITY_LABEL[a.sensitivity];
  if (sensitivity) details.push([t("anomaly.detail.sensitivity"), t(sensitivity)]);
  details.push([t("anomaly.detail.firstSeen"), when(a.firstSeenAt)]);
  details.push([t("anomaly.detail.lastSeen"), when(a.lastSeenAt)]);
  for (const [key, labelKey] of DETAIL_LABEL) {
    const value = a.details[key];
    if (typeof value !== "number") continue;
    if (key === "z") details.push([t(labelKey), value.toFixed(1)]);
    // The detector measures a rate per second; an hour is the span a reader
    // can picture for a backup.
    else if (key === "refRate") details.push([t(labelKey), isolateLtr(humanBytes(value * 3600))]);
    else if (key === "refBytes" || key === "slopePerDay") details.push([t(labelKey), isolateLtr(humanBytes(value))]);
    else details.push([t(labelKey), metricValue(a, value, lang, t)]);
  }

  return (
    <div className="flex items-start gap-2">
      {selectable && (
        <input
          type="checkbox"
          checked={selected}
          disabled={!isOpen}
          onChange={(e) => onSelect?.(a.id, e.target.checked)}
          aria-label={t("common.selectItem").replace("{name}", label)}
          className="mt-1 h-4 w-4 shrink-0 cursor-pointer"
          style={{ accentColor: "var(--accent)" }}
        />
      )}
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
          <Badge tone={anomalySeverityTone(a.severity)} size="small">
            {t(ANOMALY_SEVERITY_LABEL[a.severity])}
          </Badge>
          <span className="min-w-0 text-sm text-carbon-text">{anomalySentence(a, t, lang)}</span>
          <span className="text-xs text-carbon-textSub">{relativeTime(t, a.lastSeenAt)}</span>
          {a.occurrences > 1 && (
            <span className="text-xs text-carbon-textSub">
              {t("anomaly.occurrences", a.occurrences)}
            </span>
          )}
          {a.severity === "critical" && a.recoveredAt > 0 && (
            <span className="inline-flex items-center gap-1">
              <Badge tone="muted" size="small">{t("anomaly.recovered")}</Badge>
              <InfoBubble tip={t("anomaly.recoveredHint").replace("{date}", when(a.recoveredAt))} />
            </span>
          )}
        </div>

        {a.retentionHeld && <p className="text-xs text-statusWarn">{t("anomaly.retentionPaused")}</p>}

        {a.lastGood && (
          <p className="flex flex-wrap items-baseline gap-x-2 text-xs text-carbon-textSub">
            <span>{t("anomaly.lastGood").replace("{date}", when(a.lastGood.at))}</span>
            {restorePath && (
              <Link to={restorePath} className="text-accentText hover:underline">
                {t("anomaly.action.restoreLastGood").replace("{date}", when(a.lastGood.at))}
              </Link>
            )}
          </p>
        )}

        {!isOpen && (
          <p className="flex flex-wrap items-baseline gap-x-2 text-xs text-carbon-textSub">
            <span>{t(ANOMALY_STATE_LABEL[a.state])}</span>
            {settled && a.ackedAt > 0 && (
              <span>{t("anomaly.closedAt").replace("{date}", when(a.ackedAt))}</span>
            )}
            {settled && a.ackNote && (
              <span>
                {t("anomaly.noteLabel")}: {a.ackNote}
              </span>
            )}
            {settled && a.stillPresent && (
              <span className="text-statusWarn">{t("anomaly.stillPresent")}</span>
            )}
          </p>
        )}

        {isOpen && (onAcknowledge || onExpected) && (
          <div className="flex flex-wrap items-center gap-2">
            {onAcknowledge && (
              <span className="inline-flex items-center gap-1">
                <Button
                  label={t("anomaly.action.acknowledge")}
                  labelKey="anomaly.action.acknowledge"
                  onClick={() => void run(onAcknowledge, "anomaly.action.acknowledge")}
                  disabled={busy}
                />
                <InfoBubble tip={t("anomaly.acknowledgeHint")} />
              </span>
            )}
            {onExpected && a.expectable && (
              <span className="inline-flex items-center gap-1">
                <Button
                  label={t("anomaly.action.expected")}
                  labelKey="anomaly.action.expected"
                  onClick={() => void run(onExpected, "anomaly.action.expected")}
                  disabled={busy}
                />
                <InfoBubble tip={t("anomaly.expectedHint")} />
              </span>
            )}
          </div>
        )}

        {!compact && (
          <div className="flex flex-col gap-1">
            <button
              type="button"
              onClick={() => setOpen(!open)}
              aria-expanded={open}
              className="flex w-fit items-center gap-1 text-xs text-carbon-textSub hover:text-carbon-text"
            >
              <IconDisclosure open={open} />
              {t("anomaly.action.details")}
            </button>
            {open && (
              <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-0.5 text-xs">
                {details.map(([term, value]) => (
                  <div key={term} className="contents">
                    <dt className="text-carbon-textSub">{term}</dt>
                    <dd className="text-carbon-text">{value}</dd>
                  </div>
                ))}
                {itemPath && a.targetId && (
                  <div className="contents">
                    <dt />
                    <dd>
                      <Link to={itemPath} className="text-accentText hover:underline">
                        {t("anomaly.openItem")}
                      </Link>
                    </dd>
                  </div>
                )}
              </dl>
            )}
          </div>
        )}
      </div>
      {confirmDialog}
    </div>
  );
}
