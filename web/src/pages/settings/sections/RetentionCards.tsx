import { alsoDirectText } from "../../../lib/directRepo";
import { InfoBubble } from "../../../components/InfoBubble";
import { RetentionPreview } from "../../../components/RetentionPreview";
import { useT } from "../../../lib/i18n";
import { Card } from "../shared";
import { RetentionRulesCard } from "../OwnRetentionCard";
import { useSettings } from "../settingsStore";

/** RetentionCards are the keep rules of the built-in off-site copies and the
 *  preview of what the next run deletes. */
export function RetentionCards({ nextHue }: { nextHue: () => number }) {
  const { t } = useT();
  const { settings, setSettings, fieldDirects, saveOffsiteRetention, debouncedSave, cancelDebounce } = useSettings();

  return (
    <div id="retention" className="flex scroll-mt-6 flex-col gap-6 md:gap-10">
      <RetentionRulesCard
        settings={settings}
        setSettings={setSettings}
        save={saveOffsiteRetention}
        debouncedSave={debouncedSave}
        cancelDebounce={cancelDebounce}
        t={t}
        hueIndex={nextHue()}
      >
        {fieldDirects.map((u) => (
          <p key={u.target.id} className="text-xs text-carbon-textMuted">
            {alsoDirectText(t, u)}
          </p>
        ))}
      </RetentionRulesCard>

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
