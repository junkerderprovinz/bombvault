// ---------------------------------------------------------------------------
// Minimal 5-field cron parsing + next-fire evaluation for the CadenceBuilder's
// "Cron" mode (#107). Pure logic, no dependency.
//
// The grammar deliberately mirrors robfig/cron v3's STANDARD parser — the
// backend's validity authority (internal/schedule ParseCadence): per field
// "*" / "?", single values, "a-b" ranges, an optional "/step" (where "a/step"
// means "a-max/step"), comma lists, JAN-DEC month names and SUN-SAT weekday
// names (case-insensitive, ranges of names allowed), and day-of-week 0-6
// (7 is NOT Sunday here — robfig rejects it, so we must too). Matching the
// backend exactly means the client never green-lights an expression the save
// will bounce, and never blocks one the backend would take.
//
// The evaluator exists ONLY for the preview ("next: …"). It must never show a
// WRONG fire time — callers degrade to a plain "valid expression" note when it
// returns null or finds no fire within the search horizon (e.g. "0 0 30 2 *",
// which never fires). Date stepping uses local wall-clock Date arithmetic, so
// DST gaps are skipped forward the way a local cron daemon behaves.
// ---------------------------------------------------------------------------

const MONTH_NAMES: Record<string, number> = {
  jan: 1, feb: 2, mar: 3, apr: 4, may: 5, jun: 6,
  jul: 7, aug: 8, sep: 9, oct: 10, nov: 11, dec: 12,
};

const DOW_NAMES: Record<string, number> = {
  sun: 0, mon: 1, tue: 2, wed: 3, thu: 4, fri: 5, sat: 6,
};

interface FieldBounds {
  min: number;
  max: number;
  names?: Record<string, number>;
}

// minute, hour, day-of-month, month, day-of-week — in expression order.
const FIELD_BOUNDS: FieldBounds[] = [
  { min: 0, max: 59 },
  { min: 0, max: 23 },
  { min: 1, max: 31 },
  { min: 1, max: 12, names: MONTH_NAMES },
  { min: 0, max: 6, names: DOW_NAMES },
];

interface ParsedField {
  values: Set<number>;
  /** True when any list item starts with "*" or "?" — robfig's "star bit",
   *  which flips the dom/dow combination from OR to AND (see dayMatches). */
  star: boolean;
}

/** One parsed 5-field cron expression. Field names follow crontab order. */
export interface CronSchedule {
  minute: ParsedField;
  hour: ParsedField;
  dom: ParsedField;
  month: ParsedField;
  dow: ParsedField;
}

// parseBound resolves one range endpoint: a number in bounds or a known name.
function parseBound(token: string, b: FieldBounds): number | null {
  if (b.names) {
    const named = b.names[token.toLowerCase()];
    if (named !== undefined) return named;
  }
  if (!/^\d+$/.test(token)) return null;
  const n = parseInt(token, 10);
  if (n < b.min || n > b.max) return null;
  return n;
}

// parseField parses one comma-list field ("*/6", "1-5", "8,20", "mon-fri", …).
function parseField(expr: string, b: FieldBounds): ParsedField | null {
  if (expr === "") return null;
  const values = new Set<number>();
  let star = false;
  for (const item of expr.split(",")) {
    const rangeAndStep = item.split("/");
    if (rangeAndStep.length > 2) return null;
    let step = 1;
    if (rangeAndStep.length === 2) {
      if (!/^\d+$/.test(rangeAndStep[1])) return null;
      step = parseInt(rangeAndStep[1], 10);
      if (step < 1) return null;
    }
    const range = rangeAndStep[0];
    let start: number;
    let end: number;
    if (range === "*" || range === "?") {
      start = b.min;
      end = b.max;
      star = true;
    } else {
      const lowHigh = range.split("-");
      if (lowHigh.length > 2) return null;
      const lo = parseBound(lowHigh[0], b);
      if (lo === null) return null;
      start = lo;
      if (lowHigh.length === 2) {
        const hi = parseBound(lowHigh[1], b);
        if (hi === null) return null;
        end = hi;
      } else {
        // robfig: a bare value with a step ("5/15") means "5-max/15".
        end = rangeAndStep.length === 2 ? b.max : lo;
      }
    }
    if (start > end) return null;
    for (let v = start; v <= end; v += step) values.add(v);
  }
  return values.size > 0 ? { values, star } : null;
}

/**
 * parseCronExpression parses a 5-field cron expression, or returns null when
 * it is not valid under the backend's grammar (see the header comment).
 */
export function parseCronExpression(expr: string): CronSchedule | null {
  const fields = expr.trim().split(/\s+/);
  if (fields.length !== 5) return null;
  const parsed: ParsedField[] = [];
  for (let i = 0; i < 5; i++) {
    const f = parseField(fields[i], FIELD_BOUNDS[i]);
    if (f === null) return null;
    parsed.push(f);
  }
  return { minute: parsed[0], hour: parsed[1], dom: parsed[2], month: parsed[3], dow: parsed[4] };
}

/** isValidCronExpression reports whether the backend would accept this string
 *  as a raw cron cadence (structure + per-field ranges/names). */
export function isValidCronExpression(expr: string): boolean {
  return parseCronExpression(expr) !== null;
}

// dayMatches mirrors robfig's dom/dow combination: when either field carries
// the star bit the two must BOTH match (the starred one trivially does);
// when both are restricted, EITHER matching suffices (classic cron OR).
function dayMatches(s: CronSchedule, t: Date): boolean {
  const dom = s.dom.values.has(t.getDate());
  const dow = s.dow.values.has(t.getDay());
  if (s.dom.star || s.dow.star) return dom && dow;
  return dom || dow;
}

// nextFire finds the first fire strictly after `after`, or null when none
// exists within a ~5-year horizon (e.g. "0 0 30 2 *"). Local wall-clock
// stepping: the Date constructor normalizes over DST gaps, so a fire time
// falling into a skipped hour moves forward, like a local cron daemon.
function nextFire(s: CronSchedule, after: Date): Date | null {
  let t = new Date(after.getFullYear(), after.getMonth(), after.getDate(), after.getHours(), after.getMinutes() + 1, 0, 0);
  const yearLimit = after.getFullYear() + 5;
  while (t.getFullYear() <= yearLimit) {
    if (!s.month.values.has(t.getMonth() + 1)) {
      t = new Date(t.getFullYear(), t.getMonth() + 1, 1, 0, 0, 0, 0);
      continue;
    }
    if (!dayMatches(s, t)) {
      t = new Date(t.getFullYear(), t.getMonth(), t.getDate() + 1, 0, 0, 0, 0);
      continue;
    }
    if (!s.hour.values.has(t.getHours())) {
      t = new Date(t.getFullYear(), t.getMonth(), t.getDate(), t.getHours() + 1, 0, 0, 0);
      continue;
    }
    if (!s.minute.values.has(t.getMinutes())) {
      t = new Date(t.getFullYear(), t.getMonth(), t.getDate(), t.getHours(), t.getMinutes() + 1, 0, 0);
      continue;
    }
    return t;
  }
  return null;
}

/**
 * nextCronFires returns the next `count` fire times of `expr` after `from`
 * (default: now), soonest first. Returns null when the expression is invalid;
 * returns fewer (possibly zero) dates when the schedule stops firing within
 * the search horizon. Callers must treat a short result as "no preview",
 * never invent times.
 */
export function nextCronFires(expr: string, count: number, from: Date = new Date()): Date[] | null {
  const sched = parseCronExpression(expr);
  if (sched === null) return null;
  const out: Date[] = [];
  let cursor = from;
  for (let i = 0; i < count; i++) {
    const next = nextFire(sched, cursor);
    if (next === null) break;
    out.push(next);
    cursor = next;
  }
  return out;
}

/**
 * cronPeriodSeconds approximates how often the expression fires, in seconds —
 * the gap between its first two fires from a fixed base, mirroring the
 * backend's Cadence.PeriodSeconds. Returns 0 when the expression is invalid
 * or the gap cannot be computed, so callers can treat 0 as "unknown".
 */
export function cronPeriodSeconds(expr: string): number {
  const sched = parseCronExpression(expr);
  if (sched === null) return 0;
  // Fixed local base (2000-01-01 was a Saturday) keeps the result stable
  // regardless of when it is computed — same trick as the backend.
  const base = new Date(2000, 0, 1, 0, 0, 0, 0);
  const first = nextFire(sched, base);
  if (first === null) return 0;
  const second = nextFire(sched, first);
  if (second === null) return 0;
  const gap = (second.getTime() - first.getTime()) / 1000;
  return gap > 0 ? gap : 0;
}
