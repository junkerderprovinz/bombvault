import { useState } from "react";
import { NumberField } from "../../../components/NumberField";
import { Rows, SettingRow } from "../../../components/storage/rows";
import { useT } from "../../../lib/i18n";
import { tLtr } from "../../../lib/ltrFragments";
import { SettingsPortabilityCard } from "../SettingsPortabilityCard";
import { Card, ToggleRow, hueCounter, type SaveState } from "../shared";
import { useSettings } from "../settingsStore";

const NUMBER = "rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus w-28";

export function SystemPage() {
  const { t } = useT();
  const {
    settings,
    setSettings,
    save,
    debouncedSave,
    applyImportedSettings,
    fieldPulse,
    autoSaveField,
    mergedFieldBusy,
    mergedFieldShake,
  } = useSettings();
  const [, setSaveState] = useState<SaveState>("idle");
  const [, setSaveError] = useState<string | null>(null);

  function saveNumber(key: "resticCacheMaxMB" | "backupCores", raw: string) {
    const n = Math.max(0, parseInt(raw, 10) || 0);
    setSettings((prev) => (prev ? { ...prev, [key]: n } : prev));
    debouncedSave(key, () => void save({ [key]: n }, setSaveState, setSaveError));
  }

  const nextHue = hueCounter();

  return (
    <>
      <SettingsPortabilityCard t={t} hueIndex={nextHue()} applyImport={applyImportedSettings}>
        <div className="flex flex-col gap-3 border-t border-carbon-border pt-4">
          <ToggleRow
            label={t("export.encrypt.enable")}
            hint={`${t("export.encrypt.hint")} ${t("export.encrypt.ageInfo")} ${t("export.encrypt.enableHint")} ${t("export.encrypt.kitSealed")}`}
            checked={settings.exportEncryptEnabled}
            onChange={(v) => void autoSaveField("exportEncryptEnabled", v, setSaveState, setSaveError)}
            disabled={mergedFieldBusy.exportEncryptEnabled}
            shakeNonce={mergedFieldShake.exportEncryptEnabled}
            pulseNonce={fieldPulse.exportEncryptEnabled}
          />
          {settings.exportEncryptEnabled && (
            <label className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("export.encrypt.recipients")}</span>
              <textarea
                value={settings.exportAgeRecipients}
                spellCheck={false}
                rows={3}
                onChange={(e) => {
                  const v = e.target.value;
                  setSettings((prev) => (prev ? { ...prev, exportAgeRecipients: v } : prev));
                  debouncedSave("exportAgeRecipients", () =>
                    void save({ exportAgeRecipients: v }, setSaveState, setSaveError)
                  );
                }}
                placeholder={t("export.encrypt.recipientsPlaceholder")}
                dir="ltr"
                className="rounded-control bg-carbon-surface2 px-3 py-2 text-start font-mono text-sm text-carbon-text glim-field-focus"
              />
              {/* The accepted key syntax is looked up while pasting, so it stays on the page. */}
              <span className="text-xs text-carbon-textMuted">{t("export.encrypt.recipientsHint")}</span>
              {!settings.exportAgeRecipients.trim() && (
                <span className="text-xs text-statusFail">{t("export.encrypt.recipientsRequired")}</span>
              )}
            </label>
          )}
        </div>
      </SettingsPortabilityCard>

      <Card title={t("tile.system.performance")} hint={t("tile.system.performanceHint")} hueIndex={nextHue()}>
        <Rows>
          <SettingRow label={t("settings.cacheLimitLabel")} hint={tLtr(t, "settings.cacheHint")}>
            <NumberField
              min={0}
              value={settings.resticCacheMaxMB}
              aria-label={t("settings.cacheLimitLabel")}
              onChange={(e) => saveNumber("resticCacheMaxMB", e.target.value)}
              className={NUMBER}
            />
          </SettingRow>
          <SettingRow label={t("settings.coresLabel")} hint={t("settings.coresHint")}>
            <NumberField
              min={0}
              value={settings.backupCores}
              aria-label={t("settings.coresLabel")}
              onChange={(e) => saveNumber("backupCores", e.target.value)}
              className={NUMBER}
            />
          </SettingRow>
        </Rows>
      </Card>
    </>
  );
}
