import { useState } from "react";
import { useDebouncedSave } from "../../lib/useDebouncedSave";
import { NumberField } from "../NumberField";
import { RateField } from "../RateField";

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
  rate,
}: {
  /** The field's accessible name; the visible label stands beside it. */
  label: string;
  value: number;
  onCommit: (next: number) => void;
  disabled?: boolean;
  /** The count is a rate in KiB/s, shown in MB/s and Mbit/s as well. */
  rate?: boolean;
}) {
  const Field = rate ? RateField : NumberField;
  const [draft, setDraft] = useState(value);
  const [seen, setSeen] = useState(value);
  const { debouncedSave } = useDebouncedSave();
  if (seen !== value) {
    setSeen(value);
    setDraft(value);
  }
  return (
    <Field
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

/** The count typed into a CountSetting: 0 for an empty field, undefined for
 *  one that is no whole number from 1 to max. */
export function typedCount(text: string, max: number): number | undefined {
  if (text.trim() === "") return 0;
  const n = Number(text);
  return Number.isInteger(n) && n >= 1 && n <= max ? n : undefined;
}

/**
 * CountSetting is a count from 1 to max that may also be left empty, which
 * saves 0. A number outside the range is marked and not saved.
 */
export function CountSetting({
  label,
  value,
  max,
  placeholder,
  onCommit,
  disabled,
}: {
  label: string;
  value: number;
  max: number;
  /** What an empty field stands for. */
  placeholder: string;
  onCommit: (next: number) => void;
  disabled?: boolean;
}) {
  const shown = value > 0 ? String(value) : "";
  const [draft, setDraft] = useState(shown);
  const [seen, setSeen] = useState(value);
  const { debouncedSave } = useDebouncedSave();
  if (seen !== value) {
    setSeen(value);
    setDraft(shown);
  }
  const valid = typedCount(draft, max) !== undefined;
  return (
    <NumberField
      min={1}
      max={max}
      value={draft}
      placeholder={placeholder}
      aria-label={label}
      aria-invalid={valid ? undefined : true}
      disabled={disabled}
      onChange={(e) => {
        setDraft(e.target.value);
        const next = typedCount(e.target.value, max);
        if (next !== undefined) debouncedSave(() => onCommit(next));
      }}
      wrapperClassName="w-28"
      className={`${FIELD} w-full border ${valid ? "border-transparent" : "border-statusFail"}`}
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
