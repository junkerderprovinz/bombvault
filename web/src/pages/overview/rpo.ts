import type { DomainStatus } from "../../lib/api";
import type { TranslationKey, useT } from "../../lib/i18n";

export type RpoHealth = "overdue" | "warn" | "never" | "ok" | "off";

const RPO_LABEL: Record<RpoHealth, TranslationKey> = {
  overdue: "dashboard.rpoOverdue",
  warn: "dashboard.rpoWarn",
  never: "dashboard.rpoNever",
  ok: "dashboard.rpoOk",
  off: "dashboard.rpoOff",
};

// chipForRpo maps an RPO status to a statusTone/Badge color variant.
export function chipForRpo(status: string): string {
  switch (status) {
    case "ok":
      return "success";
    case "warn":
      return "info";
    case "overdue":
      return "failed";
    default:
      return "neutral";
  }
}

/** Worst RPO status across enabled, non-off domains: any overdue is red, else
 *  any warn is amber, else a section waiting for its first run is neutral, else
 *  any ok is green, else all off = neutral. The
 *  summary tier's "Overall health" cell and the mobile repo-health card's
 *  four-status line are the two consumers. */
export function worstRpoStatus(domains: DomainStatus[]): RpoHealth {
  const active = domains.filter((d) => d.enabled && d.status !== "off");
  const has = (status: RpoHealth) => active.some((d) => d.status === status);
  return (["overdue", "warn", "never", "ok"] as const).find(has) ?? "off";
}

/** The reader-facing label for worstRpoStatus. */
export function worstRpoLabel(t: ReturnType<typeof useT>["t"], health: RpoHealth): string {
  return t(RPO_LABEL[health]);
}
