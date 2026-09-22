import { useCallback, useEffect, useRef, useState } from "react";
import { listPlacementDefaults, type DefaultRow } from "./api";
import { subscribePlacement } from "./placementEvents";
import { subscribeRepos } from "./useNamedRepos";
import { subscribeOffsiteTargets } from "./useOffsiteTargets";

/** usePlacementDefaults reads the three defaults, and again after every write
 *  they depend on. A failed read keeps the rows it had. */
export function usePlacementDefaults(): { rows: DefaultRow[] | null; error: string | null; reload: () => void } {
  const [rows, setRows] = useState<DefaultRow[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  // Only the newest read may land; an older answer arriving late would undo it.
  const seq = useRef(0);

  const reload = useCallback(() => {
    const n = ++seq.current;
    listPlacementDefaults()
      .then((r) => {
        if (n !== seq.current) return;
        if (r.ok) {
          setRows(r.defaults ?? []);
          setError(null);
        } else {
          setError(r.error ?? "");
        }
      })
      .catch((err: unknown) => {
        if (n === seq.current) setError(err instanceof Error ? err.message : "");
      });
  }, []);

  useEffect(() => {
    reload();
    const offs = [subscribeOffsiteTargets(reload), subscribeRepos(reload), subscribePlacement(reload)];
    return () => offs.forEach((off) => off());
  }, [reload]);

  return { rows, error, reload };
}
