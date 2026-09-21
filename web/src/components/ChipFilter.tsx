// ---------------------------------------------------------------------------
// ChipFilter — the schedule/backup chip strip shared by the Containers and
// VMs toolbars, plus the localStorage read it pairs with.
//
// ONE copy of the caption-span + small-well Selector chip: Containers.tsx and
// VMs.tsx carried byte-identical local copies that could drift (the same
// near-identical-copies drift the SortControl/FilterControl comment in
// Containers.tsx records for the pre-Selector rendering). Modeled on the
// IncludeToggle/FilterPopover placement precedent for page-shared controls:
// generic over its option set, no `t` inside — labels arrive pre-translated
// from the page, which keeps this module out of the i18n graph.
// ---------------------------------------------------------------------------
import { Selector } from "./Selector";

export function ChipFilter<K extends string>({
  label,
  options,
  value,
  onChange,
}: {
  label: string;
  options: { key: K; label: string }[];
  value: K;
  onChange: (k: K) => void;
}) {
  return (
    <div className="flex items-center gap-2 flex-wrap">
      <span className="text-xs text-carbon-textMuted">{label}</span>
      <Selector
        items={options.map((o) => ({ id: o.key, label: o.label }))}
        label={label}
        variant="well"
        select="one"
        active={value}
        onChange={(id) => onChange(id as K)}
      />
    </div>
  );
}

// The read-validate-fallback localStorage pattern both pages duplicated for
// every stored filter dimension: try/catch around getItem (private-mode and
// disabled-storage browsers throw), membership check against the valid keys,
// fallback otherwise. Written once so the next stored filter can't re-copy it.
export function loadStoredFilterKey<K extends string>(
  storageKey: string,
  valid: readonly K[],
  fallback: K,
): K {
  try {
    const v = localStorage.getItem(storageKey);
    if (v !== null && (valid as readonly string[]).includes(v)) return v as K;
  } catch {
    // localStorage unavailable (storage disabled/private mode): the fallback
    // is the same answer a missing key gets.
  }
  return fallback;
}
