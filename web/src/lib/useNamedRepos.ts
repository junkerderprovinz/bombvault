import { useEffect, useState } from "react";
import { listRepos, type NamedRepo } from "./api";

/** useNamedRepos is the named repositories, direct ones included. A failed load
 *  keeps the list it had, so a hint that depends on it does not flicker away. */
export function useNamedRepos(): NamedRepo[] {
  const [repos, setRepos] = useState<NamedRepo[]>([]);

  useEffect(() => {
    let active = true;
    listRepos()
      .then((r) => {
        if (active && r.ok) setRepos(r.repos ?? []);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, []);

  return repos;
}
