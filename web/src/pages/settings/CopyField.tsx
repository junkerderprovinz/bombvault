import { Button } from "../../components/Button";
import { IconCopy } from "../../components/navGlyphs";
import type { TranslationKey } from "../../lib/i18n";

/** CopyField is an address to hand on: read-only, in a box, with its copy button. */
export function CopyField({
  value,
  label,
  onCopy,
  t,
}: {
  value: string;
  label: string;
  onCopy: () => void;
  t: (key: TranslationKey) => string;
}) {
  return (
    <span className="flex max-w-full items-center gap-1.5 rounded-control bg-carbon-surface2 py-0.5 ps-3 pe-0.5">
      <input
        readOnly
        value={value}
        aria-label={label}
        dir="ltr"
        className="h-8 w-64 min-w-0 max-w-full truncate bg-transparent text-start font-mono text-xs text-carbon-text outline-none"
      />
      <Button label={t("common.copy")} labelKey="common.copy" glyph={<IconCopy />} onClick={onCopy} />
    </span>
  );
}
