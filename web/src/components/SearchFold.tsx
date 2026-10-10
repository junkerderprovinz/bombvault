import { useEffect, useRef, useState } from "react";
import { Button } from "./Button";
import { IconSearch } from "./glyphs";

// How long an emptied field waits before it folds back. A click on the button
// next to it blurs the field first, and folding at once would move that
// button from under the pointer.
const FOLD_DELAY_MS = 250;

/**
 * SearchFold is a search that waits as a button and opens into a field. It
 * stays open while it holds a query, and Escape clears it.
 */
export function SearchFold({
  value,
  onChange,
  label,
  placeholder,
}: {
  value: string;
  onChange: (next: string) => void;
  /** The button's words and the field's accessible name. */
  label: string;
  placeholder: string;
}) {
  const [open, setOpen] = useState(false);
  const fold = useRef<number | undefined>(undefined);

  useEffect(() => () => window.clearTimeout(fold.current), []);

  if (!open && value === "") {
    return (
      <Button
        label={label}
        labelKey={null}
        glyph={<IconSearch />}
        tone="subtle"
        onClick={() => setOpen(true)}
        className="max-md:min-h-11"
      />
    );
  }

  return (
    <label className="flex h-(--btn-h) w-64 max-md:h-11 items-center gap-2 rounded-control bg-carbon-surface2 px-3 glim-field-focus-within max-md:w-auto max-md:flex-1">
      <span className="shrink-0 text-carbon-textMuted [&_svg]:h-4 [&_svg]:w-4">
        <IconSearch />
      </span>
      <input
        type="search"
        aria-label={label}
        placeholder={placeholder}
        value={value}
        autoFocus={open}
        spellCheck={false}
        autoComplete="off"
        onChange={(e) => onChange(e.target.value)}
        onFocus={() => {
          window.clearTimeout(fold.current);
          setOpen(true);
        }}
        onBlur={() => {
          fold.current = window.setTimeout(() => setOpen(false), FOLD_DELAY_MS);
        }}
        onKeyDown={(e) => {
          if (e.key !== "Escape" || value === "") return;
          e.stopPropagation();
          onChange("");
        }}
        className="min-w-0 flex-1 bg-transparent text-sm text-carbon-text outline-none [&::-webkit-search-cancel-button]:hidden"
      />
    </label>
  );
}
