import type { ResticSnapshot } from "../../../lib/restic";

// ---------------------------------------------------------------------------
// SnapshotRow — the display model rendered by the snapshots page
// ---------------------------------------------------------------------------

export interface SnapshotRow {
  /** Full restic snapshot id (40-char hex). */
  id: string;
  /** Short id as returned by restic (8-char hex). */
  shortId: string;
  /** ISO 8601 timestamp string of the snapshot. */
  time: string;
  /** Tags associated with the snapshot (empty array if none). */
  tags: string[];
}

/**
 * Pure view-model function: convert a list of ResticSnapshot objects into
 * SnapshotRow display models ready for rendering.
 *
 * @param snaps - ResticSnapshot[] from the restic snapshots() call.
 */
export function toSnapshotRows(snaps: ResticSnapshot[]): SnapshotRow[] {
  return snaps.map((snap) => ({
    id: snap.id,
    shortId: snap.short_id,
    time: snap.time,
    tags: snap.tags ?? [],
  }));
}
