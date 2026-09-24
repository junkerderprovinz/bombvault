// A finding that lost data links to the backup to restore from. The link lands
// on the item's own page as ?restore=<snapshot>&item=<name>, and that page
// opens the item's restore panel with the snapshot's choices already showing.

import { useState } from "react";

export type RestoreRequest = {
  /** The snapshot to offer first, "" without a request. */
  snapshot: string;
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
