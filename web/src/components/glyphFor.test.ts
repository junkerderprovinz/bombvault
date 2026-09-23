// The mapping is keyed off translation keys so the same verb gets the same
// symbol wherever it appears, and a specific rule wins over a general one.
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

  it("gives a confirmation the symbol of what it confirms", () => {
    expect(glyphFor("fleet.confirmRemove")).toEqual(glyphFor("fleet.remove"));
    expect(glyphFor("common.confirm")).toEqual(glyphFor("fleet.mesh.accept"));
    expect(glyphFor("common.confirm")).not.toEqual(glyphFor("common.save"));
  });

  it("returns undefined rather than a meaningless symbol", () => {
    // A key nothing sensible matches keeps its text in glyph mode, which is
    // the better failure: a wrong symbol is worse than a word.
    expect(glyphFor("zzz.somethingWithNoVerb")).toBeUndefined();
  });
});

describe("glyphFor on the About card", () => {
  it("gives the coffee and the mail button a symbol", () => {
    expect(glyphFor("about.coffeeButton")).toBeDefined();
    expect(glyphFor("about.mail")).toBeDefined();
  });

  it("keeps those two apart", () => {
    expect(glyphFor("about.coffeeButton")).not.toEqual(glyphFor("about.mail"));
  });

  it("gives the repository button no glyph", () => {
    // It wears GitHub's own mark, passed at its one call site. A pattern on
    // "repo" would put that logo on repository settings that have nothing to
    // do with GitHub.
    expect(glyphFor("about.repo")).toBeUndefined();
    expect(glyphFor("offsite.repoUrl")).toBeUndefined();
  });

  it("does not let the mail rule swallow unrelated keys", () => {
    // The rule is anchored on a dot so it takes `about.mail` and
    // `settings.mailFrom`, not every key that happens to contain the letters.
    expect(glyphFor("recovery.emailless")).toBeUndefined();
  });
});
