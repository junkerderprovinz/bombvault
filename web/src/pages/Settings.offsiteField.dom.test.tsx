// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";
import { targetPreview } from "../lib/placement.testsupport";

const fake = await vi.hoisted(async () => (await import("../lib/placement.testsupport")).createPlacementApi());
const stored = vi.hoisted(() => ({ containersOffsite: "", puts: [] as Settings[] }));

function baseSettings(): Settings {
  return {
    encryptionEnabled: true,
    containersEnabled: true,
    vmsEnabled: false,
    flashEnabled: false,
    filesEnabled: false,
    zfsEnabled: false,
    configEnabled: false,
    receiverEnabled: false,
    fleetEnabled: false,
    containersPath: "backups/containers",
    vmsPath: "backups/vms",
    flashPath: "backups/flash",
    filesPath: "backups/files",
    zfsPath: "backups/zfs",
    configPath: "backups/config",
    restoreFolder: "restore",
    containersSchedule: "daily 02:00",
    vmsSchedule: "off",
    flashSchedule: "off",
    filesSchedule: "off",
    zfsSchedule: "off",
    configSchedule: "off",
    containersOffsite: stored.containersOffsite,
    vmsOffsite: "",
    flashOffsite: "",
    configOffsite: "",
    filesOffsite: "",
    zfsOffsite: "",
    containersOffsiteSchedule: "",
    vmsOffsiteSchedule: "",
    flashOffsiteSchedule: "",
    configOffsiteSchedule: "",
    filesOffsiteSchedule: "",
    zfsOffsiteSchedule: "",
    everythingSchedule: "",
    everythingPreHook: "",
    everythingPostHook: "",
    retentionKeepLast: 5,
    retentionKeepDaily: 7,
    retentionKeepWeekly: 4,
    retentionKeepMonthly: 6,
    retentionKeepYearly: 0,
    offsiteRetentionKeepLast: 7,
    offsiteRetentionKeepDaily: 0,
    offsiteRetentionKeepWeekly: 0,
    offsiteRetentionKeepMonthly: 0,
    offsiteRetentionKeepYearly: 0,
    compression: {},
    defaultLanguage: "en",
    registryAuths: [],
    ownRetention: {},
    ownOffsiteRetention: {},
  } as unknown as Settings;
}

vi.mock("../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../lib/api")>()),
  ...fake.api,
  getSettings: () =>
    Promise.resolve({ ok: true, settings: baseSettings(), hostMountRoot: "/host/user", platform: "unraid" }),
  putSettings: (s: Settings) => {
    stored.puts.push(s);
    return Promise.resolve({ ok: true, warnings: [], notes: [] });
  },
  getAuth: () => Promise.resolve({ enabled: false, authed: false }),
  listContainers: () => Promise.resolve({ ok: true, containers: [] }),
  listVMs: () => Promise.resolve({ ok: true, vms: [] }),
  listFileSets: () => Promise.resolve({ ok: true, fileSets: [] }),
  getStatus: () => Promise.resolve({ ok: true }),
  listOffsiteTargets: () => Promise.resolve({ ok: true, targets: [] }),
  getCloud: () =>
    Promise.resolve({
      ok: true, s3KeyId: "", s3Region: "", restUser: "", s3StorageClass: "", s3SecretSet: false, restPasswordSet: false,
    }),
  getCloudCredSets: () => Promise.resolve({ ok: true, sets: [] }),
  getRclone: () => Promise.resolve({ ok: true, remotes: [] }),
}));

const { SettingsPage } = await import("./Settings");

const PLACEHOLDER = "rest:http://host:8000/repo";

async function renderOffsitePage() {
  await act(async () => {
    render(
      <MemoryRouter initialEntries={["/settings/storage"]}>
        <Routes>
          <Route
            path="/settings/:page"
            element={
              <I18nProvider>
                <ToastProvider>
                  <SettingsPage />
                </ToastProvider>
              </I18nProvider>
            }
          />
        </Routes>
      </MemoryRouter>
    );
  });
}

/** The card that lists every section's off-site copy. */
async function copiesCard(): Promise<HTMLElement> {
  const heading = await screen.findByRole("heading", { name: new RegExp(`^${en["offsite.copies.title"]}`) });
  return heading.closest("[data-search-card]") as HTMLElement;
}

/** Opens the window of one section's off-site copy from its row. */
async function openCopy(section: string) {
  const card = await copiesCard();
  await act(async () => {
    fireEvent.click(within(card).getByRole("button", { name: section }));
  });
  await screen.findByPlaceholderText(PLACEHOLDER);
}

function field(): HTMLInputElement {
  return screen.getByPlaceholderText(PLACEHOLDER) as HTMLInputElement;
}

/** The question a new location asks, above the section's window. */
async function question(): Promise<HTMLElement> {
  await waitFor(() => expect(screen.getAllByRole("dialog")).toHaveLength(2));
  return screen.getAllByRole("dialog")[1];
}

async function type(value: string) {
  await act(async () => {
    fireEvent.change(field(), { target: { value } });
  });
  await act(async () => {
    vi.advanceTimersByTime(900);
  });
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
  (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
  fake.reset();
  stored.containersOffsite = "";
  stored.puts.length = 0;
});

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("the off-site field", () => {
  it("saves a new location only after Enter and the question, then leaves the ticked items out", async () => {
    fake.reply("getNewTargetPreview", {
      ok: true,
      preview: targetPreview({ formerlyExcluded: [{ identity: "container:plex", skip: ["t-x"] }] }),
    });
    await renderOffsitePage();
    await openCopy(en["nav.containers"]);
    await type("b2:bucket:containers");
    expect(stored.puts).toHaveLength(0);
    await act(async () => {
      fireEvent.keyDown(field(), { key: "Enter" });
    });
    const dialog = await question();
    expect(dialog.textContent).toContain("At its first run b2:bucket:containers receives every item not set to Local.");
    fireEvent.click(within(dialog).getByRole("switch", { name: "Leave these out here too" }));
    await act(async () => {
      fireEvent.click(within(dialog).getByRole("button", { name: en["common.confirm"] }));
    });
    await waitFor(() => expect(stored.puts).toHaveLength(1));
    expect(stored.puts[0].containersOffsite).toBe("b2:bucket:containers");
    await waitFor(() =>
      expect(fake.callsTo("excludeFromTarget")).toEqual([
        [{ domain: "containers", field: true, identities: ["container:plex"], default: false }],
      ])
    );
  });

  it("keeps the typed location when the question is cancelled", async () => {
    await renderOffsitePage();
    await openCopy(en["nav.containers"]);
    await type("b2:bucket:containers");
    await act(async () => {
      fireEvent.blur(field());
    });
    await act(async () => {
      fireEvent.click(within(await question()).getByRole("button", { name: en["common.cancel"] }));
    });
    expect(field().value).toBe("b2:bucket:containers");
    expect(stored.puts).toHaveLength(0);
  });

  it("clears a location without asking", async () => {
    stored.containersOffsite = "b2:bucket:containers";
    await renderOffsitePage();
    await openCopy(en["nav.containers"]);
    await type("");
    await act(async () => {
      fireEvent.click(within(field().parentElement as HTMLElement).getByRole("button", { name: en["settings.save"] }));
    });
    await waitFor(() => expect(stored.puts).toHaveLength(1));
    expect(stored.puts[0].containersOffsite).toBe("");
    expect(screen.getAllByRole("dialog")).toHaveLength(1);
    expect(fake.callsTo("getNewTargetPreview")).toEqual([]);
  });

  it("saves the flash field on Enter without asking", async () => {
    await renderOffsitePage();
    await openCopy(en["nav.flash"]);
    await type("b2:bucket:flash");
    await act(async () => {
      fireEvent.keyDown(field(), { key: "Enter" });
    });
    await waitFor(() => expect(stored.puts).toHaveLength(1));
    expect(stored.puts[0].flashOffsite).toBe("b2:bucket:flash");
    expect(fake.callsTo("getNewTargetPreview")).toEqual([]);
  });
});

/** Matches a label whatever direction marks it carries around paths. */
const bare = (text: string) => (label: string) => label.replace(/[\u2066-\u2069]/g, "") === text;

describe("setting up an off-site copy", () => {
  it("opens the setup in a window from the section's row", async () => {
    await renderOffsitePage();
    const card = await copiesCard();
    await act(async () => {
      fireEvent.click(within(card).getAllByRole("button", { name: en["offsite.wizard.setup"] })[0]);
    });
    const title = en["offsite.copyDomainTitle"].replace("{domain}", en["nav.containers"]);
    const window = screen.getByRole("dialog", { name: title });
    expect(within(window).getByText(en["offsite.wizard.step1"])).toBeTruthy();
  });

  it("lists every section on one card and keeps the location in its window", async () => {
    await renderOffsitePage();
    const card = await copiesCard();
    for (const section of ["nav.containers", "nav.vms", "nav.flash", "nav.files", "nav.zfs", "nav.config"] as const) {
      expect(within(card).getByRole("button", { name: en[section] })).toBeTruthy();
    }
    expect(screen.queryByPlaceholderText(PLACEHOLDER)).toBeNull();
    await openCopy(en["nav.vms"]);
    expect(screen.getByRole("dialog", { name: en["offsite.copyDomainTitle"].replace("{domain}", en["nav.vms"]) })).toBeTruthy();
  });

  it("explains the location's syntax in bubbles instead of lines of text", async () => {
    await renderOffsitePage();
    await copiesCard();
    expect(screen.queryByText(en["settings.offsiteHint"])).toBeNull();
    expect(screen.getAllByLabelText(en["settings.offsiteHint"]).length).toBeGreaterThan(0);
    await openCopy(en["nav.containers"]);
    expect(screen.getAllByLabelText(bare(en["offsite.repoLocalHint"])).length).toBeGreaterThan(0);
  });
});
