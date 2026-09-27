import { RestoreChecksSection } from "../RestoreChecksSection";
import { CadenceBuilder } from "../../../components/CadenceBuilder";
import { ScheduleRow } from "../../../components/ScheduleBadge";
import { Card } from "../shared";
import { IntegrityCard } from "../IntegrityCard";
import type { SettingsTabProps } from "./types";

export function IntegrityTab({
  t,
  settings,
  setSettings,
  schedFieldBusy,
  schedFieldShake,
  fieldPulse,
  save,
  scheduleField,
  scheduleUpdate,
}: SettingsTabProps) {
  // Mirrors immutableOffsiteDomains in internal/schedule: the tamper test is
  // only scheduled when some domain has an off-site repo flagged immutable,
  // otherwise its cadence never runs.
  const tamperScheduleActive =
    (settings.containersOffsite !== "" && settings.containersOffsiteImmutable) ||
    (settings.vmsOffsite !== "" && settings.vmsOffsiteImmutable) ||
    (settings.flashOffsite !== "" && settings.flashOffsiteImmutable) ||
    (settings.configOffsite !== "" && settings.configOffsiteImmutable) ||
    (settings.filesOffsite !== "" && settings.filesOffsiteImmutable);

  let hueSeq = 0;
  const nextHue = () => hueSeq++;

  return (
    <>
      {/* Not behind Advanced: manual restore drills, the real off-site
          restore included, are part of the core ransomware protection. */}
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

      {/* The scheduled off-site tamper test. One hue slot feeds both the
          heading and the builder's time picker; a second nextHue() call
          would give one card two colours. */}
      {(() => {
        const hueIdx = nextHue();
        return (
          <Card title={t("settings.schedulesChecks")} hueIndex={hueIdx}>
            {/* No `enabled` here: the cadence's own "off" mode is the switch.
                A missing immutable domain is not folded into the badge,
                because it would contradict the cadence visible in the editor;
                the warning below explains it instead. */}
            <ScheduleRow schedule={settings.tamperTestSchedule} />
            <div className="rounded-card bg-carbon-surface2 p-4">
              <CadenceBuilder
                label={t("settings.tamperTestSchedule")}
                value={settings.tamperTestSchedule}
                onChange={(v) => scheduleField("tamperTestSchedule", v)}
                hueIndex={hueIdx}
              />
              {/* The only place that says why a set cadence never runs. */}
              {!tamperScheduleActive && (
                <div className="mt-3 rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
                  {t("settings.tamperScheduleInactive")}
                </div>
              )}
            </div>
          </Card>
        );
      })()}
    </>
  );
}
