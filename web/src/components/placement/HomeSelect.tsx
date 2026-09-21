import { useEffect, useState } from "react";
import { Button } from "../Button";
import { SelectField, type SelectOption } from "../SelectField";
import { useT } from "../../lib/i18n";

/** HomeSelect picks a location as a draft; only Set hands it on, so the wheel
 *  can browse the list without writing anything. */
export function HomeSelect({
  label,
  value,
  options,
  locked,
  disabled,
  onCommit,
}: {
  label: string;
  value: string;
  options: SelectOption<string>[];
  locked: boolean;
  disabled?: boolean;
  onCommit: (value: string) => void;
}) {
  const { t } = useT();
  const [draft, setDraft] = useState(value);
  useEffect(() => {
    setDraft(value);
  }, [value]);

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
        className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus text-start"
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
