// Pins the en copy of the Files-page folder tree. The other locales are held to
// en by the parity and quality tests, so a reword here is a visible change.
import { describe, expect, it } from "vitest";
import { en } from "./i18n";

describe("files selection-tree copy", () => {
  it("files.foldersToggle is the row-level disclosure label", () => {
    expect(en["files.foldersToggle"]).toBe("Choose folders");
  });

  it("files.foldersHint explains ticking, exclusion and the live count", () => {
    expect(en["files.foldersHint"]).toBe(
      "Tick the folders this set covers. Untick a subfolder to leave it out; the count shows how many paths the next backup hands restic.",
    );
  });

  it("files.emptySelectionBlocked refuses the last untick and names Delete folder set", () => {
    expect(en["files.emptySelectionBlocked"]).toBe(
      "A set needs at least one folder, so the last tick cannot be removed. Use Delete folder set if you no longer want this set.",
    );
    // The tree has no reset, so the refusal names the card's own delete
    // control as the way out.
    expect(en["files.emptySelectionBlocked"]).toContain(en["files.deleteSet"]);
  });

  it("files.pathChangeHint says a folder change clears the selection", () => {
    expect(en["files.pathChangeHint"]).toBe(
      "Changing the folder clears the ticked sub-folder selection.",
    );
  });

  it("all four keys are user-visible text and carry no em dashes", () => {
    for (const key of [
      en["files.foldersToggle"],
      en["files.foldersHint"],
      en["files.emptySelectionBlocked"],
      en["files.pathChangeHint"],
    ]) {
      expect(key).not.toMatch(/[\u2013\u2014]/);
    }
  });
});
