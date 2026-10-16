// @vitest-environment jsdom
// The window of what is new: a button of the Version card that counts what is
// left, and one card per release since the one the browser saw before, each
// with its notes behind a button and its changes under their pages, the way
// there, what was seen greyed, and the one button that marks the lot as seen
// or new.
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider, en } from "../../lib/i18n";
import { CHANGES, placed, RELEASES } from "../../lib/whatsNew";
import { beginWhatsNew, markAbsent, markSeen, openWhatsNew } from "../../lib/whatsNewStore";
import { WhatsNewButton, WhatsNewHost } from "./WhatsNew";

const KEY = "bv-whats-new";
const NEWEST = RELEASES[0].version;
// A release that brought fixes only: the server knows it, the list does not.
const QUIET = "9.9.5";
const stored = () => JSON.parse(localStorage.getItem(KEY)!) as { seen: string[]; absent: string[] };
const markAll = (n: number) => `${en["whatsNew.markAll"]} · ${n}`;
const showAll = `${en["whatsNew.showAll"]} · ${CHANGES.length}`;
const line = (id: string) => document.querySelector<HTMLElement>(`[data-change="${id}"]`)!;
const card = (version: string) => document.querySelector<HTMLElement>(`[data-release="${version}"]`)!;
const cards = () => [...document.querySelectorAll<HTMLElement>("[data-release]")].map((el) => el.dataset.release);
const notesButton = (version: string) => within(card(version)).getByRole("button", { name: en["whatsNew.notesTab"] });
const unplaced = CHANGES.filter((change) => !placed(change));

function Where() {
  return <output data-testid="where">{useLocation().pathname}</output>;
}

async function renderApp(open: boolean) {
  await act(async () => {
    render(
      <I18nProvider>
        <MemoryRouter initialEntries={["/settings/info"]}>
          <WhatsNewButton />
          <WhatsNewHost version={`v${NEWEST}`} />
          <Where />
        </MemoryRouter>
      </I18nProvider>
    );
    openWhatsNew(open);
  });
}

const renderWindow = () => renderApp(true);
const renderButton = () => renderApp(false);
const isOpen = () => screen.queryByRole("dialog") !== null;

async function press(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

function serve(versions: string[] | null) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) => {
      if (url.startsWith("/api/release-notes/versions")) {
        return versions ? new Response(JSON.stringify({ ok: true, versions: versions.map((v) => `v${v}`) }), { status: 200 }) : new Response("", { status: 500 });
      }
      return new Response(JSON.stringify({ ok: true, body: "## Added\n\n- One list." }), { status: 200 });
    })
  );
}

beforeEach(() => {
  localStorage.clear();
  localStorage.setItem("bv-lang", "en");
  beginWhatsNew("v9.9.1");
  serve([...RELEASES.map((r) => r.version), QUIET]);
});
afterEach(() => {
  act(() => openWhatsNew(false));
  cleanup();
  vi.unstubAllGlobals();
});

describe("the New button", () => {
  it("counts what is left and opens the window", async () => {
    markSeen(["overview-head"]);
    await renderButton();
    await press(screen.getByRole("button", { name: `${en["whatsNew.title"]} · ${CHANGES.length - 1}` }));
    expect(screen.getByRole("dialog", { name: en["whatsNew.windowTitleSince"].replace("{version}", "9.9.1") })).toBeTruthy();
  });

  it("drops the count once everything was seen", async () => {
    markSeen(CHANGES.map((change) => change.id));
    await renderButton();
    expect(screen.getByRole("button", { name: en["whatsNew.title"] })).toBeTruthy();
  });
});

describe("the cards", () => {
  it("show every release since the last one seen, newest first, each with its own changes", async () => {
    await renderWindow();
    expect(cards()).toEqual([...RELEASES.map((r) => r.version), QUIET]);
    for (const release of RELEASES) {
      expect(card(release.version).querySelectorAll("[data-change]")).toHaveLength(release.changes.length);
    }
    expect(card(QUIET).querySelectorAll("[data-change]")).toHaveLength(0);
  });

  it("open the notes of the newest release and keep the others behind their button", async () => {
    await renderWindow();
    expect(await within(card(NEWEST)).findByText("One list.")).toBeTruthy();
    expect(notesButton(NEWEST).getAttribute("aria-expanded")).toBe("true");
    expect(notesButton(QUIET).getAttribute("aria-expanded")).toBe("false");
    expect(within(card(QUIET)).queryByText("One list.")).toBeNull();
    await press(notesButton(QUIET));
    expect(await within(card(QUIET)).findByText("One list.")).toBeTruthy();
  });

  it("fall back to the list's releases when the server does not answer", async () => {
    serve(null);
    await renderWindow();
    expect(cards()).toEqual(RELEASES.map((r) => r.version));
  });

  it("name each change under the page it is on, and those no page shows first", async () => {
    await renderWindow();
    const v10 = card("10.0.0");
    const overview = within(v10).getByRole("heading", { level: 4, name: en["nav.overview"] }).parentElement!;
    expect(within(overview).getByText(en["whatsNew.overviewHead"])).toBeTruthy();
    expect(within(v10).getByRole("heading", { level: 4, name: `${en["nav.settings"]} › ${en["pairing.title"]}` })).toBeTruthy();
    for (const change of unplaced) {
      const headings = within(card(change.since)).getAllByRole("heading", { level: 4 });
      expect(headings[0].textContent).toBe(en["whatsNew.page.none"]);
      expect(within(line(change.id)).queryByRole("button", { name: en["whatsNew.go"] })).toBeNull();
    }
  });

  it("show only the running release in a browser that saw no older one", async () => {
    localStorage.removeItem(KEY);
    await renderWindow();
    expect(cards()).toEqual([NEWEST]);
    expect(screen.getByRole("dialog", { name: en["whatsNew.windowTitle"].replace("{version}", NEWEST) })).toBeTruthy();
    expect(document.querySelectorAll("[data-seen]")).toHaveLength(card(NEWEST).querySelectorAll("[data-change]").length);
  });
});

describe("the changes", () => {
  it("count what is left on the button and grey what was seen", async () => {
    markSeen(["overview-head", "backups-bar"]);
    await renderWindow();
    expect(screen.getByRole("button", { name: markAll(CHANGES.length - 2) })).toBeTruthy();
    expect(line("overview-head").dataset.seen).toBe("true");
    expect(line("overview-head").className).not.toContain("opacity");
    expect(line("backups-tiles").dataset.seen).toBeUndefined();
  });

  it("tell a screen reader which changes are new and where each button leads", async () => {
    markSeen(["overview-head"]);
    await renderWindow();
    const go = (id: string) => within(line(id)).getByRole("button", { name: en["whatsNew.go"] });
    const said = (id: string) => document.getElementById(go(id).getAttribute("aria-describedby")!.split(" ")[0])!.textContent;
    expect(said("overview-head")).toBe(en["whatsNew.overviewHead"]);
    expect(said("backups-tiles")).toBe(`${en["whatsNew.title"]}: ${en["whatsNew.backupsTiles"]}`);
  });

  it("lead to the page of a change and close, and to the list for a page that belongs to one entry", async () => {
    await renderWindow();
    await press(within(line("settings-week")).getByRole("button", { name: en["whatsNew.go"] }));
    expect(screen.getByTestId("where").textContent).toBe("/settings/schedules");
    expect(isOpen()).toBe(false);
    await act(async () => openWhatsNew(true));
    await press(within(line("entry-page")).getByRole("button", { name: en["whatsNew.go"] }));
    expect(screen.getByTestId("where").textContent).toBe("/backups");
  });

  it("are all marked as seen with one button, and shown again with the same", async () => {
    await renderWindow();
    expect(screen.queryByRole("button", { name: showAll })).toBeNull();

    await press(screen.getByRole("button", { name: markAll(CHANGES.length) }));
    expect(stored().seen).toHaveLength(CHANGES.length);
    expect(document.querySelectorAll("[data-seen]")).toHaveLength(CHANGES.length);

    await press(screen.getByRole("button", { name: showAll }));
    expect(stored().seen).toEqual([]);
    expect(screen.getByRole("button", { name: markAll(CHANGES.length) })).toBeTruthy();
  });

  it("say of a change that its page does not show it, and still count it", async () => {
    markAbsent(["overview-decline"]);
    await renderWindow();
    expect(line("overview-decline").dataset.absent).toBe("true");
    expect(within(line("overview-decline")).getByText(en["whatsNew.absent"])).toBeTruthy();
    expect(within(line("overview-head")).queryByText(en["whatsNew.absent"])).toBeNull();

    await press(screen.getByRole("button", { name: markAll(CHANGES.length) }));
    expect(stored().absent).toEqual([]);
    expect(line("overview-decline").dataset.absent).toBeUndefined();
  });

  it("count as seen once the window that listed them closes, when no page shows them", async () => {
    await renderWindow();
    await press(screen.getByRole("button", { name: en["common.close"] }));
    expect(stored().seen.sort()).toEqual(unplaced.map((change) => change.id).sort());
  });
});

describe("the window's foot", () => {
  it("links to the running release on GitHub", async () => {
    await renderWindow();
    const link = screen.getByRole("link", { name: en["whatsnew.viewOnGitHub"] });
    expect(link.getAttribute("href")).toBe(`https://github.com/junkerderprovinz/bombvault/releases/tag/v${NEWEST}`);
  });
});
