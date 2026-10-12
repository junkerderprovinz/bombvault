// @vitest-environment jsdom
/**
 * Changing how a saved destination is reached.
 *
 * The window shows what the server shows of the connection and sends only
 * what was typed. The server tries the change before it saves, so a refusal
 * has to stay on screen as a sentence that says what to do instead.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import type { Backend, BackendOption, Destination, Provider, Settings, StorageLocation } from "../lib/api";
import { I18nProvider } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";

class NoopEventSource {
  onmessage: unknown = null;
  onerror: unknown = null;
  close() {}
  addEventListener() {}
  removeEventListener() {}
}
(globalThis as unknown as { EventSource: unknown }).EventSource = NoopEventSource;

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getStorageLocation: vi.fn(),
    listOffsiteTargets: vi.fn(),
    listDestinations: vi.fn(),
    updateDestinationConnection: vi.fn(),
    getSettings: vi.fn(),
    getProviders: vi.fn(),
    getCloud: vi.fn(),
    getCloudCredSets: vi.fn(),
    getRcloneSections: vi.fn(),
    draftFolders: vi.fn(),
    draftMakeFolder: vi.fn(),
  };
});

const api = await import("../lib/api");
const getStorageLocation = vi.mocked(api.getStorageLocation);
const updateDestinationConnection = vi.mocked(api.updateDestinationConnection);
const getRcloneSections = vi.mocked(api.getRcloneSections);
const draftFolders = vi.mocked(api.draftFolders);
const draftMakeFolder = vi.mocked(api.draftMakeFolder);

const { StorageLocationPage } = await import("./StorageLocation");

function option(name: string, over: Partial<BackendOption> = {}): BackendOption {
  return { name, help: "", required: false, secret: false, password: false, advanced: false, essential: true, default: "", ...over };
}

const SFTP: Backend = {
  name: "sftp",
  description: "",
  options: [option("host", { required: true }), option("user"), option("pass", { password: true }), option("port", { essential: false })],
};

const S3: Backend = {
  name: "s3",
  description: "",
  options: [option("access_key_id"), option("secret_access_key"), option("region"), option("endpoint")],
};

const PROVIDERS = [
  { id: "storagebox", name: "Hetzner Storage Box", backend: "sftp", group: "server", route: "rclone" },
  { id: "b2", name: "Backblaze B2", backend: "s3", group: "storage", route: "s3" },
] as Provider[];

const KEEP = { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 6, keepYearly: 1 };

function location(over: Partial<StorageLocation> = {}): StorageLocation {
  return {
    id: "destination:box",
    object: "destination",
    kind: "offsite",
    provider: "storagebox",
    backend: "rclone",
    name: "Storage Box",
    where: "rclone:storagebox:bombvault",
    enabled: true,
    offPremises: true,
    sections: [],
    retention: KEEP,
    compression: "auto",
    limitUpload: 0,
    limitDownload: 0,
    protection: { immutable: false, testable: false },
    capacity: {},
    ...over,
  };
}

const B2 = location({
  id: "destination:b2",
  provider: "b2",
  backend: "s3",
  name: "Backblaze B2",
  where: "s3:https://s3.eu-central-003.backblazeb2.com/bv-fotos/server",
  credsRef: "set-b2",
});

async function renderPage(loc: StorageLocation) {
  getStorageLocation.mockResolvedValue({ ok: true, location: loc });
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <MemoryRouter initialEntries={[`/storage/${encodeURIComponent(loc.id)}`]}>
            <Routes>
              <Route path="/storage/:id" element={<StorageLocationPage />} />
            </Routes>
          </MemoryRouter>
        </ToastProvider>
      </I18nProvider>
    );
  });
  await act(async () => {});
}

async function click(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

async function type(el: HTMLElement, value: string) {
  await act(async () => {
    fireEvent.change(el, { target: { value } });
  });
}

async function openWindow(name: string): Promise<HTMLElement> {
  await click(screen.getByRole("button", { name: "Change" }));
  const dialog = screen.getByRole("dialog", { name: `Change the connection of ${name}` });
  await act(async () => {});
  return dialog;
}

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({ ok: true }) }));
  vi.mocked(api.listOffsiteTargets).mockResolvedValue({ ok: true, targets: [] });
  vi.mocked(api.listDestinations).mockResolvedValue({
    ok: true,
    destinations: [{ id: "box", name: "Storage Box" }, { id: "b2", name: "Backblaze B2" }] as Destination[],
  });
  vi.mocked(api.getSettings).mockResolvedValue({ ok: true, settings: {} as Settings, hostMountRoot: "", platform: "" });
  vi.mocked(api.getProviders).mockResolvedValue({ ok: true, providers: PROVIDERS, backends: { sftp: SFTP, s3: S3 } });
  vi.mocked(api.getCloud).mockResolvedValue({ ok: true });
  vi.mocked(api.getCloudCredSets).mockResolvedValue({
    ok: true,
    sets: [{ id: "set-b2", name: "Backblaze B2", s3KeyId: "003abc", s3Region: "eu-central-003", restUser: "", s3StorageClass: "", s3SecretSet: true, restPasswordSet: false }],
  });
  getRcloneSections.mockResolvedValue({
    ok: true,
    sections: [
      {
        name: "storagebox",
        type: "sftp",
        options: [{ name: "host", value: "u123.your-storagebox.de" }, { name: "user", value: "u123" }, { name: "pass", hidden: true }],
      },
    ],
  });
  updateDestinationConnection.mockResolvedValue({ ok: true, warnings: [] });
  draftFolders.mockImplementation(async (d) => ({ ok: true, folders: d.dir === "" ? ["archive", "bombvault"] : [] }));
  draftMakeFolder.mockResolvedValue({ ok: true, dir: "fresh" });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

describe("the connection of an rclone destination", () => {
  it("opens with what the config holds, and keeps a secret out of the fields", async () => {
    await renderPage(location());
    const dialog = await openWindow("Storage Box");

    expect((within(dialog).getByLabelText("Server") as HTMLInputElement).value).toBe("u123.your-storagebox.de");
    expect((within(dialog).getByLabelText("User name") as HTMLInputElement).value).toBe("u123");
    const password = within(dialog).getByLabelText("Password") as HTMLInputElement;
    expect(password.value).toBe("");
    expect(password.placeholder).toBe("saved (leave blank to keep)");
    expect((within(dialog).getByLabelText("Folder") as HTMLInputElement).value).toBe("bombvault");
    // A setting most people leave alone stands in a sub-card of its own.
    expect(within(within(dialog).getByRole("region", { name: "More settings" })).getByLabelText("Port")).toBeTruthy();
    expect(within(dialog).queryByRole("switch", { name: "More settings" })).toBeNull();
  });

  it("has nothing to save until a field differs from what is stored", async () => {
    await renderPage(location());
    const dialog = await openWindow("Storage Box");
    const save = within(dialog).getByRole("button", { name: "Test and save" }) as HTMLButtonElement;
    expect(save.disabled).toBe(true);

    await type(within(dialog).getByLabelText("Server"), "u999.your-storagebox.de");
    expect(save.disabled).toBe(false);
    await type(within(dialog).getByLabelText("Server"), "u123.your-storagebox.de");
    expect(save.disabled).toBe(true);
  });

  it("sends only what was typed and closes once the server took it", async () => {
    await renderPage(location());
    const dialog = await openWindow("Storage Box");

    await type(within(dialog).getByLabelText("Server"), "u999.your-storagebox.de");
    await type(within(dialog).getByLabelText("Password"), "hunter2");
    await click(within(dialog).getByRole("button", { name: "Test and save" }));

    expect(updateDestinationConnection).toHaveBeenCalledWith("box", {
      settings: { host: "u999.your-storagebox.de", pass: "hunter2" },
    });
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByText("Storage Box saved")).toBeTruthy();
  });

  it("sends a changed folder as the folder, not as a setting", async () => {
    await renderPage(location());
    const dialog = await openWindow("Storage Box");

    await type(within(dialog).getByLabelText("Folder"), "/archive/");
    await click(within(dialog).getByRole("button", { name: "Test and save" }));

    expect(updateDestinationConnection).toHaveBeenCalledWith("box", { dir: "/archive/" });
  });

  it("sends a further setting from its sub-card", async () => {
    await renderPage(location());
    const dialog = await openWindow("Storage Box");

    await type(within(dialog).getByLabelText("Port"), "23");
    await click(within(dialog).getByRole("button", { name: "Test and save" }));

    expect(updateDestinationConnection).toHaveBeenCalledWith("box", { settings: { port: "23" } });
  });
});

describe("the folder of a destination", () => {
  it("is picked by walking the remote's folders with the connection in the form", async () => {
    await renderPage(location());
    const dialog = await openWindow("Storage Box");

    await type(within(dialog).getByLabelText("Password"), "hunter2");
    await click(within(dialog).getByRole("button", { name: "Browse folders" }));
    const picker = screen.getByRole("dialog", { name: "Folder" });
    expect(draftFolders).toHaveBeenLastCalledWith({
      provider: "storagebox",
      settings: { host: "u123.your-storagebox.de", user: "u123", pass: "hunter2" },
      dir: "bombvault",
    });
    await click(within(picker).getByRole("button", { name: ".." }));
    await click(within(picker).getByRole("button", { name: "archive" }));
    await click(within(picker).getByRole("button", { name: "Use this folder" }));

    expect((within(dialog).getByLabelText("Folder") as HTMLInputElement).value).toBe("archive");
    await click(within(dialog).getByRole("button", { name: "Test and save" }));
    expect(updateDestinationConnection).toHaveBeenCalledWith("box", { settings: { pass: "hunter2" }, dir: "archive" });
  });

  it("makes a new folder where the walk stands", async () => {
    await renderPage(location());
    const dialog = await openWindow("Storage Box");

    await click(within(dialog).getByRole("button", { name: "Browse folders" }));
    const picker = screen.getByRole("dialog", { name: "Folder" });
    await click(within(picker).getByRole("button", { name: ".." }));
    await type(within(picker).getByLabelText("New folder name"), "fresh");
    await click(within(picker).getByRole("button", { name: "New folder" }));

    expect(draftMakeFolder).toHaveBeenCalledWith(expect.objectContaining({ provider: "storagebox", dir: "", folder: "fresh" }));
    expect(draftFolders).toHaveBeenLastCalledWith(expect.objectContaining({ dir: "fresh" }));
  });

  it("says to type the saved secret again when the walk is refused", async () => {
    draftFolders.mockResolvedValue({ ok: false, error: "ssh: unable to authenticate" });
    await renderPage(location());
    const dialog = await openWindow("Storage Box");

    await click(within(dialog).getByRole("button", { name: "Browse folders" }));
    const picker = screen.getByRole("dialog", { name: "Folder" });
    expect(within(picker).getByText(/ssh: unable to authenticate.*type the saved password or secret key again/)).toBeTruthy();
  });
});

describe("a change the server refuses", () => {
  it("says to add a second location when the place holds backups", async () => {
    updateDestinationConnection.mockResolvedValue({ ok: false, code: "destination-holds-backups", error: "this destination holds backups" });
    await renderPage(location());
    const dialog = await openWindow("Storage Box");

    await type(within(dialog).getByLabelText("Folder"), "elsewhere");
    await click(within(dialog).getByRole("button", { name: "Test and save" }));

    expect(within(dialog).getByText(/The location cannot move while something backs up here.*Add a second storage location/)).toBeTruthy();
    expect(within(dialog).getByRole("button", { name: "Not connected" })).toBeTruthy();
    expect(screen.queryByText("this destination holds backups")).toBeNull();
  });

  it("says to check the entry when the backups do not open through the change", async () => {
    updateDestinationConnection.mockResolvedValue({ ok: false, code: "backups-not-there", error: "the backups do not open" });
    await renderPage(location());
    const dialog = await openWindow("Storage Box");

    await type(within(dialog).getByLabelText("Server"), "typo.example");
    await click(within(dialog).getByRole("button", { name: "Test and save" }));

    expect(within(dialog).getByText(/do not open through the changed connection.*Check what you entered/)).toBeTruthy();
  });

  it("shows the server's own words for a failure without a code, and drops them at the next edit", async () => {
    updateDestinationConnection.mockResolvedValue({ ok: false, error: "ssh: handshake failed" });
    await renderPage(location());
    const dialog = await openWindow("Storage Box");

    await type(within(dialog).getByLabelText("Server"), "typo.example");
    await click(within(dialog).getByRole("button", { name: "Test and save" }));
    expect(within(dialog).getByText("ssh: handshake failed")).toBeTruthy();

    await type(within(dialog).getByLabelText("Server"), "u123.example");
    expect(within(dialog).queryByText("ssh: handshake failed")).toBeNull();
  });
});

describe("the connection of an S3 destination", () => {
  it("reads the endpoint and the bucket off the address and the key off its credential set", async () => {
    await renderPage(B2);
    const dialog = await openWindow("Backblaze B2");

    expect((within(dialog).getByLabelText("Endpoint") as HTMLInputElement).value).toBe("https://s3.eu-central-003.backblazeb2.com");
    expect((within(dialog).getByLabelText("Access key ID") as HTMLInputElement).value).toBe("003abc");
    expect((within(dialog).getByLabelText("Region") as HTMLInputElement).value).toBe("eu-central-003");
    expect((within(dialog).getByLabelText("Secret access key") as HTMLInputElement).placeholder).toBe("saved (leave blank to keep)");
    expect((within(dialog).getByLabelText("Folder") as HTMLInputElement).value).toBe("bv-fotos/server");
  });
});

describe("a location without a connection form", () => {
  it("offers no change for a target that was typed in", async () => {
    await renderPage(location({ id: "target:t1", object: "target", provider: "", backend: "rest", where: "rest:https://nas:8000/tower" }));

    expect(screen.queryByRole("button", { name: "Change" })).toBeNull();
    expect(screen.queryByRole("button", { name: "More" })).toBeNull();
  });
});
