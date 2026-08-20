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
// solid accent BUTTON would.
function statusDotClass(status: string): string {
  switch (status.toLowerCase()) {
    case "success":
      return "bg-statusOkSolid";
    case "failed":
      return "bg-statusFailSolid";
    case "running":
      return "bg-accent";
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
