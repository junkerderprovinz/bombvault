import { useEffect, useRef, useState } from "react";
import type { RetentionKeep } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { setAskKeepLess, useAskKeepLess } from "../../lib/keepAsk";
import { KEEP_PRESETS, presetOf, type KeepCounts, type KeepPreset } from "../../lib/keepPlan";
import { keepCounts, retentionKeep } from "../../lib/storageLocations";
import { Card } from "../../pages/settings/shared";
import { Badge } from "../Badge";
import { Selector } from "../Selector";
import { Toggle } from "../Toggle";
import { KeepCountFields, useKeepLessQuestion } from "./keepFields";
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

type T = ReturnType<typeof useT>["t"];

/** A rule's five counts as one sentence. */
export function keepSummary(t: T, keep: RetentionKeep): string {
  return t("storage.keep.summary")
    .replace("{last}", String(keep.keepLast))
    .replace("{daily}", String(keep.keepDaily))
    .replace("{weekly}", String(keep.keepWeekly))
    .replace("{monthly}", String(keep.keepMonthly))
    .replace("{yearly}", String(keep.keepYearly));
}

/**
 * KeepCard sets how long a location keeps its backups: one of three presets or
 * five counts of its own, with a picture of what the rule keeps. The location
 * reports no backups of its own, so the picture is drawn over a sample history.
 */
export function KeepCard({ edit, retention, hueIndex }: { edit: LocationEdit; retention: RetentionKeep; hueIndex: number }) {
  const { t } = useT();
  const { location, can, save, follow, busy } = edit;
  const less = useKeepLessQuestion();
  const ask = useAskKeepLess();
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
    if (asks && !(await less.agreed(stored, next, location.name))) {
      revert();
      return;
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
      {less.dialog}
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
            {mode === "custom" ? keepSummary(t, retention) : t(`storage.keep.${mode}`)}
          </Badge>
        </div>
      )}

      {editable && custom && (
        <KeepCountFields
          counts={draft}
          disabled={busy}
          onCommit={(i, n) => void apply(latest.current.map((count, at) => (at === i ? n : count)) as KeepCounts)}
        />
      )}

      <KeepTimeline counts={draft} backups={less.backups} />

      <Rows>
        <Deviations
          location={location}
          setting="retention"
          describe={(section) => keepSummary(t, section.retention)}
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
