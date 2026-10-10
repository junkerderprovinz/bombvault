import { useState } from "react";
import { CadenceBuilder } from "../../../components/CadenceBuilder";
import { ScheduleRow } from "../../../components/ScheduleBadge";
import { useT } from "../../../lib/i18n";
import { useAdvanced } from "../../../lib/advanced";
import { NotifyCard } from "../NotifyCard";
import { Card, ToggleRow, hueCounter, type SaveState } from "../shared";
import { useSettings } from "../settingsStore";

export function NotificationsPage() {
  const { t } = useT();
  const { advanced } = useAdvanced();
  const {
    settings,
    setSettings,
    platformKind,
    save,
    debouncedSave,
    fieldPulse,
    autoSaveToggle,
    fieldBusy,
    fieldShake,
  } = useSettings();

  // Weekly digest and overdue-backup watchdog on the notifications page.
  const [, setDigestSaveState] = useState<SaveState>("idle");
  const [, setDigestSaveError] = useState<string | null>(null);

  const [, setWatchdogSaveState] = useState<SaveState>("idle");
  const [, setWatchdogSaveError] = useState<string | null>(null);

  const nextHue = hueCounter();

  return (
    <>
      {/* The channel and Healthchecks cards only paint in the advanced view, so */}
      {/* their hue positions are counted inside that condition and none is spent */}
      {/* while it is off. */}
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

      {/* Weekly digest: one summary message per week through the channels
          above. One hueIdx feeds both the heading and the time picker. */}
      {(() => {
        const hueIdx = nextHue();
        return (
          <Card title={t("settings.digestTitle")} hint={t("settings.digestHint")} hueIndex={hueIdx}>
            <ToggleRow
              label={t("settings.digestToggle")}
              checked={settings.digestEnabled}
              onChange={(v) => void autoSaveToggle("digestEnabled", v, setDigestSaveState, setDigestSaveError)}
              disabled={fieldBusy.digestEnabled}
              shakeNonce={fieldShake.digestEnabled}
              pulseNonce={fieldPulse.digestEnabled}
            />
            {/* `enabled` follows digestEnabled: the on/off is a separate
                toggle here, not the cadence string's own "off" mode. */}
            <ScheduleRow schedule={settings.digestSchedule} enabled={settings.digestEnabled} />
            {/* The editor goes with the toggle above, as in
                RestoreChecksSection: an editor greyed because a switch
                elsewhere is off offers an edit nobody can make. The badge
                stays either way, so switching the report off still shows what
                would have run. */}
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

      {/* Overdue-backup watchdog: a fixed daily check at 09:00 that sends one
          notification per overdue episode through the channels above; a new
          successful backup re-arms it. */}
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
