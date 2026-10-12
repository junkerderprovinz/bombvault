// @vitest-environment jsdom
/**
 * The Storage locations page below its list: which section is backed up
 * where, the entries that left their section's default, and the window that
 * adds a location.
 *
 * A line is a switch on a section's default, so a click has to go through the
 * same preview and confirmation as every other change to a default, and a
 * line the server would refuse must not take a click at all.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useParams } from "react-router-dom";

import type { Container, OffsiteTarget, Provider, StorageLocation, StorageLocationSection } from "../lib/api";
import { I18nProvider } from "../lib/i18n";
import {
  defaultImpact,
  defaultRow,
  placementOptions,
  placementView,
  stubEventSource,
  targetImpact,
  uploadEstimate,
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
}));

const api = await import("../lib/api");
const listStorageLocations = vi.mocked(api.listStorageLocations);
const listOffsiteTargets = vi.mocked(api.listOffsiteTargets);
const listContainers = vi.mocked(api.listContainers);
const getProviders = vi.mocked(api.getProviders);
const listGroupReceivers = vi.mocked(api.listGroupReceivers);
const checkDraft = vi.mocked(api.checkDraft);
const createDestination = vi.mocked(api.createDestination);

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

  it("asks before a line goes off, names what the target stops getting and saves with those numbers", async () => {
    const impact = defaultImpact({ dropped: [targetImpact({ items: 15, snapshots: 210 })] });
    fake.reply("previewPlacementDefault", { ok: true, impact });
    await renderPage();

    await click(line("Containers to B2"));
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("Items and project folders that B2 no longer gets: 15. Copies that stay there: 210.");
    expect(fake.callsTo("putPlacementDefault")).toEqual([]);
    await click(within(dialog).getByRole("button", { name: "Confirm" }));

    await waitFor(() => expect(fake.callsTo("putPlacementDefault")).toEqual([["containers", { skip: ["*"] }, impact]]));
    expect(fake.callsTo("previewPlacementDefault")).toEqual([["containers", { skip: ["*"] }]]);
  });

  it("opens the question at once and fills it in when the server has worked out the impact", async () => {
    const release = fake.hold("previewPlacementDefault");
    await renderPage();

    await click(line("Containers to B2"));
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain("Checking…");
    expect((within(dialog).getByRole("button", { name: /Confirm/ }) as HTMLButtonElement).disabled).toBe(true);

    await act(async () => release({ ok: true, impact: defaultImpact({ dropped: [targetImpact({ items: 15, snapshots: 210 })] }) }));
    expect(dialog.textContent).toContain("Items and project folders that B2 no longer gets: 15.");
    expect((within(dialog).getByRole("button", { name: "Confirm" }) as HTMLButtonElement).disabled).toBe(false);
  });

  it("writes nothing when the window is closed before the impact is in", async () => {
    const release = fake.hold("previewPlacementDefault");
    await renderPage();

    await click(line("Containers to B2"));
    await click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Cancel" }));
    await act(async () => release({ ok: true, impact: defaultImpact({ dropped: [targetImpact()] }) }));

    expect(screen.queryByRole("dialog")).toBeNull();
    expect(fake.callsTo("putPlacementDefault")).toEqual([]);
  });

  it("writes nothing when the question is answered with no", async () => {
    fake.reply("previewPlacementDefault", { ok: true, impact: defaultImpact({ dropped: [targetImpact()] }) });
    await renderPage();

    await click(line("Containers to B2"));
    await click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Cancel" }));

    expect(fake.callsTo("putPlacementDefault")).toEqual([]);
    expect(line("Containers to B2").getAttribute("aria-checked")).toBe("true");
  });

  it("switches a line on for a destination without a target: makes the target, then asks with what it will receive", async () => {
    const impact = defaultImpact({ added: [targetImpact({ targetId: "t-new", name: "Storage Box", items: 7, snapshots: 32 })] });
    fake.reply("previewPlacementDefault", { ok: true, impact });
    await renderPage();

    await click(line("Containers to Storage Box"));
    const dialog = await screen.findByRole("dialog");
    expect(fake.callsTo("destinationForDomain")).toEqual([["box", "containers"]]);
    expect(dialog.textContent).toContain("Storage Box gets from now on: 7.");
    await click(within(dialog).getByRole("button", { name: "Confirm" }));

    await waitFor(() => expect(fake.callsTo("putPlacementDefault")).toEqual([["containers", { skip: [] }, impact]]));
  });

  it("saves a change that affects nothing without a question, and reads the locations again", async () => {
    await renderPage();
    expect(listStorageLocations).toHaveBeenCalledTimes(1);

    await click(line("Containers to B2"));

    await waitFor(() => expect(fake.callsTo("putPlacementDefault")).toHaveLength(1));
    expect(screen.queryByRole("dialog")).toBeNull();
    await waitFor(() => expect(listStorageLocations).toHaveBeenCalledTimes(2));
  });

  it("marks a line that cannot be switched, says why in a bubble and takes no click", async () => {
    await renderPage();

    const locked = line("Flash to Local");
    expect(locked.getAttribute("aria-disabled")).toBe("true");
    expect(locked.getAttribute("aria-checked")).toBe("true");
    expect(locked.getAttribute("tabindex")).toBeNull();
    expect(screen.getByLabelText("Flash always backs up to its own folder first. The off-site copies come from that backup, so it stays on.")).toBeTruthy();

    await click(locked);
    expect(fake.calls.filter((c) => c.fn.startsWith("put") || c.fn.startsWith("update"))).toEqual([]);
  });

  it("locks Local where no target could take the backups in its place", async () => {
    const alone = placementOptions({ sendTo: [], segmentLocks: { "offsite-only": "no-target" } });
    fake.reply("getPlacementOptions", { ok: true, options: alone }, { ok: true, options: alone }, { ok: true, options: alone });
    fake.reply("listPlacementDefaults", { ok: true, defaults: [defaultRow({ skip: ["*"] }), defaultRow({ domain: "vms" }), defaultRow({ domain: "files" })] });
    await renderPage();

    expect(line("Containers to Local").getAttribute("aria-disabled")).toBe("true");
    await click(line("Containers to Local"));
    expect(fake.callsTo("previewPlacementDefault")).toEqual([]);
  });

  it("switches the copy of a section without placement with its target's own switch", async () => {
    await renderPage();

    await click(line("Flash to B2"));

    await waitFor(() => expect(fake.callsTo("updateOffsiteTarget")).toEqual([["t-b2-flash", { ...FLASH_TARGET, enabled: false }]]));
    expect(listOffsiteTargets).toHaveBeenCalledWith("flash");
    expect(fake.callsTo("previewPlacementDefault")).toEqual([]);
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
    expect((within(flash).getByRole("button", { name: /^Local/ }) as HTMLButtonElement).disabled).toBe(true);

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

describe("own choice", () => {
  it("is not shown while every entry follows its section", async () => {
    await renderPage();

    expect(screen.queryByText("Own choice")).toBeNull();
  });

  it("lists only the entries that left the default, with what each uses and the way to its page", async () => {
    listContainers.mockResolvedValue({
      ok: true,
      containers: [container("gitea"), container("jellyfin", placementView({ copiesFollow: false, skip: ["*"] }))],
    });
    await renderPage();

    const row = (await screen.findByRole("link", { name: "jellyfin" })).closest("li")!;
    expect(row.textContent).toContain("Backs up to Local · Default: Local and B2");
    expect(screen.getByRole("link", { name: "jellyfin" }).getAttribute("href")).toBe("/backups/container/jellyfin");
    expect(screen.queryByRole("link", { name: "gitea" })).toBeNull();
  });

  it("brings an entry back after saying where its backups go then and what is uploaded", async () => {
    listContainers.mockResolvedValue({
      ok: true,
      containers: [container("jellyfin", placementView({ copiesFollow: false, skip: ["*"] }))],
    });
    fake.reply("previewItemPlacement", { ok: true, added: [uploadEstimate()], dropped: [] });
    await renderPage();

    await click(await screen.findByRole("button", { name: "Follow the default" }));
    const dialog = await screen.findByRole("dialog");
    expect(dialog.textContent).toContain(
      "Should jellyfin follow the default? New backups then go to Local and B2. The earlier ones stay where they are."
    );
    expect(dialog.textContent).toContain("B2: about 40");
    expect(fake.callsTo("setItemPlacement")).toEqual([]);
    await click(within(dialog).getByRole("button", { name: "Follow the default" }));

    const follow = { home: { follow: true }, copies: { follow: true } };
    await waitFor(() => expect(fake.callsTo("setItemPlacement")).toEqual([[{ domain: "containers", key: "jellyfin" }, follow]]));
    expect(fake.callsTo("previewItemPlacement")).toEqual([[{ domain: "containers", key: "jellyfin" }, follow]]);
  });

  it("keeps the entry as it is when the question is answered with no", async () => {
    listContainers.mockResolvedValue({
      ok: true,
      containers: [container("jellyfin", placementView({ copiesFollow: false, skip: ["*"] }))],
    });
    await renderPage();

    await click(await screen.findByRole("button", { name: "Follow the default" }));
    await click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Cancel" }));

    expect(fake.callsTo("setItemPlacement")).toEqual([]);
  });

  it("offers no button where the first backup fixed the place and nothing else differs, and says why", async () => {
    const fixed = placementView({ repo: "repo-nas", repoKind: "local", repoLabel: "NAS Keller", locked: true, lockReason: "first-backup" });
    listContainers.mockResolvedValue({ ok: true, containers: [container("plex", fixed)] });
    await renderPage();

    const row = (await screen.findByRole("link", { name: "plex" })).closest("li")!;
    expect(within(row).queryByRole("button", { name: "Follow the default" })).toBeNull();
    expect(within(row).getByLabelText(/^Fixed since the first backup: /)).toBeTruthy();
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

  it("opens from the page's main action with the providers in their groups, a local folder first", async () => {
    const window = await open();

    const groups = within(window).getAllByRole("heading", { level: 3 }).map((h) => h.textContent ?? "");
    expect(groups.map((g) => g.replace(/(best|good) fit|works, but slower/, ""))).toEqual(["Local", "NAS or server", "S3 service", "Cloud storage"]);
    expect(tileNames(window)).toEqual(["Local folder", "Rest server", "Backblaze B2", "Google Drive"]);
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
