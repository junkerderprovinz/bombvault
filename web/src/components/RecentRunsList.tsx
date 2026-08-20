import { useEffect, useState } from "react";

import { listRuns } from "../lib/api";
import type { Run } from "../lib/api";
import type { useT } from "../lib/i18n";
import { formatTs, formatDuration } from "../lib/reltime";

type T = ReturnType<typeof useT>["t"];

// Cap the per-target list — the disclosure is a quick "how have my recent
// backups gone" glance, not the full log (that lives in the dashboard).
const MAX_RUNS = 8;

// statusDotClass maps a run status to a small coloured dot, matching the
// dashboard's Badge palette (green success, red fail, amber running).
//
// "running" (Task 7: resolve the fifth hue) — was bg-statusInfoSolid, the
// old fifth hue. Genuine activity, a solid dot to match its ok/failed/
// skipped siblings in this same function (all solid dots, so a lone
// outline/spinner treatment here would look inconsistent). This list is
// scoped to ONE target's history (filtered by domain+name), so in practice
// at most one dot is ever "running" at a time — a solid dot doesn't
// compete with rule 3's "at most one solid accent" the way a repeated
// solid accent BUTTON would. bg-accentText, not the flat bg-accent: a
// spec-compliance review measured the flat accent gold at 1.61:1 in light
// theme here (was ~5.2:1 as bg-statusInfoSolid before this task) — badly
// under SC 1.4.11's 3:1 non-text-indicator minimum. See index.css's
// --accent-text comment for the fix and the measured numbers.
//
// NOTE for whoever next touches rainbow/hue rows (Task 8 territory): this
// component sometimes renders inside a .glim-hue row (via RestorePanel/
// FileSetRestorePanel), but --color-accentText is deliberately NOT
// redefined by index.css's [data-rainbow] .glim-hue block the way
// --color-accent is — so this dot always shows the fixed, contrast-verified
// global accent-text colour, never a rainbow row's own hue, even inside a
// hue-enabled subtree. That's on purpose, not an oversight: individual
// rainbow palette hues aren't WCAG-verified the way --accent-text is (this
// task only fixed the DEFAULT accent's contrast), so a genuine
// activity/status indicator keeping guaranteed contrast wins over matching
// its row's identity colour. If a future task WCAG-verifies the full
// rainbow palette, revisit whether this should follow the hue after all.
function statusDotClass(status: string): string {
  switch (status.toLowerCase()) {
    case "success":
      return "bg-statusOkSolid";
    case "failed":
      return "bg-statusFailSolid";
    case "running":
      return "bg-accentText";
    case "skipped":
      return "bg-statusNeutralSolid";
    default:
      return "bg-carbon-surface3";
  }
}

/**
 * RecentRunsList shows the most recent backup runs for one container/VM with
 * their start → end time and duration (issue #50), so the per-run timing is
 * visible right on the domain page, not only in the dashboard run history. It
 * fetches the global run log once and filters to this target by domain + name.
 */
export function RecentRunsList({
  name,
  domain,
  t,
}: {
  name: string;
  domain: "container" | "vm" | "files";
  t: T;
}) {
  const [runs, setRuns] = useState<Run[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let alive = true;
    listRuns()
      .then((res) => {
        if (!alive || !res.ok) return;
        setRuns(
          (res.runs ?? [])
            .filter((r) => r.kind === "backup" && r.domain === domain && r.target === name)
            .slice(0, MAX_RUNS),
        );
      })
      .catch(() => undefined)
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [name, domain]);

  if (loading) {
    return <p className="py-2 text-[11px] text-carbon-textMuted">{t("common.loadingBackups")}</p>;
  }
  if (runs.length === 0) return null;

  return (
    <div className="py-2 border-b border-carbon-border flex flex-col gap-1">
      <p className="text-[11px] uppercase tracking-wide text-carbon-textMuted">
        {t("run.recentTitle")}
      </p>
      {runs.map((run) => {
        const dur = run.finishedAt != null ? formatDuration(run.finishedAt - run.startedAt) : "";
        return (
          <div key={run.id} className="flex items-center gap-2 text-[11px]">
            <span className={`w-1.5 h-1.5 rounded-full shrink-0 ${statusDotClass(run.status)}`} />
            <span className="text-carbon-textSub whitespace-nowrap">
              {formatTs(run.startedAt)}
              {run.finishedAt != null && (
                <>
                  {" "}
                  <span className="inline-block rtl:-scale-x-100">→</span> {formatTs(run.finishedAt)}
                </>
              )}
            </span>
            {dur && <span className="text-carbon-textMuted whitespace-nowrap">({dur})</span>}
          </div>
        );
      })}
    </div>
  );
}
