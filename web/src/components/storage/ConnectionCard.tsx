import type { Compression } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { STORAGE_CLASSES } from "../../lib/storageClasses";
import { storageClassOf } from "../../lib/storageWrites";
import { Card } from "../../pages/settings/shared";
import { COMPRESSION_MODES } from "../CompressionSelector";
import { SelectField } from "../SelectField";
import { Selector } from "../Selector";
import { Toggle } from "../Toggle";
import { TextSetting } from "./fields";
import { Deviations, Rows, SettingRow } from "./rows";
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
 * the settings that belong to the place as a whole. The address itself is
 * shown and not edited: a location that moved would start empty.
 */
export function ConnectionCard({ edit, hueIndex }: { edit: LocationEdit; hueIndex: number }) {
  const { t } = useT();
  const { location, records, can, save, follow, busy } = edit;
  const local = location.backend === "local";
  // A folder that holds one repository per section, as opposed to a named
  // repository, which is one.
  const perSection = location.object === "path" || location.object === "destination";
  const storageClass = storageClassOf(location, records);
  const routeHint = ROUTE_HINT[location.backend];
  const whereHint = local ? (perSection ? t("storage.folderHint") : undefined) : routeHint && t(routeHint);

  return (
    <Card title={t("storage.connection")} hueIndex={hueIndex}>
      <Rows>
        <SettingRow label={t("dest.name")} hint={can("name") ? t("dest.nameHint") : undefined}>
          {can("name") ? (
            <TextSetting label={t("dest.name")} value={location.name} disabled={busy} onCommit={(name) => void save({ name })} />
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
              onChange={(enabled) => void save({ enabled })}
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
        </SettingRow>

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

        {can("storageClass") && storageClass !== undefined && (
          <SettingRow label={t("storage.storageClass")} hint={t("storage.storageClassHint")}>
            <SelectField
              value={storageClass}
              onChange={(next) => void save({ storageClass: next })}
              label={t("storage.storageClass")}
              disabled={busy}
              options={[
                { value: "", label: t("cloud.storageClass.default") },
                ...STORAGE_CLASSES.map((name) => ({ value: name as string, label: name as string })),
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
    </Card>
  );
}
