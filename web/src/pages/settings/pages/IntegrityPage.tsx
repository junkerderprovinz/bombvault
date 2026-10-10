import { useState } from "react";
import { RestoreChecksSection } from "../RestoreChecksSection";
import { AnomalyCard } from "../AnomalyCard";
import { useAnomalySummary } from "../../../lib/useAnomalies";
import { CadenceBuilder } from "../../../components/CadenceBuilder";
import { ScheduleRow } from "../../../components/ScheduleBadge";
import { useT } from "../../../lib/i18n";
import { Card, hueCounter, type SaveState } from "../shared";
import { IntegrityCard } from "../IntegrityCard";
import { useSettings } from "../settingsStore";

export function IntegrityPage() {
  const { t } = useT();
  const {
    settings,
    setSettings,
    save,
    fieldPulse,
    autoSaveToggle,
    fieldBusy,
    fieldShake,
    scheduleField,
    scheduleUpdate,
    schedFieldBusy,
    schedFieldShake,
  } = useSettings();
  const { summary: anomalySummary } = useAnomalySummary();

  // Anomalies card (integrity page): every field saves on its own through
  // autoSaveToggle, which puts the old value back when the save is refused.
  const [, setAnomalySaveState] = useState<SaveState>("idle");
  const [, setAnomalySaveError] = useState<string | null>(null);

  // Tamper-test schedule eligibility (#109) mirrors immutableOffsiteDomains in
  // internal/schedule/schedule.go: the scheduler only wires the tamper-test job
  // when at least one domain's off-site repo is set and flagged immutable.
  // Otherwise the cadence editor below would silently never run.
  const tamperScheduleActive =
    (settings.containersOffsite !== "" && settings.containersOffsiteImmutable) ||
    (settings.vmsOffsite !== "" && settings.vmsOffsiteImmutable) ||
    (settings.flashOffsite !== "" && settings.flashOffsiteImmutable) ||
    (settings.configOffsite !== "" && settings.configOffsiteImmutable) ||
    (settings.filesOffsite !== "" && settings.filesOffsiteImmutable) ||
    (settings.zfsOffsite !== "" && settings.zfsOffsiteImmutable);

  const nextHue = hueCounter();

  return (
    <>
      {/* Restore drills, the off-site DR restore among them, are part of the */}
      {/* default view. */}
      <IntegrityCard t={t} settings={settings} setSettings={setSettings} save={save} hueIndex={nextHue()} />

      <RestoreChecksSection
        settings={settings}
        update={scheduleUpdate}
        busy={schedFieldBusy}
        shake={schedFieldShake}
        pulse={fieldPulse}
        t={t}
        hueIndex={nextHue()}
      />

      {/* The scheduled off-site append-only tamper test. One hueIdx feeds
          both the heading and the time picker; a second nextHue() call
          would give the one card two colours. */}
      {(() => {
        const hueIdx = nextHue();
        return (
          <Card title={t("settings.schedulesChecks")} hueIndex={hueIdx}>
            {/* No `enabled` here: the cadence string's own "off" mode is the
                control, as in the domain cards. tamperScheduleActive stays
                out of the badge: it is not this card's on/off, it has its
                own warning below, and a "no schedule" badge would contradict
                the cadence visible in the editor. */}
            <ScheduleRow schedule={settings.tamperTestSchedule} />
            <div className="rounded-card bg-carbon-surface2 p-4">
              <CadenceBuilder
                label={t("settings.tamperTestSchedule")}
                value={settings.tamperTestSchedule}
                onChange={(v) => scheduleField("tamperTestSchedule", v)}
                hueIndex={hueIdx}
              />
              {/* The scheduler stays inert without a qualifying domain, and
                  this is the only place that says why the test never runs
                  (#109). */}
              {!tamperScheduleActive && (
                <div className="mt-3 rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
                  {t("settings.tamperScheduleInactive")}
                </div>
              )}
            </div>
          </Card>
        );
      })()}

      {/* Last, so the restore checks and their schedule stay next to each
          other. The target of /settings/integrity#anomalies; the margin keeps the
          heading badge, which straddles the card's top edge, in view. */}
      <div id="anomalies" className="scroll-mt-6">
        <AnomalyCard
          t={t}
          settings={settings}
          summary={anomalySummary}
          save={(key, next) => void autoSaveToggle(key, next, setAnomalySaveState, setAnomalySaveError)}
          busy={fieldBusy}
          shake={fieldShake}
          pulse={fieldPulse}
          hueIndex={nextHue()}
        />
      </div>
    </>
  );
}
