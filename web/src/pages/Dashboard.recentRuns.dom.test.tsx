// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Dashboard phone recent-runs block: the four states of the runs read.
//
// The block answers "what has backed up lately?" so it may only state
// "No runs yet" once the runs read actually answered and the answer is
// genuinely none. While the read is in flight the block says "checking", and
// when the read refused or failed it says so instead of letting the empty
// list impersonate a backed-up-nothing history: the desktop face grew that
// failed arm in fix de5e36ae's discipline (the off-site line's twin, the
// model for this suite), and the phone glance kept reading a failed load as
// "No runs yet". Each test pins one state
// by driving the page's listRuns mock; the "No runs yet" claim must appear in
// exactly one of the four.
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
import type { Run } from "../lib/api";
import { Dashboard } from "./Dashboard";

// One knob per test: what the page's polled /api/runs read does.
let runsAnswer: () => Promise<{ ok: boolean; runs?: Run[] }> = () =>
  Promise.resolve({ ok: true, runs: [] });

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listRuns: () => runsAnswer(),
    getStatus: () => Promise.resolve({ ok: true, domains: [] }),
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

/** A finished run with every wire field the row and the page read. */
function makeRun(over: Partial<Run> & { id: string }): Run {
  const now = Math.floor(Date.now() / 1000);
  return {
    targetId: "containers:plex",
    kind: "backup",
    status: "success",
    startedAt: now - 60,
    finishedAt: now - 30,
    snapshotId: "abc12345",
    bytes: 1024,
    error: "",
    acknowledged: false,
    target: "plex",
    domain: "containers",
    ...over,
  };
}

// --- jsdom environment stubs (same harness as the off-site line test) -------

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
  runsAnswer = () => Promise.resolve({ ok: true, runs: [] });
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

/** The phone recent-runs section, located by its filled section label. */
function recentRunsSection(): HTMLElement {
  const labels = screen.getAllByText(en["dashboard.recentRuns"]);
  expect(labels.length).toBeGreaterThan(0);
  return labels[0].closest("section") as HTMLElement;
}

describe("Dashboard phone recent-runs states", () => {
  it("reads checking while the runs read is in flight, never a claim", async () => {
    runsAnswer = () => new Promise(() => {}); // never settles
    renderPage();
    await settle();
    const section = recentRunsSection();
    expect(within(section).getAllByText(en["dashboard.checking"]).length).toBeGreaterThan(0);
    expect(within(section).queryByText(en["dashboard.noRuns"])).toBeNull();
    expect(within(section).queryByText(en["dashboard.loadRunsFailed"])).toBeNull();
  });

  it("reports the failed read instead of an empty history when runs errors", async () => {
    // Bug B1 itself: the dense ladder only asked "any rows?", so a failed
    // read rendered "No runs yet", an answer the read never gave.
    runsAnswer = () => Promise.reject(new Error("boom"));
    renderPage();
    await settle();
    const section = recentRunsSection();
    expect(within(section).getByText(en["dashboard.loadRunsFailed"])).toBeTruthy();
    expect(within(section).queryByText(en["dashboard.noRuns"])).toBeNull();
  });

  it("says no runs yet only once the read answered and none exist", async () => {
    renderPage();
    await settle();
    const section = recentRunsSection();
    expect(within(section).getByText(en["dashboard.noRuns"])).toBeTruthy();
    expect(within(section).queryByText(en["dashboard.loadRunsFailed"])).toBeNull();
  });

  it("renders run rows once the read answered with runs", async () => {
    runsAnswer = () =>
      Promise.resolve({
        ok: true,
        runs: [
          makeRun({ id: "run-plex-1", target: "plex" }),
          makeRun({ id: "run-jelly-1", target: "jellyfin", targetId: "containers:jellyfin" }),
        ],
      });
    renderPage();
    await settle();
    const section = recentRunsSection();
    expect(within(section).getAllByText(/plex|jellyfin/i).length).toBeGreaterThan(0);
    expect(within(section).queryByText(en["dashboard.noRuns"])).toBeNull();
    expect(within(section).queryByText(en["dashboard.loadRunsFailed"])).toBeNull();
  });
});
