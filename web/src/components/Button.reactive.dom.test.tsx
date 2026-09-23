// @vitest-environment jsdom
// Reactive mode shows only the glyph and reveals the label on hover. The
// animation is CSS; these tests pin what it needs from the markup: the label
// in the DOM in a collapsed box, and `glim-reactive` on the button for the
// hover rule to match.
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
  expect(label.className).toBe("glim-label-reactive");
  // Neither of the two treatments that cannot animate.
  expect(label.className).not.toContain("sr-only");
  expect(label.className).not.toContain("glim-btn-label");
});

it("marks the control so the reveal has something to key off", () => {
  setLabelMode("buttons", "reactive");
  renderButton();
  expect(screen.getByRole("button").className).toContain("glim-reactive");
});

it("marks nothing in the other three modes", () => {
  for (const mode of ["text", "textGlyph", "glyph"] as const) {
    cleanup();
    setLabelMode("buttons", mode);
    renderButton();
    expect(screen.getByRole("button").className).not.toContain("glim-reactive");
    expect(document.querySelector(".glim-label-reactive")).toBeNull();
  }
});

it("still has its accessible name, like every other mode", () => {
  setLabelMode("buttons", "reactive");
  renderButton();
  expect(screen.getByRole("button", { name: "Off-site-DR-Prüfung starten" })).toBeTruthy();
});

it("takes no width stage, so the button itself can grow as the words arrive", () => {
  setLabelMode("buttons", "reactive");
  renderButton();
  const el = screen.getByRole("button");
  expect([...el.classList].find((c) => /^glim-btn-(xs|sm|md|lg)$/.test(c))).toBeUndefined();
});

it("carries its label's length so the reveal is neither clipped nor sluggish", () => {
  setLabelMode("buttons", "reactive");
  renderButton("Off-site-DR-Prüfung starten");
  // The reveal's ceiling comes from the label length, 27 visual units here, so
  // a long label is not clipped and a short one does not snap open.
  const style = screen.getByRole("button").getAttribute("style") ?? "";
  expect(style).toContain("--reactive-chars: 27");
});

it("shows its text outright when it has no glyph to fall back on", () => {
  setLabelMode("buttons", "reactive");
  render(<Button label="Clear" onClick={() => {}} />);
  // As in glyph mode, a button with no glyph shows its text, not an empty box.
  expect(screen.getByText("Clear").className).toContain("glim-btn-label");
  expect(screen.getByRole("button").className).not.toContain("glim-reactive");
});

it("does not also put the label in a bubble, since hovering already reveals it", () => {
  setLabelMode("buttons", "reactive");
  renderButton();
  expect(screen.getByRole("button").getAttribute("title")).toBeNull();
  // A `title` prop still goes to the bubble; the label does not, because the
  // same hover paints it into the button.
  cleanup();
  render(
    <Button label="Clear" glyph={<svg />} title="Another backup is running" onClick={() => {}} />
  );
  expect(screen.getByRole("button").className).toContain("glim-reactive");
});
