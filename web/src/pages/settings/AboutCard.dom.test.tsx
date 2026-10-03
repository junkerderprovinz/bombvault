// @vitest-environment jsdom
// The About card's order is a GlimStone standard shared by every app: each
// sentence sits directly above the button it asks for. Moving one line breaks
// that without failing a type check, so these tests assert position in the
// document rather than presence. The card must also never name a contact route
// it has no button for.
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { afterEach, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider, en } from "../../lib/i18n";
import { AboutCard } from "./AboutCard";

// The card fetches the running version on mount; without it the footer is
// only half rendered.
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

/** Position of the first element containing text. A button renders its label
 *  twice (painted and in the accessible tree), so the first match counts. */
function positionOf(text: string): number {
  return Math.min(...screen.getAllByText(text, { exact: false }).map(where));
}

/** Found by role because "GitHub" also appears in the sentence above the
 *  button. */
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
  // The coffee button belongs to the coffee sentence, not to a row of buttons
  // at the end.
  expect(coffee).toBeLessThan(coffeeButton);
  expect(coffeeButton).toBeLessThan(report);
  expect(report).toBeLessThan(repoButton);
  expect(repoButton).toBeLessThan(versions);
});

it("runs the give buttons hosted-page first, wallet last", () => {
  // GlimStone fixes this order for every app: the routes most people already
  // have an account for first, then the one that needs none.
  renderCard();
  const coffee = positionOfButton(en["about.coffeeButton"]);
  const paypal = positionOfButton(en["about.paypal"]);
  const crypto = positionOfButton(en["about.crypto"]);

  expect(coffee).toBeLessThan(paypal);
  expect(paypal).toBeLessThan(crypto);
});

it("stands the give row a blank line apart from the sentences around it", () => {
  renderCard();
  const row = screen.getByRole("button", { name: new RegExp(en["about.coffeeButton"], "i") }).closest(
    ".glim-readme-btn-rows"
  );
  // Without the space the buttons sit as close to the report sentence below
  // as to the sentence they answer, and the eye pairs them with the wrong one.
  expect(row?.className).toContain("glim-about-give");
  for (const key of ["about.body", "about.coffee", "about.report"] as const) {
    expect(screen.getByText(en[key], { exact: false }).className, key).not.toMatch(/\bm[ty]-/);
  }
});

it("gives and reports with README buttons that light up in their brand's colour", () => {
  renderCard();
  const tiles: [string, string][] = [
    [en["about.coffeeButton"], "glim-tile-coffee"],
    [en["about.paypal"], "glim-tile-paypal"],
    [en["about.crypto"], "glim-tile-bitcoin"],
    [en["about.repo"], "glim-tile-github"],
    [en["about.mail"], "glim-tile-house"],
  ];
  for (const [name, tile] of tiles) {
    const button = screen.getByRole("button", { name: new RegExp(name, "i") });
    expect(button.className, name).toContain("glim-brand-tile");
    expect(button.closest(".glim-readme-btn-unit")?.className, name).toContain(tile);
  }
});

it("wears Buy Me a Coffee's own artwork and an envelope that opens under the pointer", () => {
  renderCard();
  const coffee = screen.getByRole("button", { name: new RegExp(en["about.coffeeButton"], "i") });
  expect(coffee.querySelector(".glim-readme-btn-art svg")).not.toBeNull();

  const mail = screen.getByRole("button", { name: new RegExp(en["about.mail"], "i") });
  const rest = mail.querySelector("svg.glim-mark-rest");
  const open = mail.querySelector("svg.glim-mark-hover");
  // Each envelope sits in the box cropped to its own ink, so the two read the
  // same size.
  const glyphs: { name: string; viewBox: string }[] = JSON.parse(
    readFileSync(join(__dirname, "..", "..", "..", "..", "scripts", "glyphs.json"), "utf8"),
  );
  const box = (name: string) => glyphs.find((g) => g.name === name)?.viewBox;
  expect(rest?.getAttribute("viewBox")).toBe(box("IconMail"));
  expect(open?.getAttribute("viewBox")).toBe(box("IconMailOpen"));
});

it("puts every way to give under the sentence that asks, and none of them elsewhere", () => {
  renderCard();
  // All answer the same sentence; a second row further down would read as a
  // separate offer.
  const coffee = positionOf(en["about.coffee"]);
  const report = positionOf(en["about.report"]);
  for (const key of ["about.coffeeButton", "about.paypal", "about.crypto"] as const) {
    const button = positionOfButton(en[key]);
    expect(button, key).toBeGreaterThan(coffee);
    expect(button, key).toBeLessThan(report);
  }
});

it("opens the crypto window on the button, closed until then", () => {
  renderCard();
  // Absent rather than hidden: an address list in the document from the start
  // is one CSS mistake away from being shown to somebody who never asked.
  expect(screen.queryByRole("dialog")).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: new RegExp(en["about.crypto"], "i") }));
  const dialog = screen.getByRole("dialog");
  expect(dialog.textContent).toContain(en["about.cryptoTitle"]);
});

it("loads nothing from BMAC or PayPal until their windows open", () => {
  renderCard();
  // The app promises no telemetry, so a payment provider is not contacted
  // just because somebody looked at Settings.
  const external = () => ({
    frames: document.querySelectorAll("iframe").length,
    scripts: [...document.querySelectorAll("script")].filter((s) => s.src.includes("paypal.com")).length,
  });
  expect(external()).toEqual({ frames: 0, scripts: 0 });

  fireEvent.click(screen.getByRole("button", { name: new RegExp(en["about.coffeeButton"], "i") }));
  expect(screen.getByRole("dialog").textContent).toContain(en["about.coffeeIntro"]);
  expect(external()).toEqual({ frames: 1, scripts: 0 });
  fireEvent.click(screen.getByRole("button", { name: new RegExp(en["common.close"], "i") }));
  expect(screen.queryByRole("dialog")).toBeNull();

  fireEvent.click(screen.getByRole("button", { name: new RegExp(en["about.paypal"], "i") }));
  expect(screen.getByRole("dialog").textContent).toContain(en["about.paypalIntro"]);
  expect(external()).toEqual({ frames: 0, scripts: 1 });
});

it("names no route without a control, and offers no control the sentence does not name", () => {
  renderCard();
  const sentence = en["about.report"];
  // Whichever routes the product has, the sentence and the buttons under it
  // have to agree, or somebody writes and then waits.
  const has = (label: string) =>
    Boolean(screen.queryByRole("button", { name: new RegExp(label, "i") }));

  expect(has(en["about.mail"])).toBe(/e-?mail/i.test(sentence));
  expect(has(en["about.repo"])).toBe(/github/i.test(sentence));
});
