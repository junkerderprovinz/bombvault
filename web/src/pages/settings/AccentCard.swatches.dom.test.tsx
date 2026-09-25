// @vitest-environment jsdom
// The accent row keeps one footprint: every preset disc and the reset badge are
// 32px with no rim, and the chosen preset carries a ring that takes no room, so
// the reset badge never looks bigger than the colours beside it.
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { AccentCard } from "./AccentCard";
import { DEFAULT_ACCENT_PRESETS } from "../../lib/accent";

const t = ((key: string) => key) as unknown as Parameters<typeof AccentCard>[0]["t"];

function discs(container: HTMLElement): HTMLButtonElement[] {
  return [...container.querySelectorAll<HTMLButtonElement>('button[aria-haspopup="dialog"]')];
}

beforeEach(() => localStorage.clear());
afterEach(cleanup);

describe("the accent row", () => {
  it("gives every preset and the reset badge the same 32px box without a rim", () => {
    const { container } = render(<AccentCard t={t} />);
    const presets = discs(container);
    expect(presets).toHaveLength(DEFAULT_ACCENT_PRESETS.length);
    for (const disc of presets) {
      expect(disc.className).toMatch(/\bw-8\b/);
      expect(disc.className).toMatch(/\bh-8\b/);
      expect(disc.parentElement!.className).not.toMatch(/\bborder-2\b/);
    }
    expect(screen.getByRole("button", { name: "settings.accentReset" }).className).not.toMatch(/\bborder-2\b/);
  });

  it("rings the chosen preset and no other", () => {
    const { container } = render(<AccentCard t={t} />);
    const ringed = discs(container).filter((d) => /\boutline-2\b/.test(d.parentElement!.className));
    expect(ringed.map((d) => d.getAttribute("aria-label"))).toEqual(["settings.accentPreset 1"]);
  });
});
