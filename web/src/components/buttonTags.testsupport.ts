// Finds every `<Button …>` opening tag in the source tree, with its file, line
// and raw props, for the guards that check call sites. It scans source text
// rather than an AST, because the guards ask about the code at every call site
// and a regex answers that without a parser dependency.
import { readSource, walkTsx } from "./sourceTree.testsupport";

export type ButtonTag = {
  file: string;
  line: number;
  props: string;
  /** Offset of the tag's `<`, and of the character after the `>` that closes
   *  the opening tag, so a caller can tell whether two buttons are siblings. */
  start: number;
  end: number;
  /** The file's full text. */
  source: string;
};

export function buttonTags(src: string): ButtonTag[] {
  const out: ButtonTag[] = [];
  for (const file of walkTsx(src)) {
    const s = readSource(file);
    const re = /<Button\b/g;
    let m: RegExpExecArray | null;
    while ((m = re.exec(s))) {
      // Skip any '>' inside the braces of a prop value, such as an arrow
      // function.
      let depth = 0;
      let i = re.lastIndex;
      let end = -1;
      for (; i < s.length; i++) {
        const c = s[i];
        if (c === "{") depth++;
        else if (c === "}") depth--;
        else if (c === ">" && depth === 0) {
          end = i;
          break;
        }
      }
      if (end < 0) continue;
      out.push({
        file: file.slice(src.length + 1).replace(/\\/g, "/"),
        line: s.slice(0, m.index).split("\n").length,
        props: s.slice(re.lastIndex, end),
        start: m.index,
        end: end + 1,
        source: s,
      });
    }
  }
  return out;
}
