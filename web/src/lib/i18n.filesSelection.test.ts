/**
 * Files-page selection-tree copy contract (Phase 4, INTEG-02).
 *
 * The en copy below is the UI-SPEC source of truth for the Files-page tree
 * editor that plans 04-03 (page mount) and 04-04 (dialog disclosure) render.
 * This file pins that copy BEFORE the renderer exists, for three reasons:
 *
 * 1. The i18n orphan gate (i18n.orphans.test.ts) refuses any en key that
 *    nothing names anywhere in src; its own doc says "a key some test still
 *    names counts as used". The keys land in the same task as the locale
 *    fan-out (interface-first), one task ahead of their first renderer, so
 *    the contract test is the honest reference that keeps the gate truthful
 *    instead of a mechanical 't("files.foldersToggle")' no-op.
 * 2. Pinning the exact en string makes copy drift a compile-checked, test
 *    -surfaced event rather than a silent reword. The 40 lazy locale chunks
 *    are pinned value-for-value by i18n.parity/quality gates against en;
 *    this file is the anchor they all ultimately hang from.
 * 3. The wording is load-bearing, not decorative: the empty-selection copy
 *    orients to the card's "Remove set" action (D-06 - the Files tree has no
 *    Reset and no auto-detection fallback, so unlike the folders.* keys the
 *    refusal must name the real exit), and the path-change copy discloses
 *    the PATCH-time clear rule from plan 04-02 (A3) before it happens.
 */
import { describe, expect, it } from "vitest";
import { en } from "./i18n";

// The en table is flat with dotted keys (typed via TranslationKey), so the
// bracket form below is the codebase's access style for programmatic pins.
describe("files selection-tree copy contract (UI-SPEC source of truth)", () => {
  it("files.foldersToggle is the row-level disclosure label", () => {
    expect(en["files.foldersToggle"]).toBe("Choose folders");
  });

  it("files.foldersHint explains ticking, exclusion and the live count", () => {
    expect(en["files.foldersHint"]).toBe(
      "Tick the folders this set covers. Untick a subfolder to leave it out; the count shows how many paths the next backup hands restic.",
    );
  });

  it("files.emptySelectionBlocked refuses the last untick and orients to Remove set (D-06)", () => {
    expect(en["files.emptySelectionBlocked"]).toBe(
      "A set needs at least one folder, so the last tick cannot be removed. Use Remove set if you no longer want this set.",
    );
    // The refusal must name the actual exit action; it is the card's own
    // delete control, so the copy embeds the en files.deleteSet label.
    expect(en["files.emptySelectionBlocked"]).toContain(en["files.deleteSet"]);
  });

  it("files.pathChangeHint discloses the PATCH-time clear rule (04-02 A3)", () => {
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
      expect(key).not.toMatch(/—|–/);
    }
  });
});
