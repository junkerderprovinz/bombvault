// @vitest-environment jsdom
// The DOM and localStorage half of appearance.ts: applyRainbow, getRainbow and
// setRainbow. setRainbow has to persist the value applyRainbow accepted (the
// clamped seed, the replaced palette), not the raw patch, or every later call
// re-merges from the invalid stored value and storage never converges.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { RAINBOW, RAINBOW_OFF, applyRainbow, getRainbow, setRainbow } from "./appearance";
import { contrastOn } from "./accent";

const STORAGE_KEY = "bv-rainbow";

const CUSTOM_PALETTE = [
  "#111111",
  "#222222",
  "#333333",
  "#444444",
  "#555555",
  "#666666",
  "#777777",
  "#888888",
];

beforeEach(() => {
  localStorage.clear();
  // applyRainbow arms a 500ms timer for the colour wipe whenever
  // data-rainbow changes, and these tests flip it constantly.
  vi.useFakeTimers();
  document.documentElement.classList.remove("glim-colour-wipe");
  // appearance.ts keeps its state at module scope, shared by every test in
  // this file. Reset it, then drain any wipe the reset itself armed.
  applyRainbow(RAINBOW_OFF);
  vi.runOnlyPendingTimers();
  document.documentElement.classList.remove("glim-colour-wipe");
});

afterEach(() => {
  vi.useRealTimers();
});

describe("applyRainbow: data-rainbow attribute", () => {
  it("removes the attribute when off", () => {
    applyRainbow({ on: false });
    expect(document.documentElement.hasAttribute("data-rainbow")).toBe(false);
  });

  it('sets data-rainbow="on" when on and not reactive', () => {
    applyRainbow({ on: true, reactive: false });
    expect(document.documentElement.getAttribute("data-rainbow")).toBe("on");
  });

  it('sets data-rainbow="reactive" when on and reactive', () => {
    applyRainbow({ on: true, reactive: true });
    expect(document.documentElement.getAttribute("data-rainbow")).toBe("reactive");
  });
});

describe("applyRainbow: --rb-* custom properties", () => {
  it("stamps --rb-0..--rb-7 with the built-in palette even while off", () => {
    applyRainbow({ on: false });
    const root = document.documentElement;
    for (let i = 0; i < RAINBOW.length; i++) {
      expect(root.style.getPropertyValue(`--rb-${i}`)).toBe(RAINBOW[i]);
    }
  });

  it("reflects a custom, valid palette in the --rb-* properties", () => {
    applyRainbow({ on: true, palette: CUSTOM_PALETTE });
    const root = document.documentElement;
    for (let i = 0; i < CUSTOM_PALETTE.length; i++) {
      expect(root.style.getPropertyValue(`--rb-${i}`)).toBe(CUSTOM_PALETTE[i]);
    }
  });

  it("falls back to the built-in RAINBOW for an invalid palette (all-or-nothing)", () => {
    const bad = [...RAINBOW.slice(0, 7), "javascript:alert(1)"];
    applyRainbow({ on: true, palette: bad });
    const root = document.documentElement;
    for (let i = 0; i < RAINBOW.length; i++) {
      expect(root.style.getPropertyValue(`--rb-${i}`)).toBe(RAINBOW[i]);
    }
  });

  it("stamps each position's ink beside its colour, rotation included", () => {
    applyRainbow({ on: true, palette: CUSTOM_PALETTE, rotate: true, seed: 3 });
    const root = document.documentElement;
    for (let i = 0; i < CUSTOM_PALETTE.length; i++) {
      const colour = root.style.getPropertyValue(`--rb-${i}`);
      expect(colour).toBe(CUSTOM_PALETTE[(i + 3) % CUSTOM_PALETTE.length]);
      expect(root.style.getPropertyValue(`--rb-ink-${i}`)).toBe(contrastOn(colour));
    }
  });
});

describe("getRainbow", () => {
  it("defaults to RAINBOW_OFF when nothing is stored", () => {
    expect(getRainbow()).toEqual(RAINBOW_OFF);
  });

  it("round-trips a validly stored state", () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ on: true, reactive: true, rotate: true, seed: 2, palette: CUSTOM_PALETTE }),
    );
    expect(getRainbow()).toEqual({
      on: true,
      reactive: true,
      rotate: true,
      seed: 2,
      palette: CUSTOM_PALETTE,
    });
  });

  it("falls back to the built-in palette when the stored palette is invalid", () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ on: true, palette: ["#fff"] }));
    expect(getRainbow().palette).toEqual(RAINBOW);
  });

  it("falls back to RAINBOW_OFF entirely on corrupt JSON", () => {
    localStorage.setItem(STORAGE_KEY, "{not json");
    expect(getRainbow()).toEqual(RAINBOW_OFF);
  });
});

describe("setRainbow", () => {
  it("persists, applies, and returns the merged state for a valid patch", () => {
    const result = setRainbow({ on: true, reactive: true });
    expect(result.on).toBe(true);
    expect(result.reactive).toBe(true);
    expect(document.documentElement.getAttribute("data-rainbow")).toBe("reactive");
    expect(getRainbow()).toEqual(result);
  });

  it("merges onto the previously persisted state, not a fresh default", () => {
    setRainbow({ on: true, rotate: true, seed: 3 });
    const result = setRainbow({ reactive: true });
    expect(result.on).toBe(true);
    expect(result.rotate).toBe(true);
    expect(result.seed).toBe(3);
    expect(result.reactive).toBe(true);
  });

  it("persists the clamped seed, not the raw out-of-range one", () => {
    const clamped = 99 % RAINBOW.length;
    const result = setRainbow({ on: true, rotate: true, seed: 99 });
    expect(result.seed).toBe(clamped);

    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY)!);
    expect(stored.seed).toBe(clamped);
    expect(stored.seed).not.toBe(99);
    expect(getRainbow().seed).toBe(clamped);
  });

  it("persists the replacement for a rejected palette, not the raw one", () => {
    const bad = [...RAINBOW.slice(0, 7), "javascript:alert(1)"];
    const result = setRainbow({ on: true, palette: bad });
    expect(result.palette).toEqual(RAINBOW);

    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY)!);
    expect(stored.palette).toEqual(RAINBOW);
    expect(stored.palette).not.toEqual(bad);
    expect(getRainbow().palette).toEqual(RAINBOW);
  });

  it("converges on the validated value across repeated calls", () => {
    // Each call re-merges from getRainbow(), so a raw seed written by the
    // first call would survive every later one.
    setRainbow({ on: true, rotate: true, seed: 99 });
    setRainbow({ reactive: true });
    const third = setRainbow({ rotate: true });

    const clamped = 99 % RAINBOW.length;
    expect(third.seed).toBe(clamped);
    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY)!);
    expect(stored.seed).toBe(clamped);
  });
});

// index.css turns .glim-colour-wipe into a transition (only under
// prefers-reduced-motion: no-preference). These tests cover the JS side: the
// class lands on a real flip, not on a re-apply that changes nothing, and
// comes off again by itself.
describe("applyRainbow: colour-wipe class", () => {
  it("adds .glim-colour-wipe when rainbow turns on", () => {
    applyRainbow({ on: true, reactive: false });
    expect(document.documentElement.classList.contains("glim-colour-wipe")).toBe(true);
  });

  it("adds .glim-colour-wipe when on turns reactive", () => {
    applyRainbow({ on: true, reactive: false });
    vi.runOnlyPendingTimers();
    document.documentElement.classList.remove("glim-colour-wipe");
    applyRainbow({ on: true, reactive: true });
    expect(document.documentElement.classList.contains("glim-colour-wipe")).toBe(true);
  });

  it("does not add .glim-colour-wipe when the resolved state is unchanged", () => {
    applyRainbow({ on: true, reactive: false });
    vi.runOnlyPendingTimers();
    document.documentElement.classList.remove("glim-colour-wipe");
    // A different call with the same resolved data-rainbow value.
    applyRainbow({ on: true, reactive: false, seed: 3 });
    expect(document.documentElement.classList.contains("glim-colour-wipe")).toBe(false);
  });

  it("removes .glim-colour-wipe again after its own timer fires", () => {
    applyRainbow({ on: true, reactive: false });
    expect(document.documentElement.classList.contains("glim-colour-wipe")).toBe(true);
    vi.runOnlyPendingTimers();
    expect(document.documentElement.classList.contains("glim-colour-wipe")).toBe(false);
  });

  it("restarts its own timer on a second rapid flip instead of removing the class early", () => {
    applyRainbow({ on: true, reactive: false }); // timer A, due at t=500
    vi.advanceTimersByTime(200);
    applyRainbow({ on: true, reactive: true }); // clears A, timer B due at t=700
    vi.advanceTimersByTime(400); // t=600: A would have fired by now, B has not
    expect(document.documentElement.classList.contains("glim-colour-wipe")).toBe(true);
    vi.advanceTimersByTime(150); // t=750
    expect(document.documentElement.classList.contains("glim-colour-wipe")).toBe(false);
  });
});
