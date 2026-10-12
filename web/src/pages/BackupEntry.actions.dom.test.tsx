// @vitest-environment jsdom
/**
 * What is done to an entry from its page, kind by kind.
 *
 * Pausing writes the flag the schedule switch shows, so the corner, the badge
 * and the switch have to follow each other. An entry whose container or VM is
 * gone can be taken out of the schedule and removed, with or without its
 * backups. A renamed one can take its old entry over. A VM is always sent by
 * its libvirt name. And a read that answers late must not undo what a newer
 * one brought.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";

import type { BackupItem, Container, FileSetView, VM } from "../lib/api";
import { ENTRY_ROUTE } from "../lib/backupEntry";
import { I18nProvider, en } from "../lib/i18n";
import { placementChanged } from "../lib/placementEvents";
import { AnomalyProvider } from "../lib/useAnomalies";
import { DESKTOP_QUERY } from "../lib/useMediaQuery";
import { reposChanged } from "../lib/useNamedRepos";
import {
  FLASH,
  GITEA,
  IMMICH,
  PHOTOS,
  PIHOLE,
  PLACEMENT,
  SET_ID,
  WIN11,
  containerView,
  entryItem,
  fileSetView,
  placementOptions,
  serveEntries,
  vmView,
  type Staged,
} from "./backupEntry.testsupport";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getBackupItem: vi.fn(),
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
    getTimeline: vi.fn(),
    getPlacementOptions: vi.fn(),
    getContainerMounts: vi.fn(),
    listSnapshots: vi.fn(),
    listVMSnapshots: vi.fn(),
    setInclude: vi.fn(),
    setVMInclude: vi.fn(),
    patchFileSet: vi.fn(),
    setVMMethod: vi.fn(),
    deleteBackups: vi.fn(),
    deleteBackupsVM: vi.fn(),
    deleteFileSetBackups: vi.fn(),
    forgetContainer: vi.fn(),
    forgetVM: vi.fn(),
    takeOverContainer: vi.fn(),
    takeOverVM: vi.fn(),
    setBackupDeclined: vi.fn(),
  };
});

const pushed: { message: string; action?: { label: string; onClick: () => void } }[] = [];
vi.mock("../lib/toast", () => ({
  useToast: () => ({
    push: (message: string, _severity?: string, action?: { label: string; onClick: () => void }) =>
      pushed.push({ message, action }),
  }),
}));

const api = await import("../lib/api");
const mocks = {
  getBackupItem: vi.mocked(api.getBackupItem),
  listContainers: vi.mocked(api.listContainers),
  listVMs: vi.mocked(api.listVMs),
  listFileSets: vi.mocked(api.listFileSets),
  listZFSDatasets: vi.mocked(api.listZFSDatasets),
  listItemRuns: vi.mocked(api.listItemRuns),
  getSettings: vi.mocked(api.getSettings),
};
const setInclude = vi.mocked(api.setInclude);
const setVMInclude = vi.mocked(api.setVMInclude);
const patchFileSet = vi.mocked(api.patchFileSet);
const setVMMethod = vi.mocked(api.setVMMethod);
const deleteBackups = vi.mocked(api.deleteBackups);
const deleteBackupsVM = vi.mocked(api.deleteBackupsVM);
const deleteFileSetBackups = vi.mocked(api.deleteFileSetBackups);
const forgetContainer = vi.mocked(api.forgetContainer);
const forgetVM = vi.mocked(api.forgetVM);
const takeOverContainer = vi.mocked(api.takeOverContainer);
const takeOverVM = vi.mocked(api.takeOverVM);
const setBackupDeclined = vi.mocked(api.setBackupDeclined);

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

class NoopEventSource {
  onmessage: unknown = null;
  close() {}
}

function Elsewhere() {
  const { pathname, search } = useLocation();
  return <p>{`at ${pathname}${search}`}</p>;
}

async function renderPage(path: string) {
  await act(async () => {
    render(
      <I18nProvider>
        <MemoryRouter initialEntries={[path]}>
          <AnomalyProvider>
            <Routes>
              <Route path={ENTRY_ROUTE} element={<BackupEntryPage />} />
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
const card = (title: string) => screen.getByRole("heading", { level: 2, name: new RegExp(`^${title}`) }).parentElement!;
const scheduleSwitch = () => screen.getByRole("switch", { name: en["entry.settings.inSchedule"] });

async function confirmWith(name: string) {
  await press(within(await screen.findByRole("dialog")).getByRole("button", { name }));
}

/** The actions beside the entry's name. */
async function openMenu() {
  return within(document.querySelector<HTMLElement>('[data-new="entry-menu"]')!);
}

async function choose(action: string) {
  const menu = await openMenu();
  await press(menu.getByRole("button", { name: action }));
}

/** The entries the link window offers to take over. */
async function linkable() {
  await choose(en["entry.menu.link"]);
  const window = screen.getByRole("dialog", { name: en["entry.menu.link"] });
  return within(window)
    .getAllByRole("listitem")
    .map((row) => row.querySelector("span")?.textContent)
    .sort();
}

// The display name and the libvirt name differ, as on TrueNAS, and only the
// libvirt name reaches a route.
const OLD_VM_KEY = "1_windows-10";
const OLD_VM = entryItem({ kind: "vm", key: OLD_VM_KEY, name: "Windows 10", installed: false, state: undefined });

function goneVM(over: Partial<VM> = {}): VM {
  return vmView({ name: "Windows 10", libvirtName: OLD_VM_KEY, state: "not-installed", ...over });
}

beforeEach(() => {
  desktop = true;
  pushed.length = 0;
  localStorage.clear();
  vi.stubGlobal("EventSource", NoopEventSource);
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => ({ ok: true, json: async () => ({ ok: false }) }))
  );
  serveEntries(mocks);
  vi.mocked(api.listRuns).mockResolvedValue({ ok: true, runs: [] });
  vi.mocked(api.getAnomalies).mockResolvedValue({ ok: true, anomalies: [], nextCursor: "" });
  vi.mocked(api.getAnomalySummary).mockResolvedValue({ ok: false });
  vi.mocked(api.getAnomalyItems).mockResolvedValue({ ok: true, items: [] });
  vi.mocked(api.getTimeline).mockResolvedValue({ ok: true, places: [], rows: [] });
  vi.mocked(api.getPlacementOptions).mockImplementation(async (domain) => ({ ok: true, options: placementOptions(domain) }));
  vi.mocked(api.getContainerMounts).mockResolvedValue({ ok: false } as never);
  vi.mocked(api.listSnapshots).mockResolvedValue({ ok: true, snapshots: [] });
  vi.mocked(api.listVMSnapshots).mockResolvedValue({ ok: true, snapshots: [] });
  for (const call of [
    setInclude,
    setVMInclude,
    patchFileSet,
    setVMMethod,
    deleteBackups,
    deleteBackupsVM,
    deleteFileSetBackups,
    forgetContainer,
    forgetVM,
    takeOverContainer,
    takeOverVM,
  ]) {
    call.mockResolvedValue({ ok: true } as never);
  }
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

interface Pausable {
  what: string;
  path: string;
  /** What the server holds for the entry while it is in or out of the schedule. */
  staged: (included: boolean) => Staged;
  save: typeof setInclude | typeof patchFileSet;
  /** The arguments the save takes for the new value. */
  sent: (included: boolean) => unknown[];
}

const PAUSABLE: Pausable[] = [
  {
    what: "a container",
    path: "/backups/container/immich",
    staged: (included) => ({
      items: [{ ...IMMICH, included }],
      containers: [containerView({ includeInSchedule: included })],
    }),
    save: setInclude,
    sent: (included) => ["immich", included],
  },
  {
    what: "a VM",
    path: "/backups/vm/win11",
    staged: (included) => ({ items: [{ ...WIN11, included }], vms: [vmView({ includeInSchedule: included })] }),
    save: setVMInclude as unknown as typeof setInclude,
    sent: (included) => ["win11", included],
  },
  {
    what: "a folder set",
    path: `/backups/files/${SET_ID}`,
    staged: (included) => ({ items: [{ ...PHOTOS, included }], fileSets: [fileSetView({ enabled: included })] }),
    save: patchFileSet,
    sent: (included) => [SET_ID, { enabled: included }],
  },
];

describe.each(PAUSABLE)("pausing $what", ({ path, staged, save, sent }) => {
  // The save changes what the next read brings, as on the server.
  function serve(included: boolean) {
    serveEntries(mocks, staged(included));
    save.mockImplementation((async (_key: string, value: boolean | { enabled?: boolean }) => {
      serveEntries(mocks, staged(typeof value === "boolean" ? value : value.enabled === true));
      return { ok: true };
    }) as never);
  }

  it("takes it out of the schedule, marks it paused and offers Resume", async () => {
    serve(true);
    await renderPage(path);
    expect(screen.queryByText(en["schedule.paused"])).toBeNull();
    await press(corner().getByRole("button", { name: en["schedule.pause"] }));
    expect(save).toHaveBeenCalledWith(...sent(false));
    expect(await screen.findByText(en["schedule.paused"])).toBeTruthy();
    expect(corner().getByRole("button", { name: en["schedule.resume"] })).toBeTruthy();
  });

  it("puts it back with Resume", async () => {
    serve(false);
    await renderPage(path);
    expect(screen.getByText(en["schedule.paused"])).toBeTruthy();
    await press(corner().getByRole("button", { name: en["schedule.resume"] }));
    expect(save).toHaveBeenCalledWith(...sent(true));
    await waitFor(() => expect(screen.queryByText(en["schedule.paused"])).toBeNull());
    expect(corner().getByRole("button", { name: en["schedule.pause"] })).toBeTruthy();
  });

  it("shows on the switch of the Settings tab, and the corner follows that switch", async () => {
    serve(false);
    await renderPage(`${path}?tab=settings`);
    expect(scheduleSwitch().getAttribute("aria-checked")).toBe("false");
    await press(scheduleSwitch());
    expect(save).toHaveBeenCalledWith(...sent(true));
    expect(await corner().findByRole("button", { name: en["schedule.pause"] })).toBeTruthy();
    expect(scheduleSwitch().getAttribute("aria-checked")).toBe("true");
    expect(screen.queryByText(en["schedule.paused"])).toBeNull();
  });

  it("leaves Back up now available", async () => {
    serve(false);
    await renderPage(path);
    const backup = corner().getByRole("button", { name: en["containers.backupNow"] }) as HTMLButtonElement;
    expect(backup.disabled).toBe(false);
  });

  it("works the same on a phone", async () => {
    desktop = false;
    serve(true);
    await renderPage(path);
    await press(corner().getByRole("button", { name: en["schedule.pause"] }));
    expect(save).toHaveBeenCalledWith(...sent(false));
    expect(await screen.findByText(en["schedule.paused"])).toBeTruthy();
  });
});

describe("a container that is gone", () => {
  const without = (): Staged => ({
    items: [{ ...PIHOLE, included: true, lastBackup: 0 }],
    containers: [containerView({ name: "pihole", installed: false, state: "", lastBackup: null })],
  });

  it("keeps the schedule switch, which ends the skipped runs", async () => {
    serveEntries(mocks, without());
    await renderPage("/backups/container/pihole?tab=settings");
    expect(scheduleSwitch().getAttribute("aria-checked")).toBe("true");
    await press(scheduleSwitch());
    expect(setInclude).toHaveBeenCalledWith("pihole", false);
  });

  it("offers Remove entry, not Delete all backups, while it has no backups", async () => {
    serveEntries(mocks, without());
    await renderPage("/backups/container/pihole");
    const menu = await openMenu();
    expect(menu.queryByRole("button", { name: en["containers.deleteBackups"] })).toBeNull();
    await press(menu.getByRole("button", { name: en["vms.removeEntry"] }));
    await confirmWith(en["vms.removeEntry"]);
    await waitFor(() => expect(forgetContainer).toHaveBeenCalledWith("pihole"));
    expect(deleteBackups).not.toHaveBeenCalled();
    expect(await screen.findByText("at /backups?kind=container")).toBeTruthy();
  });

  it("offers Delete all backups, not Remove entry, once it has backups", async () => {
    await renderPage("/backups/container/pihole");
    const menu = await openMenu();
    expect(menu.queryByRole("button", { name: en["vms.removeEntry"] })).toBeNull();
    await press(menu.getByRole("button", { name: en["containers.deleteBackups"] }));
    await confirmWith(en["containers.deleteBackups"]);
    await waitFor(() => expect(deleteBackups).toHaveBeenCalledWith("pihole"));
    expect(forgetContainer).not.toHaveBeenCalled();
  });

  it("is offered no entry to take over", async () => {
    serveEntries(mocks, {
      containers: [
        containerView({ name: "pihole", installed: false, state: "", renameFrom: "pihole-old", renameReason: "docker-id" }),
      ],
    });
    await renderPage("/backups/container/pihole");
    expect(screen.queryByText(en["takeover.looksLike"].replace("{old}", "pihole-old"))).toBeNull();
    expect((await openMenu()).queryByRole("button", { name: en["entry.menu.link"] })).toBeNull();
  });
});

describe("a VM that is gone", () => {
  const path = `/backups/vm/${OLD_VM_KEY}`;

  it("keeps the schedule switch and saves it under the libvirt name", async () => {
    serveEntries(mocks, { items: [OLD_VM], vms: [goneVM()] });
    await renderPage(`${path}?tab=settings`);
    await press(scheduleSwitch());
    expect(setVMInclude).toHaveBeenCalledWith(OLD_VM_KEY, false);
  });

  it("offers Remove entry, not Delete all backups, while it has no backups", async () => {
    serveEntries(mocks, { items: [{ ...OLD_VM, lastBackup: 0 }], vms: [goneVM({ lastBackup: null })] });
    await renderPage(path);
    const menu = await openMenu();
    expect(menu.queryByRole("button", { name: en["containers.deleteBackups"] })).toBeNull();
    await press(menu.getByRole("button", { name: en["vms.removeEntry"] }));
    await confirmWith(en["vms.removeEntry"]);
    await waitFor(() => expect(forgetVM).toHaveBeenCalledWith(OLD_VM_KEY));
    expect(deleteBackupsVM).not.toHaveBeenCalled();
    expect(await screen.findByText("at /backups?kind=vm")).toBeTruthy();
  });

  it("offers Delete all backups, not Remove entry, once it has backups", async () => {
    serveEntries(mocks, { items: [OLD_VM], vms: [goneVM()] });
    await renderPage(path);
    const menu = await openMenu();
    expect(menu.queryByRole("button", { name: en["vms.removeEntry"] })).toBeNull();
    await press(menu.getByRole("button", { name: en["containers.deleteBackups"] }));
    await confirmWith(en["containers.deleteBackups"]);
    await waitFor(() => expect(deleteBackupsVM).toHaveBeenCalledWith(OLD_VM_KEY, "local"));
    expect(forgetVM).not.toHaveBeenCalled();
  });
});

describe("a VM", () => {
  it("shows both backup methods with the stored one chosen, and saves the other under the libvirt name", async () => {
    serveEntries(mocks, { vms: [vmView({ method: "graceful" })] });
    await renderPage("/backups/vm/win11?tab=settings");
    const method = within(card(en["entry.settings.vm"])).getByRole("tablist", { name: en["vm.method"] });
    expect(within(method).queryByRole("combobox")).toBeNull();
    const graceful = within(method).getByRole("tab", { name: en["vm.method.graceful"] });
    const live = within(method).getByRole("tab", { name: en["vm.method.live"] });
    expect(graceful.getAttribute("aria-selected")).toBe("true");
    expect(live.getAttribute("aria-selected")).toBe("false");
    await press(live);
    expect(setVMMethod).toHaveBeenCalledWith("win11", "live");
  });

  it("reads its backups under the libvirt name", async () => {
    await renderPage("/backups/vm/win11");
    expect(api.getTimeline).toHaveBeenCalledWith("vms", "win11");
  });

  it("deletes its local backups after the question that names them", async () => {
    await renderPage("/backups/vm/win11");
    await choose(en["entry.deleteLocal"]);
    expect(within(await screen.findByRole("dialog")).getByText(en["snapshots.deleteAllConfirm"])).toBeTruthy();
    await confirmWith(en["snapshots.deleteAll"]);
    await waitFor(() => expect(deleteBackupsVM).toHaveBeenCalledWith("win11", "local"));
  });

  it("reads its backups again once the local ones are deleted", async () => {
    await renderPage("/backups/vm/win11");
    const reads = vi.mocked(api.getTimeline).mock.calls.length;
    await choose(en["entry.deleteLocal"]);
    await confirmWith(en["snapshots.deleteAll"]);
    await waitFor(() => expect(vi.mocked(api.getTimeline).mock.calls.length).toBeGreaterThan(reads));
  });
});

describe("a folder set", () => {
  it("reads its backups by its id", async () => {
    await renderPage(`/backups/files/${SET_ID}`);
    expect(api.getTimeline).toHaveBeenCalledWith("files", SET_ID);
  });

  it("deletes its local backups after the way that names them is taken, and leaves for the list", async () => {
    await renderPage(`/backups/files/${SET_ID}`);
    await choose(en["zfs.removeMissing"]);
    const window = within(await screen.findByRole("dialog"));
    expect(window.getByText(en["entry.remove.dropTextLocal"])).toBeTruthy();
    await press(window.getByRole("radio", { name: new RegExp(`^${en["entry.remove.drop"]}`) }));
    await confirmWith(en["entry.remove.dropGo"]);
    await waitFor(() => expect(deleteFileSetBackups).toHaveBeenCalledWith(SET_ID));
    expect(await screen.findByText("at /backups?kind=files")).toBeTruthy();
  });

  it("offers no deleting while it has no backup", async () => {
    serveEntries(mocks, { items: [{ ...PHOTOS, lastBackup: 0 }], fileSets: [fileSetView({ lastBackup: 0 })] });
    await renderPage(`/backups/files/${SET_ID}`);
    await choose(en["zfs.removeMissing"]);
    const window = within(await screen.findByRole("dialog"));
    expect(window.getAllByRole("radio")).toHaveLength(1);
    expect(window.queryByRole("radio", { name: new RegExp(`^${en["entry.remove.drop"]}`) })).toBeNull();
  });
});

describe("taking over the entry of a renamed container", () => {
  const fresh = (over: Partial<Container> = {}) =>
    containerView({ name: "gitea", ip: "", lastBackup: null, lastBackupStarted: null, includeInSchedule: false, ...over });
  const gone = (name: string, over: Partial<Container> = {}) =>
    containerView({ name, installed: false, state: "", ip: "", ...over });

  it("takes the suggested entry over and reads the entry again", async () => {
    serveEntries(mocks, { containers: [fresh({ renameFrom: "gitea-old", renameReason: "appdata-bind" })] });
    await renderPage("/backups/container/gitea");
    const before = mocks.getBackupItem.mock.calls.length;
    await press(await screen.findByRole("button", { name: en["takeover.accept"] }));
    await confirmWith(en["takeover.accept"]);
    await waitFor(() => expect(takeOverContainer).toHaveBeenCalledWith("gitea", "gitea-old"));
    await waitFor(() => expect(mocks.getBackupItem.mock.calls.length).toBeGreaterThan(before));
  });

  it("offers to link by hand the entries whose container is gone, and no other", async () => {
    serveEntries(mocks, { containers: [fresh(), gone("gitea-old"), gone("lidarr-old")] });
    await renderPage("/backups/container/gitea");
    expect(await linkable()).toEqual(["gitea-old", "lidarr-old", "pihole"]);
  });

  it("leaves out an entry whose name is another entry's former name", async () => {
    serveEntries(mocks, {
      containers: [fresh(), containerView({ aliases: ["immich-old"] }), gone("immich-old"), gone("lidarr-old")],
    });
    await renderPage("/backups/container/gitea");
    expect(await linkable()).toEqual(["lidarr-old", "pihole"]);
  });

  it("offers no link to an entry that has backups of its own", async () => {
    await renderPage("/backups/container/immich");
    expect((await openMenu()).queryByRole("button", { name: en["entry.menu.link"] })).toBeNull();
  });

  // An entry rebuilt from the storage carries its former names before its first run.
  it("offers no link to an entry with former names and no backup, and unlinks those on the Settings tab", async () => {
    serveEntries(mocks, { containers: [fresh({ aliases: ["gitea-old"] }), gone("lidarr-old")] });
    await renderPage("/backups/container/gitea?tab=settings");
    expect(button(en["takeover.unlinkName"].replace("{old}", "gitea-old"))).toBeTruthy();
    expect((await openMenu()).queryByRole("button", { name: en["entry.menu.link"] })).toBeNull();
  });
});

describe("taking over the entry of a renamed VM", () => {
  const fresh = (over: Partial<VM> = {}) => vmView({ lastBackup: null, lastBackupStarted: null, ...over });
  const items: BackupItem[] = [{ ...WIN11, lastBackup: 0 }];

  it("sends the libvirt name, never the name on show", async () => {
    serveEntries(mocks, { items, vms: [fresh({ renameFrom: OLD_VM_KEY, renameReason: "libvirt-uuid" })] });
    await renderPage("/backups/vm/win11");
    await press(await screen.findByRole("button", { name: en["takeover.accept"] }));
    await confirmWith(en["takeover.accept"]);
    await waitFor(() => expect(takeOverVM).toHaveBeenCalledWith("win11", OLD_VM_KEY));
    expect(takeOverVM).not.toHaveBeenCalledWith(WIN11.name, expect.anything());
  });

  it("offers to link by hand the VMs that are gone, without one that is another's former name", async () => {
    serveEntries(mocks, {
      items,
      vms: [
        fresh(),
        vmView({ name: "debian", libvirtName: "1_debian", aliases: ["1_debian-old"] }),
        goneVM({ name: "1_debian-old", libvirtName: "1_debian-old" }),
        goneVM(),
      ],
    });
    await renderPage("/backups/vm/win11");
    expect(await linkable()).toEqual([OLD_VM_KEY]);
  });
});

interface Reads {
  /** Answers the read that started nth, counting the page's first as zero. */
  answer(nth: number, res: unknown): void;
  started(): number;
}

function held(list: typeof mocks.listContainers): Reads {
  const waiting: ((res: unknown) => void)[] = [];
  list.mockImplementation(() => new Promise((resolve) => waiting.push(resolve)) as never);
  return { answer: (nth, res) => waiting[nth]?.(res), started: () => waiting.length };
}

const OWN = { ...PLACEMENT, copiesFollow: false, skip: ["t1"] };

describe("a read older than a placement write", () => {
  it.each([
    {
      what: "a container",
      path: "/backups/container/immich",
      list: mocks.listContainers,
      answer: (own: boolean) => ({ ok: true, containers: [containerView(own ? { placement: OWN } : {})] }),
    },
    {
      what: "a VM",
      path: "/backups/vm/win11",
      list: mocks.listVMs,
      answer: (own: boolean) => ({ ok: true, vms: [vmView(own ? { placement: OWN } : {})] }),
    },
    {
      what: "a folder set",
      path: `/backups/files/${SET_ID}`,
      list: mocks.listFileSets,
      answer: (own: boolean) => ({ ok: true, fileSets: [fileSetView(own ? { placement: OWN } : {}) as FileSetView] }),
    },
  ])("does not put $what back on what it saw", async ({ path, list, answer }) => {
    const reads = held(list);
    await renderPage(`${path}?tab=settings`);
    await act(async () => reads.answer(0, answer(false)));
    expect(await screen.findByText(en["placement.followsDefault"])).toBeTruthy();

    // A changed repository starts a read, the placement write starts another,
    // and the first of the two answers last with what it saw before the write.
    act(() => reposChanged());
    act(() => placementChanged());
    await waitFor(() => expect(reads.started()).toBe(3));
    await act(async () => reads.answer(2, answer(true)));
    await act(async () => reads.answer(1, answer(false)));
    expect(screen.getByText(en["placement.ownCopies"])).toBeTruthy();
  });
});

describe("saying that an entry is not to be backed up", () => {
  const NO_SCHEDULE = { kind: "none", spec: "", reason: "excluded" } as const;
  const gitea = (declined: boolean): Staged => ({
    items: [{ ...GITEA, effectiveSchedule: NO_SCHEDULE, backupDeclined: declined }],
  });
  /** The server keeps the answer, so the next read of the entry shows it. */
  const serveAnswer = () =>
    setBackupDeclined.mockImplementation(async (_kind, _key, declined) => {
      serveEntries(mocks, gitea(declined));
      return { ok: true };
    });
  const headLine = (key: "entry.reason.declined" | "entry.reason.pausedNever") => screen.queryByText(en[key], { exact: false });

  it("is offered for an entry no schedule reaches, shows in the head at once and can be undone", async () => {
    serveEntries(mocks, gitea(false));
    serveAnswer();
    await renderPage("/backups/container/gitea");
    expect(headLine("entry.reason.declined")).toBeNull();
    const menu = await openMenu();
    expect(menu.queryByRole("button", { name: en["backups.accept"] })).toBeNull();

    await press(menu.getByRole("button", { name: en["backups.decline"] }));

    expect(setBackupDeclined).toHaveBeenCalledWith("container", "gitea", true);
    await waitFor(() => expect(headLine("entry.reason.declined")).toBeTruthy());
    expect(pushed.at(-1)?.message).toBe(en["backups.declined"].replace("{name}", "gitea"));
    expect(pushed.at(-1)?.action?.label).toBe(en["takeover.undo"]);

    await act(async () => pushed.at(-1)!.action!.onClick());

    expect(setBackupDeclined).toHaveBeenLastCalledWith("container", "gitea", false);
    await waitFor(() => expect(headLine("entry.reason.pausedNever")).toBeTruthy());
  });

  it("can be taken back from the menu of an entry set aside", async () => {
    serveEntries(mocks, gitea(true));
    serveAnswer();
    await renderPage("/backups/container/gitea");
    expect(headLine("entry.reason.declined")).toBeTruthy();
    const menu = await openMenu();
    expect(menu.queryByRole("button", { name: en["backups.decline"] })).toBeNull();

    await press(menu.getByRole("button", { name: en["backups.accept"] }));

    expect(setBackupDeclined).toHaveBeenCalledWith("container", "gitea", false);
    await waitFor(() => expect(headLine("entry.reason.declined")).toBeNull());
    expect(pushed).toEqual([]);
  });

  it("stands before the removals of the menu", async () => {
    serveEntries(mocks, { items: [{ ...PHOTOS, included: false, effectiveSchedule: NO_SCHEDULE }] });
    await renderPage(`/backups/files/${SET_ID}`);
    const names = (await openMenu()).getAllByRole("button").map((entry) => entry.textContent);
    expect(names.slice(-2)).toEqual([en["backups.decline"], en["zfs.removeMissing"]]);
  });

  it("is not offered while a schedule reaches the entry", async () => {
    await renderPage("/backups/container/immich");
    const menu = await openMenu();
    expect(menu.queryByRole("button", { name: en["backups.decline"] })).toBeNull();
    expect(menu.queryByRole("button", { name: en["backups.accept"] })).toBeNull();
  });

  it("is not offered for an entry whose container is gone", async () => {
    serveEntries(mocks, { items: [{ ...PIHOLE, effectiveSchedule: NO_SCHEDULE }] });
    await renderPage("/backups/container/pihole");
    expect((await openMenu()).queryByRole("button", { name: en["backups.decline"] })).toBeNull();
  });

  it("gives the flash drive a menu when nothing else would be in it", async () => {
    serveEntries(mocks, { items: [{ ...FLASH, lastBackup: 0, lastRunStatus: "", effectiveSchedule: NO_SCHEDULE }] });
    await renderPage("/backups/flash/flash");
    const names = (await openMenu()).getAllByRole("button").map((entry) => entry.textContent);
    expect(names).toEqual([en["backups.decline"]]);
  });

  it("says what the server answered when it will not set the entry aside", async () => {
    serveEntries(mocks, gitea(false));
    setBackupDeclined.mockResolvedValue({ ok: false, code: "scheduled", error: "a schedule backs this entry up" });
    await renderPage("/backups/container/gitea");

    await choose(en["backups.decline"]);

    expect(pushed.map((toast) => toast.message)).toEqual(["a schedule backs this entry up"]);
  });
});

describe("BombVault's own container", () => {
  it("is offered neither an entry to take over nor a storage location", async () => {
    serveEntries(mocks, {
      containers: [
        containerView({
          name: "bombvault",
          self: true,
          lastBackup: null,
          renameFrom: "bombvault-old",
          renameReason: "docker-id",
        }),
        containerView({ name: GITEA.key, installed: false, state: "" }),
      ],
    });
    await renderPage("/backups/container/bombvault?tab=settings");
    expect(screen.getByText(en["backups.selfNote"])).toBeTruthy();
    expect(screen.queryByText(en["takeover.looksLike"].replace("{old}", "bombvault-old"))).toBeNull();
    expect(screen.queryByRole("button", { name: en["nav.more"] })).toBeNull();
    expect(screen.queryByText(en["placement.title"])).toBeNull();
  });
});
