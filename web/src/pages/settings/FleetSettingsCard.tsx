// FleetSettingsCard holds this instance's name as the other members of its
// pairing group show it, below the pairing cards on the same tab.
import { Settings } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { Card, type SaveState } from "./shared";
import { useRef, useState } from "react";

export function FleetSettingsCard({
  t,
  settings,
  setSettings,
  save,
  hueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  settings: Settings;
  setSettings: React.Dispatch<React.SetStateAction<Settings | null>>;
  save: (
    patch: Partial<Settings>,
    setSaveState: (s: SaveState) => void,
    setSaveError: (e: string | null) => void
  ) => Promise<boolean>;
  hueIndex?: number;
}) {
  // save() reports the outcome in a toast, so only the setters are used.
  const [, setNameSaveState] = useState<SaveState>("idle");
  const [, setNameSaveError] = useState<string | null>(null);
  // The name saves itself after a pause in typing. Only the save prop
  // crosses over from the settings page, so the card keeps its own debounce.
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  return (
    <Card title={t("settings.fleet")} hint={t("settings.fleetHint")} hueIndex={hueIndex}>
      <div className="flex flex-col gap-1.5">
        <label className="text-xs text-carbon-textSub">{t("settings.instanceName")}</label>
        <input
          type="text"
          value={settings.instanceName}
          onChange={(e) => {
            const v = e.target.value;
            setSettings((prev) => (prev ? { ...prev, instanceName: v } : prev));
            if (timer.current) clearTimeout(timer.current);
            timer.current = setTimeout(() => void save({ instanceName: v }, setNameSaveState, setNameSaveError), 800);
          }}
          spellCheck={false}
          autoComplete="off"
          placeholder="tower"
          className="flex-1 min-w-0 rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
        />
      </div>
    </Card>
  );
}
