import { useState } from "react";
import { RevealInput } from "../../../components/RevealInput";
import { useReveal } from "../../../lib/useReveal";
import { useT } from "../../../lib/i18n";
import { tLtr } from "../../../lib/ltrFragments";
import { SpikePanel } from "../../../components/SpikePanel";
import { Card, ToggleRow, hueCounter, type SaveState } from "../shared";
import { VMSSHCard } from "../VMSSHCard";
import { DashboardWidgetCard } from "../DashboardWidgetCard";
import { McpServerCard } from "../McpServerCard";
import { ApiTokensCard } from "../ApiTokensCard";
import { HomeAssistantCard } from "../HomeAssistantCard";
import { NetworkCard } from "../NetworkCard";
import { mcpShipped } from "../../../lib/mcpSwitch";
import { useSettings } from "../settingsStore";

export function IntegrationsPage() {
  const { t } = useT();
  const {
    settings,
    setSettings,
    savedBaseline,
    authEnabled,
    save,
    debouncedSave,
    fieldPulse,
    autoSaveToggle,
    fieldBusy,
    fieldShake,
  } = useSettings();

  const revealMetricsToken = useReveal();

  const [, setMetricsSaveState] = useState<SaveState>("idle");
  const [, setMetricsSaveError] = useState<string | null>(null);

  const nextHue = hueCounter();

  return (
    <>
      <Card title={t("settings.metrics")} hueIndex={nextHue()}>
        {/* The /metrics and bearer-token syntax can sit in a bubble: it is
            used right here, not typed into another page from memory. */}
        <ToggleRow
          label={tLtr(t, "settings.metricsEnable")}
          hint={tLtr(t, "settings.metricsHint")}
          checked={settings.metricsEnabled}
          onChange={(v) => void autoSaveToggle("metricsEnabled", v, setMetricsSaveState, setMetricsSaveError)}
          disabled={fieldBusy.metricsEnabled}
          shakeNonce={fieldShake.metricsEnabled}
          pulseNonce={fieldPulse.metricsEnabled}
        />
        {/* Write-only secret (the GET never echoes it): a blank save keeps the
            stored token, so a stored one shows the same "already set"
            placeholder the cloud-credential secrets use. */}
        <label className="flex flex-col gap-1.5">
          <span className="text-xs text-carbon-textSub">{t("settings.metricsToken")}</span>
          <RevealInput
            {...revealMetricsToken}
            value={settings.metricsToken}
            spellCheck={false}
            autoComplete="off"
            onChange={(e) => {
              const v = e.target.value;
              setSettings((prev) => prev ? { ...prev, metricsToken: v } : prev);
              // A non-blank token marks itself set; a blank save keeps
              // whatever was stored.
              debouncedSave("metricsToken", () =>
                void save(
                  { metricsToken: v, metricsTokenSet: v.trim() !== "" || settings.metricsTokenSet },
                  setMetricsSaveState,
                  setMetricsSaveError
                )
              );
            }}
            placeholder={settings.metricsTokenSet && settings.metricsToken === "" ? t("cloud.secretSet") : ""}
            wrapperClassName="w-full"
            className="rounded-control bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus"
          />
        </label>
      </Card>

      <DashboardWidgetCard
        t={t}
        tokenSet={settings.widgetTokenSet}
        onTokenSet={(set) => {
          // Keep both the live state and the saved baseline in sync: the token
          // is managed by its own endpoints, so a later save (which merges onto
          // the baseline) must not carry a stale widgetTokenSet.
          setSettings((prev) => (prev ? { ...prev, widgetTokenSet: set } : prev));
          if (savedBaseline.current) {
            savedBaseline.current = { ...savedBaseline.current, widgetTokenSet: set };
          }
        }}
        hueIndex={nextHue()}
      />
      {mcpShipped && <McpServerCard hueIndex={nextHue()} passwordSet={authEnabled} />}
      <ApiTokensCard hueIndex={nextHue()} passwordSet={authEnabled} />
      <HomeAssistantCard hueIndex={nextHue()} />
      <NetworkCard hueIndex={nextHue()} />

      <VMSSHCard t={t} hueIndex={nextHue()} />

      {(() => {
        // One hueIdx for the heading and SpikePanel's button.
        const hueIdx = nextHue();
        return (
          <Card title={t("spike.title")} hueIndex={hueIdx}>
            <SpikePanel t={t} hueIndex={hueIdx} />
          </Card>
        );
      })()}
    </>
  );
}
