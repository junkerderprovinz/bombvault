// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// The fourth label mode (jdp: "Ich möchte einen vierten modus implementieren
// 'reaktiver Text und Symbole' wo alles buttons etc. nur den Glyph zeigen aber
// bei mouseover der text mit eingeblendet wird durch eine schöne animation").
//
// What is worth pinning here is not the animation — that is CSS, verified in a
// browser — but the three things the animation depends on and that a later
// change could quietly break:
//
//   1. The words are REALLY THERE at rest, in a collapsed box. Not `sr-only`,
//      which cannot be revealed, and not absent, which cannot be revealed
//      either. A reveal animation needs something to reveal.
//   2. The control is marked for the CSS to key off. Without `bv-reactive` on
//      the button, the hover rule has nothing to match and the label stays
//      collapsed forever — a mode that silently does nothing.
//   3. The width does not move. This is the reason the mode can exist at all:
//      the stage already reserves the label's width, so the words arrive
//      inside a box that was always that size.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { Button } from "./Button";
import { setLabelMode } from "../lib/controls";

function renderButton(label = "Off-site-DR-Prüfung starten") {
  return render(<Button label={label} glyph={<svg data-testid="g" />} onClick={() => {}} />);
}

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  cleanup();
  localStorage.clear();
});

it("keeps the words in the DOM, collapsed rather than hidden or dropped", () => {
  setLabelMode("buttons", "reactive");
  renderButton();
  const label = screen.getByText("Off-site-DR-Prüfung starten");
  expect(label.className).toBe("bv-label-reactive");
  // Neither of the two treatments that cannot animate.
  expect(label.className).not.toContain("sr-only");
  expect(label.className).not.toContain("bv-btn-label");
});

it("marks the control so the reveal has something to key off", () => {
  setLabelMode("buttons", "reactive");
  renderButton();
  expect(screen.getByRole("button").className).toContain("bv-reactive");
});

it("marks nothing in the other three modes", () => {
  for (const mode of ["text", "textGlyph", "glyph"] as const) {
    cleanup();
    setLabelMode("buttons", mode);
    renderButton();
    expect(screen.getByRole("button").className).not.toContain("bv-reactive");
    expect(document.querySelector(".bv-label-reactive")).toBeNull();
  }
});

it("still has its accessible name, like every other mode", () => {
  setLabelMode("buttons", "reactive");
  renderButton();
  expect(screen.getByRole("button", { name: "Off-site-DR-Prüfung starten" })).toBeTruthy();
});

// Originally this pinned "the same stage in all four modes", which was true
// when the mode shipped. jdp then asked for the opposite in the two hiding
// modes: narrow at rest, growing on hover. So the reveal is no longer free of
// layout — it IS the layout, and what has to hold instead is that the button
// takes no floor that would stop it from growing.
it("takes no width stage, so the button itself can grow as the words arrive", () => {
  setLabelMode("buttons", "reactive");
  renderButton();
  const el = screen.getByRole("button");
  expect([...el.classList].find((c) => /^bv-btn-(xs|sm|md|lg)$/.test(c))).toBeUndefined();
});

it("carries its label's length so the reveal is neither clipped nor sluggish", () => {
  setLabelMode("buttons", "reactive");
  renderButton("Off-site-DR-Prüfung starten");
  // A fixed ceiling cut this label off on hover and made short ones snap open;
  // the ceiling is derived from the text instead. 27 visual units here.
  const style = screen.getByRole("button").getAttribute("style") ?? "";
  expect(style).toContain("--reactive-chars: 27");
});

it("shows its text outright when it has no glyph to fall back on", () => {
  setLabelMode("buttons", "reactive");
  render(<Button label="Clear" onClick={() => {}} />);
  // Same rule as glyph mode: an empty box is unusable, and a reactive empty box
  // is an empty box you have to find with the pointer first.
  expect(screen.getByText("Clear").className).toContain("bv-btn-label");
  expect(screen.getByRole("button").className).not.toContain("bv-reactive");
});

it("does not also put the label in a bubble — hovering already reveals it", () => {
  setLabelMode("buttons", "reactive");
  renderButton();
  expect(screen.getByRole("button").getAttribute("title")).toBeNull();
  // A `title` prop is the changing half and still belongs in the bubble; the
  // label does not, because the same hover paints it into the button.
  cleanup();
  render(
    <Button label="Clear" glyph={<svg />} title="Another backup is running" onClick={() => {}} />
  );
  expect(screen.getByRole("button").className).toContain("bv-reactive");
});
