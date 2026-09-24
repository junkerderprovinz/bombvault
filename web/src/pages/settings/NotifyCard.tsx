import { Button } from "../../components/Button";
import { NumberField } from "../../components/NumberField";
import { SelectField } from "../../components/SelectField";
import { Card, ToggleRow, type SaveState } from "./shared";
import { InfoBubble } from "../../components/InfoBubble";
import { RevealInput } from "../../components/RevealInput";
import { HUE_OFFSET, Selector } from "../../components/Selector";
import { getNotify, setNotify, testNotify, type NotifyConfig } from "../../lib/api";
import { tLtr } from "../../lib/ltrFragments";
import { useAdvanced } from "../../lib/advanced";
import { useEffect, useRef, useState } from "react";
import { useReveal } from "../../lib/useReveal";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";

// emptyNotify is the default notification config shown before the saved one loads.
const emptyNotify: NotifyConfig = {
  on: "never",
  webhookEnabled: false,
  webhookUrl: "",
  webhookFormat: "generic",
  matrixEnabled: false,
  matrixHomeserver: "",
  matrixToken: "",
  matrixRoom: "",
  healthchecksUrl: "",
  unraid: false,
  smtpEnabled: false,
  smtpHost: "",
  smtpPort: 587,
  smtpUsername: "",
  smtpPassword: "",
  smtpFrom: "",
  smtpTo: "",
  smtpTls: "starttls",
  appriseEnabled: false,
  appriseUrl: "",
  appriseTags: "",
  scheduledSummary: false,
  notifyOnUpdate: false,
};

// NotifyCard configures backup notifications. The config is stored encrypted,
// the form pre-fills from it, and Test sends to the current form values
// whether or not they have been saved.
//
// It renders three cards (settings, channels, Healthchecks) that all edit one
// NotifyConfig, which is why it is one component with three hue props.
export function NotifyCard({
  t,
  platformKind,
  hueIndex,
  channelsHueIndex,
  healthchecksHueIndex,
}: {
  t: ReturnType<typeof useT>["t"];
  // The detected or overridden platform kind ("unraid" | "generic" |
  // "truenas"). The mismatch banner below the Unraid toggle reads it.
  platformKind: string;
  hueIndex?: number;
  // The channels card renders only in advanced mode, so the call site has to
  // take this from a nextHue() call inside that same gate. An unconditional
  // nextHue() would burn a slot with Advanced off and shift every later
  // heading on the tab by one.
  channelsHueIndex?: number;
  // Same rule as channelsHueIndex.
  healthchecksHueIndex?: number;
}) {
  const { push } = useToast();
  // Simple mode still gets notify-on-failure via Unraid; the extra channels
  // (webhook/Matrix/Healthchecks/SMTP) are power-user features, so gate those.
  const { advanced } = useAdvanced();
  const [cfg, setCfg] = useState<NotifyConfig>(emptyNotify);
  const [, setState] = useState<SaveState>("idle");
  // The SMTP password / Matrix token are never sent to the browser; track whether
  // one is stored so the field shows "configured" and a blank submit keeps it.
  const [secretSet, setSecretSet] = useState({ smtp: false, matrix: false });
  const revealMatrixToken = useReveal();
  const revealSmtpPassword = useReveal();
  // The config as the server last confirmed it. A failed save reverts its
  // field to this value, so the form never shows an edit that did not persist.
  const lastGoodRef = useRef<NotifyConfig>(emptyNotify);
  // A counter per field. Each control uses its counter as its key, so bumping
  // it remounts the control and replays the .glim-shake animation.
  const [fieldShake, setFieldShake] = useState<Partial<Record<string, number>>>({});
  function bumpShake(key: string) {
    setFieldShake((sh) => ({ ...sh, [key]: (sh[key] ?? 0) + 1 }));
  }
  // Every field saves itself. A blank secret field keeps the stored secret, so
  // auto-saving the two secret fields is safe.
  const debounceTimers = useRef<Record<string, ReturnType<typeof setTimeout>>>({});
  function debounced(key: string, run: () => void) {
    const existing = debounceTimers.current[key];
    if (existing) clearTimeout(existing);
    debounceTimers.current[key] = setTimeout(run, 800);
  }

  // setNotify replaces the whole config. Saving on top of a failed load would
  // post every field blank, and the server would clear the stored config
  // including the SMTP password and Matrix token. So nothing saves until the
  // load has succeeded.
  const [loaded, setLoaded] = useState(false);
  const [loadErr, setLoadErr] = useState(false);
  useEffect(() => {
    getNotify()
      .then((r) => {
        if (r.ok && r.notify) {
          const merged = { ...emptyNotify, ...r.notify };
          setCfg(merged);
          lastGoodRef.current = merged;
          setLoaded(true);
          setLoadErr(false);
        } else {
          setLoadErr(true);
        }
        setSecretSet({ smtp: !!r.smtpPasswordSet, matrix: !!r.matrixTokenSet });
      })
      .catch(() => setLoadErr(true));
  }, []);

  // persistNotify merges patch onto the current config and posts the whole
  // object, since setNotify has no partial form. It reports success so the
  // caller can revert its own field on failure. On success the whole merged
  // object becomes lastGoodRef: a full replace persisted every field it
  // carried, including edits still waiting on their own debounce.
  async function persistNotify(patch: Partial<NotifyConfig>): Promise<boolean> {
    if (!loaded) {
      push(t("settings.notLoadedNoSave"), "fail");
      return false;
    }
    setState("saving");
    const merged = { ...cfg, ...patch };
    try {
      const r = await setNotify(merged);
      if (r.ok) {
        setState("idle");
        push(t("settings.saved"), "success");
        lastGoodRef.current = merged;
        return true;
      } else {
        setState("idle");
        push(r.error ?? t("settings.error"), "fail");
        return false;
      }
    } catch (err) {
      setState("idle");
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
      return false;
    }
  }

  // set is for typed fields: it updates the form at once and saves after a
  // pause. A failed save puts the field back to what the server holds and
  // shakes it.
  function set<K extends keyof NotifyConfig>(k: K, v: NotifyConfig[K]) {
    setCfg((c) => ({ ...c, [k]: v }));
    debounced(String(k), () => {
      void persistNotify({ [k]: v } as Partial<NotifyConfig>).then((ok) => {
        if (!ok) {
          setCfg((c) => ({ ...c, [k]: lastGoodRef.current[k] }));
          bumpShake(String(k));
        }
      });
    });
  }

  // setImmediate is set for discrete choices (toggles, selects), which save
  // without the pause.
  function setImmediate<K extends keyof NotifyConfig>(k: K, v: NotifyConfig[K]) {
    setCfg((c) => ({ ...c, [k]: v }));
    void persistNotify({ [k]: v } as Partial<NotifyConfig>).then((ok) => {
      if (!ok) {
        setCfg((c) => ({ ...c, [k]: lastGoodRef.current[k] }));
        bumpShake(String(k));
      }
    });
  }

  async function handleTest() {
    try {
      const r = await testNotify(cfg);
      if (r.ok) {
        push(t("notify.tested"), "success");
      } else {
        push(r.error ?? t("settings.error"), "fail");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("settings.error"), "fail");
    }
  }

  const inputCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 glim-field-focus-well";
  const selectCls =
    "rounded-control bg-carbon-surface3 text-carbon-text text-sm px-2.5 py-1.5 glim-field-focus-well";
  const labelCls = "flex flex-col gap-1 text-xs text-carbon-textSub";

  return (
    <>
    <Card title={t("notify.title")} hint={t("notify.hint")} hueIndex={hueIndex}>
      {/* The card refuses to save until the load succeeds, so a failed load
          has to be on screen. */}
      {loadErr && <span className="text-xs text-statusFail">{t("settings.notLoadedNoSave")}</span>}
      {/* A span, not a <label>: a label wrapping a multi-segment control
          hands its click and its name to the first segment. */}
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs text-carbon-textSub">{t("notify.on")}</span>
        <Selector
          key={fieldShake.on}
          items={[
            { id: "never", label: t("notify.onNever") },
            { id: "failure", label: t("notify.onFailure") },
            { id: "always", label: t("notify.onAlways") },
          ]}
          label={t("notify.on")}
          // Its own start, so it does not repeat the tab strip's colours
          // column for column one card above it.
          hueOffset={HUE_OFFSET.notifyOn}
          select="one"
          // Configs saved before this setting existed hold "" here, which
          // Config.shouldSend treats as "never". Show it as such instead of
          // lighting no segment at all.
          active={cfg.on || "never"}
          onChange={(id) => setImmediate("on", id)}
          variant="well"
          className={fieldShake.on ? "glim-shake" : undefined}
        />
      </div>

      <ToggleRow
        label={t("notify.scheduledSummary")}
        hint={t("notify.scheduledSummaryHint")}
        checked={cfg.scheduledSummary}
        onChange={(v) => setImmediate("scheduledSummary", v)}
        hueIndex={0}
        shakeNonce={fieldShake.scheduledSummary}
      />
      <ToggleRow
        label={t("notify.notifyOnUpdate")}
        hint={t("notify.notifyOnUpdateHint")}
        checked={cfg.notifyOnUpdate}
        onChange={(v) => setImmediate("notifyOnUpdate", v)}
        hueIndex={1}
        shakeNonce={fieldShake.notifyOnUpdate}
      />
      {/* Unraid native notifications, delivered over the host SSH connection. */}
      <ToggleRow
        label={t("notify.unraid")}
        hint={t("notify.unraidHint")}
        checked={cfg.unraid}
        onChange={(v) => setImmediate("unraid", v)}
        hueIndex={2}
        shakeNonce={fieldShake.unraid}
      />

      {/* The toggle is on but platform detection did not find Unraid, so the
          backend (unraidGate) sends nothing. Usually an Unraid host whose
          container lacks the template's /boot -> /host/boot mount. Without
          this banner only "Send test" would reveal it. */}
      {cfg.unraid && platformKind !== "unraid" && (
        <div className="rounded-card bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
          {tLtr(t, "notify.unraidPlatformMismatch").replace("{platform}", platformKind)}
        </div>
      )}

      {/* A plain hash link: the settings page switches tabs on hashchange,
          which a router navigation does not fire. */}
      <a href="#integrity" className="text-xs text-accentText hover:underline">
        {t("anomaly.settings.notifyCrossLink")}
      </a>

      {/* Test lives on this card rather than the channels card so it still
          works with Advanced off. */}
      <div className="flex items-center gap-3 pt-1 flex-wrap">
        <Button
          label={t("notify.test")}
          labelKey="notify.test"
          // Sending a test is what tells you a channel is wired up, so it
          // carries the card.
          tone="accent"
          onClick={() => void handleTest()}
        />
      </div>
    </Card>

    {advanced && (
      <Card title={t("notify.channelsTitle")} hint={t("notify.channelsHint")} hueIndex={channelsHueIndex}>
      {/* Each channel's toggle also gates the backend send, and its fields
          hide while it is off. The four channel toggles form one group for
          hueIndex, in the order they appear. */}
      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <ToggleRow
          label={t("notify.webhookChannel")}
          checked={cfg.webhookEnabled}
          onChange={(v) => setImmediate("webhookEnabled", v)}
          hueIndex={0}
          shakeNonce={fieldShake.webhookEnabled}
        />
        {cfg.webhookEnabled && (
          <>
            <label className={labelCls}>
              {t("notify.webhook")}
              <input key={fieldShake.webhookUrl} value={cfg.webhookUrl} onChange={(e) => set("webhookUrl", e.target.value)} spellCheck={false}
                placeholder="https://discord.com/api/webhooks/..." dir="ltr"
                className={`${inputCls} text-start${fieldShake.webhookUrl ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.webhookFormat")}
              <SelectField
                key={fieldShake.webhookFormat}
                value={cfg.webhookFormat}
                onChange={(v) => setImmediate("webhookFormat", v)}
                label={t("notify.webhookFormat")}
                options={[
                  { value: "generic", label: "Generic JSON" },
                  { value: "discord", label: "Discord" },
                  { value: "slack", label: "Slack" },
                  { value: "gotify", label: "Gotify" },
                  { value: "ntfy", label: "ntfy" },
                ]}
                className={`${selectCls}${fieldShake.webhookFormat ? " glim-shake" : ""}`}
              />
            </label>
          </>
        )}
      </div>

      {/* Apprise posts to a user-run apprise-api server, which reaches
          Apprise's 100+ services without bundling Python. */}
      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <ToggleRow
          label={t("notify.apprise")}
          hint={tLtr(t, "notify.appriseHint")}
          checked={cfg.appriseEnabled}
          onChange={(v) => setImmediate("appriseEnabled", v)}
          hueIndex={1}
          shakeNonce={fieldShake.appriseEnabled}
        />
        {cfg.appriseEnabled && (
          <>
            <label className={labelCls}>
              {t("notify.appriseUrl")}
              <input key={fieldShake.appriseUrl} value={cfg.appriseUrl} onChange={(e) => set("appriseUrl", e.target.value)} spellCheck={false}
                placeholder="http://apprise:8000/notify/bombvault" dir="ltr"
                className={`${inputCls} text-start${fieldShake.appriseUrl ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.appriseTags")}
              <input key={fieldShake.appriseTags} value={cfg.appriseTags} onChange={(e) => set("appriseTags", e.target.value)} spellCheck={false}
                placeholder="backups,homelab" className={`${inputCls}${fieldShake.appriseTags ? " glim-shake" : ""}`} />
            </label>
          </>
        )}
      </div>

      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <ToggleRow
          label={t("notify.matrix")}
          checked={cfg.matrixEnabled}
          onChange={(v) => setImmediate("matrixEnabled", v)}
          hueIndex={2}
          shakeNonce={fieldShake.matrixEnabled}
        />
        {cfg.matrixEnabled && (
          <>
            <label className={labelCls}>
              {t("notify.matrixHomeserver")}
              <input key={fieldShake.matrixHomeserver} value={cfg.matrixHomeserver} onChange={(e) => set("matrixHomeserver", e.target.value)} spellCheck={false}
                placeholder="https://matrix.org" dir="ltr"
                className={`${inputCls} text-start${fieldShake.matrixHomeserver ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.matrixToken")}
              <RevealInput {...revealMatrixToken} key={fieldShake.matrixToken} value={cfg.matrixToken} onChange={(e) => set("matrixToken", e.target.value)} spellCheck={false}
                placeholder={secretSet.matrix ? t("cloud.secretSet") : ""} wrapperClassName="w-full"
                className={`${inputCls}${fieldShake.matrixToken ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.matrixRoom")}
              <input key={fieldShake.matrixRoom} value={cfg.matrixRoom} onChange={(e) => set("matrixRoom", e.target.value)} spellCheck={false}
                placeholder="!abcdef:matrix.org" dir="ltr"
                className={`${inputCls} text-start${fieldShake.matrixRoom ? " glim-shake" : ""}`} />
            </label>
          </>
        )}
      </div>

      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <ToggleRow
          label={t("notify.smtp")}
          checked={cfg.smtpEnabled}
          onChange={(v) => setImmediate("smtpEnabled", v)}
          hueIndex={3}
          shakeNonce={fieldShake.smtpEnabled}
        />
        {cfg.smtpEnabled && (
          <>
            <label className={labelCls}>
              {t("notify.smtpHost")}
              <input key={fieldShake.smtpHost} value={cfg.smtpHost} onChange={(e) => set("smtpHost", e.target.value)} spellCheck={false}
                placeholder="smtp.example.com" dir="ltr" className={`${inputCls} text-start${fieldShake.smtpHost ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpPort")}
              {/* The steppers read min/max to know when to grey out. */}
              <NumberField key={fieldShake.smtpPort} value={cfg.smtpPort} onChange={(e) => set("smtpPort", Number(e.target.value) || 0)} spellCheck={false}
                min={1} max={65535} placeholder="587" className={`${inputCls}${fieldShake.smtpPort ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpTls")}
              <SelectField
                key={fieldShake.smtpTls}
                value={cfg.smtpTls}
                onChange={(v) => setImmediate("smtpTls", v)}
                label={t("notify.smtpTls")}
                options={[
                  { value: "starttls", label: "STARTTLS" },
                  { value: "tls", label: "TLS (implicit)" },
                  { value: "none", label: "None" },
                ]}
                className={`${selectCls}${fieldShake.smtpTls ? " glim-shake" : ""}`}
              />
            </label>
            <label className={labelCls}>
              {t("notify.smtpUser")}
              <input key={fieldShake.smtpUsername} value={cfg.smtpUsername} onChange={(e) => set("smtpUsername", e.target.value)} spellCheck={false}
                dir="ltr" className={`${inputCls} text-start${fieldShake.smtpUsername ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpPass")}
              <RevealInput {...revealSmtpPassword} key={fieldShake.smtpPassword} value={cfg.smtpPassword} onChange={(e) => set("smtpPassword", e.target.value)} spellCheck={false}
                placeholder={secretSet.smtp ? t("cloud.secretSet") : ""} wrapperClassName="w-full"
                className={`${inputCls}${fieldShake.smtpPassword ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpFrom")}
              <input key={fieldShake.smtpFrom} value={cfg.smtpFrom} onChange={(e) => set("smtpFrom", e.target.value)} spellCheck={false}
                placeholder="bombvault@example.com" dir="ltr" className={`${inputCls} text-start${fieldShake.smtpFrom ? " glim-shake" : ""}`} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpTo")}
              <input key={fieldShake.smtpTo} value={cfg.smtpTo} onChange={(e) => set("smtpTo", e.target.value)} spellCheck={false}
                placeholder="admin@example.com" dir="ltr" className={`${inputCls} text-start${fieldShake.smtpTo ? " glim-shake" : ""}`} />
            </label>
          </>
        )}
      </div>
      </Card>
    )}

    {advanced && (
      <Card title={t("notify.healthchecksTitle")} hueIndex={healthchecksHueIndex}>
      {/* The InfoBubble is a button, so clicking it inside the label opens
          the bubble rather than focusing the input. */}
      <label className={labelCls}>
        <span className="flex items-center gap-1">
          {t("notify.healthchecks")}
          <InfoBubble tip={t("notify.healthchecksLifecycle")} />
        </span>
        <input key={fieldShake.healthchecksUrl} value={cfg.healthchecksUrl} onChange={(e) => set("healthchecksUrl", e.target.value)} spellCheck={false}
          placeholder="https://hc-ping.com/your-uuid" className={`${inputCls}${fieldShake.healthchecksUrl ? " glim-shake" : ""}`} />
      </label>

      {/* Per-domain ping URLs; a blank one falls back to the global URL.
          These edit one entry of a nested map rather than a top-level key,
          so they bypass set() and a failure reverts only that domain's
          entry. */}
      <div className="flex flex-col gap-2 rounded-card bg-carbon-surface2 p-3">
        <span className="flex items-center gap-1 text-xs font-medium text-carbon-textSub">
          {t("notify.hcPerDomain")}
          <InfoBubble tip={t("notify.hcPerDomainHint")} />
        </span>
        {(
          [
            ["container", t("nav.containers")],
            ["VM", t("nav.vms")],
            ["flash", t("nav.flash")],
            ["config", t("nav.config")],
            ["files", t("nav.files")],
          ] as const
        ).map(([key, label]) => (
          <label key={key} className={labelCls}>
            {label}
            <input
              key={fieldShake[`hc-${key}`]}
              value={cfg.healthchecksByDomain?.[key] ?? ""}
              onChange={(e) => {
                const v = e.target.value;
                const nextMap = { ...cfg.healthchecksByDomain, [key]: v };
                setCfg((c) => ({ ...c, healthchecksByDomain: nextMap }));
                debounced(`hc-${key}`, () => {
                  void persistNotify({ healthchecksByDomain: nextMap }).then((ok) => {
                    if (!ok) {
                      setCfg((c) => ({
                        ...c,
                        healthchecksByDomain: {
                          ...c.healthchecksByDomain,
                          [key]: lastGoodRef.current.healthchecksByDomain?.[key] ?? "",
                        },
                      }));
                      bumpShake(`hc-${key}`);
                    }
                  });
                });
              }}
              spellCheck={false}
              placeholder="https://hc-ping.com/your-uuid"
              dir="ltr"
              className={`${inputCls} text-start${fieldShake[`hc-${key}`] ? " glim-shake" : ""}`}
            />
          </label>
        ))}
      </div>
      </Card>
    )}
    </>
  );
}
