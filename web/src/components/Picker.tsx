import type { ReactNode } from "react";
import { Badge } from "./Badge";
import { IconSearch } from "./glyphs";
import { InfoBubble } from "./InfoBubble";

/** What a picker window's search needs: its query and one name for the field. */
export interface PickerSearchProps {
  value: string;
  onChange: (next: string) => void;
  /** The field's placeholder and its accessible name, such as "Search providers". */
  label: string;
}

/**
 * PickerSearch is the field at the top right of a picker window. It is open
 * from the start, since a long list is searched rather than read, and Escape
 * empties it before it closes the window.
 */
export function PickerSearch({ value, onChange, label }: PickerSearchProps) {
  return (
    <label className="flex h-(--btn-h) w-64 items-center gap-2 rounded-control bg-carbon-surface2 px-3 glim-field-focus-within max-md:h-11 max-md:w-full">
      <span className="shrink-0 text-carbon-textMuted [&_svg]:h-4 [&_svg]:w-4">
        <IconSearch />
      </span>
      <input
        type="search"
        value={value}
        aria-label={label}
        placeholder={label}
        spellCheck={false}
        autoComplete="off"
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key !== "Escape" || value === "") return;
          e.stopPropagation();
          onChange("");
        }}
        className="h-full min-w-0 flex-1 bg-transparent text-sm text-carbon-text outline-none placeholder:text-carbon-textMuted [&::-webkit-search-cancel-button]:hidden"
      />
    </label>
  );
}

/**
 * PickerNoMatch says that a search left no tile. It stays in the page while
 * tiles are found, so a screen reader announces the sentence when it appears.
 */
export function PickerNoMatch({ shown, text }: { shown: boolean; text: string }) {
  return (
    <p role="status" className={shown ? "text-sm text-carbon-textMuted" : "sr-only"}>
      {shown ? text : ""}
    </p>
  );
}

/**
 * PickerGroup is one group of a picker's tiles under an eyebrow heading. The
 * heading can say how well the group suits the job and carry an (i).
 */
export function PickerGroup({
  title,
  fit,
  tip,
  children,
}: {
  title: string;
  fit?: { label: string; tone: "ok" | "warn" };
  tip?: string;
  children: ReactNode;
}) {
  return (
    <section className="flex flex-col gap-3">
      <h3 className="glim-eyebrow flex flex-wrap items-center gap-2">
        {title}
        {fit && (
          <Badge tone={fit.tone} size="small">
            {fit.label}
          </Badge>
        )}
        {tip && <InfoBubble tip={tip} />}
      </h3>
      {children}
    </section>
  );
}
