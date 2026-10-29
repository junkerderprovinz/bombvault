// @vitest-environment jsdom
/**
 * The Storage locations page below its list: which section is backed up
 * where, the entries that left their section's default, and the window that
 * adds a location.
 *
 * Every line is a switch that one click turns on or off: it
 * only decides where new backups go.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useParams } from "react-router-dom";

import type { Container, OffsiteTarget, Provider, StorageLocation, StorageLocationSection } from "../lib/api";
import { I18nProvider, en } from "../lib/i18n";
import {
  defaultImpact,
  defaultRow,
  placementView,
  stubEventSource,
  targetImpact,
} from "../lib/placement.testsupport";
import { ToastProvider } from "../lib/toast";

stubEventSource();

const fake = await vi.hoisted(async () => (await import("../lib/placement.testsupport")).createPlacementApi());

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  ...fake.api,
  listStorageLocations: vi.fn(),
  getStreaming: vi.fn(),
  listOffsiteTargets: vi.fn(),
  listContainers: vi.fn(),
  listVMs: vi.fn(),
  listFileSets: vi.fn(),
  getProviders: vi.fn(),
  listGroupReceivers: vi.fn(),
  groupReceiverLogin: vi.fn(),
  checkDraft: vi.fn(),
  createDestination: vi.fn(),
  getRclone: vi.fn(),
  setRclone: vi.fn(),
}));

const api = await import("../lib/api");
const listStorageLocations = vi.mocked(api.listStorageLocations);
const listOffsiteTargets = vi.mocked(api.listOffsiteTargets);
const listContainers = vi.mocked(api.listContainers);
const getProviders = vi.mocked(api.getProviders);
const listGroupReceivers = vi.mocked(api.listGroupReceivers);
const checkDraft = vi.mocked(api.checkDraft);
const createDestination = vi.mocked(api.createDestination);
const getRclone = vi.mocked(api.getRclone);
const setRclone = vi.mocked(api.setRclone);

const { StoragePage } = await import("./Storage");

const KEEP = { keepLast: 0, keepDaily: 7, keepWeekly: 4, keepMonthly: 6, keepYearly: 1 };

function section(over: Partial<StorageLocationSection>): StorageLocationSection {
  return {
    domain: "containers",
    use: "home",
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
    id: "path:local",
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

const LOCAL = location({ sections: [section({ domain: "containers" }), section({ domain: "flash" })] });
const B2 = location({
  id: "destination:b2",
  object: "destination",
  kind: "offsite",
  backend: "s3",
  name: "B2",
  offPremises: true,
  sections: [
    section({ domain: "containers", use: "copy", targetId: "t-b2" }),
    section({ domain: "flash", use: "copy", targetId: "t-b2-flash" }),
  ],
});
const BOX = location({ id: "destination:box", object: "destination", kind: "offsite", backend: "rclone", name: "Storage Box", offPremises: true });

const FLASH_TARGET = { id: "t-b2-flash", domain: "flash", name: "B2", enabled: true, destinationId: "b2" } as OffsiteTarget;

function container(name: string, placement = placementView()): Container {
  return { name, placement } as Container;
}

const PROVIDERS = [
  { id: "gdrive", name: "Google Drive", backend: "drive", group: "cloud", route: "rclone" },
  { id: "b2", name: "Backblaze B2", backend: "", group: "storage", route: "s3", lock: true },
  { id: "rest", name: "Rest server", backend: "", group: "server", route: "rest", lock: true },
  { id: "path", name: "Mounted path", backend: "", group: "server", route: "path" },
] as Provider[];

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
  await act(async () => {});
}

function line(name: string): HTMLElement {
  return screen.getByRole("switch", { name });
}

async function click(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

beforeEach(() => {
  localStorage.clear();
  fake.reset();
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({ ok: true }) }));
  listStorageLocations.mockResolvedValue({ ok: true, locations: [LOCAL, B2, BOX] });
  vi.mocked(api.getStreaming).mockResolvedValue({ ok: false });
  listOffsiteTargets.mockResolvedValue({ ok: true, targets: [FLASH_TARGET] });
  listContainers.mockResolvedValue({ ok: true, containers: [container("gitea")] });
  vi.mocked(api.listVMs).mockResolvedValue({ ok: true, vms: [] });
  vi.mocked(api.listFileSets).mockResolvedValue({ ok: true, fileSets: [] });
  getProviders.mockResolvedValue({ ok: true, providers: PROVIDERS, backends: {} });
  listGroupReceivers.mockResolvedValue({ ok: true, receivers: [] });
  getRclone.mockResolvedValue({ ok: true, remotes: [] });
  checkDraft.mockResolvedValue({ ok: true, reachable: true, initialized: false });
  createDestination.mockResolvedValue({ ok: true, destination: { id: "new", name: "Rest server" } as never });
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

describe("the flow", () => {
  it("draws a line from each section to every place it can be backed up to, on or off", async () => {
    await renderPage();

    expect(line("Containers to Local").getAttribute("aria-checked")).toBe("true");
    expect(line("Containers to B2").getAttribute("aria-checked")).toBe("true");
    expect(line("Containers to Storage Box").getAttribute("aria-checked")).toBe("false");
    expect(line("Flash to B2").getAttribute("aria-checked")).toBe("true");
    expect(line("Flash to Storage Box").getAttribute("aria-checked")).toBe("false");
  });

  it("counts copies, media and off-site places for the section with the most", async () => {
    await renderPage();

    const count = screen.getByText("3").closest("p")!;
    expect(count.textContent).toContain("3 copies");
    expect(count.textContent).toContain("2 media");
    expect(count.textContent).toContain("1 off-site");
  });

  it("switches a line off with one click and no window, and saves with the numbers the server named", async () => {
    const impact = defaultImpact({ dropped: [targetImpact({ items: 15, snapshots: 210 })] });
    fake.reply("previewPlacementDefault", { ok: true, impact });
    await renderPage();

    await click(line("Containers to B2"));

    await waitFor(() => expect(fake.callsTo("putPlacementDefault")).toEqual([["containers", { skip: ["*"] }, impact]]));
    expect(fake.callsTo("previewPlacementDefault")).toEqual([["containers", { skip: ["*"] }]]);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("writes again with the fresh numbers when the server calls the first ones stale", async () => {
    const fresh = defaultImpact({ dropped: [targetImpact({ items: 16 })] });
    fake.reply("putPlacementDefault", { ok: false, code: "stale", impact: fresh }, { ok: true });
    await renderPage();

    await click(line("Containers to B2"));

    await waitFor(() => expect(fake.callsTo("putPlacementDefault")).toHaveLength(2));
    expect(fake.callsTo("putPlacementDefault")[1]).toEqual(["containers", { skip: ["*"] }, fresh]);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("saves without numbers when the preview did not come back, then with the numbers the server names", async () => {
    const impact = defaultImpact({ dropped: [targetImpact({ items: 15, snapshots: 210 })] });
    fake.reply("previewPlacementDefault", new api.ApiError(502, en["apiError.gateway"]));
    fake.reply("putPlacementDefault", { ok: false, code: "stale", impact }, { ok: true });
    await renderPage();

    await click(line("Containers to B2"));

    await waitFor(() => expect(fake.callsTo("putPlacementDefault")).toHaveLength(2));
    expect(fake.callsTo("putPlacementDefault")).toEqual([
      ["containers", { skip: ["*"] }, undefined],
      ["containers", { skip: ["*"] }, impact],
    ]);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("says in a message why the server refused, writes nothing and leaves the line as it was", async () => {
    fake.reply("previewPlacementDefault", new api.ApiError(403, en["auth.relayLoginNeeded"], "own-login-needed"));
    await renderPage();

    await click(line("Containers to B2"));

    expect(await screen.findByText(en["auth.relayLoginNeeded"])).toBeTruthy();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(fake.callsTo("putPlacementDefault")).toEqual([]);
    expect(line("Containers to B2").getAttribute("aria-checked")).toBe("true");
  });

  it("asks before switching Local off makes the target's direct repository, and makes it the home on Create and use", async () => {
    await renderPage();

    await click(line("Containers to Local"));

    expect(await screen.findByText("Direct repository at B2")).toBeTruthy();
    expect(fake.callsTo("createDirectRepo")).toEqual([]);
    expect(fake.callsTo("putPlacementDefault")).toEqual([]);
    await screen.findByDisplayValue("b2:bucket:containers-direct");
    await click(screen.getByRole("button", { name: "Create and use" }));

    await waitFor(() => expect(fake.callsTo("putPlacementDefault")).toHaveLength(1));
    expect(fake.callsTo("createDirectRepo")).toEqual([["t-b2", "", "b2:bucket:containers-direct"]]);
    expect(fake.callsTo("putPlacementDefault")[0].slice(0, 2)).toEqual(["containers", { home: "repo-direct", skip: ["*"] }]);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("leaves everything as it was when the direct repository dialog is cancelled", async () => {
    await renderPage();

    await click(line("Containers to Local"));
    await screen.findByText("Direct repository at B2");
    await click(screen.getByRole("button", { name: "Cancel" }));

    expect(screen.queryByText("Direct repository at B2")).toBeNull();
    expect(fake.callsTo("createDirectRepo")).toEqual([]);
    expect(fake.callsTo("putPlacementDefault")).toEqual([]);
    expect(line("Containers to Local").getAttribute("aria-checked")).toBe("true");
  });

  it("offers the place the server picks at a rest-server user root", async () => {
    fake.reply("getDirectRepo", { ok: true, repo: null, suggestion: { location: "rest:https://box:8000/jdp/containers-direct" } });
    await renderPage();

    await click(line("Containers to Local"));
    await screen.findByDisplayValue("rest:https://box:8000/jdp/containers-direct");
    await click(screen.getByRole("button", { name: "Create and use" }));

    await waitFor(() => expect(fake.callsTo("putPlacementDefault")).toHaveLength(1));
    expect(fake.callsTo("createDirectRepo")).toEqual([["t-b2", "", "rest:https://box:8000/jdp/containers-direct"]]);
  });

  it("shows the line busy until the server has saved the new home", async () => {
    const release = fake.hold("putPlacementDefault");
    await renderPage();

    await click(line("Containers to Local"));
    await screen.findByDisplayValue("b2:bucket:containers-direct");
    await click(screen.getByRole("button", { name: "Create and use" }));

    await waitFor(() => expect(line("Containers to Local").getAttribute("aria-busy")).toBe("true"));
    const busy = line("Containers to Local");
    expect(busy.querySelector(".animate-spin")).toBeTruthy();
    expect(line("Containers to B2").getAttribute("aria-busy")).toBeNull();
    await act(async () => release());
    await waitFor(() => expect(line("Containers to Local").getAttribute("aria-busy")).toBeNull());
    expect(line("Containers to Local").querySelector(".animate-spin")).toBeNull();
  });

  it("shows the busy switch in the table of switches too", async () => {
    const release = fake.hold("putPlacementDefault");
    await renderPage();
    await click(screen.getByRole("radio", { name: "Switches" }));
    const containers = screen.getByRole("group", { name: "Containers" });

    await click(within(containers).getByRole("button", { name: /^Local/ }));
    await screen.findByDisplayValue("b2:bucket:containers-direct");
    await click(screen.getByRole("button", { name: "Create and use" }));

    await waitFor(() => expect(within(containers).getByRole("button", { name: /^Local/ }).querySelector(".animate-spin")).toBeTruthy());
    expect(within(containers).getByRole("button", { name: "B2" }).querySelector(".animate-spin")).toBeNull();
    await act(async () => release());
    await waitFor(() => expect(within(containers).getByRole("button", { name: /^Local/ }).querySelector(".animate-spin")).toBeNull());
  });

  it("lets every line of a section go off, and says the section is not backed up", async () => {
    const nowhere = defaultRow({ home: "none", homeKind: "none" });
    fake.reply("listPlacementDefaults", { ok: true, defaults: [nowhere, defaultRow({ domain: "vms" }), defaultRow({ domain: "files" })] });
    await renderPage();

    expect(line("Containers to Local").getAttribute("aria-checked")).toBe("false");
    expect(line("Containers to B2").getAttribute("aria-checked")).toBe("false");
    expect(screen.getByText(`Containers: ${en["storage.flow.nowhere"]}`)).toBeTruthy();

    await click(line("Containers to Local"));
    await waitFor(() => expect(fake.callsTo("putPlacementDefault")).toHaveLength(1));
    expect(fake.callsTo("putPlacementDefault")[0].slice(0, 2)).toEqual(["containers", { home: "", skip: ["*"] }]);
  });

  it("switches a line on for a destination without a target: makes the target and lights it as a copy", async () => {
    await renderPage();

    await click(line("Containers to Storage Box"));

    await waitFor(() => expect(fake.callsTo("putPlacementDefault")).toHaveLength(1));
    expect(fake.callsTo("destinationForDomain")).toEqual([["box", "containers"]]);
    expect(fake.callsTo("putPlacementDefault")[0].slice(0, 2)).toEqual(["containers", { skip: [] }]);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("reads the locations again after a change", async () => {
    await renderPage();
    expect(listStorageLocations).toHaveBeenCalledTimes(1);

    await click(line("Containers to B2"));

    await waitFor(() => expect(fake.callsTo("putPlacementDefault")).toHaveLength(1));
    await waitFor(() => expect(listStorageLocations).toHaveBeenCalledTimes(2));
  });

  it("switches the own folder of a section without placement off and on again", async () => {
    await renderPage();

    const local = line("Flash to Local");
    expect(local.getAttribute("aria-disabled")).toBeNull();
    expect(local.getAttribute("tabindex")).toBe("0");
    await click(local);
    await waitFor(() => expect(fake.callsTo("setSectionLocal")).toEqual([["flash", false]]));

    listStorageLocations.mockResolvedValue({ ok: true, locations: [{ ...LOCAL, sections: [section({ domain: "containers" }), section({ domain: "flash", off: true })] }, B2, BOX] });
    await act(async () => {
      window.dispatchEvent(new Event("bv:storage-locations-changed"));
    });
    await waitFor(() => expect(line("Flash to Local").getAttribute("aria-checked")).toBe("false"));
    await click(line("Flash to Local"));
    await waitFor(() => expect(fake.callsTo("setSectionLocal")).toEqual([["flash", false], ["flash", true]]));
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("switches the copy of a section without placement with its target's own switch", async () => {
    await renderPage();

    await click(line("Flash to B2"));

    await waitFor(() => expect(fake.callsTo("updateOffsiteTarget")).toEqual([["t-b2-flash", { ...FLASH_TARGET, enabled: false }]]));
    expect(listOffsiteTargets).toHaveBeenCalledWith("flash");
    expect(fake.callsTo("previewPlacementDefault")).toEqual([]);
  });

  it("switches the copy the section's off-site settings describe like any other", async () => {
    const primary = { ...B2, sections: [section({ domain: "containers", use: "copy", targetId: "t-b2" }), section({ domain: "flash", use: "copy", targetId: "t-b2-flash", primary: true })] };
    listStorageLocations.mockResolvedValue({ ok: true, locations: [LOCAL, primary, BOX] });
    await renderPage();

    await click(line("Flash to B2"));

    await waitFor(() => expect(fake.callsTo("updateOffsiteTarget")).toEqual([["t-b2-flash", { ...FLASH_TARGET, enabled: false }]]));
  });

  it("gives such a section a target under a destination that has none for it", async () => {
    await renderPage();

    await click(line("Flash to Storage Box"));

    await waitFor(() => expect(fake.callsTo("destinationForDomain")).toEqual([["box", "flash"]]));
  });

  it("answers the keyboard like a switch", async () => {
    await renderPage();

    await act(async () => {
      fireEvent.keyDown(line("Containers to B2"), { key: " " });
    });

    await waitFor(() => expect(fake.callsTo("previewPlacementDefault")).toEqual([["containers", { skip: ["*"] }]]));
  });

  it("gives the choice of view a colour of its own under rainbow", async () => {
    await renderPage();

    expect(screen.getByRole("radio", { name: "Graph" }).classList.contains("glim-hue")).toBe(true);
  });

  it("brings the table of switches back and remembers the choice", async () => {
    await renderPage();

    await click(screen.getByRole("radio", { name: "Switches" }));

    expect(screen.queryByRole("switch", { name: "Containers to B2" })).toBeNull();
    expect(localStorage.getItem("bombvault.flowView")).toBe("switches");
    const containers = screen.getByRole("group", { name: "Containers" });
    expect(within(containers).getByRole("button", { name: "B2" }).getAttribute("aria-pressed")).toBe("true");
    expect(within(containers).getByRole("button", { name: "Storage Box" }).getAttribute("aria-pressed")).toBe("false");
    const flash = screen.getByRole("group", { name: "Flash" });
    expect((within(flash).getByRole("button", { name: /^Local/ }) as HTMLButtonElement).disabled).toBe(false);

    await click(within(containers).getByRole("button", { name: "B2" }));
    await waitFor(() => expect(fake.callsTo("previewPlacementDefault")).toEqual([["containers", { skip: ["*"] }]]));

    cleanup();
    await renderPage();
    expect(screen.getByRole("radio", { name: "Switches" }).getAttribute("aria-checked")).toBe("true");
    expect(screen.getByRole("group", { name: "Containers" })).toBeTruthy();
  });

  it("is left out while no section is backed up at all", async () => {
    listStorageLocations.mockResolvedValue({ ok: true, locations: [BOX] });
    await renderPage();

    expect(screen.queryByText("Storage locations per section")).toBeNull();
  });
});

describe("entries that differ", () => {
  const own = placementView({ copiesFollow: false, skip: ["*"] });

  it("are not mentioned while every entry follows its section", async () => {
    await renderPage();

    expect(screen.queryByText(/entr(y|ies) differs?$/)).toBeNull();
    expect(document.querySelector('[data-new="storage-flow"]')?.getAttribute("data-search-card")).toBe("Storage locations per section");
  });

  it("are counted under the section, and one leads to the settings of that entry", async () => {
    listContainers.mockResolvedValue({ ok: true, containers: [container("gitea"), container("jellyfin", own)] });
    await renderPage();

    const hint = await screen.findByRole("link", { name: "1 entry differs" });
    expect(hint.getAttribute("href")).toBe("/backups/container/jellyfin?tab=settings");
    expect(hint.getAttribute("data-new")).toBe("storage-flow");
    expect(hint.closest("p")!.textContent).toContain("Containers:");
  });

  it("open a list where there are several, with what each uses and the way to its settings", async () => {
    const fixed = placementView({ repo: "repo-nas", repoKind: "local", repoLabel: "NAS Keller", locked: true, lockReason: "first-backup" });
    listContainers.mockResolvedValue({
      ok: true,
      containers: [container("gitea"), container("jellyfin", own), container("plex", fixed)],
    });
    await renderPage();

    await click(await screen.findByRole("button", { name: "2 entries differ" }));
    const window = await screen.findByRole("dialog", { name: "Containers: 2 entries differ" });
    const jellyfin = within(window).getByRole("link", { name: "jellyfin" });
    expect(jellyfin.getAttribute("href")).toBe("/backups/container/jellyfin?tab=settings");
    expect(jellyfin.closest("li")!.textContent).toContain("Backs up to Local · Default: Local and B2");
    expect(within(window).getByRole("link", { name: "plex" }).getAttribute("href")).toBe("/backups/container/plex?tab=settings");
    expect(within(window).queryByRole("link", { name: "gitea" })).toBeNull();
  });

  it("count only the entries with a choice of their own once the section went elsewhere", async () => {
    const moved = defaultRow({ home: "repo-direct", homeKind: "direct", homeTarget: "t-b2" });
    fake.reply("listPlacementDefaults", { ok: true, defaults: [moved, defaultRow({ domain: "vms" }), defaultRow({ domain: "files" })] });
    const followed = placementView({ repo: "repo-direct", repoKind: "direct", homeFollows: true, segment: "offsite-only" });
    const kept = placementView({ repo: "repo-nas", repoKind: "local", repoLabel: "NAS Keller" });
    listContainers.mockResolvedValue({
      ok: true,
      containers: [container("gitea", followed), container("immich", followed), container("jellyfin", own), container("plex", kept)],
    });
    await renderPage();

    await click(await screen.findByRole("button", { name: "2 entries differ" }));
    const window = await screen.findByRole("dialog", { name: "Containers: 2 entries differ" });
    expect(within(window).getAllByRole("link").map((a) => a.textContent)).toEqual(["jellyfin", "plex"]);
  });

  it("show in the switches as well", async () => {
    listContainers.mockResolvedValue({ ok: true, containers: [container("jellyfin", own)] });
    await renderPage();
    await click(screen.getByRole("radio", { name: "Switches" }));

    expect(await screen.findByRole("link", { name: "1 entry differs" })).toBeTruthy();
  });
});

describe("adding a storage location", () => {
  async function open(): Promise<HTMLElement> {
    await renderPage();
    await click(screen.getByRole("button", { name: "Add storage location" }));
    const window = await screen.findByRole("dialog", { name: "Add storage location" });
    await within(window).findByRole("button", { name: /^Rest server/ });
    return window;
  }

  function tileNames(window: HTMLElement): string[] {
    return [...window.querySelectorAll(".glim-provider-pick button")].map((b) => (b.textContent ?? "").trim());
  }

  it("opens from the list's own button with the providers in their groups, a local folder first and an own rclone configuration last", async () => {
    const window = await open();

    const groups = within(window).getAllByRole("heading", { level: 3 }).map((h) => h.textContent ?? "");
    expect(groups.map((g) => g.replace(/(best|good) fit|works, but slower/, ""))).toEqual([
      "Local",
      "NAS or server",
      "S3 service",
      "Cloud storage",
      "Own rclone configuration",
    ]);
    expect(tileNames(window)).toEqual(["Local folder", "Rest server", "Backblaze B2", "Google Drive", "Rclone"]);
  });

  it("saves a pasted rclone configuration and lists its remotes", async () => {
    getRclone.mockResolvedValue({ ok: true, remotes: ["b2"] });
    setRclone.mockResolvedValue({ ok: true });
    const picker = await open();
    await waitFor(() => expect(tileNames(picker)).toContain("Rcloneb2"));
    await click(within(picker).getByRole("button", { name: /^Rclone/ }));

    const window = await screen.findByRole("dialog", { name: "Add storage location" });
    expect(within(window).getByText("b2")).toBeTruthy();
    const paste = within(window).getByRole("textbox", { name: "Paste a config" });
    await act(async () => {
      fireEvent.change(paste, { target: { value: "[nas]\ntype = smb" } });
    });
    getRclone.mockResolvedValue({ ok: true, remotes: ["b2", "nas"] });
    await click(within(window).getByRole("button", { name: "Save config" }));

    expect(setRclone).toHaveBeenCalledWith("[nas]\ntype = smb");
    expect(await within(window).findByText("nas")).toBeTruthy();
    expect((paste as HTMLTextAreaElement).value).toBe("");
    await click(within(window).getByRole("button", { name: "Back" }));
    expect(tileNames(await screen.findByRole("dialog", { name: "Add storage location" }))).toContain("Rcloneb2, nas");
  });

  it("keeps a storage a paired instance offers as a choice, and fills its sign-in", async () => {
    listGroupReceivers.mockResolvedValue({
      ok: true,
      receivers: [{ memberId: "m-tower", name: "tower-2", url: "https://tower-2.local:8000", needsAddress: false }],
    });
    vi.mocked(api.groupReceiverLogin).mockResolvedValue({
      ok: true,
      login: { name: "tower-2", url: "https://tower-2.local:8000/cellar", user: "cellar", password: "s3cret" },
    });
    const window = await open();

    expect(within(window).getByText("From your group")).toBeTruthy();
    await click(within(window).getByRole("button", { name: /^tower-2/ }));

    expect(await within(window).findByDisplayValue("https://tower-2.local:8000/cellar")).toBeTruthy();
    expect(within(window).getByText("Sign in to Rest server")).toBeTruthy();
  });

  it("says a paired instance known only by a browser's name has to set its receiving server's address", async () => {
    listGroupReceivers.mockResolvedValue({
      ok: true,
      receivers: [{ memberId: "m-tower", name: "tower-2", needsAddress: false, needsHost: true }],
    });
    const window = await open();

    expect(within(window).getByLabelText(en["dest.receiver.needsHostTip"])).toBeTruthy();
    expect(within(window).queryByLabelText(en["dest.receiver.needsAddressTip"])).toBeNull();
  });

  it("narrows the tiles to what the search asks for", async () => {
    const window = await open();

    await act(async () => {
      fireEvent.change(within(window).getByRole("searchbox", { name: "Search providers" }), { target: { value: "back" } });
    });
    expect(tileNames(window)).toEqual(["Backblaze B2"]);

    await act(async () => {
      fireEvent.change(within(window).getByRole("searchbox", { name: "Search providers" }), { target: { value: "zzz" } });
    });
    expect(tileNames(window)).toEqual([]);
    expect(within(window).getByText(/^No provider matches\./)).toBeTruthy();
  });

  it("walks the steps in one window and holds the later ones back until the connection is tested", async () => {
    const window = await open();
    await click(within(window).getByRole("button", { name: /^Rest server/ }));

    expect(within(window).getByText("Sign in to Rest server")).toBeTruthy();
    expect((within(window).getByRole("radio", { name: "Folder" }) as HTMLButtonElement).disabled).toBe(true);
    const foot = within(window).getAllByRole("button").map((b) => b.textContent);
    expect(foot.slice(-3)).toEqual(["Cancel", "Back", "Test connection"]);

    await click(within(window).getByRole("button", { name: "Test connection" }));
    expect(await within(window).findByText("Connection established", { exact: false })).toBeTruthy();
    expect(within(window).getByRole("radio", { name: "Folder" }).getAttribute("aria-checked")).toBe("true");

    await click(within(window).getByRole("button", { name: "Back" }));
    expect(within(window).getByText("Sign in to Rest server")).toBeTruthy();
    await click(within(window).getByRole("button", { name: "Back" }));
    expect(tileNames(window)).toContain("Local folder");
  });

  it("asks last which sections use the new place and switches it on for exactly those", async () => {
    const window = await open();
    await click(within(window).getByRole("button", { name: /^Rest server/ }));
    await click(within(window).getByRole("button", { name: "Test connection" }));
    await within(window).findByText("Connection established", { exact: false });
    await click(within(window).getByRole("button", { name: "Next" }));
    expect(within(window).getByText("Protection against deletion")).toBeTruthy();
    await click(within(window).getByRole("button", { name: "Next" }));

    expect(within(window).getByText("Which sections back up here?")).toBeTruthy();
    expect(await within(window).findByText("15 entries · up to 420 snapshots")).toBeTruthy();
    expect(within(window).getByRole("switch", { name: "Containers" }).getAttribute("aria-checked")).toBe("true");
    await click(within(window).getByRole("switch", { name: "Flash" }));
    expect(createDestination).not.toHaveBeenCalled();

    await click(within(window).getByRole("button", { name: "Save storage location" }));

    expect(await screen.findByText("opened destination:new")).toBeTruthy();
    expect(createDestination).toHaveBeenCalledTimes(1);
    expect(fake.callsTo("destinationForDomain")).toEqual([["new", "containers"]]);
    expect(fake.callsTo("previewPlacementDefault")).toEqual([["containers", { skip: [] }]]);
    expect(fake.callsTo("putPlacementDefault")).toEqual([["containers", { skip: [] }, defaultImpact()]]);
  });

  it("closes without saving anything on Cancel", async () => {
    const window = await open();
    await click(within(window).getByRole("button", { name: "Cancel" }));

    expect(screen.queryByRole("dialog")).toBeNull();
    expect(createDestination).not.toHaveBeenCalled();
  });
});
