import { Link } from "react-router-dom";

import { InfoBubble } from "../../../components/InfoBubble";
import { RetentionPreview } from "../../../components/RetentionPreview";
import { useT } from "../../../lib/i18n";
import { Card } from "../shared";
import { useSettings } from "../settingsStore";

/** RetentionCards point to the storage locations, which hold every keep rule,
 *  and preview what the next run deletes. */
export function RetentionCards({ nextHue }: { nextHue: () => number }) {
  const { t } = useT();
  const { settings } = useSettings();

  return (
    <div id="retention" className="flex scroll-mt-6 flex-col gap-6 md:gap-10">
      <Card title={t("storage.keep.title")} hueIndex={nextHue()}>
        <p className="text-sm text-carbon-textSub">{t("settings.retentionWhere")}</p>
        <Link
          to="/storage"
          className="w-fit text-sm text-accentText hover:underline pointer-coarse:inline-flex pointer-coarse:min-h-(--btn-h) pointer-coarse:items-center"
        >
          {t("storage.rules.open")}
        </Link>
      </Card>

      {/* It runs on a button, because it costs one restic call per item per
          repository. */}
      <Card title={t("restore.preview")} hueIndex={nextHue()}>
        <div className="flex items-center gap-1 text-sm text-carbon-text">
          {t("retentionPreview.title")}
          <InfoBubble tip={t("retentionPreview.hint")} />
        </div>
        <RetentionPreview t={t} hasOffsite={(domain) => settings[`${domain}Offsite`] !== ""} />
      </Card>
    </div>
  );
}
