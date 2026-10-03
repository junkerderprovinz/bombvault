import { useEffect, useState } from "react";
import { setVMBlockBackup, type VM } from "../lib/api";
import { useT, type TranslationKey } from "../lib/i18n";
import { ToggleRow } from "../pages/settings/shared";
import { useToast } from "../lib/toast";

const REASONS = new Set([
  "first",
  "newer_backup",
  "no_checkpoint",
  "disks_changed",
  "checkpoint_broken",
  "chain_broken",
  "vm_off",
  "disk_format",
  "zvol",
  "no_ssh",
  "unsupported",
  "no_space",
  "job_failed",
]);

/** The line under the switch that says how the last backup read the disks,
 *  or null before the first run with the switch on. */
export function blockModeLine(t: (k: TranslationKey) => string, mode?: string, reason?: string): string | null {
  const why = reason && REASONS.has(reason) ? t(`vm.blocks.reason.${reason}` as TranslationKey) : "";
  switch (mode) {
    case "changed":
      return t("vm.blocks.last.changed");
    case "full":
      return why ? t("vm.blocks.last.full").replace("{reason}", why) : t("vm.blocks.last.fullPlain");
    case "classic":
      return why ? t("vm.blocks.last.classic").replace("{reason}", why) : t("vm.blocks.last.classicPlain");
    default:
      return null;
  }
}

/** VMBlockToggle switches changed-block backups for one VM and shows how the
 *  last backup read its disks. */
export function VMBlockToggle({ vm }: { vm: VM }) {
  const { t } = useT();
  const { push } = useToast();
  const [enabled, setEnabled] = useState(!!vm.blockBackup);
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);

  useEffect(() => setEnabled(!!vm.blockBackup), [vm.blockBackup]);

  async function handleChange(next: boolean) {
    setBusy(true);
    try {
      const res = await setVMBlockBackup(vm.libvirtName, next);
      if (res.ok) {
        setEnabled(next);
      } else {
        push(res.error ?? t("vm.blocks.saveFailed"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("vm.blocks.saveFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  // The line describes the server's last run, so it only shows while the
  // switch the list reported is still on.
  const line = enabled && vm.blockBackup ? blockModeLine(t, vm.blockMode, vm.blockReason) : null;
  return (
    <div className="flex flex-col gap-1">
      <ToggleRow
        label={t("vm.blocks")}
        hint={t("vm.blocks.hint")}
        checked={enabled}
        onChange={(next) => void handleChange(next)}
        disabled={busy}
        shakeNonce={shake}
      />
      {line && <span className="text-xs text-carbon-textMuted text-end">{line}</span>}
    </div>
  );
}
