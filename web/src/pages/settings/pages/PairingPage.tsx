import { useNavigate, useSearchParams } from "react-router-dom";
import { Button } from "../../../components/Button";
import { IconRecovery } from "../../../components/navGlyphs";
import { useT } from "../../../lib/i18n";
import { emergencyPath } from "../../../lib/restorePlan";
import { hueCounter } from "../shared";
import { NameRow } from "../pairing/NameRow";
import { PairingSection } from "../pairing/PairingSection";
import { useSettings } from "../settingsStore";

export function PairingPage() {
  const { t } = useT();
  const { settings, setSettings, save, toggleDomainEnabled, domainToggleBusy, domainToggleShake, fieldPulse } = useSettings();

  // The emergency window sends a new install here to join the group that
  // keeps its key.
  const navigate = useNavigate();
  const forEmergency = useSearchParams()[0].get("for") === "emergency";

  const nextHue = hueCounter();

  return (
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
        nameRow={(defaultName) => <NameRow t={t} settings={settings} setSettings={setSettings} save={save} defaultName={defaultName} />}
        giveBack={{
          on: settings.appImportEnabled,
          onChange: (on) => void toggleDomainEnabled("appImportEnabled", on),
          busy: domainToggleBusy.appImportEnabled,
          shakeNonce: domainToggleShake.appImportEnabled,
          pulseNonce: fieldPulse.appImportEnabled,
        }}
      />
    </div>
  );
}
