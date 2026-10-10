import { describe, expect, it } from "vitest";

import type { AnomalyView, BackupItem, Run } from "./api";
import { needsOf } from "./needs";

function item(over: Partial<BackupItem>): BackupItem {
  return {
    kind: "container",
    key: "plex",
    name: "plex",
    included: true,
    paused: false,
    effectiveSchedule: { kind: "domain", spec: "daily 03:00", alsoSpec: "" },
    lastBackup: 1_700_000_000,
    lastRunStatus: "success",
    sourceBytes: 1,
    installed: true,
    runs: [],
    ...over,
  };
}

function run(over: Partial<Run> & { id: string }): Run {
  return {
    targetId: "plex",
    kind: "backup",
    status: "failed",
    startedAt: 100,
    finishedAt: 130,
    snapshotId: "",
    bytes: 0,
    error: "boom",
    acknowledged: false,
    target: "plex",
    domain: "container",
    ...over,
  };
}

const finding = (id: string, severity: string) => ({ id, severity }) as AnomalyView;
const leftOut = {
  included: false,
  lastBackup: 0,
  lastRunStatus: "",
  effectiveSchedule: { kind: "none", spec: "", alsoSpec: "", reason: "excluded" },
} as const;

const nothing = { items: [], runs: [], anomalies: [], kitDue: false };

describe("needsOf", () => {
  it("is empty where nothing waits", () => {
    expect(needsOf({ ...nothing, items: [item({})] })).toEqual([]);
  });

  it("lists a failed run with the entry it ran for", () => {
    const plex = item({ runs: [{ id: "r1", kind: "backup", status: "failed", startedAt: 100 }] });
    const needs = needsOf({ ...nothing, items: [plex], runs: [run({ id: "r1" })] });
    expect(needs).toEqual([{ is: "failedRun", run: run({ id: "r1" }), item: plex }]);
  });

  it("lists a failed run whose entry the list no longer has", () => {
    const needs = needsOf({ ...nothing, runs: [run({ id: "r1" })] });
    expect(needs).toMatchObject([{ is: "failedRun", item: undefined }]);
  });

  it("drops a failure that was acknowledged or repeated with success", () => {
    const acked = run({ id: "r1", targetId: "a", acknowledged: true });
    const retried = [run({ id: "r3", targetId: "b", status: "success" }), run({ id: "r2", targetId: "b" })];
    expect(needsOf({ ...nothing, runs: [acked, ...retried] })).toEqual([]);
  });

  it("lists an entry no schedule reaches and that has no backup", () => {
    const gitea = item({ key: "gitea", ...leftOut });
    expect(needsOf({ ...nothing, items: [item({}), gitea] })).toEqual([{ is: "unprotected", item: gitea }]);
  });

  it("does not list an entry that only waits for its first scheduled run", () => {
    expect(needsOf({ ...nothing, items: [item({ lastBackup: 0, lastRunStatus: "" })] })).toEqual([]);
  });

  it("lists an entry once when its only run failed and it has no backup", () => {
    const gitea = item({ key: "gitea", ...leftOut, runs: [{ id: "r1", kind: "backup", status: "failed", startedAt: 100 }] });
    const needs = needsOf({ ...nothing, items: [gitea], runs: [run({ id: "r1", targetId: "gitea" })] });
    expect(needs.map((n) => n.is)).toEqual(["failedRun"]);
  });

  it("lists critical and warning findings and keeps notes out", () => {
    const anomalies = [finding("a", "critical"), finding("b", "info"), finding("c", "warning")];
    expect(needsOf({ ...nothing, anomalies }).map((n) => (n.is === "anomaly" ? n.finding.id : n.is))).toEqual(["a", "c"]);
  });

  it("lists the recovery kit while it is due", () => {
    expect(needsOf({ ...nothing, kitDue: true })).toEqual([{ is: "recoveryKit" }]);
  });

  it("orders failures, entries, findings and the kit", () => {
    const needs = needsOf({
      items: [item({ key: "gitea", ...leftOut })],
      runs: [run({ id: "r1" })],
      anomalies: [finding("a", "warning")],
      kitDue: true,
    });
    expect(needs.map((n) => n.is)).toEqual(["failedRun", "unprotected", "anomaly", "recoveryKit"]);
  });
});
