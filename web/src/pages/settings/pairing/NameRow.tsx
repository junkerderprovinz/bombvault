// NameRow holds the one name this instance goes by, at its other instances,
// on its phones and at the other apps of the fortress. It saves itself after
// a pause in typing, as the settings do.
import { useRef, useState } from "react";
import { fill } from "@glimstone/core";
import { FieldRow } from "@glimstone/react/FieldRow";
import type { Settings } from "../../../lib/api";
import type { useT } from "../../../lib/i18n";
import type { SaveState } from "../shared";

const SAVE_AFTER_MS = 800;

export function NameRow({
  t,
  settings,
  setSettings,
  save,
  defaultName,
}: {
  t: ReturnType<typeof useT>["t"];
  settings: Settings;
  setSettings: React.Dispatch<React.SetStateAction<Settings | null>>;
  save: (patch: Partial<Settings>, setSaveState: (s: SaveState) => void, setSaveError: (e: string | null) => void) => Promise<boolean>;
  /** The name it goes by while the field is empty. */
  defaultName: string;
}) {
  // save() reports the outcome in a toast, so only the setters are used.
  const [, setSaveState] = useState<SaveState>("idle");
  const [, setSaveError] = useState<string | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  return (
    <FieldRow label={t("settings.instanceName")} hint={fill(t("pairing.nameHint"), { name: defaultName })}>
      <input
        id="instance-name"
        type="text"
        aria-label={t("settings.instanceName")}
        value={settings.instanceName}
        maxLength={64}
        onChange={(e) => {
          const v = e.target.value;
          setSettings((prev) => (prev ? { ...prev, instanceName: v } : prev));
          if (timer.current) clearTimeout(timer.current);
          timer.current = setTimeout(() => void save({ instanceName: v }, setSaveState, setSaveError), SAVE_AFTER_MS);
        }}
        spellCheck={false}
        autoComplete="off"
        placeholder={defaultName}
        className="w-[18rem] max-w-full rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
      />
    </FieldRow>
  );
}
