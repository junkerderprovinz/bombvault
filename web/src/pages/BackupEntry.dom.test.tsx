// @vitest-environment jsdom
/**
 * The page of one backup entry.
 *
 * Six kinds of entry share the page, each read through the list of its kind.
 * The head has to say the one thing that matters about the entry, and every
 * action has to reach the call of the kind it belongs to. Only EventSource is
 * faked for the progress stream, because jsdom has none.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation, useNavigate } from "react-router-dom";

import { I18nProvider, en } from "../lib/i18n";
import { ENTRY_ROUTE } from "../lib/backupEntry";
import { AnomalyProvider } from "../lib/useAnomalies";
import { DESKTOP_QUERY } from "../lib/useMediaQuery";
import { formatRecent } from "../lib/reltime";
import {
  IMMICH,
  PHOTOS,
  PIHOLE,
  SELF,
  SET_ID,
  TANK,
  WIN11,
  ZFS_ID,
  backupRun,
  containerView,
  serveEntries,
  watched,
} from "./backupEntry.testsupport";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listItems: vi.fn(),
    listContainers: vi.fn(),
    listVMs: vi.fn(),
    listFileSets: vi.fn(),
    listZFSDatasets: vi.fn(),
    listItemRuns: vi.fn(),
    listRuns: vi.fn(),
    getSettings: vi.fn(),
    getAnomalies: vi.fn(),
    getAnomalySummary: vi.fn(),
    getAnomalyItems: vi.fn(),
    listSnapshots: vi.fn(),
    setInclude: vi.fn(),
    patchFileSet: vi.fn(),
    backupNow: vi.fn(),
    backupVMNow: vi.fn(),
    backupFlashNow: vi.fn(),
    deleteFileSet: vi.fn(),
    deleteBackups: vi.fn(),
    takeOverContainer: vi.fn(),
    getZFSReplica: vi.fn(),
  };
});

const pushed: { message: string; severity?: string }[] = [];
vi.mock("../lib/toast", () => ({
  useToast: () => ({ push: (message: string, severity?: string) => pushed.push({ message, severity }) }),
}));

const api = await import("../lib/api");
const mocks = {
  listItems: vi.mocked(api.listItems),
  listContainers: vi.mocked(api.listContainers),
  listVMs: vi.mocked(api.listVMs),
  listFileSets: vi.mocked(api.listFileSets),
  listZFSDatasets: vi.mocked(api.listZFSDatasets),
  listItemRuns: vi.mocked(api.listItemRuns),
  getSettings: vi.mocked(api.getSettings),
};
const listRuns = vi.mocked(api.listRuns);
const getAnomalies = vi.mocked(api.getAnomalies);
const getAnomalySummary = vi.mocked(api.getAnomalySummary);
const getAnomalyItems = vi.mocked(api.getAnomalyItems);
const listSnapshots = vi.mocked(api.listSnapshots);
const setInclude = vi.mocked(api.setInclude);
const patchFileSet = vi.mocked(api.patchFileSet);
const backupNow = vi.mocked(api.backupNow);
const backupVMNow = vi.mocked(api.backupVMNow);
const backupFlashNow = vi.mocked(api.backupFlashNow);
const deleteFileSet = vi.mocked(api.deleteFileSet);
const deleteBackups = vi.mocked(api.deleteBackups);

const { BackupEntryPage } = await import("./BackupEntry");

// useIsDesktop keeps the first MediaQueryList it gets, so the stub answers
// from a variable instead of being swapped per test.
let desktop = true;
window.matchMedia = ((query: string) => ({
  get matches() {
    return query === DESKTOP_QUERY ? desktop : false;
  },
  media: query,
  onchange: null,
  addListener: () => {},
  removeListener: () => {},
  addEventListener: () => {},
  removeEventListener: () => {},
  dispatchEvent: () => false,
})) as unknown as typeof window.matchMedia;

const streams: FakeEventSource[] = [];

class FakeEventSource {
  onmessage: ((ev: MessageEvent<string>) => void) | null = null;
  constructor(_url: string) {
    streams.push(this);
  }
  close(): void {}
}

function progress(frame: Record<string, unknown>) {
  act(() => {
    streams.at(-1)?.onmessage?.({ data: JSON.stringify(frame) } as MessageEvent<string>);
  });
}

function Elsewhere() {
  const { pathname, search } = useLocation();
  return <p>{`at ${pathname}${search}`}</p>;
}

function Address() {
  const { pathname, search } = useLocation();
  const navigate = useNavigate();
  return (
    <>
      <p data-testid="address">{`${pathname}${search}`}</p>
      <button type="button" onClick={() => navigate(-1)}>
        back
      </button>
    </>
  );
}

async function renderPage(path: string) {
  await act(async () => {
    render(
      <I18nProvider>
        <MemoryRouter initialEntries={["/backups", path]}>
          <AnomalyProvider>
            <Routes>
              <Route
                path={ENTRY_ROUTE}
                element={
                  <>
                    <BackupEntryPage />
                    <Address />
                  </>
                }
              />
              <Route path="*" element={<Elsewhere />} />
            </Routes>
          </AnomalyProvider>
        </MemoryRouter>
      </I18nProvider>
    );
  });
}

async function press(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

const corner = () => within(screen.getByTestId("page-actions"));
const button = (name: string) => screen.getByRole("button", { name });
const address = () => screen.getByTestId("address").textContent;

beforeEach(() => {
  pushed.length = 0;
  streams.length = 0;
  desktop = true;
  localStorage.clear();
  vi.stubGlobal("EventSource", FakeEventSource);
  // Whatever a card reads beyond the staged calls answers "not available".
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => ({ ok: true, json: async () => ({ ok: false }) }))
  );
  serveEntries(mocks);
  listRuns.mockResolvedValue({ ok: true, runs: [] });
  getAnomalies.mockResolvedValue({ ok: true, anomalies: [], nextCursor: "" });
  getAnomalySummary.mockResolvedValue({ ok: false });
  getAnomalyItems.mockResolvedValue({ ok: true, items: [] });
  listSnapshots.mockResolvedValue({ ok: true, snapshots: [] });
  vi.mocked(api.getZFSReplica).mockRejectedValue(new Error("no replica"));
  for (const call of [setInclude, patchFileSet, deleteFileSet, deleteBackups]) call.mockResolvedValue({ ok: true });
  for (const call of [backupNow, backupVMNow, backupFlashNow]) call.mockResolvedValue({ ok: true, started: true });
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe("reading the entry", () => {
  it("names the entry, says when it was last backed up and that it is protected", async () => {
    await renderPage("/backups/container/immich");
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe("immich");
    expect(
      screen.getByText(en["entry.reason.lastBackup"].replace("{when}", formatRecent(IMMICH.lastBackup, "en")), {
        exact: false,
      })
    ).toBeTruthy();
    expect(screen.getByText("192.168.20.40")).toBeTruthy();
    expect(screen.getByText(en["ransomware.protGreen"])).toBeTruthy();
  });

  it.each([
    ["a VM", "/backups/vm/win11", WIN11.name],
    ["a folder set", `/backups/files/${SET_ID}`, PHOTOS.name],
    ["a ZFS item", `/backups/zfs/${ZFS_ID}`, TANK.name],
    ["the flash drive", "/backups/flash/flash", en["backups.name.flash"]],
    ["the self-backup", "/backups/config/config", en["backups.name.config"]],
  ])("opens %s under its own name", async (_, path, name) => {
    await renderPage(path);
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe(name);
    expect(corner().getByRole("button", { name: en["containers.backupNow"] })).toBeTruthy();
  });

  it("says an entry is gone when its list no longer holds it", async () => {
    await renderPage("/backups/container/vanished");
    expect(screen.getByText(en["entry.missing"])).toBeTruthy();
    expect(screen.getByRole("link", { name: en["backups.title"] }).getAttribute("href")).toBe("/backups");
  });

  it("does not ask the server about a kind it does not know", async () => {
    await renderPage("/backups/tapes/1");
    expect(screen.getByText(en["entry.missing"])).toBeTruthy();
    expect(mocks.listItems).not.toHaveBeenCalled();
  });

  it("offers to try again when the entry could not be read", async () => {
    mocks.listItems.mockRejectedValueOnce(new Error("down"));
    await renderPage("/backups/container/immich");
    expect(screen.getByText(en["backups.loadFailed"])).toBeTruthy();
    await press(button(en["anomaly.retry"]));
    await waitFor(() => expect(screen.getByRole("heading", { level: 1 }).textContent).toBe("immich"));
  });
});

describe("the state of the entry", () => {
  it("puts the reason of a failed run under the head", async () => {
    serveEntries(mocks, {
      items: [{ ...IMMICH, lastRunStatus: "failed" }],
      runs: [backupRun("r-failed", 0, { status: "failed", error: "restic exited with 1" }), backupRun("r-ok", 1)],
    });
    await renderPage("/backups/container/immich");
    expect(screen.getByText(/restic exited with 1/)).toBeTruthy();
    expect(screen.getByText(en["entry.retryNote"], { exact: false })).toBeTruthy();
    expect(
      screen.getByText(en["entry.reason.lastFailed"].replace("{when}", formatRecent(IMMICH.lastBackup, "en")), {
        exact: false,
      })
    ).toBeTruthy();
  });

  it("marks an entry that is out of the schedule and has no backup as not protected", async () => {
    await renderPage("/backups/container/gitea");
    expect(screen.getByText(en["backups.status.notProtected"])).toBeTruthy();
    expect(screen.getByText(en["entry.reason.pausedNever"], { exact: false })).toBeTruthy();
    expect(screen.getAllByText(en["schedule.paused"]).length).toBeGreaterThan(0);
  });

  it("links the open findings of the entry", async () => {
    getAnomalySummary.mockResolvedValue({ ok: true, summary: { enabled: true, generation: 1 } } as never);
    getAnomalyItems.mockResolvedValue({ ok: true, items: [watched({ open: { critical: 0, warning: 2, info: 0 } })] });
    await renderPage("/backups/container/immich");
    const link = await screen.findByRole("link", { name: /2 anomalies/ });
    expect(link.getAttribute("href")).toBe("/anomalies?scope=item:tg-immich");
  });

  it("shows a running backup in place of the state, with the way to stop it", async () => {
    await renderPage("/backups/container/immich");
    progress({ key: "container:immich", phase: "backup", percent: 42, active: true });
    expect(screen.getByText(en["backups.status.running"].replace("{pct}", "42"))).toBeTruthy();
    expect(screen.queryByText(en["ransomware.protGreen"])).toBeNull();
    expect(button(en["backup.cancel"])).toBeTruthy();
  });

  it("reads the entry again when its backup ends", async () => {
    await renderPage("/backups/container/immich");
    progress({ key: "container:immich", phase: "backup", percent: 42, active: true });
    const before = mocks.listItems.mock.calls.length;
    progress({ key: "container:immich", phase: "backup", percent: 100, active: false, finished: true });
    await waitFor(() => expect(mocks.listItems.mock.calls.length).toBeGreaterThan(before));
  });

  it("says of BombVault's own container why it has no backup and offers nothing else", async () => {
    await renderPage("/backups/container/bombvault");
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe(SELF.name);
    expect(screen.getByText(en["backups.selfNote"])).toBeTruthy();
    expect(screen.queryByRole("tablist")).toBeNull();
    expect(screen.queryByTestId("page-actions")).toBeNull();
  });
});

describe("the tabs", () => {
  it("open on Backups and put the chosen tab into the address", async () => {
    await renderPage("/backups/container/immich");
    const tabs = within(screen.getByRole("tablist"));
    expect(tabs.getByRole("tab", { name: en["backups.title"] }).getAttribute("aria-selected")).toBe("true");
    await press(tabs.getByRole("tab", { name: en["nav.settings"] }));
    expect(address()).toBe("/backups/container/immich?tab=settings");
    expect(tabs.getByRole("tab", { name: en["nav.settings"] }).getAttribute("aria-selected")).toBe("true");
  });

  it("leave the page on Back however many were looked at", async () => {
    await renderPage("/backups/container/immich");
    const tabs = within(screen.getByRole("tablist"));
    await press(tabs.getByRole("tab", { name: en["nav.settings"] }));
    await press(tabs.getByRole("tab", { name: en["entry.tab.verify"] }));
    await press(button("back"));
    expect(screen.getByText("at /backups")).toBeTruthy();
  });

  it("open on the tab the address names", async () => {
    await renderPage("/backups/container/immich?tab=verify");
    expect(
      within(screen.getByRole("tablist")).getByRole("tab", { name: en["entry.tab.verify"] }).getAttribute("aria-selected")
    ).toBe("true");
  });
});

describe("the actions of the corner", () => {
  it("warns once that a container stops, then starts its backup", async () => {
    await renderPage("/backups/container/immich");
    await press(corner().getByRole("button", { name: en["containers.backupNow"] }));
    expect(backupNow).not.toHaveBeenCalled();
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText(en["containers.stopWarning"])).toBeTruthy();
    await press(within(dialog).getByRole("button", { name: en["containers.backupNow"] }));
    await waitFor(() => expect(backupNow).toHaveBeenCalledWith("immich"));
  });

  it.each([
    ["a VM", "/backups/vm/win11", backupVMNow, ["win11"]],
    ["the flash drive", "/backups/flash/flash", backupFlashNow, []],
  ] as const)("starts the backup of %s without a question", async (_, path, call, args) => {
    await renderPage(path);
    await press(corner().getByRole("button", { name: en["containers.backupNow"] }));
    await waitFor(() => expect(call).toHaveBeenCalledWith(...args));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("pauses the schedule of the entry and reads it again", async () => {
    await renderPage("/backups/container/immich");
    const before = mocks.listItems.mock.calls.length;
    await press(corner().getByRole("button", { name: en["schedule.pause"] }));
    expect(setInclude).toHaveBeenCalledWith("immich", false);
    await waitFor(() => expect(mocks.listItems.mock.calls.length).toBeGreaterThan(before));
  });

  it("resumes a paused folder set through the call of its kind", async () => {
    serveEntries(mocks, { items: [{ ...PHOTOS, included: false }] });
    await renderPage(`/backups/files/${SET_ID}`);
    await press(corner().getByRole("button", { name: en["schedule.resume"] }));
    expect(patchFileSet).toHaveBeenCalledWith(SET_ID, { enabled: true });
  });

  it.each([
    ["the flash drive", "/backups/flash/flash"],
    ["the self-backup", "/backups/config/config"],
  ])("has no pause for %s, which has no switch of its own", async (_, path) => {
    await renderPage(path);
    expect(corner().queryByRole("button", { name: en["schedule.pause"] })).toBeNull();
  });

  it("offers nothing to back up for an entry that is gone", async () => {
    await renderPage("/backups/container/pihole");
    expect(screen.getByText(en["entry.gone.container"])).toBeTruthy();
    expect(screen.getByText(en["entry.reason.gone"], { exact: false })).toBeTruthy();
    expect(corner().queryByRole("button", { name: en["containers.backupNow"] })).toBeNull();
  });
});

describe("the rare actions", () => {
  it("deletes the backups of a container that is gone and leaves for the list", async () => {
    await renderPage("/backups/container/pihole");
    await press(button(en["containers.deleteBackups"]));
    await press(within(screen.getByRole("dialog")).getByRole("button", { name: en["containers.deleteBackups"] }));
    await waitFor(() => expect(deleteBackups).toHaveBeenCalledWith(PIHOLE.key));
    await waitFor(() => expect(screen.getByText("at /backups?kind=container")).toBeTruthy());
  });

  it("takes a folder set off the list after asking", async () => {
    await renderPage(`/backups/files/${SET_ID}`);
    await press(button(en["zfs.removeMissing"]));
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText(en["files.deleteSetConfirm"])).toBeTruthy();
    await press(within(dialog).getByRole("button", { name: en["zfs.removeMissing"] }));
    await waitFor(() => expect(deleteFileSet).toHaveBeenCalledWith(SET_ID));
    await waitFor(() => expect(screen.getByText("at /backups?kind=files")).toBeTruthy());
  });

  it("keeps the entry when the server refuses to remove it", async () => {
    deleteFileSet.mockResolvedValue({ ok: false, error: "busy" });
    await renderPage(`/backups/files/${SET_ID}`);
    await press(button(en["zfs.removeMissing"]));
    await press(within(screen.getByRole("dialog")).getByRole("button", { name: en["zfs.removeMissing"] }));
    await waitFor(() => expect(pushed).toContainEqual({ message: "busy", severity: "fail" }));
    expect(address()).toBe(`/backups/files/${SET_ID}`);
  });

  it("offers the entry a renamed container looks like", async () => {
    serveEntries(mocks, {
      containers: [containerView({ name: "gitea", lastBackup: null, renameFrom: "gitea-old", renameReason: "appdata-bind" })],
    });
    await renderPage("/backups/container/gitea");
    expect(await screen.findByText(en["takeover.looksLike"].replace("{old}", "gitea-old"))).toBeTruthy();
    expect(button(en["takeover.accept"])).toBeTruthy();
  });

  it("names what an entry was called before", async () => {
    serveEntries(mocks, { containers: [containerView({ name: "immich", aliases: ["immich-old"] })] });
    await renderPage("/backups/container/immich");
    expect(screen.getByText(en["takeover.renamed"])).toBeTruthy();
  });
});
