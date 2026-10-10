import type { ReactNode } from "react";
import type { BackupItem } from "../../lib/api";
import type { EntryKind } from "../../lib/backupEntry";
import { NOT_INSTALLED, TILE_KINDS, type Tile } from "../../lib/backupList";
import type { TranslationKey } from "../../lib/i18n";
import { IconArchive, IconClearSelection } from "../glyphs";
import { IconConfig, IconContainers, IconFlash, IconFolder, IconVM, IconZFS } from "../navGlyphs";

type T = (key: TranslationKey, n?: number) => string;

export const KIND_LABEL: Record<EntryKind, TranslationKey> = {
  container: "nav.containers",
  vm: "nav.vms",
  flash: "nav.flash",
  files: "nav.files",
  zfs: "nav.zfs",
  config: "nav.config",
};

/** The title of each kind's schedule card, which the schedule sentence names. */
export const KIND_SCHEDULE_LABEL: Record<EntryKind, TranslationKey> = {
  container: "jobs.containersSection",
  vm: "jobs.vmsSection",
  flash: "jobs.flashSection",
  files: "jobs.filesSection",
  zfs: "jobs.zfsSection",
  config: "nav.config",
};

const KIND_GLYPH: Record<EntryKind, () => ReactNode> = {
  container: IconContainers,
  vm: IconVM,
  flash: IconFlash,
  files: IconFolder,
  zfs: IconZFS,
  config: IconConfig,
};

export function KindGlyph({ kind }: { kind: EntryKind }) {
  const Glyph = KIND_GLYPH[kind];
  return <Glyph />;
}

export function TileGlyph({ tile }: { tile: Tile }) {
  if (tile === "all") return <IconArchive />;
  if (tile === NOT_INSTALLED) return <IconClearSelection />;
  return <KindGlyph kind={tile} />;
}

export function tileLabel(tile: Tile, t: T): string {
  if (tile === "all") return t("filter.all");
  if (tile === NOT_INSTALLED) return t("containers.notInstalled");
  return t(KIND_LABEL[tile]);
}

// A kind keeps one palette position, so its tile and its rows share a colour
// whichever tiles are on the page.
const TILE_ORDER: readonly Tile[] = ["all", ...TILE_KINDS, NOT_INSTALLED];

export function tileHue(tile: Tile): number {
  return TILE_ORDER.indexOf(tile);
}

export function kindHue(kind: EntryKind): number {
  const at = TILE_ORDER.indexOf(kind as Tile);
  return at < 0 ? TILE_ORDER.length : at;
}

/** The flash drive and the self-backup arrive named by their key. */
export function entryName(item: BackupItem, t: T): string {
  if (item.kind === "flash") return t("backups.name.flash");
  if (item.kind === "config") return t("backups.name.config");
  return item.name;
}
