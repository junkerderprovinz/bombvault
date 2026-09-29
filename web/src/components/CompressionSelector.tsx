import type { Compression, Settings } from "../lib/api";
import { useT } from "../lib/i18n";
import { InfoBubble } from "./InfoBubble";
import { Selector } from "./Selector";

const MODES = [
  { id: "off", labelKey: "settings.compression.off" },
  { id: "auto", labelKey: "settings.compression.auto" },
  { id: "max", labelKey: "settings.compression.max" },
] as const satisfies readonly { id: Compression; labelKey: string }[];

/** Picks restic's --compression for one repository. */
export function CompressionSelector({
  value,
  onChange,
  disabled,
}: {
  value: Compression;
  onChange: (next: Compression) => void;
  disabled?: boolean;
}) {
  const { t } = useT();
  return (
    <div className="flex items-center justify-between gap-2 flex-wrap">
      <span className="flex items-center gap-1 text-xs text-carbon-textSub">
        {t("settings.compression")}
        <InfoBubble tip={t("settings.compressionInfo")} />
      </span>
      <Selector
        items={MODES.map((m) => ({ id: m.id, label: t(m.labelKey) }))}
        label={t("settings.compression")}
        size="sm"
        select="one"
        active={value}
        onChange={(id) => onChange(id as Compression)}
        disabled={disabled}
      />
    </div>
  );
}

type SaveSettings = (
  patch: Partial<Settings>,
  setState: (s: "idle" | "saving" | "saved" | "error") => void,
  setError: (e: string | null) => void
) => Promise<boolean>;

/** Saves one key of the settings' compression map right away, and puts the
 *  old map back when the server refuses. save() reports either way. */
export async function saveCompression(
  key: string,
  next: Compression,
  settings: Settings,
  setSettings: React.Dispatch<React.SetStateAction<Settings | null>>,
  save: SaveSettings
) {
  const before = settings.compression;
  const compression = { ...before, [key]: next };
  setSettings((prev) => (prev ? { ...prev, compression } : prev));
  const ok = await save({ compression }, () => undefined, () => undefined);
  if (!ok) setSettings((prev) => (prev ? { ...prev, compression: before } : prev));
}
