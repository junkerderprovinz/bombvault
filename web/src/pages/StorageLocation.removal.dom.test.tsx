// @vitest-environment jsdom
/**
 * Removing a storage location.
 *
 * The window says which sections stop backing up to the place before
 * anything is removed, and sends back exactly what it showed, so the server
 * can refuse a removal whose sections changed in between.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import type { Destination, DestinationRemoval, RemovalSection, Settings, StorageLocation } from "../lib/api";
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
    previewDestinationRemoval: vi.fn(),
    removeDestination: vi.fn(),
    getSettings: vi.fn(),
    getProviders: vi.fn(),
    getCloud: vi.fn(),
    getCloudCredSets: vi.fn(),
    getRclone: vi.fn(),
    getRcloneSections: vi.fn(),
  };
});

const api = await import("../lib/api");
const getStorageLocation = vi.mocked(api.getStorageLocation);
const previewDestinationRemoval = vi.mocked(api.previewDestinationRemoval);
const removeDestination = vi.mocked(api.removeDestination);

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

const PREVIEW: DestinationRemoval = {
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

async function openRemoval(): Promise<HTMLElement> {
  await click(screen.getByRole("button", { name: "More" }));
  await click(screen.getByRole("menuitem", { name: "Remove storage location" }));
  return screen.getByRole("dialog", { name: "Remove Storage Box?" });
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
  vi.mocked(api.getRclone).mockResolvedValue({ ok: true, remotes: ["storagebox"] });
  vi.mocked(api.getRcloneSections).mockResolvedValue({ ok: true, sections: [] });
  previewDestinationRemoval.mockResolvedValue({ ok: true, removal: PREVIEW });
  removeDestination.mockResolvedValue({ ok: true });
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

    expect(previewDestinationRemoval).toHaveBeenCalledWith("box");
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

  it("offers one way only: nothing in the window deletes what is stored there", async () => {
    await renderPage();
    const dialog = await openRemoval();

    expect(within(dialog).queryByRole("radio")).toBeNull();
    expect(dialog.textContent).not.toMatch(/delete the backups/i);
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

    expect(removeDestination).toHaveBeenCalledWith("box", PREVIEW);
    expect(screen.getByText("the list")).toBeTruthy();
    expect(screen.getByText("Storage Box removed, the backups there stay")).toBeTruthy();
  });

  it("removes nothing when the window is cancelled", async () => {
    await renderPage();
    const dialog = await openRemoval();

    await click(within(dialog).getByRole("button", { name: "Cancel" }));

    expect(removeDestination).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe("Storage Box");
  });

  it("shows the sections as they are when they changed since the preview, and asks again", async () => {
    const now: DestinationRemoval = { sections: [PREVIEW.sections[0]] };
    removeDestination.mockResolvedValueOnce({ ok: false, code: "stale", error: "stale", removal: now });
    await renderPage();
    const dialog = await openRemoval();

    await click(within(dialog).getByRole("button", { name: "Remove storage location" }));

    expect(within(dialog).getByText("The numbers changed since they were shown. Check them again.")).toBeTruthy();
    expect(within(dialog).getAllByRole("listitem")).toHaveLength(1);

    await click(within(dialog).getByRole("button", { name: "Remove storage location" }));
    expect(removeDestination).toHaveBeenLastCalledWith("box", now);
    expect(screen.getByText("the list")).toBeTruthy();
  });

  it("says so for a place no section backs up to", async () => {
    previewDestinationRemoval.mockResolvedValue({ ok: true, removal: { sections: [] } });
    await renderPage();
    const dialog = await openRemoval();

    expect(within(dialog).getByText("No section backs up here yet.")).toBeTruthy();
    expect((within(dialog).getByRole("button", { name: "Remove storage location" }) as HTMLButtonElement).disabled).toBe(false);
  });

  it("cannot remove while the preview did not load", async () => {
    previewDestinationRemoval.mockResolvedValue({ ok: false, error: "database is locked" });
    await renderPage();
    const dialog = await openRemoval();

    expect(within(dialog).getByText("database is locked")).toBeTruthy();
    expect((within(dialog).getByRole("button", { name: "Remove storage location" }) as HTMLButtonElement).disabled).toBe(true);
  });
});

describe("a location that is not a destination", () => {
  it("has no removal", async () => {
    await renderPage({ ...BOX, id: "path:abc", object: "path", kind: "local", backend: "local", where: "/mnt/user/backups" });

    expect(screen.queryByRole("button", { name: "More" })).toBeNull();
  });
});
