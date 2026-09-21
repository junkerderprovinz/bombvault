import { useEffect, useState } from "react";
import { listRepos, type NamedRepo } from "./api";

const REPOS_CHANGED = "bv:repos-changed";

/** reposChanged announces a successful write to the named repositories. */
export function reposChanged(): void {
  window.dispatchEvent(new Event(REPOS_CHANGED));
}

export function subscribeRepos(onChange: () => void): () => void {
  window.addEventListener(REPOS_CHANGED, onChange);
  return () => window.removeEventListener(REPOS_CHANGED, onChange);
}

/** useNamedRepos is the named repositories, direct ones included. A failed load
 *  keeps the list it had, so a hint that depends on it does not flicker away. */
export function useNamedRepos(): NamedRepo[] {
  const [repos, setRepos] = useState<NamedRepo[]>([]);

  useEffect(() => {
    let active = true;
    const load = () => {
      listRepos()
        .then((r) => {
          if (active && r.ok) setRepos(r.repos ?? []);
        })
        .catch(() => undefined);
    };
    load();
    const off = subscribeRepos(load);
    return () => {
      active = false;
      off();
    };
  }, []);

  return repos;
}
