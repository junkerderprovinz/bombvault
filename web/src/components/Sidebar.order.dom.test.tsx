// @vitest-environment jsdom
// The rail reads top down as what needs looking at, then the backups and the
// way back from them, then the pinned Settings pages, with Settings itself at
// the foot.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

import { Sidebar } from "./Sidebar";
import { AnomalyProvider } from "../lib/useAnomalies";
import { I18nProvider, en } from "../lib/i18n";
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
        <AnomalyProvider>
          <Sidebar settings={settings as Settings} authEnabled={false} />
        </AnomalyProvider>
      </I18nProvider>
    </MemoryRouter>
  );
  return screen.getAllByRole("link").map((l) => l.textContent);
}

describe("the rail order", () => {
  it("puts Overview and Anomalies first, then Backups and Restore, then the pinned pages, then Settings", () => {
    const labels = railLabels({ anomalyEnabled: true, fleetEnabled: true });
    expect(labels).toEqual([
      en["nav.overview"],
      en["nav.anomalies"],
      en["backups.title"],
      en["nav.restore"],
      en["storage.title"],
      en["instances.title"],
      en["nav.settings"],
    ]);
  });

  it("keeps Restore right below Backups when nothing else is shown", () => {
    localStorage.setItem("bombvault.navPins", JSON.stringify({ storage: false }));
    const labels = railLabels({});
    expect(labels).toEqual([en["nav.overview"], en["backups.title"], en["nav.restore"], en["nav.settings"]]);
  });

  it("shows no entry per backup type, whatever types are on", () => {
    const labels = railLabels({
      vmsEnabled: true,
      flashEnabled: true,
      filesEnabled: true,
      zfsEnabled: true,
      configEnabled: true,
    });
    expect(labels).toEqual([
      en["nav.overview"],
      en["backups.title"],
      en["nav.restore"],
      en["storage.title"],
      en["nav.settings"],
    ]);
  });

  it("puts the rows above sign out in the list and Settings below it", () => {
    render(
      <MemoryRouter initialEntries={["/"]}>
        <I18nProvider>
          <Sidebar settings={null} authEnabled />
        </I18nProvider>
      </MemoryRouter>
    );
    const signOut = screen.getByRole("button", { name: en["auth.logout"] });
    const before = (name: string) =>
      Boolean(screen.getByRole("link", { name }).compareDocumentPosition(signOut) & Node.DOCUMENT_POSITION_FOLLOWING);
    for (const key of ["nav.overview", "backups.title", "nav.restore", "storage.title"] as const) {
      expect(before(en[key]), `${en[key]} stands above sign out`).toBe(true);
    }
    expect(before(en["nav.settings"]), "Settings stands below sign out").toBe(false);
  });
});
