// ---------------------------------------------------------------------------
// activityLog — happy-path tests for the pure merge/filter entry points.
// buildLogLines/filterLogLines are deliberately framework-free (see the module
// doc comment), so these run in the node environment with a stub resolver:
// resolveName renders "key a=1 b=2", which keeps the translation key AND the
// interpolated params assertable without any i18n context.
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { buildLogLines, filterLogLines, formatLogDate } from "./activityLog";
import type { LogLine } from "./activityLog";
import type { Run, ScheduleNext } from "./api";
import type { ProgressMap } from "./progress";

const resolveName = (key: string, params?: Record<string, string>): string =>
  params
    ? `${key} ${Object.entries(params)
        .map(([k, v]) => `${k}=${v}`)
        .join(" ")}`
    : key;

function makeRun(over: Partial<Run>): Run {
  return {
    id: "r1",
    targetId: "c-1",
    kind: "backup",
    status: "success",
    startedAt: 1000,
    finishedAt: 1030,
    snapshotId: "snap",
    bytes: 2048,
    error: "",
    target: "plex",
    domain: "container",
    ...over,
  };
}

describe("buildLogLines", () => {
  it("renders a finished backup run as a localized success line", () => {
    const lines = buildLogLines([makeRun({})], {}, [], resolveName, 2_000_000);
    expect(lines).toHaveLength(1);
    const line = lines[0];
    expect(line.id).toBe("run:r1");
    expect(line.status).toBe("success");
    expect(line.domain).toBe("containers"); // singular run domain → plural literal
    expect(line.kind).toBe("backup");
    expect(line.live).toBe(false);
    expect(line.atMs).toBe(1030 * 1000); // ordered by finish time, in ms
    expect(line.text).toContain("activityLog.lineBackupSuccess");
    expect(line.text).toContain("name=plex");
    expect(line.text).toContain("bytes=2.0 KB");
    expect(line.text).toContain("duration=30s");
  });

  it("keeps live lines last and suppresses the finished run they supersede", () => {
    const progress: ProgressMap = {
      "container:plex": { phase: "backup", percent: 41.4, active: true, lastSeen: 5_000_000 },
    };
    const other = makeRun({ id: "r0", target: "sonarr", finishedAt: 900 });
    const lines = buildLogLines([other, makeRun({})], progress, [], resolveName, 5_000_000);
    expect(lines.map((l) => l.id)).toEqual(["run:r0", "live:container:plex"]);
    const live = lines[1];
    expect(live.live).toBe(true);
    expect(live.status).toBe("running");
    expect(live.text).toContain("activityLog.lineBackingUpItem");
    expect(live.text).toContain("percent=41"); // clamped + rounded
  });

  it("appends the idle next-up line only when nothing is active", () => {
    const next: ScheduleNext[] = [
      { job: "backup", domain: "containers", next: new Date(7_200_000).toISOString() },
    ];
    const lines = buildLogLines([], {}, next, resolveName, 3_600_000);
    expect(lines).toHaveLength(1);
    expect(lines[0].idle).toBe(true);
    expect(lines[0].text).toContain("activityLog.lineNextWithDomain");
    expect(lines[0].text).toContain("countdown=1h 0m");
  });
});

describe("filterLogLines", () => {
  const makeLine = (over: Partial<LogLine>): LogLine => ({
    id: "x",
    atMs: 0,
    status: "success",
    text: "Backed up plex",
    domain: "containers",
    kind: "backup",
    live: false,
    ...over,
  });

  const lines = [
    makeLine({ id: "a" }),
    makeLine({ id: "b", domain: "vms", text: "Backed up win11" }),
    makeLine({ id: "c", kind: "prune", text: "Pruned containers" }),
    makeLine({ id: "idle", idle: true, domain: "", kind: "", text: "next up" }),
  ];

  it("filters by domain and kind but never hides the idle line", () => {
    expect(filterLogLines(lines, { domain: "vms", kind: "all", text: "" }).map((l) => l.id)).toEqual(["b", "idle"]);
    expect(filterLogLines(lines, { domain: "all", kind: "backup", text: "" }).map((l) => l.id)).toEqual(["a", "b", "idle"]);
  });

  it("matches free text case-insensitively (idle line included)", () => {
    expect(filterLogLines(lines, { domain: "all", kind: "all", text: "PLEX" }).map((l) => l.id)).toEqual(["a"]);
  });
});

// ---------------------------------------------------------------------------
// #104 — the activity log now spans many days, so each line must show AND be
// searchable by its date, not just its time. Timestamps below are built with
// the local Date constructor (not a bare epoch number) so the expected
// calendar day is stable no matter which timezone the test runner is in —
// formatLogDate/isoDateOf both read back via local getters too.
// ---------------------------------------------------------------------------
describe("formatLogDate", () => {
  it("orders day/month per the active language (locale-aware date, fixed-face time stays formatClockTime's job)", () => {
    const atMs = new Date(2026, 6, 23, 5, 4, 8).getTime(); // 23 July 2026, local wall-clock
    expect(formatLogDate(atMs, "de")).toBe("23.07.");
    expect(formatLogDate(atMs, "en")).toBe("07/23");
  });
});

describe("filterLogLines — date search (#104)", () => {
  const day1 = new Date(2026, 6, 23, 5, 4, 8).getTime(); // 23 July 2026
  const day2 = new Date(2026, 6, 24, 9, 0, 0).getTime(); // 24 July 2026

  const dateLines: LogLine[] = [
    { id: "d1", atMs: day1, status: "success", text: "Backed up plex", domain: "containers", kind: "backup", live: false },
    { id: "d2", atMs: day2, status: "success", text: "Backed up sonarr", domain: "containers", kind: "backup", live: false },
  ];

  it("matches an ISO date typed into the filter, regardless of the active language", () => {
    expect(filterLogLines(dateLines, { domain: "all", kind: "all", text: "2026-07-23" }).map((l) => l.id)).toEqual(["d1"]);
    expect(
      filterLogLines(dateLines, { domain: "all", kind: "all", text: "2026-07-24", lang: "de" }).map((l) => l.id)
    ).toEqual(["d2"]);
  });

  it("matches the localized short date the UI actually displays", () => {
    expect(
      filterLogLines(dateLines, { domain: "all", kind: "all", text: "23.07", lang: "de" }).map((l) => l.id)
    ).toEqual(["d1"]);
    expect(
      filterLogLines(dateLines, { domain: "all", kind: "all", text: "07/24", lang: "en" }).map((l) => l.id)
    ).toEqual(["d2"]);
  });

  it("defaults the localized-date match to English ordering when no language is given", () => {
    expect(filterLogLines(dateLines, { domain: "all", kind: "all", text: "07/23" }).map((l) => l.id)).toEqual(["d1"]);
  });

  it("still narrows by plain message text alongside the new date matching", () => {
    expect(filterLogLines(dateLines, { domain: "all", kind: "all", text: "sonarr" }).map((l) => l.id)).toEqual(["d2"]);
  });
});
