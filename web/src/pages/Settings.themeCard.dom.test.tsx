// @vitest-environment jsdom
// ThemeCard, the light/dark Selector on Settings' General tab. Clicking a
// segment sets that theme directly and persists it through setTheme(), the
// active segment carries aria-selected, and while "system" is stored an OS
// prefers-color-scheme change repaints it live.
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { ThemeCard } from "./settings/ThemeCard";
import { I18nProvider, useT } from "../lib/i18n";

const STORAGE_KEY = "bv-theme";

function Harness() {
  const { t } = useT();
  return <ThemeCard t={t} />;
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

describe("ThemeCard", () => {
  it("renders as a Card with the theme heading and both segments always present", () => {
    localStorage.setItem(STORAGE_KEY, "light");
    renderCard();
    expect(screen.getByText("Theme")).toBeTruthy();
    expect(screen.getByRole("tab", { name: "Light" })).toBeTruthy();
    expect(screen.getByRole("tab", { name: "Dark" })).toBeTruthy();
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

  it("with no stored preference (\"system\" default), the segment matching what the OS currently resolves to is selected", () => {
    stubMatchMedia(true); // OS prefers dark
    renderCard();
    expect(screen.getByRole("tab", { name: "Dark" }).getAttribute("aria-selected")).toBe("true");
  });

  it("while on \"system\", an OS-level prefers-color-scheme flip repaints the active segment live", () => {
    stubMatchMedia(false); // OS starts light, no stored preference -> "system"
    renderCard();
    expect(screen.getByRole("tab", { name: "Light" }).getAttribute("aria-selected")).toBe("true");

    // The OS flips to dark. The listener sets state outside any Testing
    // Library helper, so act() makes React flush before the assertion.
    stubMatchMedia(true);
    expect(changeListeners.length).toBeGreaterThan(0);
    act(() => {
      changeListeners.forEach((cb) => cb());
    });

    expect(screen.getByRole("tab", { name: "Dark" }).getAttribute("aria-selected")).toBe("true");
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
