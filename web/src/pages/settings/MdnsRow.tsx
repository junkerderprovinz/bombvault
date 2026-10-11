import { useEffect, useState } from "react";
import { getMdns, setMdns, type MdnsState } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { SettingRow } from "../../components/storage/rows";
import { Toggle } from "../../components/Toggle";

// MdnsRow switches the mDNS announcement that lets a browser find BombVault as
// bombvault.local.
export function MdnsRow() {
  const { t } = useT();
  const { push } = useToast();
  const [state, setState] = useState<MdnsState | null>(null);
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);

  useEffect(() => {
    getMdns()
      .then((res) => {
        if (res.ok) setState(res);
      })
      .catch(() => {});
  }, []);

  async function toggle(on: boolean) {
    setBusy(true);
    try {
      const res = await setMdns(on);
      if (!res.ok) throw new Error(res.error || t("common.saveFailed"));
      setState(res);
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.saveFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  let status = t("mdns.statusOff");
  if (state?.running) status = t("mdns.statusOn").replace("{url}", state.url);
  else if (state?.enabled && state.error) status = t("mdns.statusError").replace("{error}", state.error);
  else if (state?.enabled) status = t("mdns.statusStarting");

  return (
    <SettingRow label={t("mdns.enable")} hint={t("mdns.hint")} note={state === null ? undefined : status}>
      {state !== null && (
        <Toggle
          key={shake}
          hideLabel
          label={t("mdns.enable")}
          checked={state.enabled}
          onChange={(v) => void toggle(v)}
          disabled={busy}
          className={shake ? "glim-shake" : ""}
        />
      )}
    </SettingRow>
  );
}
