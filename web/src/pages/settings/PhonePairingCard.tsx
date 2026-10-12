import { useEffect, useState } from "react";
import { getPhonePairing, setPhonePairing } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { secretsSentence } from "../../lib/secretsRefusal";
import { useToast } from "../../lib/toast";
import { Card, ToggleRow } from "./shared";

/** PhonePairingCard switches whether a phone of the group may pair with the
 *  twelve words. A password reset switches it off. */
export function PhonePairingCard({ hueIndex }: { hueIndex: number }) {
  const { t } = useT();
  const { push } = useToast();
  const [on, setOn] = useState<boolean | null>(null);
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);
  const [pulse, setPulse] = useState(0);

  useEffect(() => {
    let alive = true;
    getPhonePairing()
      .then((res) => {
        if (alive && res.ok) setOn(res.on ?? true);
      })
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);

  async function change(next: boolean) {
    setBusy(true);
    try {
      const res = await setPhonePairing(next);
      if (res.ok) {
        setOn(next);
        setPulse((n) => n + 1);
        push(t("settings.saved"), "success");
      } else {
        setShake((n) => n + 1);
        push(secretsSentence(t, res.code) ?? res.error ?? t("auth.saveError"), "fail");
      }
    } catch {
      setShake((n) => n + 1);
      push(t("auth.saveError"), "fail");
    } finally {
      setBusy(false);
    }
  }

  if (on === null) return null;
  return (
    <Card title={t("apps.phone.title")} hueIndex={hueIndex}>
      <ToggleRow
        label={t("auth.phonePairing")}
        hint={t("auth.phonePairingHint")}
        checked={on}
        onChange={(v) => void change(v)}
        disabled={busy}
        shakeNonce={shake}
        pulseNonce={pulse}
      />
    </Card>
  );
}
