import { useT } from "../lib/i18n";
import { CadenceBuilder, formatCadence, type CadenceMode } from "./CadenceBuilder";
import { TapPopover } from "./mobile/TapPopover";
import { IconSchedules } from "./navGlyphs";
import { SelectArrow } from "./SelectField";

/**
 * PlanButton shows a stored schedule as one sentence on a button. The editor
 * opens beside it only when the schedule is to change, so a page of schedules
 * reads as a list of sentences.
 */
export function PlanButton({
  label,
  value,
  onChange,
  modes,
  hueIndex,
  disabled,
  offText,
}: {
  /** What the schedule is for. It names the editor and goes before the
   *  sentence for a screen reader. */
  label: string;
  value: string;
  onChange: (next: string) => void;
  modes?: CadenceMode[];
  hueIndex?: number;
  disabled?: boolean;
  /** The sentence for an empty value, where empty means something other than
   *  off. */
  offText?: string;
}) {
  const { t, lang } = useT();
  const text = formatCadence(value, t, lang) || (value.trim() === "" && offText ? offText : t("cadence.off"));
  return (
    <TapPopover
      label={label}
      touchFloor={false}
      disabled={disabled}
      trigger={
        <>
          <span className="inline-flex shrink-0 text-accentText">
            <IconSchedules />
          </span>
          <span className="sr-only">{label}: </span>
          <span className="block min-w-0 truncate first-letter:uppercase">{text}</span>
          <SelectArrow />
        </>
      }
      triggerClassName="glim-btn min-w-0 max-w-full gap-2 bg-carbon-surface2 px-3 text-carbon-text hover:bg-carbon-surface3"
      panelClassName="p-4 max-w-[min(94vw,36rem)]"
    >
      <CadenceBuilder label={label} value={value} onChange={onChange} modes={modes} hueIndex={hueIndex} />
    </TapPopover>
  );
}
