import { useEffect, useState } from "react";
import { offsiteRunProgress, useProgress } from "../lib/progress";
import type { ProgressState } from "../lib/progress";
import { useT } from "../lib/i18n";
import type { TranslationKey } from "../lib/i18n";
import { elapsedSince } from "../lib/reltime";
import { InfoBubble } from "./InfoBubble";

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
 *   1. A RUN-LEVEL percentage can be derived (offsiteRunProgress returns a
 *      value — see its doc comment): "Replicating… {percent}% overall
 *      (snapshot {index} of {total})". The percentage leads and owns the line;
 *      "snapshot k of N" is the parenthetical detail behind it, which is the
 *      reading order that cannot be misparsed. This used to render as
 *      "Replicating snapshot {index} of {total} ({percent}%)", where the
 *      percentage was restic's PER-SNAPSHOT pack progress sitting in
 *      parentheses right after an unrelated fraction — "15 of 126 (55%)" reads
 *      as "15/126 = 55%" to everyone, and contradicted itself.
 *   2. Only a startedAt is known (no live percentage yet — restic still
 *      walking the source tree, or no candidate-count estimate to divide by):
 *      the plain elapsed-duration text.
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
 * represents the whole domain replication across a multiTarget loop, which no
 * single fraction covers; a determinate fill would misleadingly suggest
 * otherwise), while the TEXT shows the real, live numbers once available:
 * "Replicating… 41% overall (snapshot 2 of 4)". That percentage is the
 * per-snapshot signal FOLDED INTO the snapshot count, not printed beside it —
 * the first cut showed "snapshot 2 of 4 (63%)", where the two correct-but
 * -differently-scoped numbers read as one contradictory claim (see
 * offsiteRunProgress in lib/progress.ts, and issue #159). Falls back to a live
 * "(2m 14s)"-style elapsed duration (from the backend-stamped StartedAt)
 * whenever no run-level percentage can be derived (restic still walking the
 * source tree, or no candidate-count estimate to divide by), and further to
 * the plain label when even startedAt isn't known yet.
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
  // The run-level percentage is snapshot-COUNT progress against a best-effort
  // estimate, not byte progress — house rule says that explanation belongs
  // behind an (i), never as permanent prose on the line. Only shown in the
  // tier that actually renders a percentage.
  const showsRunPercent = offsiteRunProgress(state) !== null;
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
      {showsRunPercent ? <InfoBubble tip={t("offsite.overallPercentHint")} /> : null}
    </span>
  );
}
