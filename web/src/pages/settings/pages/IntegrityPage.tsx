import { useState } from "react";
import { RestoreChecksSection } from "../RestoreChecksSection";
import { AnomalyCard } from "../AnomalyCard";
import { useAnomalySummary } from "../../../lib/useAnomalies";
import { useT } from "../../../lib/i18n";
import { hueCounter, type SaveState } from "../shared";
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
    scheduleUpdate,
    schedFieldBusy,
    schedFieldShake,
  } = useSettings();
  const { summary: anomalySummary } = useAnomalySummary();

  // Anomalies card (integrity page): every field saves on its own through
  // autoSaveToggle, which puts the old value back when the save is refused.
  const [, setAnomalySaveState] = useState<SaveState>("idle");
  const [, setAnomalySaveError] = useState<string | null>(null);

  // Mirrors immutableOffsiteDomains in internal/schedule/schedule.go: the
  // scheduler only wires the tamper test when an off-site repo is set and
  // flagged immutable (#109).
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
        tamperActive={tamperScheduleActive}
        busy={schedFieldBusy}
        shake={schedFieldShake}
        pulse={fieldPulse}
        t={t}
        hueIndex={nextHue()}
      />

      {/* The target of /settings/integrity#anomalies; the margin keeps the
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
