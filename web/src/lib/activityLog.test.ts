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

// ---------------------------------------------------------------------------
// #109 — drills, tamper tests and the flash-ZIP export publish live progress
// keys ("drill:<domain>", "tamper:<domain>", "export:flash") so they show in
// the log WHILE running, with the same dedupe-signature mechanics as the
// other domain-op live lines (offsite/prune/verify).
// ---------------------------------------------------------------------------
describe("buildLogLines — live domain-op checks (#109)", () => {
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
    // The backend records the drill run (recordDomainRun) BEFORE the terminal
    // progress frame clears the live entry — during that window both exist.
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

// ---------------------------------------------------------------------------
// Issue #159 — a first cut of this feature concluded restic copy had no
// machine-readable percentage and shipped a duration-only live off-site line.
// That conclusion was wrong (see restic.Copy's doc comment for the corrected
// story): restic copy DOES print a real per-snapshot pack-copy percentage
// once RESTIC_PROGRESS_FPS is wired up, so offsiteLiveLineText shows it once
// available, falling back to the honest elapsed-duration signal (from the
// backend-stamped startedAt) and finally to the plain running line whenever
// neither is known/usable — so a stale/skewed/zero/negative timestamp can
// never render "NaN" or a negative span.
// ---------------------------------------------------------------------------
describe("buildLogLines — live off-site line (#159)", () => {
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

  it("degrades to the plain running line — asserting the ACTUAL fallback text, not just the absence of NaN — when startedAt is in the future (clock skew)", () => {
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

  it("shows a RUN-LEVEL percentage once the backend reports a snapshot index/total", () => {
    const progress: ProgressMap = {
      "offsite:containers": { phase: "replicate", percent: 62.6, active: true, lastSeen: 5_000_000, snapshotIndex: 2, snapshotTotal: 4 },
    };
    const lines = buildLogLines([], progress, [], resolveName, 5_000_000);
    // 1 whole snapshot done + 62.6% of the second, out of 4 => 40.65% => 41.
    // NOT 63, which is snapshot 2's own pack progress (see offsiteRunProgress).
    expect(lines[0].text).toBe(
      "activityLog.lineOffsiteRunningSnapshotPercent domain=activityLog.domainContainers index=2 total=4 percent=41 duration="
    );
  });

  // The exact numbers from issue #159's report: "snapshot 15 of 126 (55%)" on a
  // 1h 7m run. 55 was snapshot 15's own pack progress, but sat in parentheses
  // right after the fraction, so it read as "15/126 = 55%" — which is what made
  // the line look broken. Real run progress is ~12%, and the fraction beside it
  // now agrees instead of contradicting.
  it("renders the reported 15-of-126-at-55% case as ~12% overall, never 55%", () => {
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

  // The backend publishes snapshotTotal 0/absent when it could not estimate the
  // candidate count at all (see api.progBeginCopySink). There is no honest
  // denominator then, so the line must fall back rather than divide by the live
  // index and claim a confident ~99%.
  it("falls back to the duration line when the backend has no snapshot total to divide by", () => {
    const progress: ProgressMap = {
      "offsite:containers": { phase: "replicate", percent: 55, active: true, lastSeen: 5_030_000, startedAt: 5000, snapshotIndex: 15 },
    };
    const lines = buildLogLines([], progress, [], resolveName, 5_030_000);
    expect(lines[0].text).toBe("activityLog.lineOffsiteRunningWithDuration domain=activityLog.domainContainers duration=30s");
  });

  // Review fix: ActivityLog.tsx's `now` ticks at a coarse 60s cadence (its
  // idle countdown doesn't need better) — reusing it for the off-site
  // duration meant that for a run's first ~60s, `now` could sit BEHIND the
  // backend-stamped startedAt, making the computed span go NEGATIVE (blank),
  // then jump once `now` finally caught up. `liveNow` is the fix: a second,
  // faster-ticking clock buildLogLines/buildLiveLines take SEPARATELY from
  // `now`, used ONLY for this computation.
  it("uses liveNow (not now) for the off-site duration, so a stale `now` doesn't go negative", () => {
    // startedAt = 5000s (5,000,000ms). `now` is 1s BEHIND that in ms terms —
    // reusing `now` for elapsedSince (the pre-fix bug) would compute a
    // NEGATIVE span, which formatDuration rejects, rendering blank. `liveNow`
    // is 3s AHEAD of startedAt and must be what actually drives the text.
    const now = 4_999_000;
    const liveNow = 5_003_000;
    const progress: ProgressMap = {
      "offsite:containers": { phase: "replicate", percent: 0, active: true, lastSeen: now, startedAt: 5000 },
    };
    const lines = buildLogLines([], progress, [], resolveName, now, liveNow);
    expect(lines[0].text).toBe("activityLog.lineOffsiteRunningWithDuration domain=activityLog.domainContainers duration=3s");
  });

  it("still gates staleness on `now`, independent of liveNow", () => {
    // lastSeen is far enough behind `now` to be stale (> STALE_MS), even
    // though `liveNow` alone would suggest the entry is fresh — staleness must
    // stay governed by `now`, unaffected by the liveNow fix.
    const progress: ProgressMap = {
      "offsite:containers": { phase: "replicate", percent: 0, active: true, lastSeen: 0, startedAt: 0 },
    };
    const now = 20_000; // 20s > STALE_MS (15s, see progress.ts)
    const lines = buildLogLines([], progress, [], resolveName, now, now);
    expect(lines.some((l) => l.text.includes("lineOffsiteRunning"))).toBe(false);
  });

  it("buildLogLines defaults liveNow to now when the caller doesn't pass one (backward compatible)", () => {
    const progress: ProgressMap = {
      "offsite:containers": { phase: "replicate", percent: 0, active: true, lastSeen: 5_030_000, startedAt: 5000 },
    };
    const lines = buildLogLines([], progress, [], resolveName, 5_030_000); // no 6th arg
    expect(lines[0].text).toBe("activityLog.lineOffsiteRunningWithDuration domain=activityLog.domainContainers duration=30s");
  });
});

// ---------------------------------------------------------------------------
// #109 follow-up — the off-site DR drill has its own kind "drdrill" (live key
// "drdrill:<domain>", run kind "drdrill") so it is distinguishable from the
// local subset drill ("drill"), and a tamper run that produced no verdict is
// recorded "skipped" and rendered as a neutral info line, never a red.
// ---------------------------------------------------------------------------
describe("buildLogLines — DR drill kind + skipped tamper (#109 follow-up)", () => {
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
    expect(lines[0].status).toBe("info"); // neutral — the test ran, no verdict; never a red
    expect(lines[0].text).toContain("activityLog.lineTamperSkipped");
    expect(lines[0].text).toContain("error=only REST repos are verifiable");
  });

  it("keeps the subset drill line unchanged alongside the new DR kind", () => {
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

  it("defaults the localized-date match to the environment's own locale when no language is given (#108)", () => {
    // With lang omitted the haystack uses the engine's default negotiation —
    // whatever THIS environment renders for day1 must match (and does so on
    // any OS locale, which is exactly the #108 contract).
    const shown = formatLogDate(day1);
    expect(filterLogLines(dateLines, { domain: "all", kind: "all", text: shown }).map((l) => l.id)).toEqual(["d1"]);
  });

  it("still narrows by plain message text alongside the new date matching", () => {
    expect(filterLogLines(dateLines, { domain: "all", kind: "all", text: "sonarr" }).map((l) => l.id)).toEqual(["d2"]);
  });
});

// ---------------------------------------------------------------------------
// Heatmap → Activity Log drilldown — the optional `day` filter (ISO
// YYYY-MM-DD) keeps only lines on that LOCAL calendar day. Timestamps are
// built with the local Date constructor so the expected day is stable in any
// runner timezone (isoDateOf reads back via local getters too).
// ---------------------------------------------------------------------------
describe("filterLogLines — heatmap day filter", () => {
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

  it("returns no run lines for a day with no runs (honest empty drilldown)", () => {
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
    // The idle "next up" line sits on day2 but survives a day1 filter — the
    // chip can never hide the only line telling the user what's coming next.
    const ids = filterLogLines(lines, { domain: "all", kind: "all", text: "", day: "2026-07-23" }).map((l) => l.id);
    expect(ids).toContain("idle");
  });

  it("is off when day is omitted (backwards compatible)", () => {
    expect(filterLogLines(lines, { domain: "all", kind: "all", text: "" })).toHaveLength(lines.length);
  });
});

// -- #108: an omitted locale = the engine's default negotiation --------------
// The log's date must use the SAME default-locale path as every other date in
// the app (formatTs's plain toLocaleString) — never navigator.language, which
// can disagree with the browser's formatting default (e.g. a macOS "en-US" UI
// language with a Portuguese region). Omitting the locale is that contract.
describe("formatLogDate with omitted locale (#108)", () => {
  it("matches toLocaleDateString's default-locale day/month rendering", () => {
    const ts = new Date(2026, 6, 23, 5, 0, 0).getTime();
    // Build the expected day/month string via the same default negotiation.
    const expected = new Intl.DateTimeFormat(undefined, { day: "2-digit", month: "2-digit" }).format(new Date(ts));
    expect(formatLogDate(ts)).toBe(expected);
  });
});
