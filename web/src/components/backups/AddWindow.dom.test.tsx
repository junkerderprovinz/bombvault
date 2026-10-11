// @vitest-environment jsdom
// The window that adds a folder or a ZFS dataset: what the choice says about
// each kind, what the folder form offers and sends, and that the ZFS form
// stands in the same window.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import type { EntryKind } from "../../lib/backupEntry";
import type { AddedKind } from "../../lib/backupList";
import { DESKTOP_QUERY } from "../../lib/useMediaQuery";
import { renderWithProviders } from "../../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../../lib/placement.testsupport")).createPlacementApi());
const calls = vi.hoisted(() => ({
  browse: vi.fn(),
  listFileSets: vi.fn(),
  getFileSetPreset: vi.fn(),
  zfsHostDatasets: vi.fn(),
}));

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...fake.api,
  ...calls,
  listRepos: () => Promise.resolve({ ok: true, repos: [] }),
}));

const { AddWindow } = await import("./AddWindow");

window.matchMedia = ((query: string) => ({
  matches: query === DESKTOP_QUERY,
  media: query,
  onchange: null,
  addListener: () => {},
  removeListener: () => {},
  addEventListener: () => {},
  removeEventListener: () => {},
  dispatchEvent: () => false,
})) as unknown as typeof window.matchMedia;

const SHARES = ["documents", "media", "my photos"].map((name) => ({ name, path: `user/${name}` }));
const NO_PRESET = { ok: true, offered: false, name: "", path: "", excludes: [] };
const HOST = {
  ok: true,
  available: true,
  code: "ok",
  target: "root@tower",
  datasets: [],
  hiddenLegacy: 0,
  unusedZvols: 0,
  notInItem: 3,
  truncated: false,
  maxNameLength: 200,
};

const onClose = vi.fn();
const onAdded = vi.fn();

async function open(start: AddedKind | "choose", enabled: Partial<Record<EntryKind, boolean>> = { files: true, zfs: true }) {
  await act(async () => {
    renderWithProviders(
      <AddWindow start={start} enabled={enabled} hostMountRoot="/host/user" hostRoot="/mnt" onClose={onClose} onAdded={onAdded} />
    );
  });
}

const choice = (name: RegExp) => screen.getByRole("button", { name });
const found = () => within(screen.getByTestId("found-folders"));

beforeEach(() => {
  fake.reset();
  vi.clearAllMocks();
  localStorage.setItem("bv-lang", "en");
  calls.browse.mockResolvedValue({ ok: true, dirs: SHARES });
  calls.listFileSets.mockResolvedValue({ ok: true, fileSets: [{ id: "f1", name: "Documents", path: "user/documents" }] });
  calls.getFileSetPreset.mockResolvedValue(NO_PRESET);
  calls.zfsHostDatasets.mockResolvedValue(HOST);
});

afterEach(cleanup);

describe("the choice", () => {
  it("says how many shares the server has and how many are backed up already", async () => {
    await open("choose");
    expect(calls.browse).toHaveBeenCalledWith("user");
    expect(choice(/^One folder/).textContent).toContain("3 shares found, you back up 1 of them already");
  });

  it("names the host's folder where the server has no shares to offer", async () => {
    calls.browse.mockResolvedValue({ ok: false, error: "could not read directory" });
    await open("choose");
    expect(choice(/^One folder/).textContent).toContain("Any folder under /mnt");
  });

  it("says how many datasets no item covers, or that the server does not answer", async () => {
    await open("choose");
    expect(choice(/^A ZFS dataset/).textContent).toContain("3 datasets on the host are in no entry");

    cleanup();
    calls.zfsHostDatasets.mockResolvedValue({ ...HOST, available: false, code: "ssh-auth" });
    await open("choose");
    expect(choice(/^A ZFS dataset/).textContent).toContain("The connection to the server is not working right now");
  });

  it("holds back a kind that is switched off, says where to switch it on and does not ask its server", async () => {
    await open("choose", { files: true, zfs: false });
    const zfs = choice(/^A ZFS dataset/) as HTMLButtonElement;
    expect(zfs.disabled).toBe(true);
    expect(zfs.textContent).toContain("The ZFS section is switched off under Settings › General");
    expect(calls.zfsHostDatasets).not.toHaveBeenCalled();
    expect((choice(/^One folder/) as HTMLButtonElement).disabled).toBe(false);
  });

  it("leaves through Cancel", async () => {
    await open("choose");
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});

describe("the folder form", () => {
  it("lists the shares by their path on the host, with all of them as one more", async () => {
    await open("files");
    expect(found().getAllByRole("button").map((b) => b.textContent)).toEqual([
      "documents/mnt/user/documentsAlready backed up",
      "media/mnt/user/media",
      "my photos/mnt/user/my photos",
      "All shares/mnt/user",
    ]);
    expect((found().getByRole("button", { name: /^documents/ }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("fills the name and the folder from a share, with a name a set may carry", async () => {
    await open("files");
    fireEvent.click(found().getByRole("button", { name: /^my photos/ }));
    expect((screen.getByPlaceholderText("e.g. Documents") as HTMLInputElement).value).toBe("My-photos");
    expect((screen.getByPlaceholderText("user/appdata") as HTMLInputElement).value).toBe("user/my photos");
    expect(found().getByRole("button", { name: /^my photos/ }).getAttribute("aria-pressed")).toBe("true");
  });

  it("creates the set, says so, reloads the list and closes", async () => {
    await open("files");
    fireEvent.click(found().getByRole("button", { name: /^media/ }));
    fireEvent.change(screen.getByRole("textbox", { name: /^Exclude patterns/ }), { target: { value: "*.tmp\n\n cache/ " } });
    fireEvent.click(screen.getByRole("switch", { name: "On schedule" }));
    fireEvent.click(screen.getByRole("button", { name: "Create" }));
    await waitFor(() => expect(onClose).toHaveBeenCalledTimes(1));
    expect(fake.callsTo("createFileSet")).toEqual([
      [{ name: "Media", path: "user/media", excludes: ["*.tmp", "cache/"], enabled: false }],
    ]);
    expect(onAdded).toHaveBeenCalledTimes(1);
    expect(screen.getByText("Media created")).toBeTruthy();
  });

  it("names its rows as the entry page does, with the storage locations under their own name", async () => {
    await open("files");
    expect(screen.getByRole("switch", { name: "On schedule" })).toBeTruthy();
    expect(screen.getByText("Storage locations")).toBeTruthy();
    expect(screen.queryByText("Placement")).toBeNull();
  });

  it("keeps the window open and says why when the server refuses the set", async () => {
    fake.reply("createFileSet", { ok: false, error: "invalid file set name" });
    await open("files");
    fireEvent.click(found().getByRole("button", { name: /^media/ }));
    fireEvent.click(screen.getByRole("button", { name: "Create" }));
    await screen.findByText("invalid file set name");
    expect(onClose).not.toHaveBeenCalled();
    expect(onAdded).not.toHaveBeenCalled();
  });

  it("waits for a name and a folder before it can create", async () => {
    await open("files");
    const create = screen.getByRole("button", { name: "Create" }) as HTMLButtonElement;
    expect(create.disabled).toBe(true);
    fireEvent.change(screen.getByPlaceholderText("e.g. Documents"), { target: { value: "Docs" } });
    expect(create.disabled).toBe(true);
    fireEvent.change(screen.getByPlaceholderText("user/appdata"), { target: { value: "user/docs" } });
    expect(create.disabled).toBe(false);
  });

  it("suggests the system configuration where the platform has one, with its name and patterns", async () => {
    calls.browse.mockResolvedValue({ ok: true, dirs: [] });
    calls.getFileSetPreset.mockResolvedValue({ ok: true, offered: true, name: "host-system-config", path: "etc", excludes: ["*.cache"] });
    await open("files");
    fireEvent.click(found().getByRole("button", { name: /^System configuration/ }));
    expect((screen.getByPlaceholderText("e.g. Documents") as HTMLInputElement).value).toBe("host-system-config");
    expect((screen.getByPlaceholderText("user/appdata") as HTMLInputElement).value).toBe("etc");
    expect((screen.getByRole("textbox", { name: /^Exclude patterns/ }) as HTMLTextAreaElement).value).toBe("*.cache");
  });

  it("offers nothing to choose from on a server without shares or a suggestion", async () => {
    calls.browse.mockResolvedValue({ ok: true, dirs: [] });
    await open("files");
    expect(screen.queryByTestId("found-folders")).toBeNull();
    expect(screen.getByPlaceholderText("e.g. Documents")).toBeTruthy();
  });

  it("goes back to the choice", async () => {
    await open("files");
    fireEvent.click(screen.getByRole("button", { name: "Other selection" }));
    expect(choice(/^A ZFS dataset/)).toBeTruthy();
  });
});

describe("the ZFS form", () => {
  it("stands in the same window, with the way back to the choice", async () => {
    await open("choose");
    fireEvent.click(choice(/^A ZFS dataset/));
    expect(await screen.findByRole("dialog", { name: "Add ZFS datasets" })).toBeTruthy();
    expect(screen.getByPlaceholderText("Filter datasets…")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Other selection" }));
    expect(screen.getByRole("dialog", { name: "What else should BombVault back up?" })).toBeTruthy();
  });

  it("can be left while the server does not answer", async () => {
    calls.zfsHostDatasets.mockResolvedValue({ ...HOST, available: false, code: "ssh-auth" });
    await open("zfs");
    await screen.findByText("The datasets could not be read from the server.");
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
