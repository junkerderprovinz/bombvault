import { useState } from "react";
import { useT } from "../../lib/i18n";
import { CryptScene } from "./pictures";
import { PageSentence } from "./SidebarTile";
import { Card, ToggleRow, type SaveState } from "./shared";
import { useSettings } from "./settingsStore";

/** EncryptionCard switches the encryption of the repositories and shows what
 *  it means: the storage location holds sealed packs, the key stays here. */
export function EncryptionCard({ hueIndex }: { hueIndex: number }) {
  const { t } = useT();
  const { settings, fieldPulse, autoSaveField, mergedFieldBusy, mergedFieldShake } = useSettings();
  const [, setSaveState] = useState<SaveState>("idle");
  const [, setSaveError] = useState<string | null>(null);

  return (
    <Card title={t("tile.crypt.title")} hueIndex={hueIndex}>
      <ToggleRow
        label={settings.encryptionEnabled ? t("settings.encryptionOn") : t("settings.encryptionOff")}
        hint={`${t("settings.encryptionHint")} ${t("settings.encryptionPasswordWhere")}`}
        checked={settings.encryptionEnabled}
        onChange={(v) => void autoSaveField("encryptionEnabled", v, setSaveState, setSaveError)}
        disabled={mergedFieldBusy.encryptionEnabled}
        shakeNonce={mergedFieldShake.encryptionEnabled}
        pulseNonce={fieldPulse.encryptionEnabled}
      />
      <CryptScene />
      <p className="text-center text-sm text-carbon-textSub">{t("tile.crypt.line")}</p>
      <PageSentence text="tile.crypt.kitLine" page="activityLog.typeRestore" to="/settings/restore" />
    </Card>
  );
}
