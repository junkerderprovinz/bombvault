import { useProgress } from "../lib/progress";
import { useT } from "../lib/i18n";

type Domain = "containers" | "vms" | "flash";

/**
 * Active (indeterminate) "off-site replication running" indicator for a domain.
 * `restic copy` exposes no machine-readable progress, so this shows an animated
 * pill while a replication is in flight (which domain is running) rather than a
 * filling percentage bar. Renders nothing when no replication is active.
 *
 * withLabel prefixes the domain name (used on the dashboard, where several
 * domains share one view); on a domain's own page the label is omitted.
 */
export function OffsiteIndicator({ domain, withLabel }: { domain: Domain; withLabel?: boolean }) {
  const { t } = useT();
  const p = useProgress()["offsite:" + domain];
  if (!p || !p.active) return null;
  const navKey = { containers: "nav.containers", vms: "nav.vms", flash: "nav.flash" } as const;
  const label = withLabel ? `${t(navKey[domain])} · ` : "";
  return (
    <span className="inline-flex items-center gap-1.5 text-xs text-carbon-textSub">
      <span
        className="h-2.5 w-2.5 rounded-full border-2 border-t-transparent animate-spin inline-block"
        style={{ borderColor: "var(--accent)", borderTopColor: "transparent" }}
      />
      ↗ {label}{t("offsite.replicating")}
    </span>
  );
}
