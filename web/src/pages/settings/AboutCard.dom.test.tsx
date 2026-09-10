// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// The About card — the two things about it that are a STANDARD rather than a
// preference, and that a later edit could quietly undo.
//
//   1. The ORDER. GlimStone lays it down for every app in the family: what this
//      is, then the money with its own button, then the report with its own
//      buttons, then the versions as a footer. The point of it is the pairing —
//      each sentence sits directly above the thing it asks for — and the way it
//      breaks is somebody moving one line, which no type checker and no parity
//      test can see. Position in the document is therefore what is asserted,
//      not merely presence.
//
//   2. NO ROUTE THIS PRODUCT CANNOT REACH. BombVault has no address of its own,
//      so the card offers GitHub and says so, and nothing on it may invite a
//      mail. It has been wrong in that direction once already: the button
//      pointed at a different product's inbox, which is worse than no contact
//      route, because somebody writes and then waits.
// ---------------------------------------------------------------------------
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import { AboutCard } from "./AboutCard";

// The card asks for the running version on mount. A promise that never settles
// would leave the footer half-rendered, so this resolves with a real one.
vi.mock("../../lib/api", () => ({
  getHealth: () => Promise.resolve({ version: "8.3.1" }),
}));

afterEach(cleanup);

function renderCard() {
  return render(
    <I18nProvider>
      <AboutCard />
    </I18nProvider>
  );
}

const where = (el: Element) => Array.from(document.querySelectorAll("*")).indexOf(el);

/** Where a piece of text FIRST appears in the rendered document.
 *
 *  The first match rather than the only one: a button carries its label twice,
 *  once painted and once in the accessible tree for glyph mode, so insisting on
 *  a single match would be asserting something about the Button engine instead
 *  of about this card's order. */
function positionOf(text: string): number {
  return Math.min(...screen.getAllByText(text, { exact: false }).map(where));
}

/** Where a button sits. By ROLE, not by text: "GitHub" is also a word inside
 *  the sentence above it, so a text search finds the paragraph first and the
 *  order assertion compares a line against itself. */
function positionOfButton(name: string): number {
  return where(screen.getByRole("button", { name: new RegExp(name, "i") }));
}

it("keeps the house order: what it is, the coffee, the report, the versions", () => {
  renderCard();
  const body = positionOf(en["about.body"]);
  const coffee = positionOf(en["about.coffee"]);
  const coffeeButton = positionOfButton(en["about.coffeeButton"]);
  const report = positionOf(en["about.report"]);
  const repoButton = positionOfButton(en["about.repo"]);
  const versions = positionOf("GlimStone");

  expect(body).toBeLessThan(coffee);
  // Each sentence above the thing it asks for: the coffee button belongs to the
  // coffee sentence and has to sit between it and the next sentence, not in a
  // row of buttons at the end.
  expect(coffee).toBeLessThan(coffeeButton);
  expect(coffeeButton).toBeLessThan(report);
  expect(report).toBeLessThan(repoButton);
  expect(repoButton).toBeLessThan(versions);
});

it("gives the report sentence the extra line above it, and only that one", () => {
  renderCard();
  const report = screen.getByText(en["about.report"], { exact: false });
  const body = screen.getByText(en["about.body"], { exact: false });
  const coffee = screen.getByText(en["about.coffee"], { exact: false });

  // Without the break the coffee button sits as close to the sentence below it
  // as to the one it belongs to, and the eye pairs it with the wrong text.
  expect(report.className).toContain("mt-2");
  expect(body.className).not.toContain("mt-2");
  expect(coffee.className).not.toContain("mt-2");
});

it("puts both ways to give under the sentence that asks, and none of them elsewhere", () => {
  renderCard();
  // Two buttons because they reach different people: the coffee takes a card,
  // the crypto window takes what somebody already holds in a wallet. Both
  // belong to the SAME sentence, so both have to sit between it and the next
  // one — a second row further down would read as a second, unrelated offer.
  const coffee = positionOf(en["about.coffee"]);
  const report = positionOf(en["about.report"]);
  for (const key of ["about.coffeeButton", "about.crypto"] as const) {
    const button = positionOfButton(en[key]);
    expect(button, key).toBeGreaterThan(coffee);
    expect(button, key).toBeLessThan(report);
  }
});

it("opens the crypto window on the button, closed until then", () => {
  renderCard();
  // The window is not merely hidden while it is shut: an address list that is
  // in the document from the start is one CSS mistake away from being read by
  // somebody who never asked for it, and one selector away from being copied.
  expect(screen.queryByRole("dialog")).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: new RegExp(en["about.crypto"], "i") }));
  const dialog = screen.getByRole("dialog");
  expect(dialog.textContent).toContain(en["about.cryptoTitle"]);
});

it("names no route without a control, and offers no control the sentence does not name", () => {
  renderCard();
  const sentence = en["about.report"];
  // The rule rather than today's answer: whichever routes this product has,
  // the sentence and the buttons under it have to agree. It has been wrong in
  // both directions already — a mail button pointing at another product's
  // inbox, and later a sentence that would have promised a mailbox that did
  // not exist yet. Either way somebody writes and then waits.
  const has = (label: string) =>
    Boolean(screen.queryByRole("button", { name: new RegExp(label, "i") }));

  expect(has(en["about.mail"])).toBe(/e-?mail/i.test(sentence));
  expect(has(en["about.repo"])).toBe(/github/i.test(sentence));
});
