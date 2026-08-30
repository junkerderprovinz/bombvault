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

// Tone -> classes. ConfirmDialog used to own this table and asserted the class
// names itself; it passes a tone NAME now, so the mapping is pinned here
// instead of quietly losing the coverage in the move.
//
// `danger` and `warn` deliberately use the SOLID status tokens over
// `carbon-background`: both themes' solid fail/warn values sit at the opposite
// lightness to that theme's own background, so one ink stays legible in both
// without a dedicated contrast token.
it("resolves each tone to its own fill", () => {
  setLabelMode("buttons", "text");
  for (const [tone, expected] of [
    ["accent", "bg-accent"],
    ["neutral", "bg-carbon-surface3"],
    ["danger", "bg-statusFailSolid"],
    ["warn", "bg-statusWarnSolid"],
  ] as const) {
    cleanup();
    render(<Button label="Delete" tone={tone} onClick={() => {}} />);
    expect(screen.getByRole("button").className).toContain(expected);
  }
});

it("keeps the destructive and the warning fills distinct", () => {
  setLabelMode("buttons", "text");
  render(<Button label="Delete" tone="warn" onClick={() => {}} />);
  expect(screen.getByRole("button").className).not.toContain("bg-statusFailSolid");
});

// The chip variant (the remove control inside a pill). Its whole reason to
// exist is that it must NOT take a width stage - it sits inside a 0.75rem
// pill, and a stage would burst it - while still carrying a real accessible
// name, which is what four of these lost when they were first converted to a
// bare "x".
it("a chip takes no width stage and never shows its text", () => {
  for (const mode of ["text", "textGlyph", "glyph"] as const) {
    cleanup();
    setLabelMode("buttons", mode);
    render(<Button label="Remove plex" variant="chip" onClick={() => {}} />);
    const el = screen.getByRole("button");
    expect(el.className).toContain("bv-btn-chip");
    for (const stage of ["bv-btn-xs", "bv-btn-sm", "bv-btn-md", "bv-btn-lg"]) {
      expect(el.className).not.toContain(stage);
    }
    // Announced and on hover, never painted next to the thing it removes.
    expect(screen.getByText("Remove plex").className).toBe("sr-only");
    expect(el.getAttribute("title")).toContain("Remove plex");
  }
});

it("a chip carries the pill's own ink rather than painting a surface", () => {
  setLabelMode("buttons", "textGlyph");
  render(<Button label="Remove plex" variant="chip" onClick={() => {}} />);
  const cls = screen.getByRole("button").className;
  for (const fill of ["bg-carbon-surface3", "bg-accent", "bg-statusFailSolid"]) {
    expect(cls).not.toContain(fill);
  }
});

it("a chip still gets a glyph when the call site passes none", () => {
  setLabelMode("buttons", "text");
  render(<Button label="Remove plex" variant="chip" onClick={() => {}} />);
  expect(screen.getByRole("button").querySelector("svg")).toBeTruthy();
});
