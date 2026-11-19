import type { CSSProperties } from "react";
import type { StorageLocation } from "../../lib/api";
import { ProviderMark, storageEntry } from "../destinations/providers";
import { IconCloud, IconLocal } from "../navGlyphs";

const SIZES = {
  row: "h-10 w-10 [&_svg]:h-5 [&_svg]:w-5",
  head: "h-11 w-11 [&_svg]:h-6 [&_svg]:w-6",
  scene: "h-[22px] w-[22px] [&_svg]:h-3.5 [&_svg]:w-3.5",
  chip: "h-[18px] w-[18px] [&_svg]:h-3 [&_svg]:w-3",
} as const;

/**
 * LocationMark is a storage location's tile, like an app icon: the provider's
 * mark on its brand colour, and the accent for a folder on this host. A bare
 * mark has no tile: a brand keeps its own colours and every other glyph takes
 * the colour of the text around it.
 */
export function LocationMark({
  location,
  size = "row",
  bare = false,
}: {
  location: Pick<StorageLocation, "provider" | "kind">;
  size?: keyof typeof SIZES;
  bare?: boolean;
}) {
  const brand = storageEntry(location.provider)?.mark;
  const local = !location.provider && location.kind === "local";
  const tile = brand?.tile ?? (local ? "var(--accent)" : "var(--carbon-surface3)");
  const ink = brand?.ink ?? (local ? "var(--accent-contrast)" : "var(--carbon-text)");
  return (
    <span
      aria-hidden="true"
      className={`inline-flex flex-none items-center justify-center rounded-control ${SIZES[size]}`}
      style={bare ? undefined : ({ background: tile, color: ink, "--mark-ink": ink, "--mark-cut": "transparent" } as CSSProperties)}
    >
      {local ? <IconLocal /> : location.provider ? <ProviderMark id={location.provider} /> : <IconCloud />}
    </span>
  );
}
