// @vitest-environment jsdom
// The Anomalies card on the Integrity tab. Every control saves on its own, and
// the card names what detection could not see: an unread history, unchecked
// items, a muted channel and disks without a free-space figure.
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

import { AnomalyCard } from "./AnomalyCard";
import { countText, en } from "../../lib/i18n";
import type { AnomalySummary, Settings } from "../../lib/api";

const t = ((key: string, n?: number) =>
  countText((en as Record<string, string>)[key] ?? key, "en", n)) as unknown as Parameters<
  typeof AnomalyCard
>[0]["t"];

function summary(over: Partial<AnomalySummary> = {}): AnomalySummary {
  return {
    enabled: true,
    ready: true,
    generation: 1,
    open: { critical: 0, warning: 0, info: 0 },
    recoveredCritical: 0,
    learningItems: 0,
    retentionHeld: 0,
    evalErrors: 0,
    notifyMuted: false,
    backfill: { slots: 2, done: 2, failed: 0, filled: 40, withoutSummary: 0 },
    unmeasuredVolumes: [],
    ...over,
  };
}

function settings(over: Partial<Settings> = {}): Settings {
  return {
    anomalyEnabled: true,
    anomalySensitivity: "balanced",
    anomalyNotifyMin: "critical",
    anomalyRetentionHold: true,
    ...over,
  } as Settings;
}

function renderCard(over: { settings?: Settings; summary?: AnomalySummary | null } = {}) {
  const save = vi.fn();
  render(
    <MemoryRouter>
      <AnomalyCard
        t={t}
        settings={over.settings ?? settings()}
        summary={over.summary === undefined ? summary() : over.summary}
        save={save}
      />
    </MemoryRouter>
  );
  return { save };
}

function pick(field: string, option: string) {
  fireEvent.click(screen.getByRole("combobox", { name: field }));
  fireEvent.click(screen.getByRole("option", { name: option }));
}

function warnText(text: string): HTMLElement {
  return screen.getByText(text).closest("[class*='statusWarn']") as HTMLElement;
}

afterEach(cleanup);

describe("AnomalyCard", () => {
  it("saves the switch the moment it is flipped", () => {
    const { save } = renderCard();
    fireEvent.click(screen.getByRole("switch", { name: en["anomaly.settings.toggle"] }));
    expect(save).toHaveBeenCalledWith("anomalyEnabled", false);
  });

  it("saves the hold switch the moment it is flipped", () => {
    const { save } = renderCard();
    fireEvent.click(screen.getByRole("switch", { name: en["anomaly.settings.holdToggle"] }));
    expect(save).toHaveBeenCalledWith("anomalyRetentionHold", false);
  });

  it("saves a new sensitivity and a new notification minimum", () => {
    const { save } = renderCard();
    pick(en["anomaly.settings.sensitivity"], en["anomaly.sensitivity.strict"]);
    expect(save).toHaveBeenCalledWith("anomalySensitivity", "strict");
    pick(en["anomaly.settings.notifyMin"], en["anomaly.settings.notify.off"]);
    expect(save).toHaveBeenCalledWith("anomalyNotifyMin", "off");
  });

  it("hides the controls of a switched-off feature", () => {
    renderCard({ settings: settings({ anomalyEnabled: false }) });
    expect(screen.queryByRole("combobox", { name: en["anomaly.settings.sensitivity"] })).toBeNull();
    expect(screen.queryByRole("combobox", { name: en["anomaly.settings.notifyMin"] })).toBeNull();
    expect(screen.queryByRole("switch", { name: en["anomaly.settings.holdToggle"] })).toBeNull();
    // The way to what was found before it was switched off stays.
    expect(screen.getByRole("link", { name: en["anomaly.settings.openPage"] })).toBeTruthy();
  });

  it("warns about a history it could not read", () => {
    renderCard({
      summary: summary({ backfill: { slots: 3, done: 1, failed: 1, filled: 12, withoutSummary: 7 } }),
    });
    expect(screen.getByText("History read from repositories: 1 of 3")).toBeTruthy();
    expect(screen.getByText(en["anomaly.settings.backfillPending"])).toBeTruthy();
    expect(warnText("Repositories whose history could not be read: 1. Tried again once a day.")).toBeTruthy();
    expect(screen.getByText("Backups made before restic 0.17 carry no size history: 7")).toBeTruthy();
  });

  it("warns about items it could not check and about a muted channel", () => {
    renderCard({ summary: summary({ evalErrors: 4, notifyMuted: true }) });
    expect(
      warnText("Items that could not be checked in the last pass: 4. The log names them.")
    ).toBeTruthy();
    expect(warnText(en["anomaly.settings.notifyMuted"])).toBeTruthy();
    expect(screen.getByRole("link", { name: en["anomaly.settings.openNotifications"] })).toBeTruthy();
  });

  it("names the repositories with no free-space figure", () => {
    renderCard({ summary: summary({ unmeasuredVolumes: ["Backblaze", "Wasabi"] }) });
    expect(screen.getByText("Repositories without a free-space figure: Backblaze, Wasabi")).toBeTruthy();
  });
});
