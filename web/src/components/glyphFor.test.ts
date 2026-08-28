// ---------------------------------------------------------------------------
// glyphFor (#178, [202]) — the mapping from meaning to symbol.
//
// The point of these tests is not that a given key returns "an icon": it is
// that the SAME VERB gets the SAME symbol wherever it appears, which is the
// entire reason the mapping is keyed off translation keys rather than chosen
// per call site. And that specificity wins: "backupSelected" must not fall
// through to the generic "selected" rule.
// ---------------------------------------------------------------------------
import { describe, expect, it } from "vitest";
import { glyphFor } from "./glyphFor";

describe("glyphFor", () => {
  it("gives one verb one symbol across different pages", () => {
    const a = glyphFor("containers.backupNow");
    const b = glyphFor("config.backupNow");
    const c = glyphFor("files.backupAll");
    expect(a).toEqual(b);
    expect(a).toEqual(c);
  });

  it("keeps distinct verbs distinct", () => {
    expect(glyphFor("snapshots.delete")).not.toEqual(glyphFor("snapshots.restore"));
    expect(glyphFor("offsite.test")).not.toEqual(glyphFor("files.addSet"));
  });

  it("lets a specific rule win over a general one", () => {
    // "backupSelected" is a backup, not a selection.
    expect(glyphFor("containers.backupSelected")).toEqual(glyphFor("containers.backupNow"));
    // "unlock" is not "lock"/"key".
    expect(glyphFor("integrity.unlock")).not.toEqual(glyphFor("cloud.credSets.add"));
  });

  it("returns undefined rather than a meaningless symbol", () => {
    // A key nothing sensible matches keeps its text in glyph mode, which is
    // the better failure: a wrong symbol is worse than a word.
    expect(glyphFor("zzz.somethingWithNoVerb")).toBeUndefined();
  });
});
