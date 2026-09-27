// @vitest-environment jsdom
// The anomalies card on the phone Home. Open critical and warning findings are
// what a phone is opened for, so while any are open the card leads the column;
// otherwise it waits below storage. Either way the hues run down the column in
// order, as static positions, and the card appears once.
//
// The desktop media query is held at "phone"; jsdom otherwise answers desktop
// and the phone surface would never mount.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { ToastProvider } from "../lib/toast";
import { AdvancedProvider } from "../lib/advanced";
import { AnomalyProvider } from "../lib/useAnomalies";
import { DESKTOP_QUERY } from "../lib/useMediaQuery";
import type { AnomalySummary } from "../lib/api";
import { Dashboard } from "./Dashboard";

const getAnomalySummary = vi.fn();

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    listRuns: () => Promise.resolve({ ok: true, runs: [] }),
    getStatus: () => Promise.resolve({ ok: true, domains: [] }),
    getScheduleNext: () => Promise.resolve([]),
    getStats: () => Promise.resolve({ ok: false }),
    listContainers: () => Promise.resolve({ ok: true, containers: [] }),
    listVMs: () => Promise.resolve({ ok: true, vms: [] }),
    getSettings: () => Promise.resolve({ ok: true, settings: {} as never }),
    getHistory: () => Promise.resolve({ ok: true, days: [] }),
    getSpike: () => Promise.resolve({ ok: false }),
    getAnomalySummary: () => getAnomalySummary(),
    getAnomalies: () => Promise.resolve({ ok: true, anomalies: [], nextCursor: "" }),
  };
});

let desktopMatches = false;

beforeEach(() => {
  window.matchMedia = ((query: string) => ({
    get matches() {
      return query === DESKTOP_QUERY ? desktopMatches : false;
    },
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
  vi.stubGlobal(
    "EventSource",
    class {
      onmessage = null;
      onerror = null;
      close() {}
    }
  );
  desktopMatches = false;
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

function summary(open: AnomalySummary["open"]): AnomalySummary {
  return {
    enabled: true,
    ready: true,
    generation: 1,
    open,
    recoveredCritical: 0,
    learningItems: 0,
    retentionHeld: 0,
    evalErrors: 0,
    notifyMuted: false,
    backfill: { slots: 0, done: 0, failed: 0, filled: 0, withoutSummary: 0 },
    unmeasuredVolumes: [],
  };
}

async function renderHome(open: AnomalySummary["open"]) {
  getAnomalySummary.mockResolvedValue({ ok: true, summary: summary(open) });
  render(
    <MemoryRouter>
      <I18nProvider>
        <ToastProvider>
          <AdvancedProvider>
            <AnomalyProvider>
              <Dashboard />
            </AnomalyProvider>
          </AdvancedProvider>
        </ToastProvider>
      </I18nProvider>
    </MemoryRouter>
  );
  await act(async () => {});
}

/** The phone column's cards in order, each with the hue position it wears. */
function column(): { title: string; hue: string }[] {
  const titles = [
    en["anomaly.title"],
    en["dashboard.summaryNextBackup"],
    en["dashboard.recentRuns"],
    en["dashboard.storageTitle"],
    en["activityLog.title"],
  ];
  return screen
    .getAllByRole("heading", { level: 2 })
    .map((h) => ({ h, title: titles.find((title) => h.textContent?.startsWith(title)) }))
    .filter((c): c is { h: HTMLElement; title: string } => c.title !== undefined)
    .map(({ h, title }) => ({
      title,
      hue: (h.closest(".glim-hue") as HTMLElement | null)?.style.getPropertyValue("--item-hue") ?? "",
    }));
}

describe("the phone Home's anomalies card", () => {
  it("leads the column while a critical or warning finding is open", async () => {
    await renderHome({ critical: 1, warning: 2, info: 0 });
    expect(column()).toEqual([
      { title: en["anomaly.title"], hue: "var(--rb-0)" },
      { title: en["dashboard.summaryNextBackup"], hue: "var(--rb-1)" },
      { title: en["dashboard.recentRuns"], hue: "var(--rb-2)" },
      { title: en["dashboard.storageTitle"], hue: "var(--rb-3)" },
      { title: en["activityLog.title"], hue: "var(--rb-4)" },
    ]);
  });

  it("sits below storage when only notes are open", async () => {
    await renderHome({ critical: 0, warning: 0, info: 3 });
    expect(column()).toEqual([
      { title: en["dashboard.summaryNextBackup"], hue: "var(--rb-0)" },
      { title: en["dashboard.recentRuns"], hue: "var(--rb-1)" },
      { title: en["dashboard.storageTitle"], hue: "var(--rb-2)" },
      { title: en["anomaly.title"], hue: "var(--rb-3)" },
      { title: en["activityLog.title"], hue: "var(--rb-4)" },
    ]);
  });
});
