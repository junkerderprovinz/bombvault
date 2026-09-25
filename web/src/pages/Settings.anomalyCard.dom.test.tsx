// @vitest-environment jsdom
// The Anomalies card is reached from the dashboard and from the Notifications
// tab through /settings#anomalies, and it is the last card on the Integrity
// tab, so the link has to bring it into view. Each control saves on its own,
// and a refused save must leave the control showing what the server still
// holds.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import type { Settings } from "../lib/api";

function baseSettings(over: Partial<Settings> = {}): Settings {
  return {
    encryptionEnabled: true,
    containersEnabled: true,
    vmsEnabled: false,
    flashEnabled: false,
    filesEnabled: false,
    configEnabled: false,
    receiverEnabled: false,
    fleetEnabled: false,
    pullEnabled: false,
    dbDumpsEnabled: true,
    containersSchedule: "off",
    vmsSchedule: "off",
    flashSchedule: "off",
    filesSchedule: "off",
    configSchedule: "off",
    retentionKeepLast: 5,
    retentionKeepDaily: 7,
    retentionKeepWeekly: 4,
    retentionKeepMonthly: 6,
    defaultLanguage: "en",
    registryAuths: [],
    drDrillTarget: "",
    drDrillTargetVm: "",
    anomalyEnabled: true,
    anomalySensitivity: "balanced",
    anomalyNotifyMin: "critical",
    anomalyRetentionHold: true,
    ...over,
  } as unknown as Settings;
}

let putAnswer: { ok: boolean; error?: string } = { ok: true };
const putBodies: Settings[] = [];

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getSettings: () =>
      Promise.resolve({ ok: true, settings: baseSettings(), hostMountRoot: "/host/user", platform: "unraid" }),
    putSettings: (s: Settings) => {
      putBodies.push(s);
      return Promise.resolve(putAnswer);
    },
    getAuth: () => Promise.resolve({ enabled: false, authed: false }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    listFileSets: () => Promise.resolve({ ok: true, fileSets: [] }),
    getStatus: () => Promise.resolve({ ok: true }),
    getDrills: () => Promise.resolve({ ok: true, drills: [], latest: null }),
  };
});

const { SettingsPage } = await import("./Settings");
const { AnomalyProvider } = await import("../lib/useAnomalies");

function stubBrowser() {
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
  window.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}

async function renderAt(hash: string) {
  await act(async () => {
    window.location.hash = hash;
    render(
      <MemoryRouter>
        <I18nProvider>
          <ToastProvider>
            <SettingsPage />
          </ToastProvider>
        </I18nProvider>
      </MemoryRouter>
    );
  });
  await screen.findByRole("switch", { name: en["anomaly.settings.toggle"] });
}

function stubSummary(over: Record<string, unknown>) {
  const summary = {
    enabled: true,
    ready: true,
    generation: 1,
    open: { critical: 0, warning: 0, info: 0 },
    recoveredCritical: 0,
    learningItems: 0,
    retentionHeld: 0,
    evalErrors: 0,
    notifyMuted: false,
    backfill: { slots: 0, done: 0, failed: 0, filled: 0, withoutSummary: 0 },
    ...over,
  };
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
      if (path !== "/api/anomalies/summary") throw new Error(`unexpected request ${path}`);
      return new Response(JSON.stringify({ ok: true, summary }), { status: 200 });
    })
  );
}

async function renderWithSummary() {
  await act(async () => {
    window.location.hash = "#integrity";
    render(
      <MemoryRouter>
        <I18nProvider>
          <ToastProvider>
            <AnomalyProvider>
              <SettingsPage />
            </AnomalyProvider>
          </ToastProvider>
        </I18nProvider>
      </MemoryRouter>
    );
  });
  await waitFor(() => expect(fetch).toHaveBeenCalledWith("/api/anomalies/summary", expect.anything()));
  await act(async () => {});
}

let scrolled: Element[] = [];

beforeEach(() => {
  stubBrowser();
  putAnswer = { ok: true };
  putBodies.length = 0;
  scrolled = [];
  Element.prototype.scrollIntoView = function (this: Element) {
    scrolled.push(this);
  };
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  window.location.hash = "";
});

describe("the Anomalies card on the Integrity tab", () => {
  it("is where #anomalies lands, scrolled into view", async () => {
    await renderAt("#anomalies");
    await waitFor(() => expect(scrolled.map((el) => el.id)).toContain("anomalies"));
  });

  it("comes after the restore-check schedule, so the checks and their schedule stay together", async () => {
    await renderAt("#integrity");
    const schedule = screen.getByRole("heading", { name: en["settings.schedulesChecks"] });
    const anomalies = document.getElementById("anomalies");
    expect(anomalies).not.toBeNull();
    expect(schedule.compareDocumentPosition(anomalies!) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("saves the switch on its own", async () => {
    await renderAt("#integrity");
    await act(async () => {
      fireEvent.click(screen.getByRole("switch", { name: en["anomaly.settings.toggle"] }));
    });
    await waitFor(() => expect(putBodies).toHaveLength(1));
    expect(putBodies[0].anomalyEnabled).toBe(false);
  });

  it("puts a refused switch back and says why", async () => {
    putAnswer = { ok: false, error: "the database is read-only" };
    await renderAt("#integrity");
    const toggle = screen.getByRole("switch", { name: en["anomaly.settings.toggle"] });
    await act(async () => {
      fireEvent.click(toggle);
    });
    expect(await screen.findByText("the database is read-only")).toBeTruthy();
    await waitFor(() =>
      expect(
        screen.getByRole("switch", { name: en["anomaly.settings.toggle"] }).getAttribute("aria-checked")
      ).toBe("true")
    );
  });

  it("opens with a summary that sends null for its list of unmeasured repositories", async () => {
    stubSummary({ unmeasuredVolumes: null });
    await renderWithSummary();
    expect(screen.getByRole("switch", { name: en["anomaly.settings.toggle"] })).toBeTruthy();
    expect(screen.queryByText(/Repositories without a free-space figure/)).toBeNull();
  });

  it("opens with a summary from a server that leaves the list out", async () => {
    stubSummary({ unmeasuredVolumes: undefined });
    await renderWithSummary();
    expect(screen.getByRole("switch", { name: en["anomaly.settings.toggle"] })).toBeTruthy();
  });

  it("still shows the unmeasured repositories and failed checks a summary reports", async () => {
    stubSummary({ unmeasuredVolumes: ["rest remote"], evalErrors: 2 });
    await renderWithSummary();
    expect(
      await screen.findByText(en["anomaly.settings.unmeasured"].replace("{names}", "rest remote"))
    ).toBeTruthy();
    expect(screen.getByText(en["anomaly.settings.evalErrors"].replace("{n}", "2"))).toBeTruthy();
  });

  it("puts a refused sensitivity back and says why", async () => {
    putAnswer = { ok: false, error: "the database is read-only" };
    await renderAt("#integrity");
    await act(async () => {
      fireEvent.click(screen.getByRole("combobox", { name: en["anomaly.settings.sensitivity"] }));
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("option", { name: en["anomaly.sensitivity.strict"] }));
    });
    expect(await screen.findByText("the database is read-only")).toBeTruthy();
    expect(putBodies[0].anomalySensitivity).toBe("strict");
    await waitFor(() =>
      expect(
        screen.getByRole("combobox", { name: en["anomaly.settings.sensitivity"] }).textContent
      ).toContain(en["anomaly.sensitivity.balanced"])
    );
  });
});
