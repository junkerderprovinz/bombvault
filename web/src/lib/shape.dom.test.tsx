// @vitest-environment jsdom
// The shape setting round-trip. jsdom is here for `document` and
// `localStorage`; nothing is rendered.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { beforeEach, describe, expect, it } from "vitest";
import {
  LEAF_CLICKS,
  SHAPES,
  applyShape,
  armShapeTransitions,
  getShape,
  leafTap,
  setShape,
  type Shape,
} from "./shape";

const STORAGE_KEY = "bv-shape";

beforeEach(() => {
  localStorage.clear();
  document.documentElement.removeAttribute("data-shape");
  document.documentElement.classList.remove("glim-shape-transitions");
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

  it('defaults to "soft" for undefined', () => {
    applyShape(undefined);
    expect(document.documentElement.getAttribute("data-shape")).toBe("soft");
  });

  it('defaults to "soft" for an invalid/unknown string', () => {
    applyShape("triangle");
    expect(document.documentElement.getAttribute("data-shape")).toBe("soft");
  });
});

describe("getShape", () => {
  it('defaults to "soft" when nothing is stored', () => {
    expect(getShape()).toBe("soft");
  });

  it("round-trips a validly stored shape", () => {
    localStorage.setItem(STORAGE_KEY, "square");
    expect(getShape()).toBe("square");
  });

  it('falls back to "soft" for a corrupt/invalid stored value', () => {
    localStorage.setItem(STORAGE_KEY, "not-a-shape");
    expect(getShape()).toBe("soft");
  });

  it('keeps a stored "round" rather than moving it to the default', () => {
    localStorage.setItem(STORAGE_KEY, "round");
    expect(getShape()).toBe("round");
  });
});

describe("setShape", () => {
  it("persists the choice and applies it to the document immediately", () => {
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

describe("the leaf", () => {
  it("is not in the list a picker builds from", () => {
    expect(SHAPES).not.toContain("leaf");
  });

  it("opens on the fifth click, and only from the shape already chosen", () => {
    const state = { taps: 0 };
    for (let i = 1; i < LEAF_CLICKS; i += 1) {
      expect(leafTap(state, "square", "square")).toBeUndefined();
    }
    expect(leafTap(state, "square", "square")).toBe("leaf");
  });

  it("resets the count once it opens", () => {
    const state = { taps: 0 };
    for (let i = 1; i < LEAF_CLICKS; i += 1) leafTap(state, "square", "square");
    leafTap(state, "square", "square");
    expect(state.taps).toBe(0);
  });

  it("cannot be reached from any other shape", () => {
    const state = { taps: 0 };
    for (let i = 0; i < LEAF_CLICKS * 2; i += 1) {
      expect(leafTap(state, "round", "round")).toBeUndefined();
      expect(leafTap(state, "soft", "soft")).toBeUndefined();
    }
  });

  // The click that moves to square is not one of the five.
  it("does not count the click that chooses square", () => {
    const state = { taps: 0 };
    leafTap(state, "square", "soft");
    for (let i = 1; i < LEAF_CLICKS; i += 1) {
      expect(leafTap(state, "square", "square")).toBeUndefined();
    }
    expect(leafTap(state, "square", "square")).toBe("leaf");
  });

  // A click on another shape in between is browsing the picker, not insisting.
  it("forgets the count when another shape is clicked in between", () => {
    const state = { taps: 0 };
    leafTap(state, "square", "square");
    leafTap(state, "square", "square");
    leafTap(state, "round", "square");
    for (let i = 1; i < LEAF_CLICKS; i += 1) {
      expect(leafTap(state, "square", "square")).toBeUndefined();
    }
    expect(leafTap(state, "square", "square")).toBe("leaf");
  });

  // Without these rules the attribute is set and nothing looks like a leaf.
  it("has its own token block and takes the other diagonal from every element", () => {
    const css = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "..", "index.css"), "utf8");
    expect(css).toContain(':root[data-shape="leaf"] {');
    const rule = css.match(/:root\[data-shape="leaf"\] \*,[^{]*\{([^}]*)\}/);
    expect(rule?.[0]).toContain("*::before");
    expect(rule?.[0]).toContain("*::after");
    expect(rule?.[1]).toContain("border-top-right-radius: 0 !important");
    expect(rule?.[1]).toContain("border-bottom-left-radius: 0 !important");
  });

  // Validation accepts a shape the picker does not offer.
  it("survives a reload even though no picker offers it", () => {
    setShape("leaf");
    expect(localStorage.getItem(STORAGE_KEY)).toBe("leaf");
    expect(getShape()).toBe("leaf");
    document.documentElement.removeAttribute("data-shape");
    applyShape(getShape());
    expect(document.documentElement.getAttribute("data-shape")).toBe("leaf");
  });
});

// index.css turns the class into a border-radius transition; these tests
// cover only when the class is present.
describe("armShapeTransitions", () => {
  it("does not add .glim-shape-transitions until called", () => {
    expect(document.documentElement.classList.contains("glim-shape-transitions")).toBe(false);
  });

  it("adds .glim-shape-transitions when called", () => {
    armShapeTransitions();
    expect(document.documentElement.classList.contains("glim-shape-transitions")).toBe(true);
  });

  it("is idempotent: a second call neither removes nor duplicates the class", () => {
    armShapeTransitions();
    armShapeTransitions();
    expect(document.documentElement.classList.contains("glim-shape-transitions")).toBe(true);
    expect(document.documentElement.className.split(/\s+/).filter((c) => c === "glim-shape-transitions").length).toBe(1);
  });

  it("keeps the class through a later setShape()", () => {
    armShapeTransitions();
    setShape("soft");
    expect(document.documentElement.classList.contains("glim-shape-transitions")).toBe(true);
    expect(document.documentElement.getAttribute("data-shape")).toBe("soft");
  });
});
