// @vitest-environment jsdom
/**
 * Applying a location's keep rule right away.
 *
 * A changed rule otherwise waits for the next backup. The card shows what the
 * rule would remove, and removes it only once the person says yes.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import type { LocationKeepPreview, RetentionPreviewSnapshot, Settings, StorageLocation, StorageLocationSection } from "../lib/api";
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
    getSettings: vi.fn(),
    getProviders: vi.fn(),
    getCloud: vi.fn(),
    getCloudCredSets: vi.fn(),
    getRcloneSections: vi.fn(),
    previewLocationKeep: vi.fn(),
    applyLocationKeep: vi.fn(),
  };
});

const api = await import("../lib/api");
const getStorageLocation = vi.mocked(api.getStorageLocation);
const previewLocationKeep = vi.mocked(api.previewLocationKeep);
const applyLocationKeep = vi.mocked(api.applyLocationKeep);

const { StorageLocationPage } = await import("./StorageLocation");

const KEEP = { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 6, keepYearly: 2 };
const NONE = { keepLast: 0, keepDaily: 0, keepWeekly: 0, keepMonthly: 0, keepYearly: 0 };

function section(over: Partial<StorageLocationSection> = {}): StorageLocationSection {
  return {
    domain: "containers",
    use: "home",
    where: "/mnt/user/backups/containers",
    enabled: true,
    immutable: false,
    retention: KEEP,
    compression: "auto",
    limitUpload: 0,
    limitDownload: 0,
    own: [],
    ...over,
  };
}

function location(over: Partial<StorageLocation> = {}): StorageLocation {
  return {
    id: "destination:disk",
    object: "destination",
    kind: "local",
    provider: "path",
    backend: "local",
    name: "Disk",
    where: "/mnt/user/backups",
    enabled: true,
    offPremises: false,
    sections: [section()],
    retention: KEEP,
    compression: "auto",
    limitUpload: 0,
    limitDownload: 0,
    protection: { immutable: false, testable: false },
    capacity: {},
    ...over,
  };
}

const snap = (id: string): RetentionPreviewSnapshot => ({ id, time: "2026-10-01T03:00:00Z" });

function preview(removes: string[], over: Partial<LocationKeepPreview> = {}): LocationKeepPreview {
  return {
    sections: [
      {
        domain: "containers",
        use: "home",
        preview: {
          policy: { ...KEEP, on: true, own: false },
          repos: [{ name: "folder containers", appendOnly: false, items: [{ tag: "container:plex", keep: [snap("keep1")], remove: removes.map(snap) }] }],
        },
      },
    ],
    ...over,
  };
}

async function renderPage(loc: StorageLocation = location()) {
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
  await act(async () => {});
}

function keepCard(): HTMLElement {
  const heading = screen.getAllByRole("heading", { level: 2 }).find((h) => h.textContent === "How long to keep");
  if (!heading) throw new Error("no keep card");
  return heading.parentElement!;
}

const applyNow = () => within(keepCard()).queryByRole("button", { name: "Apply now" });

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({ ok: true }) }));
  vi.mocked(api.listOffsiteTargets).mockResolvedValue({ ok: true, targets: [] });
  vi.mocked(api.listDestinations).mockResolvedValue({ ok: true, destinations: [] });
  vi.mocked(api.getSettings).mockResolvedValue({ ok: true, settings: {} as Settings, hostMountRoot: "", platform: "" });
  vi.mocked(api.getProviders).mockResolvedValue({ ok: true, providers: [] });
  vi.mocked(api.getCloud).mockResolvedValue({ ok: true });
  vi.mocked(api.getCloudCredSets).mockResolvedValue({ ok: true, sets: [] });
  vi.mocked(api.getRcloneSections).mockResolvedValue({ ok: true, sections: [] });
  previewLocationKeep.mockResolvedValue({ ok: true, preview: preview(["old1", "old2", "old3"]) });
  applyLocationKeep.mockResolvedValue({ ok: true });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

describe("where the rule can be applied now", () => {
  it("offers it on a location whose rule removes old backups", async () => {
    await renderPage();

    expect(applyNow()).toBeTruthy();
  });

  it("offers nothing where the location keeps every backup", async () => {
    await renderPage(location({ retention: NONE }));

    expect(applyNow()).toBeNull();
  });

  it("offers nothing where the location refuses deletes", async () => {
    await renderPage(location({ protection: { immutable: true, testable: false } }));

    expect(applyNow()).toBeNull();
  });
});

describe("applying the rule now", () => {
  it("shows what goes and removes it only after yes", async () => {
    await renderPage();

    await click(applyNow()!);

    expect(previewLocationKeep).toHaveBeenCalledWith("destination:disk");
    const dialog = screen.getByRole("dialog", { name: "Apply the rule now" });
    expect(dialog.textContent).toContain("Backups to remove: 3");
    expect(dialog.textContent).toContain("Containers: 3");
    expect(applyLocationKeep).not.toHaveBeenCalled();

    await click(within(dialog).getByRole("button", { name: "Apply now" }));

    expect(applyLocationKeep).toHaveBeenCalledWith("destination:disk");
    expect(await screen.findByText("Disk now keeps only what the rule says.")).toBeTruthy();
  });

  it("removes nothing on cancel", async () => {
    await renderPage();

    await click(applyNow()!);
    await click(within(screen.getByRole("dialog")).getByRole("button", { name: "Cancel" }));

    expect(applyLocationKeep).not.toHaveBeenCalled();
  });

  it("asks nothing when the rule removes nothing", async () => {
    previewLocationKeep.mockResolvedValue({ ok: true, preview: preview([]) });
    await renderPage();

    await click(applyNow()!);

    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByText("Nothing to remove. Everything at Disk already fits the rule.")).toBeTruthy();
    expect(applyLocationKeep).not.toHaveBeenCalled();
  });

  it("says so when there are no backups here yet", async () => {
    previewLocationKeep.mockResolvedValue({ ok: true, preview: { sections: [], empty: true } });
    await renderPage();

    await click(applyNow()!);

    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByText("There are no backups at Disk yet.")).toBeTruthy();
    expect(applyLocationKeep).not.toHaveBeenCalled();
  });

  it("names the entries an unusual backup holds", async () => {
    applyLocationKeep.mockResolvedValue({ ok: true, paused: ["container:plex"] });
    await renderPage();

    await click(applyNow()!);
    await click(within(screen.getByRole("dialog")).getByRole("button", { name: "Apply now" }));

    expect(await screen.findByText("Pruned. Old backups were kept for items with an open anomaly: plex")).toBeTruthy();
  });

  it("puts the server's refusal on screen", async () => {
    previewLocationKeep.mockResolvedValue({ ok: false, error: "Disk could not be reached" });
    await renderPage();

    await click(applyNow()!);

    expect(screen.queryByRole("dialog")).toBeNull();
    expect(applyLocationKeep).not.toHaveBeenCalled();
    expect(screen.getByText("Disk could not be reached")).toBeTruthy();
  });
});
