// @vitest-environment jsdom
// ThemePicker, the Light, Dark and System segments on the Look page. A click
// goes through lib/theme.ts, which stores the choice and paints the page.
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { ThemePicker } from "./settings/ThemeCard";
import { I18nProvider, useT } from "../lib/i18n";

const STORAGE_KEY = "bv-theme";

function Harness() {
  const { t } = useT();
  return <ThemePicker t={t} />;
}

function renderCard() {
  return render(
    <I18nProvider>
      <Harness />
    </I18nProvider>
  );
}

let changeListeners: Array<() => void> = [];

/** jsdom has no matchMedia. `matches` is the OS preference the test wants, and
 * the captured change callbacks let a test simulate an OS-level flip. */
function stubMatchMedia(prefersDark: boolean) {
  window.matchMedia = ((query: string) => ({
    matches: prefersDark,
    media: query,
    onchange: null,
    addEventListener: (_event: string, cb: () => void) => {
      changeListeners.push(cb);
    },
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}

beforeEach(() => {
  localStorage.removeItem(STORAGE_KEY);
  changeListeners = [];
  stubMatchMedia(false);
});

afterEach(() => {
  cleanup();
  localStorage.removeItem(STORAGE_KEY);
  document.documentElement.removeAttribute("data-theme");
});

describe("ThemePicker", () => {
  it("offers Light, Dark and System under the name Theme", () => {
    localStorage.setItem(STORAGE_KEY, "light");
    renderCard();
    const picker = screen.getByRole("tablist", { name: "Theme" });
    expect(within(picker).getAllByRole("tab").map((tab) => tab.textContent)).toEqual(["Light", "Dark", "System"]);
  });

  it("the segment matching the current mode is the one marked selected", () => {
    localStorage.setItem(STORAGE_KEY, "light");
    renderCard();
    expect(screen.getByRole("tab", { name: "Light" }).getAttribute("aria-selected")).toBe("true");
    expect(screen.getByRole("tab", { name: "Dark" }).getAttribute("aria-selected")).toBe("false");
  });

  it("clicking the Dark segment sets dark and persists via lib/theme.ts", () => {
    localStorage.setItem(STORAGE_KEY, "light");
    renderCard();
    fireEvent.click(screen.getByRole("tab", { name: "Dark" }));
    expect(screen.getByRole("tab", { name: "Dark" }).getAttribute("aria-selected")).toBe("true");
    expect(localStorage.getItem(STORAGE_KEY)).toBe("dark");
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
  });

  it("clicking the Light segment sets light and persists via lib/theme.ts", () => {
    localStorage.setItem(STORAGE_KEY, "dark");
    renderCard();
    fireEvent.click(screen.getByRole("tab", { name: "Light" }));
    expect(screen.getByRole("tab", { name: "Light" }).getAttribute("aria-selected")).toBe("true");
    expect(localStorage.getItem(STORAGE_KEY)).toBe("light");
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
  });

  it("clicking the already-active segment is a harmless no-op", () => {
    localStorage.setItem(STORAGE_KEY, "light");
    renderCard();
    fireEvent.click(screen.getByRole("tab", { name: "Light" }));
    expect(localStorage.getItem(STORAGE_KEY)).toBe("light");
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
  });

  it("shows System as chosen while nothing is stored, whatever the device resolves it to", () => {
    stubMatchMedia(true);
    renderCard();
    expect(screen.getByRole("tab", { name: "System" }).getAttribute("aria-selected")).toBe("true");
    expect(screen.getByRole("tab", { name: "Dark" }).getAttribute("aria-selected")).toBe("false");
  });

  it("leads back to the device's theme through the System segment", () => {
    localStorage.setItem(STORAGE_KEY, "light");
    stubMatchMedia(true);
    renderCard();
    fireEvent.click(screen.getByRole("tab", { name: "System" }));
    expect(screen.getByRole("tab", { name: "System" }).getAttribute("aria-selected")).toBe("true");
    expect(localStorage.getItem(STORAGE_KEY)).toBe("system");
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
  });

  it("once explicitly set to \"light\", a later OS flip does not change the active segment", () => {
    localStorage.setItem(STORAGE_KEY, "light");
    renderCard();
    stubMatchMedia(true);
    act(() => {
      changeListeners.forEach((cb) => cb());
    });
    expect(screen.getByRole("tab", { name: "Light" }).getAttribute("aria-selected")).toBe("true");
    expect(localStorage.getItem(STORAGE_KEY)).toBe("light");
  });
});
