import { Link } from "react-router-dom";

import { LabelledSelect } from "../../components/SelectField";
import { ANOMALY_NOTIFY_LABEL, ANOMALY_SENSITIVITY_LABEL } from "../../lib/anomalies";
import type { AnomalySummary, Settings } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { Card, ToggleRow } from "./shared";

type AnomalyFieldKey = "anomalyEnabled" | "anomalyRetentionHold" | "anomalySensitivity" | "anomalyNotifyMin";

/**
 * AnomalyCard is the switch for anomaly detection and its global settings,
 * on the Integrity tab next to the restore checks. Every control saves on its
 * own. Below them the card says what the detection could not see: a history
 * it failed to read, items it could not check, a channel that would swallow
 * every message and disks it has no figure for.
 */
export function AnomalyCard({
  t,
  settings,
  summary,
  save,
  busy,
  shake,
  pulse,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  settings: Settings;
  /** Null until the first summary arrives, or when it could not be read. */
  summary: AnomalySummary | null;
  save: <K extends AnomalyFieldKey>(key: K, next: Settings[K]) => void;
  busy?: Partial<Record<AnomalyFieldKey, boolean>>;
  shake?: Partial<Record<AnomalyFieldKey, number>>;
  pulse?: Partial<Record<AnomalyFieldKey, number>>;
  hueIndex?: number;
}) {
  const on = settings.anomalyEnabled;
  const backfill = summary?.backfill;

  return (
    <Card title={t("anomaly.settings.title")} hint={t("anomaly.settings.hint")} hueIndex={hueIndex}>
      <ToggleRow
        label={t("anomaly.settings.toggle")}
        checked={on}
        onChange={(v) => save("anomalyEnabled", v)}
        disabled={busy?.anomalyEnabled}
        shakeNonce={shake?.anomalyEnabled}
        pulseNonce={pulse?.anomalyEnabled}
      />

      {on && (
        <>
          <div className="flex flex-wrap items-start gap-4">
            <LabelledSelect
              label={t("anomaly.settings.sensitivity")}
              hint={t("anomaly.settings.sensitivityHint")}
              value={settings.anomalySensitivity}
              onChange={(next: string) => save("anomalySensitivity", next)}
              options={Object.entries(ANOMALY_SENSITIVITY_LABEL).map(([value, key]) => ({ value, label: t(key) }))}
            />
            <div className="flex flex-col gap-1">
              <LabelledSelect
                label={t("anomaly.settings.notifyMin")}
                hint={t("anomaly.settings.notifyHint")}
                value={settings.anomalyNotifyMin}
                onChange={(next: string) => save("anomalyNotifyMin", next)}
                options={Object.entries(ANOMALY_NOTIFY_LABEL).map(([value, key]) => ({ value, label: t(key) }))}
              />
              {summary?.notifyMuted && (
                <p className="text-xs text-statusWarn">
                  {t("anomaly.settings.notifyMuted")}{" "}
                  {/* A hash link, because the settings page switches tabs on
                      hashchange and a router navigation does not fire one. */}
                  <a href="#notifications" className="text-accentText hover:underline">
                    {t("anomaly.settings.openNotifications")}
                  </a>
                </p>
              )}
            </div>
          </div>

          <ToggleRow
            label={t("anomaly.settings.holdToggle")}
            hint={t("anomaly.settings.holdHint")}
            checked={settings.anomalyRetentionHold}
            onChange={(v) => save("anomalyRetentionHold", v)}
            disabled={busy?.anomalyRetentionHold}
            shakeNonce={shake?.anomalyRetentionHold}
            pulseNonce={pulse?.anomalyRetentionHold}
          />

          <div className="flex flex-col gap-1 text-xs text-carbon-textSub">
            {backfill && backfill.slots > 0 && (
              <p>
                {t("anomaly.settings.backfill")
                  .replace("{done}", backfill.done.toLocaleString())
                  .replace("{slots}", backfill.slots.toLocaleString())}
              </p>
            )}
            {backfill && backfill.done + backfill.failed < backfill.slots && (
              <p>{t("anomaly.settings.backfillPending")}</p>
            )}
            {backfill && backfill.failed > 0 && (
              <p className="text-statusWarn">
                {t("anomaly.settings.backfillFailed").replace("{failed}", backfill.failed.toLocaleString())}
              </p>
            )}
            {backfill && backfill.withoutSummary > 0 && (
              <p>{t("anomaly.settings.backfillOld").replace("{n}", backfill.withoutSummary.toLocaleString())}</p>
            )}
            {summary && summary.evalErrors > 0 && (
              <p className="text-statusWarn">
                {t("anomaly.settings.evalErrors").replace("{n}", summary.evalErrors.toLocaleString())}
              </p>
            )}
            {summary && summary.unmeasuredVolumes.length > 0 && (
              <p>{t("anomaly.settings.unmeasured").replace("{names}", summary.unmeasuredVolumes.join(", "))}</p>
            )}
          </div>
        </>
      )}

      {/* Stays with the switch off: earlier findings are still on the page. */}
      <Link to="/anomalies" className="w-fit text-sm text-accentText hover:underline">
        {t("anomaly.settings.openPage")}
      </Link>
    </Card>
  );
}
