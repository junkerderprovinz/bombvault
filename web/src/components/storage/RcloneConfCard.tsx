import { useT } from "../../lib/i18n";
import { useRcloneRemotes } from "../../lib/useRcloneRemotes";
import { Card } from "../../pages/settings/shared";
import { Rows, SettingRow } from "./rows";

/**
 * RcloneConfCard names the section of BombVault's rclone.conf a location goes
 * through, among all the remotes that file holds. The server hands out the
 * names only, never a section's contents.
 */
export function RcloneConfCard({ remote, hueIndex }: { remote: string; hueIndex: number }) {
  const { t } = useT();
  const { remotes } = useRcloneRemotes();
  return (
    <Card title="rclone.conf" hueIndex={hueIndex}>
      <Rows>
        <SettingRow label={t("storage.rclone.section")}>
          <code dir="ltr" className="break-all text-start font-mono text-xs text-carbon-textSub">
            [{remote}]
          </code>
        </SettingRow>
        {remotes.length > 0 && (
          <SettingRow label={t("storage.rclone.all")}>
            <code dir="ltr" className="break-all text-start font-mono text-xs text-carbon-textSub">
              {remotes.join(" · ")}
            </code>
          </SettingRow>
        )}
      </Rows>
    </Card>
  );
}
