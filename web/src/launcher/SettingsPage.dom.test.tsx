// @vitest-environment jsdom
// The app's appearance card: OLED and the phone's own colour are the phone's,
// so neither takes the look over from the server it follows. The Version card
// names the store the app came from.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { componentText } from "@glimstone/core";
import { GLIMSTONE_VERSION } from "../lib/glimstoneVersion";
import { I18nProvider, useT } from "../lib/i18n";
import { setSystemAccent } from "../lib/accent";
import { SettingsPage } from "./SettingsPage";
import { following } from "./look";

const APP = { version: "10.6.0", versionCode: 100600, android: "36", model: "Pixel 8", deviceName: "" };

function Harness({ app = APP }: { app?: typeof APP & { installer?: string } }) {
  const { t } = useT();
  return (
    <SettingsPage t={t} app={app} onBack={() => {}} onLanguage={() => {}} onRename={() => {}} onFollow={() => {}} onRemoveAll={() => {}} />
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

function renderPage(installer?: string) {
  render(
    <I18nProvider>
      <Harness app={installer === undefined ? APP : { ...APP, installer }} />
    </I18nProvider>,
  );
}

const words = componentText("en");
const card = (title: string) => within(screen.getByRole("heading", { name: new RegExp(`^${title}`) }).closest("section")!);

describe("the Version and Components cards", () => {
  it.each([
    ["com.android.vending", "Google Play"],
    ["org.fdroid.fdroid", "F-Droid"],
    ["", "GitHub Releases"],
    [undefined, "GitHub Releases"],
  ])("say the updates of an app installed by %s come through %s", (installer, store) => {
    renderPage(installer);
    expect(card(words.versionCard.title).getByRole("status").textContent).toBe(words.versionCard.store(store));
  });

  it("show BombVault with its number and no check, and open the running release's notes", () => {
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    renderPage("com.android.vending");
    const version = card(words.versionCard.title);
    expect(version.getByRole("link", { name: "10.6.0" }).getAttribute("href")).toBe("https://github.com/junkerderprovinz/bombvault/releases/latest");
    expect(version.getAllByRole("button").map((b) => b.textContent)).toEqual([words.versionCard.whatsNew]);
    fireEvent.click(version.getByRole("button"));
    expect(open).toHaveBeenCalledWith("https://github.com/junkerderprovinz/bombvault/releases/tag/v10.6.0", "_blank", "noopener,noreferrer");
    open.mockRestore();
  });

  it("list GlimStone in the Components card", () => {
    renderPage();
    expect(card(words.componentsCard.title).getByRole("link", { name: GLIMSTONE_VERSION })).toBeTruthy();
  });
});

describe("the app's appearance", () => {
  it("switches OLED without leaving the followed look", () => {
    renderPage();
    fireEvent.click(screen.getByRole("switch", { name: "OLED" }));
    expect(document.documentElement.hasAttribute("data-oled")).toBe(true);
    expect(following()).toBe(true);
  });

  it("refuses OLED while Light is chosen and frees it again under Dark", () => {
    renderPage();
    fireEvent.click(screen.getByRole("radio", { name: "Light" }));

    const oled = screen.getByRole("switch", { name: "OLED" });
    fireEvent.click(oled);
    expect(oled.getAttribute("aria-disabled")).toBe("true");
    expect(document.documentElement.hasAttribute("data-oled")).toBe(false);
    expect(screen.getByText("Only with Dark or System. Light is chosen above.")).toBeTruthy();

    fireEvent.click(screen.getByRole("radio", { name: "Dark" }));
    fireEvent.click(screen.getByRole("switch", { name: "OLED" }));
    expect(document.documentElement.hasAttribute("data-oled")).toBe(true);
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
