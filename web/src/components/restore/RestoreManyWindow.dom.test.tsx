// @vitest-environment jsdom
/**
 * The window that restores several entries one after the other: it picks and
 * checks every entry the way the window for one entry does, never starts an
 * entry gone from the server without a yes in its row, and ends every entry
 * however its run ends.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";

import type { AnomalyView, BackupItem, RestoreCheckRequest, Run, TimelinePlace, TimelineRow } from "../../lib/api";
import { I18nProvider, en } from "../../lib/i18n";

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    checkRestore: vi.fn(),
    restore: vi.fn(),
    listRuns: vi.fn(),
    getTimeline: vi.fn(),
    getTimelinePlace: vi.fn(),
    cancelRestore: vi.fn(),
    openedAsPairedPhone: vi.fn(() => false),
  };
});

const pushed: string[] = [];
vi.mock("../../lib/toast", () => ({ useToast: () => ({ push: (message: string) => pushed.push(message) }) }));

let openFindings: AnomalyView[] = [];
vi.mock("../../lib/useAnomalies", () => ({
  useOpenAnomalies: () => ({
    list: openFindings,
    byRunId: new Map(),
    flagged: new Set(openFindings.flatMap((a) => a.flaggedSnapshots ?? [])),
    error: false,
  }),
}));

const api = await import("../../lib/api");
const checkRestore = vi.mocked(api.checkRestore);
const restore = vi.mocked(api.restore);
const listRuns = vi.mocked(api.listRuns);
const getTimeline = vi.mocked(api.getTimeline);
const getTimelinePlace = vi.mocked(api.getTimelinePlace);

const { RestoreManyWindow } = await import("./RestoreManyWindow");
const { NotStartedNotice } = await import("./NotStartedNotice");

window.matchMedia = ((query: string) => ({
  matches: true,
  media: query,
  onchange: null,
  addListener: () => {},
  removeListener: () => {},
  addEventListener: () => {},
  removeEventListener: () => {},
  dispatchEvent: () => false,
})) as unknown as typeof window.matchMedia;

class NoopEventSource {
  onmessage: unknown = null;
  close() {}
}

const DAY = 86_400;
const NOW = 1_800_000_000;

function item(key: string, over: Partial<BackupItem> = {}): BackupItem {
  return {
    kind: "container",
    key,
    name: key,
    included: true,
    paused: false,
    effectiveSchedule: { kind: "everything", spec: "daily@03:00" },
    lastBackup: NOW,
    lastRunStatus: "success",
    sourceBytes: 1024,
    installed: true,
    runs: [],
    ...over,
  };
}

const LOCAL: TimelinePlace = { place: "local", label: "local", kind: "home", remote: false, enabled: true, appendOnly: false, state: "read" };
const OFFSITE: TimelinePlace = { place: "offsite:b2", label: "b2", kind: "target", remote: true, enabled: true, appendOnly: false, state: "unchecked" };

function row(id: string, daysAgo: number, tags: string[] = [], place = "local"): TimelineRow {
  return { key: id, time: new Date((NOW - daysAgo * DAY) * 1000).toISOString(), places: [{ place, snapshotIds: [id], tags }] };
}

/** Each entry has a backup today and one two days ago, unless set here. */
let rowsOf: Record<string, TimelineRow[]> = {};
/** How the recorded run of each entry ends; "running" keeps it going. */
let endOf: Record<string, string> = {};
let runs: Run[] = [];
let runCount = 0;

function recordRun(target: string, status: string, error = "") {
  runs = [
    {
      id: `run-${++runCount}`,
      targetId: target,
      kind: "restore",
      status,
      startedAt: 0,
      finishedAt: status === "running" ? null : 1,
      snapshotId: "",
      bytes: 0,
      error,
      acknowledged: false,
      target,
      domain: "container",
    } as Run,
    ...runs,
  ];
}

function settle(target: string, status: string, error = "") {
  runs = runs.map((r) => (r.target === target && r.status === "running" ? { ...r, status, error, finishedAt: 1 } : r));
}

const onClose = vi.fn();

function open(items: BackupItem[]) {
  return render(
    <I18nProvider>
      <RestoreManyWindow items={items} onClose={onClose} />
    </I18nProvider>
  );
}

const startButton = () => screen.getByRole("button", { name: new RegExp(`^${en["snapshots.restore"]} · \\d+`) });
const startCount = () => Number(/· (\d+)/.exec(startButton().textContent ?? "")?.[1]);
const entry = (name: string) => screen.getAllByTestId("restore-many-entry").find((li) => li.textContent?.includes(name))!;
const tick = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));
const lastCheckOf = (name: string): RestoreCheckRequest | undefined =>
  checkRestore.mock.calls.map(([req]) => req).filter((req) => req.name === name).at(-1);

async function readyToStart(count: number) {
  await waitFor(() => expect(startCount()).toBe(count));
  await waitFor(() => expect(startButton()).not.toHaveProperty("disabled", true));
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(NOW * 1000);
  localStorage.clear();
  localStorage.setItem("bv-lang", "en");
  vi.stubGlobal("EventSource", NoopEventSource);
  pushed.length = 0;
  openFindings = [];
  rowsOf = {};
  endOf = {};
  runs = [];
  onClose.mockClear();
  getTimeline.mockImplementation(async (_domain, key) => ({
    ok: true,
    places: [LOCAL],
    rows: rowsOf[key] ?? [row(`${key}-new`, 0), row(`${key}-old`, 2)],
  }));
  getTimelinePlace.mockResolvedValue({ ok: false });
  checkRestore.mockResolvedValue({ ok: true, ready: true, checks: [{ id: "repository", status: "ok" }], plan: { shared: [] } as never });
  restore.mockImplementation(async (name) => {
    recordRun(name, endOf[name] ?? "success");
    return { ok: true, started: true };
  });
  listRuns.mockImplementation(async () => ({ ok: true, runs }));
  vi.mocked(api.cancelRestore).mockResolvedValue({ ok: true } as never);
  vi.mocked(api.openedAsPairedPhone).mockReturnValue(false);
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("a restore that ends in another way than success or failure", () => {
  it("is ended as cancelled, stops the rest and leaves the window closable", async () => {
    endOf = { plex: "running" };
    open([item("plex"), item("sonarr")]);
    await readyToStart(2);
    fireEvent.click(startButton());
    await tick(3000);
    expect(entry("plex").dataset.outcome).toBe("running");

    settle("plex", "cancelled");
    await tick(3000);
    expect(entry("plex").dataset.outcome).toBe("cancelled");
    expect(entry("sonarr").dataset.outcome).toBe("waiting");
    expect(restore).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: en["common.close"] }));
    expect(onClose).toHaveBeenCalled();
  });

  it("can be cancelled from its own row", async () => {
    endOf = { plex: "running" };
    open([item("plex"), item("sonarr")]);
    await readyToStart(2);
    fireEvent.click(startButton());
    await tick(3000);
    fireEvent.click(within(entry("plex")).getByRole("button", { name: en["restore.cancel"] }));
    fireEvent.click(await screen.findByRole("button", { name: en["common.confirm"] }));
    await waitFor(() => expect(api.cancelRestore).toHaveBeenCalledWith("container:plex"));
  });
});

describe("leaving while the restores run", () => {
  it("keeps the window on Back and says how to stop", async () => {
    endOf = { plex: "running" };
    open([item("plex"), item("sonarr")]);
    await readyToStart(2);
    fireEvent.click(startButton());
    await tick(1000);
    act(() => window.history.back());
    await tick(100);
    expect(screen.getAllByTestId("restore-many-entry")).toHaveLength(2);
    expect(pushed.at(-1)).toContain(en["restoreMany.stop"]);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("warns before the tab closes", async () => {
    endOf = { plex: "running" };
    open([item("plex")]);
    await readyToStart(1);
    fireEvent.click(startButton());
    await tick(1000);
    const leave = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(leave);
    expect(leave.defaultPrevented).toBe(true);
  });

  it("leaves the names of the entries that did not start for the Restore page", async () => {
    endOf = { plex: "running" };
    const view = open([item("plex"), item("sonarr"), item("radarr")]);
    await readyToStart(3);
    fireEvent.click(startButton());
    await tick(3000);
    view.unmount();
    render(
      <I18nProvider>
        <NotStartedNotice />
      </I18nProvider>
    );
    expect(await screen.findByRole("status")).toHaveProperty(
      "textContent",
      en["restoreMany.leftBehind"].replace("{names}", "sonarr, radarr")
    );
  });

  it("names nothing once the run has ended in the window", async () => {
    endOf = { plex: "running" };
    const view = open([item("plex"), item("sonarr")]);
    await readyToStart(2);
    fireEvent.click(startButton());
    await tick(1000);
    fireEvent.click(screen.getByRole("button", { name: en["restoreMany.stop"] }));
    settle("plex", "success");
    await tick(3000);
    await waitFor(() => expect(screen.getByRole("button", { name: en["common.close"] })).toBeTruthy());
    expect(entry("sonarr").dataset.outcome).toBe("waiting");
    view.unmount();
    render(
      <I18nProvider>
        <NotStartedNotice />
      </I18nProvider>
    );
    await tick(100);
    expect(screen.queryByRole("status")).toBeNull();
  });
});

describe("the backup each entry takes", () => {
  it("passes over a backup taken before a restore", async () => {
    rowsOf = { plex: [row("plex-before", 0, ["bv:before-restore:abcd1234"]), row("plex-good", 1)] };
    open([item("plex")]);
    await readyToStart(1);
    expect(lastCheckOf("plex")).toMatchObject({ snapshotId: "plex-good" });
    expect(entry("plex").textContent).toContain(en["restoreMany.passedOver"]);
  });

  it("is the last good one before an open finding, and says why", async () => {
    openFindings = [
      {
        id: "a1",
        severity: "critical",
        state: "open",
        domain: "container",
        name: "plex",
        targetId: "tg-plex",
        metric: "source_bytes_shrink",
        scopeKind: "item",
        observed: 1,
        expected: 2,
        details: {},
        lastGood: { runId: "r1", snapshotId: "plex-old", at: NOW - 2 * DAY },
        flaggedSnapshots: ["plex-new"],
      } as unknown as AnomalyView,
    ];
    open([item("plex")]);
    await readyToStart(1);
    expect(lastCheckOf("plex")).toMatchObject({ snapshotId: "plex-old" });
    expect(entry("plex").textContent).toContain(en["restoreWindow.whyBefore"].split("{finding}")[0]);
    fireEvent.click(startButton());
    await tick(3000);
    expect(restore.mock.calls[0][1]).toBe("plex-old");
  });

  it("takes the first moment when one moment is picked before the backups are read", async () => {
    let answer: (value: Awaited<ReturnType<typeof api.getTimeline>>) => void = () => undefined;
    getTimeline.mockImplementationOnce(() => new Promise((resolve) => (answer = resolve)));
    open([item("plex")]);
    fireEvent.click(screen.getByRole("tab", { name: en["restoreMany.at"] }));
    answer({ ok: true, places: [LOCAL], rows: [row("plex-new", 0), row("plex-old", 2)] });
    await readyToStart(1);
    expect(lastCheckOf("plex")).toMatchObject({ snapshotId: "plex-new" });
  });

  it("is read from a remote place when no local place holds one", async () => {
    getTimeline.mockResolvedValue({ ok: true, places: [LOCAL, OFFSITE], rows: [] });
    getTimelinePlace.mockResolvedValue({
      ok: true,
      place: { ...OFFSITE, state: "read" },
      rows: [row("plex-far", 1, [], "offsite:b2")],
    });
    open([item("plex")]);
    await readyToStart(1);
    expect(lastCheckOf("plex")).toMatchObject({ snapshotId: "plex-far", source: "offsite:b2" });
  });
});

describe("the check before the start", () => {
  it("checks every entry as the window for one entry would", async () => {
    open([item("plex"), item("sonarr")]);
    await readyToStart(2);
    expect(lastCheckOf("plex")).toEqual({ kind: "container", name: "plex", snapshotId: "plex-new", source: "local", target: "beside" });
    expect(lastCheckOf("sonarr")).toMatchObject({ target: "beside" });
  });

  it("leaves out an entry whose check failed and says why", async () => {
    checkRestore.mockImplementation(async (req) =>
      req.name === "sonarr"
        ? { ok: true, ready: false, checks: [{ id: "space", status: "fail", reason: "short", need: 100, free: 50 }] }
        : { ok: true, ready: true, checks: [], plan: { shared: [] } as never }
    );
    open([item("plex"), item("sonarr")]);
    await readyToStart(1);
    expect(entry("sonarr").dataset.verdict).toBe("blocked");
    expect(entry("sonarr").textContent).toContain(en["restoreCheck.blockedBy"].split("{line}")[0]);
  });

  it("names the folders other containers share and an empty database", async () => {
    checkRestore.mockResolvedValue({
      ok: true,
      ready: true,
      checks: [],
      plan: { shared: [{ path: "/mnt/user/appdata/plex", containers: ["tautulli"] }] } as never,
    });
    open([item("plex", { dbDataCoverage: "none" })]);
    await readyToStart(1);
    expect(entry("plex").textContent).toContain("/mnt/user/appdata/plex");
    expect(entry("plex").textContent).toContain("tautulli");
    expect(entry("plex").textContent).toContain(en["restoreWindow.dbNone"]);
  });

  it("starts nothing while a check is still out", async () => {
    checkRestore.mockImplementation((req) =>
      req.name === "sonarr" ? new Promise(() => undefined) : Promise.resolve({ ok: true, ready: true, checks: [], plan: { shared: [] } as never })
    );
    open([item("plex"), item("sonarr")]);
    await waitFor(() => expect(startCount()).toBe(1));
    await tick(1000);
    expect(startButton()).toHaveProperty("disabled", true);
  });

  it("creates an entry gone from the server again only after a yes in its row", async () => {
    open([item("plex"), item("jellyfin", { installed: false })]);
    await readyToStart(1);
    expect(lastCheckOf("jellyfin")).toMatchObject({ kind: "container", name: "jellyfin" });
    expect(lastCheckOf("jellyfin")?.target).toBeUndefined();
    fireEvent.click(within(entry("jellyfin")).getByRole("switch", { name: en["restoreMany.recreateYes"] }));
    await readyToStart(2);
  });

  it("refuses an entry gone from the server whose folder a running container uses", async () => {
    checkRestore.mockImplementation(async (req) =>
      req.name === "plex"
        ? {
            ok: true,
            ready: false,
            checks: [{ id: "space", status: "fail", reason: "refused", detail: "the running container plex-new uses /mnt/user/appdata/plex" }],
          }
        : { ok: true, ready: true, checks: [], plan: { shared: [] } as never }
    );
    open([item("plex", { installed: false }), item("sonarr")]);
    await waitFor(() => expect(entry("plex").dataset.verdict).toBe("blocked"));
    fireEvent.click(within(entry("plex")).getByRole("switch", { name: en["restoreMany.recreateYes"] }));
    await readyToStart(1);
    expect(entry("plex").textContent).toContain("plex-new uses /mnt/user/appdata/plex");
  });
});

describe("the stop button", () => {
  it("ignores the second click of a double click on the start", async () => {
    open([item("plex"), item("sonarr")]);
    await readyToStart(2);
    fireEvent.click(startButton());
    fireEvent.click(screen.getByRole("button", { name: en["restoreMany.stop"] }));
    await tick(8000);
    await waitFor(() => expect(entry("sonarr").dataset.outcome).toBe("done"));
  });

  it("is free again for a restore without the runtime after a stop", async () => {
    endOf = { plex: "running" };
    open([item("plex"), item("sonarr")]);
    await readyToStart(2);
    fireEvent.click(startButton());
    await tick(1000);
    fireEvent.click(screen.getByRole("button", { name: en["restoreMany.stop"] }));
    settle("plex", "failed", "restore failed: the container used a GPU or runtime this host does not have: nvidia");
    await tick(3000);
    endOf = { plex: "running" };
    fireEvent.click(within(entry("plex")).getByRole("button", { name: en["restore.withoutRuntime"] }));
    await tick(1000);
    expect(screen.getByRole("button", { name: en["restoreMany.stop"] })).toHaveProperty("disabled", false);
  });
});

describe("for a screen reader", () => {
  it("names the bar and says each entry's state", async () => {
    endOf = { plex: "running" };
    open([item("plex"), item("sonarr")]);
    await readyToStart(2);
    fireEvent.click(startButton());
    await tick(1000);
    expect(screen.getByRole("progressbar", { name: `plex: ${en["restoreMany.running"]}` })).toBeTruthy();
    expect(screen.getByRole("status").textContent).toBe(
      en["restoreMany.announce"].replace("{n}", "1").replace("{total}", "2").replace("{name}", "plex").replace("{state}", en["restoreMany.running"])
    );
  });
});

describe("a flash backup", () => {
  it("is offered for download", async () => {
    open([item("flash", { kind: "flash", name: "flash" })]);
    await waitFor(() => expect(within(entry("lash")).getByRole("button", { name: en["tile.info.download"] })).toBeTruthy());
  });

  it("is not offered on a paired phone, which the server refuses it", async () => {
    vi.mocked(api.openedAsPairedPhone).mockReturnValue(true);
    open([item("flash", { kind: "flash", name: "flash" })]);
    await waitFor(() => expect(entry("lash").textContent).toContain(en["restoreMany.flashOwnLogin"]));
    expect(within(entry("lash")).queryByRole("button", { name: en["tile.info.download"] })).toBeNull();
  });
});
