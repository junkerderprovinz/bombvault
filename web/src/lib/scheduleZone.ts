// A schedule's "03:00" is read on the server's clock, while every time stamp on
// the page is in the browser's zone. Where the two differ, a schedule line names
// the server's clock, or "daily at 3:00" and "next run at 05:00" read as two
// different runs.

import { useEffect, useState } from "react";
import { getSettings, type ScheduleZone } from "./api";

/** The browser's own offset from UTC, in seconds east. */
export function browserOffsetSeconds(at = new Date()): number {
  return -at.getTimezoneOffset() * 60;
}

/** The server's clock when it differs from the browser's, else null. */
export function foreignZone(zone: ScheduleZone | null | undefined, browserOffset: number): ScheduleZone | null {
  return zone && zone.offsetSeconds !== browserOffset ? zone : null;
}

/** An offset as "UTC" or "UTC+02:00", which reads the same in every language. */
export function zoneLabel(zone: ScheduleZone): string {
  const off = zone.offsetSeconds;
  if (off === 0) return "UTC";
  const abs = Math.abs(off);
  const hh = String(Math.floor(abs / 3600)).padStart(2, "0");
  const mm = String(Math.floor((abs % 3600) / 60)).padStart(2, "0");
  return `UTC${off < 0 ? "-" : "+"}${hh}:${mm}`;
}

// One request serves every schedule line on the page. A failed one is not
// kept, so the next line to mount asks again.
let pending: Promise<ScheduleZone | null> | null = null;

function loadZone(): Promise<ScheduleZone | null> {
  pending ??= getSettings()
    .then((res) => res.scheduleZone ?? null)
    .catch(() => null)
    .then((zone) => {
      if (!zone) pending = null;
      return zone;
    });
  return pending;
}

/** useForeignScheduleZone is the server's clock while it differs from the
 *  browser's, and null before the answer arrives or when they agree. */
export function useForeignScheduleZone(): ScheduleZone | null {
  const [zone, setZone] = useState<ScheduleZone | null>(null);
  useEffect(() => {
    let alive = true;
    void loadZone().then((z) => {
      if (alive) setZone(foreignZone(z, browserOffsetSeconds()));
    });
    return () => {
      alive = false;
    };
  }, []);
  return zone;
}
