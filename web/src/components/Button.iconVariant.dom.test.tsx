// @vitest-environment jsdom
// The "icon" variant is a shape, not a label mode. It becomes a square only
// when the mode prints no words, so a row of icon actions still follows the
// app-wide label setting like the buttons beside it.
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
    expect(screen.getByRole("button").textContent).toContain("Delete");
    cleanup();
  }
});

it("stays out of the square in reactive mode", () => {
  // Reactive mode grows as the words arrive on hover, which a fixed square
  // cannot do.
  setLabelMode("buttons", "reactive");
  renderIconAction();
  expect(classes()).not.toContain("glim-btn-icon");
});

it("leaves every other button alone", () => {
  setLabelMode("buttons", "glyph");
  render(<Button label="Back up" labelKey={null} glyph={<svg />} onClick={() => {}} />);
  expect(classes()).not.toContain("glim-btn-icon");
});
