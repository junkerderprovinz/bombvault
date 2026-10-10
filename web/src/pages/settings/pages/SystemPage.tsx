import { useT } from "../../../lib/i18n";
import { hueCounter } from "../shared";
import { SettingsPortabilityCard } from "../SettingsPortabilityCard";
import { useSettings } from "../settingsStore";

export function SystemPage() {
  const { t } = useT();
  const { applyImportedSettings } = useSettings();

  const nextHue = hueCounter();

  return (
    <>
      {/* Moves this instance's settings and off-site targets, credentials only */}
      {/* when asked, to another install. Backups and history are never touched. */}
      <SettingsPortabilityCard t={t} hueIndex={nextHue()} applyImport={applyImportedSettings} />
    </>
  );
}
