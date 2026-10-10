// What the settings pages draw, as source text, for the guards that hold the
// search index and the palette starts to it.
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const HERE = dirname(fileURLToPath(import.meta.url));
const PAGES_DIR = join(HERE, "pages");
const SECTIONS_DIR = join(HERE, "sections");
const PAGE_MAP = readFileSync(join(PAGES_DIR, "index.ts"), "utf8");

/** A page's source followed by that of each section it mounts, since a
 *  section's cards stand on the page like its own. */
function withSections(source: string): string {
  const sections = [...source.matchAll(/<([A-Z][A-Za-z]*Cards)\b/g)]
    .map(([, name]) => join(SECTIONS_DIR, `${name}.tsx`))
    .filter((file) => existsSync(file))
    .map((file) => readFileSync(file, "utf8"));
  return [source, ...sections].join("\n");
}

/** The source of the component PAGE_COMPONENTS names for a page id, with its
 *  sections. Empty when the id has no component. */
export function pageSource(page: string): string {
  const component = new RegExp(`^  ${page}: (\\w+),$`, "m").exec(PAGE_MAP)?.[1];
  return component ? withSections(readFileSync(join(PAGES_DIR, `${component}.tsx`), "utf8")) : "";
}

/** Every page component file with its sections, by name. */
export function pageFiles(): { file: string; text: string }[] {
  return readdirSync(PAGES_DIR)
    .filter((name) => name.endsWith(".tsx"))
    .map((name) => ({ file: `pages/${name}`, text: withSections(readFileSync(join(PAGES_DIR, name), "utf8")) }));
}
