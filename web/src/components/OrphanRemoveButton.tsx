import { useState } from "react";
import type { OkEnvelope } from "../lib/api";
import type { useT } from "../lib/i18n";
import { useConfirm } from "../lib/useConfirm";
import { useToast } from "../lib/toast";
import { Button } from "./Button";

type T = ReturnType<typeof useT>["t"];

interface OrphanRemoveButtonProps {
  /** Whether the entry still has backups, read off its last-backup time. A VM
   *  entry rebuilt by Discover has no run record and, unlike a container, no
   *  snapshot-time fallback, so it reads as none: the button then removes only
   *  the entry, and its snapshots stay for the next Discover to find. */
  hasBackups: boolean;
  /** Confirmation texts, already translated; each page names its own kind of item. */
  deleteConfirm: string;
  removeConfirm: string;
  /** Deletes every backup and the entry. */
  deleteBackups: () => Promise<OkEnvelope>;
  /** Removes only the entry, never a snapshot. */
  removeEntry: () => Promise<OkEnvelope>;
  onDone: () => void;
  t: T;
}

// The removal button on a not-installed card on the Containers and VMs pages.
// With backups it is "Delete all backups", because removing only the entry
// would leave snapshots in the repository that nothing on the page points at.
// Without backups it is "Remove entry". Keeping the backups while dropping the
// entry from the schedule is the card's schedule switch.
//
// Neutral rather than red, like every destructive action in the app: the label
// and the confirmation carry the meaning.
export function OrphanRemoveButton({
  hasBackups,
  deleteConfirm,
  removeConfirm,
  deleteBackups,
  removeEntry,
  onDone,
  t,
}: OrphanRemoveButtonProps) {
  const [pending, setPending] = useState(false);
  const [shake, setShake] = useState(0);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const failText = t(hasBackups ? "common.deleteFailed" : "common.removeFailed");

  async function run() {
    // TODO: pass what is at stake ("N snapshots, X GB") to confirm() once the
    // interpolated i18n keys exist, as in VMs.tsx and Files.tsx.
    const asked = hasBackups
      ? confirm(deleteConfirm, { confirmKey: "containers.deleteBackups" })
      : confirm(removeConfirm, { confirmKey: "vms.removeEntry" });
    if (!(await asked)) return;
    setPending(true);
    try {
      const res = await (hasBackups ? deleteBackups() : removeEntry());
      if (res.ok) onDone();
      else {
        push(res.error ?? failText, "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : failText, "fail");
      setShake((n) => n + 1);
    } finally {
      setPending(false);
    }
  }

  // Two literal call sites rather than one with the key in a variable, because
  // glyphFor.reach.test.ts reads labelKey statically.
  const shared = {
    tone: "neutral" as const,
    onClick: () => void run(),
    disabled: pending,
    busy: pending,
    title: pending ? t("dashboard.checking") : undefined,
    className: shake ? "glim-shake" : "",
  };
  return (
    <>
      {hasBackups ? (
        <Button key={shake} label={t("containers.deleteBackups")} labelKey="containers.deleteBackups" {...shared} />
      ) : (
        <Button key={shake} label={t("vms.removeEntry")} labelKey="vms.removeEntry" {...shared} />
      )}
      {confirmDialog}
    </>
  );
}
