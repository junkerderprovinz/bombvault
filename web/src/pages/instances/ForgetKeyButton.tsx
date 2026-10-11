import { useState } from "react";

import { Button } from "../../components/Button";
import { forgetMemberKey } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";

/** ForgetKeyButton drops the signing key this instance pinned for a member,
 *  on the second click. The member names its key again on its next signed
 *  call, so a key somebody else pinned in its name can be cleared. */
export function ForgetKeyButton({ memberId }: { memberId: string }) {
  const { t } = useT();
  const { push } = useToast();
  const [armed, setArmed] = useState(false);
  const [busy, setBusy] = useState(false);

  async function forget() {
    setBusy(true);
    try {
      const res = await forgetMemberKey(memberId);
      if (res.ok) {
        setArmed(false);
        push(t("instances.keyForgotten"), "success");
        return;
      }
      push(res.error ?? t("fleet.saveError"), "fail");
    } catch (err) {
      push(err instanceof Error ? err.message : t("fleet.saveError"), "fail");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Button
      label={t(armed ? "instances.forgetKeyConfirm" : "instances.forgetKey")}
      labelKey={armed ? "instances.forgetKeyConfirm" : "instances.forgetKey"}
      tone="neutral"
      hint={t("instances.forgetKeyHint")}
      onClick={() => (armed ? void forget() : setArmed(true))}
      disabled={busy}
      busy={busy}
    />
  );
}
