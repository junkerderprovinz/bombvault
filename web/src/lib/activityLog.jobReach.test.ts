// The scheduler names its jobs in Go (jobDomainFromName) and the activity log
// translates them through JOB_KEYS. A job missing from JOB_KEYS shows as a bare
// English word on a translated dashboard, and neither tsc (JOB_KEYS is a
// Record<string, string>) nor the i18n parity check can see it. This test reads
// the Go source itself, since a copied list would go stale.
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { en } from "./i18n";
import { JOB_KEYS } from "./activityLog";

const HERE = dirname(fileURLToPath(import.meta.url));
const SCHEDULE_GO = join(HERE, "..", "..", "..", "internal", "schedule", "schedule.go");

/** Every job literal jobDomainFromName can return, read out of the Go source. */
function jobNamesFromGo(): string[] {
  const src = readFileSync(SCHEDULE_GO, "utf8");
  const start = src.indexOf("func jobDomainFromName(");
  expect(start, "jobDomainFromName not found - has it been renamed?").toBeGreaterThan(-1);
  // The function ends at the first closing brace in column 0 after it, which is
  // how gofmt writes every top-level function.
  const end = src.indexOf("\n}", start);
  expect(end, "could not find the end of jobDomainFromName").toBeGreaterThan(start);
  const body = src.slice(start, end);

  // The first string of each return is the job; the second is the domain.
  const names = new Set<string>();
  for (const m of body.matchAll(/return\s+"([^"]+)"/g)) {
    names.add(m[1]);
  }
  return [...names].sort();
}

describe("schedule job names reach a translation", () => {
  it("finds the job names in the Go source at all", () => {
    const names = jobNamesFromGo();
    // An empty parse would let the other tests pass without checking anything.
    expect(names.length).toBeGreaterThanOrEqual(7);
    expect(names).toContain("backup");
    expect(names).toContain("pull");
  });

  it("gives every job name an entry in JOB_KEYS", () => {
    const missing = jobNamesFromGo().filter((n) => !JOB_KEYS[n]);
    expect(
      missing,
      `The scheduler emits ${missing.join(", ")}, and JOB_KEYS in activityLog.ts has no entry. ` +
        "jobLabel falls back to the raw string, so the dashboard's next-up line would show the " +
        "bare English identifier in all 42 languages. Add the key here and to every locale table.",
    ).toEqual([]);
  });

  it("points every JOB_KEYS entry at a string the English table actually has", () => {
    const dangling = Object.entries(JOB_KEYS)
      .filter(([, key]) => !(key in en))
      .map(([job, key]) => `${job} -> ${key}`);
    expect(
      dangling,
      `These JOB_KEYS entries name a translation key that does not exist: ${dangling.join(", ")}. ` +
        "resolveName would return the key itself, which renders as a dotted identifier on screen.",
    ).toEqual([]);
  });
});
