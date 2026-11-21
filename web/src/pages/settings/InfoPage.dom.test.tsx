// @vitest-environment jsdom
// The Info page: BombVault's version with the check for a newer release, the
// versions of what it brings along, the ways to get help and the list of what
// the last update changed. The check runs on the server and can be refused,
// and the bug report can be too; both refusals have to reach the person in words.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, waitFor, within } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { componentText } from "@glimstone/core";
import { I18nProvider, en } from "../../lib/i18n";
import { GLIMSTONE_VERSION } from "../../lib/glimstoneVersion";
import { IconSync } from "../../components/navGlyphs";

const api = vi.hoisted(() => ({
  getVersions: vi.fn(),
  checkForUpdate: vi.fn(),
  downloadDiagnostics: vi.fn(),
}));
const pushed: { message: string; severity?: string }[] = [];

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  ...api,
}));

vi.mock("../../lib/toast", () => ({
  useToast: () => ({
    push: (message: string, severity?: string) => pushed.push({ message, severity }),
    quiet: false,
    setQuiet: () => {},
  }),
}));

const { InfoPage } = await import("./pages/InfoPage");

async function renderPage() {
  await act(async () => {
    render(
      <I18nProvider>
        <MemoryRouter>
          <InfoPage />
        </MemoryRouter>
      </I18nProvider>
    );
  });
}

function card(title: string): HTMLElement {
  const el = document.querySelector<HTMLElement>(`[data-search-card="${title}"]`);
  if (!el) throw new Error(`no card ${title}`);
  return el;
}

const words = componentText("en");
const version = () => within(card(en["about.version"]));
const components = () => within(card(en["tile.info.components"]));
const checkButton = () => version().getByRole("button", { name: words.versionCard.check });
const state = () => version().getByRole("status").textContent ?? "";

async function check() {
  await act(async () => {
    fireEvent.click(checkButton());
  });
}

beforeEach(() => {
  localStorage.setItem("bv-lang", "en");
  api.getVersions.mockResolvedValue({ ok: true, versions: { bombvault: "10.0.0", restic: "0.18.1", rclone: null } });
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  for (const fn of Object.values(api)) fn.mockReset();
  pushed.length = 0;
  localStorage.removeItem("bv-lang");
});

describe("the Version card", () => {
  it("holds BombVault alone, with a number that opens its newest release", async () => {
    await renderPage();
    const link = version().getByRole("link", { name: "10.0.0" });
    expect(link.getAttribute("href")).toBe("https://github.com/junkerderprovinz/bombvault/releases/latest");
    expect(version().getByText("BombVault")).toBeTruthy();
    for (const name of ["GlimStone", "restic", "rclone"]) expect(card(en["about.version"]).textContent).not.toContain(name);
  });

  it("shows a tagged version without its v", async () => {
    api.getVersions.mockResolvedValue({ ok: true, versions: { bombvault: "v10.0.0", restic: "0.18.1", rclone: "v1.75.2" } });
    await renderPage();
    expect(version().getByRole("link", { name: "10.0.0" })).toBeTruthy();
    expect(card(en["about.version"]).textContent).not.toMatch(/\bv\d/);
  });

  it("names GlimStone 4.8.2", () => {
    expect(GLIMSTONE_VERSION).toBe("4.8.2");
  });

  it("asks the server only when Check for updates is pressed, and says nothing before", async () => {
    await renderPage();
    expect(api.checkForUpdate).not.toHaveBeenCalled();
    expect(state()).toBe("");
  });

  it("wears the two arrows on Check for updates", async () => {
    await renderPage();
    const drawn = checkButton().querySelector("svg")?.innerHTML;
    const { container } = render(<IconSync />);
    expect(drawn).toBe(container.querySelector("svg")?.innerHTML);
  });

  it("says it is up to date and when it checked, once nothing newer exists", async () => {
    api.checkForUpdate.mockResolvedValue({
      ok: true,
      update: { current: "10.0.0", latest: "10.0.0", updateAvailable: false, releaseUrl: "" },
    });
    await renderPage();
    await check();
    expect(state()).toContain(words.versionCard.current);
    expect(version().getByRole("status").querySelector(".glim-version-note")?.textContent).toBeTruthy();
    expect(checkButton()).toBeTruthy();
  });

  it("names a newer version, leads to the install guide and lets What's new open its release", async () => {
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    api.checkForUpdate.mockResolvedValue({
      ok: true,
      update: { current: "10.0.0", latest: "v10.0.1", updateAvailable: true, releaseUrl: "https://example.org/v10.0.1" },
    });
    await renderPage();
    await check();
    expect(state()).toContain(words.versionCard.available("10.0.1"));
    expect(version().queryByRole("button", { name: words.versionCard.check })).toBeNull();
    expect(version().queryByRole("link", { name: /10\.0\.1/ })).toBeNull();
    fireEvent.click(version().getByRole("button", { name: words.versionCard.guide }));
    expect(open).toHaveBeenLastCalledWith(
      "https://junkerderprovinz.github.io/bombvault/getting-started/#install-on-unraid",
      "_blank",
      "noopener,noreferrer",
    );
    fireEvent.click(version().getByRole("button", { name: words.versionCard.whatsNew }));
    expect(open).toHaveBeenLastCalledWith("https://example.org/v10.0.1", "_blank", "noopener,noreferrer");
  });

  it.each([
    ["dev-build", "tile.info.refused.devBuild"],
    ["offline", "tile.info.refused.offline"],
    ["rate-limited", "tile.info.refused.rateLimited"],
    ["bad-response", "tile.info.refused.badResponse"],
  ] as const)("says the check failed and why when it is refused as %s", async (code, key) => {
    api.checkForUpdate.mockResolvedValue({ ok: false, code });
    await renderPage();
    await check();
    expect(state()).toContain(words.versionCard.failed);
    expect(state()).toContain(en[key]);
    expect(version().getByRole("status").getAttribute("data-tone")).toBe("fail");
  });

  it("says so as well when the request itself fails", async () => {
    api.checkForUpdate.mockRejectedValue(new Error("HTTP 502 Bad Gateway"));
    await renderPage();
    await check();
    expect(state()).toContain("HTTP 502 Bad Gateway");
  });
});

describe("the Components card", () => {
  it("lists the bundled tools by name and GlimStone last, each with what it is in its (i)", async () => {
    await renderPage();
    const names = [...card(en["tile.info.components"]).querySelectorAll(".glim-version-part")].map((el) => el.textContent);
    expect(names).toEqual(["rclone", "restic", "GlimStone"]);
    for (const tip of [en["tile.info.resticHint"], en["tile.info.rcloneHint"], words.componentsCard.design]) {
      expect(components().getByLabelText(tip)).toBeTruthy();
    }
  });

  it("opens each project's newest release from its number, and says so where a version is unknown", async () => {
    await renderPage();
    expect(components().getByRole("link", { name: "0.18.1" }).getAttribute("href")).toBe("https://github.com/restic/restic/releases/latest");
    expect(components().getByRole("link", { name: GLIMSTONE_VERSION }).getAttribute("href")).toBe(
      "https://github.com/junkerderprovinz/glimstone/releases/latest",
    );
    expect(components().getByText(en["tile.info.versionUnknown"])).toBeTruthy();
  });
});

describe("the Help card", () => {
  const help = () => within(card(en["tile.info.help"]));

  it("opens the welcome page again, for an install that is long past it", async () => {
    function Where() {
      const at = useLocation();
      return <output data-testid="where">{at.pathname + at.search}</output>;
    }
    await act(async () => {
      render(
        <I18nProvider>
          <MemoryRouter>
            <InfoPage />
            <Where />
          </MemoryRouter>
        </I18nProvider>
      );
    });
    const row = help().getByText(en["tile.info.welcome"]).closest("[data-search-row], div")!.parentElement!;
    fireEvent.click(within(row).getByRole("button", { name: en["instances.open"] }));
    expect(document.querySelector('[data-testid="where"]')?.textContent).toBe("/welcome?again");
  });

  it("offers GitHub, Discord, the manual and the bug report, in that order, and no mail", async () => {
    await renderPage();
    const names = help()
      .getAllByRole("button")
      .map((b) => b.getAttribute("aria-label") ?? b.textContent ?? "");
    const at = (word: string) => names.findIndex((n) => n.includes(word));
    const order = [en["about.repo"], en["about.discord"], en["tile.info.manual"], en["tile.info.bugReport"]].map(at);
    expect(order.every((i) => i >= 0)).toBe(true);
    expect([...order].sort((a, b) => a - b)).toEqual(order);
    expect(names.some((n) => /e-?mail/i.test(n))).toBe(false);
    expect(help().getByText(en["about.report"]).textContent).not.toMatch(/e-?mail/i);
  });

  it("names Discord in its sentence and opens the server's invite in the browser", async () => {
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    await renderPage();
    expect(help().getByText(en["about.report"]).textContent).toMatch(/Discord/);
    const discord = help().getByRole("button", { name: new RegExp(en["about.discord"]) });
    expect(discord.closest(".glim-readme-btn-unit")?.className).toContain("glim-tile-discord");
    expect(discord.querySelector(".glim-discord-mark svg")).not.toBeNull();
    fireEvent.click(discord);
    expect(open).toHaveBeenCalledWith("https://discord.gg/RSdmzWtN25", "_blank", "noopener,noreferrer");
  });

  it("opens the documentation in the browser from the manual button, which says Open", async () => {
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    await renderPage();
    const manual = help().getByRole("button", { name: new RegExp(en["tile.info.manual"]) });
    expect(manual.textContent).toContain(en["instances.open"]);
    fireEvent.click(manual);
    expect(open).toHaveBeenCalledWith("https://junkerderprovinz.github.io/bombvault/", "_blank", "noopener,noreferrer");
  });

  it("draws the manual's book closed at rest and brings the open one for the pointer, a finger and the focus", async () => {
    await renderPage();
    const manual = help().getByRole("button", { name: new RegExp(en["tile.info.manual"]) });
    const drawings = [...manual.querySelectorAll(".glim-docs-mark svg")];
    expect(drawings.map((svg) => svg.getAttribute("class"))).toEqual(["glim-mark-rest", "glim-mark-hover"]);
    // Each drawing in the square cropped to its own ink, like the envelope's two.
    expect(drawings.map((svg) => svg.getAttribute("viewBox"))).toEqual(["0 0 14 14", "0 0.125 14 14"]);
  });

  it("puts the bug of the glyph set on the bug report", async () => {
    await renderPage();
    const report = help().getByRole("button", { name: new RegExp(en["tile.info.bugReport"]) });
    expect(report.querySelector(".glim-github-mark svg")?.getAttribute("viewBox")).toBe("0.05 0.05 13.9 13.9");
  });

  it("downloads the bug report when its button is pressed", async () => {
    api.downloadDiagnostics.mockResolvedValue(null);
    await renderPage();
    fireEvent.click(help().getByRole("button", { name: new RegExp(en["tile.info.bugReport"]) }));
    await waitFor(() => expect(api.downloadDiagnostics).toHaveBeenCalledTimes(1));
    expect(pushed).toHaveLength(0);
  });

  it("shows the server's refusal of the bug report in its own words", async () => {
    api.downloadDiagnostics.mockResolvedValue("set a login password before downloading the diagnostics bundle");
    await renderPage();
    fireEvent.click(help().getByRole("button", { name: new RegExp(en["tile.info.bugReport"]) }));
    await waitFor(() => expect(pushed).toHaveLength(1));
    expect(pushed[0].message).toContain("login password");
    expect(pushed[0].severity).toBe("fail");
  });
});

describe("the list of what is new", () => {
  it("is a button of the Version card, and Components, Help and About follow in that order", async () => {
    await renderPage();
    const titles = [...document.querySelectorAll("[data-search-card]")].map((el) => el.getAttribute("data-search-card"));
    expect(titles).toEqual([en["about.version"], en["tile.info.components"], en["tile.info.help"], en["about.title"]]);
    expect(version().getByRole("button", { name: words.versionCard.whatsNew })).toBeTruthy();
  });

  it("has one button for the window, before the update check", async () => {
    await renderPage();
    const names = version()
      .getAllByRole("button")
      .map((button) => button.textContent ?? "");
    const at = (label: string) => names.findIndex((name) => name.includes(label));
    expect(at(en["whatsNew.notesTab"])).toBe(-1);
    expect(at(words.versionCard.whatsNew)).toBeGreaterThanOrEqual(0);
    expect(at(words.versionCard.check)).toBe(at(words.versionCard.whatsNew) + 1);
  });
});

describe("the About card", () => {
  it("holds what BombVault is and the ways to give, and leaves reporting to Help", async () => {
    await renderPage();
    const about = within(card(en["about.title"]));
    expect(about.getByText(en["about.body"])).toBeTruthy();
    expect(about.getByRole("button", { name: new RegExp(en["about.paypal"]) })).toBeTruthy();
    expect(about.queryByRole("button", { name: new RegExp(en["about.repo"]) })).toBeNull();
  });
});
