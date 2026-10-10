import { useEffect, useState } from "react";
import { getSpike } from "../../lib/api";
import type { SpikeCheck } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { statusLabel, statusTone } from "../../lib/runDisplay";
import { Badge } from "../../components/Badge";
import { Card } from "./Card";

export function SpikeCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const [checks, setChecks] = useState<SpikeCheck[] | null>(null);
  const [allOk, setAllOk] = useState<boolean | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Display-only on the dashboard: load the cached result (warmed at container
  // startup). Running the check lives in Settings, not here. t() is only read
  // to build a failure message, and fetching again on a language switch would
  // be a wasted round-trip.
  useEffect(() => {
    let active = true;
    setLoading(true);
    getSpike()
      .then((res) => {
        if (!active) return;
        setChecks(res.checks);
        setAllOk(res.allOk);
      })
      .catch((err) => {
        if (!active) return;
        setError(err instanceof Error ? err.message : t("common.checkFailed"));
        setAllOk(false);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const hasRun = !loading && allOk !== null;
  const overallStatus = allOk ? "ok" : "degraded";
  const overallLabel = allOk ? t("dashboard.allOk") : t("dashboard.degraded");

  // A best-effort (optional) check that fails is informational, not a failure.
  const chipFor = (c: SpikeCheck) => (c.OK ? "ok" : c.BestEffort ? "info" : "failed");

  return (
    <Card title={t("spike.title")} hueIndex={hueIndex}>
      {loading && (
        <p className="text-xs text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}

      {hasRun && (
        <div className="flex items-center gap-2">
          <span className="text-xs text-carbon-textMuted">{t("spike.overall")}</span>
          <Badge tone={statusTone(overallStatus)}>{statusLabel(overallStatus, t)}</Badge>
          <span className="text-sm text-carbon-text">{overallLabel}</span>
        </div>
      )}

      {error && (
        <p className="text-xs text-statusFail">{error}</p>
      )}

      {checks && checks.length > 0 && (
        // Rows separated by shade (soft tiles), never divider lines.
        <div className="flex flex-col gap-1">
          {checks.map((c) => (
            <div key={c.Name} className="flex items-center gap-3 rounded-control bg-carbon-surface2 px-2 py-2 text-sm">
              <Badge tone={statusTone(chipFor(c))}>{statusLabel(chipFor(c), t)}</Badge>
              <span className="font-mono text-carbon-text w-32 shrink-0">{c.Name}</span>
              <span className="text-carbon-textMuted truncate flex-1">{c.Detail}</span>
              {c.BestEffort && (
                <span className="text-xs text-carbon-textMuted shrink-0">
                  {t("spike.bestEffort")}
                </span>
              )}
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}
