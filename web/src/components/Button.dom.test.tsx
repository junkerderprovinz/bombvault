// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// Button (#178) — the two promises the engine makes.
//
//   1. The width does not move between modes. jdp: "Alle buttons bleiben in
//      allen drei modi auch gleich breit." That is why the stage is derived
//      from the label rather than from what is painted.
//   2. A button always has an accessible name. Glyph mode is where an engine
//      like this normally introduces 197 unlabelled controls at once, so the
//      label is hidden visually and kept in the accessible tree, never removed.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { Button } from "./Button";
import { setLabelMode } from "../lib/controls";

function renderButton(label = "Clear", withGlyph = true) {
  return render(
    <Button label={label} glyph={withGlyph ? <svg data-testid="g" /> : undefined} onClick={() => {}} />
  );
}

function stageClass(): string {
  const el = screen.getByRole("button");
  return [...el.classList].find((c) => /^bv-btn-(xs|sm|md|lg)$/.test(c)) ?? "";
}

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  cleanup();
  localStorage.clear();
});

it("keeps the same width stage in all three modes", () => {
  // A label long enough to sit ABOVE the smallest stage, deliberately: with a
  // short one ("Clear"), a broken implementation that sized from the visible
  // text would still land on "xs" in every mode and this test would pass while
  // proving nothing. It has to be able to tell the two apart.
  const long = "Off-site-DR-Prüfung starten";
  const stages: string[] = [];
  for (const mode of ["text", "textGlyph", "glyph"] as const) {
    setLabelMode("buttons", mode);
    renderButton(long);
    stages.push(stageClass());
    cleanup();
  }
  // One distinct stage across all three: the mode changed what is SHOWN, never
  // how wide the control is.
  expect(new Set(stages).size).toBe(1);
  expect(stages[0]).toBe("bv-btn-lg");
});

it("still has an accessible name in glyph mode", () => {
  setLabelMode("buttons", "glyph");
  renderButton();
  // Found BY ITS NAME, which is the whole point: the text is invisible but a
  // screen reader still announces it.
  expect(screen.getByRole("button", { name: "Clear" })).toBeTruthy();
});

it("turns the label into the tooltip when the text is hidden", () => {
  setLabelMode("buttons", "glyph");
  renderButton();
  expect(screen.getByRole("button").getAttribute("title")).toBe("Clear");
});

it("does not repeat the label as a tooltip while the text is visible", () => {
  setLabelMode("buttons", "textGlyph");
  renderButton();
  expect(screen.getByRole("button").getAttribute("title")).toBeNull();
});

it("shows a button's text in glyph mode when it has no glyph yet", () => {
  setLabelMode("buttons", "glyph");
  renderButton("Clear", false);
  // 146 of the app's buttons have no glyph yet. Until they do, a blank square
  // is the worse failure, so those fall back to their text.
  expect(screen.getByText("Clear").className).toContain("bv-btn-label");
});

it("gives a longer label a wider stage", () => {
  setLabelMode("buttons", "text");
  renderButton("Kijelölés törlése");
  expect(stageClass()).toBe("bv-btn-md");
});
