import { useEffect, useState } from "react";
import { getPlacesCatalog, type CatalogProvider } from "./places";

/** usePlacesCatalog reads the providers once per mount. `failed` holds the
 *  reason when they could not be read, and `providers` stays empty. */
export function usePlacesCatalog(): { providers: CatalogProvider[]; loaded: boolean; failed: string | null } {
  const [state, setState] = useState<{ providers: CatalogProvider[]; loaded: boolean; failed: string | null }>({
    providers: [],
    loaded: false,
    failed: null,
  });
  useEffect(() => {
    let live = true;
    getPlacesCatalog()
      .then((r) => {
        if (live) setState({ providers: r.ok ? (r.providers ?? []) : [], loaded: true, failed: r.ok ? null : (r.error ?? "") });
      })
      .catch((err: unknown) => {
        if (live) setState({ providers: [], loaded: true, failed: err instanceof Error ? err.message : "" });
      });
    return () => {
      live = false;
    };
  }, []);
  return state;
}
