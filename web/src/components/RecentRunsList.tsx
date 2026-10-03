import { useEffect, useState, type ReactNode } from "react";

import { listRuns } from "../lib/api";
import type { Run } from "../lib/api";
import type { useT } from "../lib/i18n";
import { formatTs, formatDuration } from "../lib/reltime";
import { useOpenAnomalies } from "../lib/useAnomalies";
import { RunAnomalyBadge } from "./RunAnomalyBadge";
import { bottleneckText } from "../lib/bottleneck";
import { runKindLabel } from "../lib/runKind";

type T = ReturnType<typeof useT>["t"];

// A quick look at recent runs; the full log is on the dashboard.
const MAX_RUNS = 8;

// An import leaves a restore point just as a backup does.
const LISTED_KINDS = new Set(["backup", "import"]);

// statusDotClass maps a run status to a dot colour. Running uses bg-accentText
// because the plain accent measures 1.61:1 in the light theme, under the 3:1
// WCAG 1.4.11 asks of a status indicator. index.css does not rebind
// accentText inside .glim-hue rows, so the dot keeps that contrast and stays a
// status colour: in a resting reactive-rainbow row the rebound accent is the
// same grey as the "skipped" dot.
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
 * RecentRunsList shows one target's latest backups and imports with their
 * start and end time and duration. It fetches the run log when it mounts and
 * again whenever `refreshKey` changes, and filters it by domain and name. With
 * `renderDetail` each row opens to what that run did, which is where a domain
 * with per-run detail of its own puts it.
 */
export function RecentRunsList({
  name,
  domain,
  t,
  renderDetail,
  refreshKey,
}: {
  name: string;
  domain: "container" | "vm" | "files" | "zfs";
  t: T;
  renderDetail?: (run: Run) => ReactNode;
  /** Changes when a run of this target starts or ends. */
  refreshKey?: string;
}) {
  const [runs, setRuns] = useState<Run[]>([]);
  const [loading, setLoading] = useState(true);
  const [openRun, setOpenRun] = useState("");
  const { byRunId } = useOpenAnomalies();

  useEffect(() => {
    let alive = true;
    listRuns()
      .then((res) => {
        if (!alive || !res.ok) return;
        setRuns(
          (res.runs ?? [])
            .filter((r) => LISTED_KINDS.has(r.kind) && r.domain === domain && r.target === name)
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
  }, [name, domain, refreshKey]);

  if (loading) {
    return <p className="py-2 text-caption text-carbon-textMuted">{t("common.loadingBackups")}</p>;
  }
  if (runs.length === 0) return null;

  return (
    <div className="py-2 border-b border-carbon-border flex flex-col gap-1">
      <p className="text-caption uppercase tracking-wide text-carbon-textMuted">
        {t("run.recentTitle")}
      </p>
      {runs.map((run) => {
        const dur = run.finishedAt != null ? formatDuration(run.finishedAt - run.startedAt) : "";
        const open = openRun === run.id;
        const line = (
          <>
            <span className={`w-1.5 h-1.5 rounded-full shrink-0 ${statusDotClass(run.status)}`} />
            {/* On a phone the range breaks at the arrow, never inside a time. */}
            <span className="text-carbon-textSub whitespace-nowrap max-md:whitespace-normal">
              <span className="whitespace-nowrap">{formatTs(run.startedAt)}</span>
              {run.finishedAt != null && (
                <>
                  {" "}
                  <span className="inline-block rtl:-scale-x-100">→</span>{" "}
                  <span className="whitespace-nowrap">{formatTs(run.finishedAt)}</span>
                </>
              )}
            </span>
            {dur && <span className="text-carbon-textMuted whitespace-nowrap">({dur})</span>}
            {run.kind === "import" && <span className="text-carbon-textMuted">{runKindLabel(t, run.kind)}</span>}
            {/* The grey dot alone reads as a failure. */}
            {run.status === "cancelled" && <span className="text-carbon-textMuted">{t("run.statusCancelled")}</span>}
          </>
        );
        const badge = <RunAnomalyBadge findings={byRunId.get(run.id)} t={t} />;
        const slow = run.bottleneck && (
          <p className="ps-3.5 text-caption text-carbon-textSub">{bottleneckText(run.bottleneck, t)}</p>
        );
        if (!renderDetail) {
          return (
            <div key={run.id} className="flex flex-col gap-0.5">
              <div className="flex items-center gap-2 text-caption">
                {line}
                {badge}
              </div>
              {slow}
            </div>
          );
        }
        // The badge carries its own bubble button, so it sits beside the row's
        // toggle and not inside it.
        return (
          <div key={run.id} className="flex flex-col gap-1">
            <div className="flex items-center gap-2">
              <button
                type="button"
                aria-expanded={open}
                onClick={() => setOpenRun(open ? "" : run.id)}
                className="flex items-center gap-2 text-caption text-start hover:text-carbon-text pointer-coarse:min-h-11"
              >
                {line}
              </button>
              {badge}
            </div>
            {slow}
            {open && renderDetail(run)}
          </div>
        );
      })}
    </div>
  );
}
