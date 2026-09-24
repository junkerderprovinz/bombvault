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
import {
  getAnomalies,
  getAnomalyItems,
  getAnomalySummary,
  type AnomalyItem,
  type AnomalySummary,
  type AnomalyView,
} from "./api";
import { ANOMALY_CHANGED_EVENT } from "./anomalies";

const POLL_MS = 15000;
const PAGE_SIZE = 500;

export interface AnomalySummaryState {
  summary: AnomalySummary | null;
  /** The request failed or was refused. Consumers must not read this as calm. */
  error: boolean;
  loading: boolean;
  reload: () => void;
}

const AnomalyContext = createContext<AnomalySummaryState>({
  summary: null,
  error: false,
  loading: false,
  reload: () => undefined,
});

export interface OpenAnomaliesState {
  list: AnomalyView[];
  /** The open findings a run raised or was last seen in, by run id. */
  byRunId: Map<string, AnomalyView[]>;
  /** The snapshots open data-loss findings were raised on. */
  flagged: Set<string>;
  error: boolean;
}

const NO_OPEN: OpenAnomaliesState = { list: [], byRunId: new Map(), flagged: new Set(), error: false };

// The open rows live in the provider, fetched only while something on screen
// reads them: the dashboard shows them in three places, and three readers of
// one pass must not be three requests.
const OpenAnomalyContext = createContext<{ open: OpenAnomaliesState; watch: () => () => void }>({
  open: NO_OPEN,
  watch: () => () => undefined,
});

function indexByRun(list: AnomalyView[]): Map<string, AnomalyView[]> {
  const out = new Map<string, AnomalyView[]>();
  for (const a of list) {
    for (const id of new Set([a.runId, a.lastRunId])) {
      if (!id) continue;
      out.set(id, [...(out.get(id) ?? []), a]);
    }
  }
  return out;
}

async function readAllOpen(): Promise<AnomalyView[]> {
  const all: AnomalyView[] = [];
  let cursor = "";
  do {
    const res = await getAnomalies({ state: "open", limit: PAGE_SIZE, cursor: cursor || undefined });
    if (!res.ok) throw new Error("the findings were refused");
    all.push(...res.anomalies);
    cursor = res.nextCursor;
  } while (cursor);
  return all;
}

export function AnomalyProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<Omit<AnomalySummaryState, "reload">>({
    summary: null,
    error: false,
    loading: true,
  });
  const [openRows, setOpenRows] = useState<{ list: AnomalyView[]; error: boolean }>({ list: [], error: false });
  const [watchers, setWatchers] = useState(0);

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

  const generation = state.summary?.generation;
  const watched = watchers > 0;
  useEffect(() => {
    if (!watched || generation === undefined) return;
    let active = true;
    readAllOpen()
      .then((list) => {
        if (active) setOpenRows({ list, error: false });
      })
      .catch(() => {
        if (active) setOpenRows((prev) => ({ ...prev, error: true }));
      });
    return () => {
      active = false;
    };
  }, [generation, watched]);

  const watch = useCallback(() => {
    setWatchers((n) => n + 1);
    return () => setWatchers((n) => n - 1);
  }, []);

  const value = useMemo(() => ({ ...state, reload: load }), [state, load]);
  const open = useMemo(
    () => ({
      open: {
        list: openRows.list,
        byRunId: indexByRun(openRows.list),
        flagged: new Set(openRows.list.flatMap((a) => a.flaggedSnapshots ?? [])),
        error: openRows.error,
      },
      watch,
    }),
    [openRows, watch]
  );

  return createElement(
    AnomalyContext.Provider,
    { value },
    createElement(OpenAnomalyContext.Provider, { value: open }, children)
  );
}

export function useAnomalySummary(): AnomalySummaryState {
  return useContext(AnomalyContext);
}

/**
 * useOpenAnomalies reads every open finding, following the cursor to the end:
 * badges and counts are drawn from this list, so a page cut at 500 would show
 * an item as clean.
 */
export function useOpenAnomalies(): OpenAnomaliesState {
  const { open, watch } = useContext(OpenAnomalyContext);
  useEffect(() => watch(), [watch]);
  return open;
}

export interface AnomalyItemsState {
  items: AnomalyItem[];
  byTarget: Map<string, AnomalyItem>;
  /** The item a page row stands for. Containers and VMs know their name
   *  rather than the target id the engine keys on. */
  find: (domain: string, nameOrId: string) => AnomalyItem | undefined;
  /** The request failed or was refused. Consumers must not read this as calm. */
  error: boolean;
  loading: boolean;
  retry: () => void;
}

/**
 * useAnomalyItems reads the watched items once per pass. Badges beside item
 * names and the Items tab both come from here, so a name and its figures are
 * never two requests apart.
 */
export function useAnomalyItems(): AnomalyItemsState {
  const { summary, error: summaryFailed, reload } = useAnomalySummary();
  const generation = summary?.generation;
  const [items, setItems] = useState<AnomalyItem[]>([]);
  const [error, setError] = useState(false);
  const [loading, setLoading] = useState(true);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (generation === undefined) {
      // A refused summary leaves nothing to list against, and a caller still
      // waiting would read that as a list on its way.
      if (summaryFailed) {
        setError(true);
        setLoading(false);
      }
      return;
    }
    let active = true;
    getAnomalyItems()
      .then((res) => {
        if (!active) return;
        if (!res.ok) {
          setError(true);
        } else {
          setItems(res.items);
          setError(false);
        }
        setLoading(false);
      })
      .catch(() => {
        if (!active) return;
        setError(true);
        setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [generation, summaryFailed, attempt]);

  const retry = useCallback(() => {
    reload();
    setAttempt((n) => n + 1);
  }, [reload]);

  return useMemo(() => {
    const byTarget = new Map(items.map((i) => [i.targetId, i]));
    // The domain is checked on the id path too: a container may be called
    // "flash", which is also the flash drive's target id.
    const find = (domain: string, nameOrId: string) => {
      const byId = byTarget.get(nameOrId);
      if (byId?.domain === domain) return byId;
      return items.find((i) => i.domain === domain && i.name === nameOrId);
    };
    return { items, byTarget, find, error, loading, retry };
  }, [items, error, loading, retry]);
}
