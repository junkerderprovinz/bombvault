// The app's one reader of the anomaly endpoints. The summary is polled in the
// layout and shared, so the sidebar count, the dashboard card and the page all
// answer from the same figures; the open rows are refetched whenever that
// summary reports a new generation, so a count and the rows under it can never
// describe two different passes.

import {
  createContext,
  createElement,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { getAnomalies, getAnomalySummary, type AnomalySummary, type AnomalyView } from "./api";
import { ANOMALY_CHANGED_EVENT } from "./anomalies";

const POLL_MS = 15000;
const PAGE_SIZE = 500;

export interface AnomalySummaryState {
  summary: AnomalySummary | null;
  /** The request failed or was refused. Consumers must not read this as calm. */
  error: boolean;
  loading: boolean;
}

const AnomalyContext = createContext<AnomalySummaryState>({
  summary: null,
  error: false,
  loading: false,
});

export function AnomalyProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AnomalySummaryState>({
    summary: null,
    error: false,
    loading: true,
  });

  const load = useCallback(() => {
    getAnomalySummary()
      .then((res) =>
        setState((prev) =>
          res.ok
            ? { summary: res.summary, error: false, loading: false }
            : { summary: prev.summary, error: true, loading: false }
        )
      )
      .catch(() => setState((prev) => ({ summary: prev.summary, error: true, loading: false })));
  }, []);

  useEffect(() => {
    load();
    // A background tab has nobody to show a new finding to, and the engine
    // reports it again on the next visible tick.
    const timer = window.setInterval(() => {
      if (document.visibilityState === "visible") load();
    }, POLL_MS);
    return () => window.clearInterval(timer);
  }, [load]);

  useEffect(() => {
    const onChange = () => load();
    window.addEventListener(ANOMALY_CHANGED_EVENT, onChange);
    window.addEventListener("bv:settings-changed", onChange);
    return () => {
      window.removeEventListener(ANOMALY_CHANGED_EVENT, onChange);
      window.removeEventListener("bv:settings-changed", onChange);
    };
  }, [load]);

  return createElement(AnomalyContext.Provider, { value: state }, children);
}

export function useAnomalySummary(): AnomalySummaryState {
  return useContext(AnomalyContext);
}

/**
 * useOpenAnomalies reads every open finding, following the cursor to the end:
 * badges and counts are drawn from this list, so a page cut at 500 would show
 * an item as clean.
 */
export function useOpenAnomalies(): { list: AnomalyView[]; error: boolean } {
  const { summary } = useAnomalySummary();
  const generation = summary?.generation;
  const [list, setList] = useState<AnomalyView[]>([]);
  const [error, setError] = useState(false);

  useEffect(() => {
    if (generation === undefined) return;
    let active = true;
    void (async () => {
      const all: AnomalyView[] = [];
      let cursor = "";
      do {
        const res = await getAnomalies({ state: "open", limit: PAGE_SIZE, cursor: cursor || undefined });
        if (!res.ok) throw new Error("the findings were refused");
        all.push(...res.anomalies);
        cursor = res.nextCursor;
      } while (cursor);
      if (!active) return;
      setList(all);
      setError(false);
    })().catch(() => {
      if (active) setError(true);
    });
    return () => {
      active = false;
    };
  }, [generation]);

  return useMemo(() => ({ list, error }), [list, error]);
}
