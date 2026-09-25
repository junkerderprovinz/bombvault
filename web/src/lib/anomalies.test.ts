// The sentences a reader sees. The backend sends numbers and ids only, so
// every wording, unit and label is decided here, and a metric the tables have
// no sentence for still has to say something.
import { describe, expect, it } from "vitest";
import {
  anomalyDomainsLabel,
  anomalyErrorText,
  anomalyItemLabel,
  anomalySentence,
  anomalySeverityTone,
  anomalyTimeSpan,
  sortOpenAnomalies,
  worstSeverity,
} from "./anomalies";
import { en } from "./i18n";
import { isolateLtr } from "./ltrFragments";
import type { TranslationKey } from "./i18n";
import type { AnomalyView } from "./api";

/** A stub t that answers with the en value, so an unreplaced placeholder shows. */
const t = (key: TranslationKey) => (en as Record<string, string>)[key] ?? key;

/** A t that also records which key it was asked for. */
function recording() {
  const keys: string[] = [];
  const fn = (key: TranslationKey) => {
    keys.push(key);
    return t(key);
  };
  return { fn, keys };
}

function view(over: Partial<AnomalyView>): AnomalyView {
  return {
    id: "an-1",
    detector: "new_data",
    metric: "new_data",
    severity: "warning",
    state: "open",
    scopeKind: "item",
    scopeId: "tg-1",
    targetId: "tg-1",
    domain: "container",
    part: "",
    targetName: "",
    name: "plex",
    runId: "run-1",
    lastRunId: "run-1",
    lastRunAt: 1700000000,
    observed: 0,
    expected: 0,
    threshold: 0,
    samples: 12,
    sensitivity: "balanced",
    details: {},
    occurrences: 1,
    firstSeenAt: 1700000000,
    lastSeenAt: 1700000000,
    recoveredAt: 0,
    resolvedAt: 0,
    ackedAt: 0,
    clearedAt: 0,
    ackNote: "",
    notifiedAt: 0,
    expectable: true,
    retentionHeld: false,
    stillPresent: false,
    ...over,
  };
}

/** metric, the row it describes, the key it must pick, what has to be in it. */
const CASES: [string, AnomalyView, TranslationKey, string[]][] = [
  [
    "new data",
    view({ metric: "new_data", observed: 3 * 1024 ** 3, details: { refBytes: 512 * 1024 ** 2 } }),
    "anomaly.sentence.newData",
    ["plex", "3.0 GB", "512.0 MB"],
  ],
  [
    "a rewrite",
    view({
      metric: "new_data_rewrite",
      observed: 8 * 1024 ** 3,
      details: { sourceBytes: 10 * 1024 ** 3 },
    }),
    "anomaly.sentence.newDataRewrite",
    ["plex", "8.0 GB", "10.0 GB"],
  ],
  [
    "a rewrite where every file looks new",
    view({
      metric: "new_data_rewrite",
      observed: 8 * 1024 ** 3,
      details: { sourceBytes: 10 * 1024 ** 3, allFilesNew: true },
    }),
    "anomaly.sentence.newDataRenamed",
    ["plex", "8.0 GB", "10.0 GB"],
  ],
  [
    "a repository that matched nothing",
    view({ metric: "new_data_full", observed: 2 * 1024 ** 4 }),
    "anomaly.sentence.newDataFull",
    ["plex", "2.0 TB"],
  ],
  [
    "a collapsed source",
    view({
      metric: "source_bytes_shrink",
      observed: 1024,
      expected: 40 * 1024 ** 3,
      details: { collapse: true },
    }),
    "anomaly.sentence.sourceCollapse",
    ["plex", "1.0 KB", "40.0 GB"],
  ],
  [
    "a draining source",
    view({
      metric: "source_bytes_shrink",
      observed: 4 * 1024 ** 3,
      expected: 40 * 1024 ** 3,
      details: { drain: true },
    }),
    "anomaly.sentence.sourceDrain",
    ["plex", "4.0 GB", "40.0 GB"],
  ],
  [
    "a shrunken source",
    view({ metric: "source_bytes_shrink", observed: 20 * 1024 ** 3, expected: 40 * 1024 ** 3 }),
    "anomaly.sentence.sourceShrink",
    ["plex", "20.0 GB", "40.0 GB"],
  ],
  [
    "a grown source",
    view({ metric: "source_bytes_growth", observed: 90 * 1024 ** 3, expected: 40 * 1024 ** 3 }),
    "anomaly.sentence.sourceGrowth",
    ["plex", "90.0 GB", "40.0 GB"],
  ],
  [
    "a collapsed dump",
    view({
      metric: "dump_bytes_shrink",
      scopeKind: "dump",
      observed: 2048,
      expected: 900 * 1024 ** 2,
      details: { collapse: true },
    }),
    "anomaly.sentence.dumpCollapse",
    ["plex", "2.0 KB", "900.0 MB"],
  ],
  [
    "a shrunken dump",
    view({
      metric: "dump_bytes_shrink",
      scopeKind: "dump",
      observed: 300 * 1024 ** 2,
      expected: 900 * 1024 ** 2,
    }),
    "anomaly.sentence.dumpShrink",
    ["plex", "300.0 MB", "900.0 MB"],
  ],
  [
    "a grown dump",
    view({
      metric: "dump_bytes_growth",
      scopeKind: "dump",
      observed: 4 * 1024 ** 3,
      expected: 900 * 1024 ** 2,
    }),
    "anomaly.sentence.dumpGrowth",
    ["plex", "4.0 GB", "900.0 MB"],
  ],
  [
    "a collapsed file count",
    view({
      metric: "source_files_shrink",
      observed: 3,
      expected: 12000,
      details: { collapse: true },
    }),
    "anomaly.sentence.filesCollapse",
    ["plex", "3", (12000).toLocaleString()],
  ],
  [
    "a dropped file count",
    view({ metric: "source_files_shrink", observed: 9000, expected: 12000 }),
    "anomaly.sentence.filesShrink",
    ["plex", (9000).toLocaleString(), (12000).toLocaleString()],
  ],
  [
    "a slower backup",
    view({ metric: "duration_slower", observed: 3_600_000, expected: 300_000 }),
    "anomaly.sentence.duration",
    ["plex", "1h 0m", "5m 0s"],
  ],
  [
    "a slower dump",
    view({ metric: "dump_duration_slower", scopeKind: "dump", observed: 90_000, expected: 12_000 }),
    "anomaly.sentence.dumpDuration",
    ["plex", "1m 30s", "12s"],
  ],
  [
    "a failure streak",
    view({ metric: "failure_streak", observed: 4 }),
    "anomaly.sentence.failureStreak",
    ["plex", "4"],
  ],
  [
    "a dump failure streak",
    view({ metric: "dump_failure_streak", scopeKind: "dump", observed: 3 }),
    "anomaly.sentence.dumpFailureStreak",
    ["plex", "3"],
  ],
  [
    "a flaky item",
    view({ metric: "flaky", details: { failed: 3, total: 10 } }),
    "anomaly.sentence.flaky",
    ["plex", "3", "10"],
  ],
  [
    "a flaky dump",
    view({ metric: "dump_flaky", scopeKind: "dump", details: { failed: 2, total: 10 } }),
    "anomaly.sentence.dumpFlaky",
    ["plex", "2", "10"],
  ],
  [
    "a failed restore check",
    view({
      metric: "drill_subset",
      scopeKind: "domain",
      domain: "containers",
      name: "",
      details: { source: "local" },
    }),
    "anomaly.sentence.drill",
    ["Containers", en["source.local"]],
  ],
  [
    "a failed restore check on a named target",
    view({
      metric: "drill_subset",
      scopeKind: "domain",
      domain: "containers",
      name: "",
      targetName: "wasabi",
      details: { source: "offsite" },
    }),
    "anomaly.sentence.drillTarget",
    ["Containers", "wasabi"],
  ],
  [
    "a failed test restore",
    view({ metric: "drill_dr", scopeKind: "domain", domain: "files", name: "" }),
    "anomaly.sentence.drillDr",
    ["Folders"],
  ],
  [
    "a failed test restore from a named target",
    view({
      metric: "drill_dr",
      scopeKind: "domain",
      domain: "files",
      name: "",
      targetName: "wasabi",
    }),
    "anomaly.sentence.drillDrTarget",
    ["Folders", "wasabi"],
  ],
  [
    "a disk that fills up",
    view({
      metric: "capacity_eta",
      scopeKind: "volume",
      domain: "container,files",
      name: "",
      observed: 21,
    }),
    "anomaly.sentence.capacityEta",
    ["Containers, Folders", "3 weeks"],
  ],
  [
    "a disk with little left",
    view({
      metric: "capacity_low",
      scopeKind: "volume",
      domain: "container",
      name: "",
      observed: 0.04,
      details: { freeBytes: 30 * 1024 ** 3 },
    }),
    "anomaly.sentence.capacityLow",
    ["Containers", "30.0 GB", "4"],
  ],
  [
    "a metric this build does not know",
    view({ metric: "from_a_newer_build" }),
    "anomaly.sentence.unknown",
    ["plex"],
  ],
];

describe("anomalySentence", () => {
  it.each(CASES)("builds the sentence for %s", (_what, a, key, parts) => {
    const { fn, keys } = recording();
    const sentence = anomalySentence(a, fn, "en");
    expect(keys).toContain(key);
    for (const part of parts) expect(sentence).toContain(part);
  });

  it("leaves no placeholder unfilled in any sentence", () => {
    for (const [what, a] of CASES) {
      expect(anomalySentence(a, t, "en"), what).not.toMatch(/\{[a-zA-Z]+\}/);
    }
  });

  // An empty name would leave the sentence starting with a space, and the
  // reader with no idea which backup is meant.
  it("names the backup type where an item has no name of its own", () => {
    const flash = view({
      metric: "source_bytes_shrink",
      domain: "flash",
      name: "",
      observed: 1024,
      expected: 1024 ** 3,
    });
    expect(anomalySentence(flash, t, "en")).toContain(en["dashboard.domainFlash"]);
  });

  // A dump row carries the container's name, and the sentence has to say that
  // the database dump shrank rather than the container.
  it("says a dump is a dump rather than the container", () => {
    const dump = view({
      metric: "dump_bytes_shrink",
      scopeKind: "dump",
      observed: 1024,
      expected: 1024 ** 3,
    });
    expect(anomalySentence(dump, t, "en")).toBe(
      en["anomaly.sentence.dumpShrink"]
        .replace("{name}", "plex")
        .replace("{current}", isolateLtr("1.0 KB"))
        .replace("{typical}", isolateLtr("1.0 GB"))
    );
  });

  // In Arabic or Hebrew prose "4.0 GB" would otherwise show as "GB 4.0".
  it("keeps each figure in one piece inside right-to-left prose", () => {
    const shrink = view({ metric: "source_bytes_shrink", observed: 1024, expected: 1024 ** 3 });
    const sentence = anomalySentence(shrink, t, "en");
    expect(sentence).toContain(isolateLtr("1.0 KB"));
    expect(sentence).toContain(isolateLtr("1.0 GB"));
    const files = view({ metric: "source_files_shrink", observed: 0, expected: 2057 });
    expect(anomalySentence(files, t, "en")).toContain(isolateLtr((2057).toLocaleString()));
  });

  it("names the dataset of a ZFS row, not the pool item", () => {
    const dataset = view({
      metric: "source_bytes_shrink",
      scopeKind: "zfsds",
      part: "tank/appdata",
      observed: 1024,
      expected: 1024 ** 3,
    });
    expect(anomalySentence(dataset, t, "en")).toContain("tank/appdata");
  });
});

describe("anomalyItemLabel", () => {
  it("labels a dump row as the container's database dump", () => {
    expect(anomalyItemLabel({ name: "plex", domain: "container", scopeKind: "dump", part: "" }, t)).toBe(
      en["anomaly.dumpOf"].replace("{name}", "plex")
    );
  });

  it("labels an item by its own name", () => {
    expect(anomalyItemLabel({ name: "plex", domain: "container", scopeKind: "item", part: "" }, t)).toBe("plex");
  });

  it("labels a nameless item by its backup type", () => {
    expect(anomalyItemLabel({ name: "", domain: "config", scopeKind: "item", part: "" }, t)).toBe(
      en["dashboard.domainConfig"]
    );
  });

  it("labels a dataset by the dataset", () => {
    expect(
      anomalyItemLabel({ name: "tank", domain: "zfs", scopeKind: "zfsds", part: "tank/appdata" }, t)
    ).toBe("tank/appdata");
  });
});

describe("anomalyDomainsLabel", () => {
  it("translates a list of backup types", () => {
    expect(anomalyDomainsLabel("container,files", t)).toBe("Containers, Folders");
  });

  it("names a disk that holds the ZFS repository", () => {
    expect(anomalyDomainsLabel("zfs,files", t)).toBe(`${en["dashboard.domainZFS"]}, Folders`);
  });

  // A capacity finding belongs to a volume, which no backup type may own; the
  // sentence still has to name what the disk holds.
  it("falls back to the repositories where no backup type is named", () => {
    expect(anomalyDomainsLabel("", t)).toBe(en["repos.title"]);
  });
});

describe("anomalyTimeSpan", () => {
  it("says less than a day rather than rounding to zero", () => {
    expect(anomalyTimeSpan(0.4, t, "en")).toBe(en["anomaly.days.lessThanOne"]);
  });

  it("uses the locale's plural rules", () => {
    expect(anomalyTimeSpan(1, t, "en")).toBe("1 day");
    expect(anomalyTimeSpan(2, t, "en")).toBe("2 days");
    expect(anomalyTimeSpan(2, t, "de")).toBe("2 Tage");
    expect(anomalyTimeSpan(2, t, "ru")).toBe("2 дня");
    expect(anomalyTimeSpan(5, t, "ru")).toBe("5 дней");
    expect(anomalyTimeSpan(21, t, "ru")).toBe("3 недели");
    expect(anomalyTimeSpan(21, t, "en")).toBe("3 weeks");
  });

  it("counts in weeks once the span passes a fortnight", () => {
    expect(anomalyTimeSpan(13, t, "en")).toBe("13 days");
    expect(anomalyTimeSpan(70, t, "en")).toBe("10 weeks");
  });
});

describe("anomalyErrorText", () => {
  it("names the three failures a caller can act on", () => {
    expect(anomalyErrorText("bad-filter", t)).toBe(en["anomaly.error.badFilter"]);
    expect(anomalyErrorText("bad-request", t)).toBe(en["anomaly.error.badRequest"]);
    expect(anomalyErrorText("not-found", t)).toBe(en["anomaly.error.notFound"]);
  });

  it("falls back to the general wording", () => {
    expect(anomalyErrorText(undefined, t)).toBe(en["anomaly.error.generic"]);
    expect(anomalyErrorText("boom", t)).toBe(en["anomaly.error.generic"]);
  });
});

describe("severity", () => {
  it("paints each severity in its own tone", () => {
    expect(anomalySeverityTone("critical")).toBe("fail");
    expect(anomalySeverityTone("warning")).toBe("warn");
    expect(anomalySeverityTone("info")).toBe("neutral");
  });

  it("reports the worst open severity, or none", () => {
    expect(worstSeverity({ critical: 0, warning: 2, info: 9 })).toBe("warning");
    expect(worstSeverity({ critical: 1, warning: 2, info: 0 })).toBe("critical");
    expect(worstSeverity({ critical: 0, warning: 0, info: 3 })).toBe("info");
    expect(worstSeverity({ critical: 0, warning: 0, info: 0 })).toBe(null);
  });
});

describe("sortOpenAnomalies", () => {
  it("puts the critical, unrecovered and newest first", () => {
    const rows = [
      view({ id: "info", severity: "info", lastSeenAt: 500 }),
      view({ id: "warn", severity: "warning", lastSeenAt: 400 }),
      view({ id: "crit-old", severity: "critical", lastSeenAt: 100 }),
      view({ id: "crit-recovered", severity: "critical", lastSeenAt: 900, recoveredAt: 950 }),
      view({ id: "crit-new", severity: "critical", lastSeenAt: 300 }),
    ];
    expect(sortOpenAnomalies(rows).map((a) => a.id)).toEqual([
      "crit-new",
      "crit-old",
      "crit-recovered",
      "warn",
      "info",
    ]);
  });

  it("leaves the caller's array alone", () => {
    const rows = [view({ id: "a", severity: "info" }), view({ id: "b", severity: "critical" })];
    sortOpenAnomalies(rows);
    expect(rows.map((a) => a.id)).toEqual(["a", "b"]);
  });
});
