import type { RepoSource } from "../components/SourceToggle";
import type { TimelineMark, TimelinePlace, TimelineRow } from "./api";

/** The code a restore answers with when the chosen place no longer holds the id. */
export const SNAPSHOT_MISSING = "snapshot-missing";

function newestFirst(a: TimelineRow, b: TimelineRow): number {
  return Date.parse(b.time) - Date.parse(a.time) || a.key.localeCompare(b.key);
}

/** mergePlaceRows puts one place's rows into the list by key, never by time.
 *  The place's earlier marks go first, so a second load leaves no double. */
export function mergePlaceRows(rows: TimelineRow[], place: string, placeRows: TimelineRow[]): TimelineRow[] {
  const byKey = new Map<string, TimelineRow>();
  for (const row of rows) {
    const marks = row.places.filter((m) => m.place !== place);
    if (marks.length > 0) byKey.set(row.key, { ...row, places: marks });
  }
  for (const row of placeRows) {
    const have = byKey.get(row.key);
    byKey.set(row.key, have ? { ...have, places: [...have.places, ...row.places] } : row);
  }
  return [...byKey.values()].sort(newestFirst);
}

function rank(p: TimelinePlace): number {
  if (p.kind === "home") return 0;
  return p.remote ? 2 : 1;
}

/** autoMark is where a restore goes unless the row says otherwise: the first
 *  read place holding the whole backup, the item's location before targets. */
export function autoMark(row: TimelineRow, places: TimelinePlace[]): TimelineMark | null {
  const order = places.filter((p) => p.state === "read").sort((a, b) => rank(a) - rank(b));
  for (const p of order) {
    const mark = row.places.find((m) => m.place === p.place && !m.incomplete);
    if (mark) return mark;
  }
  return null;
}

export function newestId(mark: TimelineMark): string {
  return mark.snapshotIds[0];
}

export function sourceOfPlace(place: string): RepoSource {
  return place === "local" ? "local" : (place as `offsite:${string}`);
}
