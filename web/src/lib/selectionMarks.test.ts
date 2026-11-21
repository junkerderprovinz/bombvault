// A selection is marked the same way everywhere: the picked control turns into
// the accent. No component marks it with a ring, frame or bar in the accent.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const SRC = fileURLToPath(new URL("..", import.meta.url));

const MARKS = [/\bring-accent\w*/g, /\bborder-accent(?=[\s"'`])/g, /shadow-\[inset_0_0_0_[\d.]+px_var\(--accent(?:-text)?\)\]/g];

// What draws in the accent without marking a pick: the legend swatch of the
// learning rings, which is a ring in the picture it explains, the drop zone
// while a file is held over it, and the frame of the camera's view while a
// code is scanned.
const ALLOWED: Record<string, string[]> = {
  "components/anomalies/LearningRings.tsx": ["shadow-[inset_0_0_0_2.5px_var(--accent)]"],
  "components/restore/KitPicker.tsx": ["border-accent"],
  "launcher/QRScanner.tsx": ["border-accent"],
};

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return sources(path);
    return /\.tsx$/.test(name) && !/\.test\.tsx$/.test(name) ? [path] : [];
  });
}

describe("selection marks", () => {
  it("leave no ring, frame or bar in the accent on a component", () => {
    const found = sources(SRC).flatMap((path) => {
      const file = relative(SRC, path).replaceAll("\\", "/");
      const text = readFileSync(path, "utf8");
      return MARKS.flatMap((mark) => [...text.matchAll(mark)].map((m) => m[0]))
        .filter((hit) => !(ALLOWED[file] ?? []).includes(hit))
        .map((hit) => `${file}: ${hit}`);
    });
    expect(found).toEqual([]);
  });
});
