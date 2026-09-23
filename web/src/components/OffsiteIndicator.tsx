import { useEffect, useState } from "react";
import { offsiteRunProgress, useProgress } from "../lib/progress";
import type { ProgressState } from "../lib/progress";
import { useT } from "../lib/i18n";
import type { TranslationKey } from "../lib/i18n";
import { elapsedSince } from "../lib/reltime";
import { InfoBubble } from "./InfoBubble";

type Domain = "containers" | "vms" | "flash" | "files";

// A replication of an already seeded repo can finish in well under a second,
// and the progress store lingers only about 0.8s, so the indicator stays up
// this long after the replication ends.
const MIN_VISIBLE_MS = 2500;

// A local tick, so the elapsed time counts smoothly between the backend's 5s
// heartbeats.
const ELAPSED_TICK_MS = 1000;

/**
 * offsiteStatusText picks the live status text for an "offsite:<domain>"
 * progress state, most informative first:
 *
 *   1. A run-level percentage (see offsiteRunProgress): "Replicating… 41%
 *      overall (snapshot 2 of 4)". The percentage leads so it cannot be read
 *      as the ratio of the two snapshot numbers.
 *   2. Only startedAt is known: the elapsed duration.
 *   3. Neither: the bare "Replicating…" label.
 *
 * Each tier has a "WithDuration" key that appends the live duration.
 */
export function offsiteStatusText(
  t: (key: TranslationKey) => string,
  state: ProgressState | undefined,
  duration: string
): string {
  const run = offsiteRunProgress(state);
  if (run) {
    const key: TranslationKey = duration ? "offsite.replicatingSnapshotPercentWithDuration" : "offsite.replicatingSnapshotPercent";
    return t(key)
      .replace("{index}", String(run.index))
      .replace("{total}", String(run.total))
      .replace("{percent}", String(run.percent))
      .replace("{duration}", duration);
  }
  return duration ? t("offsite.replicatingWithDuration").replace("{duration}", duration) : t("offsite.replicating");
}

/**
 * The "off-site replication running" indicator for a domain. It renders
 * nothing while no replication is active.
 *
 * restic copy reports progress per source snapshot, never for the whole batch,
 * so the backend pairs each percentage with its own estimate of the snapshot
 * count (api.copyToOffsiteTarget, restic.PendingCopyIDs). The bar stays
 * indeterminate because it stands for the domain's whole replication across
 * all targets, which no single fraction covers; the numbers go in the text.
 *
 * The bar borrows ProgressBar's `glim-indeterminate` animation and accent
 * colour but not the component, whose two layouts do not fit a single status
 * line.
 *
 * withLabel prefixes the domain name, for the dashboard, where several domains
 * share one view.
 */
export function OffsiteIndicator({ domain, withLabel }: { domain: Domain; withLabel?: boolean }) {
  const { t } = useT();
  const state = useProgress()["offsite:" + domain];
  const active = !!state?.active;
  const [visible, setVisible] = useState(false);
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (active) {
      setVisible(true);
      return;
    }
    const timer = setTimeout(() => setVisible(false), MIN_VISIBLE_MS);
    return () => clearTimeout(timer);
  }, [active]);

  useEffect(() => {
    if (!visible) return;
    const id = setInterval(() => setNow(Date.now()), ELAPSED_TICK_MS);
    return () => clearInterval(id);
  }, [visible]);

  if (!visible) return null;
  const navKey = { containers: "nav.containers", vms: "nav.vms", flash: "nav.flash", files: "nav.files" } as const;
  const label = withLabel ? `${t(navKey[domain])} · ` : "";
  const duration = elapsedSince(state?.startedAt, now);
  const statusText = offsiteStatusText(t, state, duration);
  // The (i) says the percentage counts snapshots against an estimate, not
  // bytes, so it only appears when a percentage does.
  const showsRunPercent = offsiteRunProgress(state) !== null;
  return (
    <span className="inline-flex items-center gap-1.5 text-xs text-carbon-textSub">
      <span
        className="relative h-1 w-5 overflow-hidden rounded-pill inline-block"
        style={{ background: "var(--carbon-border)" }}
      >
        <span
          className="absolute inset-y-0 w-1/3 rounded-pill"
          style={{ background: "var(--accent)", animation: "glim-indeterminate 1.2s ease-in-out infinite" }}
        />
      </span>
      ↗ {label}{statusText}
      {showsRunPercent ? <InfoBubble tip={t("offsite.overallPercentHint")} /> : null}
    </span>
  );
}
