import { useEffect, useState } from "react";
import { getProviders, type Backend, type Provider } from "./api";

interface Known {
  providers: Provider[];
  backends: Record<string, Backend>;
}

let known: Promise<Known> | null = null;

/** The storage products BombVault knows, read once for the whole page load:
 *  the list is built into the server and does not change while it runs. */
function loadProviders(): Promise<Known> {
  known ??= getProviders()
    .then((res) => ({ providers: res.providers ?? [], backends: res.backends ?? {} }))
    .catch(() => {
      known = null;
      return { providers: [], backends: {} };
    });
  return known;
}

/** A provider with the rclone backend that describes its form. A provider
 *  that is not reached through rclone has none. */
export interface ProviderForm {
  provider: Provider;
  backend?: Backend;
}

/** useProviderForm is the provider a location was set up with, undefined
 *  while the list loads and for a location that was typed in. */
export function useProviderForm(id: string): ProviderForm | undefined {
  const [form, setForm] = useState<ProviderForm>();
  useEffect(() => {
    let alive = true;
    if (!id) return;
    void loadProviders().then(({ providers, backends }) => {
      const provider = providers.find((p) => p.id === id);
      if (alive) setForm(provider && { provider, backend: backends[provider.backend] });
    });
    return () => {
      alive = false;
    };
  }, [id]);
  return id ? form : undefined;
}

export function useProvider(id: string): Provider | undefined {
  return useProviderForm(id)?.provider;
}
