import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "../../components/Button";
import { InfoBubble } from "../../components/InfoBubble";
import { NumberField } from "../../components/NumberField";
import { RevealInput } from "../../components/RevealInput";
import { getHomeAssistant, setHomeAssistant, type HomeAssistantSettings } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { useReveal } from "../../lib/useReveal";
import { Card, ToggleRow } from "./shared";

// HomeAssistantCard connects BombVault to the MQTT broker Home Assistant
// reads, where it shows up as a device through MQTT discovery. The form saves
// as a whole, because every save reconnects.

const CODE_MESSAGE: Record<string, TranslationKey> = {
  "mqtt-host-invalid": "ha.hostInvalid",
  "mqtt-port-invalid": "ha.portInvalid",
  "mqtt-prefix-invalid": "ha.prefixInvalid",
};

/** How often the card asks again while the connection is not up yet. */
const STATUS_POLL_MS = 5000;

type Form = {
  enabled: boolean;
  host: string;
  port: number;
  username: string;
  password: string;
  tls: boolean;
  prefix: string;
  buttons: boolean;
};

function formOf(s: HomeAssistantSettings): Form {
  return {
    enabled: s.enabled,
    host: s.host,
    port: s.port,
    username: s.username,
    password: "",
    tls: s.tls,
    prefix: s.prefix,
    buttons: s.buttons,
  };
}

export function HomeAssistantCard({ hueIndex }: { hueIndex?: number }) {
  const { t } = useT();
  const { push } = useToast();
  const [saved, setSaved] = useState<HomeAssistantSettings | null>(null);
  const [form, setForm] = useState<Form | null>(null);
  const [failed, setFailed] = useState(false);
  const [limits, setLimits] = useState({ cooldownMinutes: 0, itemStartsPerDay: 0 });
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);
  const reveal = useReveal();
  const dirty = useRef(false);

  const load = useCallback(async () => {
    try {
      const res = await getHomeAssistant();
      if (!res.ok || !res.settings) {
        setFailed(true);
        return;
      }
      setSaved(res.settings);
      setLimits({ cooldownMinutes: res.cooldownMinutes ?? 0, itemStartsPerDay: res.itemStartsPerDay ?? 0 });
      setFailed(false);
      // A poll must not throw away what the operator is typing.
      if (!dirty.current) setForm(formOf(res.settings));
    } catch {
      setFailed(true);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const waiting = saved?.enabled === true && !saved.status.connected;
  useEffect(() => {
    if (!waiting) return;
    const timer = window.setInterval(() => void load(), STATUS_POLL_MS);
    return () => window.clearInterval(timer);
  }, [waiting, load]);

  function edit(patch: Partial<Form>) {
    dirty.current = true;
    setForm((f) => (f ? { ...f, ...patch } : f));
  }

  async function save() {
    if (!form) return;
    setBusy(true);
    try {
      const res = await setHomeAssistant(form);
      if (!res.ok || !res.settings) {
        const key = res.code ? CODE_MESSAGE[res.code] : undefined;
        push(key ? t(key) : (res.error ?? t("common.saveFailed")), "fail");
        setShake((n) => n + 1);
        return;
      }
      dirty.current = false;
      setSaved(res.settings);
      setForm(formOf(res.settings));
      if (res.warning === "mqtt-remove-failed") push(t("ha.removeFailed"), "fail");
      else push(t("settings.saved"), "success");
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.saveFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  const fieldCls =
    "w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-start font-mono text-sm text-carbon-text glim-field-focus";
  const labelCls = "flex flex-col gap-1.5 text-xs text-carbon-textSub";

  let statusText = t("ha.statusOff");
  let dot = "bg-carbon-textMuted";
  if (saved?.enabled && saved.status.connected) {
    statusText = t("ha.statusConnected");
    dot = "bg-statusOkSolid";
  } else if (saved?.enabled && saved.status.error) {
    statusText = t("ha.statusError").replace("{error}", saved.status.error);
    dot = "bg-statusWarn";
  } else if (saved?.enabled) {
    statusText = t("ha.statusConnecting");
  }

  return (
    <Card title={t("ha.title")} hint={t("ha.hint")} hueIndex={hueIndex}>
      {failed && <p className="text-sm text-statusWarn">{t("ha.loadFailed")}</p>}
      {!failed && form === null && <p className="text-sm text-carbon-textMuted">{t("folder.loading")}</p>}
      {form !== null && (
        <>
          <div className="flex items-center gap-2" role="status">
            <span className={`inline-block h-2 w-2 shrink-0 rounded-full ${dot}`} />
            <span className="text-sm text-carbon-text wrap-anywhere">{statusText}</span>
          </div>

          <ToggleRow label={t("ha.enable")} checked={form.enabled} onChange={(v) => edit({ enabled: v })} />

          <div key={shake} className={`grid grid-cols-1 gap-3 md:grid-cols-2${shake ? " glim-shake" : ""}`}>
            <label className={labelCls}>
              {t("ha.host")}
              <input
                value={form.host}
                onChange={(e) => edit({ host: e.target.value })}
                spellCheck={false}
                autoComplete="off"
                dir="ltr"
                placeholder={t("ha.hostPlaceholder")}
                className={fieldCls}
              />
            </label>
            <label className={labelCls}>
              {t("notify.smtpPort")}
              <NumberField
                value={form.port}
                onChange={(e) => edit({ port: Number(e.target.value) || 0 })}
                min={1}
                max={65535}
                className={fieldCls}
              />
            </label>
            <label className={labelCls}>
              {t("notify.smtpUser")}
              <input
                value={form.username}
                onChange={(e) => edit({ username: e.target.value })}
                spellCheck={false}
                autoComplete="off"
                dir="ltr"
                className={fieldCls}
              />
            </label>
            <label className={labelCls}>
              {t("notify.smtpPass")}
              <RevealInput
                {...reveal}
                value={form.password}
                onChange={(e) => edit({ password: e.target.value })}
                spellCheck={false}
                autoComplete="new-password"
                placeholder={saved?.passwordSet ? t("cloud.secretSet") : ""}
                wrapperClassName="w-full"
                className={fieldCls}
              />
            </label>
            <div className={labelCls}>
              <span className="flex items-center gap-1.5">
                <label htmlFor="bv-ha-prefix">{t("ha.prefix")}</label>
                <InfoBubble tip={t("ha.prefixHint")} />
              </span>
              <input
                id="bv-ha-prefix"
                value={form.prefix}
                onChange={(e) => edit({ prefix: e.target.value })}
                spellCheck={false}
                autoComplete="off"
                dir="ltr"
                className={fieldCls}
              />
            </div>
          </div>

          <ToggleRow label={t("ha.tls")} hint={t("ha.tlsHint")} checked={form.tls} onChange={(v) => edit({ tls: v })} />
          <ToggleRow
            label={t("ha.buttons")}
            hint={t("ha.buttonsHint")
              .replace("{minutes}", String(limits.cooldownMinutes))
              .replace("{perDay}", String(limits.itemStartsPerDay))}
            checked={form.buttons}
            onChange={(v) => edit({ buttons: v })}
          />

          <div className="flex justify-end">
            <Button
              label={t("settings.save")}
              labelKey="settings.save"
              tone="accent"
              onClick={() => void save()}
              disabled={busy}
              busy={busy}
              hueIndex={hueIndex}
            />
          </div>
        </>
      )}
    </Card>
  );
}
