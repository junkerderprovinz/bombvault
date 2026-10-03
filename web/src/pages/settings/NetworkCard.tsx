import { useEffect, useState } from "react";
import { getMdns, setMdns, type MdnsState } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { Card, ToggleRow } from "./shared";

// NetworkCard switches the mDNS announcement that lets a browser find
// BombVault as bombvault.local.
export function NetworkCard({ hueIndex }: { hueIndex?: number }) {
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
    <Card title={t("mdns.title")} hint={t("mdns.hint")} hueIndex={hueIndex}>
      {state !== null && (
        <>
          <ToggleRow
            label={t("mdns.enable")}
            checked={state.enabled}
            onChange={(v) => void toggle(v)}
            disabled={busy}
            shakeNonce={shake}
          />
          <p className="text-sm text-carbon-textSub wrap-anywhere" role="status">
            {status}
          </p>
        </>
      )}
    </Card>
  );
}
