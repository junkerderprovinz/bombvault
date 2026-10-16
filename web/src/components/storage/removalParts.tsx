// What the two ways to remove a storage location show before anything is
// removed: the sections that are detached, and the backups that are deleted.
import { useId } from "react";
import type { ErasureSection, LocationErasure, LocationRemoval, RemovalSection } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { DOMAIN_LABEL } from "../../lib/storageLocations";
import { ToggleRow } from "../../pages/settings/shared";
import { FIELD } from "../destinations/steps";
import { InfoBubble } from "../InfoBubble";
import { itemName } from "../placement/NewTargetQuestion";
import { DomainGlyph, SettingRow } from "./rows";

const HEADING = "text-xs font-semibold uppercase tracking-widest text-carbon-textSub";
const ROWS = "flex flex-col divide-y divide-carbon-border/60";
const ROW = "flex items-start gap-3 py-2.5 first:pt-0 last:pb-0";
const REPO = "break-all text-start font-mono text-xs text-carbon-textMuted";

const KEPT: Record<NonNullable<ErasureSection["kept"]>, TranslationKey> = {
  "append-only": "storage.remove.keptAppendOnly",
  "in-use": "storage.remove.keptInUse",
  shared: "storage.remove.keptShared",
  unreadable: "storage.remove.keptUnreadable",
};

const USES: { use: RemovalSection["use"]; title: TranslationKey; tip?: TranslationKey }[] = [
  { use: "copy", title: "storage.remove.detached" },
  { use: "home", title: "storage.remove.moved" },
  { use: "repo", title: "storage.remove.follow", tip: "storage.remove.followTip" },
];

function Place({ label, path }: { label: string; path: string }) {
  return (
    <span className="flex flex-wrap items-baseline gap-x-1.5">
      <span className="text-xs text-carbon-textSub">{label}</span>
      <span dir="ltr" className={REPO}>
        {path}
      </span>
    </span>
  );
}

function DetachedRow({ section, home }: { section: RemovalSection; home: string }) {
  const { t } = useT();
  return (
    <li className={ROW}>
      <span className="mt-0.5 flex">
        <DomainGlyph domain={section.domain} />
      </span>
      <div className="flex min-w-0 flex-col gap-0.5">
        <span className="flex items-center gap-1.5 text-sm text-carbon-text">
          {t(DOMAIN_LABEL[section.domain])}
          {section.primary && <InfoBubble tip={t("storage.remove.primary")} />}
        </span>
        {section.use === "home" ? (
          <>
            <Place label={t("storage.remove.now")} path={section.repo} />
            {home && <Place label={t("storage.remove.then")} path={`${home}/${section.folder ?? ""}`} />}
          </>
        ) : (
          <span dir="ltr" className={REPO}>
            {section.repo}
          </span>
        )}
        {section.listedAt > 0 && (
          <span className="text-xs text-carbon-textMuted">
            {t("backups.tile.count", section.entries)} · {t("entry.places.count", section.backups)}
          </span>
        )}
        {(section.items ?? 0) > 0 && <span className="text-xs text-carbon-textSub">{t("storage.remove.items", section.items)}</span>}
        {section.default && <span className="text-xs text-carbon-textSub">{t("storage.remove.defaultHere")}</span>}
        {section.direct?.kept && <span className="text-xs text-carbon-textSub">{t("storage.remove.directKept")}</span>}
      </div>
    </li>
  );
}

/**
 * DetachedList is what removing a storage location changes for each section:
 * a copy stops, a home moves to the new folder `home` (empty while none is
 * entered), and entries that back up to a named repository follow their
 * section again.
 */
export function DetachedList({ removal, home }: { removal: LocationRemoval; home: string }) {
  const { t } = useT();
  if (removal.sections.length === 0) return <p className="text-xs text-carbon-textMuted">{t("storage.rules.none")}</p>;
  return USES.map(({ use, title, tip }) => {
    const sections = removal.sections.filter((section) => section.use === use);
    if (sections.length === 0) return null;
    return (
      <div key={use} className="flex flex-col gap-2">
        <span className={`flex items-center gap-1.5 ${HEADING}`}>
          {t(title)}
          {tip && <InfoBubble tip={t(tip)} />}
        </span>
        <ul className={ROWS}>
          {sections.map((section) => (
            <DetachedRow key={`${section.domain}:${section.use}:${section.targetId}`} section={section} home={home} />
          ))}
        </ul>
      </div>
    );
  });
}

/** Whether anything at the location can be deleted at all. */
export function erasable(erasure: LocationErasure): boolean {
  return erasure.sections.some((section) => !section.kept);
}

/**
 * ErasureDetails lists every repository at a storage location with what is
 * deleted from it or why it stays, names the entries that lose their only
 * backup, and asks for the two things the deletion waits for: the switch that
 * includes those entries, and the location's name typed out.
 */
export function ErasureDetails({
  erasure,
  name,
  typed,
  onType,
  lastToo,
  onLastToo,
}: {
  erasure: LocationErasure;
  /** The location's name, which has to be typed to confirm. */
  name: string;
  typed: string;
  onType: (text: string) => void;
  lastToo: boolean;
  onLastToo: (on: boolean) => void;
}) {
  const { t } = useT();
  const typedId = useId();
  return (
    <>
      <div className="flex flex-col gap-2">
        <span className={HEADING}>{t("storage.remove.deleted")}</span>
        <ul className={ROWS}>
          {erasure.sections.map((section) => {
            const domain = t(DOMAIN_LABEL[section.domain]);
            return (
              <li key={`${section.domain}:${section.use ?? ""}:${section.targetId}:${section.repoId ?? ""}`} className={ROW}>
                <span className="mt-0.5 flex">
                  <DomainGlyph domain={section.domain} />
                </span>
                <div className="flex min-w-0 flex-col gap-0.5">
                  <span className="flex items-center gap-1.5 text-sm text-carbon-text">
                    {section.repoId && !section.use ? t("storage.remove.directRepo").replace("{domain}", () => domain) : domain}
                    {!section.targetId && !section.use && <InfoBubble tip={t("storage.remove.leftBehind")} />}
                  </span>
                  <span dir="ltr" className={REPO}>
                    {section.repo}
                  </span>
                  {section.kept ? (
                    <span className="text-xs text-statusWarn">{t(KEPT[section.kept])}</span>
                  ) : (
                    <span className="text-xs text-carbon-textMuted">
                      {t("backups.tile.count", section.entries)} · {t("entry.places.count", section.backups)}
                    </span>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      </div>
      {erasable(erasure) && (
        <p className="text-sm text-carbon-text">
          {t("entry.places.count", erasure.backups)} · {t("backups.tile.count", erasure.entries)}
          {erasure.backupsOnlyThere > 0 && ` · ${t("storage.remove.onlyThere", erasure.backupsOnlyThere)}`}
        </p>
      )}
      {erasure.lastCopies.length > 0 && (
        <div className="flex flex-col gap-2 rounded-control bg-statusWarnBg p-3">
          <span className="text-xs font-semibold text-statusWarn">{t("storage.remove.lastCopies")}</span>
          <ul className="flex flex-col gap-1">
            {erasure.lastCopies.map((entry) => (
              <li key={entry.identity} className="flex flex-wrap items-baseline gap-x-2 text-sm text-carbon-text">
                <span className="min-w-0 wrap-anywhere">{itemName(entry.identity)}</span>
                <span className="text-xs text-carbon-textSub">
                  {t(DOMAIN_LABEL[entry.domain])} · {t("entry.places.count", entry.backups)}
                </span>
              </li>
            ))}
          </ul>
          <ToggleRow label={t("storage.remove.lastCopiesToo")} checked={lastToo} onChange={onLastToo} />
        </div>
      )}
      {erasable(erasure) && (
        <SettingRow label={t("storage.remove.confirmName").replace("{name}", () => name)} htmlFor={typedId}>
          <input
            id={typedId}
            value={typed}
            onChange={(e) => onType(e.target.value)}
            autoComplete="off"
            spellCheck={false}
            className={`w-72 max-w-full ${FIELD}`}
          />
        </SettingRow>
      )}
    </>
  );
}
