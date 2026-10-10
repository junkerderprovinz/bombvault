import { useCallback, useEffect, useState } from "react";
import { getRclone } from "./api";

/** useRcloneRemotes is the names of the remotes in BombVault's rclone.conf.
 *  A failed read keeps the names it had. */
export function useRcloneRemotes(): { remotes: string[]; refresh: () => void } {
  const [remotes, setRemotes] = useState<string[]>([]);

  const refresh = useCallback(() => {
    getRclone()
      .then((r) => {
        if (r.ok) setRemotes(r.remotes ?? []);
      })
      .catch(() => undefined);
  }, []);

  useEffect(refresh, [refresh]);

  return { remotes, refresh };
}
