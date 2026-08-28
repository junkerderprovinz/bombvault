// ---------------------------------------------------------------------------
// Control label engine (#178) — the pure half.
//
// Two properties matter here and neither needs a DOM:
//   - the width stage comes from the LABEL, so it cannot change with the mode
//     (jdp: a button must be the same width in all three modes);
//   - CJK labels count double, or Chinese buttons would be sized as if they
//     were a third as wide as they render.
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { labelWidth, widthStage, LABEL_MODES, CONTROL_AXES } from "./controls";

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
  it("offers exactly the three modes jdp asked for", () => {
    expect(LABEL_MODES).toEqual(["text", "textGlyph", "glyph"]);
  });

  it("keeps the three axes separately settable", () => {
    // Sidebar and tabs share the same options but not the same value: a
    // sidebar reduced to glyphs changes the page layout, tabs do not.
    expect(CONTROL_AXES).toEqual(["buttons", "sidebar", "tabs"]);
  });
});
