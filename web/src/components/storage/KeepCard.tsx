import { useEffect, useMemo, useRef, useState } from "react";
import type { RetentionKeep } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { setAskKeepLess, useAskKeepLess } from "../../lib/keepAsk";
import { KEEP_PRESETS, lostByChange, presetOf, sampleBackups, type KeepCounts, type KeepPreset } from "../../lib/keepPlan";
import { keepCounts, retentionKeep } from "../../lib/storageLocations";
import { useConfirm } from "../../lib/useConfirm";
import { Card, ToggleRow } from "../../pages/settings/shared";
import { Badge } from "../Badge";
import { InfoBubble } from "../InfoBubble";
import { Selector } from "../Selector";
import { Toggle } from "../Toggle";
import { NumberSetting } from "./fields";
import { KeepTimeline } from "./KeepTimeline";
import { Deviations, Rows, SettingRow } from "./rows";
import type { LocationEdit } from "./useLocationEdit";

type Mode = KeepPreset | "custom";

const MODES: { id: Mode; label: TranslationKey }[] = [
  { id: "short", label: "storage.keep.short" },
  { id: "balanced", label: "storage.keep.balanced" },
  { id: "long", label: "storage.keep.long" },
  { id: "custom", label: "storage.keep.custom" },
];

const COUNTS: { label: TranslationKey; tip: TranslationKey }[] = [
  { label: "storage.keep.count.last", tip: "storage.keep.tip.last" },
  { label: "storage.keep.count.daily", tip: "storage.keep.tip.daily" },
  { label: "storage.keep.count.weekly", tip: "storage.keep.tip.weekly" },
  { label: "storage.keep.count.monthly", tip: "storage.keep.tip.monthly" },
  { label: "storage.keep.count.yearly", tip: "storage.keep.tip.yearly" },
];

type T = ReturnType<typeof useT>["t"];

function summary(t: T, keep: RetentionKeep): string {
  return t("storage.keep.summary")
    .replace("{last}", String(keep.keepLast))
    .replace("{daily}", String(keep.keepDaily))
    .replace("{weekly}", String(keep.keepWeekly))
    .replace("{monthly}", String(keep.keepMonthly))
    .replace("{yearly}", String(keep.keepYearly));
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
 * KeepCard sets how long a location keeps its backups: one of three presets or
 * five counts of its own, with a picture of what the rule keeps. The location
 * reports no backups of its own, so the picture is drawn over a sample history.
 */
export function KeepCard({ edit, retention, hueIndex }: { edit: LocationEdit; retention: RetentionKeep; hueIndex: number }) {
  const { t } = useT();
  const { location, can, save, follow, busy } = edit;
  const { confirm, confirmDialog } = useConfirm();
  const ask = useAskKeepLess();
  const backups = useMemo(() => sampleBackups(new Date()), []);
  const stored = keepCounts(retention);
  const storedKey = stored.join(",");
  const [draft, setDraft] = useState(stored);
  const [custom, setCustom] = useState(presetOf(stored) === "custom");
  const [seen, setSeen] = useState(storedKey);
  // Two counts typed one after the other each save after their own pause; the
  // second has to build on the first.
  const latest = useRef(draft);
  useEffect(() => {
    latest.current = draft;
  }, [draft]);
  if (seen !== storedKey) {
    setSeen(storedKey);
    setDraft(stored);
  }
  const editable = can("retention");
  const asks = location.kind !== "local";

  function revert() {
    setDraft(stored);
    setCustom(presetOf(stored) === "custom");
  }

  async function apply(next: KeepCounts) {
    setDraft(next);
    const lost = lostByChange(stored, next, backups).length;
    if (asks && ask && lost > 0) {
      let quiet = false;
      const message = t("storage.keep.askLess")
        .replace("{n}", String(lost))
        .replace("{total}", String(backups.length))
        .replace("{name}", () => location.name);
      const yes = await confirm(message, {
        confirmLabel: t("storage.keep.saveAnyway"),
        confirmLabelKey: "storage.keep.saveAnyway",
        cancelLabel: t("common.back"),
        extra: <DontAskAgain onChange={(next) => (quiet = next)} />,
      });
      if (!yes) {
        revert();
        return;
      }
      if (quiet) setAskKeepLess(false);
    }
    if (!(await save({ retention: retentionKeep(next) }))) revert();
  }

  function pick(mode: Mode) {
    setCustom(mode === "custom");
    if (mode !== "custom") void apply([...KEEP_PRESETS[mode]]);
  }

  const mode: Mode = custom ? "custom" : presetOf(draft);

  return (
    <Card title={t("storage.keep.title")} hint={t("storage.keep.hint")} hueIndex={hueIndex}>
      {confirmDialog}
      {editable ? (
        <Selector
          items={MODES.map(({ id, label }) => ({ id, label: t(label) }))}
          label={t("storage.keep.title")}
          select="one"
          activation="manual"
          inline
          pairsOnPhone
          active={mode}
          onChange={(id) => pick(id as Mode)}
          disabled={busy}
        />
      ) : (
        <div className="flex flex-wrap items-center gap-2">
          <Badge tone="neutral" wrap>
            {mode === "custom" ? summary(t, retention) : t(`storage.keep.${mode}`)}
          </Badge>
        </div>
      )}

      {editable && custom && (
        <div className="flex flex-col gap-1.5">
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-5">
            {COUNTS.map(({ label, tip }, i) => (
              <div key={label} className="flex min-w-0 flex-col gap-1">
                <span className="flex items-center gap-1 text-xs text-carbon-textSub">
                  {t(label)}
                  <InfoBubble tip={t(tip)} />
                </span>
                <NumberSetting
                  label={t(label)}
                  value={draft[i]}
                  disabled={busy}
                  onCommit={(n) => void apply(latest.current.map((count, at) => (at === i ? n : count)) as KeepCounts)}
                />
              </div>
            ))}
          </div>
          <p className="text-xs text-carbon-textMuted">{t("storage.keep.zero")}</p>
        </div>
      )}

      <KeepTimeline counts={draft} backups={backups} />

      <Rows>
        <Deviations
          location={location}
          setting="retention"
          describe={(section) => summary(t, section.retention)}
          onFollow={follow}
          busy={busy}
        />
        {editable && asks && (
          <SettingRow label={t("storage.keep.ask")} hint={t("storage.keep.askHint")}>
            <Toggle hideLabel label={t("storage.keep.ask")} checked={ask} onChange={setAskKeepLess} />
          </SettingRow>
        )}
      </Rows>
    </Card>
  );
}
