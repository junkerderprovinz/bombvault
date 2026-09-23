import { useState } from "react";
import {
  listSnapshots,
  listVMSnapshots,
  takeOverContainer,
  takeOverVM,
  unlinkContainerAlias,
  unlinkVMAlias,
  type ListSnapshotsResponse,
  type OkEnvelope,
} from "./api";
import type { TranslationKey, useT } from "./i18n";
import { useConfirm } from "./useConfirm";
import { useToast } from "./toast";

type T = ReturnType<typeof useT>["t"];

/** The calls that link a former entry to a card's entry and unlink it again. */
export interface TakeoverApi {
  takeOver: (name: string, from: string) => Promise<OkEnvelope>;
  unlink: (name: string, old: string) => Promise<OkEnvelope>;
  snapshots: (name: string) => Promise<ListSnapshotsResponse>;
}

export const containerTakeover: TakeoverApi = {
  takeOver: takeOverContainer,
  unlink: unlinkContainerAlias,
  snapshots: listSnapshots,
};

export const vmTakeover: TakeoverApi = {
  takeOver: takeOverVM,
  unlink: unlinkVMAlias,
  snapshots: listVMSnapshots,
};

export interface TakeoverEntry {
  /** The name the routes take: the container's, or the VM's libvirt name. */
  name: string;
  displayName: string;
  api: TakeoverApi;
}

/** How many backups the entry under name has, or null when the list could not be read. */
export async function countBackups(api: TakeoverApi, name: string): Promise<number | null> {
  try {
    const res = await api.snapshots(name);
    return res.ok ? (res.snapshots ?? []).length : null;
  } catch {
    return null;
  }
}

export function backupCountText(t: T, n: number): string {
  return t("takeover.backups", n);
}

function historyKey(backups: number | null): TranslationKey {
  return backups === null ? "takeover.historyUnknown" : "takeover.historyMany";
}

/**
 * Takes a former entry over onto entry, and unlinks one again, behind the
 * confirmation dialog it returns. A refusal toasts the server's reason and calls
 * onRefused, so the caller can shake the button that asked.
 */
export function useTakeOver(entry: TakeoverEntry, onDone: () => void, t: T) {
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [busy, setBusy] = useState(false);

  async function send(call: () => Promise<OkEnvelope>, fallback: TranslationKey, onRefused: () => void) {
    setBusy(true);
    try {
      const res = await call();
      if (res.ok) {
        onDone();
        return true;
      }
      push(res.error ?? t(fallback), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t(fallback), "fail");
    } finally {
      setBusy(false);
    }
    onRefused();
    return false;
  }

  function unlinkNow(old: string, onRefused: () => void) {
    return send(() => entry.api.unlink(entry.name, old), "takeover.unlinkFailed", onRefused);
  }

  async function takeOver(from: string, backups: number | null, onRefused: () => void) {
    const history = t(historyKey(backups), backups ?? undefined)
      .replace("{old}", from)
      .replace("{new}", entry.displayName);
    const message = t("takeover.confirm")
      .replace("{old}", from)
      .replace("{new}", entry.displayName)
      .replace("{history}", history);
    if (!(await confirm(message, { confirmKey: "takeover.accept" }))) return;
    if (!(await send(() => entry.api.takeOver(entry.name, from), "takeover.failed", onRefused))) return;
    push(t("takeover.done").replace("{old}", from).replace("{new}", entry.displayName), "success", {
      label: t("takeover.undo"),
      onClick: () => void unlinkNow(from, () => {}),
    });
  }

  async function unlink(old: string, onRefused: () => void) {
    const message = t("takeover.unlinkConfirm").replace("{old}", old).replace("{new}", entry.displayName);
    if (!(await confirm(message, { confirmKey: "takeover.unlink" }))) return;
    await unlinkNow(old, onRefused);
  }

  return { takeOver, unlink, busy, confirmDialog };
}
