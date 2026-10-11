import { useState } from "react";
import { Button } from "../../../components/Button";
import { FolderBrowser } from "../../../components/FolderBrowser";
import { IconDownload, IconTabRecovery } from "../../../components/navGlyphs";
import { Rows, SettingRow } from "../../../components/storage/rows";
import { downloadRecoveryKit } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { RegistriesCard } from "../RegistriesCard";
import { PageLinkCard, PageSentence } from "../SidebarTile";
import { Card, hueCounter, type SaveState } from "../shared";
import { useSettings } from "../settingsStore";

export function RestorePage() {
  const { t } = useT();
  const { settings, setSettings, hostMountRoot, save, debouncedSave } = useSettings();
  const [, setSaveState] = useState<SaveState>("idle");
  const [, setSaveError] = useState<string | null>(null);
  // Why the server refused the kit, such as a login password that is not set.
  const [kitError, setKitError] = useState<string | null>(null);
  const nextHue = hueCounter();

  return (
    <>
      <PageLinkCard
        to="/restore"
        label={t("activityLog.typeRestore")}
        sub={t("tile.restore.pageSub")}
        glyph={<IconTabRecovery />}
        hueIndex={nextHue()}
      />

      <Card title={t("tile.restore.folders")} hint={t("tile.restore.foldersHint")} hueIndex={nextHue()}>
        <FolderBrowser
          label={t("settings.restoreFolder")}
          value={settings.restoreFolder}
          hostMountRoot={hostMountRoot}
          hint={t("settings.restoreFolderHint")}
          onChange={(v) => {
            setSettings((prev) => (prev ? { ...prev, restoreFolder: v } : prev));
            debouncedSave("restoreFolder", () => void save({ restoreFolder: v }, setSaveState, setSaveError));
          }}
        />
      </Card>

      {settings.containersEnabled && <RegistriesCard hueIndex={nextHue()} />}

      <Card title={t("tile.restore.emergency")} hueIndex={nextHue()}>
        <Rows>
          <SettingRow label={t("recovery.title")} hint={t("recovery.why")}>
            <Button
              label={t("recovery.download")}
              labelKey="recovery.download"
              glyph={<IconDownload />}
              onClick={() => {
                setKitError(null);
                void downloadRecoveryKit().then(setKitError);
              }}
            />
          </SettingRow>
        </Rows>
        {kitError && <p className="text-xs text-statusFail wrap-break-word">{kitError}</p>}
        <PageSentence text="tile.restore.verifyLine" page="activityLog.typeVerify" to="/settings/integrity" />
      </Card>
    </>
  );
}
