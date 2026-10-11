import { useState } from "react";
import { RevealInput } from "../../components/RevealInput";
import { SettingRow } from "../../components/storage/rows";
import { Toggle } from "../../components/Toggle";
import { useT } from "../../lib/i18n";
import { tLtr } from "../../lib/ltrFragments";
import { useReveal } from "../../lib/useReveal";
import { HomeAssistantRows } from "./HomeAssistantRows";
import { MdnsRow } from "./MdnsRow";
import { Card, type SaveState } from "./shared";
import { useSettings } from "./settingsStore";

/** ServicesCard holds what BombVault offers to other programs on the network:
 *  Home Assistant over MQTT, the mDNS name and the Prometheus metrics. */
export function ServicesCard({ hueIndex }: { hueIndex?: number }) {
  const { t } = useT();
  const { settings, setSettings, save, debouncedSave, autoSaveToggle, fieldBusy, fieldShake } = useSettings();
  const reveal = useReveal();
  const [, setSaveState] = useState<SaveState>("idle");
  const [, setSaveError] = useState<string | null>(null);

  return (
    <Card title={t("integrations.services")} hueIndex={hueIndex}>
      <div className="flex flex-col divide-y divide-carbon-border/60">
        <HomeAssistantRows />
        <MdnsRow />
        <SettingRow label={tLtr(t, "settings.metricsEnable")} hint={tLtr(t, "settings.metricsHint")}>
          <Toggle
            key={fieldShake.metricsEnabled}
            hideLabel
            label={tLtr(t, "settings.metricsEnable")}
            checked={settings.metricsEnabled}
            onChange={(v) => void autoSaveToggle("metricsEnabled", v, setSaveState, setSaveError)}
            disabled={fieldBusy.metricsEnabled}
            className={fieldShake.metricsEnabled ? "glim-shake" : ""}
          />
        </SettingRow>
        {settings.metricsEnabled && (
          <div className="py-3 ps-4">
            <SettingRow label={t("settings.metricsToken")}>
              {/* The server never sends the token back: a blank save keeps the
                  stored one, which shows as set. */}
              <RevealInput
                {...reveal}
                value={settings.metricsToken}
                aria-label={t("settings.metricsToken")}
                spellCheck={false}
                autoComplete="off"
                onChange={(e) => {
                  const v = e.target.value;
                  setSettings((prev) => (prev ? { ...prev, metricsToken: v } : prev));
                  debouncedSave("metricsToken", () =>
                    void save(
                      { metricsToken: v, metricsTokenSet: v.trim() !== "" || settings.metricsTokenSet },
                      setSaveState,
                      setSaveError
                    )
                  );
                }}
                placeholder={settings.metricsTokenSet && settings.metricsToken === "" ? t("cloud.secretSet") : ""}
                wrapperClassName="w-80 max-w-full"
                className="w-full rounded-control bg-carbon-surface2 px-3 py-1.5 font-mono text-sm text-carbon-text glim-field-focus"
              />
            </SettingRow>
          </div>
        )}
      </div>
    </Card>
  );
}
