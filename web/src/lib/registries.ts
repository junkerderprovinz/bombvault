// The registries BombVault signs in to, by GlimStone's registry catalog, with
// what the catalog leaves to the app: the host a public registry is reached at,
// and what the address of one people run themselves or a cloud hands out per
// account looks like.
import type { TileMark } from "@glimstone/core";
import { REGISTRY_PROVIDERS, type RegistryGroup, type RegistryProviderId } from "@glimstone/core/registryProviders";

export type RegistryId = Exclude<RegistryProviderId, "other">;

export interface RegistryKind {
  id: RegistryId;
  name: string;
  group: RegistryGroup;
  /** The host a public registry is reached at. */
  host?: string;
  /** What a host of this kind looks like, as the field's placeholder. */
  placeholder?: string;
  mark: TileMark;
}

const HOSTS: Partial<Record<RegistryId, string>> = {
  dockerhub: "docker.io",
  ghcr: "ghcr.io",
  gitlab: "registry.gitlab.com",
  quay: "quay.io",
  codeberg: "codeberg.org",
};

const PLACEHOLDERS: Partial<Record<RegistryId, string>> = {
  gitea: "git.example.org",
  forgejo: "code.example.org",
  harbor: "harbor.example.org",
  ecr: "123456789012.dkr.ecr.eu-central-1.amazonaws.com",
  gar: "europe-west3-docker.pkg.dev",
  acr: "myname.azurecr.io",
};

export const REGISTRY_KINDS: RegistryKind[] = REGISTRY_PROVIDERS.entries.flatMap((e) =>
  e.id === "other" || !e.mark
    ? []
    : [{ id: e.id, name: e.name, group: e.group, host: HOSTS[e.id], placeholder: PLACEHOLDERS[e.id], mark: e.mark }]
);

/**
 * registryKind finds the kind an entry was saved with, and for an entry that
 * carries none the kind its host belongs to.
 */
export function registryKind(host: string, kind?: string): RegistryKind | undefined {
  const byId = REGISTRY_KINDS.find((k) => k.id === kind);
  if (byId) return byId;
  const h = host.trim().toLowerCase();
  if (!h) return undefined;
  return (
    REGISTRY_KINDS.find((k) => k.host === h) ??
    REGISTRY_KINDS.find((k) => k.group === "cloud" && k.placeholder && h.endsWith(k.placeholder.replace(/^[^.]*/, "")))
  );
}
