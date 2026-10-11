import { useNavigate, useSearchParams } from "react-router-dom";
import { Button } from "../../../components/Button";
import { IconRecovery } from "../../../components/navGlyphs";
import { useT } from "../../../lib/i18n";
import { emergencyPath } from "../../../lib/restorePlan";
import { Card, hueCounter } from "../shared";
import { FleetSettingsCard } from "../FleetSettingsCard";
import { PairingSection } from "../pairing/PairingSection";
import { useSettings } from "../settingsStore";

export function PairingPage() {
  const { t } = useT();
  const { settings, setSettings, save } = useSettings();

  // The emergency window sends a new install here to join the group that
  // keeps its key, before any of the sections that use a group is on.
  const navigate = useNavigate();
  const forEmergency = useSearchParams()[0].get("for") === "emergency";
  const pairingUsed = settings.receiverEnabled || settings.fleetEnabled || settings.pullEnabled || forEmergency;

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
          {forEmergency && (
            <div className="flex flex-wrap items-center gap-3 rounded-card bg-carbon-surface p-5 text-sm text-carbon-text">
              <span className="min-w-0 flex-1 basis-56">{t("pairing.forEmergency")}</span>
              <Button
                label={t("pairing.backToEmergency")}
                labelKey={null}
                glyph={<IconRecovery />}
                tone="accent"
                onClick={() => navigate(emergencyPath("access", "group"))}
              />
            </div>
          )}
          <PairingSection
            t={t}
            nextHue={nextHue}
            zfs={settings.zfsEnabled}
            instanceCard={(address, hueIndex) => (
              <FleetSettingsCard t={t} settings={settings} setSettings={setSettings} save={save} hueIndex={hueIndex}>
                {address}
              </FleetSettingsCard>
            )}
          />
        </div>
      )}
    </>
  );
}
