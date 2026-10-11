import { createElement, useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { listRuns, type AnomalyView, type Run, type Settings } from "./api";
import { ATTENTION_CHANGED_EVENT, AttentionContext, AttentionCountContext, FreshInstallContext } from "./attention";
import { forgetFirstSteps } from "./firstSteps";
import { isFreshInstall } from "./freshInstall";
import { needsOf } from "./needs";
import { useAnomalySummary, useOpenAnomalies } from "./useAnomalies";
import { useBackupItems } from "./useBackupItems";
import { useStorageLocationsOnDemand } from "./useStorageLocations";

const POLL_MS = 30000;
// The activity log reads the runs at this rate, and two readers of one
// endpoint at different rates can show two answers.
const WATCHED_POLL_MS = 10000;

const NO_FINDINGS: AnomalyView[] = [];

type KitSettings = Pick<Settings, "encryptionEnabled" | "recoveryKitAck">;

/** recoveryKitDue reports encrypted backups whose recovery kit nobody has
 *  confirmed storing. */
export function recoveryKitDue(settings: KitSettings | null): boolean {
  return !!settings && settings.encryptionEnabled && !settings.recoveryKitAck;
}

/**
 * AttentionProvider reads the entries, the runs and the storage locations for
 * everything below it and lists what needs the person. The findings and the
 * settings come from the readers the layout already has.
 */
export function AttentionProvider({ settings, children }: { settings: Settings | null; children: ReactNode }) {
  const items = useBackupItems();
  const reloadItems = items.reload;
  const locations = useStorageLocationsOnDemand();
  const reloadLocations = locations.retry;
  const [runs, setRuns] = useState<Run[]>([]);
  const [runsReady, setRunsReady] = useState(false);
  const [runsFailed, setRunsFailed] = useState(false);
  const [watchers, setWatchers] = useState(0);
  const detection = useAnomalySummary().summary?.enabled ?? false;
  const open = useOpenAnomalies().list;

  // A failed read keeps the runs it had: a count that drops to zero because
  // the server was busy would read as all clear.
  const refreshRuns = useCallback(() => {
    listRuns()
      .then((res) => {
        if (res.ok) setRuns(res.runs ?? []);
        setRunsFailed(!res.ok);
      })
      .catch(() => setRunsFailed(true))
      .finally(() => setRunsReady(true));
  }, []);

  // Marking failures as dealt with can cover failed copies, which only the
  // storage locations report.
  useEffect(() => {
    const dealtWith = () => {
      refreshRuns();
      reloadLocations();
    };
    refreshRuns();
    window.addEventListener(ATTENTION_CHANGED_EVENT, dealtWith);
    return () => window.removeEventListener(ATTENTION_CHANGED_EVENT, dealtWith);
  }, [refreshRuns, reloadLocations]);

  const pollMs = watchers > 0 ? WATCHED_POLL_MS : POLL_MS;
  useEffect(() => {
    const timer = window.setInterval(() => {
      if (document.visibilityState === "visible") refreshRuns();
    }, pollMs);
    return () => window.clearInterval(timer);
  }, [pollMs, refreshRuns]);

  // The end of a run changes what protects its entry, and the end of a copy
  // what its storage location says of it. Neither list is polled.
  const lastEnd = runs.reduce((last, run) => Math.max(last, run.finishedAt ?? 0), 0);
  const seenEnd = useRef<number | null>(null);
  useEffect(() => {
    if (!runsReady) return;
    if (seenEnd.current !== null && seenEnd.current !== lastEnd) {
      reloadItems();
      reloadLocations();
    }
    seenEnd.current = lastEnd;
  }, [runsReady, lastEnd, reloadItems, reloadLocations]);

  // A page that mounts together with the provider finds the first reads under
  // way. Only a later visit has something older to replace.
  const firstReadsBack = useRef(false);
  useEffect(() => {
    firstReadsBack.current = runsReady && (items.loaded || items.failed);
  }, [runsReady, items.loaded, items.failed]);

  const watch = useCallback(() => {
    setWatchers((n) => n + 1);
    if (firstReadsBack.current) {
      reloadItems();
      refreshRuns();
      reloadLocations();
    }
    return () => setWatchers((n) => n - 1);
  }, [reloadItems, refreshRuns, reloadLocations]);

  const kitDue = recoveryKitDue(settings);
  const anomalies = detection ? open : NO_FINDINGS;
  const needs = useMemo(
    () => needsOf({ items: items.items, runs, locations: locations.locations, anomalies, kitDue }),
    [items.items, runs, locations.locations, anomalies, kitDue]
  );
  const fresh = items.loaded && isFreshInstall(items.items);
  useEffect(() => {
    if (items.loaded && !fresh) forgetFirstSteps();
  }, [items.loaded, fresh]);
  const value = useMemo(
    () => ({ needs, fresh, items, runs, runsReady, runsFailed, refreshRuns, locations, watch }),
    [needs, fresh, items, runs, runsReady, runsFailed, refreshRuns, locations, watch]
  );

  return createElement(
    AttentionContext.Provider,
    { value },
    createElement(
      AttentionCountContext.Provider,
      { value: needs.length },
      createElement(FreshInstallContext.Provider, { value: fresh }, children)
    )
  );
}
