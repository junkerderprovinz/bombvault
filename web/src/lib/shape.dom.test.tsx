// @vitest-environment jsdom
// The shape setting round-trip. jsdom is here for `document` and
// `localStorage`; nothing is rendered.
import { beforeEach, describe, expect, it } from "vitest";
import { SHAPES, applyShape, armShapeTransitions, getShape, setShape, type Shape } from "./shape";

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
