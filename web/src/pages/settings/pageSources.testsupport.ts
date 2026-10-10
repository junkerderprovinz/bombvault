// What the settings pages draw, as source text, for the guards that hold the
// search index and the palette starts to it.
import { readdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const PAGES_DIR = join(dirname(fileURLToPath(import.meta.url)), "pages");
const PAGE_MAP = readFileSync(join(PAGES_DIR, "index.ts"), "utf8");

/** The source of the component PAGE_COMPONENTS names for a page id. Empty
 *  when the id has no component. */
export function pageSource(page: string): string {
  const component = new RegExp(`^  ${page}: (\\w+),$`, "m").exec(PAGE_MAP)?.[1];
  return component ? readFileSync(join(PAGES_DIR, `${component}.tsx`), "utf8") : "";
}

/** Every page component file, by name. */
export function pageFiles(): { file: string; text: string }[] {
  return readdirSync(PAGES_DIR)
    .filter((name) => name.endsWith(".tsx"))
    .map((name) => ({ file: `pages/${name}`, text: readFileSync(join(PAGES_DIR, name), "utf8") }));
}
