// @vitest-environment jsdom
// The accent row and the rainbow palette row each keep one footprint: every
// disc and the reset badge are 32px with no rim, and the chosen preset carries
// a ring that takes no room, so the reset badge never looks bigger than the
// colours beside it.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AccentCard } from "./AccentCard";
import { GeneralTab } from "./tabs/GeneralTab";
import type { SettingsTabProps } from "./tabs/types";
import { DEFAULT_ACCENT_PRESETS } from "../../lib/accent";
import { RAINBOW } from "../../lib/appearance";
import { I18nProvider } from "../../lib/i18n";

vi.mock("../../lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../lib/api")>()),
  getHealth: async () => ({ ok: true, version: "v9.0.0" }),
}));

const t = ((key: string) => key) as unknown as Parameters<typeof AccentCard>[0]["t"];

function discs(container: HTMLElement): HTMLButtonElement[] {
  return [...container.querySelectorAll<HTMLButtonElement>('button[aria-haspopup="dialog"]')];
}

/** A 32px square without a rim: w-8 h-8, or h-8 at a 1:1 aspect ratio. */
function expectSwatchBox(el: Element) {
  const classes = el.className.split(/\s+/);
  expect(classes, "32px high").toContain("h-8");
  expect(classes.includes("w-8") || classes.includes("aspect-square"), `32px wide: ${el.className}`).toBe(true);
  expect(classes.filter((c) => /^(?:border|ring)(?:-\d+)?$/.test(c)), "a rim").toEqual([]);
}

beforeEach(() => localStorage.clear());
afterEach(cleanup);

describe("the accent row", () => {
  it("gives every preset and the reset badge the same 32px box without a rim", () => {
    const { container } = render(<AccentCard t={t} />);
    const presets = discs(container);
    expect(presets).toHaveLength(DEFAULT_ACCENT_PRESETS.length);
    for (const disc of presets) {
      expectSwatchBox(disc);
      expect(disc.parentElement!.className).not.toMatch(/(?:^|\s)(?:border|ring)(?:-\d+)?(?=\s|$)/);
    }
    expectSwatchBox(screen.getByRole("button", { name: "settings.accentReset" }));
  });

  it("rings the chosen preset and no other", () => {
    const { container } = render(<AccentCard t={t} />);
    const ringed = discs(container).filter((d) => /\boutline-2\b/.test(d.parentElement!.className));
    expect(ringed.map((d) => d.getAttribute("aria-label"))).toEqual(["settings.accentPreset 1"]);
  });

  it("draws the chosen ring outside the focus ring, so a focused choice keeps both", () => {
    // The chosen ring sits on the wrapper, which is exactly the disc's box, so
    // it has to start where the disc's focus ring ends.
    const css = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "../../index.css"), "utf8");
    const focus = /(?:^|\n):focus-visible\s*\{([^}]*)\}/.exec(css)![1];
    const focusEnd = Number(/outline-offset:\s*(\d+)px/.exec(focus)![1]) + Number(/outline:\s*(\d+)px/.exec(focus)![1]);

    const { container } = render(<AccentCard t={t} />);
    const chosen = discs(container).find((d) => /\boutline-2\b/.test(d.parentElement!.className))!.parentElement!;
    const offset = Number(/\boutline-offset-(\d+)\b/.exec(chosen.className)?.[1]);
    expect(offset).toBeGreaterThanOrEqual(focusEnd);
  });
});

describe("the rainbow palette row", () => {
  const noop = () => {};
  const props = {
    t,
    quiet: false,
    setQuiet: noop,
    settings: {},
    shape: "round",
    setShapeLocal: noop,
    motion: "subtle",
    setMotionLocal: noop,
    stormFound: false,
    setStormFound: noop,
    stormClicks: { current: { taps: 0 } },
    discoFound: false,
    disco: false,
    setDiscoLocal: noop,
    labelModes: { buttons: "textGlyph", sidebar: "textGlyph", tabs: "textGlyph" },
    setLabelModes: noop,
    rainbow: { on: true, reactive: false, rotate: false, seed: 0, palette: RAINBOW },
    updateRainbow: noop,
    rainbowToggled: noop,
    domainToggleBusy: {},
    domainToggleShake: {},
    fieldPulse: {},
    toggleDomainEnabled: async () => {},
  } as unknown as SettingsTabProps;

  // jsdom has no matchMedia, and the theme card asks it for the system theme.
  beforeEach(() => {
    vi.stubGlobal("matchMedia", (media: string) => ({
      matches: false,
      media,
      addEventListener: noop,
      removeEventListener: noop,
    }));
  });
  afterEach(() => vi.unstubAllGlobals());

  it("gives every colour and the reset badge the same 32px box without a rim", async () => {
    await act(async () => {
      render(
        <I18nProvider>
          <GeneralTab {...props} />
        </I18nProvider>,
      );
    });
    for (let i = 1; i <= RAINBOW.length; i++) {
      expectSwatchBox(screen.getByRole("button", { name: `settings.rainbowPalette ${i}` }));
    }
    expectSwatchBox(screen.getByRole("button", { name: "settings.rainbowPaletteReset" }));
  });
});
