import { useRef, useState, type ReactNode } from "react";
import { useT } from "../lib/i18n";
import { Button } from "./Button";
import { DropdownListbox } from "./DropdownListbox";
import { IconAdd, IconTrash } from "./navGlyphs";

export interface ExceptionSection {
  id: string;
  label: string;
  glyph: ReactNode;
}

/**
 * Exceptions lists only the sections that differ from a shared rule, each
 * with its own value, and ends in the rule every other section follows. A
 * section joins the list through the Exception button and leaves it through
 * its bin, which gives it back to the rule.
 */
export function Exceptions({
  label,
  taken,
  free,
  rule,
  control,
  onAdd,
  onRemove,
  disabled,
}: {
  /** What the list sets, as the name of the list of sections to add. */
  label: string;
  /** The sections with a value of their own, in the order they show. */
  taken: ExceptionSection[];
  /** The sections that follow the rule and can still be added. */
  free: ExceptionSection[];
  /** The sentence for every section not listed. */
  rule: string;
  control: (id: string) => ReactNode;
  onAdd: (id: string) => void;
  onRemove: (id: string) => void;
  disabled?: boolean;
}) {
  const { t } = useT();
  const [open, setOpen] = useState(false);
  const addRef = useRef<HTMLButtonElement>(null);

  return (
    <div className="flex flex-col">
      {taken.map((section) => (
        <div
          key={section.id}
          data-exception={section.id}
          className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 border-b border-carbon-border/60 py-3 first:pt-0"
        >
          <span className="min-w-0 text-sm text-carbon-text wrap-break-word">{section.label}</span>
          <div className="flex min-w-0 max-w-full flex-wrap items-center justify-end gap-2">
            {control(section.id)}
            <Button
              label={`${t("exceptions.remove")}: ${section.label}`}
              labelKey="exceptions.remove"
              glyph={<IconTrash />}
              variant="icon"
              glyphOnly
              disabled={disabled}
              onClick={() => onRemove(section.id)}
            />
          </div>
        </div>
      ))}
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 pt-3 first:pt-0">
        <span className="min-w-0 text-xs text-carbon-textMuted wrap-break-word">{rule}</span>
        {free.length > 0 && (
          <Button
            ref={addRef}
            label={t("exceptions.add")}
            labelKey="exceptions.add"
            glyph={<IconAdd />}
            disabled={disabled}
            ariaExpanded={open}
            onClick={() => setOpen((v) => !v)}
          />
        )}
      </div>
      <DropdownListbox open={open} onClose={() => setOpen(false)} triggerRef={addRef} label={label}>
        {free.map((section) => (
          <button
            key={section.id}
            type="button"
            role="option"
            aria-selected={false}
            onClick={() => {
              setOpen(false);
              onAdd(section.id);
            }}
            className="flex w-full items-center gap-2.5 px-3 py-2 text-start text-sm text-carbon-textSub transition-colors hover:bg-carbon-hover hover:text-carbon-text"
          >
            <span className="inline-flex shrink-0 text-accentText">{section.glyph}</span>
            <span className="truncate">{section.label}</span>
          </button>
        ))}
      </DropdownListbox>
    </div>
  );
}
