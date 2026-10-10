import type { Destination } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { Button } from "../Button";
import { InfoBubble } from "../InfoBubble";
import { TestButton } from "../TestButton";
import { GroupReceiverPicker } from "./GroupReceiverPicker";
import { ProviderPicker, providerName } from "./ProviderPicker";
import { DESTINATION_WORDS, FolderBody, ProtectBody, RecoveryBody, SignInBody } from "./steps";
import { useDestinationDraft } from "./useDestinationDraft";

const STEP = "text-xs font-semibold text-carbon-textSub uppercase tracking-widest flex items-center gap-1.5";

export function DestinationWizard({ onDone, onCancel }: { onDone: (d: Destination) => void; onCancel: () => void }) {
  const { t } = useT();
  const draft = useDestinationDraft();
  const { picked, reached } = draft;
  const shownName = picked ? providerName(picked, t) : "";
  const withName = (key: TranslationKey) => t(key).replace("{name}", () => shownName);

  async function save() {
    const made = await draft.save();
    if (made) onDone(made);
  }

  return (
    <div className="mt-2 flex flex-col gap-4 rounded-card bg-carbon-surface2 p-4">
      <section className="flex flex-col gap-3">
        <span className={STEP}>{t("dest.step.where")}</span>
        {draft.loadError && <p className="text-xs text-statusFail">{draft.loadError}</p>}
        <GroupReceiverPicker picked={draft.fromMember} onPick={(r) => void draft.pickFromGroup(r)} />
        <ProviderPicker providers={draft.providers} picked={draft.fromMember ? undefined : picked?.id} onPick={draft.pick} />
        <p className="text-xs text-carbon-textMuted">{t("dest.ownConf")}</p>
      </section>

      {picked && (
        <section className="flex flex-col gap-3 border-t border-carbon-border pt-3">
          <span className={STEP}>{withName("dest.step.signIn")}</span>
          <SignInBody draft={draft} providerLabel={shownName} />
          <div className="flex flex-wrap items-center justify-end gap-2">
            <Button label={t("dest.otherProvider")} labelKey={null} tone="neutral" onClick={() => draft.pick({ ...picked })} />
            <TestButton label={t("offsite.test")} labelKey="offsite.test" tone="accent" test={draft.test} onClick={() => void draft.runTest()} />
          </div>
        </section>
      )}

      {picked && reached && (
        <section className="flex flex-col gap-3 border-t border-carbon-border pt-3">
          <span className={STEP}>
            {t("dest.step.folder")}
            <InfoBubble tip={t(DESTINATION_WORDS.folderTip)} />
          </span>
          <FolderBody draft={draft} />
        </section>
      )}

      {picked && reached && (
        <section className="flex flex-col gap-2 border-t border-carbon-border pt-3">
          <span className={STEP}>{t("dest.step.protect")}</span>
          <ProtectBody draft={draft} providerLabel={shownName} />
        </section>
      )}

      {picked && reached && (
        <section className="flex flex-col gap-2 border-t border-carbon-border pt-3">
          <span className={STEP}>{t("dest.step.recovery")}</span>
          <RecoveryBody draft={draft} providerLabel={shownName} />
          <div className="flex flex-wrap items-center justify-end gap-2">
            <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={onCancel} />
            <Button
              label={t("dest.save")}
              labelKey="dest.save"
              tone="accent"
              onClick={() => void save()}
              busy={draft.saving}
              disabled={draft.saving || draft.folderOpen}
            />
          </div>
        </section>
      )}
    </div>
  );
}
