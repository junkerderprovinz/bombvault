// @vitest-environment jsdom
// The Info page: BombVault's version and where its updates stand, the
// components it ships, the ways to get help and the list of what the last
// update changed. The check runs on the server and can be refused, and the bug
// report can be too; both refusals have to reach the person in words.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, waitFor, within } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { COMPONENT_LANGUAGES, componentText } from "@glimstone/core";
import { I18nProvider, en } from "../../lib/i18n";
import { GLIMSTONE_VERSION } from "../../lib/glimstoneVersion";
import { allLocales } from "../../lib/localesForTests";

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

const { InfoPage, UPDATE_GUIDE } = await import("./pages/InfoPage");

const words = componentText("en");
const LAST_CHECK = "bombvault.lastUpdateCheck";

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

const version = () => within(card(words.versionCard.title));
const components = () => within(card(words.componentsCard.title));
const status = () => version().getByRole("status").textContent ?? "";
const button = (name: string) => version().queryByRole("button", { name: new RegExp(`^${name}`) });

async function check() {
  await act(async () => {
    fireEvent.click(button(words.versionCard.check)!);
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
  localStorage.removeItem(LAST_CHECK);
});

describe("the Info tile", () => {
  it("has the Version, Components, Help and About cards, in that order", async () => {
    await renderPage();
    const titles = [...document.querySelectorAll("[data-search-card]")].map((el) => el.getAttribute("data-search-card"));
    expect(titles).toEqual([words.versionCard.title, words.componentsCard.title, en["tile.info.help"], "About"]);
  });

  it.each(COMPONENT_LANGUAGES)("lets the settings search find the cards by GlimStone's words in %s", (lang) => {
    const t = allLocales[lang];
    const w = componentText(lang);
    expect([t["about.version"], t["tile.info.components"], t["tile.info.check"], t["tile.info.whatsNew"]]).toEqual([
      w.versionCard.title,
      w.componentsCard.title,
      w.versionCard.check,
      w.versionCard.whatsNew,
    ]);
    expect(t["about.title"]).not.toContain("BombVault");
  });
});

describe("the Version card", () => {
  it("shows BombVault alone, its number opening the newest release", async () => {
    await renderPage();
    expect(card(words.versionCard.title).textContent).toContain("BombVault");
    expect(card(words.versionCard.title).textContent).not.toMatch(/GlimStone|restic|rclone/);
    const links = version()
      .getAllByRole("link")
      .map((a) => [a.textContent, a.getAttribute("href")]);
    expect(links).toEqual([["10.0.0", "https://github.com/junkerderprovinz/bombvault/releases/latest"]]);
  });

  it("shows a tagged version without its v", async () => {
    api.getVersions.mockResolvedValue({ ok: true, versions: { bombvault: "v10.0.0", restic: "0.18.1", rclone: "v1.75.2" } });
    await renderPage();
    expect(version().getByRole("link", { name: "10.0.0" })).toBeTruthy();
    expect(card(words.versionCard.title).textContent).not.toMatch(/\bv\d/);
  });

  it("calls a build without a release number what it is and links nothing", async () => {
    api.getVersions.mockResolvedValue({ ok: true, versions: { bombvault: "dev", restic: "0.18.1", rclone: null } });
    await renderPage();
    expect(version().getByText(words.versionCard.unreleased("dev"))).toBeTruthy();
    expect(version().queryAllByRole("link")).toEqual([]);
  });

  it("reads a version number as plain text and never as a button or with an underline", async () => {
    await renderPage();
    const number = version().getByRole("link", { name: "10.0.0" });
    expect(number.className).toContain("no-underline");
    expect(number.className).toContain("text-carbon-text");
    expect(number.className).toContain("hover:text-accentText");
    expect(number.className).not.toMatch(/\bbg-|\bborder\b|rounded/);
  });

  it("asks the server only when Check for updates is pressed", async () => {
    await renderPage();
    expect(api.checkForUpdate).not.toHaveBeenCalled();
    expect(status()).toBe(words.versionCard.current);
    expect(button(words.versionCard.check)).toBeTruthy();
  });

  it("puts What's new, then Check for updates, in the last row and no update button while nothing waits", async () => {
    await renderPage();
    const names = version()
      .getAllByRole("button")
      .map((b) => b.textContent ?? "");
    expect(names).toHaveLength(2);
    expect(names[0]).toContain(words.versionCard.whatsNew);
    expect(names[1]).toContain(words.versionCard.check);
  });

  it("says it is up to date and when it last looked", async () => {
    api.checkForUpdate.mockResolvedValue({
      ok: true,
      update: { current: "10.0.0", latest: "v10.0.0", updateAvailable: false, releaseUrl: "" },
    });
    await renderPage();
    await check();
    expect(api.checkForUpdate).toHaveBeenCalledTimes(1);
    expect(status()).toContain(words.versionCard.current);
    expect(status()).toContain(words.versionCard.checked(new Date()));
    expect(button(words.versionCard.check)).toBeTruthy();
  });

  it("offers How to update while a newer release waits, hides the check and opens the guide", async () => {
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    api.checkForUpdate.mockResolvedValue({
      ok: true,
      update: { current: "10.0.0", latest: "v10.0.1", updateAvailable: true, releaseUrl: "https://example.org/v10.0.1" },
    });
    await renderPage();
    await check();
    expect(status()).toBe(words.versionCard.available("10.0.1"));
    expect(version().getByRole("status").querySelector("a")).toBeNull();
    expect(button(words.versionCard.check)).toBeNull();
    fireEvent.click(button(words.versionCard.guide)!);
    expect(open).toHaveBeenCalledWith(UPDATE_GUIDE, "_blank", "noopener,noreferrer");
    expect(UPDATE_GUIDE).toBe("https://junkerderprovinz.github.io/bombvault/getting-started/#update");
  });

  it("opens the waiting release's notes from What's new", async () => {
    api.checkForUpdate.mockResolvedValue({
      ok: true,
      update: { current: "10.0.0", latest: "v10.0.1", updateAvailable: true, releaseUrl: "" },
    });
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({ ok: true, body: "" }), { status: 200 }));
    await renderPage();
    await check();
    await act(async () => {
      fireEvent.click(button(words.versionCard.whatsNew)!);
    });
    expect(document.querySelector('[role="dialog"]')?.textContent).toContain(en["whatsNew.windowTitle"].replace("{version}", "10.0.1"));
  });

  it("remembers the last answer for the version that asked, and forgets it after an update", async () => {
    localStorage.setItem(LAST_CHECK, JSON.stringify({ at: new Date().toISOString(), running: "10.0.0", latest: "v10.0.1", available: true }));
    await renderPage();
    expect(status()).toBe(words.versionCard.available("10.0.1"));
    cleanup();

    api.getVersions.mockResolvedValue({ ok: true, versions: { bombvault: "10.0.1", restic: "0.18.1", rclone: null } });
    await renderPage();
    expect(status()).toBe(words.versionCard.current);
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
    expect(status()).toBe(words.versionCard.failed + en[key]);
    expect(version().getByText(words.versionCard.failed).className).toContain("text-statusFail");
    expect(button(words.versionCard.check)).toBeTruthy();
  });

  it("says so as well when the request itself fails", async () => {
    api.checkForUpdate.mockRejectedValue(new Error("HTTP 502 Bad Gateway"));
    await renderPage();
    await check();
    expect(version().getByText("HTTP 502 Bad Gateway")).toBeTruthy();
  });
});

describe("the Components card", () => {
  it("lists the programs by name, then GlimStone, each with its (i) and no line under it", async () => {
    await renderPage();
    const rows = [...card(words.componentsCard.title).querySelectorAll("[data-search-row]")];
    expect(rows.map((r) => r.getAttribute("data-search-row"))).toEqual(["rclone", "restic", "GlimStone"]);
    const tips = rows.map((r) => r.querySelector('[role="img"]')?.getAttribute("aria-label"));
    expect(tips).toEqual([en["tile.info.rcloneHint"], en["tile.info.resticHint"], words.componentsCard.design]);
  });

  it("links every number it knows to its project's newest release and says when one is unknown", async () => {
    await renderPage();
    const links = components()
      .getAllByRole("link")
      .map((a) => [a.textContent, a.getAttribute("href")]);
    expect(links).toEqual([
      ["0.18.1", "https://github.com/restic/restic/releases/latest"],
      [GLIMSTONE_VERSION, "https://github.com/junkerderprovinz/glimstone/releases/latest"],
    ]);
    expect(components().getByText(en["tile.info.versionUnknown"])).toBeTruthy();
  });

  it("offers no switch and no button, since its programs come with BombVault's own release", async () => {
    await renderPage();
    expect(components().queryAllByRole("button")).toEqual([]);
    expect(components().queryAllByRole("switch")).toEqual([]);
    expect(components().queryAllByRole("checkbox")).toEqual([]);
  });

  it("names GlimStone 4.8.2", () => {
    expect(GLIMSTONE_VERSION).toBe("4.8.2");
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

describe("the About card", () => {
  it("holds what BombVault is and the ways to give, and leaves reporting to Help", async () => {
    await renderPage();
    const about = within(card(en["about.title"]));
    expect(about.getByText(en["about.body"])).toBeTruthy();
    expect(about.getByRole("button", { name: new RegExp(en["about.paypal"]) })).toBeTruthy();
    expect(about.queryByRole("button", { name: new RegExp(en["about.repo"]) })).toBeNull();
  });
});
