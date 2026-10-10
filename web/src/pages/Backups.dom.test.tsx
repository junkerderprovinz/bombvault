// @vitest-environment jsdom
/**
 * The Backups page.
 *
 * One list stands for six kinds of entry, so each has to land under the right
 * tile with the right status, and whatever the list offers has to reach the
 * call of the kind it is about. Only EventSource is faked for the progress
 * stream, because jsdom has none.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";

import { I18nProvider, en } from "../lib/i18n";
import { AnomalyProvider } from "../lib/useAnomalies";
import { DESKTOP_QUERY } from "../lib/useMediaQuery";
import { formatRecent } from "../lib/reltime";
import type { AnomalyItem, AnomalyView, BackupItem, BackupItemKind, Settings } from "../lib/api";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listItems: vi.fn(),
    getSettings: vi.fn(),
    getAnomalies: vi.fn(),
    getAnomalySummary: vi.fn(),
    getAnomalyItems: vi.fn(),
    discoverAll: vi.fn(),
    backupEverythingNow: vi.fn(),
    backupAll: vi.fn(),
    backupFilesAll: vi.fn(),
    backupZFSAll: vi.fn(),
    backupFlashNow: vi.fn(),
  };
});

const pushed: { message: string; severity?: string }[] = [];
vi.mock("../lib/toast", () => ({
  useToast: () => ({ push: (message: string, severity?: string) => pushed.push({ message, severity }) }),
}));

const api = await import("../lib/api");
const listItems = vi.mocked(api.listItems);
const getSettings = vi.mocked(api.getSettings);
const getAnomalies = vi.mocked(api.getAnomalies);
const getAnomalySummary = vi.mocked(api.getAnomalySummary);
const getAnomalyItems = vi.mocked(api.getAnomalyItems);
const discoverAll = vi.mocked(api.discoverAll);
const backupEverythingNow = vi.mocked(api.backupEverythingNow);
const backupAll = vi.mocked(api.backupAll);
const backupFilesAll = vi.mocked(api.backupFilesAll);
const backupFlashNow = vi.mocked(api.backupFlashNow);

const { Backups } = await import("./Backups");

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

const NOW = Math.floor(Date.now() / 1000);
const HOUR = 3600;
const EVERYTHING = { kind: "everything", spec: "daily@03:00", alsoSpec: "" } as const;

function entry(over: Partial<BackupItem> = {}): BackupItem {
  return {
    kind: "container",
    key: "plex",
    name: over.key ?? "plex",
    included: true,
    paused: false,
    effectiveSchedule: EVERYTHING,
    lastBackup: NOW - 2 * HOUR,
    lastRunStatus: "success",
    sourceBytes: 1024 ** 3,
    state: "running",
    installed: true,
    runs: [],
    ...over,
  };
}

function runs(...statuses: string[]) {
  return statuses.map((status, i) => ({ id: `r${i}`, kind: "backup", status, startedAt: NOW - i * 86400 }));
}

function watched(over: Partial<AnomalyItem> = {}): AnomalyItem {
  return {
    targetId: "tg-plex",
    domain: "container",
    name: "plex",
    scheduled: true,
    sensitivity: "",
    effective: "balanced",
    notifyMin: "",
    effectiveNotifyMin: "warning",
    learning: { samples: 10, needed: 10, newData: 10, source: 10, duration: 10, noData: false },
    typical: { sourceBytes: 1024, newDataBytes: 1024, resticMs: 1000 },
    dump: null,
    datasets: [],
    open: { critical: 0, warning: 0, info: 0 },
    retentionHeld: false,
    selectionSince: 0,
    expectations: [],
    ...over,
  };
}

const ALL_ON: Partial<Settings> = {
  containersEnabled: true,
  vmsEnabled: true,
  flashEnabled: true,
  filesEnabled: true,
  zfsEnabled: true,
  configEnabled: true,
};

function serve(items: BackupItem[], unlisted: BackupItemKind[] = []) {
  listItems.mockResolvedValue({ ok: true, items, unlisted });
}

function Elsewhere() {
  const { pathname, search } = useLocation();
  return <p>{`at ${pathname}${search}`}</p>;
}

function Address() {
  const { pathname, search } = useLocation();
  return <p data-testid="address">{`${pathname}${search}`}</p>;
}

async function renderPage(path = "/backups") {
  await act(async () => {
    render(
      <I18nProvider>
        <MemoryRouter initialEntries={[path]}>
          <AnomalyProvider>
            <Routes>
              <Route
                path="/backups"
                element={
                  <>
                    <Backups />
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

const rows = () => screen.queryAllByTestId("backup-row");
const rowNames = () => rows().map((row) => within(row).getByRole("link", { name: /.*/ }).textContent);
const row = (name: string) => rows().find((r) => within(r).queryByRole("link", { name }))!;
const tiles = () => within(screen.getByRole("group", { name: en["backups.tiles"] }));
const tile = (name: string) => tiles().getByRole("button", { name: new RegExp(`^${name}`) });
const action = (name: string) => within(screen.getByTestId("page-actions")).queryByRole("button", { name });

async function press(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

const MIXED = [
  entry({ key: "plex" }),
  entry({ key: "gitea", lastBackup: 0, lastRunStatus: "", included: false }),
  entry({ key: "bombvault", self: true, lastBackup: 0, lastRunStatus: "" }),
  entry({ key: "pihole", installed: false, state: undefined }),
  entry({ kind: "vm", key: "win11", name: "Windows 11", state: "shut off" }),
  entry({ kind: "files", key: "f1", name: "Documents", state: undefined, installed: undefined }),
  entry({ kind: "flash", key: "flash", name: "flash", state: undefined, installed: undefined }),
  entry({ kind: "config", key: "config", name: "config", state: undefined, installed: undefined }),
];

beforeEach(() => {
  desktop = true;
  streams.length = 0;
  pushed.length = 0;
  localStorage.clear();
  localStorage.setItem("bv-lang", "en");
  globalThis.EventSource = FakeEventSource as unknown as typeof EventSource;
  vi.clearAllMocks();

  getSettings.mockResolvedValue({ ok: true, settings: ALL_ON as Settings });
  getAnomalySummary.mockResolvedValue({ ok: true, summary: { enabled: true, generation: 1 } } as never);
  getAnomalyItems.mockResolvedValue({ ok: true, items: [] });
  getAnomalies.mockResolvedValue({ ok: true, anomalies: [], nextCursor: "" });
  serve(MIXED);
});

afterEach(() => {
  cleanup();
  localStorage.clear();
});

describe("the tiles", () => {
  it("count the entries of each kind and leave BombVault's own container out", async () => {
    await renderPage();
    expect(tile("All").textContent).toContain("7 entries");
    expect(tile("Containers").textContent).toContain("3 entries");
    expect(tile("VMs").textContent).toContain("1 entry");
    expect(tile("Not installed").textContent).toContain("1 entry");
  });

  it("offer a kind that is switched on before it has an entry, and none for the self-backup", async () => {
    await renderPage();
    expect(tile("ZFS").textContent).toContain("0 entries");
    expect(tiles().queryByRole("button", { name: /^Self-Backup/ })).toBeNull();
  });

  it("drop a kind that is off and has nothing, and the Not installed tile when nothing is gone", async () => {
    getSettings.mockResolvedValue({ ok: true, settings: { ...ALL_ON, zfsEnabled: false, vmsEnabled: false } as Settings });
    serve([entry({ key: "plex" }), entry({ kind: "vm", key: "win11", kindDisabled: true })]);
    await renderPage();
    expect(tiles().queryByRole("button", { name: /^ZFS/ })).toBeNull();
    expect(tiles().queryByRole("button", { name: /^Not installed/ })).toBeNull();
    // A switched-off kind keeps its tile while an entry with a backup is left.
    expect(tile("VMs").textContent).toContain("1 entry");
  });

  it("narrow the list, name it in the card's heading and keep the choice in the address", async () => {
    await renderPage();
    expect(screen.getByRole("heading", { level: 2 }).textContent).toBe("All entries");
    await press(tile("Containers"));
    expect(screen.getByTestId("address").textContent).toBe("/backups?kind=container");
    expect(rowNames()).toEqual(["gitea", "plex", "pihole"]);
    expect(screen.getByRole("heading", { level: 2 }).textContent).toBe("Containers");
    expect(tile("Containers").getAttribute("aria-pressed")).toBe("true");
    // The other tiles stay: choosing one hides rows and nothing else.
    expect(tile("VMs")).toBeTruthy();
    await press(tile("All"));
    expect(screen.getByTestId("address").textContent).toBe("/backups");
  });

  it("open on the tile the address names", async () => {
    await renderPage("/backups?kind=not-installed");
    expect(rowNames()).toEqual(["pihole"]);
    expect(tile("Not installed").getAttribute("aria-pressed")).toBe("true");
  });

  it("fall back to All when the address names a kind without a tile", async () => {
    getSettings.mockResolvedValue({ ok: true, settings: { ...ALL_ON, zfsEnabled: false } as Settings });
    await renderPage("/backups?kind=zfs");
    expect(tile("All").getAttribute("aria-pressed")).toBe("true");
    expect(rows()).toHaveLength(7);
  });
});

describe("a row", () => {
  it("words the flash drive and the self-backup, which arrive named by their key", async () => {
    await renderPage();
    expect(rowNames()).toContain("Unraid flash");
    expect(rowNames()).toContain("BombVault settings");
  });

  it("shows the last backup of a protected entry and a warning on one nothing backs up", async () => {
    await renderPage();
    expect(row("plex").textContent).toContain(formatRecent(NOW - 2 * HOUR, "en"));
    expect(row("gitea").textContent).toContain("Not protected");
  });

  it("waits for the first backup of an entry that is scheduled", async () => {
    serve([entry({ key: "new", lastBackup: 0, lastRunStatus: "" })]);
    await renderPage();
    expect(row("new").textContent).toContain("Waiting for the first backup");
    expect(within(row("new")).getByLabelText("At the next scheduled run.")).toBeTruthy();
  });

  it("says Failed for a failed last run and keeps the last good backup in the bubble", async () => {
    serve([entry({ key: "paperless", lastRunStatus: "failed", runs: runs("failed", "success", "success") })]);
    await renderPage();
    expect(row("paperless").textContent).toContain("Failed");
    const bubble = within(row("paperless")).getByLabelText(/^Last good snapshot/);
    expect(bubble.getAttribute("aria-label")).toContain(formatRecent(NOW - 2 * HOUR, "en"));
  });

  it("links open anomalies to that entry's findings", async () => {
    getAnomalyItems.mockResolvedValue({
      ok: true,
      items: [watched({ open: { critical: 1, warning: 1, info: 4 } })],
    });
    await renderPage();
    const link = within(row("plex")).getByRole("link", { name: "2 anomalies" });
    expect(link.getAttribute("href")).toBe("/anomalies?scope=item:tg-plex");
  });

  it("does not raise a note to an anomaly", async () => {
    getAnomalyItems.mockResolvedValue({ ok: true, items: [watched({ open: { critical: 0, warning: 0, info: 2 } })] });
    await renderPage();
    expect(within(row("plex")).queryByRole("link", { name: /anomal/i })).toBeNull();
  });

  it("carries what else there is to say in the bubble beside the name", async () => {
    getAnomalyItems.mockResolvedValue({
      ok: true,
      items: [watched({ learning: { samples: 3, needed: 10, newData: 3, source: 3, duration: 3, noData: false } })],
    });
    serve([
      entry({ key: "plex", state: "exited", effectiveSchedule: { kind: "own", spec: "daily@04:00", alsoSpec: "" } }),
      entry({ kind: "vm", key: "win11", state: "shut off" }),
    ]);
    await renderPage();
    const said = within(row("plex")).getByLabelText(/^The container is stopped\./).getAttribute("aria-label")!;
    expect(said).toContain("Learning 3/10.");
    expect(said).toMatch(/Runs .*, its own schedule\.$/);
    expect(within(row("win11")).getByLabelText("The VM is off.")).toBeTruthy();
  });

  it("has no bubble when it follows the common schedule and nothing is up", async () => {
    serve([entry({ key: "plex" })]);
    await renderPage();
    expect(within(row("plex")).queryByLabelText(/./)?.getAttribute("role")).toBe("img");
  });

  it("draws the last runs and says in words what the squares show", async () => {
    getAnomalies.mockResolvedValue({
      ok: true,
      anomalies: [{ id: "a1", runId: "r1", lastRunId: "r1", severity: "warning" } as AnomalyView],
      nextCursor: "",
    });
    serve([entry({ key: "plex", runs: runs("failed", "success", "success", "cancelled") })]);
    await renderPage();
    const strip = within(row("plex")).getByRole("img");
    expect(strip.getAttribute("aria-label")).toBe("Recent runs: 1 ok, 1 with an anomaly, 1 failed");
    expect(strip.children).toHaveLength(14);
  });

  it("says so when an entry has not run yet", async () => {
    serve([entry({ key: "new", lastBackup: 0, lastRunStatus: "" })]);
    await renderPage();
    expect(within(row("new")).getByRole("img").getAttribute("aria-label")).toBe("No runs yet");
  });

  it("opens the page of its kind from anywhere on it", async () => {
    await renderPage();
    await press(within(row("Documents")).getByRole("img"));
    expect(screen.getByText("at /files")).toBeTruthy();
  });

  it("stays put when its bubble is pressed", async () => {
    await renderPage();
    await press(within(row("gitea")).getByLabelText("Schedule paused."));
    expect(screen.queryByText(/^at /)).toBeNull();
  });
});

describe("the entries that are not backed up like the rest", () => {
  it("lists what is gone from the host last, dimmed, under a heading that explains it", async () => {
    await renderPage();
    expect(rowNames().at(-1)).toBe("pihole");
    expect(row("pihole").className).toContain("opacity-55");
    const heading = screen.getByRole("heading", { level: 3 });
    expect(heading.textContent).toBe("Not installed (backups only)");
    expect(within(heading).getByLabelText(en["backups.notInstalledHint"])).toBeTruthy();
  });

  it("shows BombVault's own container after the installed entries, as a note and not as a link", async () => {
    await renderPage();
    const self = screen.getByTestId("backup-self-row");
    expect(self.textContent).toContain("Does not back itself up");
    expect(within(self).queryByRole("link")).toBeNull();
    expect(within(self).getByLabelText(en["backups.selfNote"])).toBeTruthy();
    expect(self.compareDocumentPosition(row("pihole")) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(self.compareDocumentPosition(row("Windows 11")) & Node.DOCUMENT_POSITION_PRECEDING).toBeTruthy();
  });

  it("dims an entry of a switched-off kind and says why it is not backed up", async () => {
    serve([
      entry({
        kind: "files",
        key: "f1",
        name: "Documents",
        kindDisabled: true,
        effectiveSchedule: { kind: "none", spec: "", alsoSpec: "", reason: "domain-off" },
      }),
      entry({ key: "untouched", kindDisabled: true, lastBackup: 0, lastRunStatus: "" }),
    ]);
    await renderPage();
    expect(rowNames()).toEqual(["Documents"]);
    expect(row("Documents").className).toContain("opacity-55");
    expect(within(row("Documents")).getByLabelText("Not backed up automatically: the Folders domain is off.")).toBeTruthy();
  });

  it("keeps a paused entry undimmed and names the pause in its bubble", async () => {
    serve([
      entry({ key: "plex", included: false, effectiveSchedule: { kind: "none", spec: "", alsoSpec: "", reason: "excluded" } }),
      entry({ key: "sonarr", paused: true, effectiveSchedule: { kind: "none", spec: "", alsoSpec: "", reason: "override-off" } }),
    ]);
    await renderPage();
    expect(row("plex").className).not.toContain("opacity-55");
    expect(within(row("plex")).getByLabelText("Schedule paused.")).toBeTruthy();
    expect(within(row("sonarr")).getByLabelText("Schedule paused.")).toBeTruthy();
  });

  it("says why an entry in the schedule still never runs", async () => {
    serve([entry({ key: "plex", effectiveSchedule: { kind: "none", spec: "", alsoSpec: "", reason: "schedule-off" } })]);
    await renderPage();
    expect(
      within(row("plex")).getByLabelText(
        "Not backed up automatically: neither the Containers schedule nor Backup Everything is on."
      )
    ).toBeTruthy();
  });
});

describe("filter, sort and search", () => {
  it("filter by schedule, and the choice outlives the page", async () => {
    await renderPage();
    await press(screen.getByRole("button", { name: "Filters" }));
    await press(within(screen.getByRole("dialog", { name: "Filters" })).getByRole("tab", { name: "Paused" }));
    expect(rowNames()).toEqual(["gitea"]);
    // BombVault's own container has no schedule to be in.
    expect(screen.queryByTestId("backup-self-row")).toBeNull();

    cleanup();
    await renderPage();
    expect(rowNames()).toEqual(["gitea"]);
  });

  it("filter by backup and by whether the container or VM is installed", async () => {
    await renderPage();
    await press(screen.getByRole("button", { name: "Filters" }));
    const panel = within(screen.getByRole("dialog", { name: "Filters" }));
    await press(panel.getByRole("tab", { name: "Never backed up" }));
    expect(rowNames()).toEqual(["gitea"]);
    await press(panel.getAllByRole("tab", { name: "All" })[1]);
    await press(panel.getByRole("tab", { name: "Not installed" }));
    expect(rowNames()).toEqual(["pihole"]);
  });

  it("sort by the chosen order and say which one is on", async () => {
    serve([
      entry({ key: "small", sourceBytes: 10 }),
      entry({ key: "large", sourceBytes: 1000 }),
      entry({ key: "medium", sourceBytes: 100 }),
    ]);
    await renderPage();
    expect(rowNames()).toEqual(["large", "medium", "small"]);
    await press(screen.getByRole("button", { name: "Name A to Z" }));
    await press(screen.getByRole("option", { name: "Size, smallest first" }));
    expect(rowNames()).toEqual(["small", "medium", "large"]);
    expect(screen.getByRole("button", { name: "Size, smallest first" })).toBeTruthy();
  });

  it("offer every order in both directions", async () => {
    await renderPage();
    await press(screen.getByRole("button", { name: "Name A to Z" }));
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual([
      "Name A to Z",
      "Name Z to A",
      "Status, problems first",
      "Status, problems last",
      "Last backup, newest first",
      "Last backup, oldest first",
      "Size, largest first",
      "Size, smallest first",
      "Schedule, scheduled first",
      "Schedule, paused first",
    ]);
  });

  it("search turns into a field, finds by name and says when nothing matches", async () => {
    await renderPage();
    await press(screen.getByRole("button", { name: "Search" }));
    const field = screen.getByRole("searchbox", { name: "Search" });
    fireEvent.change(field, { target: { value: "win" } });
    expect(rowNames()).toEqual(["Windows 11"]);
    fireEvent.change(field, { target: { value: "nothing like this" } });
    expect(rows()).toHaveLength(0);
    expect(screen.getByText("No items match the current filters.")).toBeTruthy();
    fireEvent.keyDown(field, { key: "Escape" });
    expect(rows()).toHaveLength(7);
  });
});

describe("a running backup", () => {
  it("shows how far it is and offers to cancel it", async () => {
    await renderPage();
    progress({ key: "container:plex", phase: "backup", percent: 36.4, active: true });
    expect(row("plex").textContent).toContain("Backing up · 36 %");
    expect(within(row("plex")).getByRole("progressbar").getAttribute("aria-valuenow")).toBe("36");
    expect(within(row("plex")).getByRole("button", { name: "Cancel backup" })).toBeTruthy();
  });

  it("names the database dump and its size while that is the step", async () => {
    await renderPage();
    progress({ key: "container:plex", phase: "backup", percent: 0, active: true, stage: "dbdump", bytes: 5 * 1024 ** 2 });
    expect(row("plex").textContent).toContain("Dumping the database: 5.0 MB");
  });

  it("finds a VM by its libvirt name and a folder set by its name", async () => {
    await renderPage();
    progress({ key: "vm:win11", phase: "backup", percent: 10, active: true });
    progress({ key: "files:Documents", phase: "backup", percent: 20, active: true });
    expect(row("Windows 11").textContent).toContain("Backing up · 10 %");
    expect(row("Documents").textContent).toContain("Backing up · 20 %");
  });

  it("offers no cancel during a restore", async () => {
    await renderPage();
    progress({ key: "container:plex", phase: "restore", percent: 50, active: true });
    expect(row("plex").textContent).toContain("Being restored");
    expect(within(row("plex")).queryByRole("button", { name: "Cancel backup" })).toBeNull();
  });

  it("reads the list again when the run ends, and not while it runs", async () => {
    vi.useFakeTimers();
    try {
      await renderPage();
      expect(listItems).toHaveBeenCalledTimes(1);
      progress({ key: "container:plex", phase: "backup", percent: 50, active: true });
      progress({ key: "container:plex", phase: "backup", percent: 90, active: true });
      expect(listItems).toHaveBeenCalledTimes(1);
      progress({ key: "container:plex", phase: "backup", percent: 100, active: false });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1000);
      });
      expect(listItems).toHaveBeenCalledTimes(2);
    } finally {
      vi.useRealTimers();
    }
  });
});

describe("loading, nothing and failure", () => {
  it("says it is checking until the list arrives", async () => {
    listItems.mockReturnValue(new Promise(() => {}));
    await renderPage();
    expect(screen.getByText("Checking…")).toBeTruthy();
    expect(screen.queryByTestId("page-actions")).toBeNull();
  });

  it("says nothing is backed up yet and how something gets here", async () => {
    serve([]);
    await renderPage();
    expect(screen.getByText(en["backups.empty"])).toBeTruthy();
    expect(screen.getByText(en["backups.emptyHow"])).toBeTruthy();
    expect(action("Add")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Filters" })).toBeNull();
  });

  it("does not pass a failed load off as an empty list, and tries again on request", async () => {
    listItems.mockRejectedValueOnce(new Error("boom"));
    await renderPage();
    expect(screen.getByText(en["backups.loadFailed"])).toBeTruthy();
    expect(screen.queryByText(en["backups.empty"])).toBeNull();
    await press(screen.getByRole("button", { name: "Try again" }));
    expect(rows()).toHaveLength(7);
    expect(screen.queryByText(en["backups.loadFailed"])).toBeNull();
  });

  it("treats a refusal like a failure", async () => {
    listItems.mockResolvedValue({ ok: false, items: [], unlisted: [], error: "locked" });
    await renderPage();
    expect(screen.getByText(en["backups.loadFailed"])).toBeTruthy();
  });

  it("names the kinds the host did not answer for", async () => {
    serve(MIXED, ["container", "vm"]);
    await renderPage();
    expect(screen.getByText(en["backups.unlisted"].replace("{kinds}", "Containers and VMs"))).toBeTruthy();
  });
});

describe("the page actions", () => {
  it("search the storage for backups, say what turned up and read the list again", async () => {
    discoverAll.mockResolvedValue({
      containers: 2,
      vms: 0,
      files: 1,
      zfs: 0,
      skipped: ["Backblaze B2"],
      skippedNeedsAction: false,
      paused: [],
      leftOpen: [],
      directRepos: [],
    });
    await renderPage();
    await press(action("Discover backups")!);
    expect(pushed).toEqual([
      { message: "Not everything was searched: Backblaze B2", severity: "warn" },
      { message: "Storage locations searched: 3 new entries found.", severity: "success" },
    ]);
    expect(listItems).toHaveBeenCalledTimes(2);
  });

  it("say so when the search finds nothing new, and pass a failure on", async () => {
    const none = { containers: 0, vms: 0, files: 0, zfs: 0, skipped: [], skippedNeedsAction: false, paused: [], leftOpen: [], directRepos: [] };
    discoverAll.mockResolvedValueOnce(none).mockResolvedValueOnce({ ...none, error: "wrong key" });
    await renderPage();
    await press(action("Discover backups")!);
    await press(action("Discover backups")!);
    expect(pushed).toEqual([
      { message: "Storage locations searched: nothing new found.", severity: "success" },
      { message: "wrong key", severity: "fail" },
    ]);
  });

  it("back up everything from the All tile", async () => {
    backupEverythingNow.mockResolvedValue({ ok: true, started: true });
    await renderPage();
    await press(action("Back up everything now")!);
    expect(backupEverythingNow).toHaveBeenCalledTimes(1);
    expect(pushed.at(-1)?.severity).toBe("success");
  });

  it("say a pass is already running instead of failing", async () => {
    backupEverythingNow.mockRejectedValue(new api.ApiError(409, "busy"));
    await renderPage();
    await press(action("Back up everything now")!);
    expect(pushed).toEqual([{ message: en["settings.everythingAlreadyRunning"], severity: "warn" }]);
  });

  it("back up what the schedule would run of the chosen kind", async () => {
    backupAll.mockResolvedValue({ ok: true, started: 1 });
    await renderPage("/backups?kind=container");
    await press(action("Back up all now")!);
    // Not the paused one, not the one that is gone, not BombVault itself.
    expect(backupAll).toHaveBeenCalledWith(["plex"]);
    expect(pushed).toEqual([{ message: en["containers.batchStarted"], severity: "success" }]);
  });

  it("send a folder set's id and the flash drive nothing", async () => {
    backupFilesAll.mockResolvedValue({ ok: true });
    backupFlashNow.mockResolvedValue({ ok: true });
    await renderPage("/backups?kind=files");
    await press(action("Back up all now")!);
    expect(backupFilesAll).toHaveBeenCalledWith(["f1"]);
    await press(tile("Flash"));
    await press(action("Back up all now")!);
    expect(backupFlashNow).toHaveBeenCalledTimes(1);
  });

  it("start nothing when the kind has nothing in the schedule", async () => {
    serve([entry({ key: "gitea", included: false })]);
    await renderPage("/backups?kind=container");
    await press(action("Back up all now")!);
    expect(backupAll).not.toHaveBeenCalled();
    expect(pushed).toEqual([{ message: en["backups.nothingScheduled"], severity: "warn" }]);
  });

  it("offer no batch where the server has none: VMs and what is not installed", async () => {
    await renderPage("/backups?kind=vm");
    expect(action("Back up all now")).toBeNull();
    await press(tile("Not installed"));
    expect(action("Back up all now")).toBeNull();
    expect(action("Back up everything now")).toBeNull();
  });

  it("hold the start back while something runs and say what", async () => {
    await renderPage();
    progress({ key: "container:plex", phase: "restore", percent: 5, active: true });
    expect((action("Back up everything now") as HTMLButtonElement).disabled).toBe(true);
  });

  it("add where the chosen kind is added today, under the kind's own word", async () => {
    await renderPage();
    await press(tile("ZFS"));
    expect(action("Add dataset")).toBeTruthy();
    await press(tile("Folders"));
    await press(action("Add folder")!);
    expect(screen.getByText("at /files")).toBeTruthy();
  });

  it("offer no Add when neither folders nor ZFS are switched on", async () => {
    getSettings.mockResolvedValue({ ok: true, settings: { ...ALL_ON, filesEnabled: false, zfsEnabled: false } as Settings });
    await renderPage();
    expect(action("Add")).toBeNull();
    expect(action("Discover backups")).toBeTruthy();
  });

  it("make Add the one filled accent of the corner", async () => {
    await renderPage();
    const accents = within(screen.getByTestId("page-actions"))
      .getAllByRole("button")
      .filter((b) => b.className.includes("bg-accent"));
    expect(accents.map((b) => b.textContent)).toEqual(["Add"]);
  });
});

describe("on a phone", () => {
  it("shows twenty rows and loads more on request", async () => {
    desktop = false;
    serve(Array.from({ length: 45 }, (_, i) => entry({ key: `c${String(i).padStart(2, "0")}` })));
    await renderPage();
    expect(rows()).toHaveLength(20);
    await press(screen.getByRole("button", { name: "Load more" }));
    expect(rows()).toHaveLength(40);
    await press(screen.getByRole("button", { name: "Load more" }));
    expect(rows()).toHaveLength(45);
    expect(screen.queryByRole("button", { name: "Load more" })).toBeNull();
  });

  it("asks before it backs up everything, because the corner is one tap away", async () => {
    desktop = false;
    backupEverythingNow.mockResolvedValue({ ok: true, started: true });
    await renderPage();
    await press(action("Back up everything now")!);
    expect(backupEverythingNow).not.toHaveBeenCalled();
    expect(screen.getByText(en["home.newBackupConfirm"])).toBeTruthy();
  });
});
