import { useId } from "react";
import type { ZFSReplicaKeep, ZFSReplicaKeepPreset } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { NumberField } from "../../NumberField";
import { Selector } from "../../Selector";
import { SettingRow } from "../../storage/rows";
import { KEEP_FIELD_KEYS, KEEP_PRESET_KEY, keepSummary, withOwnCount } from "./replicaModel";

const PRESETS: ZFSReplicaKeepPreset[] = ["short", "balanced", "long", "own"];

const COUNT = "w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus";

/** ReplicaKeepField picks how many snapshots a replica keeps: a preset, or
 *  five numbers of its own. It renders rows for a Rows list. */
export function ReplicaKeepField({
  label,
  hint,
  keep,
  onChange,
  disabled,
}: {
  label: string;
  hint: string;
  keep: ZFSReplicaKeep;
  onChange: (next: ZFSReplicaKeep) => void;
  disabled?: boolean;
}) {
  const { t } = useT();
  const id = useId();
  return (
    <>
      <SettingRow label={label} hint={hint} note={keepSummary(t, keep)}>
        <Selector
          label={label}
          items={PRESETS.map((p) => ({ id: p, label: t(KEEP_PRESET_KEY[p]) }))}
          active={keep.preset}
          onChange={(id) => onChange({ ...keep, preset: id as ZFSReplicaKeepPreset })}
          activation="manual"
          disabled={disabled}
          inline
        />
      </SettingRow>
      {keep.preset === "own" &&
        KEEP_FIELD_KEYS.map((key, i) => (
          <SettingRow key={key} label={t(key)} htmlFor={`${id}-${i}`} narrow>
            <NumberField
              id={`${id}-${i}`}
              wrapperClassName="w-28"
              min={0}
              value={keep.own[i]}
              disabled={disabled}
              onChange={(e) => onChange({ ...keep, own: withOwnCount(keep.own, i, e.target.value) })}
              className={COUNT}
            />
          </SettingRow>
        ))}
    </>
  );
}
