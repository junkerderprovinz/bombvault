// @vitest-environment jsdom
/**
 * Removing a storage location.
 *
 * The window says which sections stop backing up to the place before
 * anything is removed, and sends back exactly what it showed, so the server
 * can refuse a removal whose sections changed in between. Deleting the
 * backups there is a second way with a reading of its own, and it waits for
 * the location's name and for a yes to every entry that loses its only backup.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import type { Destination, ErasureSection, LocationErasure, LocationRemoval, RemovalSection, Settings, StorageLocation } from "../lib/api";
import { en, I18nProvider } from "../lib/i18n";
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
    previewLocationRemoval: vi.fn(),
    removeLocation: vi.fn(),
    previewLocationErasure: vi.fn(),
    eraseLocation: vi.fn(),
    getSettings: vi.fn(),
    getProviders: vi.fn(),
    getCloud: vi.fn(),
    getCloudCredSets: vi.fn(),
    getRcloneSections: vi.fn(),
  };
});

const api = await import("../lib/api");
const getStorageLocation = vi.mocked(api.getStorageLocation);
const previewLocationRemoval = vi.mocked(api.previewLocationRemoval);
const removeLocation = vi.mocked(api.removeLocation);
const previewLocationErasure = vi.mocked(api.previewLocationErasure);
const eraseLocation = vi.mocked(api.eraseLocation);

const { StorageLocationPage } = await import("./StorageLocation");

const KEEP = { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 6, keepYearly: 1 };

const BOX: StorageLocation = {
  id: "destination:box",
  object: "destination",
  kind: "offsite",
  provider: "",
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
};

function part(over: Partial<RemovalSection>): RemovalSection {
  return {
    domain: "containers",
    use: "copy",
    targetId: "t-containers",
    repo: "rclone:storagebox:bombvault/containers",
    primary: false,
    appendOnly: false,
    entries: 7,
    backups: 62,
    listedAt: 1_700_000_000,
    lastCopyAt: 1_700_000_000,
    ...over,
  };
}

const PREVIEW: LocationRemoval = {
  sections: [
    part({ primary: true }),
    part({ domain: "flash", targetId: "t-flash", repo: "rclone:storagebox:bombvault/flash", entries: 1, backups: 1 }),
    part({
      domain: "vms",
      targetId: "t-vms",
      repo: "rclone:storagebox:bombvault/vms",
      listedAt: 0,
      entries: 0,
      backups: 0,
      direct: { repoId: "r1", name: "Storage Box", items: 2, defaultDomains: [], kept: true },
    }),
  ],
};

function repo(over: Partial<ErasureSection>): ErasureSection {
  return { domain: "containers", targetId: "t-containers", repo: "rclone:storagebox:bombvault/containers", backups: 62, entries: 7, ...over };
}

const ERASURE: LocationErasure = {
  removal: PREVIEW,
  sections: [
    repo({}),
    repo({ domain: "flash", targetId: "t-flash", repo: "rclone:storagebox:bombvault/flash", backups: 14, entries: 1, kept: "append-only" }),
    repo({ domain: "vms", targetId: "t-vms", repoId: "r1", repo: "rclone:storagebox:bombvault/vms", backups: 9, entries: 2, kept: "in-use" }),
    repo({ domain: "files", targetId: "t-files", repo: "rclone:storagebox:bombvault/files", backups: 0, entries: 0, kept: "unreadable" }),
    repo({ domain: "zfs", targetId: "", repo: "rclone:storagebox:bombvault/zfs", backups: 6, entries: 1 }),
  ],
  backups: 68,
  entries: 8,
  backupsOnlyThere: 5,
  lastCopies: [
    { domain: "containers", identity: "container:plex", backups: 4 },
    { domain: "zfs", identity: "zfs:tank/media", backups: 1 },
  ],
  key: "erase:box",
};

async function renderPage(loc: StorageLocation = BOX) {
  getStorageLocation.mockResolvedValue({ ok: true, location: loc });
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <MemoryRouter initialEntries={[`/storage/${encodeURIComponent(loc.id)}`]}>
            <Routes>
              <Route path="/storage/:id" element={<StorageLocationPage />} />
              <Route path="/storage" element={<p>the list</p>} />
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

async function openRemoval(name = "Storage Box"): Promise<HTMLElement> {
  await click(screen.getByRole("button", { name: "Remove storage location" }));
  return screen.getByRole("dialog", { name: `Remove ${name}?` });
}

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({ ok: true }) }));
  vi.mocked(api.listOffsiteTargets).mockResolvedValue({ ok: true, targets: [] });
  vi.mocked(api.listDestinations).mockResolvedValue({ ok: true, destinations: [{ id: "box", name: "Storage Box" }] as Destination[] });
  vi.mocked(api.getSettings).mockResolvedValue({ ok: true, settings: {} as Settings, hostMountRoot: "", platform: "" });
  vi.mocked(api.getProviders).mockResolvedValue({ ok: true, providers: [] });
  vi.mocked(api.getCloud).mockResolvedValue({ ok: true });
  vi.mocked(api.getCloudCredSets).mockResolvedValue({ ok: true, sets: [] });
  vi.mocked(api.getRcloneSections).mockResolvedValue({ ok: true, sections: [] });
  previewLocationRemoval.mockResolvedValue({ ok: true, removal: PREVIEW });
  removeLocation.mockResolvedValue({ ok: true });
  previewLocationErasure.mockResolvedValue({ ok: true, erasure: ERASURE });
  eraseLocation.mockResolvedValue({ ok: true, started: true, erasure: ERASURE });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

describe("removing a destination", () => {
  it("says what stays and lists every section that stops backing up there", async () => {
    await renderPage();
    const dialog = await openRemoval();

    expect(previewLocationRemoval).toHaveBeenCalledWith("destination:box");
    expect(within(dialog).getByText("Remove storage location, keep the backups")).toBeTruthy();
    expect(within(dialog).getByText(/The backups stay where they are/)).toBeTruthy();
    const rows = within(dialog).getAllByRole("listitem");
    expect(rows.map((row) => row.textContent)).toEqual([
      expect.stringContaining("Containers"),
      expect.stringContaining("Flash"),
      expect.stringContaining("VMs"),
    ]);
    expect(rows[0].textContent).toContain("rclone:storagebox:bombvault/containers");
    expect(rows[0].textContent).toContain("7 entries · 62 backups");
    expect(rows[1].textContent).toContain("1 entry · 1 backup");
  });

  it("offers two ways and starts on the one that keeps the backups", async () => {
    await renderPage();
    const dialog = await openRemoval();

    const ways = within(dialog).getAllByRole("radio");
    expect(ways.map((way) => way.getAttribute("aria-checked"))).toEqual(["true", "false"]);
    expect(ways[1].textContent).toContain("Remove storage location and delete the backups there");
    expect(previewLocationErasure).not.toHaveBeenCalled();
  });

  it("leaves the counts out for a section whose repository was never listed", async () => {
    await renderPage();
    const dialog = await openRemoval();

    const vms = within(dialog).getAllByRole("listitem")[2];
    expect(vms.textContent).not.toContain("0 entries");
    expect(vms.textContent).toContain("That repository stays as a storage location of its own");
  });

  it("sends back the preview it showed and returns to the list", async () => {
    await renderPage();
    const dialog = await openRemoval();

    await click(within(dialog).getByRole("button", { name: "Remove storage location" }));

    expect(removeLocation).toHaveBeenCalledWith("destination:box", PREVIEW, undefined);
    expect(screen.getByText("the list")).toBeTruthy();
    expect(screen.getByText("Storage Box removed, the backups there stay")).toBeTruthy();
  });

  it("removes nothing when the window is cancelled", async () => {
    await renderPage();
    const dialog = await openRemoval();

    await click(within(dialog).getByRole("button", { name: "Cancel" }));

    expect(removeLocation).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe("Storage Box");
  });

  it("shows the sections as they are when they changed since the preview, and asks again", async () => {
    const now: LocationRemoval = { sections: [PREVIEW.sections[0]] };
    removeLocation.mockResolvedValueOnce({ ok: false, code: "stale", error: "stale", removal: now });
    await renderPage();
    const dialog = await openRemoval();

    await click(within(dialog).getByRole("button", { name: "Remove storage location" }));

    expect(within(dialog).getByText("The numbers changed since they were shown. Check them again.")).toBeTruthy();
    expect(within(dialog).getAllByRole("listitem")).toHaveLength(1);

    await click(within(dialog).getByRole("button", { name: "Remove storage location" }));
    expect(removeLocation).toHaveBeenLastCalledWith("destination:box", now, undefined);
    expect(screen.getByText("the list")).toBeTruthy();
  });

  it("says so for a place no section backs up to", async () => {
    previewLocationRemoval.mockResolvedValue({ ok: true, removal: { sections: [] } });
    await renderPage();
    const dialog = await openRemoval();

    expect(within(dialog).getByText("No section backs up here yet.")).toBeTruthy();
    expect((within(dialog).getByRole("button", { name: "Remove storage location" }) as HTMLButtonElement).disabled).toBe(false);
  });

  it("cannot remove while the preview did not load", async () => {
    previewLocationRemoval.mockResolvedValue({ ok: false, error: "database is locked" });
    await renderPage();
    const dialog = await openRemoval();

    expect(within(dialog).getByText("database is locked")).toBeTruthy();
    expect((within(dialog).getByRole("button", { name: "Remove storage location" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("says to move a section's own folder that lies there first", async () => {
    previewLocationRemoval.mockResolvedValue({ ok: false, error: "server text", code: "own-folder-there" });
    await renderPage();
    const dialog = await openRemoval();

    expect(within(dialog).getByText(en["placementCode.ownFolderThere"])).toBeTruthy();
    expect((within(dialog).getByRole("button", { name: "Remove storage location" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("reads the preview again when its way is picked after a failed one", async () => {
    previewLocationRemoval.mockResolvedValueOnce({ ok: false, error: "database is locked" });
    await renderPage();
    const dialog = await openRemoval();

    await click(within(dialog).getAllByRole("radio")[0]);
    await act(async () => {});

    expect(previewLocationRemoval).toHaveBeenCalledTimes(2);
    expect(within(dialog).queryByText("database is locked")).toBeNull();
    expect((within(dialog).getByRole("button", { name: "Remove storage location" }) as HTMLButtonElement).disabled).toBe(false);
  });
});

describe("removing a destination and deleting the backups there", () => {
  async function openDeletion(): Promise<HTMLElement> {
    const dialog = await openRemoval();
    await click(within(dialog).getAllByRole("radio")[1]);
    await act(async () => {});
    return dialog;
  }
  const erase = (dialog: HTMLElement) => within(dialog).getByRole("button", { name: "Remove and delete" }) as HTMLButtonElement;
  const typeName = (dialog: HTMLElement, name: string) =>
    act(async () => {
      fireEvent.change(within(dialog).getByLabelText("Type Storage Box to confirm"), { target: { value: name } });
    });

  it("reads what would be deleted only once that way is picked", async () => {
    await renderPage();
    const dialog = await openDeletion();

    expect(previewLocationErasure).toHaveBeenCalledTimes(1);
    expect(previewLocationErasure).toHaveBeenCalledWith("destination:box");
    expect(within(dialog).getAllByRole("radio").map((way) => way.getAttribute("aria-checked"))).toEqual(["false", "true"]);
    expect(within(dialog).queryByText("These sections stop backing up here")).toBeNull();
  });

  it("lists every repository with what is deleted from it or why it stays", async () => {
    await renderPage();
    const dialog = await openDeletion();

    const rows = within(within(dialog).getAllByRole("list")[0]).getAllByRole("listitem");
    expect(rows.map((row) => row.textContent)).toEqual([
      expect.stringContaining("Containers"),
      expect.stringContaining("Flash"),
      expect.stringContaining("VMs, direct repository"),
      expect.stringContaining("Folders"),
      expect.stringContaining("ZFS"),
    ]);
    expect(rows[0].textContent).toContain("7 entries · 62 backups");
    expect(rows[1].textContent).toContain("Stays: delete protection refuses the deletion.");
    expect(rows[2].textContent).toContain("Stays: entries still back up straight to it.");
    expect(rows[3].textContent).toContain("Stays: it could not be read.");
    expect(within(rows[4]).getByLabelText(/moved to another storage location and left this repository behind/)).toBeTruthy();
  });

  it("sums what is deleted and says how much of it is nowhere else", async () => {
    await renderPage();
    const dialog = await openDeletion();

    expect(within(dialog).getByText("68 backups · 8 entries · 5 of them are nowhere else")).toBeTruthy();
  });

  it("names the entries that lose their only backup", async () => {
    await renderPage();
    const dialog = await openDeletion();

    const last = within(dialog).getByText("These entries lose their only backup:").parentElement!;
    expect(within(last).getAllByRole("listitem").map((row) => row.textContent)).toEqual(["plexContainers · 4 backups", "tank/mediaZFS · 1 backup"]);
  });

  it("waits for the location's name and for the switch that includes those entries", async () => {
    await renderPage();
    const dialog = await openDeletion();
    expect(erase(dialog).disabled).toBe(true);

    await typeName(dialog, "Storage Box");
    expect(erase(dialog).disabled).toBe(true);

    await click(within(dialog).getByRole("switch", { name: "Delete these as well" }));
    expect(erase(dialog).disabled).toBe(false);

    await typeName(dialog, "storage box");
    expect(erase(dialog).disabled).toBe(true);
  });

  it("needs only the name where no entry loses its only backup", async () => {
    previewLocationErasure.mockResolvedValue({ ok: true, erasure: { ...ERASURE, lastCopies: [] } });
    await renderPage();
    const dialog = await openDeletion();

    expect(within(dialog).queryByRole("switch")).toBeNull();
    await typeName(dialog, "Storage Box");
    expect(erase(dialog).disabled).toBe(false);
  });

  it("sends the removal it read and the names of those entries, and returns to the list", async () => {
    await renderPage();
    const dialog = await openDeletion();
    await typeName(dialog, "Storage Box");
    await click(within(dialog).getByRole("switch", { name: "Delete these as well" }));

    await click(erase(dialog));

    expect(eraseLocation).toHaveBeenCalledWith("destination:box", ERASURE.removal, ["container:plex", "zfs:tank/media"], undefined);
    expect(removeLocation).not.toHaveBeenCalled();
    expect(screen.getByText("the list")).toBeTruthy();
    expect(screen.getByText(/The backups in Storage Box are being deleted in the background/)).toBeTruthy();
  });

  it("shows the entries as they are when the server found more of them, and asks again", async () => {
    const now: LocationErasure = { ...ERASURE, lastCopies: [...ERASURE.lastCopies, { domain: "vms", identity: "vm:win11", backups: 2 }] };
    eraseLocation.mockResolvedValueOnce({ ok: false, code: "last-copies", error: "entries would lose their only backup", erasure: now });
    await renderPage();
    const dialog = await openDeletion();
    await typeName(dialog, "Storage Box");
    await click(within(dialog).getByRole("switch", { name: "Delete these as well" }));

    await click(erase(dialog));

    expect(within(dialog).getByText(/the list changed since it was shown/)).toBeTruthy();
    expect(within(dialog).getByText("win11")).toBeTruthy();
    expect(within(dialog).getByRole("switch", { name: "Delete these as well" }).getAttribute("aria-checked")).toBe("false");
    expect(erase(dialog).disabled).toBe(true);
  });

  it("reads again when the sections changed since the reading", async () => {
    eraseLocation.mockResolvedValueOnce({ ok: false, code: "stale", error: "stale", removal: { sections: [] } });
    await renderPage();
    const dialog = await openDeletion();
    await typeName(dialog, "Storage Box");
    await click(within(dialog).getByRole("switch", { name: "Delete these as well" }));

    await click(erase(dialog));
    await act(async () => {});

    expect(within(dialog).getByText("The numbers changed since they were shown. Check them again.")).toBeTruthy();
    expect(previewLocationErasure).toHaveBeenCalledTimes(2);
  });

  it("says to wait while a backup of one of the sections runs", async () => {
    eraseLocation.mockResolvedValueOnce({ ok: false, code: "domain-busy", error: "busy" });
    await renderPage();
    const dialog = await openDeletion();
    await typeName(dialog, "Storage Box");
    await click(within(dialog).getByRole("switch", { name: "Delete these as well" }));

    await click(erase(dialog));

    expect(within(dialog).getByText("A backup is running. Choose again once it has finished.")).toBeTruthy();
    expect(screen.queryByText("the list")).toBeNull();
  });

  it("asks for a login password before anything is read or deleted", async () => {
    previewLocationErasure.mockResolvedValueOnce({ ok: false, code: "password-not-set", error: "set a login password first" });
    await renderPage();
    const dialog = await openDeletion();

    expect(within(dialog).getByText(en["storage.remove.passwordNeeded"])).toBeTruthy();
    expect(erase(dialog).disabled).toBe(true);
  });

  it("says when a repository stays because another section or location keeps its copies there", async () => {
    const shared: ErasureSection[] = [{ ...ERASURE.sections[1], kept: "shared" }];
    previewLocationErasure.mockResolvedValue({ ok: true, erasure: { ...ERASURE, sections: shared, backups: 0, entries: 0, backupsOnlyThere: 0, lastCopies: [] } });
    await renderPage();
    const dialog = await openDeletion();

    expect(within(dialog).getByText(en["storage.remove.keptShared"])).toBeTruthy();
  });

  it("says what to do instead where nothing can be deleted", async () => {
    const kept: ErasureSection[] = [{ ...ERASURE.sections[1] }];
    previewLocationErasure.mockResolvedValue({ ok: true, erasure: { ...ERASURE, sections: kept, backups: 0, entries: 0, backupsOnlyThere: 0, lastCopies: [] } });
    await renderPage();
    const dialog = await openDeletion();

    expect(within(dialog).getByText(/None of the backups here can be deleted.*keep the backups instead/)).toBeTruthy();
    expect(within(dialog).queryByLabelText("Type Storage Box to confirm")).toBeNull();
    expect(erase(dialog).disabled).toBe(true);
  });

  it("says why the reading failed and reads again when the way is picked again", async () => {
    previewLocationErasure.mockResolvedValueOnce({ ok: false, error: "the remote did not answer" });
    await renderPage();
    const dialog = await openDeletion();
    expect(within(dialog).getByText("the remote did not answer")).toBeTruthy();
    expect(erase(dialog).disabled).toBe(true);

    await click(within(dialog).getAllByRole("radio")[0]);
    await click(within(dialog).getAllByRole("radio")[1]);
    await act(async () => {});

    expect(within(dialog).queryByText("the remote did not answer")).toBeNull();
    expect(within(dialog).getByText("What is deleted")).toBeTruthy();
  });

  it("goes back to keeping the backups without deleting anything", async () => {
    await renderPage();
    const dialog = await openDeletion();

    await click(within(dialog).getAllByRole("radio")[0]);
    await click(within(dialog).getByRole("button", { name: "Remove storage location" }));

    expect(removeLocation).toHaveBeenCalledWith("destination:box", PREVIEW, undefined);
    expect(eraseLocation).not.toHaveBeenCalled();
  });
});

const TARGET: StorageLocation = { ...BOX, id: "target:t9", object: "target", name: "Offsite NAS" };
const REPO: StorageLocation = { ...BOX, id: "repo:r1", object: "repo", name: "Media repo" };
const FOLDER: StorageLocation = {
  ...BOX,
  id: "path:9c1e",
  object: "path",
  kind: "local",
  backend: "local",
  name: "backups",
  where: "user/backups",
  offPremises: false,
};

describe.each([
  ["target", TARGET, "Offsite NAS"],
  ["named repository", REPO, "Media repo"],
  // The one folder on this host goes by Local.
  ["folder", FOLDER, "Local"],
] as const)("removing a %s", (_kind, location, shown) => {
  it("is offered and reads what it would change under the location's whole id", async () => {
    await renderPage(location);
    await openRemoval(shown);

    expect(previewLocationRemoval).toHaveBeenCalledWith(location.id);
  });
});

describe("removing a named repository", () => {
  const FOLLOW: LocationRemoval = {
    sections: [
      part({ use: "repo", targetId: "", repo: "/mnt/user/media-repo", items: 3, listedAt: 0 }),
      part({ domain: "files", use: "repo", targetId: "", repo: "/mnt/user/media-repo", items: 0, default: true, listedAt: 0 }),
    ],
  };

  it("lists per section the entries that back up there and whether its default points there", async () => {
    previewLocationRemoval.mockResolvedValue({ ok: true, removal: FOLLOW });
    await renderPage(REPO);
    const dialog = await openRemoval("Media repo");

    expect(within(dialog).getByText("These entries follow their section again")).toBeTruthy();
    const rows = within(dialog).getAllByRole("listitem");
    expect(rows[0].textContent).toContain("3 entries back up here");
    expect(rows[0].textContent).not.toContain("The section's default points here");
    expect(rows[1].textContent).toContain("The section's default points here");
    expect(rows[1].textContent).not.toContain("back up here");
  });

  it("names the repository's sections by their section alone in the deletion", async () => {
    previewLocationRemoval.mockResolvedValue({ ok: true, removal: FOLLOW });
    previewLocationErasure.mockResolvedValue({
      ok: true,
      erasure: { ...ERASURE, removal: FOLLOW, sections: [repo({ use: "repo", targetId: "", repoId: "r1", repo: "/mnt/user/media-repo" })], lastCopies: [], key: "erase:r1" },
    });
    await renderPage(REPO);
    const dialog = await openRemoval("Media repo");
    await click(within(dialog).getAllByRole("radio")[1]);
    await act(async () => {});

    const row = within(within(dialog).getAllByRole("list")[0]).getByRole("listitem");
    expect(row.textContent).toContain("Containers");
    expect(row.textContent).not.toContain("direct repository");
    expect(within(row).queryByLabelText(/left this repository behind/)).toBeNull();
  });
});

describe("removing the folder the sections keep their backups in", () => {
  const MOVE: LocationRemoval = {
    sections: [
      part({ use: "home", targetId: "", repo: "user/backups/containers", folder: "containers" }),
      part({ domain: "vms", use: "home", targetId: "", repo: "user/backups/vms", folder: "vms", listedAt: 0 }),
    ],
    suggest: "user/vault",
  };
  const action = (dialog: HTMLElement, name: string) => within(dialog).getByRole("button", { name }) as HTMLButtonElement;
  const typeFolder = (dialog: HTMLElement, from: string, to: string) =>
    act(async () => {
      fireEvent.change(within(dialog).getByDisplayValue(from), { target: { value: to } });
    });

  beforeEach(() => {
    previewLocationRemoval.mockResolvedValue({ ok: true, removal: MOVE });
  });

  it("offers the suggested folder and shows each section's place now and under it", async () => {
    await renderPage(FOLDER);
    const dialog = await openRemoval("Local");

    expect(within(dialog).getByText("New folder for the sections")).toBeTruthy();
    expect(within(dialog).getByDisplayValue("user/vault")).toBeTruthy();
    expect(within(dialog).getByText(/The sections write to the new folder from now on/)).toBeTruthy();
    const rows = within(dialog).getAllByRole("listitem");
    expect(rows[0].textContent).toContain("user/backups/containers");
    expect(rows[0].textContent).toContain("user/vault/containers");
    expect(rows[1].textContent).toContain("user/vault/vms");
  });

  it("sends the folder the sections move to", async () => {
    await renderPage(FOLDER);
    const dialog = await openRemoval("Local");
    await typeFolder(dialog, "user/vault", "user/elsewhere/");

    expect(within(dialog).getAllByRole("listitem")[0].textContent).toContain("user/elsewhere/containers");
    await click(action(dialog, "Remove storage location"));

    expect(removeLocation).toHaveBeenCalledWith("path:9c1e", MOVE, "user/elsewhere");
    expect(screen.getByText("the list")).toBeTruthy();
  });

  it("waits for a folder that is not the current one", async () => {
    await renderPage(FOLDER);
    const dialog = await openRemoval("Local");

    await typeFolder(dialog, "user/vault", "user/backups");
    expect(action(dialog, "Remove storage location").disabled).toBe(true);

    await typeFolder(dialog, "user/backups", "");
    expect(action(dialog, "Remove storage location").disabled).toBe(true);
    expect(within(dialog).queryByText("Then")).toBeNull();
  });

  it("starts empty without a suggestion", async () => {
    previewLocationRemoval.mockResolvedValue({ ok: true, removal: { sections: MOVE.sections } });
    await renderPage(FOLDER);
    const dialog = await openRemoval("Local");

    expect(action(dialog, "Remove storage location").disabled).toBe(true);
  });

  it("shows the server's sentence for a folder it refuses", async () => {
    removeLocation.mockResolvedValueOnce({ ok: false, error: "The folder is inside another storage location." });
    await renderPage(FOLDER);
    const dialog = await openRemoval("Local");

    await click(action(dialog, "Remove storage location"));

    expect(within(dialog).getByText("The folder is inside another storage location.")).toBeTruthy();
    expect(screen.queryByText("the list")).toBeNull();
  });

  it("reads the sections again when the server finds them changed without sending them", async () => {
    removeLocation.mockResolvedValueOnce({ ok: false, code: "stale", error: "stale" });
    await renderPage(FOLDER);
    const dialog = await openRemoval("Local");

    await click(action(dialog, "Remove storage location"));
    await act(async () => {});

    expect(within(dialog).getByText("The numbers changed since they were shown. Check them again.")).toBeTruthy();
    expect(previewLocationRemoval).toHaveBeenCalledTimes(2);
    expect(within(dialog).getByDisplayValue("user/vault")).toBeTruthy();
  });

  it("reads the deletion again when the server finds it changed without sending it", async () => {
    eraseLocation.mockResolvedValueOnce({ ok: false, code: "stale", error: "stale" });
    previewLocationErasure.mockResolvedValue({ ok: true, erasure: { ...ERASURE, removal: MOVE, lastCopies: [] } });
    await renderPage(FOLDER);
    const dialog = await openRemoval("Local");
    await click(within(dialog).getAllByRole("radio")[1]);
    await act(async () => {});
    await act(async () => {
      fireEvent.change(within(dialog).getByLabelText("Type Local to confirm"), { target: { value: "Local" } });
    });

    await click(action(dialog, "Remove and delete"));
    await act(async () => {});

    expect(previewLocationErasure).toHaveBeenCalledTimes(2);
  });

  it("deletes the backups there with the folder and marks no home as left behind", async () => {
    const homes: LocationErasure = {
      ...ERASURE,
      removal: MOVE,
      sections: [
        repo({ use: "home", targetId: "", repo: "user/backups/containers" }),
        // An empty folder still moves, so it is listed too.
        repo({ domain: "vms", use: "home", targetId: "", repo: "user/backups/vms", backups: 0, entries: 0 }),
      ],
      lastCopies: [],
      key: "erase:9c1e",
    };
    previewLocationErasure.mockResolvedValue({ ok: true, erasure: homes });
    await renderPage(FOLDER);
    const dialog = await openRemoval("Local");
    await click(within(dialog).getAllByRole("radio")[1]);
    await act(async () => {});

    const rows = within(within(dialog).getAllByRole("list")[0]).getAllByRole("listitem");
    expect(rows).toHaveLength(2);
    for (const row of rows) expect(within(row).queryByLabelText(/left this repository behind/)).toBeNull();

    await act(async () => {
      fireEvent.change(within(dialog).getByLabelText("Type Local to confirm"), { target: { value: "Local" } });
    });
    await click(action(dialog, "Remove and delete"));

    expect(eraseLocation).toHaveBeenCalledWith("path:9c1e", MOVE, [], "user/vault");
  });
});
