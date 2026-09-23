// A control that presses does it by the motion engine's number, never its own.
//
// `--motion-press-scale` is .97 at the lively default, .94 at storm, .99 at
// subtle and 1 at off, so a literal `active:scale-[.97]` keeps pressing at the
// level that means no movement at all. Nothing catches that: the class is
// valid Tailwind, the element renders, and the setting simply does not reach
// it. index.css gives `.glim-btn` the token; a component that writes its own
// utility has to read the same one.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..");

function sources(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir)) {
    if (entry === "node_modules" || entry === "dist" || entry === "locales") continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      out.push(...sources(full));
      continue;
    }
    if (/\.tsx?$/.test(entry) && !/\.test\.tsx?$/.test(entry)) out.push(full);
  }
  return out;
}

// Comments are stripped because they quote the literal form when they explain
// why it is wrong.
function code(text: string): string {
  return text.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/(^|[^:\w])\/\/[^\n]*/g, "$1");
}

const PRESS = /(?:active|group-active):scale-\[([^\]]*)\]/g;

describe("press scale", () => {
  const files = sources(SRC);

  it("comes from the motion token, never from a literal", () => {
    const offenders: string[] = [];
    for (const file of files) {
      const text = code(readFileSync(file, "utf8"));
      for (const m of text.matchAll(PRESS)) {
        if (m[1].includes("--motion-press-scale")) continue;
        const line = text.slice(0, m.index).split("\n").length;
        offenders.push(
          `${relative(SRC, file)}:${line} - scale-[${m[1]}] presses at every level, ` +
            `including off. Read var(--motion-press-scale) instead.`
        );
      }
    }
    expect(offenders).toEqual([]);
  });

  it("finds the call sites at all, so the scan cannot pass on a rename", () => {
    const total = files.reduce(
      (n, f) => n + [...code(readFileSync(f, "utf8")).matchAll(PRESS)].length,
      0
    );
    expect(total).toBeGreaterThanOrEqual(1);
  });
});
