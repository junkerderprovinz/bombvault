// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// GlimStone motion-engine persistence. `.dom.test.tsx` mirrors shape.dom.
// test.tsx's own naming convention for the jsdom opt-in exception (this file
// renders no JSX either — it only needs jsdom for `document`/`localStorage`,
// both of which vitest's jsdom environment provides via the per-file
// `// @vitest-environment jsdom` docblock).
//
// Covers the full round-trip: applyMotionIntensity's validate-or-default-to-
// "full" contract, getMotionIntensity's read-back of a stored value (falling
// back to "full" on nothing-stored/corrupt/invalid), and
// setMotionIntensity's persist-then-apply behavior — the identical shape of
// coverage shape.dom.test.tsx already has for its own sibling appearance
// setting.
// ---------------------------------------------------------------------------
import { beforeEach, describe, expect, it } from "vitest";
import {
  MOTION_INTENSITIES,
  applyMotionIntensity,
  getMotionIntensity,
  setMotionIntensity,
  type MotionIntensity,
} from "./motion";

const STORAGE_KEY = "bv-motion";

beforeEach(() => {
  localStorage.clear();
  document.documentElement.removeAttribute("data-motion");
});

describe("MOTION_INTENSITIES", () => {
  it("is exactly the three motion-engine values, in order", () => {
    expect(MOTION_INTENSITIES).toEqual(["off", "subtle", "full"]);
  });
});

describe("applyMotionIntensity", () => {
  it("stamps data-motion with a valid value", () => {
    applyMotionIntensity("off");
    expect(document.documentElement.getAttribute("data-motion")).toBe("off");
    applyMotionIntensity("subtle");
    expect(document.documentElement.getAttribute("data-motion")).toBe("subtle");
    applyMotionIntensity("full");
    expect(document.documentElement.getAttribute("data-motion")).toBe("full");
  });

  it('defaults to "full" for undefined', () => {
    applyMotionIntensity(undefined);
    expect(document.documentElement.getAttribute("data-motion")).toBe("full");
  });

  it('defaults to "full" for an invalid/unknown string', () => {
    applyMotionIntensity("turbo");
    expect(document.documentElement.getAttribute("data-motion")).toBe("full");
  });
});

describe("getMotionIntensity", () => {
  it('defaults to "full" when nothing is stored', () => {
    expect(getMotionIntensity()).toBe("full");
  });

  it("round-trips a validly stored intensity", () => {
    localStorage.setItem(STORAGE_KEY, "subtle");
    expect(getMotionIntensity()).toBe("subtle");
  });

  it('falls back to "full" for a corrupt/invalid stored value', () => {
    localStorage.setItem(STORAGE_KEY, "not-a-motion-level");
    expect(getMotionIntensity()).toBe("full");
  });
});

describe("setMotionIntensity", () => {
  it("persists the choice AND applies it to the document immediately", () => {
    setMotionIntensity("off");
    expect(localStorage.getItem(STORAGE_KEY)).toBe("off");
    expect(document.documentElement.getAttribute("data-motion")).toBe("off");
    expect(getMotionIntensity()).toBe("off");
  });

  it("round-trips every intensity in MOTION_INTENSITIES", () => {
    for (const m of MOTION_INTENSITIES) {
      setMotionIntensity(m);
      expect(getMotionIntensity()).toBe(m);
      expect(document.documentElement.getAttribute("data-motion")).toBe(m);
    }
  });

  it("overwrites a previously persisted choice rather than merging", () => {
    setMotionIntensity("off");
    setMotionIntensity("full");
    const stored: MotionIntensity | null = localStorage.getItem(STORAGE_KEY) as MotionIntensity | null;
    expect(stored).toBe("full");
    expect(getMotionIntensity()).toBe("full");
  });
});
