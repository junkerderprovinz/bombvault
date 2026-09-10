// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// The "icon" variant (GlimStone 1.8.0) — a SHAPE, not an answer to the mode.
//
// This is the whole content of the rule and the only thing worth testing about
// it. The variant's first version resolved to a glyph in every mode, the way a
// chip does, and it was rejected in the same words as the defect it was meant
// to fix: a row of five controls printed no word while the buttons beside them
// printed theirs, with the app-wide setting on text-plus-glyph. From outside, a
// documented exemption and a control that ignores the setting look identical.
//
// So: the square arrives exactly when there are no words to print, and never
// otherwise.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { Button } from "./Button";
import { setLabelMode } from "../lib/controls";

function renderIconAction() {
  return render(
    <Button label="Delete" labelKey={null} variant="icon" glyph={<svg data-testid="g" />} onClick={() => {}} />,
  );
}

function classes(): string[] {
  return [...screen.getByRole("button").classList];
}

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  cleanup();
  localStorage.clear();
});

it("is a square only while the mode paints no words", () => {
  setLabelMode("buttons", "glyph");
  renderIconAction();
  expect(classes()).toContain("glim-btn-icon");
});

it("is an ordinary labelled button in the modes that show words", () => {
  for (const mode of ["text", "textGlyph"] as const) {
    setLabelMode("buttons", mode);
    renderIconAction();
    // No square, and the width stage every other button of this label takes.
    expect(classes()).not.toContain("glim-btn-icon");
    expect(classes().some((c) => /^glim-btn-(xs|sm|md|lg)$/.test(c))).toBe(true);
    // The words are painted, which is the half the first version got wrong.
    expect(screen.getByRole("button").textContent).toContain("Delete");
    cleanup();
  }
});

it("stays out of the square in reactive mode", () => {
  // Reactive grows as the words arrive on hover, and a fixed width is the one
  // thing that cannot do.
  setLabelMode("buttons", "reactive");
  renderIconAction();
  expect(classes()).not.toContain("glim-btn-icon");
});

it("leaves every other button alone", () => {
  setLabelMode("buttons", "glyph");
  render(<Button label="Back up" labelKey={null} glyph={<svg />} onClick={() => {}} />);
  expect(classes()).not.toContain("glim-btn-icon");
});
