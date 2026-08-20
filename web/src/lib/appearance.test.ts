// GlimStone form-engine Phase 2, Task 1 — rainbow/reactive colour engine.
//
// Covers the pure logic only (rainbowColorAt's position/rotation math,
// hueVars, isValidPalette's all-or-nothing rule) — no `document`/
// `localStorage` needed, so this file stays on the default node environment.
// applyRainbow/getRainbow/setRainbow (the DOM/localStorage-touching half,
// including the security-relevant all-or-nothing palette rejection and the
// persist-before-validate regression) have their own jsdom-backed coverage
// in appearance.dom.test.tsx — see that file's header comment for why a
// jsdom opt-in is available and used here despite this file staying node.
import { describe, expect, it } from "vitest";
import { RAINBOW, hueVars, isValidPalette, rainbowColorAt } from "./appearance";
import { contrastOn } from "./accent";

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
  it("hands out colours by LIST POSITION, in order, when rotation is off", () => {
    for (let i = 0; i < RAINBOW.length; i++) {
      expect(rainbowColorAt(i, RAINBOW, false, /* seed */ 3)).toBe(RAINBOW[i]);
    }
  });

  it("ignores seed entirely when rotate is false, even with a nonzero seed", () => {
    expect(rainbowColorAt(0, RAINBOW, false, 5)).toBe(RAINBOW[0]);
    expect(rainbowColorAt(4, RAINBOW, false, 5)).toBe(RAINBOW[4]);
  });

  it("offsets the starting colour by seed when rotate is true", () => {
    // seed=1 means position 0 now reads what used to be position 1.
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
  it("returns the full custom-property set for a valid hex", () => {
    const vars = hueVars("#1D99F3");
    expect(vars["--item-hue"]).toBe("#1D99F3");
    expect(vars["--item-hue-ink"]).toBe(contrastOn("#1D99F3"));
    expect(vars["--item-hue-soft"]).toBe("rgba(29, 153, 243, 0.14)");
    expect(vars["--item-hue-wash"]).toBe("rgba(29, 153, 243, 0.07)");
    expect(vars["--item-hue-ring"]).toBe("rgba(29, 153, 243, 0.55)");
  });

  it("returns an empty object for an invalid hex, never a partial/garbage set", () => {
    expect(hueVars("not-a-color")).toEqual({});
    expect(hueVars("#12345")).toEqual({});
  });

  it("returns an empty object for undefined (no position owned)", () => {
    expect(hueVars(undefined)).toEqual({});
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

  it("rejects the WHOLE palette when even one entry is invalid — all-or-nothing, not 87% safe", () => {
    const almostAllValid = [...RAINBOW.slice(0, 7), "javascript:alert(1)"];
    expect(almostAllValid).toHaveLength(8);
    expect(isValidPalette(almostAllValid)).toBe(false);
  });

  it("rejects a palette with the wrong length, short or long", () => {
    expect(isValidPalette(RAINBOW.slice(0, 7))).toBe(false);
    expect(isValidPalette([...RAINBOW, "#000000"])).toBe(false);
    expect(isValidPalette([])).toBe(false);
  });

  it("rejects a 3-digit shorthand hex — only the full 6-digit form is accepted", () => {
    const withShorthand = [...RAINBOW.slice(0, 7), "#fff"];
    expect(isValidPalette(withShorthand)).toBe(false);
  });
});
