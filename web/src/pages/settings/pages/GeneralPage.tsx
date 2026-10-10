import { LanguageCard } from "../LanguageCard";
import { useT } from "../../../lib/i18n";
import { useToast } from "../../../lib/toast";
import { tLtr } from "../../../lib/ltrFragments";
import { Card, ToggleRow, hueCounter } from "../shared";
import { AboutCard } from "../AboutCard";
import { useSettings } from "../settingsStore";

export function GeneralPage() {
  const { t } = useT();
  const { quiet, setQuiet } = useToast();
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
        {/* Indented under Containers: it only acts on containers, and reads as
            a sub-option of that domain rather than a domain of its own. */}
        <div className="ps-6">
          <ToggleRow
            label={t("settings.dbDumps")}
            hint={t("settings.dbDumpsHint")}
            checked={settings.dbDumpsEnabled}
            onChange={(v) => void toggleDbDumps(v)}
            disabled={domainToggleBusy.dbDumpsEnabled}
            shakeNonce={domainToggleShake.dbDumpsEnabled}
            pulseNonce={fieldPulse.dbDumpsEnabled}
          />
        </div>
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
        <ToggleRow
          label={t("receiver.title")}
          hint={t("settings.receiverEnabledHint")}
          checked={settings.receiverEnabled}
          onChange={(v) => void toggleDomainEnabled("receiverEnabled", v)}
          disabled={domainToggleBusy.receiverEnabled}
          shakeNonce={domainToggleShake.receiverEnabled}
          pulseNonce={fieldPulse.receiverEnabled}
          hueIndex={6}
        />
        {/* Named after Instances, not the Fleet page it shows: "Flotte" alone
            does not say what this domain is. */}
        <ToggleRow
          label={t("instances.title")}
          hint={t("settings.fleetEnabledHint")}
          checked={settings.fleetEnabled}
          onChange={(v) => void toggleDomainEnabled("fleetEnabled", v)}
          disabled={domainToggleBusy.fleetEnabled}
          shakeNonce={domainToggleShake.fleetEnabled}
          pulseNonce={fieldPulse.fleetEnabled}
          hueIndex={7}
        />
        {/* Pull (#227) is the only one of the three that writes: it fetches
            another instance's backups into this box's own repository. */}
        <ToggleRow
          label={t("pull.title")}
          hint={t("settings.pullEnabledHint")}
          checked={settings.pullEnabled}
          onChange={(v) => void toggleDomainEnabled("pullEnabled", v)}
          disabled={domainToggleBusy.pullEnabled}
          shakeNonce={domainToggleShake.pullEnabled}
          pulseNonce={fieldPulse.pullEnabled}
          hueIndex={8}
        />
      </Card>

      {/* Language and the look page's appearance axes belong to the person
          and apply at once, without a Save. */}
      <LanguageCard t={t} hueIndex={nextHue()} />

      {/* Quiet toasts is a client-side display preference, separate from the
          server-side notification switch: muting a toast in this browser must
          never change what a webhook receives. */}
      <Card title={t("settings.quietToasts")} hueIndex={nextHue()}>
        <ToggleRow
          label={t("settings.quietToasts")}
          hint={t("settings.quietToastsHint")}
          checked={quiet}
          onChange={setQuiet}
        />
      </Card>

      {/* About stays last on General: the card is a footer. */}
      <AboutCard hueIndex={nextHue()} />
    </>
  );
}
