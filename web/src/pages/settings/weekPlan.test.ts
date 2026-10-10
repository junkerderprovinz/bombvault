import { describe, expect, it } from "vitest";
import { clock, crowdOf, planStarts, weekRuns, type PlannedRun } from "./weekPlan";

const days = (starts: [number, number][]) => starts.map(([d]) => d);

describe("planStarts", () => {
  it("has no start for a cadence that is off or empty", () => {
    expect(planStarts("off")).toEqual([]);
    expect(planStarts("")).toEqual([]);
  });

  it("starts a daily plan on all seven days at its time", () => {
    const starts = planStarts("daily 03:30");
    expect(days(starts)).toEqual([0, 1, 2, 3, 4, 5, 6]);
    expect(starts.every(([, minute]) => minute === 210)).toBe(true);
  });

  it("starts a weekly plan on its days, counted from Monday", () => {
    expect(planStarts("weekly Sun,Wed 04:00")).toEqual([
      [2, 240],
      [6, 240],
    ]);
  });

  it("counts a plan that runs every N days on from its next fire", () => {
    // 2026-10-14 is a Wednesday.
    const next = new Date("2026-10-14T02:00:00Z");
    expect(days(planStarts("everyN 3 02:00", next))).toEqual([2, 5]);
    expect(days(planStarts("everyN 2 02:00", next))).toEqual([0, 2, 4, 6]);
  });

  it("places the next fire on the server's day, which can be another one than UTC's", () => {
    const next = new Date("2026-10-14T23:30:00Z");
    expect(days(planStarts("everyN 7 01:30", next, 2 * 3600))).toEqual([3]);
  });

  it("leaves a plan that counts days out while its next fire is not known", () => {
    expect(planStarts("everyN 3 02:00")).toEqual([]);
  });

  it("treats every one day as daily", () => {
    expect(days(planStarts("everyN 1 02:00"))).toEqual([0, 1, 2, 3, 4, 5, 6]);
  });

  it("reads the weekdays, hours and minutes of a cron line", () => {
    expect(planStarts("30 2 * * 1-5")).toEqual([
      [0, 150],
      [1, 150],
      [2, 150],
      [3, 150],
      [4, 150],
    ]);
    expect(planStarts("0 */12 * * 0")).toEqual([
      [6, 0],
      [6, 720],
    ]);
    expect(days(planStarts("0 3 * * 7"))).toEqual([6]);
  });

  it("leaves out a cron line tied to days of the month, and one it cannot read", () => {
    expect(planStarts("0 3 1 * *")).toEqual([]);
    expect(planStarts("0 3 * 6 *")).toEqual([]);
    expect(planStarts("every tuesday")).toEqual([]);
    expect(planStarts("61 3 * * *")).toEqual([]);
  });
});

describe("weekRuns", () => {
  it("gives each start the key, kind and name of its plan", () => {
    const runs = weekRuns([
      { key: "containers", kind: "backup", label: "Containers", cadence: "weekly Mon 03:00" },
      { key: "tamper", kind: "check", label: "Tamper test", cadence: "off" },
    ]);
    expect(runs).toEqual([{ key: "containers", kind: "backup", label: "Containers", day: 0, minute: 180 }]);
  });
});

describe("crowdOf", () => {
  const run = (key: string, minute: number): PlannedRun => ({ key, kind: "backup", label: key, day: 0, minute });

  it("finds nothing while fewer than three plans start within two hours", () => {
    expect(crowdOf([run("a", 180), run("b", 200)])).toBeNull();
    expect(crowdOf([run("a", 180), run("b", 200), run("c", 301)])).toBeNull();
  });

  it("marks the stretch from the first to the last of three plans that do", () => {
    expect(crowdOf([run("c", 300), run("a", 180), run("b", 200)])).toEqual({ plans: 3, from: 180, to: 300 });
  });

  it("counts plans and not starts, so one plan running hourly crowds nothing", () => {
    expect(crowdOf([run("a", 180), run("a", 240), run("a", 300)])).toBeNull();
  });

  it("picks the stretch with the most plans", () => {
    const day = [run("a", 0), run("b", 30), run("c", 60), run("d", 600), run("e", 610), run("f", 620), run("g", 630)];
    expect(crowdOf(day)).toEqual({ plans: 4, from: 600, to: 630 });
  });
});

describe("clock", () => {
  it("writes minutes after midnight as a time of day", () => {
    expect(clock(0)).toBe("00:00");
    expect(clock(185)).toBe("03:05");
    expect(clock(1439)).toBe("23:59");
  });
});
