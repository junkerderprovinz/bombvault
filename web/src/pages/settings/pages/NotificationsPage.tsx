import { useState } from "react";
import { PlanButton } from "../../../components/PlanButton";
import { SettingRow } from "../../../components/storage/rows";
import type { Settings } from "../../../lib/api";
import type { EntryKind } from "../../../lib/backupEntry";
import { useT } from "../../../lib/i18n";
import { NotifyCard } from "../NotifyCard";
import { ToggleRow, hueCounter, type SaveState } from "../shared";
import { useSettings } from "../settingsStore";

const SECTION_SWITCH: [EntryKind, keyof Settings][] = [
  ["container", "containersEnabled"],
  ["vm", "vmsEnabled"],
  ["flash", "flashEnabled"],
  ["files", "filesEnabled"],
  ["zfs", "zfsEnabled"],
  ["config", "configEnabled"],
];

export function NotificationsPage() {
  const { t } = useT();
  const {
    settings,
    setSettings,
    platformKind,
    platformSet,
    save,
    debouncedSave,
    fieldPulse,
    autoSaveToggle,
    fieldBusy,
    fieldShake,
  } = useSettings();

  const [, setDigestSaveState] = useState<SaveState>("idle");
  const [, setDigestSaveError] = useState<string | null>(null);

  const [, setWatchdogSaveState] = useState<SaveState>("idle");
  const [, setWatchdogSaveError] = useState<string | null>(null);

  // The digest has a switch and a cadence of its own; its plan shows both, with
  // Off for the switch.
  function setDigest(plan: string) {
    const patch: Partial<Settings> =
      plan === "off" ? { digestEnabled: false } : { digestEnabled: true, digestSchedule: plan };
    setSettings((prev) => (prev ? { ...prev, ...patch } : prev));
    debouncedSave("digestSchedule", () => void save(patch, setDigestSaveState, setDigestSaveError));
  }

  const nextHue = hueCounter();
  const firstHue = nextHue();

  return (
    <NotifyCard
      t={t}
      platformKind={platformKind}
      platformSet={platformSet}
      hueIndex={firstHue}
      channelsHueIndex={nextHue()}
      healthchecksHueIndex={nextHue()}
      sections={SECTION_SWITCH.filter(([, on]) => settings[on]).map(([kind]) => kind)}
      tail={
        <SettingRow label={t("settings.digestToggle")} hint={t("settings.digestHint")}>
          <PlanButton
            label={t("settings.digestToggle")}
            value={settings.digestEnabled ? settings.digestSchedule : "off"}
            onChange={setDigest}
            hueIndex={firstHue}
          />
        </SettingRow>
      }
    >
      {/* A fixed daily check at 09:00 that sends one notification per overdue
          episode; a new successful backup re-arms it. */}
      <ToggleRow
        label={t("settings.watchdogToggle")}
        hint={t("settings.watchdogHint")}
        checked={settings.watchdogEnabled}
        onChange={(v) => void autoSaveToggle("watchdogEnabled", v, setWatchdogSaveState, setWatchdogSaveError)}
        disabled={fieldBusy.watchdogEnabled}
        shakeNonce={fieldShake.watchdogEnabled}
        pulseNonce={fieldPulse.watchdogEnabled}
        hueIndex={2}
      />
    </NotifyCard>
  );
}
