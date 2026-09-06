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
import { cleanup, render, screen } from "@testing-library/react";
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

it("names no route this product cannot reach", () => {
  const { container } = renderCard();
  expect(container.querySelector('a[href^="mailto:"]')).toBeNull();
  expect(container.textContent ?? "").not.toMatch(/e-?mail/i);
});
