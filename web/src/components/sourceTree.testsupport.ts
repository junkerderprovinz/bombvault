// The source tree as the guards read it. Several guards scan every file, some
// more than once, so each directory is walked and each file read once per test
// file; a loaded machine otherwise brings the scans close to vitest's timeout.
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";

const walks = new Map<string, readonly string[]>();

/** Every file under `dir`, recursively, in directory order. */
export function walk(dir: string): readonly string[] {
  let files = walks.get(dir);
  if (!files) {
    files = readdirSync(dir).flatMap((entry) => {
      const full = join(dir, entry);
      return statSync(full).isDirectory() ? walk(full) : [full];
    });
    walks.set(dir, files);
  }
  return files;
}

/** Every non-test .tsx under `dir`. */
export function walkTsx(dir: string): string[] {
  return walk(dir).filter((file) => /\.tsx$/.test(file) && !/\.test\.tsx$/.test(file));
}

const texts = new Map<string, string>();

export function readSource(path: string): string {
  let text = texts.get(path);
  if (text === undefined) {
    text = readFileSync(path, "utf8");
    texts.set(path, text);
  }
  return text;
}

/** Blanks keep the line numbers of what is left. */
export const blank = (m: string) => m.replace(/[^\n]/g, " ");

/** Comments quote what they warn against, so the guards read the source with
 *  them blanked. `//` after a colon or a quote is a URL or a string. */
export function blankComments(source: string): string {
  return source
    .replace(/\/\*[\s\S]*?\*\//g, blank)
    .replace(/(^|[^:"'`\\])(\/\/.*)$/gm, (_m, before: string, comment: string) => before + blank(comment));
}
