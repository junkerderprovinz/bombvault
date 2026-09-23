// @vitest-environment jsdom
import { beforeEach, describe, expect, it } from "vitest";
import {
  MOTION_INTENSITIES,
  applyMotionIntensity,
  getMotionIntensity,
  setMotionIntensity,
  STORM_CLICKS,
  stormTap,
  type MotionIntensity,
} from "./motion";

const STORAGE_KEY = "bv-motion";

beforeEach(() => {
  localStorage.clear();
  document.documentElement.removeAttribute("data-motion");
});

describe("MOTION_INTENSITIES", () => {
  it("is exactly the three motion-engine values, in order", () => {
    expect(MOTION_INTENSITIES).toEqual(["off", "subtle", "wild"]);
  });
});

describe("applyMotionIntensity", () => {
  it("stamps data-motion with a valid value", () => {
    applyMotionIntensity("off");
    expect(document.documentElement.getAttribute("data-motion")).toBe("off");
    applyMotionIntensity("subtle");
    expect(document.documentElement.getAttribute("data-motion")).toBe("subtle");
    applyMotionIntensity("wild");
    expect(document.documentElement.getAttribute("data-motion")).toBe("wild");
  });

  it('defaults to "subtle" for undefined', () => {
    applyMotionIntensity(undefined);
    expect(document.documentElement.getAttribute("data-motion")).toBe("subtle");
  });

  it('defaults to "subtle" for an unknown string', () => {
    applyMotionIntensity("turbo");
    expect(document.documentElement.getAttribute("data-motion")).toBe("subtle");
  });
});

describe("getMotionIntensity", () => {
  it('defaults to "subtle" when nothing is stored', () => {
    expect(getMotionIntensity()).toBe("subtle");
  });

  it("round-trips a validly stored intensity", () => {
    localStorage.setItem(STORAGE_KEY, "subtle");
    expect(getMotionIntensity()).toBe("subtle");
  });

  it('falls back to "subtle" for an invalid stored value', () => {
    localStorage.setItem(STORAGE_KEY, "not-a-motion-level");
    expect(getMotionIntensity()).toBe("subtle");
  });

  // "full" is the pre-2.0.0 name of "wild". Falling back to the default would
  // move someone who chose the strong level down to "subtle".
  it('migrates a pre-2.0.0 stored "full" to "wild" rather than dropping it to the default', () => {
    localStorage.setItem(STORAGE_KEY, "full");
    expect(getMotionIntensity()).toBe("wild");
    applyMotionIntensity(getMotionIntensity());
    expect(document.documentElement.getAttribute("data-motion")).toBe("wild");
  });

  // getMotionIntensity runs on every boot, so writing from it would turn one
  // stale key into a write on every page load.
  it("leaves the stored string alone while migrating it on read", () => {
    localStorage.setItem(STORAGE_KEY, "full");
    getMotionIntensity();
    expect(localStorage.getItem(STORAGE_KEY)).toBe("full");
  });

  // A lookup through the prototype chain would resolve "toString" to a
  // function and write it onto data-motion.
  it("does not treat an inherited Object key as a legacy level", () => {
    for (const key of ["toString", "constructor", "hasOwnProperty", "__proto__"]) {
      localStorage.setItem(STORAGE_KEY, key);
      expect(getMotionIntensity()).toBe("subtle");
    }
  });

  // applyMotionIntensity accepts raw localStorage values, so it has to agree
  // with the getter on legacy spellings.
  it("resolves a legacy spelling on the apply path too", () => {
    applyMotionIntensity("full");
    expect(document.documentElement.getAttribute("data-motion")).toBe("wild");
  });

  it("still falls back to the default on the apply path for real junk", () => {
    applyMotionIntensity("toString");
    expect(document.documentElement.getAttribute("data-motion")).toBe("subtle");
    applyMotionIntensity(undefined);
    expect(document.documentElement.getAttribute("data-motion")).toBe("subtle");
  });
});

describe("setMotionIntensity", () => {
  it("persists the choice and applies it immediately", () => {
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
    setMotionIntensity("wild");
    const stored: MotionIntensity | null = localStorage.getItem(STORAGE_KEY) as MotionIntensity | null;
    expect(stored).toBe("wild");
    expect(getMotionIntensity()).toBe("wild");
  });
});

describe("the storm", () => {
  it("is not in the list a picker builds from", () => {
    expect(MOTION_INTENSITIES).not.toContain("storm");
  });

  it("opens on the fifth click, and only from the level already chosen", () => {
    const state = { taps: 0 };
    for (let i = 1; i < STORM_CLICKS; i += 1) {
      expect(stormTap(state, "wild", "wild")).toBeUndefined();
    }
    expect(stormTap(state, "wild", "wild")).toBe("storm");
  });

  it("resets the count once it opens", () => {
    const state = { taps: 0 };
    for (let i = 1; i < STORM_CLICKS; i += 1) stormTap(state, "wild", "wild");
    stormTap(state, "wild", "wild");
    expect(state.taps).toBe(0);
  });

  // Clicking "off" five times means somebody is annoyed, not curious.
  it("cannot be reached from any other level", () => {
    const state = { taps: 0 };
    for (let i = 0; i < STORM_CLICKS * 2; i += 1) {
      expect(stormTap(state, "off", "off")).toBeUndefined();
      expect(stormTap(state, "subtle", "subtle")).toBeUndefined();
    }
  });

  // A click on another level in between is browsing the picker, not insisting.
  it("forgets the count when another level is clicked in between", () => {
    const state = { taps: 0 };
    stormTap(state, "wild", "wild");
    stormTap(state, "wild", "wild");
    stormTap(state, "subtle", "wild");
    for (let i = 1; i < STORM_CLICKS; i += 1) {
      expect(stormTap(state, "wild", "wild")).toBeUndefined();
    }
    expect(stormTap(state, "wild", "wild")).toBe("storm");
  });

  // Validation accepts a level the picker does not offer.
  it("survives a reload even though no picker offers it", () => {
    setMotionIntensity("storm");
    expect(localStorage.getItem(STORAGE_KEY)).toBe("storm");
    expect(getMotionIntensity()).toBe("storm");
    document.documentElement.removeAttribute("data-motion");
    applyMotionIntensity(getMotionIntensity());
    expect(document.documentElement.getAttribute("data-motion")).toBe("storm");
  });
});
