import type { ReactNode } from "react";
import type { CatalogEntry, TileMark } from "@glimstone/core";
import { STORAGE_PROVIDERS, type StorageGroup, type StorageProviderId } from "@glimstone/core/storageProviders";
import { BrandMark } from "@glimstone/react/ReadmeButton";
import { textInkOn } from "../../lib/accent";
import type { Provider } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { IconLink, IconShieldOn, IconUpload } from "../glyphs";
import { IconCloud, IconDatabase, IconFolder, IconLocal, IconTabSystem } from "../navGlyphs";

const ENTRIES = new Map(STORAGE_PROVIDERS.entries.map((e) => [e.id as string, e]));

/** A provider's entry in GlimStone's storage catalog, by the id BombVault stores. */
export function storageEntry(id: string): CatalogEntry<StorageGroup, StorageProviderId> | undefined {
  return ENTRIES.get(id);
}

const GLYPHS: Record<string, () => ReactNode> = {
  IconDatabase: () => <IconDatabase />,
  IconFolder: () => <IconFolder />,
  IconLink: () => <IconLink />,
  IconLocal: () => <IconLocal />,
  IconServer: () => <IconTabSystem />,
  IconShieldOn: () => <IconShieldOn />,
  IconUpload: () => <IconUpload />,
};

/** Our glyph for one the catalog names, a cloud where we have none like it. */
export function storageGlyph(name: string | undefined): ReactNode {
  return (GLYPHS[name ?? ""] ?? (() => <IconCloud />))();
}

// The catalog keeps no mark for an rclone.conf of one's own, which only
// BombVault offers. Rclone's mark, from its own logo.
export const RCLONE_MARK: TileMark = {
  svg: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0.01 13.26 512 485.36"><path d="M474-312.2c-18.4-22.4-45.1-39.3-75.8-49.8-3.9 14.8-11.4 29.3-22.6 43l-17.8 21.7c11.8 4.6 22.1 11.4 29.4 20.3 21.3 26 8.7 59.3-28.3 74.3s-84.2 6.1-105.6-19.9l-35.8-43.6h-57.9l-28.9 35.3 35.9 43.6c49 59.8 157.5 80.2 242.4 45.7 84.9-34.4 114-110.8 65-170.6" fill="var(--mark-ink, #70caf2)" transform="matrix(.8819 0 0 1.25276 73.007 655.001)" /><path d="M317.6-468.4c-84.9-34.5-193.4-14-242.4 45.7-18.4 22.4-25.8 47.1-23.4 71.1 20.1-5 41.7-7.7 64.3-7.7h35.6c-.2-9.5 3-19.2 10.3-28.1 21.4-26 68.6-34.9 105.6-19.9s49.6 48.3 28.3 74.3L260-289.3l28.9 35.3h57.9l35.7-43.7c49.1-59.7 20-136.2-64.9-170.7" fill="var(--mark-ink, light-dark(#4aa8dc, #b4e3f9))" transform="matrix(.8819 0 0 1.25276 54.168 621.044)" /><path d="M190.9-153.9 173-175.6c-11.6 4.9-25.1 7.8-39.7 7.8-42.7 0-77.3-24.4-77.3-54.4 0-30.1 34.6-54.4 77.3-54.4h71.6l28.9-35.3-28.8-35.3h-71.6c-98 0-177.5 55.9-177.5 124.9S35.4-97.4 133.4-97.4c36.7 0 70.8-7.8 99.1-21.3-16.2-9.6-30.4-21.4-41.6-35.2" fill="var(--mark-ink, #3f79ad)" transform="matrix(.8819 0 0 1.25276 38.903 620.511)" /></svg>',
  tile: "#3f79ad",
  ink: textInkOn("#3f79ad"),
};

/** The mark a provider is drawn with: its brand's, or the glyph of its kind. */
export function ProviderMark({ id }: { id: string }) {
  const entry = storageEntry(id);
  if (entry?.mark) return <BrandMark svg={entry.mark.svg} />;
  return <>{storageGlyph(entry?.glyph)}</>;
}

const GENERIC: Record<string, TranslationKey> = { path: "dest.provider.path", s3: "dest.provider.s3", smb: "dest.provider.smb" };

/** A provider's name: its own for a product, translated for a generic kind. */
export function providerName(p: Pick<Provider, "id" | "name">, t: ReturnType<typeof useT>["t"]): string {
  return GENERIC[p.id] ? t(GENERIC[p.id]) : p.name;
}
