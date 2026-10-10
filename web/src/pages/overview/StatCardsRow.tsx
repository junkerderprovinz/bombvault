import { useCallback, useEffect, useState } from "react";
import { getSettings, listContainers, listRuns, listVMs } from "../../lib/api";
import type { Settings } from "../../lib/api";
import type { useT } from "../../lib/i18n";
import { ErrorDetailPanel } from "../../components/ErrorDetailPanel";
import { FailureCounter } from "./FailureCounter";
import { unresolvedErrorCount } from "./runFailures";
import { StatCard } from "./StatCard";

interface StatData {
  containers: number;
  vms: number;
  activeJobs: number;
  pausedJobs: number;
  errors: number;
  missingContainers: number;
  missingVMs: number;
}

// computeStatData fetches the four inputs the stat cards need and derives the
// tile values. It runs on mount and again after the error panel acknowledges
// failures. If any fetch rejects it rejects, and the caller keeps the last
// known data.
async function computeStatData(): Promise<StatData> {
  const [contRes, settingsRes, runsRes, vmsRes] = await Promise.all([
    listContainers(),
    getSettings(),
    listRuns(),
    listVMs(),
  ]);
  const containers = contRes.ok ? (contRes.containers ?? []) : [];
  const settings: Settings | null = settingsRes.ok ? settingsRes.settings : null;
  const runs = runsRes.ok ? (runsRes.runs ?? []) : [];
  // listVMs fails or returns nothing while the VMs domain is off, which counts as none.
  const vms = vmsRes.ok ? (vmsRes.vms ?? []) : [];

  const installed = containers.filter((c) => c.installed);
  const notInstalled = containers.filter((c) => !c.installed);
  const vmsInstalled = vms.filter((v) => v.state !== "not-installed");
  const vmsMissing = vms.filter((v) => v.state === "not-installed");
  const schedEnabled = settings ? settings.containersSchedule !== "off" && settings.containersSchedule !== "" : false;
  const activeJobs = schedEnabled ? installed.filter((c) => c.includeInSchedule).length : 0;
  const pausedJobs = !schedEnabled ? installed.filter((c) => c.includeInSchedule).length : 0;
  const errors = unresolvedErrorCount(runs);

  return {
    containers: installed.length,
    vms: vmsInstalled.length,
    activeJobs,
    pausedJobs,
    errors,
    missingContainers: notInstalled.length,
    missingVMs: vmsMissing.length,
  };
}

export function StatCardsRow({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const [data, setData] = useState<StatData | null>(null);
  const [errorPanelOpen, setErrorPanelOpen] = useState(false);

  useEffect(() => {
    let active = true;
    computeStatData()
      .then((d) => {
        if (active) setData(d);
      })
      .catch(() => {
        // Non-fatal: stat cards stay null (not rendered)
      });
    return () => {
      active = false;
    };
  }, []);

  // Re-run after the error panel acknowledges failures so the errors tile
  // reflects the new count without a page reload.
  const refresh = useCallback(() => {
    computeStatData()
      .then((d) => setData(d))
      .catch(() => {
        /* non-fatal: keep the current tile values */
      });
  }, []);

  if (!data) return null;

  return (
    <>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-7">
        <StatCard label={t("dashboard.statContainers")} value={data.containers} />
        <StatCard label={t("dashboard.statVMs")} value={data.vms} />
        <StatCard label={t("dashboard.statActiveJobs")} value={data.activeJobs} />
        <StatCard label={t("dashboard.statPausedJobs")} value={data.pausedJobs} />
        <FailureCounter t={t} count={data.errors} onOpen={() => setErrorPanelOpen(true)} />
        <StatCard label={t("dashboard.statMissingContainers")} value={data.missingContainers} danger />
        <StatCard label={t("dashboard.statMissingVMs")} value={data.missingVMs} />
      </div>
      {errorPanelOpen && (
        <ErrorDetailPanel onClose={() => setErrorPanelOpen(false)} onChanged={refresh} />
      )}
    </>
  );
}
