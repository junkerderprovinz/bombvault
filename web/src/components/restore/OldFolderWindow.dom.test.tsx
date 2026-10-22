// @vitest-environment jsdom
/**
 * Restoring out of the repository of another BombVault instance.
 *
 * The window connects with the other instance's key, lists what the
 * repository holds and restores one entry at a time, with the options its
 * kind needs: a network for a container whose own this server lacks, a
 * dataset or single files of a ZFS tree.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";

import type { ForeignInventory, Snapshot } from "../../lib/api";
import { I18nProvider, countText, en } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";

class NoopEventSource {
  onmessage: ((e: MessageEvent) => void) | null = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

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

const EMPTY: ForeignInventory = { containers: [], vms: [], fileSets: [], zfs: [], dbDumps: [] };
let inventory: ForeignInventory = EMPTY;
let missingNetwork = "";
let localContainers: string[] = [];

const foreignOpen = vi.fn((location: string, key: string, creds?: unknown) => {
  void location;
  void key;
  void creds;
  return Promise.resolve({ ok: true, session: "s1", inventory });
});
const foreignClose = vi.fn((session: string) => {
  void session;
  return Promise.resolve({ ok: true });
});
const foreignRestore = vi.fn((req: unknown) => {
  void req;
  return Promise.resolve({ ok: true, started: true });
});
const checkRestore = vi.fn((req: unknown) => {
  void req;
  return Promise.resolve({ ok: true, ready: true, checks: [{ id: "repository", status: "ok" }], plan: null });
});
const listForeignFiles = vi.fn((...args: unknown[]) => {
  void args;
  return Promise.resolve({ ok: true, files: [{ path: "/mnt/cache/appdata/plex/Library", type: "dir", size: 0 }] });
});

// The kit way opens the same session, and a sealed kit is turned away until
// its identity comes along.
const foreignOpenWithKit = vi.fn((location: string, kit: string, creds?: unknown, identity?: string) => {
  void location;
  void creds;
  if (kit === SEALED && !identity) return Promise.resolve({ ok: false, code: "kit-sealed", error: "sealed" });
  return Promise.resolve({ ok: true, session: "sess-kit", inventory });
});
const SEALED = "-----BEGIN AGE ENCRYPTED FILE-----";

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    listContainers: () => Promise.resolve({ ok: true, containers: localContainers.map((name) => ({ name })) }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    listRuns: () => Promise.resolve({ ok: true, runs: [] }),
    foreignOpen: (location: string, key: string, creds?: unknown) => foreignOpen(location, key, creds),
    foreignOpenWithKit: (...a: unknown[]) => foreignOpenWithKit(...(a as [string, string, unknown, string?])),
    foreignClose: (session: string) => foreignClose(session),
    foreignContainerWarnings: () =>
      Promise.resolve({ ok: true, warnings: [], missingNetwork, networks: missingNetwork ? ["bridge", "proxy"] : [] }),
    foreignRestore: (req: unknown) => foreignRestore(req),
    listForeignFiles: (...a: unknown[]) => listForeignFiles(...a),
    checkRestore: (req: unknown) => checkRestore(req),
  };
});

// The real field opens a folder picker of its own. Here it is its input.
vi.mock("../FolderBrowser", () => ({
  FolderBrowser: ({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) => (
    <input aria-label={label} value={value} onChange={(e) => onChange(e.target.value)} />
  ),
}));

const { OldFolderWindow } = await import("./OldFolderWindow");

function snap(id: string, dataset: string, stamp: string, time: string): Snapshot {
  return { id, time, paths: [`/mnt/${dataset}/.zfs/snapshot/${stamp}`], tags: ["bombvault", "zfs:" + dataset], hostname: "other" };
}

const onClose = vi.fn();

async function open() {
  let view!: ReturnType<typeof render>;
  await act(async () => {
    view = render(
      <I18nProvider>
        <ToastProvider>
          <OldFolderWindow hostMountRoot="/host/user" onClose={onClose} />
        </ToastProvider>
      </I18nProvider>
    );
  });
  return view;
}

async function press(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

const button = (name: string) => screen.getByRole("button", { name }) as HTMLButtonElement;

/** Chooses to type the old instance's key and types one. */
async function typeKey() {
  await press(screen.getByRole("tab", { name: en["oldFolder.viaKey"] }));
  fireEvent.change(screen.getByLabelText(new RegExp(`^${en["recovery.foreignKey"]}`)), { target: { value: "a".repeat(64) } });
}

async function chooseKit(text: string) {
  await act(async () => {
    fireEvent.change(screen.getByTestId("kit-file"), { target: { files: [new File([text], "bombvault-recovery-kit.md")] } });
  });
  await screen.findByText("bombvault-recovery-kit.md");
}

async function connect() {
  const view = await open();
  fireEvent.change(screen.getByLabelText(en["oldFolder.folder"]), { target: { value: "backups/other" } });
  await typeKey();
  await press(button(en["recovery.foreignConnect"]));
  return view;
}

/** Opens the options of the only entry in the repository. */
async function options(): Promise<HTMLElement> {
  await press(button(en["oldFolder.options"]));
  return screen.getByTestId("foreign-item");
}

function choose(row: HTMLElement, label: string, option: string) {
  fireEvent.click(within(row).getByRole("combobox", { name: label }));
  fireEvent.click(screen.getByRole("option", { name: option }));
}

const lastRestore = () => foreignRestore.mock.calls.at(-1)![0] as Record<string, unknown>;

beforeEach(() => {
  inventory = EMPTY;
  missingNetwork = "";
  localContainers = [];
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("connecting", () => {
  it("stays locked until the place and the key are given, and says which is missing", async () => {
    await open();
    expect(button(en["recovery.foreignConnect"]).disabled).toBe(true);
    fireEvent.change(screen.getByLabelText(en["oldFolder.folder"]), { target: { value: "backups/other" } });
    expect(button(en["recovery.foreignConnect"]).disabled).toBe(true);
    await typeKey();
    expect(button(en["recovery.foreignConnect"]).disabled).toBe(false);
  });

  it("opens the folder with the old instance's recovery kit in place of a typed key", async () => {
    await open();
    fireEvent.change(screen.getByLabelText(en["oldFolder.folder"]), { target: { value: "backups/other" } });
    expect(screen.getByRole("tab", { name: en["oldFolder.viaKit"] }).getAttribute("aria-selected")).toBe("true");
    expect(button(en["recovery.foreignConnect"]).disabled).toBe(true);
    await chooseKit("# BombVault encryption-key recovery kit");
    await press(button(en["recovery.foreignConnect"]));
    expect(foreignOpenWithKit).toHaveBeenCalledWith("backups/other", "# BombVault encryption-key recovery kit", undefined, "");
    expect(foreignOpen).not.toHaveBeenCalled();
    expect(await screen.findByText(en["recovery.foreignConnected"])).toBeTruthy();
  });

  it("asks for the age identity of a sealed kit and sends it once", async () => {
    await open();
    fireEvent.change(screen.getByLabelText(en["oldFolder.folder"]), { target: { value: "backups/other" } });
    await chooseKit(SEALED);
    expect(document.querySelector('input[type="password"]')).toBeNull();
    await press(button(en["recovery.foreignConnect"]));
    expect(await screen.findByText(en["emergency.code.kit-sealed"])).toBeTruthy();
    const identity = document.querySelector<HTMLInputElement>('input[type="password"]')!;
    fireEvent.change(identity, { target: { value: "AGE-SECRET-KEY-1ABC" } });
    await press(button(en["recovery.foreignConnect"]));
    expect(foreignOpenWithKit).toHaveBeenLastCalledWith("backups/other", SEALED, undefined, "AGE-SECRET-KEY-1ABC");
    expect(await screen.findByText(en["recovery.foreignConnected"])).toBeTruthy();
    expect(document.body.innerHTML).not.toContain("AGE-SECRET-KEY-1ABC");
  });

  it("says in a sentence why a kit was turned away", async () => {
    foreignOpenWithKit.mockResolvedValueOnce({ ok: false, code: "kit-no-key", error: "this recovery kit names no key" } as never);
    await open();
    fireEvent.change(screen.getByLabelText(en["oldFolder.folder"]), { target: { value: "backups/other" } });
    await chooseKit("an old kit");
    await press(button(en["recovery.foreignConnect"]));
    expect(await screen.findByText(en["emergency.code.kit-no-key"])).toBeTruthy();
  });

  it("sends no credentials for a mounted folder", async () => {
    await connect();
    expect(foreignOpen).toHaveBeenCalledWith("backups/other", "a".repeat(64), undefined);
    expect(screen.getByText(en["recovery.foreignConnected"])).toBeTruthy();
  });

  it("sends the other server's credentials for a remote address", async () => {
    await open();
    await press(screen.getByRole("tab", { name: en["oldFolder.remote"] }));
    fireEvent.change(screen.getByRole("textbox", { name: en["oldFolder.remote"] }), { target: { value: "s3:https://s3.example/bucket" } });
    await typeKey();
    expect(button(en["recovery.foreignConnect"]).disabled).toBe(true);
    fireEvent.change(screen.getByLabelText(en["recovery.foreignS3KeyId"]), { target: { value: "AKIA" } });
    await press(button(en["recovery.foreignConnect"]));
    expect(foreignOpen.mock.calls[0][2]).toMatchObject({ s3KeyId: "AKIA", s3Secret: "" });
  });

  it("shows what the server said when it refuses", async () => {
    foreignOpen.mockResolvedValueOnce({ ok: false, error: "wrong key" } as never);
    await connect();
    expect(screen.getByText("wrong key")).toBeTruthy();
    expect(screen.queryByText(en["recovery.foreignConnected"])).toBeNull();
  });

  it("closes the session when it disconnects and when the window goes", async () => {
    const view = await connect();
    await press(button(en["recovery.foreignClose"]));
    expect(foreignClose).toHaveBeenCalledWith("s1");
    expect(button(en["recovery.foreignConnect"])).toBeTruthy();

    await press(button(en["recovery.foreignConnect"]));
    view.unmount();
    expect(foreignClose).toHaveBeenCalledTimes(2);
  });
});

describe("a repository that holds only database dumps", () => {
  it("is not called empty", async () => {
    inventory = { ...EMPTY, dbDumps: [{ name: "immich_postgres", snapshots: [] }] };
    await connect();
    expect(screen.queryByText(en["recovery.foreignEmpty"])).toBeNull();
    expect(screen.getByText(countText(en["recovery.foreignDbDumps"], "en", 1))).toBeTruthy();
  });
});

describe("a container", () => {
  beforeEach(() => {
    inventory = { ...EMPTY, containers: [{ name: "web", snapshots: [] }] };
  });

  it("restores from its row with what the server defaults to", async () => {
    await connect();
    await press(button(en["recovery.foreignRestore"]));
    expect(lastRestore()).toMatchObject({ session: "s1", domain: "containers", item: "web", snapshot: "latest", confirm: true });
    expect(lastRestore().target).toBeUndefined();
  });

  it("asks before it overwrites one this server has", async () => {
    localContainers = ["web"];
    await connect();
    await press(button(en["recovery.foreignRestore"]));
    expect(foreignRestore).not.toHaveBeenCalled();
    const ask = screen.getAllByRole("dialog").at(-1)!;
    expect(within(ask).getByText(en["recovery.foreignExistsConfirm"].replace("{name}", "web"))).toBeTruthy();
    await press(within(ask).getByRole("button", { name: en["common.confirm"] }));
    expect(foreignRestore).toHaveBeenCalledTimes(1);
  });

  it("goes onto bridge when this server lacks its network", async () => {
    missingNetwork = "br0.20";
    await connect();
    const row = await options();
    expect(within(row).getByText("br0.20")).toBeTruthy();
    await press(button(en["recovery.foreignRestore"]));
    expect(checkRestore.mock.calls[0][0]).toMatchObject({ kind: "foreign", name: "web", network: "bridge" });
    expect(lastRestore()).toMatchObject({ network: "bridge" });
  });

  it("goes onto the network that is picked in its place", async () => {
    missingNetwork = "br0.20";
    await connect();
    const row = await options();
    choose(row, en["recovery.foreignNetwork"], "proxy");
    await press(button(en["recovery.foreignRestore"]));
    expect(lastRestore()).toMatchObject({ network: "proxy" });
  });

  it("is asked nothing about a network this server has", async () => {
    await connect();
    const row = await options();
    expect(within(row).queryByRole("combobox", { name: en["recovery.foreignNetwork"] })).toBeNull();
    await press(button(en["recovery.foreignRestore"]));
    expect(lastRestore().network).toBeUndefined();
  });

  it("keeps its options when the list comes back", async () => {
    await connect();
    const row = await options();
    await press(within(row).getByRole("switch", { name: en["recovery.foreignOverwrite"] }));
    await press(button(en["common.back"]));
    await press(button(en["recovery.foreignRestore"]));
    expect(lastRestore()).toMatchObject({ overwrite: true });
  });
});

describe("an entry the repository has no definition of", () => {
  it("says where the definition is and restores its files into a folder", async () => {
    inventory = { ...EMPTY, containers: [{ name: "web", snapshots: [], noDefinition: true }] };
    await connect();
    expect(document.body.textContent).toContain(en["recovery.foreignNoDefinition"].replaceAll("{name}", "web"));
    expect(button(en["recovery.foreignRestore"]).disabled).toBe(true);
    const row = await options();
    expect(within(row).queryByRole("switch", { name: en["recovery.foreignOverwrite"] })).toBeNull();
    fireEvent.change(within(row).getByLabelText(en["recovery.foreignTargetFolder"]), { target: { value: "restore/web" } });
    await press(button(en["recovery.foreignRestore"]));
    expect(lastRestore()).toMatchObject({ domain: "containers", item: "web", target: "restore/web", toFolder: true });
  });

  it("asks for a folder for a VM too, with none filled in", async () => {
    inventory = { ...EMPTY, vms: [{ name: "win11", snapshots: [], noDefinition: true }] };
    await connect();
    expect(document.body.textContent).toContain(en["recovery.foreignNoDefinition"].replaceAll("{name}", "win11"));
    const row = await options();
    expect((within(row).getByLabelText(en["recovery.foreignTargetFolder"]) as HTMLInputElement).value).toBe("");
  });
});

describe("a dataset tree", () => {
  beforeEach(() => {
    inventory = {
      ...EMPTY,
      zfs: [
        {
          name: "cache/appdata",
          snapshots: [
            snap("a1a1a1a1a1", "cache/appdata", "bombvault-20260901T030000Z", "2026-09-01T03:00:00Z"),
            snap("b1b1b1b1b1", "cache/appdata/plex", "bombvault-20260901T030000Z", "2026-09-01T03:00:05Z"),
            snap("a2a2a2a2a2", "cache/appdata", "bombvault-20260902T030000Z", "2026-09-02T03:00:00Z"),
            snap("b2b2b2b2b2", "cache/appdata/plex", "bombvault-20260902T030000Z", "2026-09-02T03:00:05Z"),
          ],
        },
      ],
    };
  });

  const folder = (row: HTMLElement) => within(row).getByLabelText(en["recovery.foreignTargetFolder"]);

  it("needs a folder and restores every dataset into it", async () => {
    await connect();
    expect(button(en["recovery.foreignRestore"]).disabled).toBe(true);
    const row = await options();
    fireEvent.change(folder(row), { target: { value: "restore/other" } });
    await press(button(en["recovery.foreignRestore"]));
    expect(lastRestore()).toMatchObject({ domain: "zfs", item: "cache/appdata", snapshot: "latest", target: "restore/other", wholeTree: true });
  });

  it("offers each run of the whole tree once", async () => {
    await connect();
    fireEvent.click(screen.getByRole("combobox", { name: en["recovery.foreignLatest"] }));
    // The newest one and one per run, not one per snapshot of a dataset.
    expect(screen.getAllByRole("option")).toHaveLength(3);
  });

  it("restores one dataset from its own snapshots", async () => {
    await connect();
    const row = await options();
    choose(row, en["zfs.restore.dataset"], "cache/appdata/plex");
    fireEvent.click(within(row).getByRole("combobox", { name: en["recovery.foreignLatest"] }));
    const offered = screen.getAllByRole("option").map((o) => o.textContent ?? "");
    expect(offered).toHaveLength(3);
    expect(offered.slice(1).every((label) => label.includes("b2b2b2b2") || label.includes("b1b1b1b1"))).toBe(true);
    fireEvent.click(screen.getByRole("option", { name: new RegExp("b1b1b1b1") }));

    fireEvent.change(folder(row), { target: { value: "restore/plex" } });
    await press(button(en["recovery.foreignRestore"]));
    expect(lastRestore()).toMatchObject({ item: "cache/appdata/plex", snapshot: "b1b1b1b1b1", target: "restore/plex", wholeTree: false });
  });

  it("restores picked files of one dataset", async () => {
    await connect();
    const row = await options();
    choose(row, en["zfs.restore.dataset"], "cache/appdata/plex");
    await press(within(row).getByRole("switch", { name: en["zfs.restore.selectFiles"] }));
    expect(listForeignFiles).toHaveBeenCalledWith("s1", "cache/appdata/plex", "latest", "zfs");

    fireEvent.click(within(row).getByRole("checkbox", { name: "/mnt" }));
    fireEvent.change(folder(row), { target: { value: "restore/plex" } });
    await press(button(en["recovery.foreignRestore"]));
    expect(lastRestore()).toMatchObject({ item: "cache/appdata/plex", paths: ["/mnt"], wholeTree: false });
  });
});
