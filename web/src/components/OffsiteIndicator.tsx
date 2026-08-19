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
 * `restic copy` exposes no machine-readable progress (see internal/api/service.go's
 * copyToOffsite), so this shows an indeterminate sliding segment while a
 * replication is in flight (which domain is running) rather than a fabricated
 * percentage. Renders nothing when no replication is active.
 *
 * The segment reuses ProgressBar's own `bv-indeterminate` keyframe/accent color
 * so it reads as the same "progress bar" motif as the determinate bars
 * elsewhere in the app (Containers/VMs/Files/Flash cards, the restore panel) -
 * a small inline shape rather than the full ProgressBar component, since
 * that component's two layouts (card-pinned track, or inline track with a
 * caption above it) don't fit a compact single status line like this one.
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
        className="relative h-1 w-5 overflow-hidden rounded-pill inline-block"
        style={{ background: "var(--carbon-border)" }}
      >
        <span
          className="absolute inset-y-0 w-1/3 rounded-pill"
          style={{ background: "var(--accent)", animation: "bv-indeterminate 1.2s ease-in-out infinite" }}
        />
      </span>
      ↗ {label}{t("offsite.replicating")}
    </span>
  );
}
