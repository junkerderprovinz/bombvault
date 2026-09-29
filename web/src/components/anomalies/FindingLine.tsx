// One finding as a line: a dot for its severity, what happened, when, and a
// chevron. Opened, it says the whole sentence, the figures and what can be
// done about it. The card around it names the item, so an open finding's line
// leaves the name out; a closed one stands in a plain list and keeps it.

import { useId, useState } from "react";
import { Link } from "react-router-dom";

import { Badge } from "../Badge";
import { Button } from "../Button";
import { IconDisclosure } from "../IconDisclosure";
import { InfoBubble } from "../InfoBubble";
import {
  ANOMALY_STATE_LABEL,
  anomalyErrorText,
  anomalyFigures,
  anomalyItemLabel,
  anomalyItemPath,
  anomalyRestorePath,
  anomalySentence,
  anomalyShortLine,
  type TranslateAnomaly,
} from "../../lib/anomalies";
import type { AnomalyActionResult, AnomalySeverity, AnomalyView } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { isolateLtr } from "../../lib/ltrFragments";
import { formatTs, relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useRemoteView } from "../../lib/remoteView";

export type FindingAction = (a: AnomalyView) => Promise<AnomalyActionResult>;

const SEVERITY_DOT: Record<AnomalySeverity, string> = {
  critical: "bg-statusFailSolid",
  warning: "bg-statusWarnSolid",
  info: "bg-carbon-textMuted",
};

function when(unix: number): string {
  return isolateLtr(formatTs(unix));
}

export function FindingLine({
  a,
  t,
  open,
  onToggle,
  closed = false,
  restoreIsPrimary = false,
  onAcknowledge,
  onExpected,
}: {
  a: AnomalyView;
  t: TranslateAnomaly;
  open: boolean;
  onToggle: () => void;
  /** A settled or resolved finding in the list below the cards. */
  closed?: boolean;
  /** The card already offers this finding's restore as its main action. */
  restoreIsPrimary?: boolean;
  onAcknowledge?: FindingAction;
  onExpected?: FindingAction;
}) {
  const { lang } = useT();
  const { confirm, confirmDialog } = useConfirm();
  const { push } = useToast();
  const { remote } = useRemoteView();
  const [busy, setBusy] = useState(false);
  const panelId = useId();

  const label = anomalyItemLabel(a, t);
  const line = anomalyShortLine(a, t, lang);
  const figures = anomalyFigures(a, t, lang, when);
  const restorePath = anomalyRestorePath(a);
  const itemPath = anomalyItemPath(a);
  const recovered = a.severity === "critical" && a.recoveredAt > 0;
  const closedAt = a.ackedAt || a.resolvedAt;

  async function run(action: FindingAction, confirmKey: TranslationKey) {
    if (a.retentionHeld) {
      if (!(await confirm(t("anomaly.releaseConfirm").replace("{name}", label), { confirmKey }))) return;
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

  const more = figures.more
    .map(([term, value]) => t("anomaly.figure").replace("{label}", term).replace("{value}", value))
    .join(" · ");

  return (
    <div className="flex flex-col">
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={open}
        aria-controls={open ? panelId : undefined}
        className="-mx-2 flex min-h-11 items-start gap-3 rounded-control px-2 py-2.5 text-start hover:bg-carbon-hover"
      >
        <span aria-hidden="true" className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${SEVERITY_DOT[a.severity]}`} />
        <span className="min-w-0 flex-1 text-sm text-carbon-text wrap-anywhere">
          {closed && <span className="font-medium">{label}: </span>}
          {!closed && line.series && (
            <span
              dir={a.scopeKind === "zfsds" ? "ltr" : undefined}
              className={`text-carbon-textSub${a.scopeKind === "zfsds" ? " font-mono" : ""}`}
            >
              {line.series}:{" "}
            </span>
          )}
          {line.text}
        </span>
        {recovered && !closed && (
          <Badge tone="neutral" size="small" className="mt-0.5 shrink-0">
            {t("anomaly.recovered")}
          </Badge>
        )}
        {closed && (
          <span className="mt-0.5 shrink-0 text-xs text-carbon-textSub">{t(ANOMALY_STATE_LABEL[a.state])}</span>
        )}
        <span className="mt-0.5 shrink-0 text-xs text-carbon-textSub max-sm:hidden">
          {relativeTime(t, closed && closedAt ? closedAt : a.lastSeenAt)}
        </span>
        <span className="mt-1 shrink-0 text-carbon-textSub">
          <IconDisclosure open={open} />
        </span>
      </button>

      {open && (
        <div id={panelId} className="flex flex-col gap-2 pb-3 ps-5 glim-content-fade">
          <p className="text-sm text-carbon-textSub wrap-anywhere">{anomalySentence(a, t, lang)}</p>

          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
            {figures.main.map(([term, value]) => (
              <span key={term} className="inline-flex items-baseline gap-1.5">
                <span className="text-carbon-textSub">{term}</span>
                <span className="font-medium text-carbon-text">{value}</span>
              </span>
            ))}
            {more && <InfoBubble tip={more} />}
          </div>

          {recovered && (
            <p className="inline-flex items-center gap-1 text-xs text-carbon-textSub">
              {t("anomaly.recovered")}
              <InfoBubble tip={t("anomaly.recoveredHint").replace("{date}", when(a.recoveredAt))} />
            </p>
          )}

          {closed && (
            <p className="flex flex-wrap items-baseline gap-x-3 text-xs text-carbon-textSub">
              {closedAt > 0 && <span>{t("anomaly.closedAt").replace("{date}", when(closedAt))}</span>}
              {a.ackNote && (
                <span className="min-w-0 wrap-anywhere">
                  {t("anomaly.noteLabel")}: {a.ackNote}
                </span>
              )}
              {a.stillPresent && <span className="text-statusWarn">{t("anomaly.stillPresent")}</span>}
            </p>
          )}

          {!closed && a.retentionHeld && <RetentionNote t={t} />}

          {!closed && (
            <div className="flex flex-wrap items-center gap-2">
              {/* Marking a finding as expected, acknowledging it and opening
                  its restore are all settled here, not remote view's read of
                  the finding, so all three stay local (decision 1 leaves
                  acknowledging out on purpose; the other two are the same
                  kind of settled-here action). */}
              {!remote && onExpected && a.expectable && (
                <Button
                  label={t("anomaly.action.expected")}
                  labelKey="anomaly.action.expected"
                  onClick={() => void run(onExpected, "anomaly.action.expected")}
                  disabled={busy}
                  hint={t("anomaly.expectedHint")}
                  className="glim-btn-wrap"
                />
              )}
              {!remote && onAcknowledge && (
                <Button
                  label={t("anomaly.action.acknowledge")}
                  labelKey="anomaly.action.acknowledge"
                  onClick={() => void run(onAcknowledge, "anomaly.action.acknowledge")}
                  disabled={busy}
                  hint={t("anomaly.acknowledgeHint")}
                  className="glim-btn-wrap"
                />
              )}
              {!remote && restorePath && a.lastGood && !restoreIsPrimary && (
                <Link to={restorePath} className="text-xs text-accentText hover:underline">
                  {t("anomaly.action.restoreLastGood").replace("{date}", when(a.lastGood.at))}
                </Link>
              )}
              {itemPath && a.targetId && (
                <Link to={itemPath} className="text-xs text-accentText hover:underline">
                  {t("anomaly.openItem")}
                </Link>
              )}
            </div>
          )}
        </div>
      )}
      {confirmDialog}
    </div>
  );
}

/** Old backups of the series stay until the finding is settled. */
export function RetentionNote({ t }: { t: TranslateAnomaly }) {
  return (
    <p className="inline-flex items-center gap-1 self-start text-xs text-statusWarn">
      {t("anomaly.retentionKept")}
      <InfoBubble tip={t("anomaly.retentionPaused")} />
    </p>
  );
}
