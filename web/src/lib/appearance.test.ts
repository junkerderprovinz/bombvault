// The pure half of appearance.ts: rainbowColorAt's position and rotation
// math, hueVars and isValidPalette. The DOM half is in appearance.dom.test.tsx.
import { describe, expect, it } from "vitest";
import { RAINBOW, hueVars, isValidPalette, rainbowColorAt } from "./appearance";

describe("RAINBOW", () => {
  it("is a fixed set of eight valid hex colours", () => {
    expect(RAINBOW).toHaveLength(8);
    for (const hex of RAINBOW) {
      expect(hex).toMatch(/^#[0-9a-fA-F]{6}$/);
    }
  });

  it("includes the default accent so one row always matches it", () => {
    expect(RAINBOW).toContain("#FCC419");
  });
});

describe("rainbowColorAt", () => {
  it("hands out colours by list position when rotation is off", () => {
    for (let i = 0; i < RAINBOW.length; i++) {
      expect(rainbowColorAt(i, RAINBOW, false, /* seed */ 3)).toBe(RAINBOW[i]);
    }
  });

  it("ignores seed entirely when rotate is false, even with a nonzero seed", () => {
    expect(rainbowColorAt(0, RAINBOW, false, 5)).toBe(RAINBOW[0]);
    expect(rainbowColorAt(4, RAINBOW, false, 5)).toBe(RAINBOW[4]);
  });

  it("offsets the starting colour by seed when rotate is true", () => {
    // seed=1 gives position 0 the colour at index 1.
    expect(rainbowColorAt(0, RAINBOW, true, 1)).toBe(RAINBOW[1]);
    expect(rainbowColorAt(0, RAINBOW, true, 3)).toBe(RAINBOW[3]);
  });

  it("wraps the rotation offset around the end of the palette", () => {
    const last = RAINBOW.length - 1;
    // Position `last` + seed 1 should wrap back to index 0.
    expect(rainbowColorAt(last, RAINBOW, true, 1)).toBe(RAINBOW[0]);
  });

  it("wraps a position past the palette length back to the start", () => {
    expect(rainbowColorAt(RAINBOW.length, RAINBOW, false, 0)).toBe(RAINBOW[0]);
    expect(rainbowColorAt(RAINBOW.length + 2, RAINBOW, false, 0)).toBe(RAINBOW[2]);
  });

  it("wraps a negative position into range rather than returning undefined", () => {
    expect(rainbowColorAt(-1, RAINBOW, false, 0)).toBe(RAINBOW[RAINBOW.length - 1]);
    expect(rainbowColorAt(-RAINBOW.length, RAINBOW, false, 0)).toBe(RAINBOW[0]);
  });

  it("truncates a non-integer position rather than throwing", () => {
    expect(rainbowColorAt(2.9, RAINBOW, false, 0)).toBe(RAINBOW[2]);
  });

  it("falls back to the built-in RAINBOW palette if given an empty one", () => {
    expect(rainbowColorAt(0, [], false, 0)).toBe(RAINBOW[0]);
  });

  it("works with a custom (edited) palette, not just the built-in one", () => {
    const custom = ["#111111", "#222222", "#333333", "#444444", "#555555", "#666666", "#777777", "#888888"];
    expect(rainbowColorAt(2, custom, false, 0)).toBe("#333333");
    expect(rainbowColorAt(0, custom, true, 2)).toBe("#333333");
  });
});

describe("hueVars", () => {
  // No colour is baked into the element: a palette or rotation change, and
  // disco's glide, reach it only through the root.
  it("points every property at the root's palette position", () => {
    const vars = hueVars(5);
    expect(vars["--item-hue"]).toBe("var(--rb-5)");
    expect(vars["--item-hue-ink"]).toBe("var(--rb-ink-5)");
    expect(vars["--item-hue-soft"]).toBe("color-mix(in srgb, var(--rb-5) 14%, transparent)");
    expect(vars["--item-hue-wash"]).toBe("color-mix(in srgb, var(--rb-5) 7%, transparent)");
    expect(vars["--item-hue-ring"]).toBe("color-mix(in srgb, var(--rb-5) 55%, transparent)");
  });

  it("wraps a position past either end of the palette, as rainbowColorAt does", () => {
    expect(hueVars(RAINBOW.length + 2)).toEqual(hueVars(2));
    expect(hueVars(-1)).toEqual(hueVars(RAINBOW.length - 1));
  });

  // Nothing shades a hued fill away from its own ink. Code that starts to would
  // need contrast work an inverse-ink token skips, and failing here is cheaper
  // than finding that on screen.
  it("hands out no inverse-ink token", () => {
    for (let i = 0; i < RAINBOW.length; i++) {
      expect(hueVars(i)).not.toHaveProperty("--item-hue-ink-inv");
    }
  });
});

describe("isValidPalette", () => {
  it("accepts the built-in RAINBOW palette", () => {
    expect(isValidPalette(RAINBOW)).toBe(true);
  });

  it("accepts any full set of 8 valid hex colours", () => {
    const custom = ["#111111", "#222222", "#333333", "#444444", "#555555", "#666666", "#777777", "#888888"];
    expect(isValidPalette(custom)).toBe(true);
  });

  it("rejects the whole palette when one entry is invalid", () => {
    const almostAllValid = [...RAINBOW.slice(0, 7), "javascript:alert(1)"];
    expect(almostAllValid).toHaveLength(8);
    expect(isValidPalette(almostAllValid)).toBe(false);
  });

  it("rejects a palette with the wrong length, short or long", () => {
    expect(isValidPalette(RAINBOW.slice(0, 7))).toBe(false);
    expect(isValidPalette([...RAINBOW, "#000000"])).toBe(false);
    expect(isValidPalette([])).toBe(false);
  });

  it("rejects a 3-digit shorthand hex", () => {
    const withShorthand = [...RAINBOW.slice(0, 7), "#fff"];
    expect(isValidPalette(withShorthand)).toBe(false);
  });
});
