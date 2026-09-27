// @vitest-environment jsdom
// The rail reads top down as what needs looking at, then what is backed up,
// then the rest: Dashboard and Anomalies first, every backup type after them,
// and Recovery and the other instances below the last one.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

import { Sidebar } from "./Sidebar";
import { AnomalyProvider } from "../lib/useAnomalies";
import { I18nProvider, en } from "../lib/i18n";
import { AdvancedProvider } from "../lib/advanced";
import type { Settings } from "../lib/api";

vi.mock("../lib/api", async () => {
  const actual = await vi.importActual<typeof import("../lib/api")>("../lib/api");
  return { ...actual, getAnomalySummary: vi.fn(), getAnomalies: vi.fn() };
});

const api = await import("../lib/api");

beforeEach(() => {
  localStorage.clear();
  vi.mocked(api.getAnomalySummary).mockResolvedValue({ ok: false, error: "offline" });
  vi.mocked(api.getAnomalies).mockResolvedValue({ ok: true, anomalies: [], nextCursor: "" });
});

afterEach(cleanup);

function railLabels(settings: Partial<Settings>): (string | null)[] {
  render(
    <MemoryRouter initialEntries={["/"]}>
      <I18nProvider>
        <AdvancedProvider>
          <AnomalyProvider>
            <Sidebar settings={settings as Settings} authEnabled={false} />
          </AnomalyProvider>
        </AdvancedProvider>
      </I18nProvider>
    </MemoryRouter>
  );
  return screen.getAllByRole("link").map((l) => l.textContent);
}

describe("the rail order", () => {
  it("puts Dashboard and Anomalies first, then every backup type, then the rest", () => {
    const labels = railLabels({
      anomalyEnabled: true,
      vmsEnabled: true,
      flashEnabled: true,
      filesEnabled: true,
      zfsEnabled: true,
      configEnabled: true,
      fleetEnabled: true,
    });
    expect(labels).toEqual([
      en["nav.dashboard"],
      en["nav.anomalies"],
      en["nav.containers"],
      en["nav.vms"],
      en["nav.flash"],
      en["nav.files"],
      en["nav.zfs"],
      en["nav.config"],
      en["nav.recovery"],
      en["instances.title"],
      en["nav.settings"],
    ]);
  });

  it("keeps Recovery below the backup types when most of them are off", () => {
    const labels = railLabels({ zfsEnabled: true });
    expect(labels).toEqual([
      en["nav.dashboard"],
      en["nav.containers"],
      en["nav.zfs"],
      en["nav.recovery"],
      en["nav.settings"],
    ]);
  });
});
