import { useEffect, useState } from "react";
import { getProviders, type Provider } from "./api";

let known: Promise<Provider[]> | null = null;

/** The storage products BombVault knows, read once for the whole page load:
 *  the list is built into the server and does not change while it runs. */
function loadProviders(): Promise<Provider[]> {
  known ??= getProviders()
    .then((res) => res.providers ?? [])
    .catch(() => {
      known = null;
      return [];
    });
  return known;
}

/** useProvider is the provider a location was set up with, undefined while
 *  the list loads and for a location that was typed in. */
export function useProvider(id: string): Provider | undefined {
  const [provider, setProvider] = useState<Provider>();
  useEffect(() => {
    let alive = true;
    if (!id) return;
    void loadProviders().then((providers) => {
      if (alive) setProvider(providers.find((p) => p.id === id));
    });
    return () => {
      alive = false;
    };
  }, [id]);
  return id ? provider : undefined;
}
