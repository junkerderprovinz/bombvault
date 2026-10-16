// internal/api/widget.html is a standalone English page served by the Go
// binary, so it carries its own copy of the log's line shapes and labels. A
// run kind it has no case for falls through to "backed up". This test reads
// both sources, since a copied list would go stale.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { en } from "./i18n";
import { domainLabel, JOB_KEYS } from "./activityLog";
import { KEY_RUN_KIND } from "./runKind";

const HERE = dirname(fileURLToPath(import.meta.url));
const ACTIVITY_LOG = join(HERE, "activityLog.ts");
const WIDGET = join(HERE, "..", "..", "..", "internal", "api", "widget.html");

const english = (key: string) => (en as Record<string, string>)[key] ?? key;

/** Every run kind finishedLineText gives a line of its own. */
function lineKinds(): string[] {
  const src = readFileSync(ACTIVITY_LOG, "utf8");
  const start = src.indexOf("function finishedLineText(");
  expect(start, "finishedLineText not found - has it been renamed?").toBeGreaterThan(-1);
  // The return type closes with "} {", the body with a brace on its own line.
  const end = src.indexOf("\n}\n", start);
  const kinds = new Set<string>();
  for (const m of src.slice(start, end).matchAll(/run\.kind === "([a-z]+)"/g)) {
    kinds.add(m[1]);
  }
  return [...kinds].sort();
}

describe("the widget knows every line the activity log knows", () => {
  const page = readFileSync(WIDGET, "utf8");

  it("finds the run kinds in activityLog.ts at all", () => {
    const kinds = lineKinds();
    expect(kinds.length).toBeGreaterThanOrEqual(10);
    expect(kinds).toContain("replica");
  });

  it("gives every run kind a case of its own", () => {
    const missing = lineKinds().filter((k) => !page.includes(`case "${k}":`));
    expect(missing, `widget.html would show ${missing.join(", ")} as a backup`).toEqual([]);
  });

  it("names every key event the way the log does", () => {
    const missing = Object.entries(KEY_RUN_KIND)
      .filter(([kind, key]) => !page.includes(`${kind}: "${english(key)}"`))
      .map(([kind]) => kind);
    expect(missing).toEqual([]);
  });

  it("names every scheduled job the way the log does", () => {
    const missing = Object.entries(JOB_KEYS)
      .filter(([job, key]) => !page.includes(`${job}: "${english(key)}"`))
      .map(([job]) => job);
    expect(missing).toEqual([]);
  });

  it("names every domain the way the log does", () => {
    const missing = ["containers", "vms", "flash", "config", "files", "zfs", "everything"].filter(
      (d) => !page.includes(`${d}: "${domainLabel(english, d)}"`),
    );
    expect(missing).toEqual([]);
  });
});
