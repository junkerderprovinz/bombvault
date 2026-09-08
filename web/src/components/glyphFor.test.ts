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

// The About card's buttons (jdp, 2026-09-08: "in der übercard fehlen die
// glyphen auf den buttons"). They had none because nothing in the table
// matched, and an unmatched key deliberately returns undefined rather than a
// stand-in symbol.
describe("glyphFor — the About card", () => {
  it("gives the coffee and the mail button a symbol", () => {
    expect(glyphFor("about.coffeeButton")).toBeDefined();
    expect(glyphFor("about.mail")).toBeDefined();
  });

  it("keeps those two apart", () => {
    expect(glyphFor("about.coffeeButton")).not.toEqual(glyphFor("about.mail"));
  });

  it("still gives the repository button NOTHING, on purpose", () => {
    // It wears GitHub's own mark, passed explicitly at its one call site. A
    // pattern on "repo" would put that logo on repository settings that have
    // nothing to do with GitHub, so the table must never be able to reach it.
    expect(glyphFor("about.repo")).toBeUndefined();
    expect(glyphFor("offsite.repoUrl")).toBeUndefined();
  });

  it("does not let the mail rule swallow unrelated keys", () => {
    // The rule is anchored on a dot so it takes `about.mail` and
    // `settings.mailFrom`, not every key that happens to contain the letters.
    expect(glyphFor("recovery.emailless")).toBeUndefined();
  });
});
