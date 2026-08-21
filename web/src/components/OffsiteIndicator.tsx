import { useEffect, useState } from "react";
import { useProgress } from "../lib/progress";
import type { ProgressState } from "../lib/progress";
import { useT } from "../lib/i18n";
import type { TranslationKey } from "../lib/i18n";
import { elapsedSince } from "../lib/reltime";

type Domain = "containers" | "vms" | "flash" | "files";

// A flash/containers replication can finish in well under a second (small repo,
// already seeded), and the shared progress store only lingers ~0.8s. Latch the
// indicator visible for at least this long after it first goes active so a fast
// replication is still noticeable.
const MIN_VISIBLE_MS = 2500;

// How often the elapsed-duration readout re-renders while visible. Purely a
// local UI tick (Date.now()) — independent of how often the backend's own
// heartbeat (offsiteProgressHeartbeat, 5s) actually re-publishes an event, so
// the counter still reads as smoothly ticking between those pushes.
const ELAPSED_TICK_MS = 1000;

/**
 * offsiteStatusText picks the honest live status text for an "offsite:<domain>"
 * progress state (issue #159), in three tiers, most-informative first:
 *
 *   1. A live per-snapshot signal is available (state.snapshotIndex > 0, a
 *      real restic-copy percentage — see restic.Copy's doc comment): "Replicating
 *      snapshot {index} of {total} ({percent}%)". `total` is widened to at
 *      least `index` in case the backend's own upfront candidate-count
 *      ESTIMATE undercounted — the live index is always the ground truth,
 *      the total is only ever a best-effort guess (restic never reports one).
 *   2. Only a startedAt is known (no live percentage yet, e.g. restic is
 *      still walking the source tree): the plain elapsed-duration text.
 *   3. Neither is known yet: the bare "Replicating…" label.
 *
 * Each tier has its own "WithDuration" sibling key appending the SAME live
 * duration, so tier 1 never has to drop the duration just because a
 * percentage became available too. Exported (and framework-free beyond the
 * `t` callback) so it is unit-testable without mounting the component.
 */
export function offsiteStatusText(
  t: (key: TranslationKey) => string,
  state: ProgressState | undefined,
  duration: string
): string {
  const index = state?.snapshotIndex;
  const percent = state?.percent;
  if (typeof index === "number" && index > 0 && typeof percent === "number") {
    const total = Math.max(state?.snapshotTotal ?? 0, index);
    const pct = String(Math.round(Math.max(0, Math.min(100, percent))));
    const key: TranslationKey = duration ? "offsite.replicatingSnapshotPercentWithDuration" : "offsite.replicatingSnapshotPercent";
    return t(key)
      .replace("{index}", String(index))
      .replace("{total}", String(total))
      .replace("{percent}", pct)
      .replace("{duration}", duration);
  }
  return duration ? t("offsite.replicatingWithDuration").replace("{duration}", duration) : t("offsite.replicating");
}

/**
 * Active "off-site replication running" indicator for a domain. Renders
 * nothing when no replication is active.
 *
 * Issue #159 ("offsite upload duration — can we get a percentage counter?"):
 * a first cut of this feature concluded restic copy had no honest percentage
 * or even a reliable "N of M" to show, and shipped a duration-only readout.
 * That conclusion was wrong: verified against upstream cmd/restic/cmd_copy.go
 * and internal/ui/progress/terminal.go, and hands-on against the installed
 * restic 0.17.3 binary, restic copy DOES print real, parseable progress on
 * stdout — it just needed the same RESTIC_PROGRESS_FPS wiring backup/restore
 * already had (see restic.Copy's doc comment for the whole story). The one
 * part of the original investigation that WAS correct: restic copy reports
 * progress PER SOURCE SNAPSHOT, never a whole-run total across a
 * multi-snapshot batch — so the backend pairs each live per-snapshot
 * percentage with its own best-effort "of N" candidate count (see
 * api.copyToOffsiteTarget/restic.PendingCopyIDs).
 *
 * So the segment beneath the label stays an INDETERMINATE sliding bar (it
 * represents the whole domain replication, which — across a multi-snapshot
 * batch or a multiTarget loop — has no single completion fraction of its
 * own; a determinate fill here would misleadingly suggest otherwise), while
 * the TEXT shows the real, live numbers once available: "Replicating
 * snapshot 2 of 4 (63%)". Falls back to a live "(2m 14s)"-style elapsed
 * duration (from the backend-stamped StartedAt) whenever no live
 * per-snapshot signal has arrived yet (e.g. restic is still walking the
 * source tree), and further to the plain label when even startedAt isn't
 * known yet.
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
  const state = useProgress()["offsite:" + domain];
  const active = !!state?.active;
  const [visible, setVisible] = useState(false);
  const [now, setNow] = useState(() => Date.now());

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

  // Tick `now` once a second only while shown, so the elapsed readout counts up
  // smoothly without polling anything while the indicator is hidden.
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
      ↗ {label}{statusText}
    </span>
  );
}
