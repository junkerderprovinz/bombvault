// @vitest-environment jsdom
/**
 * The accent row while rainbow mode owns the colours (jdp, 2026-09-08: "wenn
 * man den regenbogen modus aktiviert, sollen die akzentfarben abgedunkelt und
 * 'deaktiviert' werden").
 *
 * Three things have to hold together, and the third is the one that makes the
 * other two honest: the row is dimmed, it cannot be operated, and it SAYS why.
 * A dimmed row with no sentence is indistinguishable from a broken row, and the
 * switch that caused it sits below rather than above.
 *
 * Not asserted here, deliberately: that the accent has no effect anywhere.
 * It still does — `[data-rainbow] .glim-hue` overrides --accent only on
 * elements carrying a palette position, and a handful never got one. That gap
 * is a pass of its own; this file pins the control's behaviour, not a claim
 * about the whole stylesheet.
 */
import { render, screen, cleanup, fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { AccentCard } from "./AccentCard";
import { getAccent, setAccent, DEFAULT_ACCENT } from "../../lib/accent";

const t = ((key: string) => key) as unknown as Parameters<typeof AccentCard>[0]["t"];

function draw(rainbowOn: boolean) {
  return render(<AccentCard t={t} rainbowOn={rainbowOn} />);
}

beforeEach(() => localStorage.clear());
afterEach(cleanup);

describe("AccentCard under rainbow mode", () => {
  it("says nothing extra while rainbow is off", () => {
    draw(false);
    expect(screen.queryByText("settings.accentRainbowHint")).toBeNull();
  });

  it("explains itself while rainbow is on", () => {
    draw(true);
    expect(screen.getByText("settings.accentRainbowHint")).toBeTruthy();
  });

  it("dims the row rather than removing it", () => {
    const { container } = draw(true);
    const row = container.querySelector(".opacity-45");
    expect(row).not.toBeNull();
    // Still there: the reader must see that an accent colour exists and is
    // currently not the one in charge.
    expect(screen.getByText("settings.accentColor")).toBeTruthy();
  });

  it("makes the swatches inert, so a click cannot change the stored accent", () => {
    const { container } = draw(true);
    const swatches = container.querySelectorAll('[aria-disabled="true"]');
    expect(swatches.length).toBeGreaterThan(0);
    const before = getAccent();
    fireEvent.click(swatches[swatches.length - 1]);
    expect(getAccent()).toBe(before);
    expect(before.toLowerCase()).toBe(DEFAULT_ACCENT.toLowerCase());
  });

  it("leaves the swatches operable while rainbow is off", () => {
    const { container } = draw(false);
    expect(container.querySelectorAll('[aria-disabled="true"]').length).toBe(0);
  });

  it("disables the reset badge too, even when there IS something to reset", () => {
    // The seed matters, and without it this test is blind: on a fresh store the
    // accent is already the default, so `nothingToReset` disables the badge on
    // its own and the assertion would pass whatever rainbowOn does. Drifting
    // the accent first is what makes rainbow mode the only reason it is off.
    setAccent("#00ffcc");
    expect(getAccent().toLowerCase()).not.toBe(DEFAULT_ACCENT.toLowerCase());

    const { unmount } = draw(false);
    // Proof the seed reached the control: with rainbow off it is live.
    expect(screen.getByRole("button", { name: /accentReset/i })).toHaveProperty("disabled", false);
    unmount();

    draw(true);
    expect(screen.getByRole("button", { name: /accentReset/i })).toHaveProperty("disabled", true);
  });
});
