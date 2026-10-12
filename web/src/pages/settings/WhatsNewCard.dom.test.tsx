// @vitest-environment jsdom
// The list of what is new: every change under its page with the way there,
// what was seen dimmed, and the one button that marks the lot as seen or new.
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { I18nProvider, en } from "../../lib/i18n";
import { CHANGES, WHATS_NEW_RELEASE } from "../../lib/whatsNew";
import { beginWhatsNew, markAbsent, markSeen } from "../../lib/whatsNewStore";
import { WhatsNewCard } from "./WhatsNewCard";

const KEY = `bv-whats-new@${WHATS_NEW_RELEASE}`;
const stored = () => JSON.parse(localStorage.getItem(KEY)!) as { seen: string[]; absent: string[] };
const markAll = (n: number) => `${en["whatsNew.markAll"]} · ${n}`;
const showAll = `${en["whatsNew.showAll"]} · ${CHANGES.length}`;
const line = (id: string) => document.querySelector<HTMLElement>(`[data-change="${id}"]`)!;

function Where() {
  return <output data-testid="where">{useLocation().pathname}</output>;
}

async function renderCard() {
  await act(async () => {
    render(
      <I18nProvider>
        <MemoryRouter initialEntries={["/settings/info"]}>
          <WhatsNewCard />
          <Where />
        </MemoryRouter>
      </I18nProvider>
    );
  });
}

async function press(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

beforeEach(() => {
  localStorage.clear();
  localStorage.setItem("bv-lang", "en");
  beginWhatsNew("v9.9.1");
});
afterEach(cleanup);

describe("the list of what is new", () => {
  it("names every change once, under the page it is on", async () => {
    await renderCard();
    expect(document.querySelectorAll("[data-change]")).toHaveLength(CHANGES.length);
    const overview = screen.getByRole("heading", { level: 3, name: en["nav.overview"] }).parentElement!;
    expect(within(overview).getByText(en["whatsNew.overviewHead"])).toBeTruthy();
    expect(screen.getByRole("heading", { level: 3, name: `${en["nav.settings"]} › ${en["pairing.title"]}` })).toBeTruthy();
  });

  it("counts what is left on its button and dims what was seen", async () => {
    markSeen(["overview-head", "backups-bar"]);
    await renderCard();
    expect(screen.getByRole("button", { name: markAll(CHANGES.length - 2) })).toBeTruthy();
    expect(line("overview-head").dataset.seen).toBe("true");
    expect(line("overview-head").className).toContain("opacity-55");
    expect(line("backups-tiles").dataset.seen).toBeUndefined();
  });

  it("leads to the page of a change, and to the list for a page that belongs to one entry", async () => {
    await renderCard();
    await press(within(line("settings-week")).getByRole("button", { name: en["whatsNew.go"] }));
    expect(screen.getByTestId("where").textContent).toBe("/settings/schedules");
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
