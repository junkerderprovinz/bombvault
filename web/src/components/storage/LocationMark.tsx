import type { CSSProperties } from "react";
import type { StorageLocation } from "../../lib/api";
import { PROVIDER_TILES } from "../../lib/providerMarks";
import { ProviderMark } from "../destinations/ProviderPicker";
import { IconLocal } from "../navGlyphs";

const SIZES = {
  row: "h-10 w-10 [&_svg]:h-5 [&_svg]:w-5",
  head: "h-11 w-11 [&_svg]:h-6 [&_svg]:w-6",
  scene: "h-[22px] w-[22px] [&_svg]:h-3.5 [&_svg]:w-3.5",
} as const;

/**
 * LocationMark is a storage location's tile, like an app icon: the provider's
 * mark on its brand colour, and the accent for a folder on this host.
 */
export function LocationMark({
  location,
  size = "row",
}: {
  location: Pick<StorageLocation, "mark" | "kind">;
  size?: keyof typeof SIZES;
}) {
  const brand = location.mark ? PROVIDER_TILES[location.mark] : undefined;
  const local = !location.mark && location.kind === "local";
  const tile = brand?.tile ?? (local ? "var(--accent)" : "var(--carbon-surface3)");
  const ink = brand?.ink ?? (local ? "var(--accent-contrast)" : "var(--carbon-text)");
  return (
    <span
      aria-hidden="true"
      className={`inline-flex flex-none items-center justify-center rounded-control ${SIZES[size]}`}
      style={{ background: tile, color: ink, "--mark-ink": ink, "--mark-cut": "transparent" } as CSSProperties}
    >
      {local ? <IconLocal /> : <ProviderMark provider={location} />}
    </span>
  );
}
