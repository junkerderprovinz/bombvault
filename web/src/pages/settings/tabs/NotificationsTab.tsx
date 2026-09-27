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
      {/* NotifyCard renders the channels and Healthchecks cards only under
          Advanced, so their hue slots are taken only then; otherwise a hidden
          card would shift every later heading on the tab. */}
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

      {/* One hue slot feeds both the heading and the builder's time picker,
          so the card keeps one colour. */}
      {(() => {
        const hueIdx = nextHue();
        return (
          <Card title={t("settings.digestTitle")} hint={t("settings.digestHint")} hueIndex={hueIdx}>
            {/* The toggle saves at once and the cadence is debounced. */}
            <ToggleRow
              label={t("settings.digestToggle")}
              checked={settings.digestEnabled}
              onChange={(v) => void autoSaveToggle("digestEnabled", v, setDigestSaveState, setDigestSaveError)}
              disabled={fieldBusy.digestEnabled}
              shakeNonce={fieldShake.digestEnabled}
              pulseNonce={fieldPulse.digestEnabled}
            />
            {/* `enabled` comes from the separate toggle, not from the
                cadence's own "off" mode. */}
            <ScheduleRow schedule={settings.digestSchedule} enabled={settings.digestEnabled} />
            {/* The editor goes with the toggle, since a greyed editor offers
                an edit nobody can make. The badge stays, so a switched-off
                report still shows what would have run. */}
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

      {/* A fixed daily check at 09:00 that sends one notification per
          overdue episode; the next successful backup re-arms it. */}
      <Card title={t("settings.watchdogTitle")} hint={t("settings.watchdogHint")} hueIndex={nextHue()}>
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
