// The pure half of the control label engine. The width stage comes from the
// label, so a button keeps its width in every mode, and CJK characters count
// double, or Chinese buttons would be sized far narrower than they render.
import { describe, expect, it } from "vitest";
import { labelWidth, widthStage, hidesLabel, LABEL_MODES, CONTROL_AXES } from "./controls";

describe("labelWidth", () => {
  it("counts latin characters singly", () => {
    expect(labelWidth("Clear")).toBe(5);
    expect(labelWidth("")).toBe(0);
  });

  it("counts CJK characters as two cells, because that is what they occupy", () => {
    // Four glyphs, but visually as wide as eight latin characters.
    expect(labelWidth("立即复制")).toBe(8);
  });

  it("handles a mixed label", () => {
    expect(labelWidth("S3 立即")).toBe(3 + 4);
  });
});

describe("widthStage", () => {
  it("puts short labels on the smallest stage", () => {
    expect(widthStage("Clear")).toBe("xs");
    expect(widthStage("Show")).toBe("xs");
  });

  it("moves a label up a stage when its translation grows", () => {
    // The real pair that made a global stage untenable: 3.4x growth.
    expect(widthStage("Clear")).toBe("xs");
    expect(widthStage("Kijelölés törlése")).toBe("md");
  });

  it("keeps the longest real labels on the largest stage", () => {
    expect(widthStage("Off-site-DR-Prüfung starten")).toBe("lg");
  });

  it("never returns undefined for an empty label", () => {
    expect(widthStage("")).toBe("xs");
  });
});

describe("the engine's shape", () => {
  it("offers the four label modes in order", () => {
    // The Settings strip renders them in this order, and reactive is glyph
    // plus a reveal on hover, so it follows glyph.
    expect(LABEL_MODES).toEqual(["text", "textGlyph", "glyph", "reactive"]);
  });

  it("counts both hiding modes as hiding", () => {
    // Call sites ask hidesLabel instead of comparing against "glyph", so a
    // hiding mode reaches all of them at once.
    expect(LABEL_MODES.filter(hidesLabel)).toEqual(["glyph", "reactive"]);
  });

  it("keeps the three axes separately settable", () => {
    // Sidebar and tabs share the same options but not the same value: a
    // sidebar reduced to glyphs changes the page layout, tabs do not.
    expect(CONTROL_AXES).toEqual(["buttons", "sidebar", "tabs"]);
  });
});
