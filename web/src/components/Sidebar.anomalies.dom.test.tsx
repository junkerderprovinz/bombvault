// @vitest-environment jsdom
// The rail's anomalies entry. It sits right under Dashboard so an open
// critical is visible above the fold whatever order the dashboard cards are
// in, and it follows the detection switch, because a rail entry for a feature
// nobody switched on is a dead end.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

import { Sidebar } from "./Sidebar";
import { AnomalyProvider } from "../lib/useAnomalies";
import { I18nProvider, en } from "../lib/i18n";
import { AdvancedProvider } from "../lib/advanced";
import type { AnomalySummary, Settings } from "../lib/api";

vi.mock("../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../lib/api")>("../lib/api");
  return { ...actual, getAnomalySummary: vi.fn(), getAnomalies: vi.fn() };
});

const api = await import("../lib/api");
const getAnomalySummary = vi.mocked(api.getAnomalySummary);
const getAnomalies = vi.mocked(api.getAnomalies);

function settings(anomalyEnabled: boolean): Settings {
  return { anomalyEnabled } as unknown as Settings;
}

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

function renderRail(s: Settings | null) {
  return render(
    <MemoryRouter initialEntries={["/"]}>
      <I18nProvider>
        <AdvancedProvider>
          <AnomalyProvider>
            <Sidebar settings={s} authEnabled={false} />
          </AnomalyProvider>
        </AdvancedProvider>
      </I18nProvider>
    </MemoryRouter>
  );
}

beforeEach(() => {
  localStorage.clear();
  getAnomalySummary.mockReset();
  getAnomalies.mockReset();
  getAnomalySummary.mockResolvedValue({ ok: true, summary: summary({ critical: 0, warning: 0, info: 0 }) });
  getAnomalies.mockResolvedValue({ ok: true, anomalies: [], nextCursor: "" });
});

afterEach(cleanup);

describe("the anomalies entry", () => {
  it("follows Dashboard while detection is on", () => {
    renderRail(settings(true));
    const labels = screen.getAllByRole("link").map((l) => l.textContent);
    expect(labels.slice(0, 2)).toEqual([en["nav.dashboard"], en["nav.anomalies"]]);
  });

  it("is gone while detection is off", () => {
    renderRail(settings(false));
    expect(screen.queryByRole("link", { name: en["nav.anomalies"] })).toBeNull();
  });

  it("counts the open critical and warning findings", async () => {
    getAnomalySummary.mockResolvedValue({
      ok: true,
      summary: summary({ critical: 2, warning: 3, info: 9 }),
    });
    renderRail(settings(true));

    const entry = await screen.findByRole("link", { name: new RegExp(en["nav.anomalies"]) });
    const count = await within(entry).findByLabelText(
      en["anomaly.navCountAria"].replace("{n}", "5")
    );
    expect(count.textContent).toBe("5");
  });

  it("groups a four-digit count in the badge's label", async () => {
    getAnomalySummary.mockResolvedValue({
      ok: true,
      summary: summary({ critical: 200, warning: 1000, info: 0 }),
    });
    renderRail(settings(true));

    const entry = await screen.findByRole("link", { name: new RegExp(en["nav.anomalies"]) });
    // The default normalizer turns the group separator into a plain space,
    // which is the difference this asserts.
    const count = await within(entry).findByLabelText(
      en["anomaly.navCountAria"].replace("{n}", (1200).toLocaleString()),
      { normalizer: (text) => text }
    );
    expect(count.textContent).toBe("1200");
  });

  it("shows no count while nothing is open", async () => {
    renderRail(settings(true));
    const entry = screen.getByRole("link", { name: new RegExp(en["nav.anomalies"]) });
    await waitFor(() => expect(getAnomalySummary).toHaveBeenCalled());
    expect(entry.textContent).toBe(en["nav.anomalies"]);
  });
});
