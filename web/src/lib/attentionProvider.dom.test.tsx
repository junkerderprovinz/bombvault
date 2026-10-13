// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";

import type { AnomalySummary, AnomalyView, BackupItem, Run, Settings, StorageLocation } from "./api";

vi.mock("./api", async () => {
  const actual = await vi.importActual<typeof import("./api")>("./api");
  return {
    ...actual,
    listRuns: vi.fn(),
    listItems: vi.fn(),
    listStorageLocations: vi.fn(),
    getAnomalySummary: vi.fn(),
    getAnomalies: vi.fn(),
  };
});

const api = await import("./api");
const listRuns = vi.mocked(api.listRuns);
const listItems = vi.mocked(api.listItems);
const listStorageLocations = vi.mocked(api.listStorageLocations);
const getAnomalySummary = vi.mocked(api.getAnomalySummary);
const getAnomalies = vi.mocked(api.getAnomalies);

const { ATTENTION_CHANGED_EVENT, useAttention, useAttentionCount, useFreshInstall } = await import("./attention");
const { AttentionProvider, recoveryKitDue } = await import("./attentionProvider");
const { AnomalyProvider } = await import("./useAnomalies");
const { storageLocationsChanged } = await import("./useStorageLocations");

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

/** A container no schedule reaches and that has no backup. */
function leftOut(name: string, over: Partial<BackupItem> = {}): BackupItem {
  return {
    kind: "container",
    key: name,
    name,
    included: false,
    paused: false,
    effectiveSchedule: { kind: "none", spec: "", reason: "excluded" },
    lastBackup: 0,
    lastRunStatus: "",
    sourceBytes: 1,
    installed: true,
    runs: [],
    ...over,
  };
}

/** A container a schedule reaches and that has a backup. */
const backedUp = leftOut("plex", { lastBackup: 1_700_000_000, included: true, effectiveSchedule: { kind: "domain", spec: "daily 03:00" } });

/** The entries of an install that has a backup of plex. */
const items = (list: BackupItem[]) => ({ ok: true, items: [backedUp, ...list], unlisted: [] });

/** The entries of an install before its first backup. */
const freshItems = (list: BackupItem[]) => ({ ok: true, items: list, unlisted: [] });

/** A storage location whose copy of the containers has failed since it last worked. */
function failingBox(acknowledged = false): StorageLocation {
  return {
    id: "destination:box",
    object: "destination",
    kind: "offsite",
    provider: "storagebox",
    backend: "rclone",
    name: "Storage Box",
    where: "rclone:box:",
    enabled: true,
    offPremises: true,
    sections: [
      {
        domain: "containers",
        use: "copy",
        targetId: "t-containers",
        where: "rclone:box:containers",
        enabled: true,
        immutable: false,
        retention: { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 6, keepYearly: 1 },
        compression: "auto",
        limitUpload: 0,
        limitDownload: 0,
        own: [],
        lastCopy: { at: 1_700_000_000, ok: false, failingSince: 1_700_050_000, acknowledged },
      },
    ],
    protection: { immutable: false, testable: false },
    capacity: {},
  };
}

const places = (list: StorageLocation[]) => ({ ok: true, locations: list });

function summary(open: AnomalySummary["open"], enabled = true): AnomalySummary {
  return {
    enabled,
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

const finding = (id: string, severity: string) => ({ id, severity }) as AnomalyView;
const findings = (list: AnomalyView[]) => ({ ok: true, anomalies: list, nextCursor: "" });

const kitStored = { encryptionEnabled: true, recoveryKitAck: true } as Settings;
const kitDue = { encryptionEnabled: true, recoveryKitAck: false } as Settings;

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

  function Fresh() {
    return <i data-testid="fresh">{String(useFreshInstall())}</i>;
  }

  /** A page that lists what the navigation counts. */
  function Page() {
    const { needs } = useAttention();
    return <p data-testid="listed">{needs.map((need) => need.is).join(" ")}</p>;
  }

  function draw(settings: Settings | null, page = false) {
    const tree = (withPage: boolean) => (
      <AnomalyProvider>
        <AttentionProvider settings={settings}>
          <Count />
          {withPage && <Page />}
        </AttentionProvider>
      </AnomalyProvider>
    );
    const view = render(tree(page));
    return { visit: (on: boolean) => view.rerender(tree(on)) };
  }

  const shown = () => screen.getByRole("status").textContent;
  const listed = () => screen.getByTestId("listed").textContent;
  const fresh = () => screen.getByTestId("fresh").textContent;

  beforeEach(() => {
    listRuns.mockReset().mockResolvedValue({ ok: true, runs: [] });
    listItems.mockReset().mockResolvedValue(items([]));
    listStorageLocations.mockReset().mockResolvedValue(places([]));
    getAnomalySummary.mockReset().mockResolvedValue({ ok: true, summary: summary({ critical: 0, warning: 0, info: 0 }) });
    getAnomalies.mockReset().mockResolvedValue(findings([]));
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it("answers zero to a reader outside it", () => {
    render(<Count />);
    expect(shown()).toBe("0");
  });

  it("counts exactly what the page lists", async () => {
    listRuns.mockResolvedValue({ ok: true, runs: [run(), run({ id: "r2", targetId: "t2" })] });
    listItems.mockResolvedValue(items([leftOut("gitea"), leftOut("flash", { kind: "flash" })]));
    getAnomalySummary.mockResolvedValue({ ok: true, summary: summary({ critical: 1, warning: 1, info: 1 }) });
    getAnomalies.mockResolvedValue(findings([finding("a1", "critical"), finding("a2", "warning"), finding("a3", "info")]));
    listStorageLocations.mockResolvedValue(places([failingBox()]));
    draw(kitDue, true);

    await waitFor(() => expect(shown()).toBe("8"));
    expect(listed()).toBe("failedRun failedRun unprotected unprotected failedCopy anomaly anomaly recoveryKit");
  });

  it("counts an entry once whose last run failed and that has no backup", async () => {
    listRuns.mockResolvedValue({ ok: true, runs: [run()] });
    listItems.mockResolvedValue(
      items([leftOut("paperless", { runs: [{ id: "r1", kind: "backup", status: "failed", startedAt: 1_700_000_000 }] })])
    );
    draw(kitStored);

    await waitFor(() => expect(listItems).toHaveBeenCalled());
    await waitFor(() => expect(shown()).toBe("1"));
  });

  it("leaves out an entry the person said not to back up", async () => {
    const backedUp = leftOut("immich", { lastBackup: 1_700_000_000, included: true, effectiveSchedule: { kind: "domain", spec: "daily 03:00" } });
    listItems.mockResolvedValue(items([backedUp, leftOut("gitea"), leftOut("scratch", { backupDeclined: true })]));
    draw(kitStored, true);

    await waitFor(() => expect(shown()).toBe("1"));
    expect(listed()).toBe("unprotected");
  });

  it("leaves the findings out while detection is off", async () => {
    getAnomalySummary.mockResolvedValue({ ok: true, summary: summary({ critical: 1, warning: 0, info: 0 }, false) });
    getAnomalies.mockResolvedValue(findings([finding("a1", "critical")]));
    listRuns.mockResolvedValue({ ok: true, runs: [run()] });
    draw(kitStored);

    await waitFor(() => expect(getAnomalies).toHaveBeenCalled());
    await waitFor(() => expect(shown()).toBe("1"));
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

  it("drops a failed copy that was marked as seen without waiting for a run to end", async () => {
    listStorageLocations.mockResolvedValue(places([failingBox()]));
    draw(kitStored);
    await waitFor(() => expect(shown()).toBe("1"));

    listStorageLocations.mockResolvedValue(places([failingBox(true)]));
    act(() => {
      window.dispatchEvent(new Event(ATTENTION_CHANGED_EVENT));
    });
    await waitFor(() => expect(shown()).toBe("0"));
  });

  it("reads the storage locations again when one of them was written", async () => {
    listStorageLocations.mockResolvedValue(places([failingBox()]));
    draw(kitStored);
    await waitFor(() => expect(shown()).toBe("1"));

    listStorageLocations.mockResolvedValue(places([]));
    act(() => storageLocationsChanged());
    await waitFor(() => expect(shown()).toBe("0"));
  });

  it("keeps a failed copy in the count when the storage locations cannot be read", async () => {
    listStorageLocations.mockResolvedValue(places([failingBox()]));
    draw(kitStored);
    await waitFor(() => expect(shown()).toBe("1"));

    listStorageLocations.mockResolvedValue({ ok: false });
    act(() => storageLocationsChanged());
    await waitFor(() => expect(listStorageLocations).toHaveBeenCalledTimes(2));
    await act(async () => {});
    expect(shown()).toBe("1");
  });

  it("drops the recovery kit as soon as the settings say it is stored", async () => {
    const view = render(
      <AttentionProvider settings={kitDue}>
        <Count />
      </AttentionProvider>
    );
    await waitFor(() => expect(shown()).toBe("1"));

    view.rerender(
      <AttentionProvider settings={kitStored}>
        <Count />
      </AttentionProvider>
    );
    expect(shown()).toBe("0");
  });

  it("reads the entries again when the settings change", async () => {
    draw(kitStored);
    await waitFor(() => expect(listItems).toHaveBeenCalledTimes(1));

    listItems.mockResolvedValue(items([leftOut("gitea")]));
    act(() => {
      window.dispatchEvent(new Event("bv:settings-changed"));
    });
    await waitFor(() => expect(shown()).toBe("1"));
  });

  it("reads the entries again when a run has ended", async () => {
    listRuns.mockResolvedValue({ ok: true, runs: [run({ status: "running", finishedAt: null })] });
    listItems.mockResolvedValue(items([leftOut("gitea")]));
    draw(kitStored);
    await waitFor(() => expect(shown()).toBe("1"));
    expect(listItems).toHaveBeenCalledTimes(1);

    listRuns.mockResolvedValue({ ok: true, runs: [run({ status: "success" })] });
    listItems.mockResolvedValue(items([leftOut("gitea", { lastBackup: 1_700_000_500, included: true, effectiveSchedule: { kind: "domain", spec: "daily 03:00" } })]));
    act(() => {
      window.dispatchEvent(new Event(ATTENTION_CHANGED_EVENT));
    });
    await waitFor(() => expect(shown()).toBe("0"));
    expect(listItems).toHaveBeenCalledTimes(2);
  });

  it("reads the storage locations again when a copy has ended", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const copying = run({ kind: "offsite", targetId: "containers", status: "running", finishedAt: null });
    listRuns.mockResolvedValue({ ok: true, runs: [copying] });
    draw(kitStored);
    await waitFor(() => expect(listRuns).toHaveBeenCalledTimes(1));
    await act(async () => {});
    expect(shown()).toBe("0");

    listRuns.mockResolvedValue({ ok: true, runs: [{ ...copying, status: "failed", finishedAt: 1_700_050_060 }] });
    listStorageLocations.mockResolvedValue(places([failingBox()]));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30000);
    });

    await waitFor(() => expect(shown()).toBe("1"));
    expect(listStorageLocations).toHaveBeenCalledTimes(2);
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

  it("polls the runs while the tab is visible, and never the entries or the storage locations", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    draw(kitStored);
    await waitFor(() => expect(listRuns).toHaveBeenCalledTimes(1));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30000);
    });
    expect(listRuns).toHaveBeenCalledTimes(2);
    expect(listItems).toHaveBeenCalledTimes(1);
    expect(listStorageLocations).toHaveBeenCalledTimes(1);
  });

  it("reads once for a page that opens with the app", async () => {
    draw(kitStored, true);
    await waitFor(() => expect(listed()).toBe(""));
    await act(async () => {});

    expect(listRuns).toHaveBeenCalledTimes(1);
    expect(listItems).toHaveBeenCalledTimes(1);
    expect(listStorageLocations).toHaveBeenCalledTimes(1);
  });

  it("brings the lists up to date for a page that is visited later", async () => {
    const view = draw(kitStored);
    await waitFor(() => expect(listItems).toHaveBeenCalledTimes(1));
    await act(async () => {});

    listItems.mockResolvedValue(items([leftOut("gitea")]));
    view.visit(true);

    await waitFor(() => expect(listed()).toBe("unprotected"));
    expect(shown()).toBe("1");
    expect(listRuns).toHaveBeenCalledTimes(2);
    expect(listItems).toHaveBeenCalledTimes(2);
    expect(listStorageLocations).toHaveBeenCalledTimes(2);
  });

  it("reports a first start once the entries are read, until one of them has a backup", async () => {
    listItems.mockResolvedValue(freshItems([leftOut("gitea")]));
    render(
      <AttentionProvider settings={kitDue}>
        <Count />
        <Fresh />
      </AttentionProvider>
    );
    expect(fresh()).toBe("false");
    await waitFor(() => expect(fresh()).toBe("true"));
    expect(shown()).toBe("0");

    listItems.mockResolvedValue(
      freshItems([leftOut("gitea", { lastBackup: 1_700_000_500, included: true, effectiveSchedule: { kind: "domain", spec: "daily 03:00" } })])
    );
    act(() => {
      window.dispatchEvent(new Event("bv:settings-changed"));
    });
    await waitFor(() => expect(fresh()).toBe("false"));
    expect(shown()).toBe("1");
  });

  it("takes a server with nothing to list for a first start", async () => {
    listItems.mockResolvedValue({ ok: true, items: null as unknown as BackupItem[], unlisted: ["container"] });
    render(
      <AttentionProvider settings={kitStored}>
        <Fresh />
      </AttentionProvider>
    );
    await waitFor(() => expect(fresh()).toBe("true"));
  });

  it("forgets a ticked-off first step once the install has a backup", async () => {
    const { rememberSectionsChosen, sectionsChosen } = await import("./firstSteps");
    rememberSectionsChosen();
    listItems.mockResolvedValue(freshItems([leftOut("gitea")]));
    render(
      <AttentionProvider settings={kitStored}>
        <Fresh />
      </AttentionProvider>
    );
    await waitFor(() => expect(fresh()).toBe("true"));
    expect(sectionsChosen()).toBe(true);

    listItems.mockResolvedValue(items([]));
    act(() => {
      window.dispatchEvent(new Event("bv:settings-changed"));
    });
    await waitFor(() => expect(fresh()).toBe("false"));
    expect(sectionsChosen()).toBe(false);
  });

  it("does not take a list that never arrived for a first start", async () => {
    listItems.mockRejectedValue(new Error("offline"));
    render(
      <AttentionProvider settings={kitStored}>
        <Fresh />
      </AttentionProvider>
    );
    await waitFor(() => expect(listItems).toHaveBeenCalled());
    await act(async () => {});
    expect(fresh()).toBe("false");
  });

  it("reads the runs at the activity log's rate while a page shows them", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const view = draw(kitStored, true);
    await waitFor(() => expect(listRuns).toHaveBeenCalledTimes(1));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30000);
    });
    expect(listRuns).toHaveBeenCalledTimes(4);

    view.visit(false);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30000);
    });
    expect(listRuns).toHaveBeenCalledTimes(5);
  });
});
