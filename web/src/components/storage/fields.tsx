import { useState } from "react";
import { useDebouncedSave } from "../../lib/useDebouncedSave";
import { NumberField } from "../NumberField";

const FIELD = "rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus";

/**
 * NumberSetting is a count that saves itself after a pause in typing. It shows
 * what was typed until the server's value changes, so a save that is still on
 * its way does not put the old number back under the cursor.
 */
export function NumberSetting({
  label,
  value,
  onCommit,
  disabled,
}: {
  /** The field's accessible name; the visible label stands beside it. */
  label: string;
  value: number;
  onCommit: (next: number) => void;
  disabled?: boolean;
}) {
  const [draft, setDraft] = useState(value);
  const [seen, setSeen] = useState(value);
  const { debouncedSave } = useDebouncedSave();
  if (seen !== value) {
    setSeen(value);
    setDraft(value);
  }
  return (
    <NumberField
      min={0}
      value={draft}
      aria-label={label}
      disabled={disabled}
      onChange={(e) => {
        const next = Math.max(0, parseInt(e.target.value, 10) || 0);
        setDraft(next);
        debouncedSave(() => onCommit(next));
      }}
      wrapperClassName="w-28"
      className={`${FIELD} w-full`}
    />
  );
}

/** TextSetting saves when the field is left or Enter is pressed, and only a
 *  value that changed and is not empty. */
export function TextSetting({
  label,
  value,
  onCommit,
  disabled,
}: {
  label: string;
  value: string;
  onCommit: (next: string) => void;
  disabled?: boolean;
}) {
  const [draft, setDraft] = useState(value);
  const [seen, setSeen] = useState(value);
  if (seen !== value) {
    setSeen(value);
    setDraft(value);
  }
  function commit() {
    const next = draft.trim();
    if (next && next !== value) onCommit(next);
    else setDraft(value);
  }
  return (
    <input
      value={draft}
      aria-label={label}
      disabled={disabled}
      onChange={(e) => setDraft(e.target.value)}
      onBlur={commit}
      onKeyDown={(e) => {
        if (e.key === "Enter") e.currentTarget.blur();
      }}
      className={`${FIELD} w-64 max-w-full`}
    />
  );
}
