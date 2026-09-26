import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Badge, type BadgeTone } from "../../components/Badge";
import { InfoBubble } from "../../components/InfoBubble";
import { getMcpKeyActivity, type McpKeyActivity, type McpKeyEvent, type Run } from "../../lib/api";
import type { TranslationKey, useT } from "../../lib/i18n";
import { formatTs } from "../../lib/reltime";

type T = ReturnType<typeof useT>["t"];

/** A sentence for every outcome the server logs. A code it adds later that is
 *  missing here is shown as it came. */
const OUTCOME: Record<string, TranslationKey> = {
  ok: "mcp.outcomeOk",
  not_permitted: "mcp.outcomeNotPermitted",
  retention_guard: "mcp.outcomeRetentionGuard",
  busy: "mcp.outcomeBusy",
  cooldown: "mcp.outcomeCooldown",
  batch_refused: "mcp.outcomeBatch",
  invalid_argument: "mcp.outcomeInvalidArgument",
  not_found: "mcp.outcomeNotFound",
  ambiguous: "mcp.outcomeAmbiguous",
  domain_off: "mcp.outcomeDomainOff",
  nothing_to_back_up: "mcp.outcomeNothingToBackUp",
  not_running: "mcp.outcomeNotRunning",
  too_late: "mcp.outcomeTooLate",
  unavailable: "mcp.outcomeUnavailable",
  timeout: "mcp.outcomeTimeout",
  failed: "mcp.outcomeFailed",
};

const RUN_STATUS: Record<string, { key: TranslationKey; tone: BadgeTone }> = {
  success: { key: "run.statusSuccess", tone: "ok" },
  failed: { key: "run.statusFailed", tone: "fail" },
  running: { key: "run.statusRunning", tone: "active" },
  cancelled: { key: "run.statusCancelled", tone: "neutral" },
  skipped: { key: "run.statusSkipped", tone: "neutral" },
};

function outcomeText(t: T, e: McpKeyEvent): string {
  // The gate's per-minute budget and a start tool's hourly one refuse under the
  // same code, and only the tool tells them apart.
  if (e.outcome === "rate_limited") return t(e.tool === "" ? "mcp.outcomeRateLimited" : "mcp.outcomeStartLimit");
  const key = OUTCOME[e.outcome];
  return key ? t(key) : t("mcp.outcomeOther").replace("{code}", e.outcome);
}

function RunLink({ id, t }: { id: string; t: T }) {
  return (
    <Link to={`/dashboard?run=${encodeURIComponent(id)}`} className="text-xs text-accentText hover:underline">
      {t("mcp.logShowRun")}
    </Link>
  );
}

function runName(t: T, run: Run): string {
  return run.domain === "everything" ? t("activityLog.domainEverything") : run.target;
}

/** keyLogId is the id of a key's log panel, which its Log button controls. */
export function keyLogId(keyId: string): string {
  return `mcp-key-log-${keyId}`;
}

/**
 * McpKeyLog is what one key did, for its tile on the MCP card: the backups it
 * started, each linked to its line in the activity log, and its calls and
 * refusals, newest first. It loads when the tile opens it.
 */
export function McpKeyLog({ keyId, t }: { keyId: string; t: T }) {
  const [data, setData] = useState<McpKeyActivity | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let alive = true;
    getMcpKeyActivity(keyId)
      .then((res) => {
        if (!alive) return;
        if (res.ok) setData(res);
        else setFailed(true);
      })
      .catch(() => {
        if (alive) setFailed(true);
      });
    return () => {
      alive = false;
    };
  }, [keyId]);

  return (
    <div id={keyLogId(keyId)} className="flex flex-col gap-3 rounded-card bg-carbon-background px-3 py-2">
      {failed && <p className="text-xs text-statusWarn">{t("mcp.logFailed")}</p>}
      {!failed && data === null && <p className="text-xs text-carbon-textMuted">{t("folder.loading")}</p>}

      {data !== null && data.runs.length > 0 && (
        <div className="flex flex-col gap-1.5">
          <p className="text-xs font-medium text-carbon-textSub">{t("mcp.logRuns")}</p>
          <ul className="flex flex-col gap-1.5">
            {data.runs.map((run) => {
              const status = RUN_STATUS[run.status];
              return (
                <li key={run.id} className="flex flex-wrap items-center gap-2 text-xs text-carbon-text">
                  <Badge tone={status?.tone ?? "neutral"} size="small">
                    {status ? t(status.key) : run.status}
                  </Badge>
                  <span className="min-w-0 truncate">{runName(t, run)}</span>
                  <span className="text-carbon-textMuted tabular-nums">{formatTs(run.startedAt)}</span>
                  <RunLink id={run.id} t={t} />
                </li>
              );
            })}
          </ul>
        </div>
      )}

      {data !== null && (
        <div className="flex flex-col gap-1.5">
          <p className="flex items-center gap-1.5 text-xs font-medium text-carbon-textSub">
            {t("mcp.logCalls")}
            <InfoBubble tip={t("mcp.logKeptHint")} />
          </p>
          {data.events.length === 0 ? (
            <p className="text-xs text-carbon-textMuted">{t("mcp.logEmpty")}</p>
          ) : (
            <ul className="flex flex-col gap-1">
              {data.events.map((e, i) => (
                <li key={`${e.at}:${i}`} className="flex flex-wrap items-center gap-x-2 text-xs">
                  <span className="text-carbon-textMuted tabular-nums">{formatTs(e.at)}</span>
                  {e.tool ? (
                    <span dir="ltr" className="font-mono text-carbon-text">
                      {e.tool}
                    </span>
                  ) : (
                    <span className="text-carbon-text">{t("mcp.logRequest")}</span>
                  )}
                  <span className={e.outcome === "ok" ? "text-carbon-textSub" : "text-statusWarn"}>
                    {outcomeText(t, e)}
                  </span>
                  {e.runId !== "" && <RunLink id={e.runId} t={t} />}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
