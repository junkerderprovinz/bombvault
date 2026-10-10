// What the settings pages draw, as source text, for the guards that hold the
// search index and the palette starts to it.
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const HERE = dirname(fileURLToPath(import.meta.url));
const PAGES_DIR = join(HERE, "pages");
const ROUTED = readFileSync(join(HERE, "..", "Settings.tsx"), "utf8");

/** Every `{page === "X" && …}` block of Settings.tsx, brace-matched to its close. */
function gated(page: string): string {
  const needle = `{page === "${page}" &&`;
  let out = "";
  for (let i = ROUTED.indexOf(needle); i !== -1; i = ROUTED.indexOf(needle, i + 1)) {
    let depth = 0;
    for (let j = i; j < ROUTED.length; j++) {
      if (ROUTED[j] === "{") depth++;
      else if (ROUTED[j] === "}" && --depth === 0) {
        out += ROUTED.slice(i, j + 1);
        break;
      }
    }
  }
  return out;
}

/** The source behind a page id: its gates in Settings.tsx and the page
 *  components those mount. Empty when nothing draws the page. */
export function pageSource(page: string): string {
  const gates = gated(page);
  const mounted = [...gates.matchAll(/<([A-Z][A-Za-z]*Page)\b/g)]
    .map(([, name]) => join(PAGES_DIR, `${name}.tsx`))
    .filter(existsSync)
    .map((file) => readFileSync(file, "utf8"));
  return [gates, ...mounted].join("\n");
}

/** Every page component file, by name. */
export function pageFiles(): { file: string; text: string }[] {
  return readdirSync(PAGES_DIR)
    .filter((name) => name.endsWith(".tsx"))
    .map((name) => ({ file: `pages/${name}`, text: readFileSync(join(PAGES_DIR, name), "utf8") }));
}
