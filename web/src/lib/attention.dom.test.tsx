// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";

import type { AnomalySummary, CoverageReport, Run, Settings } from "./api";

vi.mock("./api", async () => {
  const actual = await vi.importActual<typeof import("./api")>("./api");
  return { ...actual, listRuns: vi.fn(), getCoverage: vi.fn(), getAnomalySummary: vi.fn() };
});

const api = await import("./api");
const listRuns = vi.mocked(api.listRuns);
const getCoverage = vi.mocked(api.getCoverage);
const getAnomalySummary = vi.mocked(api.getAnomalySummary);

const { ATTENTION_CHANGED_EVENT, AttentionProvider, attentionCount, recoveryKitDue, unprotectedEntries, useAttentionCount } =
  await import("./attention");
const { AnomalyProvider } = await import("./useAnomalies");

function run(over: Partial<Run> = {}): Run {
  return {
    id: "r1",
    targetId: "t1",
    kind: "backup",
    status: "failed",
    startedAt: 1_700_000_000,
    finishedAt: 1_700_000_500,
    snapshotId: "",
    bytes: 0,
    error: "restic exited 1",
    acknowledged: false,
    target: "paperless",
    domain: "container",
    ...over,
  };
}

function coverage(names: string[], enabled = true): CoverageReport {
  return {
    domains: [
      {
        domain: "containers",
        enabled,
        total: names.length + 3,
        protected: 3,
        unprotected: names.map((name) => ({ name, reason: "not-set-up", neverBackedUp: true })),
      },
    ],
    total: names.length + 3,
    protected: 3,
  };
}

function summary(open: AnomalySummary["open"]): AnomalySummary {
  return {
    enabled: true,
    ready: true,
    generation: 1,
    open,
    recoveredCritical: 0,
    learningItems: 0,
    retentionHeld: 0,
    evalErrors: 0,
    notifyMuted: false,
    backfill: { slots: 0, done: 0, failed: 0, filled: 0, withoutSummary: 0 },
    unmeasuredVolumes: [],
  };
}

const kitStored = { encryptionEnabled: true, recoveryKitAck: true } as Settings;
const kitDue = { encryptionEnabled: true, recoveryKitAck: false } as Settings;

describe("attentionCount", () => {
  const calm = { runs: [], coverage: null, loudAnomalies: 0, settings: kitStored };

  it("is zero when nothing asks for the person", () => {
    expect(attentionCount(calm)).toBe(0);
    expect(attentionCount({ ...calm, settings: null })).toBe(0);
  });

  it("counts a failure until it is acknowledged or a later run succeeds", () => {
    expect(attentionCount({ ...calm, runs: [run()] })).toBe(1);
    expect(attentionCount({ ...calm, runs: [run({ acknowledged: true })] })).toBe(0);
    const retried = [run({ id: "r2", status: "success", startedAt: 1_700_001_000 }), run()];
    expect(attentionCount({ ...calm, runs: retried })).toBe(0);
  });

  it("counts every entry nothing backs up", () => {
    expect(attentionCount({ ...calm, coverage: coverage(["gitea", "immich"]) })).toBe(2);
  });

  it("leaves a backup type that is switched off out", () => {
    expect(unprotectedEntries(coverage(["gitea"], false))).toEqual([]);
    expect(attentionCount({ ...calm, coverage: coverage(["gitea"], false) })).toBe(0);
  });

  it("counts the loud findings and one for a recovery kit that is still to be stored", () => {
    expect(attentionCount({ ...calm, loudAnomalies: 3 })).toBe(3);
    expect(attentionCount({ ...calm, settings: kitDue })).toBe(1);
  });

  it("adds the four up", () => {
    expect(
      attentionCount({ runs: [run()], coverage: coverage(["gitea", "immich"]), loudAnomalies: 3, settings: kitDue })
    ).toBe(7);
  });
});

describe("recoveryKitDue", () => {
  it("holds only for encrypted backups without a confirmed kit", () => {
    expect(recoveryKitDue(kitDue)).toBe(true);
    expect(recoveryKitDue(kitStored)).toBe(false);
    expect(recoveryKitDue({ encryptionEnabled: false, recoveryKitAck: false })).toBe(false);
    expect(recoveryKitDue(null)).toBe(false);
  });
});

describe("AttentionProvider", () => {
  function Count() {
    return <output>{useAttentionCount()}</output>;
  }

  function draw(settings: Settings | null) {
    return render(
      <AnomalyProvider>
        <AttentionProvider settings={settings}>
          <Count />
        </AttentionProvider>
      </AnomalyProvider>
    );
  }

  const shown = () => screen.getByRole("status").textContent;

  beforeEach(() => {
    listRuns.mockReset().mockResolvedValue({ ok: true, runs: [] });
    getCoverage.mockReset().mockResolvedValue({ ok: true, coverage: coverage([]) });
    getAnomalySummary.mockReset().mockResolvedValue({ ok: true, summary: summary({ critical: 0, warning: 0, info: 0 }) });
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it("answers zero to a reader outside it", () => {
    render(<Count />);
    expect(shown()).toBe("0");
  });

  it("adds up runs, coverage, findings and the recovery kit", async () => {
    listRuns.mockResolvedValue({ ok: true, runs: [run(), run({ id: "r2", targetId: "t2" })] });
    getCoverage.mockResolvedValue({ ok: true, coverage: coverage(["gitea"]) });
    getAnomalySummary.mockResolvedValue({ ok: true, summary: summary({ critical: 1, warning: 1, info: 5 }) });
    draw(kitDue);
    await waitFor(() => expect(shown()).toBe("6"));
  });

  it("follows an acknowledged failure without waiting for the next poll", async () => {
    listRuns.mockResolvedValue({ ok: true, runs: [run()] });
    draw(kitStored);
    await waitFor(() => expect(shown()).toBe("1"));

    listRuns.mockResolvedValue({ ok: true, runs: [run({ acknowledged: true })] });
    act(() => {
      window.dispatchEvent(new Event(ATTENTION_CHANGED_EVENT));
    });
    await waitFor(() => expect(shown()).toBe("0"));
  });

  it("reads the coverage again when the settings change", async () => {
    draw(kitStored);
    await waitFor(() => expect(getCoverage).toHaveBeenCalledTimes(1));

    getCoverage.mockResolvedValue({ ok: true, coverage: coverage(["gitea"]) });
    act(() => {
      window.dispatchEvent(new Event("bv:settings-changed"));
    });
    await waitFor(() => expect(shown()).toBe("1"));
  });

  it("keeps the last count when a read fails", async () => {
    listRuns.mockResolvedValue({ ok: true, runs: [run()] });
    draw(kitStored);
    await waitFor(() => expect(shown()).toBe("1"));

    listRuns.mockRejectedValue(new Error("offline"));
    act(() => {
      window.dispatchEvent(new Event(ATTENTION_CHANGED_EVENT));
    });
    await waitFor(() => expect(listRuns).toHaveBeenCalledTimes(2));
    expect(shown()).toBe("1");
  });

  it("polls the runs while the tab is visible", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    draw(kitStored);
    await waitFor(() => expect(listRuns).toHaveBeenCalledTimes(1));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30000);
    });
    expect(listRuns).toHaveBeenCalledTimes(2);
    expect(getCoverage).toHaveBeenCalledTimes(1);
  });
});
