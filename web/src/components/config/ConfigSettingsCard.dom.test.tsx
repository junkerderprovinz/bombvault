// @vitest-environment jsdom
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { Settings } from "../../lib/api";
import { I18nProvider, en, useT } from "../../lib/i18n";
import { ToastProvider } from "../../lib/toast";

let answer: { ok: boolean; error?: string } = { ok: true };

vi.mock("../../lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../lib/api")>();
  return {
    ...actual,
    getSettings: () => Promise.resolve({ ok: true, settings: { configEnabled: true } }),
    putSettings: () => Promise.resolve(answer),
  };
});
vi.mock("../placement/PlacementFlow", () => ({ PlacementFlow: () => null }));

const { ConfigSettingsCard } = await import("./ConfigSettingsCard");

function Harness() {
  const { t } = useT();
  const [settings, setSettings] = useState({ configEnabled: true } as Settings);
  return <ConfigSettingsCard t={t} settings={settings} setSettings={setSettings} />;
}

function show() {
  render(
    <I18nProvider>
      <ToastProvider>
        <Harness />
      </ToastProvider>
    </I18nProvider>
  );
  return screen.getByRole("switch", { name: en["config.enabled"] });
}

afterEach(cleanup);

describe("the self-backup switch", () => {
  it("goes back after a refused save", async () => {
    answer = { ok: false, error: "refused" };
    const toggle = show();
    fireEvent.click(toggle);
    await screen.findByText("refused");
    await waitFor(() => expect(toggle.getAttribute("aria-checked")).toBe("true"));
  });

  it("tells the shell after a save", async () => {
    answer = { ok: true };
    const changed = vi.fn();
    window.addEventListener("bv:settings-changed", changed);
    const toggle = show();
    fireEvent.click(toggle);
    await waitFor(() => expect(changed).toHaveBeenCalled());
    window.removeEventListener("bv:settings-changed", changed);
    expect(toggle.getAttribute("aria-checked")).toBe("false");
  });
});
