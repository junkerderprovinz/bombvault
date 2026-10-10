import { createElement, useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { listRuns, type AnomalyView, type Run, type Settings } from "./api";
import { ATTENTION_CHANGED_EVENT, AttentionContext, AttentionCountContext } from "./attention";
import { needsOf } from "./needs";
import { useAnomalySummary, useOpenAnomalies } from "./useAnomalies";
import { useBackupItems } from "./useBackupItems";

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
 * AttentionProvider reads the entries and the runs for everything below it and
 * lists what needs the person. The findings and the settings come from the
 * readers the layout already has.
 */
export function AttentionProvider({ settings, children }: { settings: Settings | null; children: ReactNode }) {
  const items = useBackupItems();
  const reloadItems = items.reload;
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

  useEffect(() => {
    refreshRuns();
    window.addEventListener(ATTENTION_CHANGED_EVENT, refreshRuns);
    return () => window.removeEventListener(ATTENTION_CHANGED_EVENT, refreshRuns);
  }, [refreshRuns]);

  const pollMs = watchers > 0 ? WATCHED_POLL_MS : POLL_MS;
  useEffect(() => {
    const timer = window.setInterval(() => {
      if (document.visibilityState === "visible") refreshRuns();
    }, pollMs);
    return () => window.clearInterval(timer);
  }, [pollMs, refreshRuns]);

  // The end of a run changes what protects its entry, and the entries are
  // not polled.
  const lastEnd = runs.reduce((last, run) => Math.max(last, run.finishedAt ?? 0), 0);
  const seenEnd = useRef<number | null>(null);
  useEffect(() => {
    if (!runsReady) return;
    if (seenEnd.current !== null && seenEnd.current !== lastEnd) reloadItems();
    seenEnd.current = lastEnd;
  }, [runsReady, lastEnd, reloadItems]);

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
    }
    return () => setWatchers((n) => n - 1);
  }, [reloadItems, refreshRuns]);

  const kitDue = recoveryKitDue(settings);
  const anomalies = detection ? open : NO_FINDINGS;
  const needs = useMemo(
    () => needsOf({ items: items.items, runs, anomalies, kitDue }),
    [items.items, runs, anomalies, kitDue]
  );
  const value = useMemo(
    () => ({ needs, items, runs, runsReady, runsFailed, refreshRuns, watch }),
    [needs, items, runs, runsReady, runsFailed, refreshRuns, watch]
  );

  return createElement(
    AttentionContext.Provider,
    { value },
    createElement(AttentionCountContext.Provider, { value: needs.length }, children)
  );
}
