// What the Backups list promises about its own shape. One layout for every
// width, one bar above the rows, a list that grows only on request and a
// folder tree that asks the pointer what it is: each of these is a way of
// writing the page, and a page written another way still renders and still
// passes every test of what it does. So this reads the source.
import { readdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

const HERE = dirname(fileURLToPath(import.meta.url));
const COMPONENTS = join(HERE, "..", "components");

function sources(dir: string): Record<string, string> {
  return Object.fromEntries(
    readdirSync(dir)
      .filter((f) => f.endsWith(".tsx") && !f.endsWith(".test.tsx"))
      .map((f) => [f, readFileSync(join(dir, f), "utf8")])
  );
}

const page = readFileSync(join(HERE, "Backups.tsx"), "utf8");
const parts = sources(join(COMPONENTS, "backups"));
const everything = [page, ...Object.values(parts)].join("\n");
const editors = { ...sources(join(COMPONENTS, "containers")), ...sources(join(COMPONENTS, "files")) };

describe("the list renders one layout", () => {
  it("reads the page and its parts", () => {
    expect(page).toContain("export function Backups");
    expect(Object.keys(parts)).toEqual(expect.arrayContaining(["BackupRow.tsx", "BulkBar.tsx", "ListBar.tsx"]));
  });

  // A block hidden by a stylesheet is a second copy of every row in the DOM,
  // with state that can drift from the copy in view.
  it.each(["max-md:hidden", "md:hidden"])("hides nothing by width with %s", (needle) => {
    const found = [["Backups.tsx", page], ...Object.entries(parts)].filter(([, src]) => src.includes(needle)).map(([f]) => f);
    expect(found, `a phone and a desktop copy of the same block: ${found.join(", ")}`).toEqual([]);
  });

  it("stands on the responsive page shell", () => {
    expect(page).toContain('import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";');
    expect(page).toContain("<div className={PAGE_SHELL_RESPONSIVE}>");
  });
});

describe("the bar above the rows", () => {
  it("is the one ListBar, at every width", () => {
    expect(page.match(/<ListBar\b/g)).toHaveLength(1);
  });

  it("searches through the shared fold, with a translated placeholder", () => {
    expect(parts["ListBar.tsx"]).toContain("<SearchFold");
    expect(parts["ListBar.tsx"]).toMatch(/placeholder=\{t\("[\w.]+"\)\}/);
    expect(everything).not.toMatch(/type="search"/);
  });

  // The bar of picked rows is the only thing that stays in view while the
  // list scrolls. A second sticky strip would stack under it on a phone.
  it("leaves staying in view to the bar of picked rows", () => {
    const sticky = [["Backups.tsx", page], ...Object.entries(parts)]
      .filter(([, src]) => /\bsticky\b/.test(src))
      .map(([f]) => f);
    expect(sticky).toEqual(["BulkBar.tsx"]);
  });
});

describe("the list grows only on request", () => {
  it("windows the rows with useLoadMore", () => {
    expect(page).toMatch(/const \{ visible, showMore, hasMore \} = useLoadMore\(shown,/);
  });

  it("has nothing that loads on scroll and no window of its own", () => {
    expect(everything).not.toContain("IntersectionObserver");
    for (const [file, src] of [
      ["Backups.tsx", page],
      ["BackupRow.tsx", parts["BackupRow.tsx"]],
    ]) {
      expect(src, `${file} listens for scrolling`).not.toContain('addEventListener("scroll"');
      expect(src, `${file} cuts the list by hand`).not.toContain("slice(0,");
    }
  });
});

describe("the folder tree's interaction mode", () => {
  // A narrow desktop window has a mouse and a large tablet has a finger, so
  // the width answers wrongly in both directions.
  it("follows the pointer", () => {
    const tree = editors["FoldersEditor.tsx"];
    expect(tree).toContain("const coarsePointer = useIsCoarsePointer();");
    expect(tree).toContain('interactionMode={coarsePointer ? "touch" : "pointer"}');
  });

  it("never follows the width", () => {
    const byWidth = Object.entries(editors)
      .filter(([, src]) => /interactionMode=\{[^}]*isDesktop/.test(src))
      .map(([f]) => f);
    expect(byWidth).toEqual([]);
  });
});
