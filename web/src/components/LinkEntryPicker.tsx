import { useEffect, useRef, useState } from "react";
import type { useT } from "../lib/i18n";
import { countBackups, useTakeOver, type TakeoverEntry } from "../lib/useTakeOver";
import { Button } from "./Button";
import { InfoBubble } from "./InfoBubble";
import { SelectField } from "./SelectField";

type T = ReturnType<typeof useT>["t"];

export function LinkEntryPicker({
  candidates,
  entry,
  onDone,
  t,
}: {
  /** The not-installed entries of the same domain, by the names their routes take. */
  candidates: string[];
  entry: TakeoverEntry;
  onDone: () => void;
  t: T;
}) {
  const [open, setOpen] = useState(false);
  const [choice, setChoice] = useState(candidates[0]);
  const [counting, setCounting] = useState(false);
  const [shake, setShake] = useState(0);
  const { takeOver, busy, confirmDialog } = useTakeOver(entry, onDone, t);
  const openerRef = useRef<HTMLButtonElement>(null);
  const selectRef = useRef<HTMLButtonElement>(null);
  const wasOpen = useRef(false);
  // A reload can take the chosen entry out of the list while the picker is open.
  const selected = candidates.includes(choice) ? choice : candidates[0];

  // The select takes focus the moment the picker opens. Closing it by hand
  // returns focus to the button that opened it, rather than leaving it
  // wherever the picker's own markup last put it.
  useEffect(() => {
    if (open) {
      selectRef.current?.focus();
    } else if (wasOpen.current) {
      openerRef.current?.focus();
    }
    wasOpen.current = open;
  }, [open]);

  async function submit() {
    setCounting(true);
    const backups = await countBackups(entry.api, selected);
    setCounting(false);
    await takeOver(selected, backups, () => setShake((n) => n + 1));
  }

  if (!open) {
    return (
      <Button
        ref={openerRef}
        label={t("takeover.link")}
        labelKey="takeover.link"
        tone="neutral"
        onClick={() => setOpen(true)}
      />
    );
  }

  return (
    <div className="flex min-w-0 flex-col gap-2 rounded-card bg-carbon-background p-3">
      <span className="flex items-center gap-1 text-xs text-carbon-textSub">
        {t("takeover.formerEntry")}
        <InfoBubble tip={t("takeover.linkHint")} />
      </span>
      <SelectField
        ref={selectRef}
        value={selected}
        onChange={setChoice}
        options={candidates.map((name) => ({ value: name, label: name }))}
        label={t("takeover.formerEntry")}
        className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus text-start"
      />
      <div className="flex items-center justify-end gap-2">
        <Button label={t("common.cancel")} labelKey="common.cancel" tone="neutral" onClick={() => setOpen(false)} />
        <Button
          key={shake}
          label={t("takeover.accept")}
          labelKey="takeover.accept"
          tone="accent"
          onClick={() => void submit()}
          disabled={busy || counting}
          busy={busy || counting}
          className={shake ? "glim-shake" : ""}
        />
      </div>
      {confirmDialog}
    </div>
  );
}
