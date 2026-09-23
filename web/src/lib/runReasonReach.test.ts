// A run reason on screen has to isolate the parts the host owns: the tool's own
// message and the names of the apps an import left stopped. RunReasonText wraps
// both in a bdi, the plain runReason string cannot, and an Arabic or Hebrew page
// then reorders a container name and drags its underscores across the sentence.
// Each page looks right on its own, so only a scan finds one that renders the
// string instead of the component.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..");

function components(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir)) {
    const path = join(dir, entry);
    if (statSync(path).isDirectory()) {
      out.push(...components(path));
    } else if (/\.tsx$/.test(entry) && !/\.test\.tsx$/.test(entry)) {
      out.push(path);
    }
  }
  return out;
}

describe("a run reason in the markup", () => {
  const files = components(SRC);

  it("finds the components at all, so an empty scan cannot pass", () => {
    expect(files.length).toBeGreaterThan(20);
  });

  it.each(files.map((f) => [f.slice(SRC.length + 1), f]))("%s renders it through RunReasonText", (_name, path) => {
    const src = readFileSync(path, "utf8");
    expect(src, "renders runReason() as a string, which leaves the names the host owns unisolated").not.toMatch(
      /\{\s*runReason\(/,
    );
  });
});
