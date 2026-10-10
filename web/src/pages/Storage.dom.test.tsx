// @vitest-environment jsdom
/**
 * The Storage locations page.
 *
 * A row says where a location is, who uses it and how full it is. A backend
 * that reports no size must not look like an empty one, and a list that did
 * not load must not look like no locations.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useParams } from "react-router-dom";

import type { StorageLocation, StreamingSettings } from "../lib/api";
import { I18nProvider } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";

// jsdom has no EventSource, and the page follows the progress stream.
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
  return { ...actual, listStorageLocations: vi.fn(), getStreaming: vi.fn(), setStreaming: vi.fn() };
});

const api = await import("../lib/api");
const listStorageLocations = vi.mocked(api.listStorageLocations);
const getStreaming = vi.mocked(api.getStreaming);
const setStreaming = vi.mocked(api.setStreaming);

const { StoragePage } = await import("./Storage");

const GB = 1024 ** 3;
const KEEP = { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 6, keepYearly: 1 };

function location(over: Partial<StorageLocation> = {}): StorageLocation {
  return {
    id: "path:abc",
    object: "path",
    kind: "local",
    provider: "",
    backend: "local",
    name: "bombvault",
    where: "/mnt/user/backups/bombvault",
    enabled: true,
    offPremises: false,
    sections: [],
    retention: KEEP,
    protection: { immutable: false, testable: false },
    capacity: {},
    ...over,
  };
}

function uses(domain: "containers" | "vms" | "files") {
  return {
    domain,
    use: "home" as const,
    where: "",
    enabled: true,
    immutable: false,
    retention: KEEP,
    compression: "auto" as const,
    limitUpload: 0,
    limitDownload: 0,
    own: [],
  };
}

const STREAMING: StreamingSettings = {
  enabled: false,
  mediaServers: [],
  mediaServersAuto: true,
  thresholdMbit: 2,
  limitKiB: 512,
  holdMin: 5,
};

function Opened() {
  return <p>opened {useParams().id}</p>;
}

async function renderPage() {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <MemoryRouter initialEntries={["/storage"]}>
            <Routes>
              <Route path="/storage" element={<StoragePage />} />
              <Route path="/storage/:id" element={<Opened />} />
            </Routes>
          </MemoryRouter>
        </ToastProvider>
      </I18nProvider>
    );
  });
}

function row(name: string): HTMLElement {
  return screen.getByRole("link", { name }).closest("li")!;
}

beforeEach(() => {
  localStorage.clear();
  getStreaming.mockResolvedValue({ ok: true, settings: STREAMING, candidates: [], streaming: "" });
  setStreaming.mockResolvedValue({ ok: true });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("the list", () => {
  it("shows where a location is, who uses it and how full it is", async () => {
    listStorageLocations.mockResolvedValue({
      ok: true,
      locations: [
        location({
          sections: [uses("containers"), uses("vms")],
          capacity: { usedBytes: 412 * GB, totalBytes: 3584 * GB, weeksToFull: 33 },
        }),
      ],
    });
    await renderPage();

    const local = row("bombvault");
    expect(within(local).getByText("/mnt/user/backups/bombvault")).toBeTruthy();
    expect(local.textContent).toContain("Used by Containers and VMs");
    expect(within(local).getByText("full in about 33 weeks")).toBeTruthy();
    expect(within(local).getByText("412.0 GB of 3.5 TB")).toBeTruthy();
    expect(within(local).getByLabelText("11 % used. At the growth of the last weeks, full in about 33 weeks.")).toBeTruthy();
  });

  it("marks where a measured location is full and leaves the mark out where the size is unknown", async () => {
    listStorageLocations.mockResolvedValue({
      ok: true,
      locations: [
        location({ capacity: { usedBytes: 10 * GB, totalBytes: 100 * GB } }),
        location({ id: "destination:b2", name: "B2", kind: "offsite", backend: "s3", capacity: { unsupported: true, storedBytes: 2 * GB } }),
      ],
    });
    await renderPage();

    expect(row("bombvault").querySelector(".glim-vessel-full")).not.toBeNull();
    const b2 = row("B2");
    expect(b2.querySelector(".glim-vessel-full")).toBeNull();
    expect(within(b2).getByText("Size unknown")).toBeTruthy();
    expect(within(b2).getByText("2.0 GB")).toBeTruthy();
    expect(
      within(b2).getByLabelText('This storage location does not report how big it is. That is why the "full" line is missing.')
    ).toBeTruthy();
  });

  it("does not claim a remote reports no size when nobody has asked it yet", async () => {
    listStorageLocations.mockResolvedValue({
      ok: true,
      locations: [location({ id: "target:t1", name: "NAS", kind: "offsite", backend: "sftp", capacity: {} })],
    });
    await renderPage();

    expect(
      within(row("NAS")).getByLabelText(
        "BombVault has not measured how full this storage location is yet. It does when you open the location's page."
      )
    ).toBeTruthy();
  });

  it("warns when a location fills within weeks", async () => {
    listStorageLocations.mockResolvedValue({
      ok: true,
      locations: [location({ capacity: { usedBytes: 95 * GB, totalBytes: 100 * GB, weeksToFull: 3 } })],
    });
    await renderPage();

    expect(row("bombvault").querySelector(".glim-vessel-warn")).not.toBeNull();
  });

  it("says a location is off and that nothing uses it, and names delete protection", async () => {
    listStorageLocations.mockResolvedValue({
      ok: true,
      locations: [location({ enabled: false, protection: { immutable: true, testable: false } })],
    });
    await renderPage();

    const local = row("bombvault");
    expect(within(local).getByText("Off")).toBeTruthy();
    expect(within(local).getByText("Delete protection")).toBeTruthy();
    expect(local.textContent).toContain("Not ticked anywhere yet");
    expect(local.querySelector(".glim-vessel-liquid")).toBeNull();
  });

  it("opens a location from its row", async () => {
    listStorageLocations.mockResolvedValue({ ok: true, locations: [location({ id: "destination:4f2a", name: "Box" })] });
    await renderPage();

    const link = screen.getByRole("link", { name: "Box" });
    expect(link.getAttribute("href")).toBe("/storage/destination%3A4f2a");
    fireEvent.click(link);
    expect(await screen.findByText("opened destination:4f2a")).toBeTruthy();
  });

  it("says so when there is no location", async () => {
    listStorageLocations.mockResolvedValue({ ok: true, locations: [] });
    await renderPage();

    expect(screen.getByText("No storage location yet.")).toBeTruthy();
  });

  it("offers to try again when the list did not load, and loads it then", async () => {
    listStorageLocations.mockRejectedValueOnce(new Error("offline"));
    await renderPage();

    expect(screen.getByText("The storage locations could not be loaded")).toBeTruthy();
    expect(screen.queryByText("No storage location yet.")).toBeNull();

    listStorageLocations.mockResolvedValue({ ok: true, locations: [location()] });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    });
    expect(await screen.findByRole("link", { name: "bombvault" })).toBeTruthy();
  });
});

describe("streaming", () => {
  beforeEach(() => {
    listStorageLocations.mockResolvedValue({ ok: true, locations: [location()] });
  });

  it("keeps the media servers and limits out of sight while the switch is off", async () => {
    await renderPage();

    expect(screen.getByRole("switch", { name: "Slow off-site copies while streaming" }).getAttribute("aria-checked")).toBe("false");
    expect(screen.queryByText("Media servers")).toBeNull();
  });

  it("saves the switch and shows what depends on it", async () => {
    await renderPage();

    await act(async () => {
      fireEvent.click(screen.getByRole("switch", { name: "Slow off-site copies while streaming" }));
    });
    await waitFor(() => expect(setStreaming).toHaveBeenCalledWith({ ...STREAMING, enabled: true }));
    expect(screen.getByText("Media servers")).toBeTruthy();
    expect(screen.getByText("Stream above (Mbit/s)")).toBeTruthy();
  });

  it("leaves the card out when its settings cannot be read", async () => {
    getStreaming.mockResolvedValue({ ok: false, error: "no docker" });
    await renderPage();

    expect(screen.queryByText("Streaming")).toBeNull();
  });
});
