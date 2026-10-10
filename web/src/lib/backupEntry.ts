// The six kinds of entry a backup can be of, described once: how an entry is
// addressed, where its page is, which key its running backup reports under and
// which calls act on it.

import {
  backupConfigNow,
  backupFileSet,
  backupFlashNow,
  backupNow,
  backupVMNow,
  backupZFSDataset,
  patchFileSet,
  patchZFSDataset,
  setInclude,
  setVMInclude,
  type BackupResponse,
  type OkEnvelope,
  type PlacementDomain,
  type TimelineDomain,
} from "./api";

export const ENTRY_KINDS = ["container", "vm", "files", "zfs", "flash", "config"] as const;

export type EntryKind = (typeof ENTRY_KINDS)[number];

export interface EntryKindSpec {
  /** What addresses an entry in its path and in the API: its name (the libvirt
   *  name of a VM), its id, or the kind alone where the kind holds one entry. */
  keyedBy: "name" | "id" | "kind";
  /** The kind's own page, which older links point to. */
  legacyPath: string;
  timelineDomain?: TimelineDomain;
  placementDomain?: PlacementDomain;
  backup: (key: string) => Promise<BackupResponse>;
  /** Takes the entry out of the schedule or back into it. Flash and config
   *  have no switch of their own, only their kind's in the settings. */
  setIncluded?: (key: string, include: boolean) => Promise<OkEnvelope>;
}

export const ENTRY_KIND: Record<EntryKind, EntryKindSpec> = {
  container: {
    keyedBy: "name",
    legacyPath: "/containers",
    timelineDomain: "containers",
    placementDomain: "containers",
    backup: backupNow,
    setIncluded: setInclude,
  },
  vm: {
    keyedBy: "name",
    legacyPath: "/vms",
    timelineDomain: "vms",
    placementDomain: "vms",
    backup: backupVMNow,
    setIncluded: setVMInclude,
  },
  files: {
    keyedBy: "id",
    legacyPath: "/files",
    timelineDomain: "files",
    placementDomain: "files",
    backup: backupFileSet,
    setIncluded: (id, enabled) => patchFileSet(id, { enabled }),
  },
  zfs: {
    keyedBy: "id",
    legacyPath: "/zfs",
    backup: backupZFSDataset,
    setIncluded: (id, enabled) => patchZFSDataset(id, { enabled }),
  },
  flash: { keyedBy: "kind", legacyPath: "/flash", timelineDomain: "flash", backup: backupFlashNow },
  config: { keyedBy: "kind", legacyPath: "/config", timelineDomain: "config", backup: backupConfigNow },
};

export function isEntryKind(value: string): value is EntryKind {
  return (ENTRY_KINDS as readonly string[]).includes(value);
}

/** kindOfDomain maps a domain as findings and runs name it, which is the kind
 *  or its plural, to the kind. */
export function kindOfDomain(domain: string): EntryKind | undefined {
  return ENTRY_KINDS.find((kind) => kind === domain || ENTRY_KIND[kind].timelineDomain === domain);
}

/** entryKey picks the key out of what the server tells about an entry. */
export function entryKey(kind: EntryKind, entry: { name: string; id: string }): string {
  const { keyedBy } = ENTRY_KIND[kind];
  return keyedBy === "kind" ? kind : entry[keyedBy];
}

/**
 * progressKey is the key a running backup of the entry reports under, and the
 * one cancelBackup takes. The server builds it from the name, also for folder
 * sets and ZFS items, whose key is the id: the set's name and the root dataset.
 */
export function progressKey(kind: EntryKind, name: string): string {
  return ENTRY_KIND[kind].keyedBy === "kind" ? kind : `${kind}:${name}`;
}

export const ENTRY_LIST_PATH = "/backups";

/** The kind stays in the path because a container may be named like another
 *  kind's only entry. */
export const ENTRY_ROUTE = `${ENTRY_LIST_PATH}/:kind/:id`;

export function entryListPath(kind?: EntryKind): string {
  return kind ? `${ENTRY_LIST_PATH}?kind=${kind}` : ENTRY_LIST_PATH;
}

export function entryPath(kind: EntryKind, key: string): string {
  return `${ENTRY_LIST_PATH}/${kind}/${encodeURIComponent(key)}`;
}

export function parseEntryPath(pathname: string): { kind: EntryKind; key: string } | null {
  const match = /^\/backups\/([^/]+)\/([^/]+)$/.exec(pathname);
  if (!match || !isEntryKind(match[1])) return null;
  const kind = match[1];
  let key: string;
  try {
    key = decodeURIComponent(match[2]);
  } catch {
    return null;
  }
  if (ENTRY_KIND[kind].keyedBy === "kind" && key !== kind) return null;
  return { kind, key };
}

/** Finds the key of the folder set or ZFS item with this name. */
export type EntryKeyLookup = (name: string) => string | undefined;

/**
 * legacyLookupName is the name a link to a kind's own page has to be looked
 * up by before it can be redirected, or "" when it needs no lookup. Such links
 * name their entry in ?item=, which is the key of a container or VM but only
 * the name of a folder set or ZFS item.
 */
export function legacyLookupName(kind: EntryKind, search: string): string {
  return ENTRY_KIND[kind].keyedBy === "id" ? (new URLSearchParams(search).get("item") ?? "") : "";
}

/**
 * legacyRedirect is where a link to a kind's own page leads. A link that
 * names an entry goes to that entry's page and keeps its other parameters,
 * which ask for a restore. Every other link, and one whose entry the lookup
 * does not find, goes to the list of that kind.
 */
export function legacyRedirect(kind: EntryKind, search: string, lookup?: EntryKeyLookup): string {
  const params = new URLSearchParams(search);
  const item = params.get("item") ?? "";
  params.delete("item");
  const { keyedBy } = ENTRY_KIND[kind];
  const key = keyedBy === "kind" ? kind : keyedBy === "name" ? item : item && lookup?.(item);
  if (!key) return entryListPath(kind);
  const query = params.toString();
  return entryPath(kind, key) + (query ? `?${query}` : "");
}
