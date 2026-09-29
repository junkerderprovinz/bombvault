// @vitest-environment jsdom
// FleetSettingsCard holds this instance's own name and the switch that lets
// a paired member open it in remote view. The name field debounces; the
// switch saves the moment it is flipped, like every other ToggleRow.
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";

import { FleetSettingsCard } from "./FleetSettingsCard";
import { countText, en } from "../../lib/i18n";
import type { Settings } from "../../lib/api";

const t = ((key: string, n?: number) =>
  countText((en as Record<string, string>)[key] ?? key, "en", n)) as unknown as Parameters<
  typeof FleetSettingsCard
>[0]["t"];

function settings(over: Partial<Settings> = {}): Settings {
  return {
    instanceName: "tower",
    remoteViewEnabled: true,
    ...over,
  } as Settings;
}

function renderCard(over: Partial<Settings> = {}) {
  const save = vi.fn().mockResolvedValue(true);
  const setSettings = vi.fn();
  render(
    <FleetSettingsCard t={t} settings={settings(over)} setSettings={setSettings} save={save} />
  );
  return { save, setSettings };
}

afterEach(cleanup);

describe("FleetSettingsCard", () => {
  it("shows the switch on, matching an instance that already allows remote view", () => {
    renderCard({ remoteViewEnabled: true });
    const toggle = screen.getByRole("switch", { name: en["settings.remoteView"] });
    expect(toggle.getAttribute("aria-checked")).toBe("true");
  });

  it("shows the switch off, matching an instance that has it disabled", () => {
    renderCard({ remoteViewEnabled: false });
    const toggle = screen.getByRole("switch", { name: en["settings.remoteView"] });
    expect(toggle.getAttribute("aria-checked")).toBe("false");
  });

  it("flipping the switch saves remoteViewEnabled at once, no debounce", () => {
    const { save, setSettings } = renderCard({ remoteViewEnabled: true });
    const toggle = screen.getByRole("switch", { name: en["settings.remoteView"] });
    act(() => fireEvent.click(toggle));
    expect(save).toHaveBeenCalledWith({ remoteViewEnabled: false }, expect.any(Function), expect.any(Function));
    expect(setSettings).toHaveBeenCalled();
  });

  it("explains the switch in an info bubble rather than leaving it unexplained", () => {
    renderCard();
    expect(screen.getByLabelText(en["settings.remoteViewHint"])).toBeTruthy();
  });
});
