// @vitest-environment jsdom
// The list of what is new: a button of the Version card that counts what is
// left and opens a window with every change under its page, the way there,
// what was seen greyed, and the one button that marks the lot as seen or new.
// The same window holds the release notes on a tab of their own.
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider, en } from "../../lib/i18n";
import { CHANGES, WHATS_NEW_RELEASE } from "../../lib/whatsNew";
import { beginWhatsNew, markAbsent, markSeen, openWhatsNew } from "../../lib/whatsNewStore";
import { WhatsNewButton, WhatsNewHost } from "./WhatsNew";

const KEY = `bv-whats-new@${WHATS_NEW_RELEASE}`;
const stored = () => JSON.parse(localStorage.getItem(KEY)!) as { seen: string[]; absent: string[] };
const markAll = (n: number) => `${en["whatsNew.markAll"]} · ${n}`;
const showAll = `${en["whatsNew.showAll"]} · ${CHANGES.length}`;
const line = (id: string) => document.querySelector<HTMLElement>(`[data-change="${id}"]`)!;

function Where() {
  return <output data-testid="where">{useLocation().pathname}</output>;
}

async function renderApp(tab: "notes" | "changes" | null) {
  await act(async () => {
    render(
      <I18nProvider>
        <MemoryRouter initialEntries={["/settings/info"]}>
          <WhatsNewButton />
          <WhatsNewHost version="v10.0.0" />
          <Where />
        </MemoryRouter>
      </I18nProvider>
    );
    openWhatsNew(tab);
  });
}

const renderCard = () => renderApp("changes");
const renderButton = () => renderApp(null);
const isOpen = () => screen.queryByRole("dialog") !== null;

async function press(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

beforeEach(() => {
  localStorage.clear();
  localStorage.setItem("bv-lang", "en");
  beginWhatsNew("v9.9.1");
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response(JSON.stringify({ ok: true, body: "## Added\n\n- One list." }), { status: 200 }))
  );
});
afterEach(() => {
  act(() => openWhatsNew(null));
  cleanup();
  vi.unstubAllGlobals();
});

describe("the New button", () => {
  const selected = () => screen.getByRole("tab", { selected: true }).textContent ?? "";

  it("counts what is left and opens the window on the list", async () => {
    markSeen(["overview-head"]);
    await renderButton();
    await press(screen.getByRole("button", { name: `${en["whatsNew.title"]} · ${CHANGES.length - 1}` }));
    const window = screen.getByRole("dialog", { name: en["whatsNew.windowTitle"].replace("{version}", WHATS_NEW_RELEASE) });
    expect(selected()).toContain(en["whatsNew.changesTab"]);
    expect(window.querySelectorAll("[data-change]")).toHaveLength(CHANGES.length);
  });

  it("drops the count once everything was seen, and opens the release notes", async () => {
    markSeen(CHANGES.map((change) => change.id));
    await renderButton();
    await press(screen.getByRole("button", { name: en["whatsNew.title"] }));
    expect(selected()).toBe(en["whatsNew.notesTab"]);
    expect(await screen.findByText("One list.")).toBeTruthy();
  });
});

describe("the list of what is new", () => {
  it("names every change once, under the page it is on", async () => {
    await renderCard();
    expect(document.querySelectorAll("[data-change]")).toHaveLength(CHANGES.length);
    const overview = screen.getByRole("heading", { level: 3, name: en["nav.overview"] }).parentElement!;
    expect(within(overview).getByText(en["whatsNew.overviewHead"])).toBeTruthy();
    expect(screen.getByRole("heading", { level: 3, name: `${en["nav.settings"]} › ${en["pairing.title"]}` })).toBeTruthy();
  });

  it("counts what is left on its button and greys what was seen", async () => {
    markSeen(["overview-head", "backups-bar"]);
    await renderCard();
    expect(screen.getByRole("button", { name: markAll(CHANGES.length - 2) })).toBeTruthy();
    expect(line("overview-head").dataset.seen).toBe("true");
    expect(line("overview-head").className).not.toContain("opacity");
    expect(line("backups-tiles").dataset.seen).toBeUndefined();
  });

  it("tells a screen reader which changes are new and where each button leads", async () => {
    markSeen(["overview-head"]);
    await renderCard();
    const go = (id: string) => within(line(id)).getByRole("button", { name: en["whatsNew.go"] });
    const said = (id: string) => document.getElementById(go(id).getAttribute("aria-describedby")!.split(" ")[0])!.textContent;
    expect(said("overview-head")).toBe(en["whatsNew.overviewHead"]);
    expect(said("backups-tiles")).toBe(`${en["whatsNew.title"]}: ${en["whatsNew.backupsTiles"]}`);
  });

  it("leads to the page of a change and closes, and to the list for a page that belongs to one entry", async () => {
    await renderCard();
    await press(within(line("settings-week")).getByRole("button", { name: en["whatsNew.go"] }));
    expect(screen.getByTestId("where").textContent).toBe("/settings/schedules");
    expect(isOpen()).toBe(false);
    await act(async () => openWhatsNew("changes"));
    await press(within(line("entry-page")).getByRole("button", { name: en["whatsNew.go"] }));
    expect(screen.getByTestId("where").textContent).toBe("/backups");
  });

  it("marks everything as seen with one button, and shows everything again with the same", async () => {
    await renderCard();
    expect(screen.queryByRole("button", { name: showAll })).toBeNull();

    await press(screen.getByRole("button", { name: markAll(CHANGES.length) }));
    expect(stored().seen).toHaveLength(CHANGES.length);
    expect(document.querySelectorAll("[data-seen]")).toHaveLength(CHANGES.length);

    await press(screen.getByRole("button", { name: showAll }));
    expect(stored().seen).toEqual([]);
    expect(screen.getByRole("button", { name: markAll(CHANGES.length) })).toBeTruthy();
  });

  it("says of a change that its page does not show it, and still counts it", async () => {
    markAbsent(["overview-decline"]);
    await renderCard();
    expect(line("overview-decline").dataset.absent).toBe("true");
    expect(within(line("overview-decline")).getByText(en["whatsNew.absent"])).toBeTruthy();
    expect(within(line("overview-head")).queryByText(en["whatsNew.absent"])).toBeNull();
    expect(screen.getByRole("button", { name: markAll(CHANGES.length) })).toBeTruthy();

    await press(screen.getByRole("button", { name: markAll(CHANGES.length) }));
    expect(stored().absent).toEqual([]);
    expect(line("overview-decline").dataset.absent).toBeUndefined();
  });

  it("counts everything as seen in a browser that saw no older release", async () => {
    localStorage.removeItem(KEY);
    await renderCard();
    expect(screen.getByRole("button", { name: showAll })).toBeTruthy();
    expect(document.querySelectorAll("[data-seen]")).toHaveLength(CHANGES.length);
  });
});

describe("the release notes", () => {
  const tab = (name: string) => screen.getByRole("tab", { name: new RegExp(`^${name}`) });

  it("open on their own tab beside the list, which counts what is left", async () => {
    markSeen(["overview-head"]);
    await renderApp("notes");
    expect(tab(en["whatsNew.notesTab"]).getAttribute("aria-selected")).toBe("true");
    expect(screen.getByRole("tabpanel", { name: en["whatsNew.notesTab"] }).getAttribute("id")).toBe(
      tab(en["whatsNew.notesTab"]).getAttribute("aria-controls")
    );
    expect(tab(en["whatsNew.changesTab"]).textContent).toContain(`· ${CHANGES.length - 1}`);
    expect(await screen.findByText("One list.")).toBeTruthy();
    expect(screen.getByTestId("release-notes").getAttribute("lang")).toBe("en");
    expect(document.querySelectorAll("[data-change]")).toHaveLength(0);

    await press(tab(en["whatsNew.changesTab"]));
    expect(document.querySelectorAll("[data-change]")).toHaveLength(CHANGES.length);
    await press(tab(en["whatsNew.notesTab"]));
    expect(screen.getByText("One list.")).toBeTruthy();
  });

  it("link to the release on GitHub from the window's foot", async () => {
    await renderApp("notes");
    const link = screen.getByRole("link", { name: en["whatsnew.viewOnGitHub"] });
    expect(link.getAttribute("href")).toBe("https://github.com/junkerderprovinz/bombvault/releases/tag/v10.0.0");
  });
});
