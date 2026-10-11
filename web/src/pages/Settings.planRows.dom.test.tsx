// @vitest-environment jsdom
// The plans and exceptions outside the schedule page: the weekly digest and the
// automatic checks read as plan sentences whose Off is their switch, and a
// section gets a Healthchecks URL of its own as an exception.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { NotifyConfig, Settings } from "../lib/api";

// The checks card follows live progress, and jsdom has no EventSource.
class FakeEventSource {
  onmessage: ((ev: MessageEvent) => void) | null = null;
  close() {}
}

function baseSettings(over: Partial<Settings> = {}): Settings {
  return {
    containersEnabled: true,
    vmsEnabled: false,
    flashEnabled: false,
    filesEnabled: false,
    zfsEnabled: true,
    configEnabled: false,
    containersSchedule: "off",
    vmsSchedule: "off",
    flashSchedule: "off",
    filesSchedule: "off",
    zfsSchedule: "off",
    configSchedule: "off",
    containersOffsite: "",
    vmsOffsite: "",
    flashOffsite: "",
    configOffsite: "",
    filesOffsite: "",
    zfsOffsite: "",
    digestEnabled: true,
    digestSchedule: "weekly Mon 08:00",
    drillsEnabled: true,
    drillsSchedule: "weekly Sun 05:00",
    drillsSubsetPct: 5,
    tamperTestSchedule: "weekly Wed 04:30",
    anomalyEnabled: false,
    registryAuths: [],
    ...over,
  } as unknown as Settings;
}

const notify: NotifyConfig = {
  on: "failure",
  webhookEnabled: false,
  webhookUrl: "",
  webhookFormat: "generic",
  matrixEnabled: false,
  matrixHomeserver: "",
  matrixToken: "",
  matrixRoom: "",
  healthchecksUrl: "https://hc-ping.com/all",
  healthchecksByDomain: { zfs: "https://hc-ping.com/zfs" },
  unraid: false,
  smtpEnabled: false,
  smtpHost: "",
  smtpPort: 587,
  smtpUsername: "",
  smtpPassword: "",
  smtpFrom: "",
  smtpTo: "",
  smtpTls: "starttls",
  appriseEnabled: false,
  appriseUrl: "",
  appriseTags: "",
  scheduledSummary: false,
  notifyOnUpdate: false,
};

const putBodies: Settings[] = [];
const notifyBodies: NotifyConfig[] = [];
let settingsOnServer = baseSettings();

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () =>
      Promise.resolve({ ok: true, settings: settingsOnServer, hostMountRoot: "/host/user", platform: "unraid" }),
    putSettings: (s: Settings) => {
      putBodies.push(s);
      return Promise.resolve({ ok: true });
    },
    getNotify: () => Promise.resolve({ ok: true, notify }),
    setNotify: (cfg: NotifyConfig) => {
      notifyBodies.push(cfg);
      return Promise.resolve({ ok: true });
    },
    getAuth: () => Promise.resolve({ enabled: false, authed: false }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    getStatus: () => Promise.resolve({ ok: true }),
  };
});

const { SettingsPage } = await import("./Settings");

function stubBrowser() {
  window.matchMedia = ((query: string) => ({
    matches: true,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
  window.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}

async function renderPage(page: string) {
  await act(async () => {
    render(
      <MemoryRouter initialEntries={[`/settings/${page}`]}>
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

function card(title: string): HTMLElement {
  return screen.getByRole("heading", { name: new RegExp(`^${title}`) }).parentElement as HTMLElement;
}

async function pickMode(plan: RegExp, editorName: string, mode: string) {
  fireEvent.click(screen.getByRole("button", { name: plan }));
  const editor = await screen.findByRole("dialog", { name: editorName });
  fireEvent.click(within(editor).getByRole("tab", { name: mode }));
}

const SAVED = { timeout: 3000 };

beforeEach(() => {
  stubBrowser();
  vi.stubGlobal("EventSource", FakeEventSource);
  putBodies.length = 0;
  notifyBodies.length = 0;
  settingsOnServer = baseSettings();
});

afterEach(() => {
  cleanup();
});

describe("the weekly digest", () => {
  it("shows its plan in the notifications card", async () => {
    await renderPage("notifications");
    expect(within(card(en["notify.title"])).getByRole("button", { name: /^Weekly digest: ?weekly/ })).toBeTruthy();
  });

  it("turns off through the plan's Off", async () => {
    await renderPage("notifications");
    await pickMode(/^Weekly digest: ?weekly/, en["settings.digestToggle"], en["cadence.off"]);
    await waitFor(() => expect(putBodies.at(-1)?.digestEnabled).toBe(false), SAVED);
  });

  it("reads Off while it is switched off", async () => {
    settingsOnServer = baseSettings({ digestEnabled: false });
    await renderPage("notifications");
    expect(screen.getByRole("button", { name: /^Weekly digest: ?Off$/ })).toBeTruthy();
  });
});

describe("Healthchecks per section", () => {
  it("lists a section with its own URL and says that the others ping the shared one", async () => {
    await renderPage("notifications");
    const hc = card(en["notify.healthchecksTitle"]);
    await waitFor(() => expect(within(hc).getByDisplayValue("https://hc-ping.com/zfs")).toBeTruthy());
    expect(within(hc).getByText(en["notify.hcRuleOthers"])).toBeTruthy();
  });

  it("gives an added section its own URL", async () => {
    await renderPage("notifications");
    const hc = card(en["notify.healthchecksTitle"]);
    await waitFor(() => within(hc).getByDisplayValue("https://hc-ping.com/zfs"));
    fireEvent.click(within(hc).getByRole("button", { name: en["exceptions.add"] }));
    fireEvent.click(await screen.findByRole("option", { name: en["nav.containers"] }));
    const field = within(hc).getByRole("textbox", { name: `${en["notify.healthchecks"]}: ${en["nav.containers"]}` });
    fireEvent.change(field, { target: { value: "https://hc-ping.com/ct" } });
    await waitFor(
      () => expect(notifyBodies.at(-1)?.healthchecksByDomain).toEqual({ zfs: "https://hc-ping.com/zfs", container: "https://hc-ping.com/ct" }),
      SAVED
    );
  });

  it("hands a section back to the shared URL when its exception is removed", async () => {
    await renderPage("notifications");
    const hc = card(en["notify.healthchecksTitle"]);
    await waitFor(() => within(hc).getByDisplayValue("https://hc-ping.com/zfs"));
    fireEvent.click(within(hc).getByRole("button", { name: `${en["exceptions.remove"]}: ${en["nav.zfs"]}` }));
    await waitFor(() => expect(notifyBodies.at(-1)?.healthchecksByDomain).toEqual({ zfs: "" }));
    expect(within(hc).getByText(en["notify.hcRuleAll"])).toBeTruthy();
  });
});

describe("the automatic checks", () => {
  it("shows the trial restore with what it does while it runs on a plan", async () => {
    await renderPage("integrity");
    const checks = card(en["verify.autoChecks"]);
    expect(within(checks).getByRole("button", { name: /^Trial restore: ?weekly/ })).toBeTruthy();
    expect(within(checks).getByText(en["settings.startTest"])).toBeTruthy();
  });

  it("switches the trial restore off through the plan's Off", async () => {
    await renderPage("integrity");
    await pickMode(/^Trial restore: ?weekly/, en["verify.trialRestore"], en["cadence.off"]);
    await waitFor(() => expect(putBodies.at(-1)?.drillsEnabled).toBe(false));
    expect(screen.queryByText(en["settings.startTest"])).toBeNull();
  });

  it("says that the delete protection test never runs without a location that refuses deletes", async () => {
    await renderPage("integrity");
    expect(within(card(en["verify.autoChecks"])).getByText(en["settings.tamperScheduleInactive"])).toBeTruthy();
  });

  it("drops that warning once an off-site location refuses deletes", async () => {
    settingsOnServer = baseSettings({ zfsOffsite: "s3:bucket", zfsOffsiteImmutable: true } as Partial<Settings>);
    await renderPage("integrity");
    expect(screen.queryByText(en["settings.tamperScheduleInactive"])).toBeNull();
  });
});
