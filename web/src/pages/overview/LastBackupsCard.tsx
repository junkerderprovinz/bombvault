import { useEffect, useState } from "react";
import { listContainers } from "../../lib/api";
import type { Container } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { formatDuration, formatTs } from "../../lib/reltime";
import { Card } from "./Card";

export function LastBackupsCard({ t, hueIndex }: { t: ReturnType<typeof useT>["t"]; hueIndex?: number }) {
  const [containers, setContainers] = useState<Container[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    listContainers()
      .then((res) => {
        if (res.ok) setContainers(res.containers ?? []);
      })
      .catch(() => {/* non-fatal */})
      .finally(() => setLoading(false));
  }, []);

  const withBackups = containers
    .filter((c) => c.lastBackup != null)
    .sort((a, b) => (b.lastBackup ?? 0) - (a.lastBackup ?? 0))
    .slice(0, 6);

  const noBackups = containers
    .filter((c) => c.lastBackup == null)
    .slice(0, 4);

  return (
    <Card title={t("dashboard.lastBackups")} hueIndex={hueIndex}>
      {loading && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      )}
      {!loading && containers.length === 0 && (
        <p className="text-sm text-carbon-textMuted">{t("dashboard.noContainers")}</p>
      )}

      {withBackups.length > 0 && (
        // Rows separated by shade (soft tiles), never divider lines.
        <div className="flex flex-col gap-1 glim-content-fade">
          {withBackups.map((c) => {
            // Older data has no lastBackupStarted. The row then shows the finish
            // time alone instead of a negative or broken duration.
            const hasStart = c.lastBackupStarted != null && c.lastBackup != null;
            const duration = hasStart
              ? formatDuration((c.lastBackup as number) - (c.lastBackupStarted as number))
              : "";
            return (
              <div key={c.name} className="flex items-center gap-3 rounded-control bg-carbon-surface2 px-2 py-2.5 text-sm">
                <div className="w-2 h-2 rounded-full bg-statusOkSolid shrink-0" />
                <span className="text-carbon-text font-medium flex-1 truncate min-w-0">{c.name}</span>
                {hasStart ? (
                  <span className="text-carbon-textMuted text-xs shrink-0 text-end">
                    {formatTs(c.lastBackupStarted)} <span className="inline-block rtl:-scale-x-100">→</span> {formatTs(c.lastBackup)}
                    {duration && (
                      <span className="ms-1" title={t("dashboard.duration")} aria-label={t("dashboard.duration")}>
                        ({duration})
                      </span>
                    )}
                  </span>
                ) : (
                  <span className="text-carbon-textMuted text-xs shrink-0">
                    {formatTs(c.lastBackup)}
                  </span>
                )}
              </div>
            );
          })}
        </div>
      )}

      {noBackups.length > 0 && (
        // Rows separated by shade (soft tiles), never divider lines.
        <div className="flex flex-col gap-1">
          {/* "Never" can mean two things. A container nobody scheduled has
              never been backed up and that is the plan; a scheduled one that
              still shows "never" is a gap. includeInSchedule tells them apart.
              `self` is the case that is no choice at all: BombVault refuses to
              back up its own container (stopping it mid-run would kill the
              run), so that row must not read as an omission somebody could
              correct. */}
          {noBackups.map((c) => {
            const deliberate = c.self || !c.includeInSchedule;
            return (
              <div key={c.name} className="flex items-center gap-3 rounded-control bg-carbon-surface2 px-2 py-2.5 text-sm">
                <div className="w-2 h-2 rounded-full bg-carbon-surface3 shrink-0" />
                <span className="text-carbon-textMuted flex-1 truncate">{c.name}</span>
                <span
                  className="text-carbon-textMuted text-xs shrink-0"
                  title={c.self ? t("dashboard.neverSelfTitle") : deliberate ? t("dashboard.neverExcludedTitle") : undefined}
                >
                  {c.self
                    ? t("dashboard.neverSelf")
                    : deliberate
                      ? t("dashboard.neverExcluded")
                      : t("containers.never")}
                </span>
              </div>
            );
          })}
        </div>
      )}
    </Card>
  );
}
