import { useCallback, useState } from "react";
import { setBackupDeclined, type BackupItem } from "./api";
import { useT } from "./i18n";
import { useToast } from "./toast";

/**
 * useDeclineBackup says not to back an entry up, or takes that back. A
 * refusal means the entry is no longer what the page shows, so `onChanged`
 * runs after every answer of the server.
 */
export function useDeclineBackup(onChanged: () => void) {
  const { t } = useT();
  const { push } = useToast();
  const [busy, setBusy] = useState(false);

  const send = useCallback(
    async (item: BackupItem, declined: boolean): Promise<boolean> => {
      setBusy(true);
      try {
        const res = await setBackupDeclined(item.kind, item.key, declined);
        if (!res.ok) push(res.error ?? t("common.saveFailed"), "fail");
        return res.ok;
      } catch (err) {
        push(err instanceof Error ? err.message : t("common.saveFailed"), "fail");
        return false;
      } finally {
        setBusy(false);
        onChanged();
      }
    },
    [push, t, onChanged]
  );

  const accept = useCallback((item: BackupItem) => send(item, false), [send]);

  /** The toast that reports it carries the way back. */
  const decline = useCallback(
    async (item: BackupItem, name: string) => {
      if (!(await send(item, true))) return;
      push(
        t("backups.declined").replace("{name}", () => name),
        "success",
        { label: t("takeover.undo"), onClick: () => void accept(item) }
      );
    },
    [send, accept, push, t]
  );

  return { decline, accept, busy };
}
