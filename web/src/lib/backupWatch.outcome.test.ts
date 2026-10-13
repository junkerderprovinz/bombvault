// How fireAndWaitRun ends a run that is not a plain success or failure.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Run } from "./api";
import type { ProgressState } from "./progress";

vi.mock("./api", async () => {
  const actual = await vi.importActual<typeof import("./api")>("./api");
  return { ...actual, listRuns: vi.fn() };
});
vi.mock("./progress", async () => {
  const actual = await vi.importActual<typeof import("./progress")>("./progress");
  return { ...actual, progressNow: vi.fn() };
});

import { listRuns } from "./api";
import { fireAndWaitRun } from "./backupWatch";
import { progressNow } from "./progress";

const runs = vi.mocked(listRuns);
const live = vi.mocked(progressNow);
const t = ((key: string) => key) as never;

function restoreRun(id: string, status: string, error = ""): Run {
  return {
    id,
    targetId: "t",
    kind: "restore",
    status,
    startedAt: 0,
    finishedAt: status === "running" ? null : 1,
    snapshotId: "",
    bytes: 0,
    error,
    acknowledged: false,
    target: "plex",
    domain: "container",
  };
}

function watch(start = vi.fn(async () => ({ ok: true, started: true })), progressKey?: string) {
  return {
    start,
    result: fireAndWaitRun({ kind: "restore", matchRun: (r) => r.target === "plex", start, t, progressKey }),
  };
}

describe("fireAndWaitRun", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    runs.mockReset();
    live.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("ends a restore that was cancelled elsewhere", async () => {
    runs.mockResolvedValueOnce({ ok: true, runs: [restoreRun("old", "success")] });
    runs.mockResolvedValue({ ok: true, runs: [restoreRun("new", "cancelled"), restoreRun("old", "success")] });
    const { result } = watch();
    await vi.advanceTimersByTimeAsync(5000);
    await expect(result).resolves.toEqual({ ok: false, cancelled: true, error: "run.statusCancelled" });
  });

  it("ends a run in a state it does not know as a failure", async () => {
    runs.mockResolvedValueOnce({ ok: true, runs: [] });
    runs.mockResolvedValue({ ok: true, runs: [restoreRun("new", "interrupted")] });
    const { result } = watch();
    await vi.advanceTimersByTimeAsync(5000);
    await expect(result).resolves.toEqual({ ok: false, error: "common.restoreFailed" });
  });

  it("keeps waiting while the run is still running", async () => {
    runs.mockResolvedValueOnce({ ok: true, runs: [] });
    runs.mockResolvedValue({ ok: true, runs: [restoreRun("new", "running")] });
    let done = false;
    void watch().result.then(() => (done = true));
    await vi.advanceTimersByTimeAsync(60_000);
    expect(done).toBe(false);
  });

  it("ends a run the server did not record once its progress came and went", async () => {
    runs.mockResolvedValue({ ok: true, runs: [] });
    let active = true;
    live.mockImplementation(() => ({ active }) as ProgressState);
    const { result } = watch(undefined, "container:plex");
    await vi.advanceTimersByTimeAsync(5000);
    active = false;
    await vi.advanceTimersByTimeAsync(10_000);
    await expect(result).resolves.toEqual({ ok: true });
  });

  it("does not count a run that finished before it fired when the list was unreadable", async () => {
    runs.mockRejectedValue(new Error("offline"));
    const { start, result } = watch();
    await vi.advanceTimersByTimeAsync(5000);
    await expect(result).resolves.toEqual({ ok: false, error: "common.networkError" });
    expect(start).not.toHaveBeenCalled();
  });

  it("fires once the list answers on a later try", async () => {
    runs.mockRejectedValueOnce(new Error("offline"));
    runs.mockResolvedValueOnce({ ok: true, runs: [restoreRun("old", "failed")] });
    runs.mockResolvedValue({ ok: true, runs: [restoreRun("new", "success"), restoreRun("old", "failed")] });
    const { start, result } = watch();
    await vi.advanceTimersByTimeAsync(8000);
    await expect(result).resolves.toEqual({ ok: true });
    expect(start).toHaveBeenCalledOnce();
  });
});
