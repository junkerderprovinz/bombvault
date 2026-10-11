import type { TranslationKey, useT } from "../lib/i18n";

type T = ReturnType<typeof useT>["t"];

// Per-browser acknowledgement of the stop warning. Storage keys keep the bv-
// prefix, since renaming one resets every user's stored state.
export const STOP_ACK_KEY = "bv-container-stop-ack";

/**
 * stopAcknowledged asks whether a container may be stopped for its backup,
 * which takes minutes on a first full one. It asks once per browser: a warning
 * on every press gets clicked away unread, and it is about how backups work,
 * not about one container.
 */
export async function stopAcknowledged(
  confirm: (message: string, options: { confirmKey: TranslationKey }) => Promise<boolean>,
  t: T
): Promise<boolean> {
  try {
    if (localStorage.getItem(STOP_ACK_KEY) === "1") return true;
  } catch {
    // Without storage it asks every time.
  }
  if (!(await confirm(t("containers.stopWarning"), { confirmKey: "containers.backupNow" }))) return false;
  try {
    localStorage.setItem(STOP_ACK_KEY, "1");
  } catch {
    // It asks again next time.
  }
  return true;
}
