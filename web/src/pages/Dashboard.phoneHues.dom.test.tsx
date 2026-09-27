// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Every phone row participates in the colour engine.
//
// The rotation is the design language's rule for "a position belongs to one
// member of a set of equals": the phone runs block's failure counter row and
// recent-run rows, the run sheet's action rows, and the phone activity-log
// card each carry .glim-hue plus the inline --item-hue of their fixed
// position. A row missing the class is the regression this file exists to
// catch (that is exactly how the activity-log card and the run rows drifted
// out of the engine while the rest of the phone surface joined).
//
// The page renders through the real components against a mocked api module,
// with the desktop media query held at "phone"; jsdom otherwise answers
// desktop and the phone surface would never mount.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import { AdvancedProvider } from "../lib/advanced";
import { DESKTOP_QUERY } from "../lib/useMediaQuery";
import type { Run } from "../lib/api";
import { Dashboard } from "./Dashboard";

const listRunsMock = vi.fn(() => Promise.resolve({ ok: true, runs: makeRuns() }));

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
    domain: "container",
    ...over,
  };
}

function makeRuns(): Run[] {
  return [
    makeRun({ id: "run-fail-1", target: "jellyfin", status: "failed", error: "exit status 1: [path]" }),
    makeRun({ id: "run-ok-1", target: "plex" }),
    makeRun({ id: "run-ok-2", target: "sonarr" }),
  ];
}

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listRuns: () => listRunsMock(),
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

/** A class-carrying engine position: .glim-hue on the element and a resolved
 *  --item-hue inline (hueVars always sets it). */
function expectEnginePosition(el: HTMLElement, label: string) {
  expect(el.className, label).toContain("glim-hue");
  expect(el.style.getPropertyValue("--item-hue"), label).not.toBe("");
}

describe("phone rows carry the colour engine", () => {
  it("puts the failure counter row and the recent-run rows on the engine", async () => {
    renderPage();
    await settle();

    const counter = screen.getByRole("button", {
      name: `${en["dashboard.statErrors"]}: 1`,
    });
    expectEnginePosition(counter, "failure counter row");

    for (const target of ["jellyfin", "plex", "sonarr"]) {
      const row = screen
        .getAllByRole("button", { name: new RegExp(target, "i") })
        .filter((el) => (el.getAttribute("aria-label") ?? "").length > 0)[0];
      expect(row).toBeTruthy();
      expectEnginePosition(row, `recent-run row ${target}`);
    }
  });

  it("puts the run sheet's action rows on the engine", async () => {
    renderPage();
    await settle();

    const rows = screen
      .getAllByRole("button", { name: /plex/i })
      .filter((el) => (el.getAttribute("aria-label") ?? "").length > 0);
    fireEvent.click(rows[0]);

    const dialog = screen.getByRole("dialog");
    expectEnginePosition(within(dialog).getByRole("button", { name: en["recovery.foreignStepBrowse"] }), "browse row");
    expectEnginePosition(within(dialog).getByRole("button", { name: en["integrity.verify"] }), "verify row");
    expectEnginePosition(within(dialog).getByRole("button", { name: en["snapshots.restore"] }), "restore row");
  });

  it("puts the phone activity-log card on the engine", async () => {
    renderPage();
    await settle();

    const title = screen.getAllByText(en["activityLog.title"])[0];
    let el: HTMLElement | null = title;
    let carried: HTMLElement | null = null;
    while (el) {
      if (el.className.includes("glim-hue")) { carried = el; break; }
      el = el.parentElement;
    }
    expect(carried, "activity-log card root").toBeTruthy();
    expect(carried?.style.getPropertyValue("--item-hue")).not.toBe("");
  });
});
