import { useState } from "react";
import { Link } from "react-router-dom";
import type { Compression, FollowedSetting, OffsiteDomain, StorageLocationSection } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { presetOf, type KeepCounts } from "../../lib/keepPlan";
import { formatList } from "../../lib/placement";
import { DOMAIN_LABEL, canFollow, keepCounts, retentionKeep } from "../../lib/storageLocations";
import { sectionSettings } from "../../lib/storageWrites";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { adoptTarget } from "../../lib/useDestinations";
import { storageLocationsChanged } from "../../lib/useStorageLocations";
import { InstanceWindow } from "../../pages/instances/InstanceWindow";
import { Card } from "../../pages/settings/shared";
import { Button } from "../Button";
import { COMPRESSION_MODES } from "../CompressionSelector";
import { IconLink, IconRefresh } from "../glyphs";
import { InfoBubble } from "../InfoBubble";
import { IconCheck, IconPencil } from "../navGlyphs";
import { Selector } from "../Selector";
import { keepSummary } from "./KeepCard";
import { KeepCountFields, useKeepLessQuestion } from "./keepFields";
import { DomainGlyph, Rows, SettingRow } from "./rows";
import type { LocationEdit } from "./useLocationEdit";

const OWN_LABEL: Record<FollowedSetting, TranslationKey> = {
  retention: "storage.keep.title",
  compression: "settings.compression",
  limits: "storage.copy.title",
  enabled: "storage.available",
};

const LIKE = "like";

/** The sections that keep backups at a location, each once: its copy, or the
 *  home where it has no copy here. */
function ruledSections(sections: StorageLocationSection[]): StorageLocationSection[] {
  const byDomain = new Map<OffsiteDomain, StorageLocationSection>();
  for (const section of sections) {
    if (section.use === "copy" || !byDomain.has(section.domain)) byDomain.set(section.domain, section);
  }
  return [...byDomain.values()];
}

/** The window in which one section gets rules of its own at a location. */
function RulesWindow({ edit, section, onClose }: { edit: LocationEdit; section: StorageLocationSection; onClose: () => void }) {
  const { t } = useT();
  const { location, records, busy } = edit;
  const less = useKeepLessQuestion();
  const settings = sectionSettings(location, records, section);
  const ownKeep = section.own.includes("retention");
  const [custom, setCustom] = useState(ownKeep);
  const domain = t(DOMAIN_LABEL[section.domain]);
  const counts = keepCounts(section.retention);
  const asks = location.kind !== "local";

  async function keep(next: KeepCounts) {
    if (asks && !(await less.agreed(counts, next, location.name))) return;
    await edit.saveSection(section, { retention: retentionKeep(next) });
  }

  async function keepLike() {
    setCustom(false);
    if (!ownKeep) return;
    const shared = location.retention ? keepCounts(location.retention) : counts;
    if (asks && !(await less.agreed(counts, shared, location.name))) {
      setCustom(true);
      return;
    }
    edit.follow(section, "retention");
  }

  return (
    <InstanceWindow
      title={t("storage.rules.windowTitle").replace("{domain}", () => domain)}
      facts={t("storage.rules.windowHint").replace("{domain}", () => domain).replace("{name}", () => location.name)}
      onClose={onClose}
      footer={<Button label={t("common.done")} labelKey="common.done" glyph={<IconCheck />} tone="accent" onClick={onClose} />}
    >
      {less.dialog}
      <Rows>
        <SettingRow label={t("storage.rules.repository")} note={t("storage.rules.follows")}>
          <code dir="ltr" className="min-w-0 break-all text-start font-mono text-xs text-carbon-textSub">
            {section.where}
          </code>
        </SettingRow>
        {settings.includes("retention") && (
          <SettingRow
            label={t("storage.keep.title")}
            hint={t("storage.rules.keepHint")}
            note={custom ? undefined : presetOf(counts) === "custom" ? keepSummary(t, section.retention) : t(`storage.keep.${presetOf(counts)}`)}
          >
            <Selector
              items={[
                { id: LIKE, label: t("storage.rules.likeLocation") },
                { id: "custom", label: t("storage.keep.custom") },
              ]}
              label={t("storage.keep.title")}
              select="one"
              activation="manual"
              inline
              active={custom ? "custom" : LIKE}
              onChange={(id) => (id === LIKE ? void keepLike() : setCustom(true))}
              disabled={busy}
            />
          </SettingRow>
        )}
        {settings.includes("retention") && custom && (
          <div className="py-3">
            <KeepCountFields
              counts={counts}
              disabled={busy}
              onCommit={(i, n) => void keep(counts.map((count, at) => (at === i ? n : count)) as KeepCounts)}
            />
          </div>
        )}
        {settings.includes("compression") && (
          <SettingRow label={t("settings.compression")} hint={t("storage.rules.compressionHint")}>
            <Selector
              items={[
                { id: LIKE, label: t("storage.rules.likeLocation") },
                ...COMPRESSION_MODES.map(({ id, labelKey, Glyph }) => ({ id, label: t(labelKey), icon: <Glyph /> })),
              ]}
              label={t("settings.compression")}
              size="sm"
              inline
              pairsOnPhone
              select="one"
              activation="manual"
              active={section.own.includes("compression") ? section.compression : LIKE}
              onChange={(id) =>
                id === LIKE ? edit.follow(section, "compression") : void edit.saveSection(section, { compression: id as Compression })
              }
              disabled={busy}
            />
          </SettingRow>
        )}
      </Rows>
      {section.own.length > 0 && canFollow(location, section) && (
        <div className="flex justify-end">
          <Button
            label={t("storage.own.reset")}
            labelKey="storage.own.reset"
            glyph={<IconRefresh />}
            tone="neutral"
            disabled={busy}
            onClick={() => {
              setCustom(false);
              edit.followAll(section);
            }}
          />
        </div>
      )}
    </InstanceWindow>
  );
}

/**
 * RulesCard lists the sections that keep backups at a location, each with
 * whether it follows the location or holds rules of its own, and opens the
 * window that changes them.
 */
export function RulesCard({ edit, hueIndex }: { edit: LocationEdit; hueIndex: number }) {
  const { t, lang } = useT();
  const { location, records } = edit;
  const [open, setOpen] = useState<OffsiteDomain | null>(null);
  const sections = ruledSections(location.sections);
  const shown = sections.find((section) => section.domain === open);

  return (
    <Card title={t("storage.rules.title")} hint={t("storage.rules.hint")} hueIndex={hueIndex}>
      {sections.length > 0 && (
        <Rows>
          {sections.map((section) => {
            const own = section.own.map((setting) => t(OWN_LABEL[setting]));
            return (
              <div key={section.domain} className="flex flex-wrap items-center gap-x-3 gap-y-2 py-3 first:pt-0 last:pb-0">
                <DomainGlyph domain={section.domain} />
                <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="flex items-center gap-1.5 text-sm text-carbon-text">
                    {t(DOMAIN_LABEL[section.domain])}
                    {section.primary && <InfoBubble tip={t("storage.own.primary")} />}
                  </span>
                  <span className="text-xs text-carbon-textMuted wrap-break-word">
                    {own.length > 0 ? t("storage.rules.own").replace("{list}", () => formatList(lang, own)) : t("storage.rules.follows")}
                  </span>
                </div>
                {sectionSettings(location, records, section).length > 0 && (
                  <Button
                    label={t("storage.rules.change")}
                    labelKey="storage.rules.change"
                    glyph={<IconPencil />}
                    tone="neutral"
                    onClick={() => setOpen(section.domain)}
                  />
                )}
              </div>
            );
          })}
        </Rows>
      )}
      <p className="flex flex-wrap gap-x-1.5 text-xs text-carbon-textMuted">
        {t(sections.length > 0 ? "storage.rules.where" : "storage.rules.none")}
        <Link to="/storage" className="underline decoration-carbon-border underline-offset-[3px] hover:text-accentText">
          {t("storage.rules.open")}
        </Link>
      </p>
      {shown && <RulesWindow key={shown.domain} edit={edit} section={shown} onClose={() => setOpen(null)} />}
    </Card>
  );
}

/**
 * AdoptCard offers the off-site targets typed in by hand whose repository
 * lies under this destination. Taking one over leaves the repository where it
 * is and lets the target follow the destination. Without such a target the
 * card is not shown.
 */
export function AdoptCard({ edit, hueIndex }: { edit: LocationEdit; hueIndex: number }) {
  const { t } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [busy, setBusy] = useState(false);
  const destination = edit.records.destination;
  const adoptable = destination?.adoptable ?? [];
  if (!destination || adoptable.length === 0) return null;

  async function adopt(targetId: string, domain: string) {
    if (!destination) return;
    const fill = (text: string) => text.replaceAll("{domain}", () => domain).replaceAll("{name}", () => destination.name);
    if (!(await confirm(fill(t("storage.adopt.ask")), { confirmKey: "dest.adopt.take" }))) return;
    setBusy(true);
    const res = await adoptTarget(destination.id, targetId);
    setBusy(false);
    if (!res.ok) {
      push(res.error ?? t("settings.error"), "fail");
      return;
    }
    push(fill(t("storage.adopt.done")), "success");
    storageLocationsChanged();
  }

  return (
    <Card title={t("storage.adopt.title")} hint={t("storage.adopt.hint")} hueIndex={hueIndex}>
      {confirmDialog}
      <Rows>
        {adoptable.map((target) => {
          const domain = target.domain in DOMAIN_LABEL ? t(DOMAIN_LABEL[target.domain as OffsiteDomain]) : target.domain;
          return (
            <div key={target.id} className="flex flex-wrap items-center gap-x-3 gap-y-2 py-3 first:pt-0 last:pb-0">
              {target.domain in DOMAIN_LABEL && <DomainGlyph domain={target.domain as OffsiteDomain} />}
              <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="text-sm text-carbon-text">{domain}</span>
                <span dir="ltr" className="break-all text-start font-mono text-xs text-carbon-textMuted">
                  {target.repo}
                </span>
              </div>
              <Button
                label={t("dest.adopt.take")}
                labelKey="dest.adopt.take"
                glyph={<IconLink />}
                tone="neutral"
                disabled={busy}
                onClick={() => void adopt(target.id, domain)}
              />
            </div>
          );
        })}
      </Rows>
    </Card>
  );
}
