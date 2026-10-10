// The rows a storage location's cards are made of: a setting with its control,
// and the sections that hold a setting for themselves.
import type { ReactNode } from "react";
import type { FollowedSetting, OffsiteDomain, StorageLocation, StorageLocationSection } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { DOMAIN_LABEL, canFollow, deviating } from "../../lib/storageLocations";
import { Button } from "../Button";
import { IconRefresh } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { IconConfig, IconContainers, IconFiles, IconFlash, IconVM, IconZFS } from "../navGlyphs";

export function Rows({ children }: { children: ReactNode }) {
  return <div className="flex flex-col divide-y divide-carbon-border/60">{children}</div>;
}

export function SettingRow({
  label,
  hint,
  note,
  children,
}: {
  label: string;
  /** Explanation in an (i) bubble beside the label. */
  hint?: string;
  /** What the control cannot say itself, under the label. */
  note?: string;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 py-3 first:pt-0 last:pb-0">
      <div className="flex min-w-0 flex-col gap-0.5">
        <span className="flex items-center gap-1.5 text-sm text-carbon-text">
          <span className="min-w-0 wrap-break-word">{label}</span>
          {hint && <InfoBubble tip={hint} />}
        </span>
        {note && <span className="text-xs text-carbon-textMuted wrap-break-word">{note}</span>}
      </div>
      <div className="flex min-w-0 max-w-full flex-wrap items-center justify-end gap-2">{children}</div>
    </div>
  );
}

const DOMAIN_GLYPH: Record<OffsiteDomain, () => ReactNode> = {
  containers: IconContainers,
  vms: IconVM,
  flash: IconFlash,
  files: IconFiles,
  zfs: IconZFS,
  config: IconConfig,
};

export function DomainGlyph({ domain }: { domain: OffsiteDomain }) {
  const Glyph = DOMAIN_GLYPH[domain];
  return (
    <span className="inline-flex text-accentText">
      <Glyph />
    </span>
  );
}

/**
 * Deviations lists the sections that hold `setting` themselves, each with its
 * own value and the way back to the location's. The copy a domain's off-site
 * settings describe cannot go back, so it gets the reason instead of a button.
 */
export function Deviations({
  location,
  setting,
  describe,
  onFollow,
  busy,
}: {
  location: StorageLocation;
  setting: FollowedSetting;
  describe: (section: StorageLocationSection) => string;
  onFollow: (section: StorageLocationSection, setting: FollowedSetting) => void;
  busy: boolean;
}) {
  const { t } = useT();
  const sections = deviating(location, setting);
  if (sections.length === 0) return null;
  return (
    <ul className="my-3 flex flex-col gap-2 rounded-control bg-carbon-surface2 p-3 first:mt-0 last:mb-0">
      {sections.map((section) => (
        <li key={`${section.domain}:${section.use}`} className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
          <span className="flex min-w-0 flex-col gap-0.5">
            <span className="flex items-center gap-2 text-sm text-carbon-text">
              <DomainGlyph domain={section.domain} />
              {t(DOMAIN_LABEL[section.domain])}
              {section.primary && <InfoBubble tip={t("storage.own.primary")} />}
            </span>
            <span className="text-xs text-carbon-textMuted wrap-break-word">
              {t("storage.keep.custom")} · {describe(section)}
            </span>
          </span>
          {canFollow(location, section) && (
            <Button
              label={t("storage.own.reset")}
              labelKey="storage.own.reset"
              glyph={<IconRefresh />}
              tone="neutral"
              disabled={busy}
              onClick={() => onFollow(section, setting)}
            />
          )}
        </li>
      ))}
    </ul>
  );
}
