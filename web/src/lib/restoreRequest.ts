// A finding that lost data links to the backup to restore from. The link lands
// on the entry's page as ?restore=<snapshot>&at=<time>, and that page opens
// the backup with what can be done with it.

export type RestoreRequest = {
  /** The snapshot to offer first, "" without a request. */
  snapshot: string;
  /** When that snapshot was taken, in Unix seconds, so a page that no longer
   *  holds it can name the nearest one. 0 when the link leaves it out. */
  at: number;
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
    dump: params.get("dump") === "1",
    dataset: params.get("dataset") ?? "",
  };
}
