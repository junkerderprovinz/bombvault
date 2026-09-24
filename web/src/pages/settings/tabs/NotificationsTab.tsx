import { CadenceBuilder } from "../../../components/CadenceBuilder";
import { ScheduleRow } from "../../../components/ScheduleBadge";
import { NotifyCard } from "../NotifyCard";
import { Card, ToggleRow } from "../shared";
import type { SettingsTabProps } from "./types";

export function NotificationsTab({
  t,
  advanced,
  settings,
  setSettings,
  platformKind,
  setDigestSaveState,
  setDigestSaveError,
  setWatchdogSaveState,
  setWatchdogSaveError,
  fieldPulse,
  save,
  fieldBusy,
  fieldShake,
  autoSaveToggle,
  debouncedSave,
}: SettingsTabProps) {
  let hueSeq = 0;
  const nextHue = () => hueSeq++;

  return (
    <>
      {/* ------------------------------------------------------------------ */}
      {/* NOTIFICATIONS: NotifyCard now renders THREE Cards internally: its   */}
      {/* settings Card (always), its channels Card (advanced only), and its  */}
      {/* Healthchecks Card (advanced only, card-split follow-up), see        */}
      {/* NotifyCard's own header comment. `channelsHueIndex`/                */}
      {/* `healthchecksHueIndex` MUST each be their own `nextHue()` call made  */}
      {/* INSIDE `tab === "notifications" && advanced &&`, not an eager call   */}
      {/* at the unconditional site above: both Cards only paint while         */}
      {/* Advanced is on, and this file already has one documented            */}
      {/* live-Playwright-caught bug from doing it the eager way, see the      */}
      {/* SYSTEM tab's Spike Card comment ("fires every render regardless")    */}
      {/* for the exact silent-hue-shift failure mode this avoids: a slot      */}
      {/* burned on a Card that never painted, shifting every later heading    */}
      {/* on this tab by one position while Advanced was off. Plain `&&`       */}
      {/* short-circuits correctly, so with Advanced off both values are       */}
      {/* simply never computed and the props below evaluate to `undefined`,   */}
      {/* NotifyCard never renders either Card in that case anyway, so the     */}
      {/* unused values never matter, and no hue slot is spent.                */}
      {/* ------------------------------------------------------------------ */}
      {(() => {
        const settingsHue = nextHue();
        const channelsHue = advanced ? nextHue() : undefined;
        const healthchecksHue = advanced ? nextHue() : undefined;
        return (
          <NotifyCard
            t={t}
            platformKind={platformKind}
            hueIndex={settingsHue}
            channelsHueIndex={channelsHue}
            healthchecksHueIndex={healthchecksHue}
          />
        );
      })()}

      {/* NOTIFICATIONS: Weekly digest: one summary message per week through
          the channels configured above. Schedule input mirrors the drills/
          tamper cadence editors (CadenceBuilder's own <fieldset disabled>
          handles the dimming, no opacity gate on the wrapping container).
            IIFE for the same reason as the tamper-test schedule Card above
          (Task 3): `hueIdx` is captured once and handed to BOTH this Card's
          own heading notch and the CadenceBuilder's TimePicker inside it,
          instead of two independent `nextHue()` calls landing on different
          colours for one visually-grouped Card. */}
      {(() => {
        const hueIdx = nextHue();
        return (
          <Card title={t("settings.digestTitle")} hint={t("settings.digestHint")} hueIndex={hueIdx}>
            {/* Full-page Speichern-Button sweep: this Card's own bottom
                SaveBar is gone, the toggle auto-saves immediately (revert +
                shake on failure, via the page-wide autoSaveToggle), the
                cadence debounces (via debouncedSave), same split every other
                toggle+cadence pairing on this page already uses.
                  No-empty-toggles audit (jdp: "Wochenbericht-Toggle mit Text
                'Wochenbericht' hinschreiben. Es soll nie 'leere' Toggles
                geben."): this row used to `hideLabel` on the Card-title-
                already-says-it reasoning, the third time that exact pattern
                got built in this file after jdp reversed it twice before
                (Rainbow master toggle, Restore-Prüfungen toggle). `hideLabel`
                is gone from ToggleRow entirely now (see its own header
                comment), the row's own label is always visible. */}
            <ToggleRow
              label={t("settings.digestToggle")}
              checked={settings.digestEnabled}
              onChange={(v) => void autoSaveToggle("digestEnabled", v, setDigestSaveState, setDigestSaveError)}
              disabled={fieldBusy.digestEnabled}
              shakeNonce={fieldShake.digestEnabled}
              pulseNonce={fieldPulse.digestEnabled}
            />
            {/* Resolved-schedule badge, NEW this round, same reason as
                RestoreChecksSection's (see that call site's own comment):
                this was the second of the three cadence editors that had no
                badge above them and relied on CadenceBuilder's own inline
                preview, now removed. `enabled` wired to `digestEnabled` for
                the same reason, the on/off is a separate toggle here, not
                the cadence string's own "off" mode. */}
            <ScheduleRow schedule={settings.digestSchedule} enabled={settings.digestEnabled} />
            {/* The editor goes with the toggle above, exactly as in
                RestoreChecksSection (GlimStone 1.10.0): an editor greyed
                because a switch ELSEWHERE is off offers an edit nobody can
                make. The badge above stays either way, so switching the
                report off still shows what would have run. */}
            {settings.digestEnabled && (
            <div className="rounded-card bg-carbon-surface2 p-4">
              <CadenceBuilder
                label={t("settings.schedule")}
                value={settings.digestSchedule}
                onChange={(v) => {
                  setSettings((prev) => (prev ? { ...prev, digestSchedule: v } : prev));
                  debouncedSave("digestSchedule", () =>
                    void save({ digestSchedule: v }, setDigestSaveState, setDigestSaveError)
                  );
                }}
                hueIndex={hueIdx}
              />
            </div>
            )}
          </Card>
        );
      })()}

      {/* NOTIFICATIONS: Overdue-backup watchdog: a fixed daily check (09:00)
          that pushes ONE notification per overdue episode through the channels
          configured above; a new successful backup re-arms it. */}
      <Card title={t("settings.watchdogTitle")} hint={t("settings.watchdogHint")} hueIndex={nextHue()}>
        {/* Full-page Speichern-Button sweep: was this Card's own bottom
            SaveBar, a single toggle, so it now just auto-saves itself. */}
        <ToggleRow
          label={t("settings.watchdogToggle")}
          checked={settings.watchdogEnabled}
          onChange={(v) => void autoSaveToggle("watchdogEnabled", v, setWatchdogSaveState, setWatchdogSaveError)}
          disabled={fieldBusy.watchdogEnabled}
          shakeNonce={fieldShake.watchdogEnabled}
          pulseNonce={fieldPulse.watchdogEnabled}
        />
      </Card>
    </>
  );
}
