import { useCallback, useState } from "react";

import { runDrill } from "../../lib/api";

export interface TrialRestore {
  /** The domain whose trial restore is under way, or null. */
  running: string | null;
  /** Why the last one left no result, per domain. A restore that ran, pass
   *  or fail, is recorded and comes back with the section's status instead. */
  refused: Record<string, string>;
  start: (domain: string) => void;
}

/**
 * useTrialRestore restores a section's newest off-site backup into a sandbox
 * on request. The server keeps the outcome, so the status is read again once
 * it is back.
 */
export function useTrialRestore(fallback: string): TrialRestore {
  const [running, setRunning] = useState<string | null>(null);
  const [refused, setRefused] = useState<Record<string, string>>({});

  const start = useCallback(
    (domain: string) => {
      setRunning(domain);
      setRefused(({ [domain]: _, ...rest }) => rest);
      runDrill(domain, "offsite", "dr")
        .then((res) => {
          if (!res.ok && !res.drill) setRefused((prev) => ({ ...prev, [domain]: res.error ?? fallback }));
        })
        .catch((err) => {
          setRefused((prev) => ({ ...prev, [domain]: err instanceof Error ? err.message : fallback }));
        })
        .finally(() => {
          setRunning((current) => (current === domain ? null : current));
          window.dispatchEvent(new Event("bv:settings-changed"));
        });
    },
    [fallback]
  );

  return { running, refused, start };
}
