// @vitest-environment jsdom
/**
 * The Backups page.
 *
 * One list stands for six kinds of entry, so each has to land under the right
 * tile with the right status, and whatever the list offers has to reach the
 * call of the kind it is about. EventSource is faked for the progress stream,
 * because jsdom has none, and the windows the page opens are stand-ins: each
 * has tests of its own.
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
    backupVMsAll: vi.fn(),
    backupFilesAll: vi.fn(),
    backupZFSAll: vi.fn(),
    backupFlashNow: vi.fn(),
    backupNow: vi.fn(),
    backupVMNow: vi.fn(),
    cancelBackup: vi.fn(),
    setInclude: vi.fn(),
    setVMInclude: vi.fn(),
    patchFileSet: vi.fn(),
    restore: vi.fn(),
    restoreVM: vi.fn(),
    zfsConnection: vi.fn(),
  };
});

vi.mock("../lib/backupWatch", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/backupWatch")>()),
  fireAndWaitRun: vi.fn(),
}));

vi.mock("../components/backups/AddWindow", () => ({
  AddWindow: ({ start }: { start: string }) => <div data-testid="add-window">{start}</div>,
}));
vi.mock("../components/backups/StackWindow", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../components/backups/StackWindow")>()),
  StackWindow: ({ initial }: { initial: string }) => <div data-testid="stack-window">{initial}</div>,
}));
vi.mock("../components/backups/ZFSConnectionWindow", () => ({
  ZFSConnectionWindow: () => <div data-testid="zfs-window" />,
}));

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
const backupVMsAll = vi.mocked(api.backupVMsAll);
const backupFilesAll = vi.mocked(api.backupFilesAll);
const backupFlashNow = vi.mocked(api.backupFlashNow);
const backupNow = vi.mocked(api.backupNow);
const backupVMNow = vi.mocked(api.backupVMNow);
const cancelBackup = vi.mocked(api.cancelBackup);
const setInclude = vi.mocked(api.setInclude);
const setVMInclude = vi.mocked(api.setVMInclude);
const patchFileSet = vi.mocked(api.patchFileSet);
const zfsConnection = vi.mocked(api.zfsConnection);

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
const EVERYTHING = { kind: "everything", spec: "daily@03:00" } as const;

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

const picked = () => rows().filter((r) => r.hasAttribute("data-picked")).map((r) => within(r).getByRole("link").textContent);
const bar = () => screen.queryByTestId("bulk-bar");
const barButton = (name: string) => within(bar()!).getByRole("button", { name });
const menuItems = () => screen.queryAllByRole("menuitem").map((item) => item.textContent);

async function click(el: HTMLElement, keys: { ctrlKey?: boolean; shiftKey?: boolean } = {}) {
  await act(async () => {
    fireEvent.click(el, keys);
  });
}

async function rightClick(el: HTMLElement) {
  await act(async () => {
    fireEvent.pointerDown(el, { pointerType: "mouse", button: 2 });
    fireEvent.contextMenu(el, { clientX: 40, clientY: 60 });
  });
}

async function chooseFromMenu(name: string) {
  await press(screen.getByRole("menuitem", { name }));
}

async function openMore() {
  await press(action("More")!);
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

// Two backed-up containers of one compose project.
const STACK = [entry({ key: "immich", stack: "immich" }), entry({ key: "immich_postgres", stack: "immich" })];

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
  zfsConnection.mockReturnValue(new Promise(() => {}));
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
    expect(tiles().queryByRole("button", { name: /^Self-backup/ })).toBeNull();
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
      entry({ key: "plex", state: "exited", effectiveSchedule: { kind: "own", spec: "daily@04:00" } }),
      entry({ kind: "vm", key: "win11", state: "shut off" }),
    ]);
    await renderPage();
    const said = within(row("plex")).getByLabelText(/^The container is stopped\./).getAttribute("aria-label")!;
    expect(said).toContain("Learning 3/10.");
    expect(said).toMatch(/Runs .*, its own schedule\.$/);
    expect(within(row("win11")).getByLabelText("The VM is off.")).toBeTruthy();
  });

  it("writes the bubble's text into the row for a mouse that hovers it", async () => {
    serve([entry({ kind: "vm", key: "win11", state: "shut off" })]);
    await renderPage();
    const facts = within(row("win11")).getByTestId("backup-row-facts");
    expect(facts.textContent).toBe("The VM is off.");
    expect(facts.className).toContain("hidden");
    expect(facts.className).toContain("pointer-fine:group-hover:block");
    expect(facts.getAttribute("aria-hidden")).toBe("true");
  });

  it("has no bubble when it follows the common schedule and nothing is up", async () => {
    serve([entry({ key: "plex" })]);
    await renderPage();
    const labelled = within(row("plex")).queryAllByLabelText(/./);
    expect(labelled.map((el) => el.getAttribute("role"))).toEqual(["checkbox", "img"]);
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
    expect(within(row("new")).getByRole("img", { name: "No runs yet" })).toBeTruthy();
  });

  it("opens the page of its entry from anywhere on it", async () => {
    await renderPage();
    await press(within(row("Documents")).getByRole("img"));
    expect(screen.getByText("at /backups/files/f1")).toBeTruthy();
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
        effectiveSchedule: { kind: "none", spec: "", reason: "domain-off" },
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
      entry({ key: "plex", included: false, effectiveSchedule: { kind: "none", spec: "", reason: "excluded" } }),
      entry({ key: "sonarr", paused: true, effectiveSchedule: { kind: "none", spec: "", reason: "override-off" } }),
    ]);
    await renderPage();
    expect(row("plex").className).not.toContain("opacity-55");
    expect(within(row("plex")).getByLabelText("Schedule paused.")).toBeTruthy();
    expect(within(row("sonarr")).getByLabelText("Schedule paused.")).toBeTruthy();
  });

  it("dims an entry the person said not to back up, says so in its bubble and still counts it on its tile", async () => {
    serve([
      entry({ key: "plex" }),
      entry({
        key: "scratch",
        included: false,
        effectiveSchedule: { kind: "none", spec: "", reason: "excluded" },
        backupDeclined: true,
      }),
    ]);
    await renderPage();
    expect(row("scratch").className).toContain("opacity-55");
    expect(within(row("scratch")).getByLabelText(`${en["entry.reason.declined"]}.`, { exact: false })).toBeTruthy();
    expect(within(row("scratch")).getByText(en["backups.status.notProtected"])).toBeTruthy();
    expect(tile("Containers").textContent).toContain("2 entries");
  });

  it("says why an entry in the schedule still never runs", async () => {
    serve([entry({ key: "plex", effectiveSchedule: { kind: "none", spec: "", reason: "schedule-off" } })]);
    await renderPage();
    expect(
      within(row("plex")).getByLabelText(
        "Not backed up automatically: neither the Containers schedule nor the full backup is on."
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

  it("offer every order, in both directions where it has two", async () => {
    await renderPage();
    await press(screen.getByRole("button", { name: "Name A to Z" }));
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual([
      "Name A to Z",
      "Name Z to A",
      "Status, problems first",
      "Status, problems last",
      "IP",
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
    await openMore();
    await chooseFromMenu("Discover backups");
    expect(pushed).toEqual([
      { message: "Not everything was searched: Backblaze B2", severity: "warn" },
      { message: "Storage locations searched: 3 new entries found.", severity: "success" },
    ]);
    expect(listItems).toHaveBeenCalledTimes(2);
  });

  it("say that a rebuilt ZFS item stays switched off until it was checked", async () => {
    discoverAll.mockResolvedValue({
      containers: 0,
      vms: 0,
      files: 0,
      zfs: 2,
      skipped: [],
      skippedNeedsAction: false,
      paused: [],
      leftOpen: [],
      directRepos: [],
    });
    await renderPage();
    await openMore();
    await chooseFromMenu("Discover backups");
    expect(pushed).toEqual([
      { message: "Storage locations searched: 2 new entries found.", severity: "success" },
      { message: "Rebuilt 2 ZFS items. They stay switched off until you check them.", severity: "success" },
    ]);
  });

  it("say so when the search finds nothing new, and pass a failure on", async () => {
    const none = { containers: 0, vms: 0, files: 0, zfs: 0, skipped: [], skippedNeedsAction: false, paused: [], leftOpen: [], directRepos: [] };
    discoverAll.mockResolvedValueOnce(none).mockResolvedValueOnce({ ...none, error: "wrong key" });
    await renderPage();
    for (let i = 0; i < 2; i++) {
      await openMore();
      await chooseFromMenu("Discover backups");
    }
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

  it("back up the VMs through their own batch, and offer none for what is not installed", async () => {
    backupVMsAll.mockResolvedValue({ ok: true });
    await renderPage("/backups?kind=vm");
    await press(action("Back up all now")!);
    expect(backupVMsAll).toHaveBeenCalledWith(["win11"]);
    await press(tile("Not installed"));
    expect(action("Back up all now")).toBeNull();
    expect(action("Back up everything now")).toBeNull();
  });

  it("hold the start back while something runs and say what", async () => {
    await renderPage();
    progress({ key: "container:plex", phase: "restore", percent: 5, active: true });
    expect((action("Back up everything now") as HTMLButtonElement).disabled).toBe(true);
  });

  it("open the add window on the chosen kind, under the kind's own word, and on the choice elsewhere", async () => {
    await renderPage();
    await press(action("Add")!);
    expect(screen.getByTestId("add-window").textContent).toBe("choose");
    await press(tile("ZFS"));
    await press(action("Add dataset")!);
    expect(screen.getByTestId("add-window").textContent).toBe("zfs");
    await press(tile("Folders"));
    await press(action("Add folder")!);
    expect(screen.getByTestId("add-window").textContent).toBe("files");
    expect(screen.queryByText(/^at /)).toBeNull();
  });

  it("offer no Add when neither folders nor ZFS are switched on", async () => {
    getSettings.mockResolvedValue({ ok: true, settings: { ...ALL_ON, filesEnabled: false, zfsEnabled: false } as Settings });
    await renderPage();
    expect(action("Add")).toBeNull();
    expect(action("More")).toBeTruthy();
  });

  it("make Add the one filled accent of the corner", async () => {
    await renderPage();
    const accents = within(screen.getByTestId("page-actions"))
      .getAllByRole("button")
      .filter((b) => b.className.includes("bg-accent"));
    expect(accents.map((b) => b.textContent)).toEqual(["Add"]);
  });
});

describe("picking rows", () => {
  it("adds a row with Ctrl+click and opens nothing", async () => {
    await renderPage();
    expect(bar()).toBeNull();
    await click(row("plex"), { ctrlKey: true });
    await click(row("Documents"), { ctrlKey: true });
    expect(picked()).toEqual(["Documents", "plex"]);
    expect(bar()!.textContent).toContain("2 selected");
    expect(screen.queryByText(/^at /)).toBeNull();
    await click(row("plex"), { ctrlKey: true });
    expect(picked()).toEqual(["Documents"]);
  });

  it("picks the range from the row picked last with Shift+click", async () => {
    await renderPage();
    await click(row("Documents"), { ctrlKey: true });
    await click(row("Unraid flash"), { shiftKey: true });
    expect(picked()).toEqual(["Documents", "gitea", "plex", "Unraid flash"]);
  });

  it("picks every row in view with Ctrl+A and lets go with Escape", async () => {
    await renderPage("/backups?kind=container");
    await act(async () => {
      fireEvent.keyDown(document.body, { key: "a", ctrlKey: true });
    });
    expect(picked()).toEqual(["gitea", "plex", "pihole"]);
    expect(within(bar()!).getByRole("checkbox", { name: "Select all" }).getAttribute("aria-checked")).toBe("true");
    await act(async () => {
      fireEvent.keyDown(document.body, { key: "Escape" });
    });
    expect(bar()).toBeNull();
    expect(picked()).toEqual([]);
  });

  it("leaves Ctrl+A to a field that is being typed in", async () => {
    await renderPage();
    await press(screen.getByRole("button", { name: "Search" }));
    const field = screen.getByRole("searchbox", { name: "Search" });
    field.focus();
    await act(async () => {
      fireEvent.keyDown(field, { key: "a", ctrlKey: true });
    });
    expect(bar()).toBeNull();
  });

  it("picks with a plain click once rows are being picked, on the name as well", async () => {
    await renderPage();
    await click(within(row("plex")).getByRole("checkbox", { name: "Select plex" }));
    await click(row("gitea"));
    await click(within(row("Documents")).getByRole("link"));
    expect(picked()).toEqual(["Documents", "gitea", "plex"]);
    expect(screen.queryByText(/^at /)).toBeNull();
  });

  it("picks and clears every row through the mark in the bar", async () => {
    await renderPage();
    await click(row("plex"), { ctrlKey: true });
    await click(within(bar()!).getByRole("checkbox", { name: "Select all" }));
    expect(picked()).toHaveLength(7);
    await click(within(bar()!).getByRole("checkbox", { name: "Select all" }));
    expect(picked()).toEqual([]);
    expect(bar()).toBeNull();
  });

  it("drops a picked row that the search hides, so nothing acts on what is out of view", async () => {
    await renderPage();
    await click(row("plex"), { ctrlKey: true });
    await click(row("Documents"), { ctrlKey: true });
    await press(screen.getByRole("button", { name: "Search" }));
    fireEvent.change(screen.getByRole("searchbox", { name: "Search" }), { target: { value: "doc" } });
    expect(bar()!.textContent).toContain("1 selected");
    fireEvent.change(screen.getByRole("searchbox", { name: "Search" }), { target: { value: "" } });
    expect(picked()).toEqual(["Documents"]);
  });

  it("starts with a long press of a finger, and the tap that ends it opens nothing", async () => {
    vi.useFakeTimers();
    try {
      await renderPage();
      const plex = row("plex");
      await act(async () => {
        fireEvent.pointerDown(plex, { pointerType: "touch", clientX: 10, clientY: 10 });
        await vi.advanceTimersByTimeAsync(600);
        fireEvent.pointerUp(plex, { pointerType: "touch" });
        fireEvent.click(plex);
      });
      expect(picked()).toEqual(["plex"]);
      expect(screen.queryByText(/^at /)).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });

  it("takes a finger that moves for scrolling", async () => {
    vi.useFakeTimers();
    try {
      await renderPage();
      const plex = row("plex");
      await act(async () => {
        fireEvent.pointerDown(plex, { pointerType: "touch", clientX: 10, clientY: 10 });
        fireEvent.pointerMove(plex, { pointerType: "touch", clientX: 10, clientY: 60 });
        await vi.advanceTimersByTimeAsync(600);
      });
      expect(bar()).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });

  it("draws no mark on a row and leaves the keyboard a switch inside the glyph tile", async () => {
    await renderPage();
    const tile = () => row("plex").querySelector(".glim-row-glyph")!;
    const pick = () => within(row("plex")).getByRole("checkbox", { name: "Select plex" });
    expect(tile().contains(pick())).toBe(true);
    expect(pick().className).toBe("sr-only");
    expect(tile().querySelector(".rounded-full")).toBeNull();

    await click(pick());
    expect(picked()).toEqual(["plex"]);
    expect(pick().getAttribute("aria-checked")).toBe("true");
    expect(tile().contains(pick())).toBe(true);
    expect(row("gitea").querySelector(".glim-row-glyph .rounded-full")).toBeNull();
    expect(within(row("gitea")).getAllByRole("checkbox")).toHaveLength(1);
  });

  it("shows a check in place of the glyph of a picked row", async () => {
    await renderPage();
    const glyph = () => row("plex").querySelector(".glim-row-glyph svg")!.innerHTML;
    const before = glyph();
    await click(row("plex"), { ctrlKey: true });
    expect(glyph()).not.toBe(before);
    await click(row("plex"), { ctrlKey: true });
    expect(glyph()).toBe(before);
  });

  it("has no mark for BombVault's own container", async () => {
    await renderPage();
    expect(within(screen.getByTestId("backup-self-row")).queryByRole("checkbox")).toBeNull();
  });
});

describe("the bar above picked rows", () => {
  async function pick(...names: string[]) {
    for (const name of names) await click(row(name), { ctrlKey: true });
  }

  it("backs up one batch per kind and leaves out what is gone", async () => {
    backupAll.mockResolvedValue({ ok: true });
    await renderPage("/backups?kind=container");
    await pick("plex", "gitea", "pihole");
    await press(barButton("Back up selected"));
    expect(backupAll).toHaveBeenCalledWith(["gitea", "plex"]);
    expect(pushed).toEqual([{ message: `${en["containers.batchStarted"]} 1 skipped`, severity: "success" }]);
  });

  it("starts the next kind once the batch before it has let go of the server", async () => {
    vi.useFakeTimers();
    try {
      backupAll.mockResolvedValue({ ok: true });
      backupVMsAll.mockRejectedValueOnce(new api.ApiError(409, "busy")).mockResolvedValue({ ok: true });
      await renderPage();
      await pick("plex", "Windows 11");
      await press(barButton("Back up selected"));
      expect(backupVMsAll).toHaveBeenCalledTimes(1);
      expect(pushed).toEqual([{ message: en["backups.batch.startedInTurn"], severity: "success" }]);
      expect(screen.getByTestId("batch-note").textContent).toContain("0 of 2");
      await act(async () => {
        await vi.advanceTimersByTimeAsync(2100);
      });
      expect(backupVMsAll).toHaveBeenCalledTimes(2);
      expect(backupVMsAll).toHaveBeenLastCalledWith(["win11"]);
    } finally {
      vi.useRealTimers();
    }
  });

  it("says a backup is already running and starts nothing more", async () => {
    backupAll.mockRejectedValue(new api.ApiError(409, "busy"));
    await renderPage();
    await pick("plex", "Windows 11");
    await press(barButton("Back up selected"));
    expect(backupVMsAll).not.toHaveBeenCalled();
    expect(pushed).toEqual([{ message: en["containers.batchAlreadyRunning"], severity: "warn" }]);
    expect(screen.queryByTestId("batch-note")).toBeNull();
  });

  it("says so when nothing picked can be backed up", async () => {
    await renderPage();
    await pick("pihole");
    await press(barButton("Back up selected"));
    expect(backupAll).not.toHaveBeenCalled();
    expect(pushed).toEqual([{ message: en["backups.batch.nothing"], severity: "warn" }]);
  });

  it("pauses and resumes each entry through the call of its kind", async () => {
    setInclude.mockResolvedValue({ ok: true });
    setVMInclude.mockResolvedValue({ ok: true });
    patchFileSet.mockResolvedValue({ ok: true });
    await renderPage();
    await pick("plex", "Windows 11", "Documents", "Unraid flash");
    await press(barButton("Pause schedule"));
    expect(setInclude).toHaveBeenCalledWith("plex", false);
    expect(setVMInclude).toHaveBeenCalledWith("win11", false);
    expect(patchFileSet).toHaveBeenCalledWith("f1", { enabled: false });
    // The flash drive has no switch of its own and is not counted.
    expect(pushed).toEqual([{ message: "Schedule for 3 items paused", severity: "success" }]);
    expect(listItems).toHaveBeenCalledTimes(2);

    await press(barButton("Resume schedule"));
    expect(setInclude).toHaveBeenLastCalledWith("plex", true);
    expect(pushed.at(-1)?.message).toBe("Schedule for 3 items is running again");
  });

  it("says a save failed instead of claiming the pause", async () => {
    setInclude.mockResolvedValue({ ok: false, error: "locked" });
    await renderPage();
    await pick("plex");
    await press(barButton("Pause schedule"));
    expect(pushed).toEqual([{ message: en["schedule.updateFailed"], severity: "fail" }]);
  });

  it("restores nothing from here, since several entries come back on the Restore page", async () => {
    await renderPage();
    await pick("plex", "Windows 11");
    expect(within(bar()!).queryByRole("button", { name: /restore/i })).toBeNull();
  });

  it("holds a backup back while something runs, and says what", async () => {
    await renderPage();
    await pick("plex");
    progress({ key: "vm:win11", phase: "backup", percent: 5, active: true });
    expect((barButton("Back up selected") as HTMLButtonElement).disabled).toBe(true);
    expect((barButton("Pause schedule") as HTMLButtonElement).disabled).toBe(false);
  });

  it("ends the picking with Done", async () => {
    await renderPage();
    await pick("plex");
    await press(barButton("Done"));
    expect(bar()).toBeNull();
    expect(picked()).toEqual([]);
  });
});

describe("a right click on a row", () => {
  it("offers what can be done with that entry", async () => {
    await renderPage();
    await rightClick(row("plex"));
    expect(screen.getByRole("menu", { name: "plex" })).toBeTruthy();
    expect(menuItems()).toEqual(["Back up now", "Pause schedule", "Open", "Select"]);
  });

  it("offers a paused entry its schedule back", async () => {
    await renderPage();
    await rightClick(row("gitea"));
    expect(menuItems()).toEqual(["Back up now", "Resume schedule", "Open", "Select"]);
    await act(async () => {
      fireEvent.keyDown(screen.getByRole("menu"), { key: "Escape" });
    });
    expect(screen.queryByRole("menu")).toBeNull();
    await rightClick(row("Documents"));
    expect(menuItems()).toEqual(["Back up now", "Pause schedule", "Open", "Select"]);
  });

  it("offers an entry that is gone no backup, and the flash drive no schedule", async () => {
    await renderPage();
    await rightClick(row("pihole"));
    expect(menuItems()).toEqual(["Pause schedule", "Open", "Select"]);
    await act(async () => {
      fireEvent.keyDown(screen.getByRole("menu"), { key: "Escape" });
    });
    await rightClick(row("Unraid flash"));
    expect(menuItems()).toEqual(["Back up now", "Open", "Select"]);
  });

  it("asks once before a container is stopped for its backup", async () => {
    backupNow.mockResolvedValue({ ok: true, started: true });
    await renderPage();
    await rightClick(row("plex"));
    await chooseFromMenu("Back up now");
    expect(backupNow).not.toHaveBeenCalled();
    expect(screen.getByText(en["containers.stopWarning"])).toBeTruthy();
    await press(within(screen.getByRole("dialog")).getByRole("button", { name: "Back up now" }));
    expect(backupNow).toHaveBeenCalledWith("plex");

    await rightClick(row("plex"));
    await chooseFromMenu("Back up now");
    expect(backupNow).toHaveBeenCalledTimes(2);
  });

  it("backs up a VM without asking and passes a refusal on", async () => {
    backupVMNow.mockResolvedValue({ ok: false, error: "a backup is already running" });
    await renderPage();
    await rightClick(row("Windows 11"));
    await chooseFromMenu("Back up now");
    expect(backupVMNow).toHaveBeenCalledWith("win11");
    expect(pushed).toEqual([{ message: "a backup is already running", severity: "warn" }]);
  });

  it("cancels a running backup after asking", async () => {
    cancelBackup.mockResolvedValue({ ok: true, cancelled: true });
    await renderPage();
    progress({ key: "container:plex", phase: "backup", percent: 20, active: true });
    await rightClick(row("plex"));
    expect(menuItems()[0]).toBe("Cancel backup");
    await chooseFromMenu("Cancel backup");
    await press(within(screen.getByRole("dialog")).getByRole("button", { name: "Confirm" }));
    expect(cancelBackup).toHaveBeenCalledWith("container:plex");
  });

  it("pauses the one entry and names it", async () => {
    setInclude.mockResolvedValue({ ok: true });
    await renderPage();
    await rightClick(row("plex"));
    await chooseFromMenu("Pause schedule");
    expect(setInclude).toHaveBeenCalledWith("plex", false);
    expect(pushed).toEqual([{ message: "Schedule for plex paused", severity: "success" }]);
  });

  it("opens the entry and picks it", async () => {
    await renderPage();
    await rightClick(row("Documents"));
    await chooseFromMenu("Select");
    expect(picked()).toEqual(["Documents"]);
    await rightClick(row("plex"));
    await chooseFromMenu("Open");
    expect(screen.getByText("at /backups/container/plex")).toBeTruthy();
  });

  it("acts on the whole selection when the row is part of it", async () => {
    backupAll.mockResolvedValue({ ok: true });
    backupFilesAll.mockResolvedValue({ ok: true });
    await renderPage();
    await click(row("plex"), { ctrlKey: true });
    await click(row("Documents"), { ctrlKey: true });
    await rightClick(row("plex"));
    expect(screen.getByRole("menu").textContent).toContain("2 selected");
    expect(menuItems()).toEqual(["Back up selected", "Pause schedule", "Clear selection"]);
    await chooseFromMenu("Back up selected");
    expect(backupAll).toHaveBeenCalledWith(["plex"]);
    expect(backupFilesAll).toHaveBeenCalledWith(["f1"]);

    await rightClick(row("Documents"));
    await chooseFromMenu("Clear selection");
    expect(picked()).toEqual([]);
  });

  it("stays on the one row when another one is picked", async () => {
    await renderPage();
    await click(row("plex"), { ctrlKey: true });
    await click(row("Documents"), { ctrlKey: true });
    await rightClick(row("gitea"));
    expect(screen.getByRole("menu", { name: "gitea" })).toBeTruthy();
    expect(menuItems()).toContain("Open");
  });

  it("leaves a finger's long press to the selection", async () => {
    await renderPage();
    await act(async () => {
      fireEvent.pointerDown(row("plex"), { pointerType: "touch" });
      fireEvent.contextMenu(row("plex"));
    });
    expect(screen.queryByRole("menu")).toBeNull();
  });
});

describe("the More menu", () => {
  it("holds the rare actions, and the ZFS connection only while ZFS is on", async () => {
    serve([...MIXED, ...STACK]);
    await renderPage();
    await openMore();
    expect(screen.getByRole("menu", { name: "More" })).toBeTruthy();
    expect(menuItems()).toEqual(["Select", "Discover backups", "Restore stack", "ZFS connection to this server"]);

    cleanup();
    getSettings.mockResolvedValue({ ok: true, settings: { ...ALL_ON, zfsEnabled: false } as Settings });
    serve(MIXED);
    await renderPage();
    await openMore();
    // No stack is backed up here either, so there is none to restore.
    expect(menuItems()).toEqual(["Select", "Discover backups"]);
  });

  it("starts picking with no row picked yet and says how", async () => {
    await renderPage();
    await openMore();
    await chooseFromMenu("Select");
    expect(bar()!.textContent).toContain(en["backups.select.hint"]);
    expect(within(bar()!).queryByRole("button", { name: "Back up selected" })).toBeNull();
    await click(row("plex"));
    expect(picked()).toEqual(["plex"]);
  });

  it("opens the windows for a stack and for the ZFS connection", async () => {
    serve([...MIXED, ...STACK]);
    await renderPage();
    await openMore();
    await chooseFromMenu("Restore stack");
    expect(screen.getByTestId("stack-window").textContent).toBe("immich");
    await openMore();
    await chooseFromMenu("ZFS connection to this server");
    expect(screen.getByTestId("zfs-window")).toBeTruthy();
  });
});

describe("the notes above the list", () => {
  const DATABASE = entry({ key: "immich_postgres", dbTier: "curated" });

  it("explain the database dumps once and lead to the first database", async () => {
    serve([...MIXED, DATABASE, entry({ key: "db-off", dbTier: "curated", dbDumpOff: true })]);
    await renderPage();
    const note = within(screen.getByTestId("dump-intro"));
    expect(note.getByRole("link", { name: "immich_postgres" }).getAttribute("href")).toBe("/backups/container/immich_postgres");
    await press(note.getByRole("button", { name: "Got it" }));
    expect(screen.queryByTestId("dump-intro")).toBeNull();

    cleanup();
    await renderPage();
    expect(screen.queryByTestId("dump-intro")).toBeNull();
  });

  it("keep the dumps to the tiles that show containers, and say nothing while dumps are off", async () => {
    serve([...MIXED, DATABASE]);
    await renderPage("/backups?kind=vm");
    expect(screen.queryByTestId("dump-intro")).toBeNull();
    await press(tile("Containers"));
    expect(screen.getByTestId("dump-intro")).toBeTruthy();

    cleanup();
    serve([...MIXED, { ...DATABASE, dbDumpsGlobalOff: true }]);
    await renderPage();
    expect(screen.queryByTestId("dump-intro")).toBeNull();
  });

  it("show a batch that runs on the server, whoever started it", async () => {
    await renderPage();
    expect(screen.queryByTestId("batch-note")).toBeNull();
    progress({ key: "batch:containers", phase: "backup", percent: 40, active: true });
    expect(screen.getByTestId("batch-note").textContent).toBe("Batch backup running40 %");
    progress({ key: "batch:containers", phase: "backup", percent: 100, active: false });
    expect(screen.queryByTestId("batch-note")).toBeNull();
  });

  it("count the entries of a batch this page started", async () => {
    backupAll.mockResolvedValue({ ok: true });
    serve([entry({ key: "plex" }), entry({ key: "sonarr" }), entry({ key: "radarr" }), entry({ key: "lidarr" })]);
    await renderPage("/backups?kind=container");
    await press(action("Back up all now")!);
    progress({ key: "batch:containers", phase: "backup", percent: 50, active: true });
    expect(screen.getByTestId("batch-note").textContent).toBe("Batch backup running50 %2 of 4");
  });

  it("put a ZFS connection that is down above the ZFS entries, and lead to it", async () => {
    zfsConnection.mockResolvedValue({ ok: true, code: "ssh-auth", target: "root@tower", uriTarget: "", version: "", detail: "" } as never);
    await renderPage("/backups?kind=zfs");
    const note = within(screen.getByTestId("zfs-down"));
    await press(note.getByRole("button", { name: "View connection" }));
    expect(screen.getByTestId("zfs-window")).toBeTruthy();
  });

  it("say nothing about a ZFS connection that works, and do not ask on other tiles", async () => {
    zfsConnection.mockResolvedValue({ ok: true, code: "ok", target: "root@tower", uriTarget: "", version: "2.2", detail: "" } as never);
    await renderPage();
    expect(zfsConnection).not.toHaveBeenCalled();
    await press(tile("ZFS"));
    expect(zfsConnection).toHaveBeenCalledTimes(1);
    expect(screen.queryByTestId("zfs-down")).toBeNull();
  });
});

describe("stacks", () => {
  it("are listed under the containers with a way to restore each", async () => {
    serve([...MIXED, ...STACK]);
    await renderPage();
    expect(screen.queryByTestId("stacks")).toBeNull();
    await press(tile("Containers"));
    const stacks = within(screen.getByTestId("stacks"));
    expect(stacks.getByText("2 containers · immich, immich_postgres")).toBeTruthy();
    await press(stacks.getByRole("button", { name: "Restore stack" }));
    expect(screen.getByTestId("stack-window").textContent).toBe("immich");
  });

  it("need two backed-up containers, of which one may be gone", async () => {
    serve([
      entry({ key: "a", stack: "pair" }),
      entry({ key: "b", stack: "pair", installed: false, state: undefined }),
      entry({ key: "c", stack: "fresh" }),
      entry({ key: "d", stack: "fresh", lastBackup: 0, lastRunStatus: "" }),
    ]);
    await renderPage("/backups?kind=container");
    expect(screen.getByTestId("stacks").textContent).toContain("pair");
    expect(screen.getByTestId("stacks").textContent).not.toContain("fresh");
  });
});

describe("what a row knows about a container or VM", () => {
  it("points at the entry it looks renamed from", async () => {
    serve([entry({ key: "radarr", lastBackup: 0, lastRunStatus: "", renameFrom: "radarr-movies", renameReason: "appdata-bind" })]);
    await renderPage();
    const said = within(row("radarr")).getByLabelText(/^Looks like radarr-movies/).getAttribute("aria-label")!;
    expect(said).toBe("Looks like radarr-movies: same appdata folders. Open the entry to take it over.");
  });

  it("names former names and what changed since the backup", async () => {
    serve([
      entry({
        key: "vaultwarden",
        formerNames: ["bitwarden", "bw"],
        changedSinceBackup: [{ field: "port", change: "removed", name: "8920/tcp" }],
      }),
    ]);
    await renderPage();
    const said = within(row("vaultwarden")).getByLabelText(/^Renamed/).getAttribute("aria-label")!;
    expect(said).toBe("Renamed, formerly: bitwarden, bw. Changed since backup: Port 8920/tcp is new.");
  });

  it("says why the last backup failed", async () => {
    serve([entry({ key: "plex", lastRunStatus: "failed", lastRunError: "the repository is locked" })]);
    await renderPage();
    expect(within(row("plex")).getByLabelText(/the repository is locked/)).toBeTruthy();
  });

  it("is found by its image, its address and a former name", async () => {
    serve([
      entry({ key: "plex", image: "linuxserver/plex:latest", ip: "192.168.20.31" }),
      entry({ key: "vaultwarden", formerNames: ["bitwarden"] }),
      entry({ key: "other" }),
    ]);
    await renderPage();
    await press(screen.getByRole("button", { name: "Search" }));
    const field = screen.getByRole("searchbox", { name: "Search" });
    expect(field.getAttribute("placeholder")).toBe("Name, image or IP");
    for (const [query, found] of [
      ["linuxserver", "plex"],
      ["20.31", "plex"],
      ["bitw", "vaultwarden"],
    ]) {
      fireEvent.change(field, { target: { value: query } });
      expect(rowNames()).toEqual([found]);
    }
  });

  it("stands by its address when the list is sorted by it, and shows the address", async () => {
    serve([
      entry({ key: "b", ip: "192.168.20.100" }),
      entry({ key: "a", ip: "192.168.20.9" }),
      entry({ kind: "files", key: "f1", name: "Documents", state: undefined, installed: undefined }),
    ]);
    await renderPage();
    expect(row("a").textContent).not.toContain("192.168.20.9");
    await press(screen.getByRole("button", { name: "Name A to Z" }));
    await press(screen.getByRole("option", { name: "IP" }));
    expect(rowNames()).toEqual(["a", "b", "Documents"]);
    expect(row("a").textContent).toContain("192.168.20.9");
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

  it("keeps the wide list whole, with nothing to load", async () => {
    serve(Array.from({ length: 45 }, (_, i) => entry({ key: `c${String(i).padStart(2, "0")}` })));
    await renderPage();
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
