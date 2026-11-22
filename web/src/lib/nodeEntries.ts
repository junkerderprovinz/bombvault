// A node lists its containers and folder sets through the routes it answers
// home on. The list and its rows take them as entries like this instance's own.
import type { BackupItem, Container, FileSetView } from "./api";

/** The kinds of entry a node backs up. */
export type NodeEntryKind = "container" | "files";

export const NODE_ENTRY_KINDS: NodeEntryKind[] = ["container", "files"];

// Home plans a node's schedules, not the node, whose own stay off, so an
// entry the node keeps in its schedule counts as one that runs.
const PLANNED = { kind: "domain", spec: "" } as const;

export function containerItem(c: Container): BackupItem {
  return {
    kind: "container",
    key: c.name,
    name: c.name,
    included: c.includeInSchedule,
    paused: false,
    effectiveSchedule: PLANNED,
    lastBackup: c.lastBackup ?? 0,
    lastRunStatus: "",
    sourceBytes: null,
    state: c.state,
    installed: c.installed,
    self: c.self,
    ip: c.ip,
    image: c.image,
    stack: c.stack,
    runs: [],
  };
}

export function fileSetItem(f: FileSetView): BackupItem {
  return {
    kind: "files",
    key: f.id,
    name: f.name,
    included: f.enabled,
    paused: false,
    effectiveSchedule: PLANNED,
    lastBackup: f.lastBackup,
    lastRunStatus: "",
    sourceBytes: null,
    runs: [],
  };
}

export const NODE_ENTRY_ROUTE = "/nodes/:node/backups/:kind/:key";

export function nodeEntryPath(node: string, kind: NodeEntryKind, key: string): string {
  return `/nodes/${encodeURIComponent(node)}/backups/${kind}/${encodeURIComponent(key)}`;
}

/** The entry the route's parameters name, null for a kind a node has none of. */
export function parseNodeEntryPath(node?: string, kind?: string, key?: string): { node: string; kind: NodeEntryKind; key: string } | null {
  if (!node || !key || !NODE_ENTRY_KINDS.includes(kind as NodeEntryKind)) return null;
  return { node, kind: kind as NodeEntryKind, key };
}
