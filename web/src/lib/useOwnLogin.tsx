// What hands out or creates key material is refused to the session the phone
// app opens this instance with, since the group's words alone must not reach
// a key. The person signs in at this instance's own interface and does it
// again from there.
import type { ReactNode } from "react";
import { logout } from "./api";
import { useT } from "./i18n";
import { useConfirm } from "./useConfirm";

/**
 * useOwnLogin offers the sign-in a refused call needs. `handled` takes the
 * answer and asks the person to sign in when it is "own-login-needed"; the
 * caller then has nothing more to say about it. Signing in drops the phone's
 * session, and the login screen comes up on the same page.
 */
export function useOwnLogin(): { handled: (answer: { code?: string }) => boolean; dialog: ReactNode } {
  const { t } = useT();
  const { confirm, confirmDialog } = useConfirm();

  async function offer() {
    const yes = await confirm(t("secrets.ownLoginNeeded"), {
      confirmLabel: t("secrets.signInHere"),
      confirmLabelKey: "auth.signIn",
    });
    if (!yes) return;
    await logout().catch(() => undefined);
    window.location.reload();
  }

  return {
    handled: (answer) => {
      if (answer.code !== "own-login-needed") return false;
      void offer();
      return true;
    },
    dialog: confirmDialog,
  };
}
