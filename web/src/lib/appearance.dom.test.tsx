// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// GlimStone form-engine Phase 2, Task 1 — rainbow/reactive colour engine.
//
// Covers the DOM/localStorage-touching half appearance.test.ts's own header
// comment deliberately leaves out: applyRainbow/getRainbow/setRainbow. The
// original report for this task justified skipping these by citing "this
// branch's established no-jsdom pattern" — but that pattern is no longer
// (if it ever fully was) accurate: VMs.test.tsx already opts a test file
// into jsdom via vitest's per-file `// @vitest-environment jsdom` docblock
// (see vitest.config.ts's own header comment), and that file is present on
// this branch too (merged into main from the earlier VM-service-layer-
// integration branch, well before this one). `.test.tsx` mirrors that same
// file's naming convention for the jsdom-opted-in exception, even though
// this file itself renders no JSX — it only needs jsdom for `document` and
// `localStorage`, both of which vitest's jsdom environment provides.
//
// Most important case here: the persist-before-validate regression. Before
// the fix, setRainbow() serialized the raw pre-validation merged patch to
// localStorage BEFORE calling applyRainbow() (which is what actually clamps
// an out-of-range seed and rejects an invalid palette all-or-nothing) — so
// an invalid value could survive in storage indefinitely even though the DOM
// and in-memory state both correctly showed the sanitized one, because every
// subsequent setRainbow() call re-merges from that same still-poisoned
// getRainbow() read. See the "persists the CLAMPED/REJECTED..." and
// "converges" tests below.
// ---------------------------------------------------------------------------
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { RAINBOW, RAINBOW_OFF, applyRainbow, getRainbow, setRainbow } from "./appearance";

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
  // Fake timers — GlimStone motion-engine, animation 4 (colour-wipe):
  // applyRainbow() now schedules a real setTimeout (beginColourWipe(), see
  // that function's own comment in appearance.ts) whenever data-rainbow
  // actually changes, and this file's OWN beforeEach below deliberately
  // drives a real on→off→on sequence across many tests sharing one module
  // instance — without fake timers each of those would leave a genuine
  // pending 500ms browser timer running past the end of its test.
  vi.useFakeTimers();
  document.documentElement.classList.remove("bv-colour-wipe");
  // Drive the module back to a known, fully-off baseline. appearance.ts
  // holds `state` at module scope deliberately (see its own "Live state"
  // comment) — applyRainbow() resets both that singleton and the DOM
  // attribute/properties, so this also isolates `state` between tests
  // sharing this same module instance within the test file/process.
  applyRainbow(RAINBOW_OFF);
  // The reset above can itself be a genuine on→off flip carried over from
  // the PREVIOUS test (module state, including the colour-wipe's own
  // wipeMounted/wipeLastAttr, persists across `it()` blocks in this file —
  // same reasoning as the comment above) and so can legitimately arm a
  // wipe of its own; clear it here so every test starts from a clean,
  // wipe-free baseline regardless of what the previous test left mid-flight.
  vi.runOnlyPendingTimers();
  document.documentElement.classList.remove("bv-colour-wipe");
});

afterEach(() => {
  vi.useRealTimers();
});

describe("applyRainbow — data-rainbow attribute", () => {
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

describe("applyRainbow — --rb-* custom properties on document.documentElement", () => {
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

  // Regression coverage for the persist-before-validate bug: localStorage
  // must end up holding the VALIDATED value applyRainbow() actually
  // accepted, never the raw pre-validation patch.
  it("persists the CLAMPED seed, not the raw out-of-range one", () => {
    const clamped = 99 % RAINBOW.length;
    const result = setRainbow({ on: true, rotate: true, seed: 99 });
    expect(result.seed).toBe(clamped);

    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY)!);
    expect(stored.seed).toBe(clamped);
    expect(stored.seed).not.toBe(99);
    // getRainbow() must also read back the validated value, not the poison.
    expect(getRainbow().seed).toBe(clamped);
  });

  it("persists the REJECTED-and-replaced palette, not the raw invalid one", () => {
    const bad = [...RAINBOW.slice(0, 7), "javascript:alert(1)"];
    const result = setRainbow({ on: true, palette: bad });
    expect(result.palette).toEqual(RAINBOW);

    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY)!);
    expect(stored.palette).toEqual(RAINBOW);
    expect(stored.palette).not.toEqual(bad);
    expect(getRainbow().palette).toEqual(RAINBOW);
  });

  it("converges on the validated value across repeated calls, rather than staying poisoned", () => {
    // Reproduces the reviewer's live repro exactly: an invalid seed must not
    // keep surviving in storage across SEVERAL subsequent setRainbow() calls
    // just because each one re-merges from a still-poisoned getRainbow()
    // read. With the pre-fix code, this would fail because the FIRST call
    // already wrote the raw seed=99 to storage before validation, and every
    // later call re-read that same raw 99 back out.
    setRainbow({ on: true, rotate: true, seed: 99 });
    setRainbow({ reactive: true });
    const third = setRainbow({ rotate: true });

    const clamped = 99 % RAINBOW.length;
    expect(third.seed).toBe(clamped);
    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY)!);
    expect(stored.seed).toBe(clamped);
  });
});

// ---------------------------------------------------------------------------
// GlimStone motion-engine, animation 4 — colour-wipe. index.css's own
// ".bv-colour-wipe" rule (inside @media (prefers-reduced-motion:
// no-preference)) is what actually turns this class into a transition; this
// suite only covers the JS half's WIRING — that the class lands on a real
// flip, never on a no-op re-apply, and comes back off on its own.
// ---------------------------------------------------------------------------
describe("applyRainbow — colour-wipe class", () => {
  it("adds .bv-colour-wipe on a real off→on flip", () => {
    applyRainbow({ on: true, reactive: false });
    expect(document.documentElement.classList.contains("bv-colour-wipe")).toBe(true);
  });

  it("adds .bv-colour-wipe on an on→reactive flip (still a resolved-attribute change)", () => {
    applyRainbow({ on: true, reactive: false });
    vi.runOnlyPendingTimers();
    document.documentElement.classList.remove("bv-colour-wipe");
    applyRainbow({ on: true, reactive: true });
    expect(document.documentElement.classList.contains("bv-colour-wipe")).toBe(true);
  });

  it("does NOT add .bv-colour-wipe on a no-op re-apply of the identical resolved state", () => {
    applyRainbow({ on: true, reactive: false });
    vi.runOnlyPendingTimers();
    document.documentElement.classList.remove("bv-colour-wipe");
    // Same on/reactive as above — a different call (e.g. re-applying a
    // stored palette edit) but the resolved data-rainbow value is unchanged.
    applyRainbow({ on: true, reactive: false, seed: 3 });
    expect(document.documentElement.classList.contains("bv-colour-wipe")).toBe(false);
  });

  it("removes .bv-colour-wipe again after its own timer fires", () => {
    applyRainbow({ on: true, reactive: false });
    expect(document.documentElement.classList.contains("bv-colour-wipe")).toBe(true);
    vi.runOnlyPendingTimers();
    expect(document.documentElement.classList.contains("bv-colour-wipe")).toBe(false);
  });

  it("restarts its own timer on a second rapid flip instead of removing the class early", () => {
    applyRainbow({ on: true, reactive: false }); // t=0: timer A armed, due at t=500
    vi.advanceTimersByTime(200); // t=200
    applyRainbow({ on: true, reactive: true }); // second flip: clears A, arms timer B due at t=700
    vi.advanceTimersByTime(400); // t=600 — B (t=700) not due yet; had A survived it WOULD have fired at 500
    expect(document.documentElement.classList.contains("bv-colour-wipe")).toBe(true);
    vi.advanceTimersByTime(150); // t=750 — past B's own t=700
    expect(document.documentElement.classList.contains("bv-colour-wipe")).toBe(false);
  });
});
