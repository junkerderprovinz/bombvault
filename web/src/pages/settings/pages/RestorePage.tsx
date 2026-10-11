import { useState } from "react";
import { Button } from "../../../components/Button";
import { IconDownload, IconTabRecovery } from "../../../components/navGlyphs";
import { Rows, SettingRow } from "../../../components/storage/rows";
import { downloadRecoveryKit } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { RegistriesCard } from "../RegistriesCard";
import { RestoreFoldersCard } from "../RestoreFoldersCard";
import { PageLinkCard, PageSentence } from "../SidebarTile";
import { Card, hueCounter } from "../shared";
import { useSettings } from "../settingsStore";

export function RestorePage() {
  const { t } = useT();
  const { settings } = useSettings();
  // Why the server refused the kit, such as a login password that is not set.
  const [kitError, setKitError] = useState<string | null>(null);
  const nextHue = hueCounter();

  return (
    <>
      <PageLinkCard
        to="/recovery"
        label={t("activityLog.typeRestore")}
        sub={t("tile.restore.pageSub")}
        glyph={<IconTabRecovery />}
        hueIndex={nextHue()}
      />

      <RestoreFoldersCard hueIndex={nextHue()} />

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
