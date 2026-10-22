// @vitest-environment jsdom
/**
 * Moving a section's copy to another storage location.
 *
 * The window offers only the destinations that can take the section, asks the
 * server what the move would copy before anything starts, and puts a refusal
 * on screen as a sentence. The move runs in the background, so the window
 * closes once it started and the section's row says that it moves.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import type { Destination, OffsiteTarget, SectionMove, Settings, StorageLocation, StorageLocationSection } from "../lib/api";
import { I18nProvider } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";

const streams: FakeEventSource[] = [];

class FakeEventSource {
  onmessage: ((ev: MessageEvent<string>) => void) | null = null;
  onerror: unknown = null;
  constructor() {
    streams.push(this);
  }
  close(): void {}
}

function progress(frame: Record<string, unknown>) {
  act(() => {
    streams.at(-1)?.onmessage?.({ data: JSON.stringify(frame) } as MessageEvent<string>);
  });
}

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getStorageLocation: vi.fn(),
    listStorageLocations: vi.fn(),
    listOffsiteTargets: vi.fn(),
    listDestinations: vi.fn(),
    previewSectionMove: vi.fn(),
    moveSection: vi.fn(),
    getSettings: vi.fn(),
    getProviders: vi.fn(),
    getCloud: vi.fn(),
    getCloudCredSets: vi.fn(),
    getRcloneSections: vi.fn(),
  };
});

const api = await import("../lib/api");
const getStorageLocation = vi.mocked(api.getStorageLocation);
const listStorageLocations = vi.mocked(api.listStorageLocations);
const previewSectionMove = vi.mocked(api.previewSectionMove);
const moveSection = vi.mocked(api.moveSection);

const { StorageLocationPage } = await import("./StorageLocation");

const KEEP = { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 6, keepYearly: 1 };

function section(over: Partial<StorageLocationSection>): StorageLocationSection {
  return {
    domain: "containers",
    use: "copy",
    where: "",
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

function location(over: Partial<StorageLocation>): StorageLocation {
  return {
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
    ...over,
  };
}

const BOX = location({
  sections: [
    section({ domain: "containers", targetId: "t-box-containers", where: "rclone:storagebox:bombvault/containers" }),
    section({ domain: "flash", targetId: "t-box-flash", where: "rclone:storagebox:bombvault/flash" }),
  ],
});
const B2 = location({
  id: "destination:b2",
  backend: "s3",
  name: "Backblaze B2",
  where: "s3:https://s3.example.com/bv",
  sections: [section({ domain: "containers", targetId: "t-b2-containers" })],
});
const NAS = location({ id: "destination:nas", backend: "rest", name: "NAS", where: "rest:https://nas:8000/tower" });
const LOCAL = location({
  id: "path:abc",
  object: "path",
  kind: "local",
  backend: "local",
  name: "bombvault",
  where: "/mnt/user/backups",
  sections: [section({ domain: "containers", use: "home" })],
});
const TYPED = location({
  id: "target:t-typed",
  object: "target",
  backend: "sftp",
  name: "Old NAS",
  where: "sftp:old:/bv/vms",
  sections: [section({ domain: "vms", targetId: "t-typed", where: "sftp:old:/bv/vms" })],
});

const PLAN: SectionMove = {
  domain: "flash",
  targetId: "t-box-flash",
  from: "rclone:storagebox:bombvault/flash",
  to: "rest:https://nas:8000/tower/flash",
  backups: 14,
  pending: 9,
  key: "move:t-box-flash",
};

function target(id: string, domain: string, destinationId: string): OffsiteTarget {
  return { id, domain, name: id, repo: "", credsRef: "", storageClass: "", immutable: false, schedule: "", enabled: true, sortOrder: 1, destinationId } as OffsiteTarget;
}

async function renderPage(loc: StorageLocation = BOX) {
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

function row(domain: string): HTMLElement {
  const heading = screen.getAllByRole("heading", { level: 2 }).find((h) => h.textContent === "Rules per section");
  const rows = Array.from(heading!.parentElement!.querySelectorAll<HTMLElement>(":scope > div > div"));
  const found = rows.find((el) => el.textContent?.startsWith(domain));
  if (!found) throw new Error(`no row for ${domain}`);
  return found;
}

async function openMove(domain = "Flash"): Promise<HTMLElement> {
  await click(within(row(domain)).getByRole("button", { name: "Move" }));
  return screen.getByRole("dialog", { name: `Move the copy of ${domain}` });
}

const start = (dialog: HTMLElement) => within(dialog).getByRole("button", { name: "Move" }) as HTMLButtonElement;

beforeEach(() => {
  localStorage.clear();
  streams.length = 0;
  vi.stubGlobal("EventSource", FakeEventSource);
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({ ok: true }) }));
  listStorageLocations.mockResolvedValue({ ok: true, locations: [LOCAL, BOX, B2, NAS, TYPED] });
  vi.mocked(api.listOffsiteTargets).mockResolvedValue({
    ok: true,
    targets: [target("t-box-containers", "containers", "box"), target("t-box-flash", "flash", "box"), target("t-typed", "vms", "")],
  });
  vi.mocked(api.listDestinations).mockResolvedValue({ ok: true, destinations: [{ id: "box", name: "Storage Box" }] as Destination[] });
  vi.mocked(api.getSettings).mockResolvedValue({ ok: true, settings: {} as Settings, hostMountRoot: "", platform: "" });
  vi.mocked(api.getProviders).mockResolvedValue({ ok: true, providers: [] });
  vi.mocked(api.getCloud).mockResolvedValue({ ok: true });
  vi.mocked(api.getCloudCredSets).mockResolvedValue({ ok: true, sets: [] });
  vi.mocked(api.getRcloneSections).mockResolvedValue({ ok: true, sections: [] });
  previewSectionMove.mockResolvedValue({ ok: true, move: PLAN });
  moveSection.mockResolvedValue({ ok: true, started: true, move: PLAN });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

describe("where a copy can move from", () => {
  it("offers the move on every section a destination holds a copy of", async () => {
    await renderPage();

    expect(within(row("Containers")).getByRole("button", { name: "Move" })).toBeTruthy();
    expect(within(row("Flash")).getByRole("button", { name: "Move" })).toBeTruthy();
  });

  it("offers none for a folder on this host, which holds no copy", async () => {
    await renderPage(LOCAL);

    expect(screen.queryByRole("button", { name: "Move" })).toBeNull();
  });

  it("offers it in the menu of a target that was set up for one section", async () => {
    await renderPage(TYPED);

    await click(screen.getByRole("button", { name: "More" }));
    await click(screen.getByRole("menuitem", { name: "Move" }));

    expect(screen.getByRole("dialog", { name: "Move the copy of VMs" })).toBeTruthy();
  });
});

describe("the window that moves a section", () => {
  it("offers every other destination that has no copy of the section yet", async () => {
    await renderPage();

    const flash = await openMove("Flash");
    expect(within(flash).getAllByRole("radio").map((r) => r.getAttribute("aria-label"))).toEqual(["Backblaze B2", "NAS"]);
    await click(within(flash).getByRole("button", { name: "Cancel" }));

    const containers = await openMove("Containers");
    expect(within(containers).getAllByRole("radio").map((r) => r.getAttribute("aria-label"))).toEqual(["NAS"]);
  });

  it("says so when no other storage location can take the section", async () => {
    listStorageLocations.mockResolvedValue({ ok: true, locations: [LOCAL, BOX, TYPED] });
    await renderPage();
    const dialog = await openMove();

    expect(within(dialog).getByText("No other storage location can take Flash. Add one first.")).toBeTruthy();
    expect(start(dialog).disabled).toBe(true);
  });

  it("asks nothing and starts nothing before a place is picked", async () => {
    await renderPage();
    const dialog = await openMove();

    expect(previewSectionMove).not.toHaveBeenCalled();
    expect(start(dialog).disabled).toBe(true);
  });

  it("shows what the move copies once a place is picked", async () => {
    await renderPage();
    const dialog = await openMove();

    await click(within(dialog).getByRole("radio", { name: "NAS" }));

    expect(previewSectionMove).toHaveBeenCalledWith("t-box-flash", "nas");
    expect(within(dialog).getByText("rclone:storagebox:bombvault/flash")).toBeTruthy();
    expect(within(dialog).getByText("rest:https://nas:8000/tower/flash")).toBeTruthy();
    expect(within(dialog).getByText("14 backups · 9 of them are still missing at the new place")).toBeTruthy();
    expect(within(dialog).getByText("Everything stays in the old place. BombVault deletes nothing there.")).toBeTruthy();
    expect(within(dialog).getByText(/runs in the background and can take long.*follow it on the overview/)).toBeTruthy();
    expect(start(dialog).disabled).toBe(false);
  });

  it("says that only the check is left when the new place holds everything", async () => {
    previewSectionMove.mockResolvedValue({ ok: true, move: { ...PLAN, pending: 0 } });
    await renderPage();
    const dialog = await openMove();

    await click(within(dialog).getByRole("radio", { name: "NAS" }));

    expect(within(dialog).getByText(/14 backups · The new place already holds all of them/)).toBeTruthy();
  });

  it("asks again for another place and drops what it showed for the first", async () => {
    await renderPage();
    const dialog = await openMove();
    await click(within(dialog).getByRole("radio", { name: "NAS" }));
    previewSectionMove.mockResolvedValue({ ok: false, code: "move-section-there", error: "the section is there" });

    await click(within(dialog).getByRole("radio", { name: "Backblaze B2" }));

    expect(previewSectionMove).toHaveBeenLastCalledWith("t-box-flash", "b2");
    expect(within(dialog).queryByText("rest:https://nas:8000/tower/flash")).toBeNull();
    expect(start(dialog).disabled).toBe(true);
  });

  it.each([
    ["move-same-place", /already keeps its copy there/],
    ["move-section-there", /already has a copy of this section of its own/],
    ["move-direct-repo", /back up straight to this place.*Switch those entries to Local first/],
    ["move-running", /already moving.*overview/],
  ] as const)("says what to do about the refusal %s", async (code, sentence) => {
    previewSectionMove.mockResolvedValue({ ok: false, code, error: "server words" });
    await renderPage();
    const dialog = await openMove();

    await click(within(dialog).getByRole("radio", { name: "NAS" }));

    expect(within(dialog).getByText(sentence)).toBeTruthy();
    expect(within(dialog).queryByText("server words")).toBeNull();
    expect(start(dialog).disabled).toBe(true);
  });

  it("starts the move to the place that was picked and closes", async () => {
    await renderPage();
    const dialog = await openMove();
    await click(within(dialog).getByRole("radio", { name: "NAS" }));

    await click(start(dialog));

    expect(moveSection).toHaveBeenCalledWith("t-box-flash", "nas");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByText("The copy of Flash is moving to NAS")).toBeTruthy();
  });

  it("stays open with the sentence when the start is refused", async () => {
    moveSection.mockResolvedValue({ ok: false, code: "move-running", error: "running" });
    await renderPage();
    const dialog = await openMove();
    await click(within(dialog).getByRole("radio", { name: "NAS" }));

    await click(start(dialog));

    expect(within(dialog).getByText(/already moving/)).toBeTruthy();
  });

  it("starts nothing when it is cancelled", async () => {
    await renderPage();
    const dialog = await openMove();
    await click(within(dialog).getByRole("radio", { name: "NAS" }));

    await click(within(dialog).getByRole("button", { name: "Cancel" }));

    expect(moveSection).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});

describe("a section that is moving", () => {
  const frame = { key: "move:t-box-flash", phase: "replicate", percent: 40, active: true, snapshotIndex: 3, snapshotTotal: 9 };

  it("says so on its row with the backup the copy has reached, and offers no second move", async () => {
    await renderPage();

    progress(frame);

    expect(within(row("Flash")).getByRole("status").textContent).toBe("Moving · Backup 3 of 9");
    expect(within(row("Flash")).queryByRole("button", { name: "Move" })).toBeNull();
    expect(within(row("Containers")).getByRole("button", { name: "Move" })).toBeTruthy();
  });

  it("reads the location again once the move ended, which is when the section has left", async () => {
    await renderPage();
    progress(frame);
    getStorageLocation.mockResolvedValue({ ok: true, location: { ...BOX, sections: [BOX.sections[0]] } });
    const reads = getStorageLocation.mock.calls.length;
    vi.useFakeTimers();

    progress({ ...frame, percent: 100, active: false });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1000);
    });
    vi.useRealTimers();
    await act(async () => {});

    expect(getStorageLocation.mock.calls.length).toBeGreaterThan(reads);
    expect(screen.queryByText("Flash")).toBeNull();
  });

  it("says so under the name of a target that is all there is to the location", async () => {
    await renderPage(TYPED);

    progress({ key: "move:t-typed", phase: "replicate", percent: 0, active: true });

    expect(screen.getByRole("status").textContent).toBe("Moving");
    await click(screen.queryByRole("button", { name: "More" }) ?? document.body);
    expect(screen.queryByRole("menuitem", { name: "Move" })).toBeNull();
  });
});

describe("a destination whose backups are being deleted", () => {
  it("says so under its name and offers no second removal", async () => {
    await renderPage();

    progress({ key: "erase:box", phase: "maintenance", percent: 33, active: true, done: 1, total: 3 });

    expect(screen.getByRole("status").textContent).toBe("Backups are being deleted");
    expect(screen.queryByRole("button", { name: "More" })).toBeNull();
  });
});
