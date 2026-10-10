// @vitest-environment jsdom
/**
 * The page of one storage location.
 *
 * A location is read from one of four kinds of object, and each kind is
 * written through its own route. A card has to show only where its object has
 * something to say, and a change has to reach the route that owns it, with
 * what that route wants sent back.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";

import type { Destination, OffsiteTarget, Provider, Settings, StorageLocation, StorageLocationSection } from "../lib/api";
import { ApiError } from "../lib/api";
import { I18nProvider } from "../lib/i18n";
import { readAskKeepLess } from "../lib/keepAsk";
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
  return {
    ...actual,
    getStorageLocation: vi.fn(),
    listOffsiteTargets: vi.fn(),
    listDestinations: vi.fn(),
    updateDestination: vi.fn(),
    updateOffsiteTarget: vi.fn(),
    updateRepo: vi.fn(),
    getSettings: vi.fn(),
    putSettings: vi.fn(),
    getProviders: vi.fn(),
    getCloud: vi.fn(),
    getCloudCredSets: vi.fn(),
    setCloudCredSets: vi.fn(),
    getRclone: vi.fn(),
    testOffsiteTarget: vi.fn(),
    tamperTestOffsiteTarget: vi.fn(),
  };
});

const api = await import("../lib/api");
const getStorageLocation = vi.mocked(api.getStorageLocation);
const listOffsiteTargets = vi.mocked(api.listOffsiteTargets);
const listDestinations = vi.mocked(api.listDestinations);
const updateDestination = vi.mocked(api.updateDestination);
const updateOffsiteTarget = vi.mocked(api.updateOffsiteTarget);
const updateRepo = vi.mocked(api.updateRepo);
const getSettings = vi.mocked(api.getSettings);
const putSettings = vi.mocked(api.putSettings);
const getProviders = vi.mocked(api.getProviders);
const getCloud = vi.mocked(api.getCloud);
const getCloudCredSets = vi.mocked(api.getCloudCredSets);
const getRclone = vi.mocked(api.getRclone);
const testOffsiteTarget = vi.mocked(api.testOffsiteTarget);
const tamperTestOffsiteTarget = vi.mocked(api.tamperTestOffsiteTarget);

const { StorageLocationPage } = await import("./StorageLocation");

const GB = 1024 ** 3;
const LONG = { keepLast: 0, keepDaily: 14, keepWeekly: 8, keepMonthly: 12, keepYearly: 3 };
const SHORT = { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 3, keepYearly: 0 };
const BALANCED = { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 6, keepYearly: 1 };

function section(over: Partial<StorageLocationSection> = {}): StorageLocationSection {
  return {
    domain: "containers",
    use: "copy",
    where: "",
    enabled: true,
    immutable: false,
    retention: LONG,
    compression: "auto",
    limitUpload: 0,
    limitDownload: 0,
    own: [],
    ...over,
  };
}

function location(over: Partial<StorageLocation> = {}): StorageLocation {
  return {
    id: "destination:box",
    object: "destination",
    kind: "offsite",
    provider: "storagebox",
    mark: "IconHetzner",
    backend: "rclone",
    name: "Storage Box",
    where: "rclone:storagebox:bombvault",
    enabled: true,
    offPremises: true,
    sections: [
      section({ domain: "containers", targetId: "t-containers", primary: true, own: ["retention", "compression", "limits", "enabled"] }),
      section({ domain: "flash", targetId: "t-flash", retention: BALANCED, own: ["retention"] }),
    ],
    retention: LONG,
    compression: "auto",
    limitUpload: 0,
    limitDownload: 0,
    protection: { immutable: false, testable: false },
    capacity: { usedBytes: 268 * GB, totalBytes: 1024 * GB },
    ...over,
  };
}

function target(over: Partial<OffsiteTarget> = {}): OffsiteTarget {
  return {
    id: "t-flash",
    domain: "flash",
    name: "Storage Box",
    repo: "rclone:storagebox:bombvault/flash",
    credsRef: "",
    storageClass: "",
    immutable: false,
    schedule: "",
    retentionKeepLast: 0,
    retentionKeepDaily: 7,
    retentionKeepWeekly: 4,
    retentionKeepMonthly: 6,
    retentionKeepYearly: 1,
    compression: "auto",
    limitUpload: 0,
    limitDownload: 0,
    growthBudgetGb: 0,
    enabled: true,
    createdAt: 1,
    sortOrder: 1,
    destinationId: "box",
    ...over,
  };
}

const BOX = { id: "box", name: "Storage Box", storageClass: "", immutable: false } as Destination;
const PROVIDERS = [{ id: "storagebox", name: "Hetzner Storage Box", backend: "sftp", group: "server", route: "rclone" }] as Provider[];

const LOCAL = location({
  id: "path:abc",
  object: "path",
  kind: "local",
  provider: "",
  mark: undefined,
  backend: "local",
  name: "bombvault",
  where: "/mnt/user/backups/bombvault",
  offPremises: false,
  sections: [section({ use: "home" }), section({ domain: "vms", use: "home", retention: SHORT, own: ["retention"] })],
  retention: BALANCED,
  compression: undefined,
  limitUpload: undefined,
  limitDownload: undefined,
});

const REST = location({
  id: "target:t-rest",
  object: "target",
  provider: "",
  mark: undefined,
  backend: "rest",
  name: "NAS",
  where: "rest:https://nas:8000/tower",
  sections: [section({ domain: "vms", targetId: "t-rest", immutable: true })],
  protection: { immutable: true, testable: true, lastTamper: { at: Math.floor(Date.now() / 1000) - 3 * 86400, protected: true } },
});

function stage(loc: StorageLocation, targets: OffsiteTarget[] = [target()]) {
  getStorageLocation.mockResolvedValue({ ok: true, location: loc });
  listOffsiteTargets.mockResolvedValue({ ok: true, targets });
}

async function renderPage(id: string) {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <MemoryRouter initialEntries={[`/storage/${encodeURIComponent(id)}`]}>
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

/** The card whose heading badge reads `title`. */
function card(title: string): HTMLElement {
  const heading = screen.getAllByRole("heading", { level: 2 }).find((h) => h.textContent === title);
  if (!heading) throw new Error(`no card titled ${title}`);
  return heading.parentElement!;
}

function cardTitles(): string[] {
  return screen.getAllByRole("heading", { level: 2 }).map((h) => h.textContent ?? "");
}

async function click(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

function keepRule(): HTMLElement {
  return screen.getByRole("radiogroup", { name: "How long to keep" });
}

beforeEach(() => {
  localStorage.clear();
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({ ok: true }) }));
  listDestinations.mockResolvedValue({ ok: true, destinations: [BOX] });
  updateDestination.mockResolvedValue({ ok: true, warnings: [] });
  updateOffsiteTarget.mockResolvedValue({ ok: true, warnings: [] });
  updateRepo.mockResolvedValue({ ok: true });
  putSettings.mockResolvedValue({ ok: true });
  getSettings.mockResolvedValue({
    ok: true,
    settings: { flashOffsiteSchedule: "daily 03:00", containersOffsiteSchedule: "", ownRetention: { vms: SHORT } } as unknown as Settings,
    hostMountRoot: "",
    platform: "",
  });
  getProviders.mockResolvedValue({ ok: true, providers: PROVIDERS });
  getCloud.mockResolvedValue({ ok: true, restUser: "tower" });
  getCloudCredSets.mockResolvedValue({ ok: true, sets: [] });
  getRclone.mockResolvedValue({ ok: true, remotes: ["storagebox", "nas"] });
  testOffsiteTarget.mockResolvedValue({ ok: true, reachable: true, initialized: true });
  tamperTestOffsiteTarget.mockResolvedValue({ ok: true, testable: true, protected: true });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

describe("a destination", () => {
  beforeEach(() => stage(location()));

  it("says what it is and shows the cards of a place that takes copies", async () => {
    await renderPage("destination:box");

    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe("Storage Box");
    expect(await screen.findByText("Hetzner Storage Box · Used by Containers and Flash")).toBeTruthy();
    expect(screen.getByText("268.0 GB of 1.0 TB")).toBeTruthy();
    expect(cardTitles()).toEqual(["Connection", "rclone.conf", "Delete protection", "Copy", "How long to keep"]);
    expect(within(card("rclone.conf")).getByText("[storagebox]")).toBeTruthy();
    expect(within(card("rclone.conf")).getByText("storagebox · nas")).toBeTruthy();
  });

  it("asks the remote for its room once the page is up", async () => {
    await renderPage("destination:box");

    await waitFor(() => expect(getStorageLocation).toHaveBeenCalledWith("destination:box", true));
  });

  it("shows the address without a control to change it", async () => {
    await renderPage("destination:box");

    const connection = card("Connection");
    expect(within(connection).getByText("rclone:storagebox:bombvault")).toBeTruthy();
    expect(within(connection).queryByDisplayValue("rclone:storagebox:bombvault")).toBeNull();
  });

  it("saves a switch through the destination's route with what that route wants back", async () => {
    await renderPage("destination:box");

    await click(within(card("Connection")).getByRole("switch", { name: "Available" }));
    expect(updateDestination).toHaveBeenCalledWith("box", { name: "Storage Box", storageClass: "", immutable: false, enabled: false });
  });

  it("asks before a shorter rule is saved, and saves nothing when the answer is no", async () => {
    await renderPage("destination:box");
    expect(within(keepRule()).getByRole("radio", { name: "Long" }).getAttribute("aria-checked")).toBe("true");

    await click(within(keepRule()).getByRole("radio", { name: "Short" }));
    const dialog = screen.getByRole("dialog");
    expect(dialog.textContent).toContain("Keep less?");
    expect(dialog.textContent).toContain("Storage Box");

    await click(within(dialog).getByRole("button", { name: "Back" }));
    expect(updateDestination).not.toHaveBeenCalled();
    expect(within(keepRule()).getByRole("radio", { name: "Long" }).getAttribute("aria-checked")).toBe("true");
  });

  it("saves the shorter rule on yes and keeps asking", async () => {
    await renderPage("destination:box");

    await click(within(keepRule()).getByRole("radio", { name: "Short" }));
    await click(within(screen.getByRole("dialog")).getByRole("button", { name: "Save anyway" }));
    expect(updateDestination).toHaveBeenCalledWith("box", expect.objectContaining({ retention: SHORT }));
    expect(readAskKeepLess()).toBe(true);
  });

  it("stops asking when the answer says so, and the card's switch shows it", async () => {
    await renderPage("destination:box");

    await click(within(keepRule()).getByRole("radio", { name: "Short" }));
    const dialog = screen.getByRole("dialog");
    await click(within(dialog).getByRole("switch", { name: "Don't ask again" }));
    await click(within(dialog).getByRole("button", { name: "Save anyway" }));

    expect(readAskKeepLess()).toBe(false);
    const ask = within(card("How long to keep")).getByRole("switch", { name: "Ask before keeping less" });
    expect(ask.getAttribute("aria-checked")).toBe("false");

    await click(within(keepRule()).getByRole("radio", { name: "Balanced" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(updateDestination).toHaveBeenLastCalledWith("box", expect.objectContaining({ retention: BALANCED }));
  });

  it("does not remember the switch of a question answered with no", async () => {
    await renderPage("destination:box");

    await click(within(keepRule()).getByRole("radio", { name: "Short" }));
    const dialog = screen.getByRole("dialog");
    await click(within(dialog).getByRole("switch", { name: "Don't ask again" }));
    await click(within(dialog).getByRole("button", { name: "Back" }));
    expect(readAskKeepLess()).toBe(true);
  });

  it("saves a rule that keeps more without a question", async () => {
    stage(location({ retention: SHORT }));
    await renderPage("destination:box");

    await click(within(keepRule()).getByRole("radio", { name: "Long" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(updateDestination).toHaveBeenCalledWith("box", expect.objectContaining({ retention: LONG }));
  });

  it("shows the five counts under Custom and saves one that was changed", async () => {
    localStorage.setItem("bombvault.askKeepLess", "off");
    await renderPage("destination:box");

    await click(within(keepRule()).getByRole("radio", { name: "Custom values" }));
    expect(updateDestination).not.toHaveBeenCalled();
    const days = within(card("How long to keep")).getByRole("spinbutton", { name: "Days" });
    expect((days as HTMLInputElement).value).toBe("14");
    fireEvent.change(days, { target: { value: "30" } });

    await waitFor(
      () => expect(updateDestination).toHaveBeenCalledWith("box", expect.objectContaining({ retention: { ...LONG, keepDaily: 30 } })),
      { timeout: 3000 }
    );
  });

  it("lists a section's own value with the way back, and takes it through the target's route", async () => {
    await renderPage("destination:box");

    const keep = card("How long to keep");
    expect(within(keep).getByText("Custom values · Keeps 0 latest, 7 daily, 4 weekly, 6 monthly, 1 yearly.")).toBeTruthy();
    const back = within(keep).getAllByRole("button", { name: "Reset to the storage location" });
    // Containers is the copy its domain's off-site settings describe.
    expect(back).toHaveLength(1);

    await click(back[0]);
    expect(updateOffsiteTarget).toHaveBeenCalledWith("t-flash", target(), undefined, ["retention"]);
    expect(await screen.findByText("Back to the storage location’s rules")).toBeTruthy();
  });

  it("offers the primary copy no way back on any setting", async () => {
    await renderPage("destination:box");

    for (const title of ["Connection", "Copy"]) {
      expect(within(card(title)).getAllByText("Containers").length).toBeGreaterThan(0);
      expect(within(card(title)).queryByRole("button", { name: "Reset to the storage location" })).toBeNull();
    }
  });

  it("says when each section copies", async () => {
    await renderPage("destination:box");

    const when = within(card("Copy")).getByText("When to copy").closest("div.flex-wrap") as HTMLElement;
    expect(await within(when).findByText("After every backup")).toBeTruthy();
    expect(within(when).getByText("Daily at 03:00")).toBeTruthy();
  });

  it("tests the connection through one of its sections and shows the verdict", async () => {
    await renderPage("destination:box");

    await click(screen.getByRole("button", { name: "Test connection" }));
    expect(testOffsiteTarget).toHaveBeenCalledWith("t-containers");
    expect(await screen.findByRole("button", { name: "Connected" })).toBeTruthy();
  });

  it("turns delete protection on for every section that follows", async () => {
    await renderPage("destination:box");

    await click(within(card("Delete protection")).getByRole("switch", { name: "Delete protection (append-only)" }));
    expect(updateDestination).toHaveBeenCalledWith("box", expect.objectContaining({ immutable: true }));
    expect(await screen.findByText("Delete protection on, also for Containers and Flash")).toBeTruthy();
  });

  it("shows a failed save and leaves the switch where the server has it", async () => {
    updateDestination.mockResolvedValue({ ok: false, error: "destination is in use" });
    await renderPage("destination:box");

    await click(within(card("Connection")).getByRole("switch", { name: "Off the premises" }));
    expect(await screen.findByText("destination is in use")).toBeTruthy();
    expect(within(card("Connection")).getByRole("switch", { name: "Off the premises" }).getAttribute("aria-checked")).toBe("true");
  });
});

describe("a folder on this host", () => {
  beforeEach(() => stage(LOCAL, []));

  it("has only the cards a folder has", async () => {
    await renderPage("path:abc");

    expect(cardTitles()).toEqual(["Connection", "How long to keep"]);
    expect(screen.getByText("Local folder · Used by Containers and VMs")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Test connection" })).toBeNull();
    expect(getStorageLocation).toHaveBeenCalledTimes(1);
  });

  it("saves the shared local rule into the settings, without a question and without the switch for one", async () => {
    await renderPage("path:abc");

    expect(within(card("How long to keep")).queryByRole("switch")).toBeNull();
    await click(within(keepRule()).getByRole("radio", { name: "Short" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    await waitFor(() => expect(putSettings).toHaveBeenCalledWith(expect.objectContaining({ retentionKeepMonthly: 3, retentionKeepYearly: 0 })));
  });

  it("lets a section with a local rule of its own go back to the shared one", async () => {
    await renderPage("path:abc");

    await click(within(card("How long to keep")).getByRole("button", { name: "Reset to the storage location" }));
    await waitFor(() => expect(putSettings).toHaveBeenCalledWith(expect.objectContaining({ ownRetention: {} })));
  });
});

describe("a named repository", () => {
  const ARCHIVE = location({
    id: "repo:archive",
    object: "repo",
    kind: "local",
    provider: "",
    mark: undefined,
    backend: "local",
    name: "Archive",
    where: "/mnt/disks/archive",
    sections: [],
    retention: BALANCED,
    offPremises: false,
  });

  beforeEach(() => stage(ARCHIVE, []));

  it("shows the rule it ages by without offering to change it", async () => {
    await renderPage("repo:archive");

    expect(cardTitles()).toEqual(["Connection", "Delete protection", "How long to keep"]);
    expect(screen.queryByRole("radiogroup", { name: "How long to keep" })).toBeNull();
    expect(within(card("How long to keep")).getByText("Balanced")).toBeTruthy();
  });

  it("saves delete protection through the repository's route", async () => {
    await renderPage("repo:archive");

    await click(within(card("Delete protection")).getByRole("switch", { name: "Delete protection (append-only)" }));
    expect(updateRepo).toHaveBeenCalledWith("archive", { immutable: true });
  });
});

describe("a rest-server set up for one section", () => {
  const OWN = target({ id: "t-rest", domain: "vms", destinationId: undefined, repo: "rest:https://nas:8000/tower", immutable: true });

  beforeEach(() => {
    stage(REST, [OWN]);
    getCloudCredSets.mockResolvedValue({
      ok: true,
      sets: [{ id: "set-nas", name: "NAS login", s3KeyId: "", s3Region: "", restUser: "backup", s3StorageClass: "", s3SecretSet: false, restPasswordSet: true }],
    });
  });

  it("says when delete protection was last proven and proves it again against its own target", async () => {
    await renderPage("target:t-rest");

    const protection = card("Delete protection");
    expect(within(protection).getByText("Last: delete refused, 3 days ago")).toBeTruthy();
    await click(within(protection).getByRole("button", { name: "Test" }));
    expect(tamperTestOffsiteTarget).toHaveBeenCalledWith("t-rest");
    expect(await within(protection).findByRole("button", { name: "Passed" })).toBeTruthy();
  });

  it("says why when the far side does not refuse", async () => {
    tamperTestOffsiteTarget.mockResolvedValue({ ok: true, testable: true, protected: false, detail: "DELETE answered 200" });
    await renderPage("target:t-rest");

    await click(within(card("Delete protection")).getByRole("button", { name: "Test" }));
    expect(await screen.findByText("DELETE answered 200")).toBeTruthy();
    expect(within(card("Delete protection")).getByRole("button", { name: "Failed" })).toBeTruthy();
  });

  it("names the credentials in use and points the location at another set", async () => {
    await renderPage("target:t-rest");

    const credentials = card("Credentials");
    expect(await within(credentials).findByText("tower")).toBeTruthy();
    await click(within(credentials).getByRole("combobox", { name: "In use" }));
    await click(screen.getByRole("option", { name: "NAS login" }));
    expect(updateOffsiteTarget).toHaveBeenCalledWith("t-rest", expect.objectContaining({ credsRef: "set-nas", repo: OWN.repo }));
  });

  it("writes a changed rule back as the target's five columns", async () => {
    localStorage.setItem("bombvault.askKeepLess", "off");
    await renderPage("target:t-rest");

    await click(within(keepRule()).getByRole("radio", { name: "Short" }));
    expect(updateOffsiteTarget).toHaveBeenCalledWith("t-rest", expect.objectContaining({ retentionKeepDaily: 7, retentionKeepMonthly: 3 }));
  });
});

describe("the copy a section's off-site settings describe", () => {
  const PRIMARY = location({
    id: "target:t-primary",
    object: "target",
    provider: "",
    mark: undefined,
    backend: "sftp",
    name: "Old NAS",
    where: "sftp:backup@nas:/srv/restic",
    sections: [section({ domain: "files", targetId: "t-primary", primary: true })],
  });

  it("shows its values and offers nothing to change them with", async () => {
    stage(PRIMARY, [target({ id: "t-primary", domain: "files", destinationId: undefined, sortOrder: 0 })]);
    await renderPage("target:t-primary");

    expect(screen.queryByRole("switch")).toBeNull();
    expect(screen.queryByRole("radiogroup")).toBeNull();
    expect(screen.queryByRole("spinbutton")).toBeNull();
    expect(within(card("How long to keep")).getByText("Long")).toBeTruthy();
    expect(
      screen.getByLabelText(
        "This is the copy from the off-site field of Folders. You set its values in Settings, under Off-site and Retention. This page only shows them."
      )
    ).toBeTruthy();
  });
});

describe("a location that cannot be shown", () => {
  it("says so when it no longer exists, with the way back to the list", async () => {
    getStorageLocation.mockRejectedValue(new ApiError(404, "HTTP 404"));
    await renderPage("destination:gone");

    expect(screen.getByText("This storage location no longer exists.")).toBeTruthy();
    await click(screen.getByRole("link", { name: "Storage locations" }));
    expect(screen.getByText("the list")).toBeTruthy();
  });

  it("offers to try again when the read failed, and shows the location then", async () => {
    getStorageLocation.mockRejectedValueOnce(new Error("offline"));
    await renderPage("path:abc");
    expect(screen.getByText("The storage locations could not be loaded")).toBeTruthy();

    stage(LOCAL, []);
    await click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByText("Local folder · Used by Containers and VMs")).toBeTruthy();
  });
});
