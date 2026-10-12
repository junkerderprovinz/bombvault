// What hands out or creates key material, or deletes backups for good, is
// refused to the session the phone app opens this instance with, since the
// group's words alone must not reach a key or the backups. The person signs in
// at this instance's own interface and does it again from there.
import type { ReactNode } from "react";
import { logout } from "./api";
import { useT } from "./i18n";
import { useConfirm } from "./useConfirm";

/**
 * useOwnLogin offers the sign-in a refused call needs. `handled` takes the
 * answer and asks the person to sign in when it is "own-login-needed"; the
 * caller then has nothing more to say about it, and `declined` runs when the
 * person does not sign in. Signing in drops the phone's session, and the login
 * screen comes up on the same page.
 */
export function useOwnLogin(): {
  handled: (answer: { code?: string }, declined?: () => void) => boolean;
  dialog: ReactNode;
} {
  const { t } = useT();
  const { confirm, confirmDialog } = useConfirm();

  async function offer(declined?: () => void) {
    const yes = await confirm(t("secrets.ownLoginNeeded"), {
      confirmLabel: t("secrets.signInHere"),
      confirmLabelKey: "auth.signIn",
    });
    if (!yes) {
      declined?.();
      return;
    }
    await logout().catch(() => undefined);
    window.location.reload();
  }

  return {
    handled: (answer, declined) => {
      if (answer.code !== "own-login-needed") return false;
      void offer(declined);
      return true;
    },
    dialog: confirmDialog,
  };
}
