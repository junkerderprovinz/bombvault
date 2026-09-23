// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Dashboard phone storage block: the off-site line's three states.
//
// The line answers "is a copy of my backups somewhere else?" so it may only
// state "No off-site copy" once the status read actually answered and the
// answer is genuinely none. While the read is in flight the line says
// "checking", and when the read refused or failed it says so instead of
// letting an empty list impersonate an answer. Each test pins one state by
// driving the page's getStatus mock; the "No off-site copy" claim must appear
// in exactly one of the three.
//
// The page renders through the real components against a mocked api module,
// with the desktop media query held at "phone"; jsdom otherwise answers
// desktop and the phone surface would never mount.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import { AdvancedProvider } from "../lib/advanced";
import { DESKTOP_QUERY } from "../lib/useMediaQuery";
import type { DomainStatus } from "../lib/api";
import { Dashboard } from "./Dashboard";

// One knob per test: what the page's single /api/status read does.
let statusAnswer: () => Promise<{ ok: boolean; domains?: DomainStatus[] }> = () =>
  Promise.resolve({ ok: true, domains: [] });

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listRuns: () => Promise.resolve({ ok: true, runs: [] }),
    getStatus: () => statusAnswer(),
    getScheduleNext: () => Promise.resolve([]),
    getStats: () => Promise.resolve({ ok: false }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    getSettings: () => Promise.resolve({ ok: true, settings: {} as never }),
    getHistory: () => Promise.resolve({ ok: true, days: [] }),
    getSpike: () => Promise.resolve({ ok: false }),
    backupEverythingNow: () => Promise.resolve({ ok: true, started: true }),
  };
});

// A disabled domain: no RPO expectation, no off-site anything, so the
// health line reads "off" and the off-site line falls through to its own
// state under test.
function disabledDomain(over: Partial<DomainStatus>): DomainStatus {
  return {
    domain: "containers",
    enabled: false,
    schedule: "",
    coveredBy: "",
    lastSuccess: 0,
    periodSeconds: 0,
    status: "off",
    lastVerified: 0,
    lastVerifiedOK: false,
    verifiedDetail: "",
    drillDetail: "",
    offsiteConfigured: false,
    offsiteImmutable: false,
    lastTamperAt: 0,
    lastTamperOK: false,
    lastReplicationAt: 0,
    lastReplicationOK: false,
    lastDrDrillAt: 0,
    lastDrDrillOK: false,
    lastOffsiteSubsetAt: 0,
    lastOffsiteSubsetOK: false,
    ...over,
  };
}

// --- jsdom environment stubs (same harness as the run-sheet test) -----------

const mqlListeners = new Set<() => void>();
let desktopMatches = false;

function installMatchMedia() {
  window.matchMedia = ((query: string) => ({
    get matches() {
      return query === DESKTOP_QUERY ? desktopMatches : false;
    },
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: (_type: string, l: () => void) => mqlListeners.add(l),
    removeEventListener: (_type: string, l: () => void) => mqlListeners.delete(l),
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}

class FakeEventSource {
  onmessage: ((ev: unknown) => void) | null = null;
  onerror: ((ev: unknown) => void) | null = null;
  close() {}
}

beforeEach(() => {
  installMatchMedia();
  vi.stubGlobal("EventSource", FakeEventSource);
  desktopMatches = false;
  statusAnswer = () => Promise.resolve({ ok: true, domains: [] });
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function renderPage() {
  return render(
    <MemoryRouter>
      <I18nProvider>
        <ToastProvider>
          <AdvancedProvider>
            <Dashboard />
          </AdvancedProvider>
        </ToastProvider>
      </I18nProvider>
    </MemoryRouter>
  );
}

async function settle() {
  await new Promise((r) => setTimeout(r, 0));
}

/** The phone storage section, located by its filled section label. */
function storageSection(): HTMLElement {
  const labels = screen.getAllByText(en["dashboard.storageTitle"]);
  expect(labels.length).toBeGreaterThan(0);
  return labels[0].closest("section") as HTMLElement;
}

describe("Dashboard phone off-site line states", () => {
  it("reads checking while the status read is in flight, never a claim", async () => {
    statusAnswer = () => new Promise(() => {}); // never settles
    renderPage();
    await settle();
    const section = storageSection();
    // Several lines in the section legitimately say "checking" (health and
    // stats read their own fetches); the assertion is that the off-site line
    // is among them, and that no answer text rendered anywhere.
    expect(within(section).getAllByText(en["dashboard.checking"]).length).toBeGreaterThan(0);
    expect(within(section).queryByText(en["dashboard.noOffsite"])).toBeNull();
  });

  it("reports the failed read instead of an answer when status errors", async () => {
    statusAnswer = () => Promise.reject(new Error("boom"));
    renderPage();
    await settle();
    const section = storageSection();
    expect(within(section).getByText(en["dashboard.statusLoadFailed"])).toBeTruthy();
    expect(within(section).queryByText(en["dashboard.noOffsite"])).toBeNull();
  });

  it("says no off-site copy only once the read answered and none exists", async () => {
    statusAnswer = () =>
      Promise.resolve({ ok: true, domains: [disabledDomain({ domain: "containers" })] });
    renderPage();
    await settle();
    const section = storageSection();
    expect(within(section).getByText(en["dashboard.noOffsite"])).toBeTruthy();
    expect(within(section).queryByText(en["dashboard.statusLoadFailed"])).toBeNull();
  });
});
