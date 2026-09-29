// @vitest-environment jsdom
// Anomalies is a read remote view shows, but decision 1 leaves acknowledging
// out on purpose, and an item's monitoring settings (sensitivity, notify
// minimum, forgetting an expectation) are a settings edit like any other, so
// all of it has to disappear rather than merely disable while a peer's
// instance is open. The findings themselves stay listed.
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../lib/i18n";
import { InstanceProvider } from "../lib/instanceScope";
import { ToastProvider } from "../lib/toast";
import { AnomalyProvider } from "../lib/useAnomalies";
import type { AnomalyFilter, AnomalyItem, AnomalySummary, AnomalyView, Settings } from "../lib/api";

vi.mock("../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../lib/api")>();
  return {
    ...actual,
    getAnomalies: vi.fn(),
    getAnomalySummary: vi.fn(),
    getAnomalyItems: vi.fn(),
    getSettings: vi.fn(),
    acknowledgeAnomalies: vi.fn(),
  };
});

const api = await import("../lib/api");
const getAnomalies = vi.mocked(api.getAnomalies);
const getAnomalySummary = vi.mocked(api.getAnomalySummary);
const getAnomalyItems = vi.mocked(api.getAnomalyItems);
const getSettings = vi.mocked(api.getSettings);

const { Anomalies } = await import("./Anomalies");

const GB = 1024 ** 3;

function summary(): AnomalySummary {
  return {
    enabled: true,
    ready: true,
    generation: 1,
    open: { critical: 1, warning: 0, info: 0 },
    recoveredCritical: 0,
    learningItems: 0,
    retentionHeld: 0,
    evalErrors: 0,
    notifyMuted: false,
    backfill: { slots: 0, done: 0, failed: 0, filled: 0, withoutSummary: 0 },
    unmeasuredVolumes: [],
  };
}

function finding(): AnomalyView {
  return {
    id: "an-1",
    detector: "source",
    metric: "source_bytes_shrink",
    severity: "critical",
    state: "open",
    scopeKind: "item",
    scopeId: "tg-plex",
    targetId: "tg-plex",
    domain: "container",
    part: "",
    targetName: "",
    name: "plex",
    runId: "run-1",
    lastRunId: "run-1",
    lastRunAt: 1700000000,
    observed: 2 * GB,
    expected: 40 * GB,
    threshold: 20 * GB,
    samples: 12,
    sensitivity: "balanced",
    details: {},
    occurrences: 1,
    firstSeenAt: 1700000000,
    lastSeenAt: 1700000000,
    recoveredAt: 0,
    resolvedAt: 0,
    ackedAt: 0,
    clearedAt: 0,
    ackNote: "",
    notifiedAt: 0,
    expectable: true,
    retentionHeld: false,
    stillPresent: false,
  };
}

function item(): AnomalyItem {
  return {
    targetId: "tg-plex",
    domain: "container",
    name: "plex",
    scheduled: true,
    sensitivity: "",
    effective: "balanced",
    notifyMin: "",
    effectiveNotifyMin: "warning",
    learning: { samples: 10, needed: 10, newData: 10, source: 10, duration: 10, noData: false },
    typical: { sourceBytes: GB, newDataBytes: 1024 ** 2, resticMs: 30000 },
    dump: null,
    datasets: [],
    open: { critical: 1, warning: 0, info: 0 },
    retentionHeld: false,
    selectionSince: 0,
    expectations: [],
  };
}

function settings(): Settings {
  return { anomalyEnabled: true, anomalySensitivity: "balanced", anomalyNotifyMin: "warning" } as Settings;
}

function page(anomalies: AnomalyView[], nextCursor = "") {
  return Promise.resolve({ ok: true as const, anomalies, nextCursor });
}

function card(name: string): HTMLElement {
  return screen.getByRole("region", { name });
}

beforeEach(() => {
  localStorage.clear();
  localStorage.setItem("bv-lang", "en");
  Element.prototype.scrollIntoView = vi.fn();
  getAnomalies.mockImplementation((f?: AnomalyFilter) => page(f?.state === "closed" ? [] : [finding()]));
  getAnomalySummary.mockResolvedValue({ ok: true, summary: summary() });
  getAnomalyItems.mockResolvedValue({ ok: true, items: [item()] });
  getSettings.mockResolvedValue({ ok: true, settings: settings() });
});

afterEach(() => {
  cleanup();
  localStorage.removeItem("bv-lang");
});

async function renderPage(initialEntries: string[]) {
  await act(async () => {
    render(
      <I18nProvider>
        <ToastProvider>
          <MemoryRouter initialEntries={initialEntries}>
            <InstanceProvider>
              <AnomalyProvider>
                <Anomalies />
              </AnomalyProvider>
            </InstanceProvider>
          </MemoryRouter>
        </ToastProvider>
      </I18nProvider>,
    );
  });
}

it("offers acknowledging and the item's monitoring settings locally", async () => {
  await renderPage(["/anomalies"]);
  const plex = card("plex");
  expect(within(plex).getByRole("button", { name: en["anomaly.action.acknowledge"] })).toBeTruthy();

  fireEvent.click(within(plex).getByRole("button", { name: en["anomaly.card.monitoring"] }));
  expect(await within(plex).findByText(en["anomaly.items.sensitivity"])).toBeTruthy();
});

it("hides acknowledging and the item's monitoring settings while a peer's instance is open, and keeps the finding listed", async () => {
  await renderPage(["/anomalies?instance=member-1&instanceName=attic"]);
  const plex = card("plex");
  expect(within(plex).getByText("plex")).toBeTruthy();
  expect(within(plex).queryByRole("button", { name: en["anomaly.action.acknowledge"] })).toBeNull();

  fireEvent.click(within(plex).getByRole("button", { name: en["anomaly.card.monitoring"] }));
  await act(async () => {});
  expect(within(plex).queryByText(en["anomaly.items.sensitivity"])).toBeNull();
});
