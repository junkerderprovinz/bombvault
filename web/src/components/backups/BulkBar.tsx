import type { ReactNode } from "react";
import type { TranslationKey } from "../../lib/i18n";
import { useIsDesktop } from "../../lib/useMediaQuery";
import { Button } from "../Button";
import { IconPause } from "../glyphs";
import { IconBackupNow, IconCheck, IconRestore, IconSchedules } from "../navGlyphs";
import { SelectMark } from "./SelectMark";

type T = (key: TranslationKey, n?: number) => string;

// The bar is a filled accent surface, so its buttons are a shade of it and
// not a second fill.
const ON_BAR = "bg-black/15! text-accentContrast! hover:bg-black/25!";

function BarButton({
  labelKey,
  glyph,
  onClick,
  disabled,
  title,
  compact,
  t,
}: {
  labelKey: TranslationKey;
  glyph: ReactNode;
  onClick: () => void;
  disabled?: boolean;
  title?: string;
  compact: boolean;
  t: T;
}) {
  return (
    <Button
      label={t(labelKey)}
      labelKey={null}
      glyph={glyph}
      tone="subtle"
      variant={compact ? "icon" : "default"}
      glyphOnly={compact}
      onClick={onClick}
      disabled={disabled}
      title={title}
      className={ON_BAR}
    />
  );
}

/**
 * BulkBar stands above the list while rows are being picked: how many are
 * picked, what can be done with them, and the way out. It stays in view while
 * the list scrolls under it.
 */
export function BulkBar({
  count,
  allPicked,
  onAll,
  onBackUp,
  onResume,
  onPause,
  onRestore,
  onDone,
  busy,
  blocked,
  t,
}: {
  count: number;
  allPicked: boolean;
  onAll: (pick: boolean) => void;
  onBackUp: () => void;
  onResume: () => void;
  onPause: () => void;
  /** Left out when no picked entry can be restored in bulk. */
  onRestore?: () => void;
  onDone: () => void;
  /** An action of this bar is under way. */
  busy: boolean;
  /** Why no backup or restore can start, when something else is running. */
  blocked?: string;
  t: T;
}) {
  const isDesktop = useIsDesktop();
  const compact = !isDesktop;
  return (
    <div
      data-testid="bulk-bar"
      className="sticky top-0 z-[3] flex flex-wrap items-center gap-2.5 rounded-card bg-accent py-2 pe-2.5 ps-4 text-sm font-semibold text-accentContrast"
    >
      <SelectMark checked={allPicked} label={t("containers.selectAll")} onChange={onAll} onAccent />
      <span className="min-w-0 flex-1">
        {count > 0 ? t("zfs.add.selected", count) : t(isDesktop ? "backups.select.hint" : "backups.select.hintTouch")}
      </span>
      {count > 0 && (
        <>
          <BarButton
            labelKey="containers.backupSelected"
            glyph={<IconBackupNow />}
            onClick={onBackUp}
            disabled={busy || !!blocked}
            title={blocked}
            compact={compact}
            t={t}
          />
          <BarButton labelKey="schedule.resume" glyph={<IconSchedules />} onClick={onResume} disabled={busy} compact={compact} t={t} />
          <BarButton labelKey="schedule.pause" glyph={<IconPause />} onClick={onPause} disabled={busy} compact={compact} t={t} />
          {onRestore && (
            <BarButton
              labelKey="backups.restoreNewest"
              glyph={<IconRestore />}
              onClick={onRestore}
              disabled={busy || !!blocked}
              title={blocked}
              compact={compact}
              t={t}
            />
          )}
        </>
      )}
      <BarButton labelKey="common.done" glyph={<IconCheck />} onClick={onDone} compact={false} t={t} />
    </div>
  );
}
