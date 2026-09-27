// One finding on the dashboard card. It leads with the sentence, because a
// severity chip alone says that something is wrong without saying what.

import { useState } from "react";
import { Link } from "react-router-dom";
import { Badge } from "./Badge";
import { Button } from "./Button";
import { InfoBubble } from "./InfoBubble";
import type { AnomalyView } from "../lib/api";
import {
  ANOMALY_SEVERITY_LABEL,
  anomalyErrorText,
  anomalyItemLabel,
  anomalyRestorePath,
  anomalySentence,
  anomalySeverityTone,
  type TranslateAnomaly,
} from "../lib/anomalies";
import { useT, type TranslationKey } from "../lib/i18n";
import { isolateLtr } from "../lib/ltrFragments";
import { formatTs, relativeTime } from "../lib/reltime";
import { useConfirm } from "../lib/useConfirm";
import { useToast } from "../lib/toast";

/** What an action answered: the coded refusal decides the message. */
export type AnomalyActionResult = { ok: boolean; code?: string };
export type AnomalyAction = (a: AnomalyView) => Promise<AnomalyActionResult>;

function when(unix: number): string {
  return isolateLtr(formatTs(unix));
}

export function AnomalyRow({
  a,
  t,
  onAcknowledge,
  onExpected,
}: {
  a: AnomalyView;
  t: TranslateAnomaly;
  onAcknowledge?: AnomalyAction;
  onExpected?: AnomalyAction;
}) {
  const { lang } = useT();
  const { confirm, confirmDialog } = useConfirm();
  const { push } = useToast();
  const [busy, setBusy] = useState(false);

  async function run(action: AnomalyAction, confirmKey: TranslationKey) {
    if (a.retentionHeld) {
      const message = t("anomaly.releaseConfirm").replace("{name}", anomalyItemLabel(a, t));
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

  const restorePath = anomalyRestorePath(a);

  return (
    <div className="flex min-w-0 flex-col gap-1">
      <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
        <Badge tone={anomalySeverityTone(a.severity)} size="small">
          {t(ANOMALY_SEVERITY_LABEL[a.severity])}
        </Badge>
        <span className="min-w-0 text-sm text-carbon-text wrap-anywhere">{anomalySentence(a, t, lang)}</span>
        <span className="text-xs text-carbon-textSub">{relativeTime(t, a.lastSeenAt)}</span>
        {a.occurrences > 1 && <span className="text-xs text-carbon-textSub">{t("anomaly.occurrences", a.occurrences)}</span>}
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

      {a.state === "open" && (onAcknowledge || onExpected) && (
        <div className="flex flex-wrap items-center gap-2">
          {onAcknowledge && (
            <Button
              label={t("anomaly.action.acknowledge")}
              labelKey="anomaly.action.acknowledge"
              onClick={() => void run(onAcknowledge, "anomaly.action.acknowledge")}
              disabled={busy}
              hint={t("anomaly.acknowledgeHint")}
              className="glim-btn-wrap"
            />
          )}
          {onExpected && a.expectable && (
            <Button
              label={t("anomaly.action.expected")}
              labelKey="anomaly.action.expected"
              onClick={() => void run(onExpected, "anomaly.action.expected")}
              disabled={busy}
              hint={t("anomaly.expectedHint")}
              className="glim-btn-wrap"
            />
          )}
        </div>
      )}
      {confirmDialog}
    </div>
  );
}
