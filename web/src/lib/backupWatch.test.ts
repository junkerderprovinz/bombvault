// ---------------------------------------------------------------------------
// Regression test for #154: "select all VMs, back up selected" silently
// skipped exactly one VM (Windows Server 2022) with NO success line, NO
// failure line, nothing in the activity log at all.
//
// Root cause: fireAndWaitRun (the bulk "back up / restore selected" fire-
// retry-wait helper every batch item runs through) used to give up firing a
// batch item after a FIXED 30s "busy" retry budget (the old BUSY_RETRY_MS).
// But the PREVIOUS item's single-flight guard (service.go's batchActive)
// does not release until that item's entire backup call returns — which
// includes its own inline, synchronous off-site replication
// (Service.replicateOffsite, called from BackupVM/Backup AFTER the run row
// is already marked "success" but BEFORE the wrapping goroutine, and
// therefore batchActive, releases). The reported activity log shows real
// inline replications taking 20-40s; a 37s one sits right past the old 30s
// budget. The next batch item's start() kept getting rejected "a backup is
// already running" until the retry gave up — and because start() never
// actually SUCCEEDED for that item, no run was ever recorded for it: nothing
// to show in the activity log, exactly the silent skip reported.
//
// The fix folds the fire-retry budget into the SAME generous watchTimeoutMs
// deadline already used to wait out a real in-progress backup/restore, so a
// transient "still releasing" busy signal is retried for as long as a batch
// item is allowed to run at all, never abandoned on an arbitrary shorter
// clock. This test pins that: it fails against the pre-fix 30s cap (the
// batch item never starts) and passes once the retry survives past it.
// ---------------------------------------------------------------------------
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

describe("fireAndWaitRun busy-retry (#154)", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    mockedListRuns.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("keeps retrying a busy start well past the old 30s cap and still starts + succeeds", async () => {
    const t0 = Date.now();
    // No prior runs for this target before we fire.
    mockedListRuns.mockResolvedValue({ ok: true, runs: [] });

    // Simulate the PREVIOUS batch item's inline off-site replication holding
    // the shared single-flight guard for 40 real seconds — longer than the
    // old fixed 30s retry budget, matching the ~37-40s replication windows
    // visible in the reported activity log.
    const BUSY_FOR_MS = 40_000;
    let startCalls = 0;
    const start = vi.fn(async () => {
      startCalls++;
      if (Date.now() - t0 < BUSY_FOR_MS) {
        return { ok: false, error: "a backup is already running" };
      }
      // The guard has freed up: this start actually succeeds and a run gets
      // recorded — from here on, listRuns() must surface it.
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
      // fireAndWaitRun now takes the translate function for its failure tail
      // (bombvault/user-message-is-translated). This test never reaches that
      // tail, so identity is enough and keeps the assertions reading in keys.
      t: ((key: string) => key) as never,
    });

    // Fast-forward well past the old 30s cap (retry) AND the poll interval
    // (watch) — the batch item must still complete successfully.
    await vi.advanceTimersByTimeAsync(70_000);

    await expect(resultPromise).resolves.toEqual({ ok: true });
    // Proves the helper kept retrying the busy start past the old 30s/30-call
    // ceiling instead of giving up early.
    expect(startCalls).toBeGreaterThan(35);
  });
});
