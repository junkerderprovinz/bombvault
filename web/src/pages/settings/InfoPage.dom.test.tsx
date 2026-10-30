// @vitest-environment jsdom
// The Info page: the versions of what BombVault is built from, the check for
// a newer release, the ways to get help and the list of what the last update
// changed. The check runs on the server and can be refused, and the bug report
// can be too; both refusals have to reach the person in words.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { I18nProvider, en } from "../../lib/i18n";
import { GLIMSTONE_VERSION } from "../../lib/glimstoneVersion";
import { IconUpdate } from "../../components/glyphs";

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

const checkButton = () => within(card(en["about.version"])).getAllByRole("button").at(-1)!;

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
  it("lists BombVault, GlimStone, restic and rclone, each in a row of its own", async () => {
    await renderPage();
    const text = card(en["about.version"]).textContent ?? "";
    for (const name of ["BombVault", "GlimStone", "restic", "rclone"]) expect(text).toContain(name);
    expect(text).toContain("0.18.1");
    expect(within(card(en["about.version"])).getByText(en["tile.info.versionUnknown"])).toBeTruthy();
  });

  it("links BombVault and GlimStone to their release pages and the bundled tools to nothing", async () => {
    await renderPage();
    const links = within(card(en["about.version"]))
      .getAllByRole("link")
      .map((a) => a.getAttribute("href"));
    expect(links).toEqual([
      "https://github.com/junkerderprovinz/bombvault/releases/tag/v10.0.0",
      `https://github.com/junkerderprovinz/glimstone/releases/tag/v${GLIMSTONE_VERSION}`,
    ]);
  });

  it("names GlimStone 3.7.1", () => {
    expect(GLIMSTONE_VERSION).toBe("3.7.1");
  });

  it("asks the server only when the button is pressed", async () => {
    await renderPage();
    expect(api.checkForUpdate).not.toHaveBeenCalled();
    expect(checkButton().textContent).toContain(en["tile.info.check"]);
  });

  it("wears the update glyph before the check has run", async () => {
    await renderPage();
    const drawn = checkButton().querySelector("svg")?.innerHTML;
    const { container } = render(<IconUpdate />);
    expect(drawn).toBe(container.querySelector("svg")?.innerHTML);
  });

  it("turns the button green and says Up to date when nothing newer exists", async () => {
    api.checkForUpdate.mockResolvedValue({
      ok: true,
      update: { current: "10.0.0", latest: "10.0.0", updateAvailable: false, releaseUrl: "" },
    });
    await renderPage();
    await act(async () => {
      fireEvent.click(checkButton());
    });
    expect(checkButton().textContent).toContain(en["tile.info.upToDate"]);
    expect(checkButton().className).toContain("bg-statusOkSolid");
  });

  it("turns it orange with the number of updates and links the release", async () => {
    api.checkForUpdate.mockResolvedValue({
      ok: true,
      update: { current: "10.0.0", latest: "10.0.1", updateAvailable: true, releaseUrl: "https://example.org/v10.0.1" },
    });
    await renderPage();
    await act(async () => {
      fireEvent.click(checkButton());
    });
    expect(checkButton().textContent).toContain("1 update");
    expect(checkButton().className).toContain("bg-statusWarnSolid");
    const release = within(card(en["about.version"])).getByRole("link", { name: /10\.0\.1/ });
    expect(release.getAttribute("href")).toBe("https://example.org/v10.0.1");
  });

  it.each([
    ["dev-build", "tile.info.refused.devBuild"],
    ["offline", "tile.info.refused.offline"],
    ["rate-limited", "tile.info.refused.rateLimited"],
    ["bad-response", "tile.info.refused.badResponse"],
  ] as const)("turns it red, shakes it and says why when the check is refused as %s", async (code, key) => {
    api.checkForUpdate.mockResolvedValue({ ok: false, code });
    await renderPage();
    await act(async () => {
      fireEvent.click(checkButton());
    });
    expect(checkButton().className).toContain("bg-statusFailSolid");
    expect(checkButton().className).toContain("glim-shake");
    expect(within(card(en["about.version"])).getByText(en[key])).toBeTruthy();
  });

  it("says so as well when the request itself fails", async () => {
    api.checkForUpdate.mockRejectedValue(new Error("HTTP 502 Bad Gateway"));
    await renderPage();
    await act(async () => {
      fireEvent.click(checkButton());
    });
    expect(checkButton().className).toContain("glim-shake");
    expect(within(card(en["about.version"])).getByText("HTTP 502 Bad Gateway")).toBeTruthy();
  });
});

describe("the Help card", () => {
  const help = () => within(card(en["tile.info.help"]));

  it("offers GitHub, Email, the manual and the bug report, in that order", async () => {
    await renderPage();
    const names = help()
      .getAllByRole("button")
      .map((b) => b.getAttribute("aria-label") ?? b.textContent ?? "");
    const at = (word: string) => names.findIndex((n) => n.includes(word));
    const order = [en["about.repo"], en["about.mail"], en["tile.info.manual"], en["tile.info.bugReport"]].map(at);
    expect(order.every((i) => i >= 0)).toBe(true);
    expect([...order].sort((a, b) => a - b)).toEqual(order);
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
  it("is a button of the Version card, and Help and About follow without a card of its own", async () => {
    await renderPage();
    const titles = [...document.querySelectorAll("[data-search-card]")].map((el) => el.getAttribute("data-search-card"));
    expect(titles).toEqual([en["about.version"], en["tile.info.help"], en["about.title"]]);
    expect(within(card(en["about.version"])).getByRole("button", { name: en["whatsNew.title"] })).toBeTruthy();
  });

  it("has one button for the window, before the update check", async () => {
    await renderPage();
    const names = within(card(en["about.version"]))
      .getAllByRole("button")
      .map((button) => button.textContent ?? "");
    const at = (label: string) => names.findIndex((name) => name.includes(label));
    expect(at(en["whatsNew.notesTab"])).toBe(-1);
    expect(at(en["whatsNew.title"])).toBeGreaterThanOrEqual(0);
    expect(at(en["tile.info.check"])).toBe(at(en["whatsNew.title"]) + 1);
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
