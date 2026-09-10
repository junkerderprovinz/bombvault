import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { expect, it } from "vitest";

/**
 * One window, one backdrop, one darkness.
 *
 * A modal backdrop in this app is `.glim-modal-backdrop`, and that class now
 * carries BOTH halves of what a backdrop does: the fade-in it always carried,
 * and the darkness, which used to be written beside it at every call site as
 * `bg-black/60`. jdp asked for a darker one (2026-09-10: "wenn man es öffnet
 * dunkelt es den Hintergrund zu wenig ab"), and a value repeated at nine call
 * sites is standardised only until the first time somebody wants it changed.
 *
 * A test rather than a note, because counting is what caught the bug and
 * nothing else would have. Removing `bg-black/60` from all nine looked like a
 * clean sweep and would have left FIVE windows with no darkness at all: four
 * of the nine backdrops carried `.glim-modal-backdrop`, five had only the
 * literal, and the two sets were never the same set. Nobody reading one call
 * site could see that - each one is correct on its own page.
 *
 * So both halves are checked, and the second is the one with teeth: a new
 * window whose backdrop forgets the class is now a failing test rather than a
 * dialog that opens over a fully readable page.
 *
 * Matched on `fixed inset-0 z-50`, which is what makes a div a backdrop here
 * (nine of them, all written that way). It reads string and template literals
 * rather than lines, the same way hoverRamp.test.ts does and for the same
 * reason: neighbouring lines belong to different elements.
 */

const here = dirname(fileURLToPath(import.meta.url));
const src = join(here, "..");

/** What marks a div as a modal backdrop in this codebase. */
const BACKDROP = "fixed inset-0 z-50";

function tsxFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((entry) => {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) return entry === "node_modules" ? [] : tsxFiles(full);
    if (!full.endsWith(".tsx") || full.includes(".test.")) return [];
    return [full];
  });
}

/** Every quoted string and template chunk in a file, without its quotes. */
function literals(source: string): string[] {
  return (source.match(/"[^"\n]*"|'[^'\n]*'|`[^`]*`/g) ?? []).map((raw) => raw.slice(1, -1));
}

const backdrops = tsxFiles(src).flatMap((file) =>
  literals(readFileSync(file, "utf8"))
    .filter((text) => text.includes(BACKDROP))
    .map((text) => ({ where: relative(src, file), text }))
);

it("finds the backdrops at all", () => {
  // The guard below passes vacuously if the search stops matching - a rename
  // of the shared class list would silently switch this whole file off.
  expect(backdrops.length).toBeGreaterThanOrEqual(9);
});

it("gives every modal backdrop the class that darkens it", () => {
  for (const { where, text } of backdrops) {
    expect(text, where).toContain("glim-modal-backdrop");
  }
});

it("keeps the darkness in the class, not beside it", () => {
  // A second value on the element wins or loses against the class depending on
  // which one Tailwind emits last, so two windows would darken differently
  // and only one of them would answer to index.css.
  for (const { where, text } of backdrops) {
    expect(text, where).not.toMatch(/bg-black\//);
  }
});
