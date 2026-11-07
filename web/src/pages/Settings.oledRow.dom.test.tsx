// @vitest-environment jsdom
// The OLED switch under the theme picker. It is this screen's own and never
// part of the look the server keeps, and it rests while the light theme shows.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";

const save = vi.fn();
vi.mock("../lib/displayPrefs", async (real) => ({ ...(await real<object>()), save: () => save() }));

import { OledRow } from "./settings/ThemeCard";
import { I18nProvider, useT } from "../lib/i18n";
import { applyStoredTheme, setTheme } from "../lib/theme";

function Harness() {
  const { t } = useT();
  return <OledRow t={t} />;
}

beforeEach(() => {
  localStorage.clear();
  save.mockClear();
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  })) as unknown as typeof window.matchMedia;
});

afterEach(() => {
  cleanup();
  document.documentElement.removeAttribute("data-theme");
  document.documentElement.removeAttribute("data-oled");
});

const oledSwitch = () => screen.getByRole("switch", { name: "OLED" });

describe("the OLED switch", () => {
  it("puts the page on true black and tells the server nothing", () => {
    localStorage.setItem("bv-theme", "dark");
    applyStoredTheme();
    render(
      <I18nProvider>
        <Harness />
      </I18nProvider>,
    );
    fireEvent.click(oledSwitch());
    expect(oledSwitch().getAttribute("aria-checked")).toBe("true");
    expect(document.documentElement.hasAttribute("data-oled")).toBe(true);
    expect(localStorage.getItem("bv-oled")).toBe("1");
    expect(save).not.toHaveBeenCalled();
  });

  it("rests while the light theme shows and wakes when dark comes back", async () => {
    localStorage.setItem("bv-theme", "light");
    applyStoredTheme();
    const { container } = render(
      <I18nProvider>
        <Harness />
      </I18nProvider>,
    );
    const label = () => container.querySelector("span.text-carbon-text")!;
    expect(label().className).toContain("opacity-50");
    expect(oledSwitch().hasAttribute("disabled")).toBe(false);

    await act(async () => setTheme("dark"));
    expect(label().className).not.toContain("opacity-50");
  });

  it("refuses while Light is chosen and says why", () => {
    localStorage.setItem("bv-theme", "light");
    applyStoredTheme();
    render(
      <I18nProvider>
        <Harness />
      </I18nProvider>,
    );

    fireEvent.click(oledSwitch());

    expect(oledSwitch().getAttribute("aria-disabled")).toBe("true");
    expect(oledSwitch().getAttribute("aria-checked")).toBe("false");
    expect(localStorage.getItem("bv-oled")).toBeNull();
    expect(screen.getByText("Only with Dark or System. Light is chosen above.")).toBeTruthy();
  });

  it("stays switchable under System on a light device, for the device's dark", () => {
    localStorage.setItem("bv-theme", "system");
    applyStoredTheme();
    render(
      <I18nProvider>
        <Harness />
      </I18nProvider>,
    );

    fireEvent.click(oledSwitch());

    expect(oledSwitch().hasAttribute("aria-disabled")).toBe(false);
    expect(localStorage.getItem("bv-oled")).toBe("1");
  });

  it("stops refusing as soon as Dark is chosen", async () => {
    localStorage.setItem("bv-theme", "light");
    applyStoredTheme();
    render(
      <I18nProvider>
        <Harness />
      </I18nProvider>,
    );

    await act(async () => setTheme("dark"));

    expect(oledSwitch().hasAttribute("aria-disabled")).toBe(false);
  });
});
