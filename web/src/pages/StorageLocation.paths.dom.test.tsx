// @vitest-environment jsdom
/**
 * Where a section's own backups are written.
 *
 * A folder of this host lists the folder of every section in it, and each is
 * changed in a window that writes the section's path into the settings. The
 * page reads the locations again only once that window is closed, because a
 * changed path can move the section to another location.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import type { Settings, StorageLocation, StorageLocationSection } from "../lib/api";
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
    getSettings: vi.fn(),
    putSettings: vi.fn(),
    getProviders: vi.fn(),
  };
});

const api = await import("../lib/api");
const getStorageLocation = vi.mocked(api.getStorageLocation);
const getSettings = vi.mocked(api.getSettings);
const putSettings = vi.mocked(api.putSettings);

const { StorageLocationPage } = await import("./StorageLocation");

const KEEP = { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 6, keepYearly: 1 };

function home(domain: StorageLocationSection["domain"], where: string): StorageLocationSection {
  return { domain, use: "home", where, enabled: true, immutable: false, retention: KEEP, compression: "auto", limitUpload: 0, limitDownload: 0, own: [] };
}

const LOCAL: StorageLocation = {
  id: "path:abc",
  object: "path",
  kind: "local",
  provider: "",
  backend: "local",
  name: "bombvault",
  where: "backups/bombvault",
  enabled: true,
  offPremises: false,
  sections: [home("containers", "backups/bombvault/containers"), home("vms", "backups/bombvault/vms")],
  retention: KEEP,
  protection: { immutable: false, testable: false },
  capacity: {},
};

const SETTINGS = {
  containersPath: "backups/bombvault/containers",
  vmsPath: "backups/bombvault/vms",
  compression: { containers: "auto" },
  ownRetention: {},
} as unknown as Settings;

async function renderPage() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <MemoryRouter initialEntries={["/storage/path%3Aabc"]}>
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

function connection(): HTMLElement {
  const heading = screen.getAllByRole("heading", { level: 2 }).find((h) => h.textContent === "Connection");
  return heading!.parentElement!;
}

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({ ok: true }) }));
  getStorageLocation.mockResolvedValue({ ok: true, location: LOCAL });
  getSettings.mockResolvedValue({ ok: true, settings: SETTINGS, hostMountRoot: "/host/user", platform: "" });
  putSettings.mockResolvedValue({ ok: true });
  vi.mocked(api.getProviders).mockResolvedValue({ ok: true, providers: [] });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

describe("the folders of a local location", () => {
  it("lists the folder of every section that is backed up there", async () => {
    await renderPage();

    const rows = within(connection()).getAllByRole("listitem");
    expect(rows.map((row) => row.textContent)).toEqual([
      expect.stringContaining("Containersbackups/bombvault/containers"),
      expect.stringContaining("VMsbackups/bombvault/vms"),
    ]);
  });

  it("changes one section's folder in a window and saves what was typed when the window closes", async () => {
    await renderPage();
    await click(within(within(connection()).getAllByRole("listitem")[1]).getByRole("button", { name: "Change" }));
    await act(async () => {});

    const dialog = screen.getByRole("dialog", { name: "VMs path" });
    const field = within(dialog).getByDisplayValue("backups/bombvault/vms");
    await act(async () => {
      fireEvent.change(field, { target: { value: "backups/machines" } });
    });
    expect(putSettings).not.toHaveBeenCalled();

    const reads = getStorageLocation.mock.calls.length;
    await click(within(dialog).getByRole("button", { name: "Done" }));

    expect(putSettings).toHaveBeenCalledWith(expect.objectContaining({ vmsPath: "backups/machines", containersPath: "backups/bombvault/containers" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(getStorageLocation.mock.calls.length).toBeGreaterThan(reads);
  });

  it("takes a remote address in the same window", async () => {
    await renderPage();
    await click(within(within(connection()).getAllByRole("listitem")[0]).getByRole("button", { name: "Change" }));
    await act(async () => {});

    const dialog = screen.getByRole("dialog", { name: "Containers path" });
    await click(within(dialog).getByRole("tab", { name: "Remote" }));
    await act(async () => {
      fireEvent.change(within(dialog).getByPlaceholderText(/s3:bucket/), { target: { value: "rest:http://nas:8000/tower" } });
    });
    await click(within(dialog).getByRole("button", { name: "Done" }));

    expect(putSettings).toHaveBeenCalledWith(expect.objectContaining({ containersPath: "rest:http://nas:8000/tower" }));
  });

  it("saves a section's compression from that window at once", async () => {
    await renderPage();
    await click(within(within(connection()).getAllByRole("listitem")[0]).getByRole("button", { name: "Change" }));
    await act(async () => {});

    const dialog = screen.getByRole("dialog", { name: "Containers path" });
    await click(within(dialog).getByRole("tab", { name: "Maximum" }));

    expect(putSettings).toHaveBeenCalledWith(expect.objectContaining({ compression: { containers: "max" } }));
  });

  it("says why the server refused a path and saves nothing more on Done", async () => {
    putSettings.mockResolvedValue({ ok: false, error: "containersPath must be relative" });
    await renderPage();
    await click(within(within(connection()).getAllByRole("listitem")[0]).getByRole("button", { name: "Change" }));
    await act(async () => {});

    const dialog = screen.getByRole("dialog", { name: "Containers path" });
    await act(async () => {
      fireEvent.change(within(dialog).getByDisplayValue("backups/bombvault/containers"), { target: { value: "/etc" } });
    });
    await click(within(dialog).getByRole("button", { name: "Done" }));

    expect(await screen.findByText("containersPath must be relative")).toBeTruthy();
    expect(putSettings).toHaveBeenCalledTimes(1);
  });

  it("opens the same window from a section's rules", async () => {
    await renderPage();
    await click(screen.getAllByRole("button", { name: "Change rules" })[0]);
    const rules = screen.getByRole("dialog", { name: "Rules for Containers" });

    await click(within(rules).getByRole("button", { name: "Type a location" }));
    await act(async () => {});

    expect(screen.getByRole("dialog", { name: "Containers path" })).toBeTruthy();
  });
});
