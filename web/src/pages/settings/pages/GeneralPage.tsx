import { useT } from "../../../lib/i18n";
import { tLtr } from "../../../lib/ltrFragments";
import { Card, ToggleRow, hueCounter } from "../shared";
import { useSettings } from "../settingsStore";

export function GeneralPage() {
  const { t } = useT();
  const {
    settings,
    fieldPulse,
    toggleDomainEnabled,
    toggleDbDumps,
    domainToggleBusy,
    domainToggleShake,
  } = useSettings();

  const nextHue = hueCounter();

  return (
    <>
      <Card title={t("settings.domains")} hint={t("settings.domainsHint")} hueIndex={nextHue()}>
        {/* Each row saves on click through toggleDomainEnabled. `disabled`
            covers the row's own request in flight, so a second click cannot
            race the first. */}
        <ToggleRow
          label={t("settings.containersEnabled")}
          hint={t("settings.containersEnabledHint")}
          checked={settings.containersEnabled}
          onChange={(v) => void toggleDomainEnabled("containersEnabled", v)}
          disabled={domainToggleBusy.containersEnabled}
          shakeNonce={domainToggleShake.containersEnabled}
          pulseNonce={fieldPulse.containersEnabled}
          hueIndex={0}
        />
        <ToggleRow
          label={t("settings.vmsEnabled")}
          hint={t("settings.vmsEnabledHint")}
          checked={settings.vmsEnabled}
          onChange={(v) => void toggleDomainEnabled("vmsEnabled", v)}
          disabled={domainToggleBusy.vmsEnabled}
          shakeNonce={domainToggleShake.vmsEnabled}
          pulseNonce={fieldPulse.vmsEnabled}
          hueIndex={1}
        />
        <ToggleRow
          label={t("settings.flashEnabled")}
          hint={tLtr(t, "settings.flashEnabledHint")}
          checked={settings.flashEnabled}
          onChange={(v) => void toggleDomainEnabled("flashEnabled", v)}
          disabled={domainToggleBusy.flashEnabled}
          shakeNonce={domainToggleShake.flashEnabled}
          pulseNonce={fieldPulse.flashEnabled}
          hueIndex={2}
        />
        <ToggleRow
          label={t("settings.filesEnabled")}
          hint={t("settings.filesEnabledHint")}
          checked={settings.filesEnabled}
          onChange={(v) => void toggleDomainEnabled("filesEnabled", v)}
          disabled={domainToggleBusy.filesEnabled}
          shakeNonce={domainToggleShake.filesEnabled}
          pulseNonce={fieldPulse.filesEnabled}
          hueIndex={3}
        />
        <ToggleRow
          label={t("settings.zfsEnabled")}
          hint={t("settings.zfsEnabledHint")}
          checked={settings.zfsEnabled}
          onChange={(v) => void toggleDomainEnabled("zfsEnabled", v)}
          disabled={domainToggleBusy.zfsEnabled}
          shakeNonce={domainToggleShake.zfsEnabled}
          pulseNonce={fieldPulse.zfsEnabled}
          hueIndex={4}
        />
        <ToggleRow
          label={t("settings.configEnabled")}
          hint={t("settings.configEnabledHint")}
          checked={settings.configEnabled}
          onChange={(v) => void toggleDomainEnabled("configEnabled", v)}
          disabled={domainToggleBusy.configEnabled}
          shakeNonce={domainToggleShake.configEnabled}
          pulseNonce={fieldPulse.configEnabled}
          hueIndex={5}
        />
      </Card>

      {settings.containersEnabled && (
        <Card title={t("nav.containers")} hint={t("tile.general.containersHint")} hueIndex={nextHue()}>
          <ToggleRow
            label={t("settings.dbDumps")}
            hint={t("settings.dbDumpsHint")}
            checked={settings.dbDumpsEnabled}
            onChange={(v) => void toggleDbDumps(v)}
            disabled={domainToggleBusy.dbDumpsEnabled}
            shakeNonce={domainToggleShake.dbDumpsEnabled}
            pulseNonce={fieldPulse.dbDumpsEnabled}
          />
        </Card>
      )}
    </>
  );
}
