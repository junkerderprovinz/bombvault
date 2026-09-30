// WordSlots lays out the twelve words in numbered tiles, the same wherever
// they appear: shown to be read out, or filling as somebody types them.
import type { ReactNode } from "react";
import { PHRASE_WORDS } from "../../../lib/phraseWords";

export function WordSlots({
  words,
  unknown = [],
  label,
}: {
  words: string[];
  /** Positions, from 0, of words not on the list. */
  unknown?: number[];
  /** The list's accessible name; without one the slots are only decoration
   *  beside a field that already says it all. */
  label?: string;
}) {
  const bad = new Set(unknown);
  const slots: ReactNode[] = [];
  for (let i = 0; i < PHRASE_WORDS; i++) {
    const w = words[i];
    const tone =
      w === undefined
        ? "ring-1 ring-inset ring-carbon-border text-carbon-textMuted"
        : bad.has(i)
          ? "bg-statusFailBg text-statusFail"
          : "bg-carbon-surface2 text-carbon-text";
    slots.push(
      <li
        key={i}
        data-slot={i + 1}
        data-unknown={bad.has(i) ? "" : undefined}
        className={`flex min-w-0 items-center gap-2 rounded-control px-3 py-1.5 text-sm ${tone}`}
      >
        <span className={`w-4.5 shrink-0 text-end text-xs tabular-nums ${bad.has(i) ? "" : "text-carbon-textMuted"}`}>{i + 1}</span>
        <span dir="ltr" className="truncate font-mono">
          {w ?? "·"}
        </span>
      </li>,
    );
  }
  return (
    <ol className="grid grid-cols-2 gap-2 sm:grid-cols-4" aria-label={label} aria-hidden={label ? undefined : true}>
      {slots}
    </ol>
  );
}
