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
  // Tamper-test schedule eligibility (#109): mirrors immutableOffsiteDomains in
  // internal/schedule/schedule.go, the scheduler only wires the scheduled
  // tamper-test job when at least one domain's off-site repo is set AND
  // flagged immutable. Without that, the cadence editor below silently never
  // runs (the same per-domain predicate as appendOnlyEligible in IntegrityCard,
  // widened to "any domain including config").
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
      {/* ------------------------------------------------------------------ */}
      {/* INTEGRITY: Integrity, maintenance & restore drills                  */}
      {/* Default-visible (v4): manual restore drills, including the real      */}
      {/* off-site DR restore, are part of the core ransomware-protection      */}
      {/* flow, alongside the un-gated off-site + retention cards above.       */}
      {/* ------------------------------------------------------------------ */}
      {/* IntegrityCard used to be documented here as the ONLY Card this tab
          ever rendered, a genuine singleton per design-language's own
          exclusion ("the only one of its kind on the page keeps the single
          accent"), so it deliberately took no `hueIndex` at all. That
          exemption no longer applies (jdp, live-review: "Gehört die
          'Automatische Restore-Prüfungen' Card nicht in den
          Integritäts-Tab?"): RestoreChecksSection and the schedulesChecks
          Card below moved here from the Schedules tab, both configuring
          WHAT gets verified and how often, a natural fit next to this
          Card's own verify/unlock/prune/drill actions. With three Cards
          now genuinely on this tab, IntegrityCard gets a real `nextHue()`
          call like everything else, first in visual order since it's the
          tab's primary/pre-existing content. */}
      <IntegrityCard t={t} settings={settings} setSettings={setSettings} save={save} hueIndex={nextHue()} />

      {/* Restore-check drills (RestoreChecksSection renders its own Card),
          moved from the Schedules tab (see that tab's own comment at its
          old call site). */}
      <RestoreChecksSection
        settings={settings}
        update={scheduleUpdate}
        busy={schedFieldBusy}
        shake={schedFieldShake}
        pulse={fieldPulse}
        t={t}
        hueIndex={nextHue()}
      />

      {/* Restore-check schedule (schedulesChecks): the scheduled off-site
          append-only tamper test, moved from the Schedules tab (see that
          tab's own comment at its old call site).
            `hueIdx` captured once in this IIFE and reused for both the
          Card's own heading notch and the CadenceBuilder's TimePicker
          inside it (Task 3, jdp: "Der Zeitpicker ist nicht im
          Regenbogenmodus"), a bare inline `<Card hueIndex={nextHue()}>`
          here has no local variable to also hand the CadenceBuilder below,
          and calling `nextHue()` a second time would consume a SECOND,
          different position for one visually-grouped Card (exactly the
          trap SaveBar's own header comment already warns about for the
          identical "one Card, two hue-aware children" shape). The IIFE is
          the smallest change that captures the single call's result
          without lifting this ad-hoc Card block into its own named
          component purely to receive a prop. */}
      {(() => {
        const hueIdx = nextHue();
        return (
          <Card title={t("settings.schedulesChecks")} hueIndex={hueIdx}>
            {/* Resolved-schedule badge, NEW this round, the third and last
                cadence editor that had none (see RestoreChecksSection's own
                comment for why CadenceBuilder's inline preview could only be
                deleted once all three had one). NO `enabled` prop here,
                unlike the other two: this Card has no on/off toggle of its
                own, the cadence string's own "off" mode IS the control, the
                same shape the four domain Cards use. The separate
                `tamperScheduleActive` precondition below is deliberately NOT
                folded into the badge: it isn't this card's own on/off but a
                cross-cutting "no qualifying domain configured" state, and it
                already has its own explicit amber explanation right beneath
                (#109, the one place that told manilx why Sun 08:00 never
                ran). Restating it as a grey "Kein Zeitplan" badge would
                contradict the cadence the user can plainly see set in the
                editor. */}
            <ScheduleRow schedule={settings.tamperTestSchedule} />
            <div className="rounded-card bg-carbon-surface2 p-4">
              <CadenceBuilder
                label={t("settings.tamperTestSchedule")}
                value={settings.tamperTestSchedule}
                onChange={(v) => scheduleField("tamperTestSchedule", v)}
                hueIndex={hueIdx}
              />
              {/* #109: the scheduler stays inert without a qualifying domain, this
                  is the only place that told manilx why Sun 08:00 never ran. */}
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
