import { useState } from "react";
import { alsoDirectText } from "../../../lib/directRepo";
import { InfoBubble } from "../../../components/InfoBubble";
import { RetentionPreview } from "../../../components/RetentionPreview";
import { useT } from "../../../lib/i18n";
import { Card, type SaveState } from "../shared";
import { RetentionRulesCard } from "../OwnRetentionCard";
import { useSettings } from "../settingsStore";

/** RetentionCards are the shared keep rules, local and off-site, and the
 *  preview of what the next run deletes. */
export function RetentionCards({ nextHue }: { nextHue: () => number }) {
  const { t } = useT();
  const {
    settings,
    setSettings,
    fieldDirects,
    save,
    saveOffsiteRetention,
    debouncedSave,
    cancelDebounce,
  } = useSettings();

  const [, setRetSaveState] = useState<SaveState>("idle");
  const [, setRetSaveError] = useState<string | null>(null);

  return (
    <div id="retention" className="flex scroll-mt-6 flex-col gap-6 md:gap-10">
      {(["local", "offsite"] as const).map((scope) => (
        <RetentionRulesCard
          key={scope}
          scope={scope}
          settings={settings}
          setSettings={setSettings}
          save={(patch) => (scope === "offsite" ? saveOffsiteRetention(patch) : save(patch, setRetSaveState, setRetSaveError))}
          debouncedSave={debouncedSave}
          cancelDebounce={cancelDebounce}
          t={t}
          hueIndex={nextHue()}
        >
          {scope === "offsite" &&
            fieldDirects.map((u) => (
              <p key={u.target.id} className="text-xs text-carbon-textMuted">
                {alsoDirectText(t, u)}
              </p>
            ))}
        </RetentionRulesCard>
      ))}

      {/* The answer the numbers above never give: which restore points the
          next run is about to delete, locally and off-site. It runs on a
          button, because it costs one restic call per item per repository. */}
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
