import { describe, expect, it } from "vitest";

import type { Run } from "./api";
import { failedRunsNeedingAttention } from "./runFailures";

function run(over: Partial<Run> & { id: string }): Run {
  return {
    targetId: "zd-media",
    kind: "replica",
    status: "failed",
    startedAt: 1_700_000_000,
    finishedAt: 1_700_000_060,
    snapshotId: "",
    bytes: 0,
    error: "peer offline",
    acknowledged: false,
    target: "tank/media",
    domain: "zfs",
    ...over,
  };
}

describe("failedRunsNeedingAttention", () => {
  it("asks for someone when the replica of an entry fails", () => {
    expect(failedRunsNeedingAttention([run({ id: "r1" })]).map((r) => r.id)).toEqual(["r1"]);
  });

  it("lets a replica that worked again clear its failure, and only that one", () => {
    const runs = [run({ id: "ok", status: "success" }), run({ id: "r1" }), run({ id: "b1", kind: "backup" })];
    expect(failedRunsNeedingAttention(runs).map((r) => r.id)).toEqual(["b1"]);
  });
});
