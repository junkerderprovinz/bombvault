// ChipFilter is one filter of a list: a caption and the choices beside it.
// The labels arrive translated, which keeps this module out of the i18n graph.
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
        inline
        select="one"
        active={value}
        onChange={(id) => onChange(id as K)}
      />
    </div>
  );
}
