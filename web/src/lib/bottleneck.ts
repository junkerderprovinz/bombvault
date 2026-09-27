import type { Bottleneck } from "./api";
import type { TranslationKey, useT } from "./i18n";

type T = ReturnType<typeof useT>["t"];

const DISK: Record<NonNullable<Bottleneck["role"]> | "", TranslationKey> = {
  "": "bottleneck.disk",
  source: "bottleneck.sourceDisk",
  target: "bottleneck.targetDisk",
  both: "bottleneck.bothDisk",
};

const KIND: Record<Exclude<Bottleneck["kind"], "disk">, TranslationKey> = {
  cpu: "bottleneck.cpu",
  cpulimit: "bottleneck.cpuLimit",
  upload: "bottleneck.upload",
};

/** bottleneckText is the line a slow run carries: that it was slow, and why. */
export function bottleneckText(b: Bottleneck, t: T): string {
  const key: TranslationKey =
    b.kind === "disk" ? DISK[b.role ?? ""] : KIND[b.kind];
  const cause = t(key)
    .replace("{name}", b.name ?? "")
    .replace("{pct}", String(Math.round(b.share * 100)));
  return `${t("bottleneck.slow")} ${cause}`;
}
