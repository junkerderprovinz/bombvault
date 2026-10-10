import { useEffect, useState } from "react";
import { getSettings, type OffsiteDomain, type Settings } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { DOMAIN_LABEL } from "../../lib/storageLocations";
import { Card } from "../../pages/settings/shared";
import { cadenceLabel } from "../ScheduleBadge";
import { NumberSetting } from "./fields";
import { Deviations, DomainGlyph, Rows, SettingRow } from "./rows";
import { CopyScene } from "./scenes";
import type { LocationEdit } from "./useLocationEdit";

const SCHEDULE: Record<OffsiteDomain, keyof Settings> = {
  containers: "containersOffsiteSchedule",
  vms: "vmsOffsiteSchedule",
  flash: "flashOffsiteSchedule",
  files: "filesOffsiteSchedule",
  zfs: "zfsOffsiteSchedule",
  config: "configOffsiteSchedule",
};

/**
 * CopyCard is about the copies a location takes: how fast they may run and
 * when they do. Every copy of a section runs on that section's off-site
 * schedule, so the times are shown here and set with the schedules.
 */
export function CopyCard({ edit, hueIndex }: { edit: LocationEdit; hueIndex: number }) {
  const { t } = useT();
  const { location, can, save, follow, busy } = edit;
  const [settings, setSettings] = useState<Settings | null>(null);
  const copies = location.sections.filter((section) => section.use === "copy");

  useEffect(() => {
    let alive = true;
    getSettings()
      .then((res) => {
        if (alive && res.ok) setSettings(res.settings);
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);

  function when(domain: OffsiteDomain): string {
    const schedule = String(settings?.[SCHEDULE[domain]] ?? "").trim();
    return schedule ? cadenceLabel(schedule, t) : t("storage.copy.afterBackup");
  }

  const limits: { key: "limitUpload" | "limitDownload"; label: "storage.copy.upload" | "storage.copy.download" }[] = [
    { key: "limitUpload", label: "storage.copy.upload" },
    { key: "limitDownload", label: "storage.copy.download" },
  ];

  return (
    <Card title={t("storage.copy.title")} hint={t("storage.copy.hint")} hueIndex={hueIndex}>
      <CopyScene target={location} />
      <Rows>
        {settings && copies.length > 0 && (
          <SettingRow label={t("storage.copy.when")} hint={t("storage.copy.whenHint")}>
            <ul className="flex flex-col items-end gap-1 text-sm text-carbon-textSub">
              {copies.map((section) => (
                <li key={section.domain} className="flex items-center gap-2">
                  <DomainGlyph domain={section.domain} />
                  <span className="text-carbon-text">{t(DOMAIN_LABEL[section.domain])}</span>
                  <span>{when(section.domain)}</span>
                </li>
              ))}
            </ul>
          </SettingRow>
        )}
        {limits.map(({ key, label }) => {
          const value = location[key];
          if (value === undefined) return null;
          return (
            <SettingRow key={key} label={t(label)} hint={t("storage.copy.unlimited")}>
              {can(key) ? (
                <NumberSetting label={t(label)} value={value} disabled={busy} onCommit={(n) => void save(key === "limitUpload" ? { limitUpload: n } : { limitDownload: n })} />
              ) : (
                <span className="text-sm tabular-nums text-carbon-textSub">{value}</span>
              )}
            </SettingRow>
          );
        })}
        <Deviations
          location={location}
          setting="limits"
          describe={(section) =>
            t("storage.copy.ownLimits")
              .replace("{up}", String(section.limitUpload))
              .replace("{down}", String(section.limitDownload))
          }
          onFollow={follow}
          busy={busy}
        />
      </Rows>
    </Card>
  );
}
