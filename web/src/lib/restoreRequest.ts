// A finding that lost data links to the backup to restore from. The link lands
// on the item's own page as ?restore=<snapshot>&at=<time>&item=<name>, and
// that page opens the item's restore panel with the snapshot's choices already
// showing.

import { useState } from "react";
import type { Run } from "./api";
import { entryKey, entryPath, kindOfDomain } from "./backupEntry";

export type RestoreRequest = {
  /** The snapshot to offer first, "" without a request. */
  snapshot: string;
  /** When that snapshot was taken, in Unix seconds, so a page that no longer
   *  holds it can name the nearest one. 0 when the link leaves it out. */
  at: number;
  /** The row the request is for. The flash and config pages hold one item and
   *  leave it empty. */
  item: string;
  /** The request is for the item's database dump rather than its files. */
  dump: boolean;
  /** The dataset of a ZFS item's tree the snapshot belongs to, "" for every
   *  other page. */
  dataset: string;
};

export function readRestoreRequest(search: string): RestoreRequest {
  const params = new URLSearchParams(search);
  return {
    snapshot: params.get("restore") ?? "",
    at: Number(params.get("at")) || 0,
    item: params.get("item") ?? "",
    dump: params.get("dump") === "1",
    dataset: params.get("dataset") ?? "",
  };
}

/**
 * useRestoreRequest reads the request once, when the page mounts. The link
 * always comes from another page, and reading the address rather than the
 * router keeps the item pages renderable on their own in tests.
 */
export function useRestoreRequest(): RestoreRequest {
  const [request] = useState(() => readRestoreRequest(window.location.search));
  return request;
}

/**
 * runRestorePath is the restore link of the backup a run wrote, or null for
 * a run that left none: one that failed, or one that was no backup.
 */
export function runRestorePath(run: Run): string | null {
  const kind = kindOfDomain(run.domain);
  const wrote = (run.kind === "backup" || run.kind === "import") && run.status === "success" && run.snapshotId !== "";
  if (!kind || !wrote) return null;
  const key = entryKey(kind, { name: run.target, id: run.targetId });
  const params = new URLSearchParams({ restore: run.snapshotId, at: String(run.finishedAt ?? run.startedAt) });
  return `${entryPath(kind, key)}?${params.toString()}`;
}
