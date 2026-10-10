import { useState } from "react";
import type { Run, ZFSDatasetView } from "../../lib/api";
import { bottleneckText } from "../../lib/bottleneck";
import { humanBytes } from "../../lib/forecast";
import { useT } from "../../lib/i18n";
import { formatClockTime, formatDuration, formatRecent } from "../../lib/reltime";
import { runKindLabel } from "../../lib/runKind";
import { useOpenAnomalies } from "../../lib/useAnomalies";
import { AnomalyWindow } from "../anomalies/AnomalyWindow";
import { Badge } from "../Badge";
import { RunDetailSheet } from "../mobile/RunDetailSheet";
import { statusDotClass } from "../RecentRunsList";
import { RunAnomalyBadge } from "../RunAnomalyBadge";
import { ZFSRunDetailView } from "../zfs/ZFSDatasetRow";

const SHOWN = 8;

// An import leaves a restore point just as a backup does.
const LISTED = new Set(["backup", "import"]);

/** From when to when a run went, or since when it has been going. */
function useSpan(): (run: Run) => string {
  const { t, lang } = useT();
  return (run) => {
    const start = formatRecent(run.startedAt, lang);
    if (run.finishedAt == null) return start;
    return t("entry.runs.span").replace("{start}", start).replace("{end}", formatClockTime(run.finishedAt, false));
  };
}

/**
 * EntryRuns lists the entry's latest backups and imports with how long each
 * took and what it added. A run opens with everything recorded about it. A
 * ZFS run opens on what happened to each dataset of its tree.
 */
export function EntryRuns({ runs, dataset }: { runs: Run[]; dataset?: ZFSDatasetView }) {
  const { t } = useT();
  const { byRunId } = useOpenAnomalies();
  const span = useSpan();
  const [openId, setOpenId] = useState<string | null>(null);
  const listed = runs.filter((run) => LISTED.has(run.kind)).slice(0, SHOWN);
  const open = listed.find((run) => run.id === openId);

  if (listed.length === 0) return <p className="text-sm text-carbon-textMuted">{t("dashboard.noRuns")}</p>;

  return (
    <>
      <ul className="flex flex-col divide-y divide-carbon-border/60">
        {listed.map((run) => (
          <li key={run.id} className="flex items-center gap-2 py-1 first:pt-0 last:pb-0">
            <button
              type="button"
              data-testid="entry-run"
              onClick={() => setOpenId(run.id)}
              className="flex min-w-0 flex-1 items-center gap-3 rounded-control px-1.5 py-1.5 text-start text-sm hover:bg-carbon-hover pointer-coarse:min-h-11"
            >
              <span className={`h-2 w-2 flex-none rounded-full ${statusDotClass(run.status)}`} />
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
                  <span className="tabular-nums text-carbon-text first-letter:uppercase">{span(run)}</span>
                  {run.kind === "import" && (
                    <Badge tone="neutral" size="small">
                      {runKindLabel(t, run.kind)}
                    </Badge>
                  )}
                  {run.status === "failed" && (
                    <Badge tone="fail" size="small">
                      {t("run.statusFailed")}
                    </Badge>
                  )}
                  {run.status === "cancelled" && (
                    <Badge tone="neutral" size="small">
                      {t("run.statusCancelled")}
                    </Badge>
                  )}
                </span>
                {run.bottleneck && (
                  <span className="text-xs text-statusWarn">{bottleneckText(run.bottleneck, t)}</span>
                )}
              </span>
              {run.finishedAt != null && (
                <span className="flex-none text-xs tabular-nums text-carbon-textMuted">
                  {formatDuration(run.finishedAt - run.startedAt)}
                </span>
              )}
              <span className="w-20 flex-none text-end text-xs tabular-nums text-carbon-textMuted max-md:w-16">
                {run.snapshotId !== "" && humanBytes(run.bytes)}
              </span>
            </button>
            {/* The badge has a bubble button of its own, so it stands beside
                the row's button and not inside it. */}
            <RunAnomalyBadge findings={byRunId.get(run.id)} t={t} />
          </li>
        ))}
      </ul>
      {open &&
        (dataset ? (
          <AnomalyWindow title={span(open)} onClose={() => setOpenId(null)}>
            <ZFSRunDetailView run={open} item={dataset} t={t} />
          </AnomalyWindow>
        ) : (
          <RunDetailSheet run={open} open windowed onClose={() => setOpenId(null)} />
        ))}
    </>
  );
}
