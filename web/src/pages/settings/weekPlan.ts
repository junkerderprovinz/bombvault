// A week of planned runs, worked out from the cadences in the settings. A
// cadence says when a run starts and nothing about how long it takes, so the
// week holds starts only.

import { parseCadenceString } from "../../components/CadenceBuilder";

export type RunKind = "backup" | "copy" | "check" | "probe";

export interface Plan {
  key: string;
  kind: RunKind;
  label: string;
  cadence: string;
  /** The next time the plan fires, which places a plan that counts days. */
  next?: Date;
}

export interface PlannedRun {
  key: string;
  kind: RunKind;
  label: string;
  /** 0 is Monday. */
  day: number;
  /** Minutes after midnight on the server's clock. */
  minute: number;
}

const DAYS = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
const WEEK = [0, 1, 2, 3, 4, 5, 6];

function minuteOf(time: string): number {
  const [h, m] = time.split(":").map(Number);
  return h * 60 + (m || 0);
}

function cronField(src: string, lo: number, hi: number): Set<number> | null {
  const out = new Set<number>();
  for (const part of src.split(",")) {
    const m = /^(\*|(\d+)(?:-(\d+))?)(?:\/(\d+))?$/.exec(part);
    if (!m) return null;
    const step = m[4] ? Number(m[4]) : 1;
    const from = m[1] === "*" ? lo : Number(m[2]);
    const to = m[1] === "*" ? hi : m[3] !== undefined ? Number(m[3]) : m[4] ? hi : from;
    if (from < lo || to > hi || from > to || step < 1) return null;
    for (let v = from; v <= to; v += step) out.add(v);
  }
  return out;
}

// A cron line that names days of the month has no fixed place in a week, so
// the week leaves it out.
function cronStarts(expr: string): [number, number][] {
  const fields = expr.trim().split(/\s+/);
  if (fields.length !== 5) return [];
  const [minutes, hours, monthDays, months, weekDays] = [
    cronField(fields[0], 0, 59),
    cronField(fields[1], 0, 23),
    cronField(fields[2], 1, 31),
    cronField(fields[3], 1, 12),
    cronField(fields[4], 0, 7),
  ];
  if (!minutes || !hours || !monthDays || !months || !weekDays) return [];
  if (monthDays.size < 31 || months.size < 12) return [];
  if (weekDays.has(7)) weekDays.add(0);
  const starts: [number, number][] = [];
  for (const day of WEEK) {
    if (!weekDays.has((day + 1) % 7)) continue;
    for (const h of hours) for (const m of minutes) starts.push([day, h * 60 + m]);
  }
  return starts.slice(0, 200);
}

/**
 * planStarts lists a cadence's starts in one week as [day, minute]. A plan
 * that runs every N days is counted on from its next fire, and stays out of
 * the week while that is not known.
 */
export function planStarts(cadence: string, next?: Date, serverOffsetSeconds = 0): [number, number][] {
  const c = parseCadenceString(cadence);
  switch (c.mode) {
    case "off":
      return [];
    case "daily":
      return WEEK.map((d) => [d, minuteOf(c.time)]);
    case "weekly":
      return WEEK.filter((d) => c.weekdays.includes(DAYS[d])).map((d) => [d, minuteOf(c.time)]);
    case "everyN": {
      if (c.intervalDays <= 1) return WEEK.map((d) => [d, minuteOf(c.time)]);
      if (!next) return [];
      const first = (new Date(next.getTime() + serverOffsetSeconds * 1000).getUTCDay() + 6) % 7;
      return WEEK.filter((d) => (d - first + 7 * c.intervalDays) % c.intervalDays === 0).map((d) => [
        d,
        minuteOf(c.time),
      ]);
    }
    case "cron":
      return cronStarts(c.cron);
  }
}

export function weekRuns(plans: Plan[], serverOffsetSeconds = 0): PlannedRun[] {
  return plans.flatMap((p) =>
    planStarts(p.cadence, p.next, serverOffsetSeconds).map(([day, minute]) => ({
      key: p.key,
      kind: p.kind,
      label: p.label,
      day,
      minute,
    }))
  );
}

export interface Crowd {
  plans: number;
  from: number;
  to: number;
}

/**
 * crowdOf finds the tightest stretch of a day in which three or more plans
 * start within two hours of each other. They share the disk and the line.
 */
export function crowdOf(day: PlannedRun[]): Crowd | null {
  const starts = [...day].sort((a, b) => a.minute - b.minute);
  let best: Crowd | null = null;
  for (let i = 0; i < starts.length; i++) {
    let j = i;
    while (j + 1 < starts.length && starts[j + 1].minute - starts[i].minute <= 120) j++;
    const plans = new Set(starts.slice(i, j + 1).map((r) => r.key)).size;
    if (plans >= 3 && (!best || plans > best.plans)) best = { plans, from: starts[i].minute, to: starts[j].minute };
  }
  return best;
}

export function clock(minute: number): string {
  const h = Math.floor(minute / 60) % 24;
  return `${String(h).padStart(2, "0")}:${String(minute % 60).padStart(2, "0")}`;
}
