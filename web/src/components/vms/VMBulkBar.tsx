import { BULK_HUE } from "../../lib/bulkHue";
import { useT } from "../../lib/i18n";
import { Button } from "../Button";
import { busyPhraseKey } from "../../lib/progress";

type T = ReturnType<typeof useT>["t"];

// VMBulkBar acts on the ticked VMs and is the same bar at both widths.
export function VMBulkBar({
  t,
  count,
  busy,
  running,
  onBackup,
  onRestore,
  onClear,
}: {
  t: T;
  count: number;
  busy: boolean;
  running: { active: boolean; phase?: string };
  onBackup: () => void;
  onRestore: () => void;
  onClear: () => void;
}) {
  return (
    <div className="flex items-center gap-3 flex-wrap rounded-card bg-carbon-surface2 px-3 py-2">
      <span className="text-xs text-carbon-textSub">
        {count} {t("containers.selectedCount")}
      </span>
      <Button
        label={t("vms.backupSelected")}
        labelKey="vms.backupSelected"
        hueIndex={BULK_HUE.backup}
        tone="accent"
        onClick={onBackup}
        disabled={busy || running.active}
      />
      <Button
        label={t("vms.restoreSelected")}
        labelKey="vms.restoreSelected"
        hueIndex={BULK_HUE.restore}
        tone="accent"
        onClick={onRestore}
        disabled={busy || running.active}
      />
      <Button
        label={t("containers.clearSelection")}
        labelKey="containers.clearSelection"
        tone="neutral"
        onClick={onClear}
        disabled={busy}
      />
      {busy && <span className="text-xs text-carbon-textMuted">{t("containers.working")}</span>}
      {!busy && running.active && (
        <span className="text-xs text-carbon-textMuted">{t(busyPhraseKey(running.phase))}</span>
      )}
    </div>
  );
}
