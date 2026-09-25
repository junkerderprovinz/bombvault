import { useState } from "react";
import { Button } from "../Button";
import { SelectField, type SelectOption } from "../SelectField";
import { useT } from "../../lib/i18n";

/** HomeSelect picks a location as a draft; only Set hands it on, so the wheel
 *  can browse the list without writing anything. A stored value that changes
 *  underneath (placement refreshes live) only pulls the draft along while the
 *  user has not diverged from it; a pending pick survives, and Set then meets
 *  the save route's own stale check rather than being silently overwritten. */
export function HomeSelect({
  label,
  value,
  options,
  locked,
  disabled,
  well = false,
  onCommit,
}: {
  label: string;
  value: string;
  options: SelectOption<string>[];
  locked: boolean;
  disabled?: boolean;
  /** The field sits in a surface2 well, where a surface2 fill would vanish. */
  well?: boolean;
  onCommit: (value: string) => void;
}) {
  const { t } = useT();
  const [draft, setDraft] = useState(value);
  const [synced, setSynced] = useState(value);
  if (value !== synced) {
    setSynced(value);
    if (draft === synced) setDraft(value);
  }

  if (locked || options.length < 2) {
    return (
      <div className="flex items-center gap-2 flex-wrap text-xs">
        <span className="text-carbon-textSub">{label}</span>
        <span className="text-carbon-text">{options.find((o) => o.value === value)?.label ?? value}</span>
        {locked && <span className="text-carbon-textMuted">{t("placement.fixedSinceFirst")}</span>}
      </div>
    );
  }
  return (
    <div className="flex items-center gap-2 flex-wrap text-xs">
      <span className="text-carbon-textSub">{label}</span>
      <SelectField
        value={draft}
        onChange={setDraft}
        options={options}
        label={label}
        disabled={disabled}
        className={`rounded-control text-carbon-text text-sm px-3 py-1.5 text-start ${
          well ? "bg-carbon-surface3 glim-field-focus-well" : "bg-carbon-surface2 glim-field-focus"
        }`}
      />
      {draft !== value && (
        <Button
          label={t("placement.saveHome")}
          labelKey="placement.saveHome"
          tone="accent"
          disabled={disabled}
          onClick={() => onCommit(draft)}
        />
      )}
    </div>
  );
}
