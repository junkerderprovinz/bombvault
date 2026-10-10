import { IconFleet } from "../../../components/navGlyphs";
import { useT, type TranslationKey } from "../../../lib/i18n";
import { PageLinkCard, PageSentence, SidebarCard } from "../SidebarTile";
import { Card, ToggleRow, hueCounter } from "../shared";
import { useSettings } from "../settingsStore";

const MODULES = [
  { key: "fleetEnabled", label: "instances.title", hint: "settings.fleetEnabledHint" },
  { key: "receiverEnabled", label: "receiver.title", hint: "settings.receiverEnabledHint" },
  { key: "pullEnabled", label: "pull.title", hint: "settings.pullEnabledHint" },
] as const satisfies readonly { key: string; label: TranslationKey; hint: TranslationKey }[];

export function InstancesPage() {
  const { t } = useT();
  const { settings, fieldPulse, toggleDomainEnabled, domainToggleBusy, domainToggleShake } = useSettings();
  const nextHue = hueCounter();
  const on = MODULES.filter((m) => settings[m.key]).map((m) => t(m.label));

  return (
    <>
      <SidebarCard pin="instances" hueIndex={nextHue()} />
      <Card title={t("tile.instances.modules")} hint={t("tile.instances.modulesHint")} hueIndex={nextHue()}>
        {MODULES.map((m, i) => (
          <ToggleRow
            key={m.key}
            label={t(m.label)}
            hint={t(m.hint)}
            checked={settings[m.key]}
            onChange={(v) => void toggleDomainEnabled(m.key, v)}
            disabled={domainToggleBusy[m.key]}
            shakeNonce={domainToggleShake[m.key]}
            pulseNonce={fieldPulse[m.key]}
            hueIndex={i}
          />
        ))}
        <PageSentence text="tile.instances.pairingLine" page="pairing.title" to="/settings/pairing" />
      </Card>
      <PageLinkCard
        to="/instances"
        label={t("instances.title")}
        sub={on.length > 0 ? on.join(", ") : t("tile.instances.none")}
        glyph={<IconFleet />}
        hueIndex={nextHue()}
      />
    </>
  );
}
