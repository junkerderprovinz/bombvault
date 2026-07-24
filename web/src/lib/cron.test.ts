// ---------------------------------------------------------------------------
// Cron validator + next-fire evaluator (#107). The grammar must mirror the
// backend's robfig/cron v3 standard parser EXACTLY — every "valid" row here
// was cross-checked against internal/schedule ParseCadence (which accepts raw
// 5-field cron), and every "invalid" row is one robfig rejects too (notably
// DOW 7, which classic cron allows but robfig does not).
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { cronPeriodSeconds, isValidCronExpression, nextCronFires } from "./cron";

describe("isValidCronExpression", () => {
  const valid = [
    "* * * * *",
    "0 */6 * * *",
    "30 2 * * 1-5",
    "0 3 1 * *",
    "0 8,20 * * *",
    "*/15 9-17 * * 1-5",
    "5/15 * * * *", // "a/step" = "a-max/step" (robfig special case)
    "0 12 * * MON,FRI",
    "0 0 1 JAN,JUL *",
    "0 0 * * mon-fri", // names are case-insensitive, ranges of names allowed
    "0 0 ? * ?", // "?" is an alias for "*"
    "59 23 31 12 6",
    "0 0 * * 0", // Sunday is 0
    "  0 4 * * *  ", // surrounding whitespace is tolerated (backend trims too)
  ];
  it.each(valid)("accepts %j", (expr) => {
    expect(isValidCronExpression(expr)).toBe(true);
  });

  const invalid = [
    "", // empty
    "   ", // whitespace only
    "0 */6 * *", // 4 fields
    "0 */6 * * * *", // 6 fields
    "60 * * * *", // minute out of range
    "0 24 * * *", // hour out of range
    "0 0 0 * *", // day-of-month min is 1
    "0 0 32 * *", // day-of-month max is 31
    "0 0 * 0 *", // month min is 1
    "0 0 * 13 *", // month max is 12
    "0 0 * * 7", // robfig rejects DOW 7 (Sunday is 0 only)
    "0 0 * * 1-7", // ...also as a range end
    "0 0 * * MOO", // unknown weekday name
    "MON * * * *", // names are not valid in the minute field
    "a b c d e", // garbage
    "1--2 * * * *", // malformed range
    "5-1 * * * *", // reversed range
    "*/0 * * * *", // step must be >= 1
    "1/2/3 * * * *", // at most one "/"
    "daily 02:00", // builder grammar is NOT cron
  ];
  it.each(invalid)("rejects %j", (expr) => {
    expect(isValidCronExpression(expr)).toBe(false);
  });
});

// hhmm collapses a fire to a comparable local "day HH:MM" tuple.
function stamp(d: Date): string {
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}

describe("nextCronFires", () => {
  it("returns null for an invalid expression (never a guessed preview)", () => {
    expect(nextCronFires("60 * * * *", 3)).toBeNull();
  });

  it("steps */6 hours", () => {
    const fires = nextCronFires("0 */6 * * *", 3, new Date(2026, 2, 10, 7, 15));
    expect(fires?.map(stamp)).toEqual(["2026-03-10 12:00", "2026-03-10 18:00", "2026-03-11 00:00"]);
  });

  it("handles minute lists", () => {
    const fires = nextCronFires("0 8,20 * * *", 3, new Date(2026, 6, 24, 0, 0));
    expect(fires?.map(stamp)).toEqual(["2026-07-24 08:00", "2026-07-24 20:00", "2026-07-25 08:00"]);
  });

  it("handles a/step minutes (5/15 = 5,20,35,50)", () => {
    const fires = nextCronFires("5/15 * * * *", 3, new Date(2026, 6, 24, 10, 0));
    expect(fires?.map(stamp)).toEqual(["2026-07-24 10:05", "2026-07-24 10:20", "2026-07-24 10:35"]);
  });

  it("handles weekday ranges (weekdays at 02:30 from a Saturday)", () => {
    // 2026-07-25 is a Saturday; the next weekday fires are Mon-Wed.
    const fires = nextCronFires("30 2 * * 1-5", 3, new Date(2026, 6, 25, 12, 0));
    expect(fires?.map(stamp)).toEqual(["2026-07-27 02:30", "2026-07-28 02:30", "2026-07-29 02:30"]);
    for (const f of fires ?? []) {
      expect(f.getDay()).toBeGreaterThanOrEqual(1);
      expect(f.getDay()).toBeLessThanOrEqual(5);
    }
  });

  it("handles day-of-month schedules (1st at 03:00)", () => {
    const fires = nextCronFires("0 3 1 * *", 2, new Date(2026, 6, 24, 12, 0));
    expect(fires?.map(stamp)).toEqual(["2026-08-01 03:00", "2026-09-01 03:00"]);
  });

  it("handles weekday names", () => {
    const fires = nextCronFires("0 12 * * MON,FRI", 4, new Date(2026, 6, 24, 13, 0));
    for (const f of fires ?? []) {
      expect([1, 5]).toContain(f.getDay());
      expect(f.getHours()).toBe(12);
    }
    expect(fires).toHaveLength(4);
  });

  it("handles month names", () => {
    const fires = nextCronFires("0 0 1 JAN,JUL *", 2, new Date(2026, 1, 15, 0, 0));
    expect(fires?.map(stamp)).toEqual(["2026-07-01 00:00", "2027-01-01 00:00"]);
  });

  it("uses OR semantics when both dom and dow are restricted (classic cron)", () => {
    // "13th of the month OR Friday". August 2026: the 13th is a Thursday,
    // Fridays are the 7th, 14th, 21st.
    const fires = nextCronFires("0 0 13 * 5", 3, new Date(2026, 7, 1, 0, 0));
    expect(fires?.map(stamp)).toEqual(["2026-08-07 00:00", "2026-08-13 00:00", "2026-08-14 00:00"]);
  });

  it("uses AND semantics when a day field is starred with a step (robfig star bit)", () => {
    // dom "*/2" carries robfig's star bit, so BOTH odd day AND Monday must
    // match. Mondays from 2026-07-24: Jul 27 (odd), Aug 3 (odd), Aug 10
    // (even, skipped), Aug 17 (odd).
    const fires = nextCronFires("0 0 */2 * 1", 3, new Date(2026, 6, 24, 0, 0));
    expect(fires?.map(stamp)).toEqual(["2026-07-27 00:00", "2026-08-03 00:00", "2026-08-17 00:00"]);
    for (const f of fires ?? []) {
      expect(f.getDay()).toBe(1);
      expect(f.getDate() % 2).toBe(1);
    }
  });

  it("returns an empty list for a schedule that never fires (Feb 30)", () => {
    expect(nextCronFires("0 0 30 2 *", 2, new Date(2026, 0, 1))).toEqual([]);
  });

  it("steps daily fires DST-safely (wall-clock time stays fixed, gaps stay ~24h)", () => {
    // 400 consecutive daily fires cross at least one DST transition in any
    // DST-observing zone. Every fire must land on the scheduled local time
    // and be 23-25 hours after the previous one (24h +/- the largest DST
    // shift) — never drift, never repeat, never go backwards.
    let cursor = new Date(2026, 0, 1, 0, 0);
    let prev: Date | null = null;
    for (let i = 0; i < 400; i++) {
      const fires = nextCronFires("30 12 * * *", 1, cursor);
      expect(fires).not.toBeNull();
      expect(fires).toHaveLength(1);
      const f = fires![0];
      expect(f.getHours()).toBe(12);
      expect(f.getMinutes()).toBe(30);
      if (prev) {
        const gapH = (f.getTime() - prev.getTime()) / 3600000;
        expect(gapH).toBeGreaterThanOrEqual(23);
        expect(gapH).toBeLessThanOrEqual(25);
      }
      prev = f;
      cursor = f;
    }
  });
});

describe("cronPeriodSeconds", () => {
  // Values mirror the backend's Cadence.PeriodSeconds for the same specs
  // (verified against internal/schedule: 21600 / 86400 / 604800 / 2678400).
  it.each([
    ["0 */6 * * *", 21600],
    ["0 5 * * *", 86400],
    ["15 4 * * 2", 604800],
    ["0 3 1 * *", 2678400],
  ] as const)("period of %j is %d s", (expr, want) => {
    expect(cronPeriodSeconds(expr)).toBe(want);
  });

  it("returns 0 for invalid or never-firing expressions", () => {
    expect(cronPeriodSeconds("not a cron")).toBe(0);
    expect(cronPeriodSeconds("0 0 30 2 *")).toBe(0);
  });
});
