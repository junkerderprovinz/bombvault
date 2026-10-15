// The pieces a retention rule is edited with, wherever it is set: for a whole
// storage location or for one section on it.
import { useMemo, useState, type ReactNode } from "react";
import { useT, type TranslationKey } from "../../lib/i18n";
import { setAskKeepLess, useAskKeepLess } from "../../lib/keepAsk";
import { lostByChange, sampleBackups, type KeepCounts, type KeepInput } from "../../lib/keepPlan";
import { useConfirm } from "../../lib/useConfirm";
import { ToggleRow } from "../../pages/settings/shared";
import { NumberSetting } from "./fields";
import { Rows, SettingRow } from "./rows";

const COUNTS: { label: TranslationKey; tip: TranslationKey }[] = [
  { label: "storage.keep.count.last", tip: "storage.keep.tip.last" },
  { label: "storage.keep.count.daily", tip: "storage.keep.tip.daily" },
  { label: "storage.keep.count.weekly", tip: "storage.keep.tip.weekly" },
  { label: "storage.keep.count.monthly", tip: "storage.keep.tip.monthly" },
  { label: "storage.keep.count.yearly", tip: "storage.keep.tip.yearly" },
];

/** The five counts of a rule, each saving itself after a pause in typing. */
export function KeepCountFields({
  counts,
  disabled,
  onCommit,
}: {
  counts: KeepCounts;
  disabled: boolean;
  onCommit: (index: number, count: number) => void;
}) {
  const { t } = useT();
  return (
    <div className="flex flex-col gap-1.5">
      <Rows>
        {COUNTS.map(({ label, tip }, i) => (
          <SettingRow key={label} label={t(label)} hint={t(tip)}>
            <NumberSetting label={t(label)} value={counts[i]} disabled={disabled} onCommit={(n) => onCommit(i, n)} />
          </SettingRow>
        ))}
      </Rows>
      <p className="text-xs text-carbon-textMuted">{t("storage.keep.zero")}</p>
    </div>
  );
}

/** The switch in the question itself. It reports up, because the answer is
 *  only stored once the question is answered with yes. */
function DontAskAgain({ onChange }: { onChange: (quiet: boolean) => void }) {
  const { t } = useT();
  const [quiet, setQuiet] = useState(false);
  return (
    <ToggleRow
      label={t("storage.keep.dontAsk")}
      hint={t("storage.keep.dontAskHint")}
      checked={quiet}
      onChange={(next) => {
        setQuiet(next);
        onChange(next);
      }}
    />
  );
}

/**
 * useKeepLessQuestion asks before a rule that keeps fewer backups is saved.
 * No location reports backups of its own, so what the change would remove is
 * counted over a sample history, the same one the picture is drawn over.
 */
export function useKeepLessQuestion(): {
  backups: KeepInput[];
  /** Resolves to whether the change may be saved. A change that removes
   *  nothing, or a question that was switched off, is not asked. */
  agreed: (before: KeepCounts, after: KeepCounts, place: string) => Promise<boolean>;
  dialog: ReactNode;
} {
  const { t } = useT();
  const { confirm, confirmDialog } = useConfirm();
  const ask = useAskKeepLess();
  const backups = useMemo(() => sampleBackups(new Date()), []);

  async function agreed(before: KeepCounts, after: KeepCounts, place: string): Promise<boolean> {
    const lost = lostByChange(before, after, backups).length;
    if (!ask || lost === 0) return true;
    let quiet = false;
    const message = t("storage.keep.askLess")
      .replace("{n}", String(lost))
      .replace("{total}", String(backups.length))
      .replace("{name}", () => place);
    const yes = await confirm(message, {
      confirmLabel: t("storage.keep.saveAnyway"),
      confirmLabelKey: "storage.keep.saveAnyway",
      cancelLabel: t("common.back"),
      extra: <DontAskAgain onChange={(next) => (quiet = next)} />,
    });
    if (yes && quiet) setAskKeepLess(false);
    return yes;
  }

  return { backups, agreed, dialog: confirmDialog };
}
