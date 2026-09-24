// buildLogLines and filterLogLines have no framework dependencies, so these
// tests run in node. The stub resolver renders "key a=1 b=2", which keeps both
// the translation key and its params assertable.
import { describe, expect, it } from "vitest";
import { buildLogLines, domainLabel, filterLogLines, formatLogDate } from "./activityLog";
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

// Drills, tamper tests and the flash ZIP export publish live progress keys
// ("drill:<domain>", "tamper:<domain>", "export:flash"), so they show in the
// log while they run, like the other domain operations.
describe("buildLogLines live check lines", () => {
  it("renders a live drill as a running restore-check line with drill kind/domain", () => {
    const progress: ProgressMap = {
      "drill:containers": { phase: "maintenance", percent: 0, active: true, lastSeen: 5_000_000 },
    };
    const lines = buildLogLines([], progress, [], resolveName, 5_000_000);
    expect(lines).toHaveLength(1); // live line suppresses the idle "nothing yet" line too
    const live = lines[0];
    expect(live.id).toBe("live:drill:containers");
    expect(live.live).toBe(true);
    expect(live.status).toBe("running");
    expect(live.kind).toBe("drill");
    expect(live.domain).toBe("containers");
    expect(live.text).toContain("activityLog.lineDrillRunning");
    expect(live.text).toContain("domain=activityLog.domainContainers");
  });

  it("supersedes the finished drill run row while its live line still shows (no doubling)", () => {
    // The backend records the drill run before the final progress frame clears
    // the live entry, so for a moment both exist.
    const finishedDrill = makeRun({ id: "r-drill", kind: "drill", targetId: "containers", target: "containers" });
    const progress: ProgressMap = {
      "drill:containers": { phase: "maintenance", percent: 0, active: true, lastSeen: 5_000_000 },
    };
    const during = buildLogLines([finishedDrill], progress, [], resolveName, 5_000_000);
    expect(during.map((l) => l.id)).toEqual(["live:drill:containers"]);

    // Once the live entry is gone, the finished run row takes over.
    const after = buildLogLines([finishedDrill], {}, [], resolveName, 5_000_000);
    expect(after.map((l) => l.id)).toEqual(["run:r-drill"]);
    expect(after[0].kind).toBe("drill");
    expect(after[0].domain).toBe("containers");
    expect(after[0].text).toContain("activityLog.lineDrillSuccess");
  });

  it("renders live tamper and flash-ZIP-export lines with their kinds", () => {
    const progress: ProgressMap = {
      "tamper:vms": { phase: "maintenance", percent: 0, active: true, lastSeen: 5_000_000 },
      "export:flash": { phase: "maintenance", percent: 0, active: true, lastSeen: 5_000_001 },
    };
    const lines = buildLogLines([], progress, [], resolveName, 5_000_100);
    expect(lines.map((l) => l.id)).toEqual(["live:tamper:vms", "live:export:flash"]);
    const [tamper, exp] = lines;
    expect(tamper.kind).toBe("tamper");
    expect(tamper.domain).toBe("vms");
    expect(tamper.text).toContain("activityLog.lineTamperRunning");
    expect(exp.kind).toBe("export");
    expect(exp.domain).toBe("flash");
    expect(exp.text).toContain("activityLog.lineExportRunning");
  });
});

// The live off-site line shows run progress once the backend reports a snapshot
// index and total, else the time since startedAt, else the plain running line,
// so a skewed, zero or negative timestamp never renders NaN or a negative span.
describe("buildLogLines live off-site line", () => {
  it("renders the plain running line when startedAt is not known yet", () => {
    const progress: ProgressMap = {
      "offsite:containers": { phase: "replicate", percent: 0, active: true, lastSeen: 5_000_000 },
    };
    const lines = buildLogLines([], progress, [], resolveName, 5_000_000);
    expect(lines).toHaveLength(1);
    const live = lines[0];
    expect(live.status).toBe("offsite");
    expect(live.kind).toBe("offsite");
    expect(live.text).toBe("activityLog.lineOffsiteRunning domain=activityLog.domainContainers");
  });

  it("appends a live elapsed duration once startedAt is known", () => {
    const progress: ProgressMap = {
      "offsite:containers": { phase: "replicate", percent: 0, active: true, lastSeen: 5_030_000, startedAt: 5000 },
    };
    const lines = buildLogLines([], progress, [], resolveName, 5_030_000);
    expect(lines).toHaveLength(1);
    const live = lines[0];
    expect(live.text).toBe("activityLog.lineOffsiteRunningWithDuration domain=activityLog.domainContainers duration=30s");
  });

  it("falls back to the plain running line when startedAt is in the future", () => {
    const progress: ProgressMap = {
      "offsite:containers": { phase: "replicate", percent: 0, active: true, lastSeen: 5_030_000, startedAt: 6000 },
    };
    const lines = buildLogLines([], progress, [], resolveName, 5_030_000);
    const live = lines[0];
    expect(live.text).toBe("activityLog.lineOffsiteRunning domain=activityLog.domainContainers");
    expect(live.text).not.toContain("NaN");
  });

  it("treats startedAt: 0 as unknown, never rendering the epoch's ~56-year elapsed span", () => {
    const progress: ProgressMap = {
      "offsite:containers": { phase: "replicate", percent: 0, active: true, lastSeen: 5_030_000, startedAt: 0 },
    };
    const lines = buildLogLines([], progress, [], resolveName, 5_030_000);
    expect(lines[0].text).toBe("activityLog.lineOffsiteRunning domain=activityLog.domainContainers");
  });

  it("treats a negative startedAt as unknown, never rendering a negative duration", () => {
    const progress: ProgressMap = {
      "offsite:containers": { phase: "replicate", percent: 0, active: true, lastSeen: 5_030_000, startedAt: -500 },
    };
    const lines = buildLogLines([], progress, [], resolveName, 5_030_000);
    expect(lines[0].text).toBe("activityLog.lineOffsiteRunning domain=activityLog.domainContainers");
    expect(lines[0].text).not.toContain("-");
  });

  it("shows the run's percentage once the backend reports a snapshot index and total", () => {
    const progress: ProgressMap = {
      "offsite:containers": { phase: "replicate", percent: 62.6, active: true, lastSeen: 5_000_000, snapshotIndex: 2, snapshotTotal: 4 },
    };
    const lines = buildLogLines([], progress, [], resolveName, 5_000_000);
    // One snapshot done plus 62.6% of the second, out of 4, is 40.65%. 63
    // would be the second snapshot's own pack progress.
    expect(lines[0].text).toBe(
      "activityLog.lineOffsiteRunningSnapshotPercent domain=activityLog.domainContainers index=2 total=4 percent=41 duration="
    );
  });

  // 55% is snapshot 15's own pack progress; next to "15 of 126" it would read
  // as progress of the whole run, which is about 12% done.
  it("shows snapshot 15 of 126 at 55% as 12% of the run", () => {
    const progress: ProgressMap = {
      "offsite:containers": { phase: "replicate", percent: 55, active: true, lastSeen: 5_000_000, snapshotIndex: 15, snapshotTotal: 126 },
    };
    const lines = buildLogLines([], progress, [], resolveName, 5_000_000);
    expect(lines[0].text).toContain("index=15 total=126 percent=12");
    expect(lines[0].text).not.toContain("percent=55");
  });

  it("combines the run-level percentage with the elapsed duration", () => {
    const progress: ProgressMap = {
      "offsite:containers": {
        phase: "replicate",
        percent: 10,
        active: true,
        lastSeen: 5_030_000,
        startedAt: 5000,
        snapshotIndex: 1,
        snapshotTotal: 1,
      },
    };
    const lines = buildLogLines([], progress, [], resolveName, 5_030_000);
    expect(lines[0].text).toBe(
      "activityLog.lineOffsiteRunningSnapshotPercentWithDuration domain=activityLog.domainContainers index=1 total=1 percent=10 duration=30s"
    );
  });

  it("widens the displayed total to at least the live index if the estimate undercounted", () => {
    const progress: ProgressMap = {
      "offsite:containers": { phase: "replicate", percent: 5, active: true, lastSeen: 5_000_000, snapshotIndex: 3, snapshotTotal: 2 },
    };
    const lines = buildLogLines([], progress, [], resolveName, 5_000_000);
    expect(lines[0].text).toContain("index=3 total=3");
  });

  it("ignores a snapshotIndex of 0 (not yet attributed) even if percent is a number", () => {
    const progress: ProgressMap = {
      "offsite:containers": { phase: "replicate", percent: 40, active: true, lastSeen: 5_000_000, snapshotIndex: 0 },
    };
    const lines = buildLogLines([], progress, [], resolveName, 5_000_000);
    expect(lines[0].text).toBe("activityLog.lineOffsiteRunning domain=activityLog.domainContainers");
  });

  // Without an estimated total there is nothing to divide by; dividing by the
  // live index would claim the run is nearly done.
  it("falls back to the duration line when the backend has no snapshot total to divide by", () => {
    const progress: ProgressMap = {
      "offsite:containers": { phase: "replicate", percent: 55, active: true, lastSeen: 5_030_000, startedAt: 5000, snapshotIndex: 15 },
    };
    const lines = buildLogLines([], progress, [], resolveName, 5_030_000);
    expect(lines[0].text).toBe("activityLog.lineOffsiteRunningWithDuration domain=activityLog.domainContainers duration=30s");
  });

  // ActivityLog.tsx ticks `now` once a minute, which early in a run can lag
  // behind startedAt, so the off-site duration uses the faster liveNow clock.
  it("uses liveNow (not now) for the off-site duration, so a stale `now` doesn't go negative", () => {
    // `now` is 1s before startedAt (5000s) and would give a negative span;
    // liveNow is 3s after it.
    const now = 4_999_000;
    const liveNow = 5_003_000;
    const progress: ProgressMap = {
      "offsite:containers": { phase: "replicate", percent: 0, active: true, lastSeen: now, startedAt: 5000 },
    };
    const lines = buildLogLines([], progress, [], resolveName, now, liveNow);
    expect(lines[0].text).toBe("activityLog.lineOffsiteRunningWithDuration domain=activityLog.domainContainers duration=3s");
  });

  it("still gates staleness on `now`, independent of liveNow", () => {
    const progress: ProgressMap = {
      "offsite:containers": { phase: "replicate", percent: 0, active: true, lastSeen: 0, startedAt: 0 },
    };
    const now = 20_000; // 20s > STALE_MS (15s, see progress.ts)
    const lines = buildLogLines([], progress, [], resolveName, now, now);
    expect(lines.some((l) => l.text.includes("lineOffsiteRunning"))).toBe(false);
  });

  it("defaults liveNow to now", () => {
    const progress: ProgressMap = {
      "offsite:containers": { phase: "replicate", percent: 0, active: true, lastSeen: 5_030_000, startedAt: 5000 },
    };
    const lines = buildLogLines([], progress, [], resolveName, 5_030_000); // no 6th arg
    expect(lines[0].text).toBe("activityLog.lineOffsiteRunningWithDuration domain=activityLog.domainContainers duration=30s");
  });
});

// The off-site DR drill has its own kind "drdrill", apart from the local subset
// drill, and a tamper run without a verdict is recorded as skipped and shown as
// a neutral info line.
describe("buildLogLines DR drill and skipped tamper", () => {
  it("renders a live DR check with kind drdrill and the DR-running line", () => {
    const progress: ProgressMap = {
      "drdrill:containers": { phase: "maintenance", percent: 0, active: true, lastSeen: 5_000_000 },
    };
    const lines = buildLogLines([], progress, [], resolveName, 5_000_000);
    expect(lines).toHaveLength(1);
    const live = lines[0];
    expect(live.id).toBe("live:drdrill:containers");
    expect(live.live).toBe(true);
    expect(live.status).toBe("running");
    expect(live.kind).toBe("drdrill");
    expect(live.domain).toBe("containers");
    expect(live.text).toContain("activityLog.lineDRDrillRunning");
    expect(live.text).toContain("domain=activityLog.domainContainers");
  });

  it("renders a finished drdrill run with its own success line and kind", () => {
    const run = makeRun({ id: "r-drdrill", kind: "drdrill", targetId: "containers", target: "containers" });
    const lines = buildLogLines([run], {}, [], resolveName, 5_000_000);
    expect(lines.map((l) => l.id)).toEqual(["run:r-drdrill"]);
    expect(lines[0].kind).toBe("drdrill");
    expect(lines[0].domain).toBe("containers");
    expect(lines[0].status).toBe("success");
    expect(lines[0].text).toContain("activityLog.lineDRDrillSuccess");
  });

  it("renders a failed drdrill run with the DR failure line", () => {
    const run = makeRun({ id: "r-drdrill-f", kind: "drdrill", targetId: "containers", target: "containers", status: "failed", error: "boom" });
    const lines = buildLogLines([run], {}, [], resolveName, 5_000_000);
    expect(lines[0].status).toBe("failed");
    expect(lines[0].text).toContain("activityLog.lineDRDrillFailed");
    expect(lines[0].text).toContain("error=boom");
  });

  it("renders a skipped tamper run as a neutral info line carrying the reason", () => {
    const run = makeRun({
      id: "r-tamper-s",
      kind: "tamper",
      targetId: "containers",
      target: "containers",
      status: "skipped",
      error: "only REST repos are verifiable",
    });
    const lines = buildLogLines([run], {}, [], resolveName, 5_000_000);
    expect(lines.map((l) => l.id)).toEqual(["run:r-tamper-s"]);
    expect(lines[0].status).toBe("info");
    expect(lines[0].text).toContain("activityLog.lineTamperSkipped");
    expect(lines[0].text).toContain("error=only REST repos are verifiable");
  });

  it("keeps the subset drill apart from the DR drill", () => {
    const subset = makeRun({ id: "r-drill", kind: "drill", targetId: "containers", target: "containers" });
    const lines = buildLogLines([subset], {}, [], resolveName, 5_000_000);
    expect(lines[0].kind).toBe("drill");
    expect(lines[0].text).toContain("activityLog.lineDrillSuccess");
    // The kind filter tells the two families apart: "drill" matches only the
    // subset check, "drdrill" only the DR check.
    const both = [
      ...buildLogLines([subset], {}, [], resolveName, 5_000_000),
      ...buildLogLines([makeRun({ id: "r-dr", kind: "drdrill", targetId: "containers", target: "containers" })], {}, [], resolveName, 5_000_000),
    ];
    expect(filterLogLines(both, { domain: "all", kind: "drill", text: "" }).map((l) => l.id)).toEqual(["run:r-drill"]);
    expect(filterLogLines(both, { domain: "all", kind: "drdrill", text: "" }).map((l) => l.id)).toEqual(["run:r-dr"]);
  });
});

// A Backup Everything pass records a parent backup run under the pseudo-domain
// "everything". Its line uses the generic backup formatter, so these tests
// cover only the domain label and the domain filter.
describe("Backup Everything pseudo-domain", () => {
  it("resolves the everything domain via domainLabel in the idle next-up line", () => {
    const next: ScheduleNext[] = [
      { job: "backup", domain: "everything", next: new Date(7_200_000).toISOString() },
    ];
    const lines = buildLogLines([], {}, next, resolveName, 3_600_000);
    expect(lines).toHaveLength(1);
    expect(lines[0].text).toContain("activityLog.lineNextWithDomain");
    expect(lines[0].text).toContain("domain=activityLog.domainEverything");
  });

  it("normalizes a finished Backup Everything run's domain and matches its filter value", () => {
    const run = makeRun({
      id: "r-everything",
      targetId: "everything",
      target: "Backup Everything",
      domain: "everything", // needs no singular to plural mapping
      bytes: 0,
    });
    const other = makeRun({}); // id "r1", domain "container" → normalized "containers"
    const lines = buildLogLines([run, other], {}, [], resolveName, 5_000_000);
    expect(lines.map((l) => l.id).sort()).toEqual(["run:r-everything", "run:r1"].sort());

    const everythingLine = lines.find((l) => l.id === "run:r-everything")!;
    expect(everythingLine.domain).toBe("everything");
    expect(everythingLine.text).toContain("activityLog.lineBackupSuccess");
    expect(everythingLine.text).toContain("name=Backup Everything");

    expect(
      filterLogLines(lines, { domain: "everything", kind: "all", text: "" }).map((l) => l.id)
    ).toEqual(["run:r-everything"]);
    expect(
      filterLogLines(lines, { domain: "containers", kind: "all", text: "" }).map((l) => l.id)
    ).toEqual(["run:r1"]);
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

// The log spans several days, so each line shows its date and matches it in
// the search. Timestamps come from the local Date constructor, so the expected
// day does not depend on the runner's timezone.
describe("formatLogDate", () => {
  it("orders day and month per the active language", () => {
    const atMs = new Date(2026, 6, 23, 5, 4, 8).getTime(); // 23 July 2026, local wall-clock
    expect(formatLogDate(atMs, "de")).toBe("23.07.");
    expect(formatLogDate(atMs, "en")).toBe("07/23");
  });
});

describe("filterLogLines date search", () => {
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

  it("matches the date in the environment's default locale when no language is given", () => {
    const shown = formatLogDate(day1);
    expect(filterLogLines(dateLines, { domain: "all", kind: "all", text: shown }).map((l) => l.id)).toEqual(["d1"]);
  });

  it("still matches plain message text", () => {
    expect(filterLogLines(dateLines, { domain: "all", kind: "all", text: "sonarr" }).map((l) => l.id)).toEqual(["d2"]);
  });
});

// The heatmap drilldown passes a `day` (YYYY-MM-DD) that keeps only lines on
// that local calendar day.
describe("filterLogLines day filter", () => {
  const day1Morning = new Date(2026, 6, 23, 0, 0, 1).getTime(); // 23 July 2026, local
  const day1Night = new Date(2026, 6, 23, 23, 59, 59).getTime(); // same local day
  const day2 = new Date(2026, 6, 24, 9, 0, 0).getTime(); // 24 July 2026, local

  const lines: LogLine[] = [
    { id: "m", atMs: day1Morning, status: "success", text: "Backed up plex", domain: "containers", kind: "backup", live: false },
    { id: "n", atMs: day1Night, status: "success", text: "Backed up win11", domain: "vms", kind: "backup", live: false },
    { id: "p", atMs: day1Night, status: "success", text: "Pruned containers", domain: "containers", kind: "prune", live: false },
    { id: "o", atMs: day2, status: "success", text: "Backed up sonarr", domain: "containers", kind: "backup", live: false },
    { id: "idle", atMs: day2, status: "info", text: "next up", domain: "", kind: "", live: false, idle: true },
  ];

  it("keeps only lines on the picked local calendar day (whole day, midnight to midnight)", () => {
    expect(
      filterLogLines(lines, { domain: "all", kind: "all", text: "", day: "2026-07-23" }).map((l) => l.id)
    ).toEqual(["m", "n", "p", "idle"]);
  });

  it("returns no run lines for a day without runs", () => {
    expect(
      filterLogLines(lines, { domain: "all", kind: "all", text: "", day: "2026-07-25" }).map((l) => l.id)
    ).toEqual(["idle"]);
  });

  it("combines with the domain and kind quick-filters", () => {
    expect(
      filterLogLines(lines, { domain: "containers", kind: "all", text: "", day: "2026-07-23" }).map((l) => l.id)
    ).toEqual(["m", "p", "idle"]);
    expect(
      filterLogLines(lines, { domain: "all", kind: "prune", text: "", day: "2026-07-23" }).map((l) => l.id)
    ).toEqual(["p", "idle"]);
    expect(
      filterLogLines(lines, { domain: "vms", kind: "backup", text: "", day: "2026-07-24" }).map((l) => l.id)
    ).toEqual(["idle"]);
  });

  it("combines with the free-text search", () => {
    expect(
      filterLogLines(lines, { domain: "all", kind: "all", text: "backed up", day: "2026-07-23" }).map((l) => l.id)
    ).toEqual(["m", "n"]);
  });

  it("exempts the idle line, like the domain/kind quick-filters do", () => {
    // The idle line is on day2, but it says what runs next, so no day hides it.
    const ids = filterLogLines(lines, { domain: "all", kind: "all", text: "", day: "2026-07-23" }).map((l) => l.id);
    expect(ids).toContain("idle");
  });

  it("is off when day is omitted", () => {
    expect(filterLogLines(lines, { domain: "all", kind: "all", text: "" })).toHaveLength(lines.length);
  });
});

// Without a locale the date follows the engine's default, like every other
// date in the app, rather than navigator.language, which can disagree with it
// (a macOS "en-US" UI language with a Portuguese region).
describe("formatLogDate without a locale", () => {
  it("matches toLocaleDateString's default-locale day/month rendering", () => {
    const ts = new Date(2026, 6, 23, 5, 0, 0).getTime();
    const expected = new Intl.DateTimeFormat(undefined, { day: "2-digit", month: "2-digit" }).format(new Date(ts));
    expect(formatLogDate(ts)).toBe(expected);
  });
});

describe("a live line whose stream has gone quiet", () => {
  // restic can go quiet for minutes while it scans a large tree, so `lastSeen`
  // ages past STALE_MS on a healthy run. The runs list decides whether the
  // line stays.
  const quiet = {
    "files:Documents": { phase: "backup", percent: 18, active: true, lastSeen: 1_000_000 },
  };
  // Well past STALE_MS (15s) since the last frame.
  const muchLater = 1_000_000 + 120_000;

  it("survives while the runs list still calls that target running", () => {
    const running = makeRun({
      id: "r-live",
      kind: "backup",
      status: "running",
      target: "Documents",
      domain: "files",
      finishedAt: null,
    });
    const lines = buildLogLines([running], quiet, [], resolveName, muchLater);
    const live = lines.filter((l) => l.live);
    expect(live).toHaveLength(1);
    expect(live[0].id).toBe("live:files:Documents");
  });

  it("still disappears once no run reports it running", () => {
    // A lost final frame must not leave a running line in place forever.
    const finished = makeRun({
      id: "r-done",
      kind: "backup",
      status: "success",
      target: "Documents",
      domain: "files",
      finishedAt: 1001,
    });
    const lines = buildLogLines([finished], quiet, [], resolveName, muchLater);
    expect(lines.filter((l) => l.live)).toHaveLength(0);
  });

  it("is not kept alive by another target's running run", () => {
    const otherRunning = makeRun({
      id: "r-other",
      kind: "backup",
      status: "running",
      target: "Photos",
      domain: "files",
      finishedAt: null,
    });
    const lines = buildLogLines([otherRunning], quiet, [], resolveName, muchLater);
    expect(lines.filter((l) => l.live)).toHaveLength(0);
  });
});

// Every log line names its domain, since a container and a folder set can share
// a name.
describe("domainLabel", () => {
  const t = (k: string) => `T:${k}`;

  it("gives containers and folder sets different labels", () => {
    expect(domainLabel(t, "containers")).not.toBe(domainLabel(t, "files"));
  });

  it("labels every domain the log can emit, none falling through to the raw literal", () => {
    for (const d of ["containers", "vms", "flash", "config", "files", "everything"]) {
      expect(domainLabel(t, d)).toBe(`T:activityLog.domain${d[0].toUpperCase()}${d.slice(1)}`.replace("domainVms", "domainVMs"));
    }
  });
});

// A dump is its own run against the container's target, so without lines of its
// own the log would print the raw kind through lineOther.
describe("dbdump runs", () => {
  const dump = (over: Partial<Run>): Run =>
    makeRun({ id: "d1", kind: "dbdump", target: "immich_postgres", bytes: 5_242_880, ...over });

  it("reads a finished dump as a dump, not as a backup", () => {
    const [line] = buildLogLines([dump({})], {}, [], resolveName, 2_000_000);
    expect(line.status).toBe("success");
    expect(line.kind).toBe("dbdump");
    expect(line.text).toContain("activityLog.lineDbDumpSuccess");
    expect(line.text).toContain("name=immich_postgres");
    expect(line.text).toContain("bytes=5.0 MB");
    expect(line.text).toContain("duration=30s");
    expect(line.text).not.toContain("lineOther");
  });

  it("keeps a success note on the line instead of dropping it", () => {
    const [line] = buildLogLines(
      [dump({ error: "database dump covers one database only" })],
      {},
      [],
      resolveName,
      2_000_000
    );
    expect(line.text).toContain("activityLog.lineDbDumpNote");
    expect(line.text).toContain("note=runReason.dbdumpOneDatabase");
  });

  it("translates a failure and keeps the tool's own message behind it", () => {
    const [line] = buildLogLines(
      [dump({ status: "failed", error: "database dump failed: the database refused the login: FATAL no" })],
      {},
      [],
      resolveName,
      2_000_000
    );
    expect(line.status).toBe("failed");
    expect(line.text).toContain("activityLog.lineDbDumpFailed");
    expect(line.text).toContain("error=runReason.dbdumpAuth: FATAL no");
  });

  // A cancelled dump is recorded as a failure carrying the cancellation as its
  // reason, so the reason decides how the line reads, not the status.
  it("says a cancelled dump was cancelled rather than that it failed", () => {
    const [line] = buildLogLines(
      [dump({ status: "failed", error: "cancelled by the user" })],
      {},
      [],
      resolveName,
      2_000_000
    );
    expect(line.text).toContain("activityLog.lineDbDumpCancelled");
    expect(line.text).not.toContain("lineDbDumpFailed");
  });

  it("flags a cancelled dump that may still be running in the container", () => {
    const [line] = buildLogLines(
      [dump({ status: "failed", error: "cancelled by the user: orphan stop failed" })],
      {},
      [],
      resolveName,
      2_000_000
    );
    expect(line.status).toBe("failed");
    expect(line.text).toContain("error=runReason.cancelled; runReason.dbdumpOrphan");
  });

  it("has its own lines for a saved dump and an import", () => {
    const runs = [
      dump({ id: "s1", kind: "dbdumpsave" }),
      dump({ id: "s2", kind: "dbdumpsave", status: "failed", error: "no space left on device" }),
      dump({ id: "s3", kind: "dbdumpsave", status: "cancelled" }),
      dump({ id: "i1", kind: "dbimport" }),
      dump({ id: "i2", kind: "dbimport", error: "database imported with errors" }),
      dump({ id: "i3", kind: "dbimport", status: "failed", error: "database import failed: the import tool reported an error" }),
    ];
    const texts = buildLogLines(runs, {}, [], resolveName, 2_000_000).map((l) => l.text);
    expect(texts[0]).toContain("activityLog.lineDbDumpSaved");
    expect(texts[1]).toContain("activityLog.lineDbDumpSaveFailed");
    expect(texts[2]).toContain("activityLog.lineDbDumpSaveCancelled");
    expect(texts[3]).toContain("activityLog.lineDbImported");
    expect(texts[4]).toContain("activityLog.lineDbImportedErrors");
    expect(texts[4]).toContain("note=runReason.dbimportErrors");
    expect(texts[5]).toContain("activityLog.lineDbImportFailed");
    expect(texts.join(" ")).not.toContain("lineOther");
  });

  it("names an app an import could not start again", () => {
    const run = dump({
      id: "i1",
      kind: "dbimport",
      error: "database imported; the previous data folder was kept: /mnt/pg.old; could not start these apps again: immich_server",
    });
    const [line] = buildLogLines([run], {}, [], resolveName, 2_000_000);
    expect(line.text).toContain("activityLog.lineDbImported");
    expect(line.text).toContain("runReason.dbimportAppsDown");
  });

  it("colours a successful run as a warning only when its note asks for action", () => {
    const runs = [
      dump({ id: "i1", kind: "dbimport", error: "database imported; the previous data folder was kept: /mnt/pg.old" }),
      dump({
        id: "i2",
        kind: "dbimport",
        error: "database imported; the previous data folder was kept: /mnt/pg.old; could not start these apps again: immich_server",
      }),
    ];
    const lines = buildLogLines(runs, {}, [], resolveName, 2_000_000);
    expect(lines.find((l) => l.id === "run:i1")?.warn).toBeUndefined();
    expect(lines.find((l) => l.id === "run:i2")?.warn).toBe(true);
    expect(lines.find((l) => l.id === "run:i2")?.status).toBe("success");
  });

  it("counts the dumped bytes on the live line, where there is no percentage", () => {
    const progress: ProgressMap = {
      "container:immich_postgres": {
        phase: "backup",
        percent: 0,
        active: true,
        lastSeen: 5_000_000,
        stage: "dbdump",
        bytes: 1_048_576,
      },
    };
    const [line] = buildLogLines([], progress, [], resolveName, 5_000_000);
    expect(line.text).toContain("activityLog.lineDumpingItem");
    expect(line.text).toContain("bytes=1.0 MB");
    expect(line.text).not.toContain("percent=");
    expect(line.kind).toBe("dbdump");
  });

  it("names the save and the import while they run", () => {
    const at = (stage: "dbdumpsave" | "dbimport"): ProgressMap => ({
      "container:immich_postgres": { phase: "restore", percent: 0, active: true, lastSeen: 5_000_000, stage, bytes: 512 },
    });
    expect(buildLogLines([], at("dbdumpsave"), [], resolveName, 5_000_000)[0].text).toContain(
      "activityLog.lineSavingDumpItem"
    );
    expect(buildLogLines([], at("dbimport"), [], resolveName, 5_000_000)[0].text).toContain(
      "activityLog.lineImportingItem"
    );
  });

  it("keeps only dump lines under the dump filter", () => {
    const lines = buildLogLines(
      [dump({}), makeRun({ id: "b1" }), dump({ id: "s1", kind: "dbdumpsave" })],
      {},
      [],
      resolveName,
      2_000_000
    );
    const filtered = filterLogLines(lines, { domain: "all", kind: "dbdump", text: "" });
    expect(filtered.map((l) => l.id)).toEqual(["run:d1"]);
  });

  it("shows a save and an import under the restore filter, where a user looks for them", () => {
    const lines = buildLogLines(
      [dump({ id: "s1", kind: "dbdumpsave" }), dump({ id: "i1", kind: "dbimport" })],
      {},
      [],
      resolveName,
      2_000_000
    );
    const filtered = filterLogLines(lines, { domain: "all", kind: "restore", text: "" });
    expect(filtered.map((l) => l.id)).toEqual(["run:s1", "run:i1"]);
  });
});

describe("a run an assistant started", () => {
  const viaMcp = (over: Partial<Run>): Run =>
    makeRun({ startedVia: "mcp", startedViaKey: "k1", startedViaLabel: "office laptop", ...over });

  it("appends via MCP with the key label", () => {
    const lines = buildLogLines([viaMcp({})], {}, [], resolveName, 2_000_000);
    expect(lines[0].text).toMatch(/^activityLog\.viaMcp /);
    expect(lines[0].text).toContain("key=office laptop");
    expect(lines[0].text).toContain("line=activityLog.lineBackupSuccess");

    // A domain-wide operation an MCP start caused says so as well.
    const prune = buildLogLines(
      [viaMcp({ id: "p1", kind: "prune", targetId: "containers" })],
      {},
      [],
      resolveName,
      2_000_000
    );
    expect(prune[0].text).toMatch(/^activityLog\.viaMcp /);
    expect(prune[0].text).toContain("line=activityLog.linePruneSuccess");
  });

  it("marks a revoked key", () => {
    const lines = buildLogLines([viaMcp({ startedViaRevoked: true })], {}, [], resolveName, 2_000_000);
    expect(lines[0].text).toContain("activityLog.viaMcpRevoked");
    expect(lines[0].text).toContain("key=office laptop");
  });

  it("without a label", () => {
    const lines = buildLogLines([viaMcp({ startedViaLabel: "" })], {}, [], resolveName, 2_000_000);
    expect(lines[0].text).toContain("activityLog.viaMcpUnknownKey");
    expect(lines[0].text).not.toContain("key=");
  });

  it("leaves a scheduled run and a live line untouched", () => {
    const progress: ProgressMap = {
      "container:plex": { phase: "backup", percent: 20, active: true, lastSeen: 5_000_000 },
    };
    const lines = buildLogLines([makeRun({ id: "s1", finishedAt: 900 })], progress, [], resolveName, 5_000_000);
    expect(lines.map((l) => l.text).join(" ")).not.toContain("viaMcp");
  });
});
