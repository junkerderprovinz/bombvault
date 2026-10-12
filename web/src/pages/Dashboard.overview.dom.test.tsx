// @vitest-environment jsdom
// The overview as a page: which cards a browser sees first, what becomes of a
// layout somebody arranged, what the customize mode does, what the status
// head says, and how a phone lays the same cards out.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";

import { listStorageLocations } from "../lib/api";
import { en } from "../lib/i18n";
import { Dashboard } from "./Dashboard";
import {
  FakeEventSource,
  domain,
  failedCopy,
  finding,
  installMatchMedia,
  item,
  location,
  resetServer,
  run,
  server,
  setDesktop,
} from "./overview/overview.testsupport";
import { inApp } from "./overview/overviewApp.testsupport";

vi.mock("../lib/api", async (importOriginal) => {
  const { stagedApi } = await import("./overview/overview.testsupport");
  return stagedApi(await importOriginal<typeof import("../lib/api")>());
});

const LAYOUT_KEY = "bombvault.dashboardLayout";
const ALL = ["needs", "protection", "targets", "history", "anomalies", "coverage", "heatmap", "storage", "summary", "stats", "spike"];
const leftOut = {
  included: false,
  lastBackup: 0,
  lastRunStatus: "",
  effectiveSchedule: { kind: "none", spec: "", reason: "excluded" },
} as const;

beforeEach(() => {
  resetServer();
  installMatchMedia();
  vi.stubGlobal("EventSource", FakeEventSource);
  window.localStorage.clear();
  window.history.replaceState(null, "", "/dashboard");
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

async function renderPage() {
  render(inApp(<Dashboard />));
  await act(async () => {});
}

const store = (layout: unknown) => window.localStorage.setItem(LAYOUT_KEY, JSON.stringify(layout));
const stored = () => JSON.parse(window.localStorage.getItem(LAYOUT_KEY) ?? "null");
const cell = (id: string) => document.querySelector<HTMLElement>(`[data-grid-block='${id}']`)!;
const shownCards = () => [...document.querySelectorAll("[data-grid-block]")].map((el) => el.getAttribute("data-grid-block"));
const head = () => within(screen.getByTestId("status-head"));
const ratio = (ok: number, total: number) =>
  en["coverage.ratio"].replace("{protected}", String(ok)).replace("{total}", String(total));

async function customize() {
  fireEvent.click(screen.getByRole("button", { name: en["dashboard.customize"] }));
  await act(async () => {});
}

describe("the cards a browser sees first", () => {
  it("are the protection status, the storage locations and the history while nothing needs the person", async () => {
    server.domains = [domain()];
    server.items = [item()];
    await renderPage();

    expect(shownCards()).toEqual(["protection", "targets", "history"]);
    expect(screen.getByTestId("status-head")).toBeTruthy();
  });

  it("start with what needs the person once something does", async () => {
    server.items = [item(), item({ key: "gitea", name: "gitea", ...leftOut })];
    await renderPage();

    expect(shownCards()).toEqual(["needs", "protection", "targets", "history"]);
  });

  it("write nothing to the browser's storage", async () => {
    await renderPage();

    expect(stored()).toBeNull();
  });

  it("put protection and the storage locations side by side and the others across", async () => {
    server.items = [item(), item({ key: "gitea", ...leftOut })];
    await renderPage();

    expect(ALL.slice(0, 4).map((id) => cell(id).getAttribute("data-span"))).toEqual(["6", "3", "3", "6"]);
    expect(cell("protection").style.gridColumn).toBe("span 3");
  });
});

describe("a layout from before the six-column grid", () => {
  const old = {
    order: ["protection", "summary", "runHistory", "activityLog", "storage", "coverage", "anomalies", "ransomware", "lastBackups", "heatmap", "stats", "spike"],
    hidden: ["stats", "spike", "heatmap"],
    widths: { protection: "half", storage: "half" },
  };

  it("keeps its order and its hidden cards, with the new cards where they belong", async () => {
    store(old);
    server.items = [item(), item({ key: "gitea", ...leftOut })];
    await renderPage();

    expect(shownCards()).toEqual(["needs", "protection", "targets", "summary", "history", "storage", "coverage", "anomalies"]);
  });

  it("turns half and full into three and six columns", async () => {
    store({ ...old, order: ["protection", "storage", "summary"], widths: { protection: "half", storage: "half", summary: "full" } });
    await renderPage();

    expect(cell("protection").getAttribute("data-span")).toBe("3");
    expect(cell("summary").getAttribute("data-span")).toBe("6");
  });
});

describe("customize mode", () => {
  it("offers every hidden card as a chip and shows one on request", async () => {
    await renderPage();
    await customize();

    const tray = screen.getByTestId("hidden-cards");
    const chips = [...tray.querySelectorAll("[data-hidden-card]")].map((el) => el.getAttribute("data-hidden-card"));
    expect(chips).toEqual(["anomalies", "coverage", "heatmap", "storage", "summary", "stats", "spike"]);

    fireEvent.click(within(tray.querySelector<HTMLElement>("[data-hidden-card='anomalies']")!).getByRole("button"));

    expect(shownCards()).toContain("anomalies");
    expect(tray.querySelector("[data-hidden-card='anomalies']")).toBeNull();
    expect(stored()).toMatchObject({ v: 2 });
    expect(stored().hidden).not.toContain("anomalies");
  });

  it("stands the card of what needs the person in the grid even while it is empty", async () => {
    await renderPage();
    expect(shownCards()).not.toContain("needs");

    await customize();

    expect(within(cell("needs")).getByText(en["overview.needs.none"])).toBeTruthy();
  });

  it("explains arranging in a bubble rather than a line of text", async () => {
    await renderPage();
    await customize();

    expect(screen.queryByText(en["overview.customizeHint"])).toBeNull();
    expect(screen.getByLabelText(en["overview.customizeHint"])).toBeTruthy();
  });

  it("hides a card into the tray", async () => {
    await renderPage();
    await customize();

    fireEvent.click(within(cell("history")).getByRole("button", { name: en["dashboard.hideCard"] }));

    expect(shownCards()).not.toContain("history");
    expect(screen.getByTestId("hidden-cards").querySelector("[data-hidden-card='history']")).toBeTruthy();
  });

  it("moves a card past its neighbour with the buttons of its bar", async () => {
    await renderPage();
    await customize();

    fireEvent.click(within(cell("protection")).getByRole("button", { name: en["dashboard.moveDown"] }));

    expect(shownCards()).toEqual(["needs", "targets", "protection", "history"]);
  });

  it("steps a card's width with the button of its bar and keeps every row filled", async () => {
    await renderPage();
    await customize();
    const half = en["overview.widthStep"].replace("{w}", "½");

    fireEvent.click(within(cell("protection")).getByRole("button", { name: half }));

    expect(stored().widths).toEqual({ protection: 4 });
    // Two thirds and a half do not share a row, so each fills its own.
    expect(cell("protection").getAttribute("data-span")).toBe("6");
    expect(cell("targets").getAttribute("data-span")).toBe("6");
  });

  it("goes back to the default on request", async () => {
    store({ v: 2, order: [...ALL].reverse(), hidden: [], widths: { history: 2 }, heights: { history: 300 } });
    await renderPage();
    await customize();

    fireEvent.click(screen.getByRole("button", { name: en["dashboard.resetLayout"] }));

    expect(shownCards()).toEqual(["needs", "protection", "targets", "history"]);
    expect(stored()).toMatchObject({ order: ALL, widths: {}, heights: {} });
  });

  it("has one filled action, the way out, and no backup button", async () => {
    await renderPage();
    await customize();

    const actions = within(screen.getByTestId("page-actions"));
    expect(actions.getAllByRole("button")).toHaveLength(1);
    expect(actions.getByRole("button", { name: en["dashboard.customizeDone"] }).className).toContain("bg-accent");
  });
});

describe("resizing a card by its handle", () => {
  // jsdom lays nothing out: the grid is 1160px wide with a 40px gap, so a
  // column with its gap is 200px, and every card starts at the origin.
  function layOut() {
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (this: HTMLElement) {
      const width = this.dataset.testid === "overview-grid" ? 1160 : 560;
      return { left: 0, top: 0, right: width, bottom: 300, width, height: 300, x: 0, y: 0, toJSON: () => ({}) };
    });
    vi.spyOn(window, "getComputedStyle").mockReturnValue({ columnGap: "40px", direction: "ltr" } as CSSStyleDeclaration);
  }
  const pointer = (type: string, x: number, y: number) => new MouseEvent(type, { clientX: x, clientY: y, bubbles: true });
  const handle = (id: string) => within(cell(id)).getByTestId("resize-handle");

  it("snaps the width to the columns and the height to steps of 20px", async () => {
    await renderPage();
    await customize();
    layOut();

    act(() => void handle("history").dispatchEvent(pointer("pointerdown", 560, 300)));
    act(() => void document.dispatchEvent(pointer("pointermove", 790, 333)));
    // While the pointer is down the card already shows the size under it.
    expect(cell("history").style.height).toBe("340px");
    act(() => void document.dispatchEvent(pointer("pointerup", 790, 333)));

    expect(stored().widths).toEqual({ history: 4 });
    expect(stored().heights).toEqual({ history: 340 });
    expect(cell("history").hasAttribute("data-sized")).toBe(true);
  });

  it("never makes a card lower than 140px", async () => {
    await renderPage();
    await customize();
    layOut();

    act(() => void handle("history").dispatchEvent(pointer("pointerdown", 560, 300)));
    act(() => void document.dispatchEvent(pointer("pointermove", 400, 20)));
    act(() => void document.dispatchEvent(pointer("pointerup", 400, 20)));

    expect(stored().heights).toEqual({ history: 140 });
  });

  it("leaves the card alone on a press without a drag", async () => {
    await renderPage();
    await customize();
    layOut();

    act(() => void handle("history").dispatchEvent(pointer("pointerdown", 560, 300)));
    act(() => void document.dispatchEvent(pointer("pointerup", 560, 300)));

    expect(stored()).toBeNull();
  });

  it("gives the natural height back on a double click", async () => {
    store({ v: 2, order: ALL, hidden: ALL.slice(4), widths: {}, heights: { history: 300 } });
    await renderPage();
    expect(cell("history").style.height).toBe("300px");
    await customize();

    fireEvent.doubleClick(handle("history"));

    expect(stored().heights).toEqual({});
    expect(cell("history").style.height).toBe("");
    expect(cell("history").hasAttribute("data-sized")).toBe(false);
  });
});

describe("the status head", () => {
  it("says it is checking until the entries and the sections have answered", async () => {
    server.items = "pending";
    await renderPage();

    expect(head().getByText(en["dashboard.checking"])).toBeTruthy();
    expect(head().queryByText(en["overview.head.allGood"])).toBeNull();
  });

  it("reports a load that failed instead of a state", async () => {
    server.items = "refused";
    server.domains = "refused";
    await renderPage();

    expect(head().getByText(en["dashboard.statusLoadFailed"])).toBeTruthy();
    expect(head().queryByText(en["backups.status.waiting"])).toBeNull();
  });

  it("says everything is backed up when every entry is and every check is green", async () => {
    server.domains = [domain()];
    server.items = [item()];
    await renderPage();

    expect(head().getByText(en["overview.head.allGood"])).toBeTruthy();
    expect(head().getByText(new RegExp(ratio(1, 1)))).toBeTruthy();
  });

  it("does not round a missing off-site copy up to all good", async () => {
    server.domains = [domain({ offsiteConfigured: false })];
    server.items = [item()];
    await renderPage();

    expect(head().getByText(en["overview.head.notes"])).toBeTruthy();
  });

  it("does not round an open note up to all good", async () => {
    server.domains = [domain()];
    server.items = [item()];
    server.detection = true;
    server.findings = [finding({ id: "a1", severity: "info" })];
    await renderPage();

    expect(head().getByText(en["overview.head.notes"])).toBeTruthy();
  });

  it("counts what needs the person: a failure, an entry nothing backs up, an anomaly and the recovery kit", async () => {
    server.domains = [domain()];
    server.items = [
      item({ runs: [{ id: "r1", kind: "backup", status: "failed", startedAt: 1 }], lastRunStatus: "failed" }),
      item({ key: "gitea", name: "gitea", ...leftOut }),
    ];
    server.runs = [run({ id: "r1", status: "failed", error: "restic exited 1" })];
    server.detection = true;
    server.findings = [finding({ id: "a1", name: "immich" }), finding({ id: "a2", severity: "info" })];
    server.kitDue = true;
    await renderPage();

    expect(head().getByText("4 things need you")).toBeTruthy();
    expect(within(cell("needs")).getAllByRole("listitem")).toHaveLength(4);
  });

  it("puts a backup key that no longer opens first among what needs the person", async () => {
    server.domains = [domain()];
    server.items = [item()];
    server.kitDue = true;
    server.keyLocked = true;
    await renderPage();

    expect(head().getByText("2 things need you")).toBeTruthy();
    const rows = within(cell("needs")).getAllByRole("listitem");
    expect(rows.map((row) => row.getAttribute("data-testid"))).toEqual(["need-key-locked", "need-recovery-kit"]);
  });

  it("takes the recovery kit off the list once it is stored", async () => {
    server.domains = [domain()];
    server.items = [item()];
    server.kitDue = true;
    await renderPage();
    expect(head().getByText("1 thing needs you")).toBeTruthy();

    fireEvent.click(within(cell("needs")).getByRole("button", { name: en["recovery.stored"] }));
    await act(async () => {});

    expect(head().getByText(en["overview.head.allGood"])).toBeTruthy();
    expect(shownCards()).not.toContain("needs");
  });

  it("keeps findings out of the count while detection is off", async () => {
    server.domains = [domain()];
    server.items = [item()];
    server.findings = [finding({ id: "a1" })];
    await renderPage();

    expect(head().getByText(en["overview.head.allGood"])).toBeTruthy();
  });

  it("counts a failed off-site copy until somebody marks it as seen", async () => {
    server.domains = [domain()];
    server.items = [item()];
    server.locations = [location({ id: "destination:box", kind: "offsite", name: "Storage Box", sections: [failedCopy()] })];
    await renderPage();

    expect(head().getByText("1 thing needs you")).toBeTruthy();
    const row = within(within(cell("needs")).getByTestId("need-failed-copy"));
    expect(row.getByText(/^Storage Box · Containers · since /)).toBeTruthy();

    fireEvent.click(row.getByRole("button", { name: en["nav.more"] }));
    fireEvent.click(screen.getByRole("menuitem", { name: en["errorPanel.resolve"] }));
    await act(async () => {});

    expect(head().getByText(en["overview.head.allGood"])).toBeTruthy();
    expect(shownCards()).not.toContain("needs");
  });

  it("reads the storage locations once for the count and for their card", async () => {
    vi.mocked(listStorageLocations).mockClear();
    server.locations = [location({ id: "destination:box", object: "destination", kind: "offsite", name: "Storage Box", sections: [failedCopy()] })];
    await renderPage();

    expect(within(cell("targets")).getByRole("link", { name: "Storage Box" })).toBeTruthy();
    expect(within(cell("needs")).getByTestId("need-failed-copy")).toBeTruthy();
    expect(listStorageLocations).toHaveBeenCalledTimes(1);
  });

  it("stands at the station the server names", async () => {
    server.domains = [domain()];
    server.items = [item()];
    await renderPage();
    const stream = FakeEventSource.last!;
    const station = () => head().getByRole("img").getAttribute("aria-label");
    const frame = (over: Record<string, unknown>) => ({ key: "container:plex", phase: "backup", percent: 0, active: true, ...over });

    act(() => stream.send(frame({ step: "stop" })));
    expect(station()).toBe(en["overview.station.stop"]);

    act(() => stream.send(frame({ step: "read" })));
    expect(station()).toBe(en["overview.station.read"]);

    act(() => stream.send(frame({ step: "read", percent: 100, committed: true })));
    expect(station()).toBe(en["overview.station.read"]);

    act(() => stream.send(frame({ step: "start", percent: 100, committed: true })));
    expect(station()).toBe(en["overview.station.start"]);
  });

  it("counts through a pass over everything and collects a mark for each entry it finished", async () => {
    server.domains = [domain()];
    server.items = [item()];
    await renderPage();
    const stream = FakeEventSource.last!;
    const pass = (over: Record<string, unknown>) => ({ key: "everything", phase: "backup", active: true, total: 4, unit: "items", ...over });
    const parent = run({ id: "all", domain: "everything", targetId: "everything", status: "running", finishedAt: null });
    const ring = () => head().getByRole("button", { name: en["overview.head.viewRun"] }).textContent;
    const marks = () => [...screen.getByTestId("pass-marks").children].map((mark) => mark.getAttribute("data-tone"));

    act(() => stream.send(pass({ percent: 0 })));
    expect(head().getByText("Backing up now: 0 of 4")).toBeTruthy();
    expect(head().getByText(en["overview.head.preparing"])).toBeTruthy();
    expect(ring()).toContain("0 %");
    expect(marks()).toEqual([]);

    act(() => stream.send({ key: "container:plex", phase: "backup", percent: 50, active: true, step: "read" }));
    expect(head().getByText("plex").parentElement!.textContent).toBe("Backing up plex");
    expect(head().getByRole("img").getAttribute("aria-label")).toBe(en["overview.station.read"]);
    expect(ring()).toContain("13 %");

    server.runs = [run({ id: "c1", groupId: "all" }), parent];
    act(() => stream.send({ key: "container:plex", phase: "backup", percent: 100, active: false }));
    act(() => stream.send(pass({ percent: 25, done: 1 })));
    expect(head().getByText("Backing up now: 1 of 4")).toBeTruthy();
    expect(ring()).toContain("25 %");
    await waitFor(() => expect(marks()).toEqual(["ok"]));

    server.runs = [run({ id: "c2", groupId: "all", status: "failed", target: "gitea" }), ...server.runs];
    act(() => stream.send(pass({ percent: 50, done: 2 })));
    await waitFor(() => expect(marks()).toEqual(["ok", "fail"]));
    expect(screen.getByTestId("pass-marks").lastElementChild!.getAttribute("title")).toBe("gitea");
  });

  it("returns to the state sentence when the pass has ended", async () => {
    server.domains = [domain()];
    server.items = [item()];
    await renderPage();
    const stream = FakeEventSource.last!;

    act(() => stream.send({ key: "everything", phase: "backup", percent: 50, active: true, done: 2, total: 4, unit: "items" }));
    expect(head().getByText("Backing up now: 2 of 4")).toBeTruthy();
    expect(within(screen.getByTestId("page-actions")).getByRole("button", { name: en["overview.head.backingUp"] })).toBeTruthy();

    act(() => stream.send({ key: "everything", phase: "backup", percent: 100, active: false, done: 4, total: 4, unit: "items" }));

    expect(head().getByText(en["overview.head.allGood"])).toBeTruthy();
    expect(screen.queryByTestId("pass-marks")).toBeNull();
  });

  it("places a backup that names no step by its progress", async () => {
    server.domains = [domain()];
    server.items = [item()];
    await renderPage();
    const stream = FakeEventSource.last!;
    const station = () => head().getByRole("img").getAttribute("aria-label");

    act(() => stream.send({ key: "container:plex", phase: "backup", percent: 0, active: true }));
    expect(head().getByText(en["overview.head.backingUp"])).toBeTruthy();
    expect(head().getByText("plex")).toBeTruthy();
    expect(station()).toBe(en["overview.station.stop"]);

    act(() => stream.send({ key: "container:plex", phase: "backup", percent: 42, active: true }));
    expect(station()).toBe(en["overview.station.read"]);
    expect(head().getByRole("button", { name: en["overview.head.viewRun"] }).textContent).toContain("42");

    act(() => stream.send({ key: "container:plex", phase: "backup", percent: 100, active: true, committed: true }));
    expect(station()).toBe(en["overview.station.start"]);
  });

  it("brings the entries up to date when the run has ended", async () => {
    server.domains = [domain()];
    server.items = [item({ lastBackup: 0, lastRunStatus: "" })];
    await renderPage();
    expect(head().getByText(new RegExp(ratio(0, 1)))).toBeTruthy();
    const stream = FakeEventSource.last!;

    act(() => stream.send({ key: "container:plex", phase: "backup", percent: 40, active: true }));
    server.runs = [run({ id: "r1" })];
    server.items = [item()];
    act(() => stream.send({ key: "container:plex", phase: "backup", percent: 100, active: false }));

    await waitFor(() => expect(head().getByText(new RegExp(ratio(1, 1)))).toBeTruthy(), { timeout: 3000 });
  });

  it("follows an off-site copy at the last station", async () => {
    server.domains = [domain()];
    server.items = [item()];
    await renderPage();

    act(() =>
      FakeEventSource.last!.send({
        key: "offsite:containers",
        phase: "replicate",
        percent: 50,
        active: true,
        snapshotIndex: 2,
        snapshotTotal: 4,
      })
    );

    expect(head().getByText(en["overview.head.copying"])).toBeTruthy();
    expect(head().getByRole("img").getAttribute("aria-label")).toBe(en["common.copy"]);
    expect(head().getByText(/Backup 2 of 4/)).toBeTruthy();
    // The section's own dot shows the copy as well.
    expect(within(cell("protection")).getByRole("img", { name: en["overview.check.offsiteRunning"] })).toBeTruthy();
  });
});

describe("an entry the person said not to back up", () => {
  const scratch = item({ key: "scratch", name: "scratch", ...leftOut });

  it("is in no count, and the card of what is not backed up names it as excluded", async () => {
    store({ v: 2, order: ALL, hidden: [], widths: {}, heights: {} });
    server.domains = [domain()];
    server.items = [item(), { ...scratch, backupDeclined: true }];
    await renderPage();

    expect(head().getByText(en["overview.head.allGood"])).toBeTruthy();
    expect(head().getByText(new RegExp(ratio(1, 1)))).toBeTruthy();
    expect(shownCards()).not.toContain("needs");
    expect(within(cell("protection")).getByText(new RegExp(ratio(1, 1)))).toBeTruthy();
    const excluded = within(cell("coverage")).getByTestId("coverage-declined");
    expect(excluded.textContent).toBe(`scratch${en["coverage.reason.declined"]}`);
    expect(within(cell("coverage")).queryByText(en["coverage.allProtected"])).toBeNull();
  });

  it("leaves the list when the person says so and comes back with Undo", async () => {
    server.domains = [domain()];
    server.items = [item(), scratch];
    await renderPage();
    expect(head().getByText("1 thing needs you")).toBeTruthy();

    fireEvent.click(within(cell("needs")).getByRole("button", { name: en["nav.more"] }));
    fireEvent.click(screen.getByRole("menuitem", { name: en["backups.decline"] }));
    await act(async () => {});

    expect(head().getByText(en["overview.head.allGood"])).toBeTruthy();
    expect(shownCards()).not.toContain("needs");

    fireEvent.click(screen.getByRole("button", { name: en["takeover.undo"] }));
    await act(async () => {});

    expect(head().getByText("1 thing needs you")).toBeTruthy();
    expect(within(cell("needs")).getByRole("link", { name: "scratch" })).toBeTruthy();
  });
});

describe("the card of what is not backed up", () => {
  it("says the entries could not be read instead of loading forever", async () => {
    store({ v: 2, order: ALL, hidden: [], widths: {}, heights: {} });
    server.items = "refused";
    await renderPage();

    expect(within(cell("coverage")).getByText(en["coverage.loadFailed"])).toBeTruthy();
    expect(within(cell("coverage")).queryByText(en["folder.loading"])).toBeNull();
  });
});

describe("one count on every card", () => {
  it("is the same in the head, on the protection card, on the coverage card and in the statistics", async () => {
    store({ v: 2, order: ALL, hidden: [], widths: {}, heights: {} });
    server.domains = [domain()];
    server.items = [
      item(),
      item({ key: "immich", name: "immich" }),
      item({ key: "gitea", name: "gitea", ...leftOut }),
      item({ key: "old", name: "old", installed: false }),
      item({ key: "bombvault", name: "bombvault", self: true, ...leftOut }),
    ];
    await renderPage();

    const count = new RegExp(ratio(2, 3));
    expect(head().getByText(count)).toBeTruthy();
    expect(within(cell("protection")).getByText(count)).toBeTruthy();
    expect(within(cell("coverage")).getByText(count)).toBeTruthy();
    const containers = within(cell("stats")).getByText(en["dashboard.statContainers"]).parentElement!;
    expect(containers.textContent).toContain("3");
    const gone = within(cell("stats")).getByText(en["containers.notInstalled"]).parentElement!;
    expect(gone.textContent).toContain("1");
  });
});

describe("the overview on a phone", () => {
  beforeEach(() => setDesktop(false));

  it("stands the cards in one order for everybody, with the next run and the repositories under what needs the person", async () => {
    store({ v: 2, order: [...ALL].reverse(), hidden: ["spike", "stats", "summary", "storage", "heatmap"], widths: {}, heights: {} });
    server.items = [item(), item({ key: "gitea", ...leftOut })];
    await renderPage();

    expect(shownCards()).toEqual(["needs", "nextRun", "storageGlance", "protection", "targets", "history", "anomalies", "coverage"]);
  });

  it("ignores the sizes somebody set on a desktop", async () => {
    store({ v: 2, order: ALL, hidden: ALL.slice(4), widths: { history: 2 }, heights: { history: 300 } });
    await renderPage();

    expect(cell("history").style.gridColumn).toBe("");
    expect(cell("history").style.height).toBe("");
    expect(cell("history").hasAttribute("data-sized")).toBe(false);
  });

  it("has no customize mode", async () => {
    await renderPage();

    expect(screen.queryByRole("button", { name: en["dashboard.customize"] })).toBeNull();
    expect(screen.getByRole("button", { name: en["backups.backUpEverything"] })).toBeTruthy();
  });
});

describe("the colour engine", () => {
  it("gives every card a palette position, on a desktop and on a phone", async () => {
    server.items = [item(), item({ key: "gitea", ...leftOut })];
    server.locations = [location()];
    for (const desktop of [true, false]) {
      setDesktop(desktop);
      await renderPage();
      for (const id of shownCards()) {
        const card = cell(id!).querySelector<HTMLElement>(".glim-notch-card")!;
        expect(card.className, id!).toContain("glim-hue");
        expect(card.style.getPropertyValue("--item-hue"), id!).not.toBe("");
      }
      cleanup();
    }
  });
});
