// @vitest-environment jsdom
// The app's appearance card: OLED and the phone's own colour are the phone's,
// so neither takes the look over from the server it follows.
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, useT } from "../lib/i18n";
import { setSystemAccent } from "../lib/accent";
import { SettingsPage } from "./SettingsPage";
import { following } from "./look";

const APP = { version: "10.6.0", versionCode: 100600, android: "36", model: "Pixel 8", deviceName: "" };

function Harness() {
  const { t } = useT();
  return (
    <SettingsPage t={t} app={APP} onBack={() => {}} onLanguage={() => {}} onRename={() => {}} onFollow={() => {}} onRemoveAll={() => {}} />
  );
}

beforeEach(() => {
  localStorage.clear();
  localStorage.setItem("bv-theme", "dark");
  document.documentElement.setAttribute("data-theme", "dark");
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  })) as unknown as typeof window.matchMedia;
});

afterEach(() => {
  cleanup();
  act(() => setSystemAccent(undefined));
  document.documentElement.removeAttribute("data-theme");
  document.documentElement.removeAttribute("data-oled");
  document.documentElement.removeAttribute("style");
});

function renderPage() {
  render(
    <I18nProvider>
      <Harness />
    </I18nProvider>,
  );
}

describe("the app's appearance", () => {
  it("switches OLED without leaving the followed look", () => {
    renderPage();
    fireEvent.click(screen.getByRole("switch", { name: "OLED" }));
    expect(document.documentElement.hasAttribute("data-oled")).toBe(true);
    expect(following()).toBe(true);
  });

  it("offers Material You's colour first and keeps the choice on the phone", () => {
    renderPage();
    expect(screen.queryByRole("button", { name: "System color" })).toBeNull();
    act(() => setSystemAccent({ light: "#3F5F91", dark: "#A8C8FF" }));
    const wand = screen.getByRole("button", { name: "System color" });
    fireEvent.click(wand);
    expect(wand.getAttribute("aria-pressed")).toBe("true");
    expect(localStorage.getItem("bv-accent-device")).toBe("system");
    expect(document.documentElement.style.getPropertyValue("--accent")).toBe("#A8C8FF");
    expect(following()).toBe(true);
  });
});
