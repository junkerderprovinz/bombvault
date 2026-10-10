// What needs the person, counted in one place for the overview and for the
// navigation entry that leads to it.

import { createContext, createElement, useContext, useEffect, useState, type ReactNode } from "react";
import { getCoverage, listRuns, type CoverageItem, type CoverageReport, type Run, type Settings } from "./api";
import { failedRunsNeedingAttention } from "./runFailures";
import { useLoudAnomalies } from "./useAnomalies";

/** Dispatched after a failure was dealt with, so the count follows without
 *  waiting for the next poll. */
export const ATTENTION_CHANGED_EVENT = "bv:attention-changed";

const POLL_MS = 30000;

type KitSettings = Pick<Settings, "encryptionEnabled" | "recoveryKitAck">;

export type UnprotectedEntry = CoverageItem & { domain: string };

/** unprotectedEntries lists what no automatic run backs up. A backup type that
 *  is switched off is left out, because nobody asked for it to be covered. */
export function unprotectedEntries(coverage: CoverageReport | null): UnprotectedEntry[] {
  return (coverage?.domains ?? [])
    .filter((d) => d.enabled)
    .flatMap((d) => d.unprotected.map((i) => ({ ...i, domain: d.domain })));
}

/** recoveryKitDue reports encrypted backups whose recovery kit nobody has
 *  confirmed storing. */
export function recoveryKitDue(settings: KitSettings | null): boolean {
  return !!settings && settings.encryptionEnabled && !settings.recoveryKitAck;
}

export interface AttentionInput {
  /** Newest first, as `listRuns` returns them. */
  runs: Run[];
  coverage: CoverageReport | null;
  /** The open critical and warning findings. */
  loudAnomalies: number;
  settings: KitSettings | null;
}

/** attentionCount is the number of things that need the person: failures
 *  nobody acknowledged, entries nothing backs up, loud findings and a recovery
 *  kit that is still to be stored. */
export function attentionCount({ runs, coverage, loudAnomalies, settings }: AttentionInput): number {
  return (
    failedRunsNeedingAttention(runs).length +
    unprotectedEntries(coverage).length +
    loudAnomalies +
    (recoveryKitDue(settings) ? 1 : 0)
  );
}

const AttentionContext = createContext(0);

/**
 * AttentionProvider keeps the count for everything below it. It reads the runs
 * and the coverage itself, so the navigation shows the figure on every page and
 * fetches nothing of its own.
 */
export function AttentionProvider({ settings, children }: { settings: Settings | null; children: ReactNode }) {
  const [runs, setRuns] = useState<Run[]>([]);
  const [coverage, setCoverage] = useState<CoverageReport | null>(null);
  const loudAnomalies = useLoudAnomalies().count;

  useEffect(() => {
    let active = true;
    // A failed read keeps the last figures: a count that drops to zero because
    // the server was busy would read as all clear.
    const loadRuns = () => {
      listRuns()
        .then((res) => {
          if (active && res.ok) setRuns(res.runs ?? []);
        })
        .catch(() => undefined);
    };
    const load = () => {
      loadRuns();
      getCoverage()
        .then((res) => {
          if (active && res.ok) setCoverage(res.coverage);
        })
        .catch(() => undefined);
    };
    load();
    // Coverage only moves with the settings, so the timer leaves it alone.
    const timer = window.setInterval(() => {
      if (document.visibilityState === "visible") loadRuns();
    }, POLL_MS);
    window.addEventListener(ATTENTION_CHANGED_EVENT, load);
    window.addEventListener("bv:settings-changed", load);
    return () => {
      active = false;
      window.clearInterval(timer);
      window.removeEventListener(ATTENTION_CHANGED_EVENT, load);
      window.removeEventListener("bv:settings-changed", load);
    };
  }, []);

  return createElement(
    AttentionContext.Provider,
    { value: attentionCount({ runs, coverage, loudAnomalies, settings }) },
    children
  );
}

export function useAttentionCount(): number {
  return useContext(AttentionContext);
}
