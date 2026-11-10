import { useCallback, useEffect, useState } from "react";

import { getSharedRepo, type SharedHolding, type SharedMembership, type SharedServer } from "../../lib/api";

export interface SharedRepoState {
  loaded: boolean;
  /** What failed while loading, "" when the server named nothing; null when nothing failed. */
  error: string | null;
  server: SharedServer | null;
  defaultPort: number;
  memberships: SharedMembership[];
  /** What this instance holds or held, as a holder sees it. */
  holdings: SharedHolding[];
  reload: () => Promise<void>;
  /** Takes the server a change on the owner's card answered with. */
  setServer: (server: SharedServer | null) => void;
}

/** useSharedRepo reads both sides of the shared repository: the one this
 *  instance holds and the ones it copies into. A paired phone is refused the
 *  call, and then neither card shows. */
export function useSharedRepo(enabled: boolean): SharedRepoState {
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [server, setServer] = useState<SharedServer | null>(null);
  const [defaultPort, setDefaultPort] = useState(8001);
  const [memberships, setMemberships] = useState<SharedMembership[]>([]);
  const [holdings, setHoldings] = useState<SharedHolding[]>([]);

  const reload = useCallback(
    (): Promise<void> =>
      getSharedRepo()
        .then((r) => {
          if (!r.ok) {
            setError(r.error ?? "");
            return;
          }
          setError(null);
          setServer(r.server ?? null);
          setDefaultPort(r.defaultPort ?? 8001);
          setMemberships(r.memberships ?? []);
          setHoldings(r.holdings ?? []);
          setLoaded(true);
        })
        .catch((e: unknown) => setError(e instanceof Error ? e.message : "")),
    [],
  );

  useEffect(() => {
    if (enabled) void reload();
  }, [enabled, reload]);

  return { loaded, error, server, defaultPort, memberships, holdings, reload, setServer };
}
