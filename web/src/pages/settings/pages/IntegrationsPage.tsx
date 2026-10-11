import { useT } from "../../../lib/i18n";
import { hueCounter } from "../shared";
import { VMSSHCard } from "../VMSSHCard";
import { DashboardWidgetCard } from "../DashboardWidgetCard";
import { McpServerCard } from "../McpServerCard";
import { ApiTokensCard } from "../ApiTokensCard";
import { ServicesCard } from "../ServicesCard";
import { mcpShipped } from "../../../lib/mcpSwitch";
import { useSettings } from "../settingsStore";

export function IntegrationsPage() {
  const { t } = useT();
  const { settings, setSettings, savedBaseline, authEnabled } = useSettings();

  const nextHue = hueCounter();

  return (
    <>
      {mcpShipped && <McpServerCard hueIndex={nextHue()} passwordSet={authEnabled} />}
      <ApiTokensCard hueIndex={nextHue()} passwordSet={authEnabled} />
      <ServicesCard hueIndex={nextHue()} />
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
      <VMSSHCard t={t} hueIndex={nextHue()} />
    </>
  );
}
