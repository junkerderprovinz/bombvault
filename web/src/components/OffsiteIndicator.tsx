import { useEffect, useState } from "react";
import { useProgress } from "../lib/progress";
import { useT } from "../lib/i18n";

type Domain = "containers" | "vms" | "flash" | "files";

// A flash/containers replication can finish in well under a second (small repo,
// already seeded), and the shared progress store only lingers ~0.8s. Latch the
// indicator visible for at least this long after it first goes active so a fast
// replication is still noticeable.
const MIN_VISIBLE_MS = 2500;

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
  const active = !!useProgress()["offsite:" + domain]?.active;
  const [visible, setVisible] = useState(false);

  // Show immediately when active; on the active→idle edge keep it up for a
  // minimum window so a near-instant replication doesn't just flash by.
  useEffect(() => {
    if (active) {
      setVisible(true);
      return;
    }
    const timer = setTimeout(() => setVisible(false), MIN_VISIBLE_MS);
    return () => clearTimeout(timer);
  }, [active]);

  if (!visible) return null;
  const navKey = { containers: "nav.containers", vms: "nav.vms", flash: "nav.flash", files: "nav.files" } as const;
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
