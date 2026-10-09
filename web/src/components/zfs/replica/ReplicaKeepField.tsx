import type { ZFSReplicaKeep, ZFSReplicaKeepPreset } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { InfoBubble } from "../../InfoBubble";
import { NumberField } from "../../NumberField";
import { Selector } from "../../Selector";
import { KEEP_FIELD_KEYS, KEEP_PRESET_KEY, keepSummary, withOwnCount } from "./replicaModel";

const PRESETS: ZFSReplicaKeepPreset[] = ["short", "balanced", "long", "own"];

/** ReplicaKeepField picks how many snapshots a replica keeps: a preset, or
 *  five numbers of its own. */
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
  return (
    <div className="flex flex-col gap-1.5">
      <span className="flex items-center gap-1.5 text-sm text-carbon-text">
        {label}
        <InfoBubble tip={hint} />
      </span>
      <Selector
        label={label}
        items={PRESETS.map((p) => ({ id: p, label: t(KEEP_PRESET_KEY[p]) }))}
        active={keep.preset}
        onChange={(id) => onChange({ ...keep, preset: id as ZFSReplicaKeepPreset })}
        activation="manual"
        disabled={disabled}
      />
      <p className="text-xs text-carbon-textMuted">{keepSummary(t, keep)}</p>
      {keep.preset === "own" && (
        <div className="grid grid-cols-5 gap-2 max-md:grid-cols-2">
          {KEEP_FIELD_KEYS.map((key, i) => (
            <label key={key} className="flex flex-col gap-1 text-xs text-carbon-textSub">
              {t(key)}
              <NumberField
                min={0}
                value={keep.own[i]}
                disabled={disabled}
                onChange={(e) => onChange({ ...keep, own: withOwnCount(keep.own, i, e.target.value) })}
                className="w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus"
              />
            </label>
          ))}
        </div>
      )}
    </div>
  );
}
