import { useState } from "react";
import type { Compression, OffsiteDomain } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { STORAGE_CLASSES } from "../../lib/storageClasses";
import { DOMAIN_LABEL } from "../../lib/storageLocations";
import { storageClassOf } from "../../lib/storageWrites";
import { Card } from "../../pages/settings/shared";
import { Button } from "../Button";
import { COMPRESSION_MODES } from "../CompressionSelector";
import { IconForward } from "../glyphs";
import { IconPencil } from "../navGlyphs";
import { SelectField } from "../SelectField";
import { Selector } from "../Selector";
import { Toggle } from "../Toggle";
import { useConfirm } from "../../lib/useConfirm";
import { useNamedRepos } from "../../lib/useNamedRepos";
import { TextSetting } from "./fields";
import { Deviations, DomainGlyph, Rows, SettingRow } from "./rows";
import { OwnFolderWindow } from "./OwnFolderWindow";
import type { LocationEdit } from "./useLocationEdit";

// How restic reaches a backend, for the ones the wizard explains as well.
const ROUTE_HINT: Record<string, TranslationKey> = {
  s3: "dest.route.s3",
  rest: "dest.route.rest",
  rclone: "dest.route.rclone",
};

function compressionLabel(mode: Compression): TranslationKey {
  return `settings.compression.${mode}`;
}

/**
 * ConnectionCard says what a storage location is and where it lies, and holds
 * the settings that belong to the place as a whole. The address is changed
 * in a window of its own, which `onChange` opens where the location has one.
 */
export function ConnectionCard({ edit, onChange, hueIndex }: { edit: LocationEdit; onChange?: () => void; hueIndex: number }) {
  const { t } = useT();
  const { location, records, can, save, follow, busy } = edit;
  const local = location.backend === "local";
  // A folder that holds one repository per section, as opposed to a named
  // repository, which is one.
  const perSection = location.object === "path" || location.object === "destination";
  const storageClass = storageClassOf(location, records);
  // A destination offers the classes its provider takes, plus the one it
  // holds, so that one can still be set back to the default.
  const offered: readonly string[] = records.destination?.storageClasses ?? STORAGE_CLASSES;
  const classes = storageClass && !offered.includes(storageClass) ? [...offered, storageClass] : offered;
  const routeHint = ROUTE_HINT[location.backend];
  const whereHint = local ? (perSection ? t("storage.folderHint") : undefined) : routeHint && t(routeHint);
  const [moving, setMoving] = useState<OffsiteDomain[] | null>(null);
  const repos = useNamedRepos();
  const { confirm, confirmDialog } = useConfirm();

  // Switching a repository off while entries back up to it is allowed, it is
  // how a share that went away is parked, but it stops those backups, and
  // nothing else on the page says so.
  async function setEnabled(enabled: boolean) {
    const repo = location.object === "repo" ? repos.find((r) => `repo:${r.id}` === location.id) : undefined;
    if (!enabled && repo && repo.inUse !== 0) {
      const question = repo.inUse < 0 ? t("repos.disableWarnUnknown") : t("repos.disableWarn").replace("{n}", String(repo.inUse));
      if (!(await confirm(question))) return;
    }
    await save({ enabled });
  }

  return (
    <Card title={t("storage.connection")} hueIndex={hueIndex}>
      {confirmDialog}
      <Rows>
        <SettingRow label={t("dest.name")} hint={can("name") ? t("dest.nameHint") : undefined}>
          {can("name") ? (
            <span data-new="location-rename" className="flex max-w-full">
              <TextSetting label={t("dest.name")} value={location.name} disabled={busy} onCommit={(name) => void save({ name })} />
            </span>
          ) : (
            <span className="text-sm text-carbon-textSub wrap-anywhere">{location.name}</span>
          )}
        </SettingRow>

        {can("enabled") && (
          <SettingRow label={t("storage.available")} hint={t("storage.availableHint")}>
            <Toggle
              hideLabel
              label={t("storage.available")}
              checked={location.enabled}
              disabled={busy}
              onChange={(enabled) => void setEnabled(enabled)}
            />
          </SettingRow>
        )}
        <Deviations
          location={location}
          setting="enabled"
          describe={(section) => t(section.enabled ? "storage.on" : "storage.off")}
          onFollow={follow}
          busy={busy}
        />

        <SettingRow label={t(local ? "storage.folder" : "dest.location")} hint={whereHint}>
          <code dir="ltr" className="min-w-0 break-all text-start font-mono text-xs text-carbon-textSub">
            {location.where}
          </code>
          {onChange && (
            <Button label={t("storage.add.change")} labelKey="storage.add.change" glyph={<IconPencil />} tone="neutral" onClick={onChange} />
          )}
        </SettingRow>
        {location.object === "path" && (
          <div className="py-3 first:pt-0 last:pb-0">
            <Rows as="ul">
              {location.sections.map((section) => (
                <SettingRow
                  key={section.domain}
                  as="li"
                  label={t(DOMAIN_LABEL[section.domain])}
                  glyph={<DomainGlyph domain={section.domain} />}
                  note={
                    <code dir="ltr" className="break-all text-start font-mono">
                      {section.where}
                    </code>
                  }
                >
                  <Button
                    label={t("storage.add.change")}
                    labelKey="storage.add.change"
                    glyph={<IconPencil />}
                    tone="neutral"
                    onClick={() => setMoving([section.domain])}
                  />
                </SettingRow>
              ))}
            </Rows>
          </div>
        )}
        {location.object === "path" && location.sections.length > 0 && (
          <SettingRow label={t("own.here")} hint={t("own.hint")}>
            <span data-new="own-folder-move" className="flex">
              <Button
                label={t("own.titleMany")}
                labelKey="own.titleMany"
                glyph={<IconForward />}
                tone="neutral"
                onClick={() => setMoving(location.sections.map((section) => section.domain))}
              />
            </span>
          </SettingRow>
        )}

        {location.compression !== undefined && (
          <SettingRow label={t("settings.compression")} hint={t("storage.compressionHint")}>
            {can("compression") ? (
              <Selector
                items={COMPRESSION_MODES.map(({ id, labelKey, Glyph }) => ({ id, label: t(labelKey), icon: <Glyph /> }))}
                label={t("settings.compression")}
                size="sm"
                inline
                select="one"
                activation="manual"
                active={location.compression}
                onChange={(id) => void save({ compression: id as Compression })}
                disabled={busy}
              />
            ) : (
              <span className="text-sm text-carbon-textSub">{t(compressionLabel(location.compression))}</span>
            )}
          </SettingRow>
        )}
        <Deviations
          location={location}
          setting="compression"
          describe={(section) => t(compressionLabel(section.compression))}
          onFollow={follow}
          busy={busy}
        />

        {can("storageClass") && storageClass !== undefined && classes.length > 0 && (
          <SettingRow label={t("storage.storageClass")} hint={t("storage.storageClassHint")}>
            <SelectField
              value={storageClass}
              onChange={(next) => void save({ storageClass: next })}
              label={t("storage.storageClass")}
              disabled={busy}
              options={[
                { value: "", label: t("cloud.storageClass.default") },
                ...classes.map((name) => ({ value: name, label: name })),
              ]}
              className="rounded-control bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 glim-field-focus"
            />
          </SettingRow>
        )}

        {can("offPremises") && (
          <SettingRow label={t("storage.offPremises")} hint={t("storage.offPremisesHint")}>
            <Toggle
              hideLabel
              label={t("storage.offPremises")}
              checked={location.offPremises}
              disabled={busy}
              onChange={(offPremises) => void save({ offPremises })}
            />
          </SettingRow>
        )}
      </Rows>
      {moving && <OwnFolderWindow domains={moving} onClose={() => setMoving(null)} />}
    </Card>
  );
}
