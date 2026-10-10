import type { DomainStatus } from "../../lib/api";
import type { useT } from "../../lib/i18n";

// chipForRpo maps an RPO status to a statusTone/Badge color variant.
export function chipForRpo(status: string): string {
  switch (status) {
    case "ok":
      return "success";
    case "warn":
      return "info";
    case "overdue":
    case "never":
      return "failed";
    default:
      return "neutral";
  }
}

/** Worst RPO status across enabled, non-off domains: any overdue/never is red,
 *  else any warn is amber, else any ok is green, else all off = neutral. The
 *  summary tier's "Overall health" cell and the mobile repo-health card's
 *  four-status line are the two consumers. */
export function worstRpoStatus(domains: DomainStatus[]): "overdue" | "warn" | "ok" | "off" {
  const active = domains.filter((d) => d.enabled && d.status !== "off");
  return active.some((d) => d.status === "overdue" || d.status === "never")
    ? "overdue"
    : active.some((d) => d.status === "warn")
      ? "warn"
      : active.some((d) => d.status === "ok")
        ? "ok"
        : "off";
}

/** The reader-facing label for worstRpoStatus. */
export function worstRpoLabel(t: ReturnType<typeof useT>["t"], health: "overdue" | "warn" | "ok" | "off"): string {
  return health === "overdue"
    ? t("dashboard.rpoOverdue")
    : health === "warn"
      ? t("dashboard.rpoWarn")
      : health === "ok"
        ? t("dashboard.rpoOk")
        : t("dashboard.rpoOff");
}
