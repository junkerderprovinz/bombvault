// Where a finding's backup differs from the last good one: a table of the
// folders that lost, gained or changed files. Showing it is what asks the
// server to compare.

import { useEffect, useState } from "react";

import type { TranslateAnomaly } from "../../lib/anomalies";
import { getAnomalyChanges, type AnomalyChanges, type AnomalyView } from "../../lib/api";
import { humanBytes } from "../../lib/forecast";
import { isolateLtr } from "../../lib/ltrFragments";
import { formatTs } from "../../lib/reltime";
import { Button } from "../Button";
import { InfoBubble } from "../InfoBubble";

// How often the box asks again while the comparison runs.
const POLL_MS = 2000;

const COMPARABLE_METRICS = new Set([
  "source_bytes_shrink",
  "source_files_shrink",
  "source_bytes_growth",
  "new_data",
  "new_data_rewrite",
]);
const COMPARABLE_DOMAINS = new Set(["container", "vm", "files"]);

/** Whether the server can compare the backups behind a finding. */
export function findingHasChanges(a: AnomalyView): boolean {
  return a.scopeKind === "item" && COMPARABLE_DOMAINS.has(a.domain) && COMPARABLE_METRICS.has(a.metric);
}

export function FindingChanges({ a, t }: { a: AnomalyView; t: TranslateAnomaly }) {
  const [data, setData] = useState<AnomalyChanges | null>(null);
  // null while the request went through; otherwise the server's reason, empty
  // when the request itself failed.
  const [failure, setFailure] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);

  useEffect(() => {
    let alive = true;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const load = (again: boolean) => {
      getAnomalyChanges(a.id, again)
        .then((res) => {
          if (!alive) return;
          if (!res.ok || !res.changes) {
            setFailure(res.error ?? "");
            return;
          }
          setFailure(null);
          setData(res.changes);
          if (res.changes.state === "running") timer = setTimeout(() => load(false), POLL_MS);
        })
        .catch(() => {
          if (alive) setFailure("");
        });
    };
    load(retry > 0);
    return () => {
      alive = false;
      clearTimeout(timer);
    };
  }, [a.id, retry]);

  const failed = failure ?? (data?.state === "failed" ? (data.error ?? "") : null);
  const summary = data?.state === "ready" ? data.summary : undefined;
  return (
    <div className="flex flex-col gap-2 rounded-control bg-carbon-surface2 px-3.5 py-3 text-sm">
      <p className="text-caption text-carbon-textSub">
        {data?.fromAt && data.toAt
          ? t("anomaly.changes.span")
              .replace("{from}", isolateLtr(formatTs(data.fromAt)))
              .replace("{to}", isolateLtr(formatTs(data.toAt)))
          : t("anomaly.changes.title")}{" "}
        <span className="inline-flex align-middle">
          <InfoBubble tip={t("anomaly.changes.hint")} />
        </span>
      </p>
      {failed !== null && (
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-caption text-statusFail">{t("breakdown.failed").replace("{error}", failed)}</p>
          <Button label={t("breakdown.retry")} labelKey="breakdown.retry" onClick={() => setRetry((n) => n + 1)} />
        </div>
      )}
      {failed === null && !summary && (
        <p role="status" className="text-caption text-carbon-textMuted">
          {t("breakdown.running")}
        </p>
      )}
      {failed === null && summary && (
        <>
          <ChangeTable summary={summary} t={t} />
          {summary.focus && (
            <p className="text-caption text-carbon-textSub wrap-anywhere">
              {t("anomaly.changes.focus").replace("{path}", isolateLtr(`${summary.focus}/`))}
            </p>
          )}
          {summary.regenerable && (
            <p className="rounded-control bg-carbon-surface3 px-3 py-2 text-caption text-carbon-textSub">
              {t("anomaly.changes.regenerable")}
            </p>
          )}
          {summary.partial && (
            <p className="rounded-control bg-statusWarnBg px-3 py-2 text-caption text-statusWarn">{t("breakdown.partial")}</p>
          )}
        </>
      )}
    </div>
  );
}

const CELL = "border-t border-carbon-surface3 py-1.5 pe-2 align-top";
const FIGURE = `${CELL} whitespace-nowrap tabular-nums`;

/** Each folder with what went away, came in and changed. A cell stays empty
 *  where nothing did. */
function ChangeTable({ summary, t }: { summary: NonNullable<AnomalyChanges["summary"]>; t: TranslateAnomaly }) {
  const { total, folders, other } = summary;
  if (total.removedFiles + total.addedFiles + total.changedFiles === 0) {
    return <p className="text-caption text-carbon-textSub">{t("anomaly.changes.none")}</p>;
  }
  const rows = other ? [...folders, other] : folders;
  if (rows.length === 0) return null;
  const heads = ["storage.folder", "anomaly.changes.col.removed", "anomaly.changes.col.added", "anomaly.changes.col.changed"] as const;
  return (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse text-[13px]">
        <thead>
          <tr>
            {heads.map((key) => (
              <th key={key} scope="col" className="pb-1.5 pe-2 pt-1 text-start font-medium text-carbon-textMuted">
                {t(key)}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((f) => (
            <tr key={f.path ? `dir:${f.path}` : "rest"}>
              <th scope="row" className={`${CELL} text-start font-normal`}>
                {f.path ? (
                  <bdi dir="ltr" className="font-mono text-xs text-carbon-text wrap-anywhere">
                    {f.path}/
                  </bdi>
                ) : (
                  <span className="text-carbon-textSub">{t("anomaly.changes.rest")}</span>
                )}
              </th>
              <td className={FIGURE}>{f.removedFiles > 0 && humanBytes(f.removedBytes)}</td>
              <td className={FIGURE}>{f.addedFiles > 0 && humanBytes(f.addedBytes)}</td>
              <td className={FIGURE}>{f.changedFiles > 0 && t("breakdown.files", f.changedFiles)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
