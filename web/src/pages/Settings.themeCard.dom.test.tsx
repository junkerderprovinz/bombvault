// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// ThemeCard (GlimStone follow-up pass, live-review round) — the dark/light
// toggle, moved out of Sidebar.tsx's own footer into its own Card in
// Settings' General tab. This covers the toggle's OWN behaviour in its new
// home: it renders the current mode, clicking it flips dark<->light and
// persists via lib/theme.ts's toggleTheme()/setTheme() (same STORAGE_KEY,
// same data-theme paint), and an OS-level prefers-color-scheme flip repaints
// the label live while "system" is the stored preference — the exact same
// state machine SidebarControls' old theme row had, just relocated.
// Sidebar.language.dom.test.tsx (sibling file) is the other half: proving
// the OLD location no longer renders it.
//
// jsdom opted in explicitly (real DOM/click behaviour needed, plus a
// matchMedia stub lib/theme.ts's getResolvedTheme()/onSystemThemeChange()
// both call — jsdom itself doesn't implement matchMedia) — see
// Selector.dom.test.tsx's own header comment for this repo's naming
// convention for the jsdom-opted-in exception.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { ThemeCard } from "./Settings";
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

/** Minimal matchMedia stub — `matches` reflects whatever the test wants the
 * OS's prefers-color-scheme to currently be, and `addEventListener`
 * captures the change callback so a test can fire it manually to simulate
 * an OS-level flip (matching lib/theme.ts's onSystemThemeChange() contract). */
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
  it("renders as a Card with the theme heading and the current mode on its trigger", () => {
    localStorage.setItem(STORAGE_KEY, "light");
    renderCard();
    expect(screen.getByText("Theme")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Light" })).toBeTruthy();
  });

  it("clicking the trigger flips light to dark and persists via lib/theme.ts", () => {
    localStorage.setItem(STORAGE_KEY, "light");
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Light" }));
    expect(screen.getByRole("button", { name: "Dark" })).toBeTruthy();
    expect(localStorage.getItem(STORAGE_KEY)).toBe("dark");
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
  });

  it("clicking again flips dark back to light", () => {
    localStorage.setItem(STORAGE_KEY, "dark");
    renderCard();
    fireEvent.click(screen.getByRole("button", { name: "Dark" }));
    expect(screen.getByRole("button", { name: "Light" })).toBeTruthy();
    expect(localStorage.getItem(STORAGE_KEY)).toBe("light");
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
  });

  it("with no stored preference (\"system\" default), shows whatever the OS currently resolves to", () => {
    stubMatchMedia(true); // OS prefers dark
    renderCard();
    expect(screen.getByRole("button", { name: "Dark" })).toBeTruthy();
  });

  it("while on \"system\", an OS-level prefers-color-scheme flip repaints the label live", () => {
    stubMatchMedia(false); // OS starts light, no stored preference -> "system"
    renderCard();
    expect(screen.getByRole("button", { name: "Light" })).toBeTruthy();

    // Simulate the OS flipping to dark: update what matchMedia reports, then
    // fire the captured change listener the way a real matchMedia would.
    // act() wraps this because the listener fires the component's setState
    // OUTSIDE any Testing Library helper (unlike fireEvent, which wraps
    // automatically) — without it, the assertion below can run before React
    // flushes the resulting re-render.
    stubMatchMedia(true);
    expect(changeListeners.length).toBeGreaterThan(0);
    act(() => {
      changeListeners.forEach((cb) => cb());
    });

    expect(screen.getByRole("button", { name: "Dark" })).toBeTruthy();
  });

  it("once explicitly toggled off \"system\", a later OS flip does NOT change the label", () => {
    localStorage.setItem(STORAGE_KEY, "light");
    renderCard();
    // Explicit preference is "light" — an OS-level flip must be ignored.
    stubMatchMedia(true);
    act(() => {
      changeListeners.forEach((cb) => cb());
    });
    expect(screen.getByRole("button", { name: "Light" })).toBeTruthy();
    expect(localStorage.getItem(STORAGE_KEY)).toBe("light");
  });
});
