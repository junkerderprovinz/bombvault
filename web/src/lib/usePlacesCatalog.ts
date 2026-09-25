import { useEffect, useState } from "react";
import { getPlacesCatalog, type CatalogProvider } from "./places";

interface Catalog {
  providers: CatalogProvider[];
  loaded: boolean;
  /** The providers could not be read, and `providers` stays empty. */
  failed: boolean;
}

/** usePlacesCatalog reads the providers once per mount. */
export function usePlacesCatalog(): Catalog {
  const [state, setState] = useState<Catalog>({ providers: [], loaded: false, failed: false });
  useEffect(() => {
    let live = true;
    getPlacesCatalog()
      .then((r) => {
        if (live) setState({ providers: r.ok ? (r.providers ?? []) : [], loaded: true, failed: !r.ok });
      })
      .catch(() => {
        if (live) setState({ providers: [], loaded: true, failed: true });
      });
    return () => {
      live = false;
    };
  }, []);
  return state;
}
