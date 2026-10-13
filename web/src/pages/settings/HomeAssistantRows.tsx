import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "../../components/Button";
import { NumberField } from "../../components/NumberField";
import { RevealInput } from "../../components/RevealInput";
import { SettingRow } from "../../components/storage/rows";
import { Toggle } from "../../components/Toggle";
import { getHomeAssistant, setHomeAssistant, type HomeAssistantSettings } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { useReveal } from "../../lib/useReveal";
import { ToggleRow } from "./shared";

// HomeAssistantRows connects BombVault to the MQTT broker Home Assistant
// reads, where it shows up as a device through MQTT discovery. The form saves
// as a whole, because every save reconnects; switching it off saves at once.

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

export function HomeAssistantRows() {
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

  async function save(next = form) {
    if (!next) return;
    setBusy(true);
    try {
      const res = await setHomeAssistant(next);
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

  function toggle(on: boolean) {
    if (!form) return;
    if (!on && saved?.enabled) void save({ ...form, enabled: false });
    else edit({ enabled: on });
  }

  const fieldCls =
    "w-80 max-w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-start font-mono text-sm text-carbon-text glim-field-focus";

  let statusText = t("ha.statusOff");
  if (saved?.enabled && saved.status.connected) {
    statusText = t("ha.statusConnected");
  } else if (saved?.enabled && saved.status.error) {
    statusText = t("ha.statusError").replace("{error}", saved.status.error);
  } else if (saved?.enabled) {
    statusText = t("ha.statusConnecting");
  }
  // The server keeps a stored password only for the broker and user it was
  // entered for, so the field stops showing it as set once either changes.
  const passwordKept =
    !!saved?.passwordSet &&
    form !== null &&
    form.host.trim() === saved.host &&
    form.port === saved.port &&
    form.username.trim() === saved.username;

  return (
    <>
      <SettingRow
        label={t("ha.title")}
        hint={t("ha.hint")}
        note={failed ? t("ha.loadFailed") : form === null ? t("folder.loading") : statusText}
      >
        {form !== null && (
          <Toggle hideLabel label={t("ha.enable")} checked={form.enabled} onChange={toggle} disabled={busy} />
        )}
      </SettingRow>
      {form?.enabled && (
        <div key={shake} className={`flex flex-col divide-y divide-carbon-border/60 py-3 ps-4 ${shake ? "glim-shake" : ""}`}>
          <SettingRow label={t("ha.host")}>
            <input
              value={form.host}
              onChange={(e) => edit({ host: e.target.value })}
              aria-label={t("ha.host")}
              spellCheck={false}
              autoComplete="off"
              dir="ltr"
              placeholder={t("ha.hostPlaceholder")}
              className={fieldCls}
            />
          </SettingRow>
          <SettingRow label={t("notify.smtpPort")}>
            <NumberField
              value={form.port}
              onChange={(e) => edit({ port: Number(e.target.value) || 0 })}
              aria-label={t("notify.smtpPort")}
              min={1}
              max={65535}
              className="w-28 rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text glim-field-focus"
            />
          </SettingRow>
          <SettingRow label={t("notify.smtpUser")}>
            <input
              value={form.username}
              onChange={(e) => edit({ username: e.target.value })}
              aria-label={t("notify.smtpUser")}
              spellCheck={false}
              autoComplete="off"
              dir="ltr"
              className={fieldCls}
            />
          </SettingRow>
          <SettingRow label={t("notify.smtpPass")}>
            <RevealInput
              {...reveal}
              value={form.password}
              onChange={(e) => edit({ password: e.target.value })}
              aria-label={t("notify.smtpPass")}
              spellCheck={false}
              autoComplete="new-password"
              placeholder={passwordKept ? t("cloud.secretSet") : ""}
              wrapperClassName="w-80 max-w-full"
              className="w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-start font-mono text-sm text-carbon-text glim-field-focus"
            />
          </SettingRow>
          <SettingRow label={t("ha.prefix")} hint={t("ha.prefixHint")}>
            <input
              value={form.prefix}
              onChange={(e) => edit({ prefix: e.target.value })}
              aria-label={t("ha.prefix")}
              spellCheck={false}
              autoComplete="off"
              dir="ltr"
              className={fieldCls}
            />
          </SettingRow>
          <div className="py-3">
            <ToggleRow label={t("ha.tls")} hint={t("ha.tlsHint")} checked={form.tls} onChange={(v) => edit({ tls: v })} />
          </div>
          <div className="py-3">
            <ToggleRow
              label={t("ha.buttons")}
              hint={t("ha.buttonsHint")
                .replace("{minutes}", String(limits.cooldownMinutes))
                .replace("{perDay}", String(limits.itemStartsPerDay))}
              checked={form.buttons}
              onChange={(v) => edit({ buttons: v })}
            />
          </div>
          <div className="flex justify-end pt-3">
            <Button
              label={t("settings.save")}
              labelKey="settings.save"
              onClick={() => void save()}
              disabled={busy}
              busy={busy}
            />
          </div>
        </div>
      )}
    </>
  );
}
