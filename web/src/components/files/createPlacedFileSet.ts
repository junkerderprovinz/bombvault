import { createDirectRepo, createFileSet, type OkEnvelope } from "../../lib/api";
import { reposChanged } from "../../lib/useNamedRepos";
import type { PlacementDraftValue } from "../placement/PlacementDraft";

export interface NewFileSet {
  name: string;
  path: string;
  excludes: string[];
  enabled: boolean;
}

/**
 * createPlacedFileSet creates a folder set at the storage location its draft
 * chose. A direct repository the draft asks for is made before the set and
 * handed to onRepoMade, so a second try after a failed create does not make
 * another one.
 */
export async function createPlacedFileSet(
  set: NewFileSet,
  placement: PlacementDraftValue,
  onRepoMade: (repo: string) => void,
  repoError: (refused: OkEnvelope) => string
): Promise<OkEnvelope & { id?: string }> {
  let home = placement.home;
  if (home && "direct" in home) {
    const made = await createDirectRepo(home.direct.targetId, home.direct.name, home.direct.location);
    if (!made.ok || !made.repo) return { ok: false, error: repoError(made) };
    reposChanged();
    onRepoMade(made.repo.id);
    home = { repo: made.repo.id };
  }
  return createFileSet({
    ...set,
    ...(home ? { repo: home.repo } : {}),
    ...(placement.copies ? { copies: placement.copies } : {}),
  });
}
