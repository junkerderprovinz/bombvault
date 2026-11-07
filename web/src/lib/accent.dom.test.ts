// @vitest-environment jsdom
import { beforeEach, describe, expect, it } from "vitest";
import { DEFAULT_ACCENT, DEFAULT_ACCENT_PRESETS, applyAccent, getAccentPresets, setAccentPresets } from "./accent";

const STORAGE_KEY = "bv-accent-presets";

const CUSTOM_PRESETS = [
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
});

describe("getAccentPresets", () => {
  it("defaults to DEFAULT_ACCENT_PRESETS when nothing is stored", () => {
    expect(getAccentPresets()).toEqual(DEFAULT_ACCENT_PRESETS);
  });

  it("round-trips a validly stored set", () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(CUSTOM_PRESETS));
    expect(getAccentPresets()).toEqual(CUSTOM_PRESETS);
  });

  it("falls back to the built-in defaults when the stored set is invalid", () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(["#fff"]));
    expect(getAccentPresets()).toEqual(DEFAULT_ACCENT_PRESETS);
  });

  it("falls back to the built-in defaults on corrupt JSON", () => {
    localStorage.setItem(STORAGE_KEY, "{not json");
    expect(getAccentPresets()).toEqual(DEFAULT_ACCENT_PRESETS);
  });
});

describe("setAccentPresets", () => {
  it("persists a valid set and returns it unchanged", () => {
    const result = setAccentPresets(CUSTOM_PRESETS);
    expect(result).toEqual(CUSTOM_PRESETS);
    expect(getAccentPresets()).toEqual(CUSTOM_PRESETS);
  });

  it("stores an edit to one preset without touching the others", () => {
    setAccentPresets(DEFAULT_ACCENT_PRESETS);
    const edited = DEFAULT_ACCENT_PRESETS.slice();
    edited[2] = "#abcdef";
    const result = setAccentPresets(edited);
    expect(result[2]).toBe("#abcdef");
    for (let i = 0; i < DEFAULT_ACCENT_PRESETS.length; i++) {
      if (i === 2) continue;
      expect(result[i]).toBe(DEFAULT_ACCENT_PRESETS[i]);
    }
    expect(getAccentPresets()).toEqual(result);
  });

  it("restores the built-in defaults on reset", () => {
    setAccentPresets(CUSTOM_PRESETS);
    expect(getAccentPresets()).toEqual(CUSTOM_PRESETS);
    const result = setAccentPresets(DEFAULT_ACCENT_PRESETS);
    expect(result).toEqual(DEFAULT_ACCENT_PRESETS);
    expect(getAccentPresets()).toEqual(DEFAULT_ACCENT_PRESETS);
  });

  it("stores the defaults instead of a set with one invalid entry", () => {
    const bad = [...DEFAULT_ACCENT_PRESETS.slice(0, 7), "javascript:alert(1)"];
    const result = setAccentPresets(bad);
    expect(result).toEqual(DEFAULT_ACCENT_PRESETS);

    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY)!);
    expect(stored).toEqual(DEFAULT_ACCENT_PRESETS);
    expect(stored).not.toEqual(bad);
  });
});

describe("accent text on a colour the presets never come near", () => {
  const root = document.documentElement.style;

  it("carries an ink of its own for the theme the mix falls short on", () => {
    applyAccent("#FAFAFA");
    expect(root.getPropertyValue("--accent-ink-light")).toMatch(/^#[0-9A-F]{6}$/);
    expect(root.getPropertyValue("--accent-ink-dark")).toBe("");
    applyAccent("#0000FF");
    expect(root.getPropertyValue("--accent-ink-dark")).toMatch(/^#[0-9A-F]{6}$/);
  });

  it("lets the stylesheet's mix stand again for a preset", () => {
    applyAccent("#0000FF");
    applyAccent(DEFAULT_ACCENT);
    expect(root.getPropertyValue("--accent-ink-light")).toBe("");
    expect(root.getPropertyValue("--accent-ink-dark")).toBe("");
  });
});
