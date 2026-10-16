import { useEffect, useState } from "react";
import { listOffsiteTargets, type OffsiteTarget } from "../../../lib/api";
import { tamperPlanned } from "../../../lib/tamperSchedule";
import { subscribeOffsiteTargets } from "../../../lib/useOffsiteTargets";
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
    containers,
    vms,
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

  const [targets, setTargets] = useState<OffsiteTarget[]>([]);
  useEffect(() => {
    const load = () =>
      void listOffsiteTargets()
        .then((r) => setTargets(r.ok ? (r.targets ?? []) : []))
        .catch(() => setTargets([]));
    load();
    return subscribeOffsiteTargets(load);
  }, []);
  const tamperScheduleActive = tamperPlanned(settings, targets);

  const nextHue = hueCounter();

  return (
    <>
      <IntegrityCard t={t} hueIndex={nextHue()} />

      <RestoreChecksSection
        settings={settings}
        update={scheduleUpdate}
        tamperActive={tamperScheduleActive}
        containers={containers}
        vms={vms}
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
