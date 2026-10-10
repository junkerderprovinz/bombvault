import { useT } from "../../../lib/i18n";
import { Card, hueCounter } from "../shared";
import { FleetSettingsCard } from "../FleetSettingsCard";
import { PairingSection } from "../pairing/PairingSection";
import { useSettings } from "../settingsStore";

export function PairingPage() {
  const { t } = useT();
  const { settings, setSettings, save } = useSettings();

  const pairingUsed = settings.receiverEnabled || settings.fleetEnabled || settings.pullEnabled;

  const nextHue = hueCounter();

  return (
    <>
      {/* Pairing only serves the domains that work over the group, so until
          one of them is on the page says where to turn it on. */}
      {!pairingUsed && (
        <Card title={t("pairing.title")} hueIndex={nextHue()}>
          <p className="text-sm text-carbon-textSub">{t("settings.pairingNeedsDomain")}</p>
        </Card>
      )}

      {pairingUsed && (
        <div id="pairing" className="flex scroll-mt-6 flex-col gap-10">
          <PairingSection t={t} nextHue={nextHue} />
          <FleetSettingsCard t={t} settings={settings} setSettings={setSettings} save={save} hueIndex={nextHue()} />
        </div>
      )}
    </>
  );
}
