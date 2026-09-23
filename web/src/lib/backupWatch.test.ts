// fireAndWaitRun retries a busy start for as long as a batch item may run.
// The previous item holds the server's single-flight guard (batchActive in
// service.go) until its whole backup call returns, and that includes its
// inline off-site replication, which can take 20 to 40 seconds. A shorter
// retry budget gives up on the next item before it ever starts, so no run is
// recorded and the item vanishes from the activity log without a line.
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { Run } from "./api";

vi.mock("./api", async () => {
  const actual = await vi.importActual<typeof import("./api")>("./api");
  return { ...actual, listRuns: vi.fn() };
});

import { listRuns } from "./api";
import { fireAndWaitRun } from "./backupWatch";

const mockedListRuns = vi.mocked(listRuns);

function makeRun(overrides: Partial<Run> = {}): Run {
  return {
    id: "run-1",
    targetId: "target-1",
    kind: "backup",
    status: "success",
    startedAt: 0,
    finishedAt: 1,
    snapshotId: "abc123",
    bytes: 0,
    error: "",
    acknowledged: false,
    target: "Windows Server 2022",
    domain: "vm",
    ...overrides,
  };
}

describe("fireAndWaitRun busy retry", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockedListRuns.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("keeps retrying a busy start past 30 seconds, then succeeds", async () => {
    const t0 = Date.now();
    // No prior runs for this target before we fire.
    mockedListRuns.mockResolvedValue({ ok: true, runs: [] });

    // The previous item's off-site replication holds the guard for 40s.
    const BUSY_FOR_MS = 40_000;
    let startCalls = 0;
    const start = vi.fn(async () => {
      startCalls++;
      if (Date.now() - t0 < BUSY_FOR_MS) {
        return { ok: false, error: "a backup is already running" };
      }
      // The guard is free: this start succeeds and listRuns reports the run.
      mockedListRuns.mockResolvedValue({
        ok: true,
        runs: [makeRun({ id: "new-run", status: "success" })],
      });
      return { ok: true, started: true };
    });

    const resultPromise = fireAndWaitRun({
      kind: "backup",
      matchRun: (r) => r.domain === "vm" && r.target === "Windows Server 2022",
      start,
      // The failure path is never reached, so an identity translator will do.
      t: ((key: string) => key) as never,
    });

    // Past the busy window and several poll intervals.
    await vi.advanceTimersByTimeAsync(70_000);

    await expect(resultPromise).resolves.toEqual({ ok: true });
    // One attempt a second, all the way through the busy window.
    expect(startCalls).toBeGreaterThan(35);
  });
});
