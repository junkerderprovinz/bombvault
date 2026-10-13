import type { ReactNode } from "react";
import type { TranslationKey } from "../../lib/i18n";
import { useIsDesktop } from "../../lib/useMediaQuery";
import { Button } from "../Button";
import { IconCheck } from "../navGlyphs";
import { SelectMark } from "./SelectMark";

type T = (key: TranslationKey, n?: number) => string;

// The bar is a filled accent surface, so its buttons are a shade of it and
// not a second fill.
const ON_BAR = "bg-black/15! text-accentContrast! hover:bg-black/25!";

/** BarButton is one action of the bar. A phone has room for its glyph only. */
export function BarButton({
  label,
  glyph,
  onClick,
  disabled,
  title,
  wide = false,
}: {
  label: string;
  glyph: ReactNode;
  onClick: () => void;
  disabled?: boolean;
  title?: string;
  /** Keeps its words on a phone too. */
  wide?: boolean;
}) {
  const compact = !useIsDesktop() && !wide;
  return (
    <Button
      label={label}
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
 * BulkBar stands above a list while rows are being picked: how many are
 * picked, what can be done with them, and the way out. It stays in view while
 * the list scrolls under it.
 */
export function BulkBar({
  count,
  allPicked,
  onAll,
  onDone,
  children,
  t,
}: {
  count: number;
  allPicked: boolean;
  onAll: (pick: boolean) => void;
  onDone: () => void;
  /** The actions for the picked rows, shown once one is picked. */
  children: ReactNode;
  t: T;
}) {
  const isDesktop = useIsDesktop();
  return (
    <div
      data-testid="bulk-bar"
      className="sticky top-0 z-[3] flex flex-wrap items-center gap-2.5 rounded-card bg-accent py-2 pe-2.5 ps-4 text-sm font-semibold text-accentContrast"
    >
      <SelectMark checked={allPicked} label={t("containers.selectAll")} onChange={onAll} onAccent />
      <span className="min-w-0 flex-1">
        {count > 0 ? t("zfs.add.selected", count) : t(isDesktop ? "backups.select.hint" : "backups.select.hintTouch")}
      </span>
      {count > 0 && children}
      <BarButton label={t("common.done")} glyph={<IconCheck />} onClick={onDone} wide />
    </div>
  );
}
