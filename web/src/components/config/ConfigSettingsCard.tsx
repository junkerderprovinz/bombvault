import { useState, type CSSProperties } from "react";
import { hueVars } from "../../lib/appearance";
import { getSettings, putSettings } from "../../lib/api";
import type { Settings } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { PlacementFlow } from "../placement/PlacementFlow";
import { ToggleRow } from "../../pages/settings/shared";
import { useToast } from "../../lib/toast";
import { Badge } from "../Badge";
import { InfoBubble } from "../InfoBubble";
import { tLtr } from "../../lib/ltrFragments";

type T = ReturnType<typeof useT>["t"];

type SaveState = "idle" | "saving";

// ConfigSettingsCard holds the self-backup toggle. Path, schedules and off-site
// settings for self-backup are edited in Settings, not here.
export function ConfigSettingsCard({
  t,
  settings,
  setSettings,
  hueIndex,
}: {
  t: T;
  settings: Settings;
  setSettings: (updater: (prev: Settings) => Settings) => void;
  /** Rainbow position for this card's heading notch. */
  hueIndex?: number;
}) {
  const { push } = useToast();
  const [, setSaveState] = useState<SaveState>("idle");

  async function persist(enabled: boolean) {
    setSaveState("saving");
    try {
      // Merge into freshly fetched settings: a PUT of this page's mount-time
      // copy would revert schedule, off-site and path changes made in Settings.
      const latest = await getSettings();
      // A failed fetch comes back as {ok:false}, not a throw. Saving the stale
      // copy instead would cause exactly that revert, so give up.
      if (!latest.ok) {
        setSaveState("idle");
        push(latest.error ?? t("config.loadSettingsFailed"), "fail");
        return;
      }
      const merged: Settings = {
        ...latest.settings,
        configEnabled: enabled,
      };
      const res = await putSettings(merged);
      if (res.ok) {
        setSaveState("idle");
        push(t("settings.saved"), "success");
      } else {
        setSaveState("idle");
        push(res.error ?? t("common.saveFailed"), "fail");
      }
    } catch (err) {
      setSaveState("idle");
      push(err instanceof Error ? err.message : t("common.saveFailed"), "fail");
    }
  }

  return (
    // glim-notch-card reveals the heading notch colour when the pointer or
    // focus is anywhere in the card; glim-hue sets the rainbow accent for
    // everything inside it.
    <div
      className={`relative glim-notch-card bg-carbon-surface rounded-card p-5 flex flex-col gap-4${
        hueIndex !== undefined ? " glim-hue" : ""
      }`}
      style={hueIndex !== undefined ? (hueVars(hueIndex) as CSSProperties) : undefined}
    >
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap hueIndex={hueIndex}>
          {t("config.settingsTitle")}
          <InfoBubble tip={t("config.settingsHint")} onAccent />
        </Badge>
      </h2>

      {/* Saves on change, like every other toggle in the app. */}
      <ToggleRow
        label={t("config.enabled")}
        hint={tLtr(t, "config.enabledHint")}
        checked={settings.configEnabled}
        onChange={(v) => {
          setSettings((prev) => ({ ...prev, configEnabled: v }));
          void persist(v);
        }}
      />

      <p className="text-xs text-carbon-textMuted">{t("config.pathMoved")}</p>

      <p className="text-xs text-carbon-textMuted">{t("config.offsiteMoved")}</p>
      <PlacementFlow domain="config" />

    </div>
  );
}
