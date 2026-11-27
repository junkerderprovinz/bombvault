// @vitest-environment jsdom
// The app's appearance card: OLED and the phone's own colour are the phone's,
// so neither takes the look over from the server it follows. The Version card
// names the store the app came from, or the release page.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { componentText } from "@glimstone/core";
import { GLIMSTONE_VERSION } from "../lib/glimstoneVersion";
import { en, I18nProvider, useT } from "../lib/i18n";
import { setSystemAccent } from "../lib/accent";
import { SettingsPage } from "./SettingsPage";
import { following } from "./look";
import { RELEASES } from "../lib/whatsNew";

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
  ])("say the updates of an app installed by %s come through %s", (installer, store) => {
    renderPage(installer);
    expect(card(words.versionCard.title).getByRole("status").textContent).toBe(words.versionCard.store(store));
  });

  it("send an APK installed by hand to the release page", () => {
    renderPage("");
    expect(card(words.versionCard.title).getByRole("status").textContent).toBe(words.versionCard.direct);
  });

  it("name no store for an app too old to say which one installed it", () => {
    renderPage();
    expect(card(words.versionCard.title).getByRole("status").textContent).toBe(words.versionCard.anyStore);
  });

  it("show BombVault with its number and no check, and open the running release's notes inside the app", async () => {
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    renderPage("com.android.vending");
    const version = card(words.versionCard.title);
    expect(version.getByRole("link", { name: "10.6.0" }).getAttribute("href")).toBe("https://github.com/junkerderprovinz/bombvault/releases/latest");
    expect(version.getAllByRole("button").map((b) => b.textContent)).toEqual([words.versionCard.whatsNew]);
    fireEvent.click(version.getByRole("button"));
    const notes = await screen.findByRole("dialog", { name: en["whatsNew.windowTitle"].replace("{version}", "10.6.0") });
    expect(await within(notes).findByText(/^BombVault 10\.6\.0 moves into the fortress/)).toBeTruthy();
    expect(within(notes).getByRole("link", { name: en["whatsnew.viewOnGitHub"] }).getAttribute("href")).toBe(
      "https://github.com/junkerderprovinz/bombvault/releases/tag/v10.6.0",
    );
    expect(open).not.toHaveBeenCalled();
    open.mockRestore();
  });

  it("list what the running release brought and leave out what a phone cannot open", async () => {
    render(
      <I18nProvider>
        <Harness app={{ ...APP, version: "10.7.0" }} />
      </I18nProvider>,
    );
    fireEvent.click(card(words.versionCard.title).getByRole("button"));
    const notes = await screen.findByRole("dialog", { name: en["whatsNew.windowTitle"].replace("{version}", "10.7.0") });
    expect(within(notes).getByText(en["whatsNew.signInServices"])).toBeTruthy();
    expect(within(notes).queryByText(en["whatsNew.instanceAddress"])).toBeNull();
  });

  it("show a build of no release the newest release's notes, as the web does", async () => {
    render(
      <I18nProvider>
        <Harness app={{ ...APP, version: "0.0.465" }} />
      </I18nProvider>,
    );
    fireEvent.click(card(words.versionCard.title).getByRole("button"));
    const newest = RELEASES[0].version;
    const notes = await screen.findByRole("dialog", { name: en["whatsNew.windowTitle"].replace("{version}", newest) });
    expect(await within(notes).findByTestId("release-notes")).toBeTruthy();
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

describe("what the web's look settings offer per device", () => {
  it("chooses the labels of the buttons and takes the look over from the server", () => {
    renderPage();
    const labels = within(screen.getByRole("radiogroup", { name: "Buttons" }));
    fireEvent.click(labels.getByRole("radio", { name: "Symbol" }));
    expect(localStorage.getItem("bv-labels-buttons")).toBe("glyph");
    expect(document.documentElement.getAttribute("data-labels-buttons")).toBe("glyph");
    expect(following()).toBe(false);
  });

  it("gives every label choice a glyph, so a narrow phone shows the glyphs alone", () => {
    renderPage();
    const choices = within(screen.getByRole("radiogroup", { name: "Buttons" })).getAllByRole("radio");
    expect(choices).toHaveLength(4);
    for (const choice of choices) expect(choice.querySelector("svg")).not.toBeNull();
  });

  it("offers Reactive Mode and Colour Rotation under the rainbow", () => {
    renderPage();
    expect(screen.queryByRole("switch", { name: "Reactive Mode" })).toBeNull();
    fireEvent.click(screen.getByRole("switch", { name: "Rainbow Mode" }));
    fireEvent.click(screen.getByRole("switch", { name: "Reactive Mode" }));
    fireEvent.click(screen.getByRole("switch", { name: "Colour Rotation" }));
    expect(JSON.parse(localStorage.getItem("bv-rainbow")!)).toMatchObject({ on: true, reactive: true, rotate: true });
    expect(following()).toBe(false);
  });

  it("keeps quiet notifications on the phone without leaving the followed look", () => {
    renderPage();
    fireEvent.click(screen.getByRole("switch", { name: "Quiet notifications" }));
    expect(localStorage.getItem("bombvault.quietToasts")).toBe("1");
    expect(following()).toBe(true);
  });

  it("opens the manual", () => {
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /^Manual/ }));
    expect(open).toHaveBeenCalledWith("https://junkerderprovinz.github.io/bombvault/", "_blank", "noopener,noreferrer");
    open.mockRestore();
  });
});
