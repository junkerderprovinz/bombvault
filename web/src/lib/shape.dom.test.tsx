// @vitest-environment jsdom
// ---------------------------------------------------------------------------
// GlimStone form-engine — shape engine persistence. `.dom.test.tsx` mirrors
// appearance.dom.test.tsx's own naming convention for the jsdom opt-in
// exception (this file renders no JSX either — it only needs jsdom for
// `document`/`localStorage`, both of which vitest's jsdom environment
// provides via the per-file `// @vitest-environment jsdom` docblock).
//
// Covers the full round-trip: applyShape's validate-or-default-to-"round"
// contract, getShape's read-back of a stored value (falling back to "round"
// on nothing-stored/corrupt/invalid), and setShape's persist-then-apply
// behavior — the same shape of coverage accent.test.ts/appearance.dom.test.tsx
// already have for their own sibling appearance settings.
// ---------------------------------------------------------------------------
import { beforeEach, describe, expect, it } from "vitest";
import { SHAPES, applyShape, getShape, setShape, type Shape } from "./shape";

const STORAGE_KEY = "bv-shape";

beforeEach(() => {
  localStorage.clear();
  document.documentElement.removeAttribute("data-shape");
});

describe("SHAPES", () => {
  it("is exactly the three shape-engine values, in order", () => {
    expect(SHAPES).toEqual(["round", "soft", "square"]);
  });
});

describe("applyShape", () => {
  it("stamps data-shape with a valid value", () => {
    applyShape("soft");
    expect(document.documentElement.getAttribute("data-shape")).toBe("soft");
    applyShape("square");
    expect(document.documentElement.getAttribute("data-shape")).toBe("square");
    applyShape("round");
    expect(document.documentElement.getAttribute("data-shape")).toBe("round");
  });

  it('defaults to "round" for undefined', () => {
    applyShape(undefined);
    expect(document.documentElement.getAttribute("data-shape")).toBe("round");
  });

  it('defaults to "round" for an invalid/unknown string', () => {
    applyShape("triangle");
    expect(document.documentElement.getAttribute("data-shape")).toBe("round");
  });
});

describe("getShape", () => {
  it('defaults to "round" when nothing is stored', () => {
    expect(getShape()).toBe("round");
  });

  it("round-trips a validly stored shape", () => {
    localStorage.setItem(STORAGE_KEY, "square");
    expect(getShape()).toBe("square");
  });

  it('falls back to "round" for a corrupt/invalid stored value', () => {
    localStorage.setItem(STORAGE_KEY, "not-a-shape");
    expect(getShape()).toBe("round");
  });
});

describe("setShape", () => {
  it("persists the choice AND applies it to the document immediately", () => {
    setShape("soft");
    expect(localStorage.getItem(STORAGE_KEY)).toBe("soft");
    expect(document.documentElement.getAttribute("data-shape")).toBe("soft");
    expect(getShape()).toBe("soft");
  });

  it("round-trips every shape in SHAPES", () => {
    for (const s of SHAPES) {
      setShape(s);
      expect(getShape()).toBe(s);
      expect(document.documentElement.getAttribute("data-shape")).toBe(s);
    }
  });

  it("overwrites a previously persisted choice rather than merging", () => {
    setShape("square");
    setShape("round");
    const stored: Shape | null = localStorage.getItem(STORAGE_KEY) as Shape | null;
    expect(stored).toBe("round");
    expect(getShape()).toBe("round");
  });
});
