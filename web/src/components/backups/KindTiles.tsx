import type { CSSProperties } from "react";
import { hueVars } from "../../lib/appearance";
import type { Tile } from "../../lib/backupList";
import type { TranslationKey } from "../../lib/i18n";
import { TileGlyph, tileHue, tileLabel } from "./kinds";

type T = (key: TranslationKey, n?: number) => string;

/**
 * KindTiles choose what the list shows: everything, one kind, or what is no
 * longer installed. Each says how many entries it stands for. Below 600px
 * they are one row of chips, so the list starts near the top of a phone.
 */
export function KindTiles({
  tiles,
  chosen,
  onChoose,
  t,
}: {
  tiles: { tile: Tile; count: number }[];
  chosen: Tile;
  onChoose: (tile: Tile) => void;
  t: T;
}) {
  return (
    <div role="group" aria-label={t("backups.tiles")} className="rounded-card bg-carbon-surface p-3 max-[600px]:p-2">
      <div className="flex flex-wrap gap-2 max-[860px]:grid max-[860px]:grid-cols-2 max-[600px]:flex max-[600px]:flex-nowrap max-[600px]:gap-1.5 max-[600px]:overflow-x-auto max-[600px]:[scrollbar-width:none]">
        {tiles.map(({ tile, count }) => {
          const on = tile === chosen;
          return (
            <button
              key={tile}
              type="button"
              aria-pressed={on}
              onClick={() => onChoose(tile)}
              style={hueVars(tileHue(tile)) as CSSProperties}
              className={`glim-hue glim-hue-icon grid min-w-max flex-1 grid-cols-[auto_minmax(0,1fr)] items-center gap-x-2.5 rounded-card px-3 py-2 text-start transition-colors max-[860px]:min-w-0 max-[600px]:flex max-[600px]:flex-none max-[600px]:gap-x-1.5 max-[600px]:rounded-pill max-[600px]:py-1.5 ${
                on
                  ? "glim-active bg-accent text-accentContrast"
                  : "bg-carbon-surface2 text-carbon-text hover:bg-carbon-surface3"
              }`}
            >
              <span className="row-span-2 flex [&_svg]:h-[22px] [&_svg]:w-[22px] max-[600px]:[&_svg]:h-4 max-[600px]:[&_svg]:w-4">
                <TileGlyph tile={tile} />
              </span>
              <span className="truncate text-sm font-semibold">{tileLabel(tile, t)}</span>
              <span className="truncate text-xs tabular-nums opacity-75 max-[600px]:hidden">
                {t("backups.tile.count", count)}
              </span>
              <span className="hidden text-xs font-semibold tabular-nums opacity-70 max-[600px]:inline">
                {count.toLocaleString()}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
