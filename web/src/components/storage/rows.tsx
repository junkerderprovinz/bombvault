// The rows every list and card in the app is made of: a setting with its
// control, and the sections of a storage location that hold a setting for
// themselves.
import type { ReactNode } from "react";
import type { FollowedSetting, OffsiteDomain, StorageLocation, StorageLocationSection } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { DOMAIN_LABEL, canFollow, deviating } from "../../lib/storageLocations";
import { Button } from "../Button";
import { IconRefresh } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { IconConfig, IconContainers, IconFiles, IconFlash, IconVM, IconZFS } from "../navGlyphs";

/**
 * Rows is how every list in the app looks: its rows parted by a hairline, with
 * no filled block behind them. A list of things rather than settings is a `ul`
 * of SettingRows `as="li"`.
 */
export function Rows({ children, as: Tag = "div", label }: { children: ReactNode; as?: "div" | "ul"; label?: string }) {
  return (
    <Tag aria-label={label} className="flex flex-col divide-y divide-carbon-border/60">
      {children}
    </Tag>
  );
}

/** RowGlyph is the tile an entry's icon sits on at the start of its row. */
export function RowGlyph({ children }: { children: ReactNode }) {
  return (
    <span className="flex h-9 w-9 flex-none items-center justify-center rounded-control bg-carbon-surface2 text-accentText [&_svg]:h-[18px] [&_svg]:w-[18px]">
      {children}
    </span>
  );
}

export function SettingRow({
  label,
  name,
  glyph,
  hint,
  htmlFor,
  note,
  narrow,
  as: Tag = "div",
  newId,
  children,
}: {
  label: string;
  /** What shows in place of the label, when an entry's name is more than text. */
  name?: ReactNode;
  /** An icon before the label. */
  glyph?: ReactNode;
  /** Explanation in an (i) bubble beside the label. */
  hint?: string;
  /** The id of the field the label names. */
  htmlFor?: string;
  /** What the control cannot say itself, under the label. */
  note?: ReactNode;
  /** The control is small enough to stay beside any label, which wraps instead. */
  narrow?: boolean;
  as?: "div" | "li";
  /** The change in lib/whatsNew.ts the row is, which puts its dot on it. */
  newId?: string;
  /** The control. A row without one only says something. */
  children?: ReactNode;
}) {
  return (
    <Tag data-search-row={label} data-new={newId} className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 py-3 first:pt-0 last:pb-0">
      {/* A long note wraps beside the control instead of pushing it onto a line of its own. */}
      <div className={`flex min-w-0 flex-col gap-0.5 ${note ? "flex-1 basis-56" : narrow ? "flex-1 basis-0" : ""}`}>
        <span className="flex items-center gap-1.5 text-sm text-carbon-text">
          {glyph && <span className="inline-flex flex-none">{glyph}</span>}
          {name ??
            (htmlFor ? (
              <label htmlFor={htmlFor} className="min-w-0 wrap-break-word">
                {label}
              </label>
            ) : (
              <span className="min-w-0 wrap-break-word">{label}</span>
            ))}
          {hint && <InfoBubble tip={hint} />}
        </span>
        {note && <div className="flex flex-col gap-1 text-xs text-carbon-textMuted wrap-break-word">{note}</div>}
      </div>
      {/* A selector wraps by the room this box has, so on a phone the box takes
          all the row leaves it, beside the label or under it. */}
      {children && (
        <div className="flex min-w-0 max-w-full flex-wrap items-center justify-end gap-2 max-md:has-[[aria-orientation=horizontal]]:grow">
          {children}
        </div>
      )}
    </Tag>
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
 * settings describe goes back only for retention; for anything else it gets
 * the reason instead of a button.
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
    <div className="py-3 first:pt-0 last:pb-0">
      <Rows as="ul">
        {sections.map((section) => (
          <SettingRow
            key={`${section.domain}:${section.use}`}
            as="li"
            label={t(DOMAIN_LABEL[section.domain])}
            glyph={<DomainGlyph domain={section.domain} />}
            hint={section.primary ? t("storage.own.primary") : undefined}
            note={`${t("storage.keep.custom")} · ${describe(section)}`}
          >
            {canFollow(location, section, setting) && (
              <Button
                label={t("storage.own.reset")}
                labelKey="storage.own.reset"
                glyph={<IconRefresh />}
                tone="neutral"
                disabled={busy}
                onClick={() => onFollow(section, setting)}
              />
            )}
          </SettingRow>
        ))}
      </Rows>
    </div>
  );
}
