import { useEffect, useState } from "react";
import { getRcloneSections, type RcloneSection } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { Card } from "../../pages/settings/shared";
import { Rows, SettingRow } from "./rows";

/**
 * RcloneConfCard shows the section of BombVault's rclone.conf a location goes
 * through, among all the remotes that file holds. The server hands out a
 * section without its secrets, so a setting that holds one shows as hidden.
 */
export function RcloneConfCard({ remote, hueIndex }: { remote: string; hueIndex: number }) {
  const { t } = useT();
  const [sections, setSections] = useState<RcloneSection[]>([]);

  useEffect(() => {
    let alive = true;
    getRcloneSections()
      .then((res) => {
        if (alive && res.ok) setSections(res.sections ?? []);
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);

  const section = sections.find((s) => s.name === remote);
  const lines = section && [
    `type = ${section.type}`,
    ...section.options.map((o) => `${o.name} = ${o.hidden ? t("storage.rclone.hidden") : (o.value ?? "")}`),
  ];

  return (
    <Card title="rclone.conf" hueIndex={hueIndex}>
      <Rows>
        <SettingRow label={t("storage.rclone.section")}>
          <code dir="ltr" className="break-all text-start font-mono text-xs text-carbon-textSub">
            [{remote}]
          </code>
        </SettingRow>
        {lines && (
          <pre
            dir="ltr"
            aria-label={`[${remote}]`}
            className="my-3 overflow-x-auto rounded-control bg-carbon-surface2 p-3 text-start font-mono text-xs leading-relaxed text-carbon-textSub whitespace-pre"
          >
            {lines.join("\n")}
          </pre>
        )}
        {sections.length > 0 && (
          <SettingRow label={t("storage.rclone.all")}>
            <code dir="ltr" className="break-all text-start font-mono text-xs text-carbon-textSub">
              {sections.map((s) => s.name).join(" · ")}
            </code>
          </SettingRow>
        )}
      </Rows>
    </Card>
  );
}
