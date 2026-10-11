import type { CSSProperties, ReactNode } from "react";
import { hueVars } from "../../lib/appearance";
import type { Tile } from "../../lib/backupList";
import type { TranslationKey } from "../../lib/i18n";
import { TileGlyph, tileHue, tileLabel } from "./kinds";

type T = (key: TranslationKey, n?: number) => string;

/** The box a row of tiles stands in. Below 600px the tiles of a list are one
 *  row of chips, so what follows starts near the top of a phone; with `chips`
 *  off they stay tiles, two to a row. `newId` is the change in
 *  lib/whatsNew.ts the row is. */
export function TileRow({
  label,
  newId,
  chips = true,
  children,
}: {
  label: string;
  newId?: string;
  chips?: boolean;
  children: ReactNode;
}) {
  return (
    <div role="group" aria-label={label} data-new={newId} className="rounded-card bg-carbon-surface p-3 max-[600px]:p-2">
      <div
        className={`flex flex-wrap gap-2 max-[860px]:grid max-[860px]:grid-cols-2 ${
          chips ? "max-[600px]:flex max-[600px]:flex-nowrap max-[600px]:gap-1.5 max-[600px]:overflow-x-auto max-[600px]:[scrollbar-width:none]" : ""
        }`}
      >
        {children}
      </div>
    </div>
  );
}

/** One tile: a glyph, what it stands for and how much of it there is. */
export function KindTile({
  on,
  hue,
  glyph,
  label,
  sub,
  count,
  chip = true,
  onClick,
}: {
  on: boolean;
  hue: number;
  glyph: ReactNode;
  label: string;
  /** The count in words, which a chip has no room for. */
  sub: string;
  count: number;
  /** Whether the tile turns into a chip on a phone, as in its TileRow. */
  chip?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-pressed={on}
      onClick={onClick}
      style={hueVars(hue) as CSSProperties}
      className={`glim-hue glim-hue-icon grid min-w-max flex-1 grid-cols-[auto_minmax(0,1fr)] items-center gap-x-2.5 rounded-card px-3 py-2 text-start transition-colors max-[860px]:min-w-0 ${
        chip ? "max-[600px]:flex max-[600px]:flex-none max-[600px]:gap-x-1.5 max-[600px]:rounded-pill max-[600px]:py-1.5" : ""
      } ${on ? "glim-active bg-accent text-accentContrast" : "bg-carbon-surface2 text-carbon-text hover:bg-carbon-surface3"}`}
    >
      <span className={`row-span-2 flex [&_svg]:h-[22px] [&_svg]:w-[22px] ${chip ? "max-[600px]:[&_svg]:h-4 max-[600px]:[&_svg]:w-4" : ""}`}>
        {glyph}
      </span>
      <span className="truncate text-sm font-semibold">{label}</span>
      <span className={`truncate text-xs tabular-nums opacity-75 ${chip ? "max-[600px]:hidden" : ""}`}>{sub}</span>
      {chip && <span className="hidden text-xs font-semibold tabular-nums opacity-70 max-[600px]:inline">{count.toLocaleString()}</span>}
    </button>
  );
}

/**
 * KindTiles choose what the list shows: everything, one kind, or what is no
 * longer installed. Each says how many entries it stands for.
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
    <TileRow label={t("backups.tiles")} newId="backups-tiles">
      {tiles.map(({ tile, count }) => (
        <KindTile
          key={tile}
          on={tile === chosen}
          hue={tileHue(tile)}
          glyph={<TileGlyph tile={tile} />}
          label={tileLabel(tile, t)}
          sub={t("backups.tile.count", count)}
          count={count}
          onClick={() => onChoose(tile)}
        />
      ))}
    </TileRow>
  );
}
