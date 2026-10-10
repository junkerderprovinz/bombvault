import { describe, expect, it } from "vitest";
import {
  KEEP_PRESETS,
  keepPlan,
  keepsFewer,
  lostByChange,
  presetOf,
  sampleBackups,
  type KeepCounts,
  type KeepInput,
} from "./keepPlan";

const at = (...times: string[]): KeepInput[] => times.map((time) => ({ time }));

function kept(counts: KeepCounts, backups: KeepInput[]): string[] {
  return keepPlan(counts, backups)
    .marks.filter((m) => m.kept)
    .map((m) => m.backup.time.slice(0, 16));
}

// The expected sets in this file are what restic 0.17.3 keeps of the same
// snapshots with the same --keep-* counts.

const monthEnd = at(
  "2024-01-30T03:00:00+01:00",
  "2024-01-31T03:00:00+01:00",
  "2024-01-31T15:00:00+01:00",
  "2024-02-01T03:00:00+01:00",
  "2024-02-02T03:00:00+01:00",
);

// 2020 has an ISO week 53, which runs from 28 December to 3 January.
const week53 = at(
  "2020-12-20T03:00:00+01:00",
  "2020-12-27T03:00:00+01:00",
  "2020-12-28T03:00:00+01:00",
  "2020-12-31T03:00:00+01:00",
  "2021-01-02T03:00:00+01:00",
  "2021-01-03T03:00:00+01:00",
  "2021-01-04T03:00:00+01:00",
  "2021-01-05T03:00:00+01:00",
);

// 30 December 2024 is a Monday and opens week 1 of 2025.
const yearEnd = at(
  "2023-06-01T03:00:00+02:00",
  "2024-12-29T03:00:00+01:00",
  "2024-12-30T03:00:00+01:00",
  "2024-12-31T23:30:00+01:00",
  "2025-01-01T00:30:00+01:00",
  "2025-01-02T03:00:00+01:00",
);

describe("presets", () => {
  it("names the preset a set of counts equals", () => {
    expect(presetOf([0, 7, 4, 3, 0])).toBe("short");
    expect(presetOf([0, 7, 4, 6, 1])).toBe("balanced");
    expect(presetOf([0, 14, 8, 12, 3])).toBe("long");
  });

  it("calls every other set of counts custom", () => {
    expect(presetOf([1, 7, 4, 6, 1])).toBe("custom");
    expect(presetOf([0, 0, 0, 0, 0])).toBe("custom");
  });

  it("maps each preset back to itself", () => {
    for (const preset of ["short", "balanced", "long"] as const) {
      expect(presetOf(KEEP_PRESETS[preset])).toBe(preset);
    }
  });
});

describe("keepPlan", () => {
  it("keeps the newest backups by count", () => {
    expect(kept([3, 0, 0, 0, 0], monthEnd)).toEqual(["2024-02-02T03:00", "2024-02-01T03:00", "2024-01-31T15:00"]);
  });

  it("keeps the newest backup of each day", () => {
    expect(kept([0, 3, 0, 0, 0], monthEnd)).toEqual(["2024-02-02T03:00", "2024-02-01T03:00", "2024-01-31T15:00"]);
  });

  it("keeps the newest backup of each month across a month end", () => {
    expect(kept([0, 0, 0, 2, 0], monthEnd)).toEqual(["2024-02-02T03:00", "2024-01-31T15:00"]);
  });

  it("puts the days around new year into ISO week 53", () => {
    expect(kept([0, 0, 2, 0, 0], week53)).toEqual(["2021-01-05T03:00", "2021-01-03T03:00"]);
    expect(kept([0, 0, 3, 0, 0], week53)).toEqual(["2021-01-05T03:00", "2021-01-03T03:00", "2020-12-27T03:00"]);
  });

  it("counts the last days of December as week 1 of the next year", () => {
    expect(kept([0, 0, 2, 0, 0], yearEnd)).toEqual(["2025-01-02T03:00", "2024-12-29T03:00"]);
  });

  it("keeps the newest backup of each year across a year end", () => {
    expect(kept([0, 0, 0, 0, 2], yearEnd)).toEqual(["2025-01-02T03:00", "2024-12-31T23:30"]);
    expect(kept([0, 0, 0, 2, 0], week53)).toEqual(["2021-01-05T03:00", "2020-12-31T03:00"]);
  });

  it("also keeps the oldest backup while a count is not used up", () => {
    expect(kept([0, 0, 0, 0, 2], monthEnd)).toEqual(["2024-02-02T03:00", "2024-01-30T03:00"]);
    expect(kept([0, 0, 0, 0, 1], monthEnd)).toEqual(["2024-02-02T03:00"]);
  });

  it("keeps a backup that any one count keeps", () => {
    expect(kept([2, 2, 2, 2, 2], yearEnd)).toEqual([
      "2025-01-02T03:00",
      "2025-01-01T00:30",
      "2024-12-31T23:30",
      "2024-12-29T03:00",
    ]);
  });

  it("names every count that keeps a backup", () => {
    const { marks } = keepPlan([2, 2, 2, 2, 2], yearEnd);
    expect(marks.map((m) => m.buckets)).toEqual([
      ["last", "daily", "weekly", "monthly", "yearly"],
      ["last", "daily"],
      ["monthly", "yearly"],
      [],
      ["weekly"],
      [],
    ]);
  });

  it("removes nothing when no count is set", () => {
    const plan = keepPlan([0, 0, 0, 0, 0], monthEnd);
    expect(plan.keepsAll).toBe(true);
    expect(plan.marks.map((m) => m.kept)).toEqual([true, true, true, true, true]);
    expect(plan.marks.every((m) => m.buckets.length === 0)).toBe(true);
  });

  it("returns the marks newest first whatever the order of the list", () => {
    const shuffled = [monthEnd[3], monthEnd[0], monthEnd[4], monthEnd[2], monthEnd[1]];
    expect(keepPlan([1, 0, 0, 0, 0], shuffled).marks.map((m) => m.backup)).toEqual([...monthEnd].reverse());
  });

  it("reads the day in the offset the backup was stamped with", () => {
    // In UTC both are 1 January 2025; on the machine that wrote them the first
    // is still New Year's Eve.
    const backups = at("2024-12-31T23:30:00.123456789-05:00", "2025-01-01T00:30:00.5-05:00");
    expect(kept([0, 0, 0, 0, 2], backups)).toEqual(["2025-01-01T00:30", "2024-12-31T23:30"]);
    expect(kept([0, 1, 0, 0, 0], backups)).toEqual(["2025-01-01T00:30"]);
  });

  it("keeps a held backup and still counts it in its buckets", () => {
    const backups: KeepInput[] = [
      { time: "2024-02-02T03:00:00+01:00" },
      { time: "2024-02-01T15:00:00+01:00", held: true },
      { time: "2024-02-01T03:00:00+01:00" },
      { time: "2024-01-31T03:00:00+01:00", held: true },
      { time: "2024-01-30T03:00:00+01:00" },
    ];
    const { marks } = keepPlan([0, 2, 0, 0, 0], backups);
    expect(marks.map((m) => m.kept)).toEqual([true, true, false, true, false]);
    expect(marks.map((m) => m.buckets)).toEqual([["daily"], ["daily"], [], [], []]);
  });
});

describe("changing a rule", () => {
  it("lists what a shorter rule removes", () => {
    expect(lostByChange([0, 7, 0, 0, 0], [0, 2, 0, 0, 0], monthEnd).map((b) => b.time.slice(0, 16))).toEqual([
      "2024-01-31T15:00",
      "2024-01-30T03:00",
    ]);
    expect(keepsFewer([0, 7, 0, 0, 0], [0, 2, 0, 0, 0], monthEnd)).toBe(true);
  });

  it("asks nothing when the new rule keeps at least the same backups", () => {
    expect(keepsFewer([0, 2, 0, 0, 0], [0, 7, 0, 0, 0], monthEnd)).toBe(false);
    expect(keepsFewer([0, 7, 4, 3, 0], [0, 7, 4, 3, 0], monthEnd)).toBe(false);
  });

  it("looks at the backups, not at the numbers", () => {
    // Four days hold a backup, so three days fewer than seven changes nothing
    // here and one day fewer than four does.
    expect(keepsFewer([0, 7, 0, 0, 0], [0, 4, 0, 0, 0], monthEnd)).toBe(false);
    expect(keepsFewer([0, 4, 0, 0, 0], [0, 3, 0, 0, 0], monthEnd)).toBe(true);
  });

  it("treats the first rule after none as shorter", () => {
    expect(keepsFewer([0, 0, 0, 0, 0], [0, 2, 0, 0, 0], monthEnd)).toBe(true);
  });

  it("never loses a backup by switching the rule off", () => {
    expect(keepsFewer([0, 2, 0, 0, 0], [0, 0, 0, 0, 0], monthEnd)).toBe(false);
  });

  it("counts a backup the new rule keeps by another count as kept", () => {
    expect(lostByChange([0, 3, 0, 0, 0], [3, 0, 0, 0, 0], monthEnd)).toEqual([]);
  });
});

describe("sampleBackups", () => {
  const sample = sampleBackups(new Date("2026-10-10T10:00:00Z"));

  it("is forty backups ending now, newest first", () => {
    expect(sample).toHaveLength(40);
    expect(sample[0].time).toBe("2026-10-10T10:00:00.000Z");
    expect(sample[39].time).toBe("2020-05-14T10:00:00.000Z");
    const times = sample.map((b) => Date.parse(b.time));
    expect(times).toEqual([...times].sort((a, b) => b - a));
  });

  it("shows each preset keeping more than the shorter one", () => {
    const count = (counts: KeepCounts) => keepPlan(counts, sample).marks.filter((m) => m.kept).length;
    expect(count(KEEP_PRESETS.short)).toBe(11);
    expect(count(KEEP_PRESETS.balanced)).toBe(14);
    expect(count(KEEP_PRESETS.long)).toBe(29);
  });
});
