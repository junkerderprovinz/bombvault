// Where a finding's backup differs from the last good one: which folders lost,
// gained or changed files. Opening it is what asks the server to compare.

import { useEffect, useId, useState } from "react";

import type { TranslateAnomaly } from "../../lib/anomalies";
import { getAnomalyChanges, type AnomalyChanges, type AnomalyView, type ChangeFolder } from "../../lib/api";
import { humanBytes } from "../../lib/forecast";
import { isolateLtr } from "../../lib/ltrFragments";
import { formatTs } from "../../lib/reltime";
import { Button } from "../Button";
import { IconDisclosure } from "../IconDisclosure";
import { InfoBubble } from "../InfoBubble";

// How often an open panel asks again while the comparison runs.
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
  const [open, setOpen] = useState(false);
  const [data, setData] = useState<AnomalyChanges | null>(null);
  // null while the request went through; otherwise the server's reason, empty
  // when the request itself failed.
  const [failure, setFailure] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  const panelId = useId();

  useEffect(() => {
    if (!open) return;
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
  }, [open, a.id, retry]);

  const failed = failure ?? (data?.state === "failed" ? (data.error ?? "") : null);
  const summary = data?.state === "ready" ? data.summary : undefined;
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-1.5">
        <Button
          label={t("anomaly.changes.title")}
          labelKey="anomaly.changes.title"
          glyph={<IconDisclosure open={open} />}
          tone="subtle"
          onClick={() => setOpen((o) => !o)}
          ariaExpanded={open}
          ariaControls={panelId}
          keepLabel
          className="glim-btn-wrap"
        />
        <InfoBubble tip={t("anomaly.changes.hint")} />
      </div>
      {open && (
        <div id={panelId} className="flex flex-col gap-2 text-sm">
          {failed !== null && (
            <div className="flex flex-wrap items-center gap-2">
              <p className="text-caption text-statusFail">{t("breakdown.failed").replace("{error}", failed)}</p>
              <Button
                label={t("breakdown.retry")}
                labelKey="breakdown.retry"
                tone="neutral"
                onClick={() => setRetry((n) => n + 1)}
              />
            </div>
          )}
          {failed === null && !summary && (
            <p role="status" className="text-caption text-carbon-textMuted">
              {t("breakdown.running")}
            </p>
          )}
          {failed === null && summary && data && (
            <>
              {data.fromAt && data.toAt && (
                <p className="text-caption text-carbon-textSub">
                  {t("anomaly.changes.span")
                    .replace("{from}", isolateLtr(formatTs(data.fromAt)))
                    .replace("{to}", isolateLtr(formatTs(data.toAt)))}
                </p>
              )}
              <TotalLine total={summary.total} t={t} />
              {summary.focus && (
                <p className="text-caption text-carbon-text wrap-anywhere">
                  {t("anomaly.changes.focus").replace("{path}", isolateLtr(`${summary.focus}/`))}
                </p>
              )}
              {summary.regenerable && (
                <p className="text-caption text-carbon-textSub">{t("anomaly.changes.regenerable")}</p>
              )}
              {summary.partial && <p className="text-caption text-statusWarn">{t("breakdown.partial")}</p>}
              <ChangeRows folders={summary.folders} other={summary.other} t={t} />
            </>
          )}
        </div>
      )}
    </div>
  );
}

function TotalLine({ total, t }: { total: ChangeFolder; t: TranslateAnomaly }) {
  const files = total.removedFiles + total.addedFiles + total.changedFiles;
  if (files === 0) return <p className="text-caption text-carbon-textSub">{t("anomaly.changes.none")}</p>;
  return (
    <p className="flex flex-wrap items-baseline gap-x-3 text-caption">
      <span className="text-carbon-textSub">{t("breakdown.files", files)}</span>
      <ChangeFigures f={total} t={t} />
    </p>
  );
}

function ChangeRows({ folders, other, t }: { folders: ChangeFolder[]; other?: ChangeFolder; t: TranslateAnomaly }) {
  const rows = other ? [...folders, other] : folders;
  if (rows.length === 0) return null;
  return (
    <ul className="flex flex-col gap-1">
      {rows.map((f) => (
        <li key={f.path ? `dir:${f.path}` : "rest"} className="flex flex-wrap items-baseline gap-x-3 text-caption">
          {f.path ? (
            <span dir="ltr" className="min-w-[8rem] flex-1 truncate font-mono text-carbon-text text-start">
              {f.path}/
            </span>
          ) : (
            <span className="min-w-[8rem] flex-1 text-carbon-textMuted">{t("anomaly.changes.rest")}</span>
          )}
          <ChangeFigures f={f} t={t} />
        </li>
      ))}
    </ul>
  );
}

/** What went away, came in and changed, each only when there is some. */
function ChangeFigures({ f, t }: { f: ChangeFolder; t: TranslateAnomaly }) {
  return (
    <>
      {f.removedFiles > 0 && (
        <span className="shrink-0 tabular-nums text-carbon-text">
          {t("anomaly.changes.removed").replace("{size}", humanBytes(f.removedBytes))}
        </span>
      )}
      {f.addedFiles > 0 && (
        <span className="shrink-0 tabular-nums text-accentText">
          {t("anomaly.changes.added").replace("{size}", humanBytes(f.addedBytes))}
        </span>
      )}
      {f.changedFiles > 0 && (
        <span className="shrink-0 tabular-nums text-carbon-textSub">
          {t("breakdown.new").replace("{size}", humanBytes(f.changedBytes))}
        </span>
      )}
    </>
  );
}
