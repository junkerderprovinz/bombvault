// @vitest-environment jsdom
// While rainbow mode owns the colours, the accent row is dimmed, cannot be
// operated, and says why. A dimmed row without a reason looks broken, and the
// switch that caused it sits further down the page.
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
    expect(screen.queryByLabelText("settings.accentRainbowHint")).toBeNull();
  });

  it("explains itself while rainbow is on, in a bubble rather than a line", () => {
    const { container } = draw(true);
    expect(screen.getByLabelText("settings.accentRainbowHint")).toBeTruthy();
    // Without this the test would pass for a card that renders both.
    expect(container.textContent).not.toContain("settings.accentRainbowHint");
  });

  it("keeps the explanation readable while the row it explains is dimmed", () => {
    // Opacity applies to a whole subtree, so this holds only while the dimming
    // sits on the label and the swatch group rather than on the row that also
    // carries the bubble.
    draw(true);
    const bubble = screen.getByLabelText("settings.accentRainbowHint");
    expect(bubble.closest(".opacity-45")).toBeNull();
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

  it("disables the reset badge even when there is something to reset", () => {
    // On a fresh store the badge is already disabled because there is nothing
    // to reset, and the assertion would pass whatever rainbowOn does.
    setAccent("#00ffcc");
    expect(getAccent().toLowerCase()).not.toBe(DEFAULT_ACCENT.toLowerCase());

    const { unmount } = draw(false);
    expect(screen.getByRole("button", { name: /accentReset/i })).toHaveProperty("disabled", false);
    unmount();

    draw(true);
    expect(screen.getByRole("button", { name: /accentReset/i })).toHaveProperty("disabled", true);
  });
});
