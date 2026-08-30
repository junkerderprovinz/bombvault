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

it("takes the same width stage as the other three, which is what makes the reveal free", () => {
  const stages: string[] = [];
  for (const mode of ["text", "textGlyph", "glyph", "reactive"] as const) {
    cleanup();
    setLabelMode("buttons", mode);
    renderButton();
    const el = screen.getByRole("button");
    stages.push([...el.classList].find((c) => /^bv-btn-(xs|sm|md|lg)$/.test(c)) ?? "");
  }
  expect(new Set(stages).size).toBe(1);
  expect(stages[0]).toBe("bv-btn-lg");
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
