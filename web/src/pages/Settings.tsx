import { useEffect, useState } from "react";
import { getSettings, putSettings, getAuth, setAuthPassword, logout, getVMSSH, testVMSSH, getRclone, setRclone, getCloud, setCloud, checkDomain, unlockDomain, pruneDomain, replicateOffsite, testOffsite, getNotify, setNotify, testNotify, runDrill, getDrills, listContainers, recoveryKitUrl, getHealth } from "../lib/api";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { FolderBrowser } from "../components/FolderBrowser";
import { OffsiteWizard } from "../components/OffsiteWizard";
import { CadenceBuilder } from "../components/CadenceBuilder";
import type { Settings, NotifyConfig, RestoreDrill, Container } from "../lib/api";
import { useT } from "../lib/i18n";
import { useAdvanced, Advanced } from "../lib/advanced";
import { SpikePanel } from "../components/SpikePanel";
import { getAccent, setAccent, DEFAULT_ACCENT } from "../lib/accent";
import { relativeTime } from "../lib/reltime";

// AboutFooter shows the running version (linking to the releases page) and a
// "Report a bug" link at the very bottom of Settings, so the sidebar stays clean.
function AboutFooter() {
  const { t } = useT();
  const [version, setVersion] = useState<string | null>(null);
  useEffect(() => {
    let active = true;
    getHealth()
      .then((h) => { if (active) setVersion(h.version ?? null); })
      .catch(() => { /* version is best-effort; ignore */ });
    return () => { active = false; };
  }, []);
  return (
    <div className="pt-6 pb-4 flex flex-col items-center gap-1 text-xs text-carbon-textMuted">
      {version && (
        <a
          href="https://github.com/junkerderprovinz/bombvault/releases"
          target="_blank"
          rel="noopener noreferrer"
          className="hover:text-carbon-text transition-colors"
          title={`BombVault ${version}`}
        >
          BombVault {version}
        </a>
      )}
      <a
        href="https://github.com/junkerderprovinz/bombvault/issues"
        target="_blank"
        rel="noopener noreferrer"
        className="hover:text-carbon-text transition-colors"
      >
        {t("nav.reportBug")}
      </a>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Card wrapper
// ---------------------------------------------------------------------------

function Card({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="bg-carbon-surface rounded-card border border-carbon-border p-5 flex flex-col gap-4">
      <h2 className="text-sm font-semibold text-carbon-textSub uppercase tracking-widest">
        {title}
      </h2>
      {children}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Toggle row
// ---------------------------------------------------------------------------

export function ToggleRow({
  label,
  description,
  checked,
  onChange,
  disabled,
}: {
  label: string;
  description?: string;
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div className="flex flex-col gap-0.5">
        <span className="text-sm text-carbon-text">{label}</span>
        {description && (
          <span className="text-xs text-carbon-textMuted">{description}</span>
        )}
      </div>
      <button
        role="switch"
        aria-checked={checked}
        disabled={disabled}
        onClick={() => onChange(!checked)}
        className={`relative inline-flex h-5 w-9 shrink-0 mt-0.5 items-center rounded-full transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#78a9ff] disabled:opacity-50 ${
          checked ? "bg-accent" : "bg-carbon-surface3"
        }`}
      >
        <span
          className={`inline-block h-3.5 w-3.5 rounded-full bg-carbon-background transition-transform ${
            checked ? "translate-x-[18px]" : "translate-x-[3px]"
          }`}
        />
      </button>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Save bar shared component
// ---------------------------------------------------------------------------

type SaveState = "idle" | "saving" | "saved" | "error";

function SaveBar({
  state,
  error,
  onSave,
  t,
  disabled = false,
}: {
  state: SaveState;
  error: string | null;
  onSave: () => void;
  t: ReturnType<typeof useT>["t"];
  disabled?: boolean;
}) {
  return (
    <div className="flex items-center gap-3 pt-1">
      <button
        onClick={onSave}
        disabled={disabled || state === "saving"}
        className="inline-flex items-center gap-2 rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
      >
        {state === "saving" ? (
          <>
            <span
              className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin"
              style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
            />
            {t("common.saving")}
          </>
        ) : (
          t("settings.save")
        )}
      </button>
      {state === "saved" && (
        <span className="text-sm text-[#6fdc8c]">{t("settings.saved")}</span>
      )}
      {state === "error" && error && (
        <span className="text-sm text-[#ff8389]">{error}</span>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Accent preset swatches
// ---------------------------------------------------------------------------

const ACCENT_PRESETS = [
  { hex: "#FCC419", label: "Sunflower" },
  { hex: "#1D99F3", label: "Blue" },
  { hex: "#6FDC8C", label: "Green" },
  { hex: "#FF8389", label: "Red" },
  { hex: "#BE95FF", label: "Purple" },
] as const;

// ---------------------------------------------------------------------------
// Settings page
// ---------------------------------------------------------------------------

// VMSSHCard shows BombVault's SSH public key (to authorize on the Unraid host)
// and a connection test. Self-contained: fetches its own data so the large
// SettingsPage doesn't need extra state.
function VMSSHCard({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const [host, setHost] = useState("");
  const [pub, setPub] = useState("");
  const [copied, setCopied] = useState(false);
  const [cmdCopied, setCmdCopied] = useState(false);
  const [testState, setTestState] = useState<"idle" | "testing" | "ok" | "fail">("idle");
  const [testMsg, setTestMsg] = useState<string | null>(null);

  // Ready-to-paste command that authorizes this key on the Unraid host, both for
  // the live session and persistently (Unraid restores root.pubkeys on boot).
  const authorizeCmd = pub
    ? `mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
echo '${pub}' | tee -a /root/.ssh/authorized_keys /boot/config/ssh/root.pubkeys >/dev/null
chmod 600 /root/.ssh/authorized_keys`
    : "";

  useEffect(() => {
    getVMSSH()
      .then((r) => {
        if (r.ok) {
          setHost(r.host ?? "");
          setPub(r.publicKey ?? "");
        }
      })
      .catch(() => undefined);
  }, []);

  async function handleTest() {
    setTestState("testing");
    setTestMsg(null);
    try {
      const r = await testVMSSH();
      if (r.ok) {
        setTestState("ok");
      } else {
        setTestState("fail");
        setTestMsg(r.error ?? t("vm.ssh.testFail"));
      }
    } catch {
      setTestState("fail");
      setTestMsg(t("vm.ssh.testFail"));
    }
  }

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(pub);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      /* clipboard unavailable (non-HTTPS) — the key is selectable in the box */
    }
  }

  async function handleCopyCmd() {
    try {
      await navigator.clipboard.writeText(authorizeCmd);
      setCmdCopied(true);
      setTimeout(() => setCmdCopied(false), 2000);
    } catch {
      /* clipboard unavailable — the command is selectable in the box */
    }
  }

  return (
    <Card title={t("vm.ssh.title")}>
      <div className="flex flex-col gap-3">
        <p className="text-sm text-carbon-textSub">{t("vm.ssh.desc")}</p>
        <div className="text-sm text-carbon-text">
          {t("vm.ssh.host")}: <span className="font-mono">{host || "—"}</span>
        </div>
        <div className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textMuted">{t("vm.ssh.publicKey")}</span>
          <div className="flex items-start gap-2">
            <code className="flex-1 break-all rounded border border-carbon-border bg-carbon-surface2 p-2 text-xs text-carbon-text">
              {pub || "—"}
            </code>
            <button
              onClick={handleCopy}
              disabled={!pub}
              className="shrink-0 rounded bg-accent px-3 py-2 text-xs font-medium text-accentContrast disabled:opacity-50"
            >
              {copied ? t("vm.ssh.copied") : t("vm.ssh.copy")}
            </button>
          </div>
        </div>

        {/* One-time setup instructions */}
        <div className="rounded-lg bg-carbon-surface2 border border-carbon-border p-3 flex flex-col gap-2">
          <span className="text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
            {t("vm.ssh.setupTitle")}
          </span>
          <ol className="list-decimal pl-5 text-xs text-carbon-textSub flex flex-col gap-1">
            <li>{t("vm.ssh.step1")}</li>
            <li>{t("vm.ssh.step2")}</li>
            <li>{t("vm.ssh.step3")}</li>
          </ol>
          <div className="flex items-start gap-2">
            <pre className="flex-1 overflow-x-auto rounded border border-carbon-border bg-carbon-background p-2 text-[11px] leading-snug text-carbon-text whitespace-pre">{authorizeCmd || "—"}</pre>
            <button
              onClick={handleCopyCmd}
              disabled={!pub}
              className="shrink-0 rounded bg-carbon-surface3 px-3 py-2 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
            >
              {cmdCopied ? t("vm.ssh.copied") : t("vm.ssh.copyCmd")}
            </button>
          </div>
          <a
            href="https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md"
            target="_blank"
            rel="noreferrer"
            className="text-xs text-[#78a9ff] hover:underline"
          >
            {t("vm.ssh.guide")} →
          </a>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={handleTest}
            disabled={testState === "testing"}
            className="rounded border border-carbon-border bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
          >
            {testState === "testing" ? t("vm.ssh.testing") : t("vm.ssh.test")}
          </button>
          {testState === "ok" && (
            <span className="text-sm text-green-500">{t("vm.ssh.testOk")}</span>
          )}
          {testState === "fail" && (
            <span className="text-sm text-red-400">{testMsg ?? t("vm.ssh.testFail")}</span>
          )}
        </div>
      </div>
    </Card>
  );
}

// RcloneCard manages the off-site rclone config (paste rclone.conf). It is
// stored encrypted; only the remote NAMES are read back for display. Backup
// paths can then be set to "rclone:<remote>:<bucket>" in Backup Paths.
export function RcloneCard({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const [remotes, setRemotes] = useState<string[]>([]);
  const [conf, setConf] = useState("");
  const [state, setState] = useState<SaveState>("idle");
  const [msg, setMsg] = useState<string | null>(null);

  function refresh() {
    getRclone()
      .then((r) => {
        if (r.ok) setRemotes(r.remotes ?? []);
      })
      .catch(() => undefined);
  }
  useEffect(() => {
    refresh();
  }, []);

  async function handleSave() {
    setState("saving");
    setMsg(null);
    try {
      const r = await setRclone(conf);
      if (r.ok) {
        setState("saved");
        setConf("");
        refresh();
        setTimeout(() => setState("idle"), 3000);
      } else {
        setState("error");
        setMsg(r.error ?? t("settings.error"));
      }
    } catch (err) {
      setState("error");
      setMsg(err instanceof Error ? err.message : t("settings.error"));
    }
  }

  return (
    <Card title={t("rclone.title")}>
      <p className="text-xs text-carbon-textMuted -mt-1">{t("rclone.hint")}</p>
      <div className="text-sm text-carbon-text">
        {t("rclone.configured")}:{" "}
        <span className="font-mono">{remotes.length > 0 ? remotes.join(", ") : "—"}</span>
      </div>
      <textarea
        value={conf}
        onChange={(e) => setConf(e.target.value)}
        spellCheck={false}
        rows={6}
        placeholder={"[b2]\ntype = b2\naccount = ...\nkey = ..."}
        className="rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-xs font-mono px-3 py-2 focus:outline-none focus:border-[#78a9ff]"
      />
      <p className="text-xs text-carbon-textMuted">{t("rclone.pathHint")}</p>
      <div className="flex items-center gap-3 pt-1">
        <button
          onClick={() => void handleSave()}
          disabled={state === "saving" || conf.trim() === ""}
          className="inline-flex items-center gap-2 rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
        >
          {state === "saving" ? t("auth.saving") : t("rclone.save")}
        </button>
        {state === "saved" && <span className="text-sm text-[#6fdc8c]">{t("settings.saved")}</span>}
        {state === "error" && msg && <span className="text-sm text-[#ff8389]">{msg}</span>}
      </div>
    </Card>
  );
}

// CloudCard stores credentials for off-site restic backends (S3 + restic REST),
// kept encrypted. Secrets are write-only: blank on load, blank-on-save keeps the
// stored value. Field labels are restic's actual env var names (self-documenting).
export function CloudCard({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const [c, setC] = useState({ s3KeyId: "", s3Secret: "", s3Region: "", restUser: "", restPassword: "" });
  const [secretSet, setSecretSet] = useState(false);
  const [pwSet, setPwSet] = useState(false);
  const [state, setState] = useState<SaveState>("idle");
  const [msg, setMsg] = useState<string | null>(null);

  function refresh() {
    getCloud()
      .then((r) => {
        if (r.ok) {
          setC((p) => ({ ...p, s3KeyId: r.s3KeyId ?? "", s3Region: r.s3Region ?? "", restUser: r.restUser ?? "" }));
          setSecretSet(!!r.s3SecretSet);
          setPwSet(!!r.restPasswordSet);
        }
      })
      .catch(() => undefined);
  }
  useEffect(refresh, []);

  function set<K extends keyof typeof c>(k: K, v: string) {
    setC((p) => ({ ...p, [k]: v }));
  }

  async function handleSave() {
    setState("saving");
    setMsg(null);
    try {
      const r = await setCloud(c);
      if (r.ok) {
        setState("saved");
        setC((p) => ({ ...p, s3Secret: "", restPassword: "" }));
        refresh();
        setTimeout(() => setState("idle"), 3000);
      } else {
        setState("error");
        setMsg(r.error ?? t("settings.error"));
      }
    } catch (err) {
      setState("error");
      setMsg(err instanceof Error ? err.message : t("settings.error"));
    }
  }

  const inputCls =
    "rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm font-mono px-3 py-1.5 focus:outline-none focus:border-[#78a9ff]";
  const fieldCls = "flex flex-col gap-1 text-xs font-mono text-carbon-textSub";

  return (
    <Card title={t("cloud.title")}>
      <p className="text-xs text-carbon-textMuted -mt-1">{t("cloud.hint")}</p>

      <div className="flex flex-col gap-2 rounded-lg bg-carbon-surface2 border border-carbon-border p-3">
        <span className="text-xs font-semibold text-carbon-textSub">Amazon S3</span>
        <label className={fieldCls}>AWS_ACCESS_KEY_ID
          <input value={c.s3KeyId} onChange={(e) => set("s3KeyId", e.target.value)} spellCheck={false} className={inputCls} /></label>
        <label className={fieldCls}>AWS_SECRET_ACCESS_KEY
          <input type="password" value={c.s3Secret} onChange={(e) => set("s3Secret", e.target.value)} spellCheck={false}
            placeholder={secretSet ? t("cloud.secretSet") : ""} className={inputCls} /></label>
        <label className={fieldCls}>AWS_DEFAULT_REGION
          <input value={c.s3Region} onChange={(e) => set("s3Region", e.target.value)} spellCheck={false} placeholder="us-east-1" className={inputCls} /></label>
      </div>

      <div className="flex flex-col gap-2 rounded-lg bg-carbon-surface2 border border-carbon-border p-3">
        <span className="text-xs font-semibold text-carbon-textSub">restic REST server</span>
        <label className={fieldCls}>RESTIC_REST_USERNAME
          <input value={c.restUser} onChange={(e) => set("restUser", e.target.value)} spellCheck={false} className={inputCls} /></label>
        <label className={fieldCls}>RESTIC_REST_PASSWORD
          <input type="password" value={c.restPassword} onChange={(e) => set("restPassword", e.target.value)} spellCheck={false}
            placeholder={pwSet ? t("cloud.secretSet") : ""} className={inputCls} /></label>
      </div>

      <div className="flex items-center gap-3 pt-1">
        <button
          onClick={() => void handleSave()}
          disabled={state === "saving"}
          className="inline-flex items-center gap-2 rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
        >
          {state === "saving" ? t("auth.saving") : t("settings.save")}
        </button>
        {state === "saved" && <span className="text-sm text-[#6fdc8c]">{t("settings.saved")}</span>}
        {state === "error" && msg && <span className="text-sm text-[#ff8389]">{msg}</span>}
      </div>
    </Card>
  );
}

// emptyNotify is the default notification config shown before the saved one loads.
const emptyNotify: NotifyConfig = {
  on: "never",
  webhookUrl: "",
  webhookFormat: "generic",
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
};

// NotifyCard configures backup notifications (webhook / Matrix / Healthchecks).
// Stored encrypted at rest; the form pre-fills from the saved config and Test
// sends to the CURRENT form values (no save needed).
function NotifyCard({ t }: { t: ReturnType<typeof useT>["t"] }) {
  // Simple mode still gets notify-on-failure via Unraid; the extra channels
  // (webhook/Matrix/Healthchecks/SMTP) are power-user features, so gate those.
  const { advanced } = useAdvanced();
  const [cfg, setCfg] = useState<NotifyConfig>(emptyNotify);
  const [state, setState] = useState<SaveState>("idle");
  const [msg, setMsg] = useState<string | null>(null);
  const [tested, setTested] = useState(false);
  // The SMTP password / Matrix token are never sent to the browser; track whether
  // one is stored so the field shows "configured" and a blank submit keeps it.
  const [secretSet, setSecretSet] = useState({ smtp: false, matrix: false });

  useEffect(() => {
    getNotify()
      .then((r) => {
        if (r.ok && r.notify) setCfg({ ...emptyNotify, ...r.notify });
        setSecretSet({ smtp: !!r.smtpPasswordSet, matrix: !!r.matrixTokenSet });
      })
      .catch(() => undefined);
  }, []);

  function set<K extends keyof NotifyConfig>(k: K, v: NotifyConfig[K]) {
    setCfg((c) => ({ ...c, [k]: v }));
  }

  async function handleSave() {
    setState("saving");
    setMsg(null);
    try {
      const r = await setNotify(cfg);
      if (r.ok) {
        setState("saved");
        setTimeout(() => setState("idle"), 3000);
      } else {
        setState("error");
        setMsg(r.error ?? t("settings.error"));
      }
    } catch (err) {
      setState("error");
      setMsg(err instanceof Error ? err.message : t("settings.error"));
    }
  }

  async function handleTest() {
    setTested(false);
    setMsg(null);
    try {
      const r = await testNotify(cfg);
      if (r.ok) {
        setTested(true);
        setTimeout(() => setTested(false), 3000);
      } else {
        setState("error");
        setMsg(r.error ?? t("settings.error"));
      }
    } catch (err) {
      setState("error");
      setMsg(err instanceof Error ? err.message : t("settings.error"));
    }
  }

  const inputCls =
    "rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm font-mono px-3 py-1.5 focus:outline-none focus:border-[#78a9ff]";
  const selectCls =
    "rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm px-2.5 py-1.5 focus:outline-none focus:border-[#78a9ff]";
  const labelCls = "flex flex-col gap-1 text-xs text-carbon-textSub";

  return (
    <Card title={t("notify.title")}>
      <p className="text-xs text-carbon-textMuted -mt-1">{t("notify.hint")}</p>

      <label className={labelCls}>
        {t("notify.on")}
        <select value={cfg.on} onChange={(e) => set("on", e.target.value)} className={selectCls}>
          <option value="never">{t("notify.onNever")}</option>
          <option value="failure">{t("notify.onFailure")}</option>
          <option value="always">{t("notify.onAlways")}</option>
        </select>
      </label>

      {/* Unraid native notifications (delivered over the host SSH connection). */}
      <label className="flex items-start gap-2 rounded-lg bg-carbon-surface2 border border-carbon-border p-3 cursor-pointer">
        <input
          type="checkbox"
          checked={cfg.unraid}
          onChange={(e) => set("unraid", e.target.checked)}
          className="mt-0.5"
          style={{ accentColor: "var(--accent)" }}
        />
        <span className="flex flex-col gap-0.5">
          <span className="text-sm text-carbon-text">{t("notify.unraid")}</span>
          <span className="text-xs text-carbon-textMuted">{t("notify.unraidHint")}</span>
        </span>
      </label>

      {advanced && (
        <>
      <div className="flex flex-col gap-2 rounded-lg bg-carbon-surface2 border border-carbon-border p-3">
        <label className={labelCls}>
          {t("notify.webhook")}
          <input value={cfg.webhookUrl} onChange={(e) => set("webhookUrl", e.target.value)} spellCheck={false}
            placeholder="https://discord.com/api/webhooks/..." className={inputCls} />
        </label>
        <label className={labelCls}>
          {t("notify.webhookFormat")}
          <select value={cfg.webhookFormat} onChange={(e) => set("webhookFormat", e.target.value)} className={selectCls}>
            <option value="generic">Generic JSON</option>
            <option value="discord">Discord</option>
            <option value="slack">Slack</option>
            <option value="gotify">Gotify</option>
            <option value="ntfy">ntfy</option>
          </select>
        </label>
      </div>

      <div className="flex flex-col gap-2 rounded-lg bg-carbon-surface2 border border-carbon-border p-3">
        <span className="text-xs font-medium text-carbon-textSub">{t("notify.matrix")}</span>
        <label className={labelCls}>
          {t("notify.matrixHomeserver")}
          <input value={cfg.matrixHomeserver} onChange={(e) => set("matrixHomeserver", e.target.value)} spellCheck={false}
            placeholder="https://matrix.org" className={inputCls} />
        </label>
        <label className={labelCls}>
          {t("notify.matrixToken")}
          <input value={cfg.matrixToken} onChange={(e) => set("matrixToken", e.target.value)} spellCheck={false}
            type="password" placeholder={secretSet.matrix ? t("cloud.secretSet") : ""} className={inputCls} />
        </label>
        <label className={labelCls}>
          {t("notify.matrixRoom")}
          <input value={cfg.matrixRoom} onChange={(e) => set("matrixRoom", e.target.value)} spellCheck={false}
            placeholder="!abcdef:matrix.org" className={inputCls} />
        </label>
      </div>

      <label className={labelCls}>
        {t("notify.healthchecks")}
        <input value={cfg.healthchecksUrl} onChange={(e) => set("healthchecksUrl", e.target.value)} spellCheck={false}
          placeholder="https://hc-ping.com/your-uuid" className={inputCls} />
      </label>
      <p className="text-xs text-carbon-textMuted -mt-1">{t("notify.healthchecksLifecycle")}</p>

      {/* Per-domain Healthchecks overrides (advanced). A blank field falls back to the global URL above. */}
      <div className="flex flex-col gap-2 rounded-lg bg-carbon-surface2 border border-carbon-border p-3">
        <span className="text-xs font-medium text-carbon-textSub">{t("notify.hcPerDomain")}</span>
        {(
          [
            ["container", t("nav.containers")],
            ["VM", t("nav.vms")],
            ["flash", t("nav.flash")],
            ["config", t("nav.config")],
          ] as const
        ).map(([key, label]) => (
          <label key={key} className={labelCls}>
            {label}
            <input
              value={cfg.healthchecksByDomain?.[key] ?? ""}
              onChange={(e) =>
                setCfg((c) => ({
                  ...c,
                  healthchecksByDomain: { ...c.healthchecksByDomain, [key]: e.target.value },
                }))
              }
              spellCheck={false}
              placeholder="https://hc-ping.com/your-uuid"
              className={inputCls}
            />
          </label>
        ))}
        <p className="text-xs text-carbon-textMuted">{t("notify.hcPerDomainHint")}</p>
      </div>

      {/* Email (SMTP), sent via the configured mail server. */}
      <div className="flex flex-col gap-2 rounded-lg bg-carbon-surface2 border border-carbon-border p-3">
        <label className="flex items-start gap-2 cursor-pointer">
          <input
            type="checkbox"
            checked={cfg.smtpEnabled}
            onChange={(e) => set("smtpEnabled", e.target.checked)}
            className="mt-0.5"
            style={{ accentColor: "var(--accent)" }}
          />
          <span className="text-sm text-carbon-text">{t("notify.smtp")}</span>
        </label>
        {cfg.smtpEnabled && (
          <>
            <label className={labelCls}>
              {t("notify.smtpHost")}
              <input value={cfg.smtpHost} onChange={(e) => set("smtpHost", e.target.value)} spellCheck={false}
                placeholder="smtp.example.com" className={inputCls} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpPort")}
              <input value={cfg.smtpPort} onChange={(e) => set("smtpPort", Number(e.target.value) || 0)} spellCheck={false}
                type="number" placeholder="587" className={inputCls} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpTls")}
              <select value={cfg.smtpTls} onChange={(e) => set("smtpTls", e.target.value)} className={selectCls}>
                <option value="starttls">STARTTLS</option>
                <option value="tls">TLS (implicit)</option>
                <option value="none">None</option>
              </select>
            </label>
            <label className={labelCls}>
              {t("notify.smtpUser")}
              <input value={cfg.smtpUsername} onChange={(e) => set("smtpUsername", e.target.value)} spellCheck={false}
                className={inputCls} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpPass")}
              <input value={cfg.smtpPassword} onChange={(e) => set("smtpPassword", e.target.value)} spellCheck={false}
                type="password" placeholder={secretSet.smtp ? t("cloud.secretSet") : ""} className={inputCls} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpFrom")}
              <input value={cfg.smtpFrom} onChange={(e) => set("smtpFrom", e.target.value)} spellCheck={false}
                placeholder="bombvault@example.com" className={inputCls} />
            </label>
            <label className={labelCls}>
              {t("notify.smtpTo")}
              <input value={cfg.smtpTo} onChange={(e) => set("smtpTo", e.target.value)} spellCheck={false}
                placeholder="admin@example.com" className={inputCls} />
            </label>
          </>
        )}
      </div>
        </>
      )}

      <div className="flex items-center gap-3 pt-1 flex-wrap">
        <button onClick={() => void handleSave()} disabled={state === "saving"}
          className="inline-flex items-center gap-2 rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50">
          {state === "saving" ? t("auth.saving") : t("notify.save")}
        </button>
        <button onClick={() => void handleTest()}
          className="rounded-lg border border-carbon-border bg-carbon-surface2 px-4 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover transition-colors">
          {t("notify.test")}
        </button>
        {state === "saved" && <span className="text-sm text-[#6fdc8c]">{t("settings.saved")}</span>}
        {tested && <span className="text-sm text-[#6fdc8c]">{t("notify.tested")}</span>}
        {state === "error" && msg && <span className="text-sm text-[#ff8389] break-words">{msg}</span>}
      </div>
    </Card>
  );
}

// ReplicateNowButton triggers an on-demand off-site replication for one domain
// (restic copy local→off-site), surfacing the result inline.
function ReplicateNowButton({
  domain,
  t,
}: {
  domain: "containers" | "vms" | "flash";
  t: ReturnType<typeof useT>["t"];
}) {
  const [st, setSt] = useState<"idle" | "busy" | "ok" | "fail">("idle");
  const [err, setErr] = useState<string | null>(null);
  async function go() {
    setSt("busy");
    setErr(null);
    try {
      const r = await replicateOffsite(domain);
      if (r.ok) {
        setSt("ok");
        setTimeout(() => setSt("idle"), 4000);
      } else {
        setSt("fail");
        setErr(r.error ?? t("offsite.replicateFailed"));
      }
    } catch (e) {
      setSt("fail");
      setErr(e instanceof Error ? e.message : t("offsite.replicateFailed"));
    }
  }
  return (
    <span className="inline-flex items-center gap-1.5">
      <button
        type="button"
        onClick={() => void go()}
        disabled={st === "busy"}
        className="rounded-lg border border-carbon-border bg-carbon-surface2 px-2.5 py-1 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
      >
        {st === "busy" ? t("offsite.replicating") : t("offsite.replicateNow")}
      </button>
      {st === "ok" && <span className="text-xs text-[#6fdc8c]">{t("integrity.ok")}</span>}
      {st === "fail" && <span className="text-xs text-[#ff8389] break-words">{err}</span>}
    </span>
  );
}

// TestConnectionButton probes a domain's off-site repo (reachable / initialised)
// without modifying it, showing the verdict inline — so the user can verify the
// configured location before relying on it.
function TestConnectionButton({
  domain,
  t,
}: {
  domain: "containers" | "vms" | "flash";
  t: ReturnType<typeof useT>["t"];
}) {
  const [st, setSt] = useState<"idle" | "busy" | "ok" | "uninit" | "fail">("idle");
  const [err, setErr] = useState<string | null>(null);
  async function go() {
    setSt("busy");
    setErr(null);
    try {
      const r = await testOffsite(domain);
      if (r.ok && r.reachable && r.initialized) {
        setSt("ok");
      } else if (r.ok && r.reachable) {
        setSt("uninit");
      } else {
        setSt("fail");
        setErr(r.error ?? null);
      }
    } catch (e) {
      setSt("fail");
      setErr(e instanceof Error ? e.message : null);
    }
  }
  return (
    <span className="inline-flex items-center gap-1.5">
      <button
        type="button"
        onClick={() => void go()}
        disabled={st === "busy"}
        className="rounded-lg border border-carbon-border bg-carbon-surface2 px-2.5 py-1 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
      >
        {t("offsite.test")}
      </button>
      {st === "ok" && <span className="text-xs text-[#6fdc8c]">{t("offsite.testOk")}</span>}
      {st === "uninit" && <span className="text-xs text-[#f1c21b]">{t("offsite.testUninitialized")}</span>}
      {st === "fail" && (
        <span className="text-xs text-[#ff8389] break-words">{err ?? t("offsite.testFailed")}</span>
      )}
    </span>
  );
}

// IntegrityCard runs per-domain repository maintenance: verify (restic check),
// unlock (clear stale locks), prune (reclaim space), and a restore-verification
// "drill". The drill has two kinds, chosen by the "Drill type" toggle:
//   - "Integrity check" (subset): restic check --read-data-subset on the selected
//     source repo — proves the backup data is intact + restorable.
//   - "Real restore (off-site)" (dr): a REAL sandbox restore of the newest
//     off-site snapshot, then verification + cleanup. Containers + flash only
//     (VMs are refused server-side — disk images too large to sandbox-restore).
// The DR-drill target (which container's off-site snapshot to restore) binds to
// the shared settings.drDrillTarget via the parent's baseline-merging save().
function IntegrityCard({
  t,
  settings,
  setSettings,
  save,
}: {
  t: ReturnType<typeof useT>["t"];
  settings: Settings;
  setSettings: React.Dispatch<React.SetStateAction<Settings | null>>;
  save: (
    patch: Partial<Settings>,
    setSaveState: (s: SaveState) => void,
    setSaveError: (e: string | null) => void
  ) => Promise<boolean>;
}) {
  // Prune deletes snapshots, so it stays advanced-only even though the rest of
  // this card (verify, unlock, DR drill) is a first-class default-mode feature.
  const { advanced } = useAdvanced();
  type ActState = "idle" | "busy" | "ok" | "fail";
  type DrillKind = "subset" | "dr";
  const [state, setState] = useState<Record<string, ActState>>({});
  const [msg, setMsg] = useState<Record<string, string>>({});
  const [source, setSource] = useState<RepoSource>("local");
  const [kind, setKind] = useState<DrillKind>("subset");
  // The last recorded drill per domain (for the current source), keyed by domain.
  const [lastDrill, setLastDrill] = useState<Record<string, RestoreDrill | null>>({});
  // Container list feeding the DR-drill target dropdown (kind "dr", containers).
  const [containers, setContainers] = useState<Container[]>([]);
  // Save state for the drill-target dropdown (persisted via the parent save()).
  const [tgtState, setTgtState] = useState<SaveState>("idle");
  const [tgtError, setTgtError] = useState<string | null>(null);

  type Domain = "containers" | "vms" | "flash";
  type Action = "verify" | "unlock" | "prune";

  const domains: { key: Domain; label: string }[] = [
    { key: "containers", label: t("settings.containersEnabled") },
    { key: "vms", label: t("settings.vmsEnabled") },
    { key: "flash", label: t("settings.flashEnabled") },
  ];

  // Load the containers once for the DR-drill target picker (includes orphans
  // that still have off-site backups, so any drillable target is selectable).
  useEffect(() => {
    let active = true;
    listContainers()
      .then((r) => {
        if (active && r.ok) setContainers(r.containers ?? []);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, []);

  // Load the latest drill for each domain on mount and whenever the source
  // changes, so the "last verified" line reflects the selected repo.
  useEffect(() => {
    let active = true;
    for (const { key: domain } of domains) {
      getDrills(domain, source, 1)
        .then((r) => {
          if (!active) return;
          if (r.ok) setLastDrill((m) => ({ ...m, [domain]: r.latest ?? null }));
        })
        .catch(() => undefined);
    }
    return () => {
      active = false;
    };
    // domains is a stable literal list; re-run only when the source changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [source]);

  async function run(domain: Domain, action: Action) {
    if (action === "prune" && !window.confirm(t("integrity.pruneConfirm"))) return;
    const key = `${domain}:${action}`;
    setState((s) => ({ ...s, [key]: "busy" }));
    setMsg((m) => ({ ...m, [key]: "" }));
    try {
      const r =
        action === "verify" ? await checkDomain(domain, source)
        : action === "unlock" ? await unlockDomain(domain, source)
        : await pruneDomain(domain, source);
      if (r.ok) {
        setState((s) => ({ ...s, [key]: "ok" }));
      } else {
        setState((s) => ({ ...s, [key]: "fail" }));
        setMsg((m) => ({ ...m, [key]: r.error ?? t("integrity.failed") }));
      }
    } catch (err) {
      setState((s) => ({ ...s, [key]: "fail" }));
      setMsg((m) => ({ ...m, [key]: err instanceof Error ? err.message : t("integrity.failed") }));
    }
  }

  // runDrillFor runs a restore-verification drill and records its result inline,
  // mirroring the per-action result-state pattern above (keyed "<domain>:drill").
  // A "dr" drill does a REAL off-site restore into a sandbox — it always targets
  // the off-site repo (source is ignored) and asks for confirmation first.
  async function runDrillFor(domain: Domain) {
    if (kind === "dr" && !window.confirm(t("drill.confirmDR"))) return;
    const key = `${domain}:drill`;
    setState((s) => ({ ...s, [key]: "busy" }));
    setMsg((m) => ({ ...m, [key]: "" }));
    try {
      const r = await runDrill(domain, kind === "dr" ? "offsite" : source, kind);
      if (r.ok && r.drill) {
        const drill = r.drill;
        setLastDrill((m) => ({ ...m, [domain]: drill }));
        setState((s) => ({ ...s, [key]: drill.ok ? "ok" : "fail" }));
        if (!drill.ok) setMsg((m) => ({ ...m, [key]: drill.detail || t("verify.failed") }));
        // A recorded drill (pass OR fail) changes the shared /api/status the
        // dashboard scorecard reads. Broadcast so the Dashboard refetches its
        // drill / "proven restorable" pills without a page reload — mirrors how
        // saving settings signals the app to refresh.
        window.dispatchEvent(new Event("bv:settings-changed"));
      } else {
        setState((s) => ({ ...s, [key]: "fail" }));
        setMsg((m) => ({ ...m, [key]: r.error ?? t("verify.failed") }));
      }
    } catch (err) {
      setState((s) => ({ ...s, [key]: "fail" }));
      setMsg((m) => ({ ...m, [key]: err instanceof Error ? err.message : t("verify.failed") }));
    }
  }

  const actions: { key: Action; label: string; busy: string }[] = [
    { key: "verify", label: t("integrity.verify"), busy: t("integrity.checking") },
    { key: "unlock", label: t("integrity.unlock"), busy: "…" },
    // Prune deletes snapshots — keep it behind Advanced so novices can't reach it.
    ...(advanced ? [{ key: "prune" as Action, label: t("integrity.prune"), busy: "…" }] : []),
  ];

  const selectCls =
    "rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm px-2.5 py-1.5 focus:outline-none focus:border-[#78a9ff]";

  return (
    <Card title={t("integrity.title")}>
      <p className="text-xs text-carbon-textMuted -mt-1">{t("integrity.hint")}</p>
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("source.label")}</span>
        <SourceToggle
          source={source}
          onChange={(next) => {
            // The ok/fail indicators belong to the previously selected source —
            // clear them so a "healthy" result doesn't carry over to the other
            // repo where no maintenance has run yet. The drill state + cached
            // last-drill clear here too; the effect above reloads them for `next`.
            setSource(next);
            setState({});
            setMsg({});
            setLastDrill({});
          }}
          disabled={Object.values(state).some((v) => v === "busy")}
        />
      </div>

      {/* Drill-type toggle: subset integrity check vs a real off-site DR restore. */}
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("drill.kindLabel")}</span>
        <div className="inline-flex rounded-lg border border-carbon-border overflow-hidden">
          {([
            ["subset", t("drill.kindSubset")],
            ["dr", t("drill.kindDR")],
          ] as const).map(([val, label]) => (
            <button
              key={val}
              type="button"
              onClick={() => {
                // Clear any lingering per-domain result so a subset "healthy"
                // doesn't read as a DR pass (or vice versa) after switching kind.
                setKind(val);
                setState({});
                setMsg({});
              }}
              disabled={Object.values(state).some((v) => v === "busy")}
              className={`px-2.5 py-1 text-xs transition-colors disabled:opacity-50 ${
                kind === val
                  ? "bg-accent text-accentContrast"
                  : "text-carbon-textSub hover:text-carbon-text"
              }`}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      {/* DR-drill controls: an explainer + the container target picker. The target
          is a shared setting (settings.drDrillTarget) saved via the parent's
          baseline-merging save(), so it never clobbers other cards' edits. Flash
          has no picker (its whole snapshot is restored); VMs are refused below. */}
      {kind === "dr" && (
        <div className="flex flex-col gap-2 rounded-lg bg-carbon-surface2 border border-carbon-border p-3">
          <p className="text-xs text-carbon-textMuted">{t("drill.drNote")}</p>
          <label className="flex flex-col gap-1 text-xs text-carbon-textSub max-w-xs">
            {t("drill.target")}
            <select
              value={settings.drDrillTarget}
              onChange={(e) => {
                const v = e.target.value;
                setSettings((prev) => (prev ? { ...prev, drDrillTarget: v } : prev));
                void save({ drDrillTarget: v }, setTgtState, setTgtError);
              }}
              className={selectCls}
            >
              <option value="">{t("drill.targetMostRecent")}</option>
              {containers.map((c) => (
                <option key={c.name} value={c.name}>{c.name}</option>
              ))}
            </select>
          </label>
          {tgtState === "saved" && <span className="text-xs text-[#6fdc8c]">{t("settings.saved")}</span>}
          {tgtState === "error" && tgtError && <span className="text-xs text-[#ff8389]">{tgtError}</span>}
        </div>
      )}

      <div className="flex flex-col gap-3">
        {domains.map(({ key: domain, label }) => {
          const dKey = `${domain}:drill`;
          const drill = lastDrill[domain];
          // A DR drill can't run for VMs (server refuses it) — show a short note
          // in place of the run button instead of a button that always errors.
          const drDisabledForVM = kind === "dr" && domain === "vms";
          return (
            <div key={domain} className="flex flex-col gap-1">
              <div className="flex items-center gap-2 flex-wrap">
                <span className="text-sm text-carbon-textSub w-24 shrink-0">{label}</span>
                {actions.map((a) => {
                  const k = `${domain}:${a.key}`;
                  return (
                    <span key={a.key} className="inline-flex items-center gap-1">
                      <button
                        onClick={() => void run(domain, a.key)}
                        disabled={state[k] === "busy"}
                        title={t(`integrity.${a.key}Hint`)}
                        className="rounded-lg border border-carbon-border bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
                      >
                        {state[k] === "busy" ? a.busy : a.label}
                      </button>
                      {state[k] === "ok" && <span className="text-sm text-[#6fdc8c]">{t("integrity.ok")}</span>}
                    </span>
                  );
                })}
              </div>

              {/* Restore-verification drill: its own row + inline result + last drill.
                  The run button + labels follow the selected drill kind; VMs can't
                  run a DR restore, so their row shows a note instead. */}
              <div className="flex items-center gap-2 flex-wrap">
                <span className="w-24 shrink-0" />
                {drDisabledForVM ? (
                  <span className="text-xs text-carbon-textMuted">{t("drill.drVMsNote")}</span>
                ) : (
                  <>
                    <button
                      onClick={() => void runDrillFor(domain)}
                      disabled={state[dKey] === "busy"}
                      title={kind === "dr" ? t("drill.drNote") : t("verify.hint")}
                      className="rounded-lg border border-carbon-border bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
                    >
                      {state[dKey] === "busy"
                        ? kind === "dr" ? t("drill.runningDR") : t("verify.running")
                        : kind === "dr" ? t("drill.runDR") : t("verify.now")}
                    </button>
                    {state[dKey] === "ok" && <span className="text-sm text-[#6fdc8c]">✓ {t("verify.ok")}</span>}
                    {state[dKey] === "fail" && (
                      <span className="text-sm text-[#ff8389] break-words">✗ {msg[dKey] || t("verify.failed")}</span>
                    )}
                    {/* Last recorded drill for this domain/source (idle state only).
                        Names WHICH check ran (off-site DR vs local integrity) and,
                        on a stored failure, the scrubbed reason. */}
                    {state[dKey] !== "busy" && state[dKey] !== "ok" && state[dKey] !== "fail" && (
                      drill ? (
                        <>
                          <span className="text-xs text-carbon-textMuted">
                            {drill.source === "offsite" && drill.kind === "dr"
                              ? t("drill.checkOffsiteDr")
                              : t("drill.checkLocal")}
                            {" · "}
                            {t("verify.last").replace("{time}", relativeTime(t, drill.at))} {drill.ok ? "✓" : "✗"}
                          </span>
                          {!drill.ok && drill.detail && (
                            <span className="text-xs text-[#ff8389] break-words" title={drill.detail}>
                              {t("drill.failReasonPrefix")} {drill.detail}
                            </span>
                          )}
                        </>
                      ) : (
                        <span className="text-xs text-carbon-textMuted">{t("verify.never")}</span>
                      )
                    )}
                  </>
                )}
              </div>

              {actions.map((a) =>
                state[`${domain}:${a.key}`] === "fail" ? (
                  <span key={a.key} className="text-xs text-[#ff8389] break-words">
                    {a.label}: {msg[`${domain}:${a.key}`] || t("integrity.failed")}
                  </span>
                ) : null
              )}
            </div>
          );
        })}
      </div>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// Schedule editors — migrated verbatim from the retired Plans page (Jobs.tsx).
// The Schedules tab is now the single owner of every backup/off-site/self-backup/
// restore-check cadence. These render their own Cards (same as on the old Plans
// page); behaviour is unchanged.
// ---------------------------------------------------------------------------

/** Convert a cadence string to a human-readable label. */
function cadenceLabel(raw: string, t: ReturnType<typeof useT>["t"]): string {
  const s = (raw ?? "").trim();
  if (!s || s === "off") return t("jobs.notScheduled");

  const dailyM = /^daily\s+(\d{1,2}:\d{2})$/.exec(s);
  if (dailyM) return t("jobs.cadenceDaily").replace("{time}", dailyM[1]);

  const weeklyM = /^weekly\s+([\w,]+)\s+(\d{1,2}:\d{2})$/.exec(s);
  if (weeklyM) return t("jobs.cadenceWeekly").replace("{days}", weeklyM[1]).replace("{time}", weeklyM[2]);

  const everyNM = /^everyN\s+(\d+)\s+(\d{1,2}:\d{2})$/.exec(s);
  if (everyNM) return t("jobs.cadenceEveryN").replace("{n}", everyNM[1]).replace("{time}", everyNM[2]);

  return s;
}

type ScheduleStatus = "active" | "paused" | "off";

function scheduleStatus(schedule: string): ScheduleStatus {
  if (!schedule || schedule === "off") return "off";
  return "active";
}

function ScheduleBadge({
  status,
  label,
}: {
  status: ScheduleStatus;
  label: string;
}) {
  const cls: Record<ScheduleStatus, string> = {
    active: "bg-[#1c3a2a] text-[#6fdc8c] border border-[#2a5540]",
    paused: "bg-[#2a2a1c] text-[#f1c21b] border border-[#4a4a2a]",
    off:    "bg-carbon-surface2 text-carbon-textSub border border-carbon-border",
  };
  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${cls[status]}`}
    >
      {label}
    </span>
  );
}

// Domain section — Containers (editable schedule + included-containers list)
function ContainersSection({
  settings,
  containers,
  onChange,
  t,
}: {
  settings: Settings;
  containers: Container[];
  onChange: (schedule: string) => void;
  t: ReturnType<typeof useT>["t"];
}) {
  const schedule = settings.containersSchedule;
  const status = scheduleStatus(schedule);
  // Exclude BombVault's own container: it can never be backed up, so it must
  // never appear as a schedule member even if a stale flag lingers on its row.
  const included = containers.filter((c) => c.installed && c.includeInSchedule && !c.self);

  return (
    <Card title={t("jobs.containersSection")}>
      {/* Cadence row */}
      <div className="flex items-center gap-3 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("settings.schedule")}:</span>
        <ScheduleBadge
          status={status}
          label={
            status === "off"
              ? t("jobs.notScheduled")
              : cadenceLabel(schedule, t)
          }
        />
      </div>

      {/* Editable cadence builder */}
      <div className="rounded-lg bg-carbon-surface2 border border-carbon-border p-4">
        <CadenceBuilder
          label={t("jobs.containersSection")}
          value={schedule}
          onChange={onChange}
        />
      </div>

      {/* Member list */}
      {included.length === 0 ? (
        <p className="text-sm text-carbon-textMuted">{t("jobs.noContainersIncluded")}</p>
      ) : (
        <div className="flex flex-col gap-1 divide-y divide-carbon-border">
          {included.map((c) => (
            <div
              key={c.name}
              className="flex items-center gap-3 py-2 text-sm"
            >
              <div
                className={`w-2 h-2 rounded-full shrink-0 ${
                  c.state.toLowerCase() === "running"
                    ? "bg-[#6fdc8c]"
                    : "bg-carbon-surface3"
                }`}
              />
              <span className="font-medium text-carbon-text flex-1 truncate">
                {c.name}
              </span>
              {c.image && (
                <span className="text-xs text-carbon-textMuted truncate hidden sm:block max-w-xs">
                  {c.image}
                </span>
              )}
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

// Domain section — VMs (editable schedule)
function VMsSection({
  settings,
  syncSchedules,
  onChange,
  t,
}: {
  settings: Settings;
  syncSchedules: boolean;
  onChange: (schedule: string) => void;
  t: ReturnType<typeof useT>["t"];
}) {
  const schedule = syncSchedules ? settings.containersSchedule : settings.vmsSchedule;
  const status = scheduleStatus(schedule);

  return (
    <Card title={t("jobs.vmsSection")}>
      <div className="flex items-center gap-3 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("settings.schedule")}:</span>
        <ScheduleBadge
          status={status}
          label={
            status === "off"
              ? t("jobs.notScheduled")
              : cadenceLabel(schedule, t)
          }
        />
      </div>
      <div className={`rounded-lg bg-carbon-surface2 border border-carbon-border p-4 ${syncSchedules ? "opacity-50" : ""}`}>
        <CadenceBuilder
          label={t("jobs.vmsSection")}
          value={schedule}
          disabled={syncSchedules}
          onChange={onChange}
        />
        {!syncSchedules && (
          <p className="text-xs text-carbon-textMuted mt-2">{t("jobs.vmIncludeHint")}</p>
        )}
      </div>
    </Card>
  );
}

// Domain section — Flash (editable schedule)
function FlashSection({
  settings,
  syncSchedules,
  onChange,
  t,
}: {
  settings: Settings;
  syncSchedules: boolean;
  onChange: (schedule: string) => void;
  t: ReturnType<typeof useT>["t"];
}) {
  const schedule = syncSchedules ? settings.containersSchedule : settings.flashSchedule;
  const status = scheduleStatus(schedule);

  return (
    <Card title={t("jobs.flashSection")}>
      <div className="flex items-center gap-3 flex-wrap">
        <span className="text-xs text-carbon-textMuted">{t("settings.schedule")}:</span>
        <ScheduleBadge
          status={status}
          label={
            status === "off"
              ? t("jobs.notScheduled")
              : cadenceLabel(schedule, t)
          }
        />
      </div>
      <div className={`rounded-lg bg-carbon-surface2 border border-carbon-border p-4 ${syncSchedules ? "opacity-50" : ""}`}>
        <CadenceBuilder
          label={t("jobs.flashSection")}
          value={schedule}
          disabled={syncSchedules}
          onChange={onChange}
        />
        {!syncSchedules && (
          <p className="text-xs text-carbon-textMuted mt-2">{t("jobs.flashNotImplemented")}</p>
        )}
      </div>
      <div className="flex items-center gap-3 py-2 text-sm border-t border-carbon-border">
        <div className="w-2 h-2 rounded-full bg-carbon-surface3 shrink-0" />
        <span className="font-medium text-carbon-text flex-1">{t("jobs.flashRow")}</span>
        <span className="text-xs text-carbon-textMuted italic">{t("jobs.flashPlanned")}</span>
      </div>
    </Card>
  );
}

// Domain section — Restore checks (scheduled restore-verification drills).
// The drill schedule sits beside the backup schedules; always visible.
function RestoreChecksSection({
  settings,
  update,
  t,
}: {
  settings: Settings;
  update: (patch: Partial<Settings>) => void;
  t: ReturnType<typeof useT>["t"];
}) {
  return (
    <Card title={t("verify.auto")}>
      <p className="text-xs text-carbon-textMuted -mt-1">{t("verify.hint")}</p>
      <ToggleRow
        label={t("verify.auto")}
        checked={settings.drillsEnabled}
        onChange={(v) => update({ drillsEnabled: v })}
      />
      {/* Sub-toggle: only meaningful while scheduled drills are on. */}
      <div className={settings.drillsEnabled ? "" : "opacity-50"}>
        <ToggleRow
          label={t("settings.offsiteDrills")}
          description={t("settings.offsiteDrillsHelp")}
          checked={settings.offsiteDrillsEnabled}
          disabled={!settings.drillsEnabled}
          onChange={(v) => update({ offsiteDrillsEnabled: v })}
        />
      </div>
      <div className={`rounded-lg bg-carbon-surface2 border border-carbon-border p-4 ${settings.drillsEnabled ? "" : "opacity-50"}`}>
        <CadenceBuilder
          label={t("settings.schedule")}
          value={settings.drillsSchedule}
          disabled={!settings.drillsEnabled}
          onChange={(v) => update({ drillsSchedule: v })}
        />
      </div>
      <label className="flex flex-col gap-1 max-w-[10rem]">
        <span className="text-xs text-carbon-textSub">{t("verify.subsetPct")}</span>
        <input
          type="number"
          min={1}
          max={100}
          value={settings.drillsSubsetPct}
          onChange={(e) => {
            const n = parseInt(e.target.value, 10);
            const clamped = isNaN(n) ? 1 : Math.min(100, Math.max(1, n));
            update({ drillsSubsetPct: clamped });
          }}
          className="rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm px-3 py-1.5 w-full focus:outline-none focus:border-[#78a9ff]"
        />
      </label>
    </Card>
  );
}

// TabKey enumerates the 7 Settings tabs. The active tab is the single source of
// truth for which card group renders; SettingsPage owns all shared state so every
// tab shares one `settings`/`save()` instance regardless of which tab is visible.
type TabKey =
  | "general"
  | "storage"
  | "schedules"
  | "offsite"
  | "notifications"
  | "integrity"
  | "system";

export function SettingsPage() {
  const { t } = useT();
  const { advanced } = useAdvanced();

  const [tab, setTab] = useState<TabKey>("general");
  const [settings, setSettings] = useState<Settings | null>(null);
  // savedSettings is the server's last-confirmed state. Each card's Save persists
  // its own fields merged onto THIS baseline (not the live, possibly-edited
  // `settings`), so saving one card never silently commits another card's
  // unsaved edits.
  const [savedSettings, setSavedSettings] = useState<Settings | null>(null);
  const [hostMountRoot, setHostMountRoot] = useState<string>("/host/user");
  const [loadError, setLoadError] = useState<string | null>(null);

  // Auth state for the Security card.
  const [authEnabled, setAuthEnabled] = useState(false);
  const [authAuthed, setAuthAuthed] = useState(false);
  const [pwNew, setPwNew] = useState("");
  const [pwConfirm, setPwConfirm] = useState("");
  const [pwSaveState, setPwSaveState] = useState<SaveState>("idle");
  const [pwSaveMsg, setPwSaveMsg] = useState<string | null>(null);

  // Accent color state — synced from/to localStorage via accent.ts
  const [accentHex, setAccentHex] = useState<string>(() => getAccent());

  // Per-section save state
  const [encSaveState, setEncSaveState] = useState<SaveState>("idle");
  const [encSaveError, setEncSaveError] = useState<string | null>(null);

  const [pathSaveState, setPathSaveState] = useState<SaveState>("idle");
  const [pathSaveError, setPathSaveError] = useState<string | null>(null);
  // Flash zip export (#28) — its own save state, persisted via the shared save().
  const [flashZipSaveState, setFlashZipSaveState] = useState<SaveState>("idle");
  const [flashZipSaveError, setFlashZipSaveError] = useState<string | null>(null);
  // Remembers the last "keep N" the user picked so toggling history OFF (which
  // zeroes flashZipExportKeep) and back ON restores their count instead of the
  // default. Updated whenever the keepN input is set to a value >= 1.
  const [rememberedKeep, setRememberedKeep] = useState(7);
  const [offsiteSaveState, setOffsiteSaveState] = useState<SaveState>("idle");
  const [offsiteSaveError, setOffsiteSaveError] = useState<string | null>(null);
  // Which domain's guided off-site setup wizard is expanded (null = none).
  const [offsiteWizard, setOffsiteWizard] = useState<"containers" | "vms" | "flash" | null>(null);

  const [domSaveState, setDomSaveState] = useState<SaveState>("idle");
  const [domSaveError, setDomSaveError] = useState<string | null>(null);

  const [retSaveState, setRetSaveState] = useState<SaveState>("idle");
  const [retSaveError, setRetSaveError] = useState<string | null>(null);

  const [limSaveState, setLimSaveState] = useState<SaveState>("idle");
  const [limSaveError, setLimSaveError] = useState<string | null>(null);

  const [metricsSaveState, setMetricsSaveState] = useState<SaveState>("idle");
  const [metricsSaveError, setMetricsSaveError] = useState<string | null>(null);

  // Schedules tab (migrated from the retired Plans page). The container list
  // feeds the Containers schedule section's included-members list; syncSchedules
  // applies the Containers cadence to VMs + Flash; schedSave* drives its SaveBar.
  const [containers, setContainers] = useState<Container[]>([]);
  const [syncSchedules, setSyncSchedules] = useState(false);
  const [schedSaveState, setSchedSaveState] = useState<SaveState>("idle");
  const [schedSaveError, setSchedSaveError] = useState<string | null>(null);

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok) {
          setSettings(res.settings);
          setSavedSettings(res.settings);
          if (res.hostMountRoot) setHostMountRoot(res.hostMountRoot);
          // Detect whether the domain schedules are already in sync (Containers ==
          // VMs == Flash, and not off), so the Schedules tab's sync checkbox
          // reflects it on load. Reproduced from the retired Plans page.
          const s = res.settings;
          if (
            s.vmsSchedule === s.containersSchedule &&
            s.flashSchedule === s.containersSchedule &&
            s.containersSchedule !== "off" &&
            s.containersSchedule !== ""
          ) {
            setSyncSchedules(true);
          }
        } else {
          setLoadError("Failed to load settings");
        }
      })
      .catch(() => setLoadError("Failed to load settings"));

    // Load auth status for the Security card.
    getAuth()
      .then((res) => {
        setAuthEnabled(res.enabled);
        setAuthAuthed(res.authed);
      })
      .catch(() => {
        // Non-fatal: Security card shows auth as off.
      });

    // Load the container list for the Schedules tab's Containers section (its
    // included-members list). Non-fatal: an empty list just shows no members.
    listContainers()
      .then((r) => {
        if (r.ok) setContainers(r.containers ?? []);
      })
      .catch(() => {
        // Non-fatal: the Containers schedule section shows an empty member list.
      });
  }, []);

  // Deep-link support: /settings#offsite (and every other tab hash) selects the
  // matching tab instead of scrolling. Read once on mount, and also listen for
  // hashchange so an in-app "#offsite" link fired while already on /settings
  // switches the tab (no remount happens in that case). The Dashboard's
  // "Link to /settings#offsite" therefore lands on the Off-site tab.
  useEffect(() => {
    const tabs: TabKey[] = [
      "general",
      "storage",
      "schedules",
      "offsite",
      "notifications",
      "integrity",
      "system",
    ];
    const applyHash = () => {
      const h = window.location.hash.replace(/^#/, "");
      if ((tabs as string[]).includes(h)) setTab(h as TabKey);
    };
    applyHash();
    window.addEventListener("hashchange", applyHash);
    return () => window.removeEventListener("hashchange", applyHash);
  }, []);

  // While "sync" is on, mirror the Containers cadence onto VMs + Flash in live
  // state (not just in the save patch), so unchecking sync doesn't snap the
  // VM/Flash editors back to stale pre-sync values. The equality guard stops
  // re-renders from looping. Reproduced verbatim from the retired Plans page.
  useEffect(() => {
    if (!syncSchedules) return;
    setSettings((prev) => {
      if (!prev) return prev;
      if (
        prev.vmsSchedule === prev.containersSchedule &&
        prev.flashSchedule === prev.containersSchedule
      ) {
        return prev;
      }
      return { ...prev, vmsSchedule: prev.containersSchedule, flashSchedule: prev.containersSchedule };
    });
  }, [syncSchedules, settings?.containersSchedule]);

  // ---------------------------------------------------------------------------
  // Generic save helper
  // ---------------------------------------------------------------------------

  // save persists one card's fields and returns true ONLY when the server confirmed
  // the write. Callers that gate a follow-up action on a confirmed save (e.g. the
  // off-site immutable toggle, which must not run a tamper test on a failed save)
  // await the boolean; fire-and-forget callers can still ignore it via `void`.
  async function save(
    patch: Partial<Settings>,
    setSaveState: (s: SaveState) => void,
    setSaveError: (e: string | null) => void
  ): Promise<boolean> {
    const base = savedSettings ?? settings;
    if (!base) return false;
    setSaveState("saving");
    setSaveError(null);
    // Persist ONLY this card's fields, merged onto the server baseline — never the
    // live `settings`, which may hold unsaved edits from other cards.
    const updated: Settings = { ...base, ...patch };
    try {
      const res = await putSettings(updated);
      if (res.ok) {
        // Advance the baseline; reflect just the saved fields in the live state so
        // other cards' in-progress edits are left untouched.
        setSavedSettings(updated);
        setSettings((prev) => (prev ? { ...prev, ...patch } : updated));
        setSaveState("saved");
        // Tell the Layout/Sidebar to refetch so a newly enabled/disabled domain
        // tab appears or vanishes immediately — no page reload needed.
        window.dispatchEvent(new Event("bv:settings-changed"));
        setTimeout(() => setSaveState("idle"), 3000);
        return true;
      }
      setSaveError(res.error ?? t("settings.error"));
      setSaveState("error");
      return false;
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : t("settings.error"));
      setSaveState("error");
      return false;
    }
  }

  // buildSchedulePatch collects EVERY schedule field for the Schedules tab's one
  // SaveBar, applying Jobs' exact sync semantics: Containers always; VMs + Flash
  // mirror Containers when synced, else their own value. Persisted via save(),
  // which merges onto the savedSettings baseline (never clobbering other tabs).
  function buildSchedulePatch(): Partial<Settings> {
    if (!settings) return {};
    const patch: Partial<Settings> = {
      containersSchedule: settings.containersSchedule,
    };
    if (syncSchedules) {
      patch.vmsSchedule = settings.containersSchedule;
      patch.flashSchedule = settings.containersSchedule;
    } else {
      patch.vmsSchedule = settings.vmsSchedule;
      patch.flashSchedule = settings.flashSchedule;
    }
    // Restore-check (drill) schedule.
    patch.drillsEnabled = settings.drillsEnabled;
    patch.offsiteDrillsEnabled = settings.offsiteDrillsEnabled;
    patch.drillsSchedule = settings.drillsSchedule;
    patch.drillsSubsetPct = settings.drillsSubsetPct;
    // Off-site replication cadences (+ config) — sole owner is now this tab.
    patch.containersOffsiteSchedule = settings.containersOffsiteSchedule;
    patch.vmsOffsiteSchedule = settings.vmsOffsiteSchedule;
    patch.flashOffsiteSchedule = settings.flashOffsiteSchedule;
    patch.configOffsiteSchedule = settings.configOffsiteSchedule;
    // Self-backup cadence + scheduled off-site tamper test.
    patch.configSchedule = settings.configSchedule;
    patch.tamperTestSchedule = settings.tamperTestSchedule;
    return patch;
  }

  if (loadError) {
    return (
      <div className="max-w-3xl">
        <p className="text-sm text-[#ff8389]">{loadError}</p>
      </div>
    );
  }

  if (!settings) {
    return (
      <div className="max-w-3xl">
        <p className="text-sm text-carbon-textMuted">{t("dashboard.checking")}</p>
      </div>
    );
  }

  // ---------------------------------------------------------------------------
  // Auth / Security helpers
  // ---------------------------------------------------------------------------

  async function handleSetPassword() {
    if (pwNew !== pwConfirm) {
      setPwSaveMsg(t("auth.passwordMismatch"));
      setPwSaveState("error");
      return;
    }
    setPwSaveState("saving");
    setPwSaveMsg(null);
    try {
      const res = await setAuthPassword(pwNew);
      if (res.ok) {
        setAuthEnabled(res.enabled ?? false);
        setPwSaveState("saved");
        setPwSaveMsg(pwNew === "" ? t("auth.passwordCleared") : t("auth.passwordSaved"));
        setPwNew("");
        setPwConfirm("");
        setTimeout(() => { setPwSaveState("idle"); setPwSaveMsg(null); }, 3000);
      } else {
        setPwSaveMsg(res.error ?? t("auth.saveError"));
        setPwSaveState("error");
      }
    } catch {
      setPwSaveMsg(t("auth.saveError"));
      setPwSaveState("error");
    }
  }

  async function handleLogout() {
    await logout().catch(() => undefined);
    // Reload so the auth gate re-checks and shows the login screen.
    window.location.reload();
  }

  return (
    <div className="flex flex-col gap-6 max-w-3xl">
      {/* Page heading */}
      <div>
        <h1 className="text-2xl font-semibold text-carbon-text">
          {t("settings.title")}
        </h1>
        <p className="mt-1 text-sm text-carbon-textSub">
          {t("settings.subtitle")}
        </p>
      </div>

      {/* ------------------------------------------------------------------ */}
      {/* Segmented tab bar (7 tabs). `tab` is the single owner of which card  */}
      {/* group renders; it wraps/scrolls gracefully for narrow widths.        */}
      {/* ------------------------------------------------------------------ */}
      <div className="flex flex-wrap gap-1">
        {([
          ["general", t("settings.tab.general")],
          ["storage", t("settings.tab.storage")],
          ["schedules", t("settings.tab.schedules")],
          ["offsite", t("settings.tab.offsite")],
          ["notifications", t("settings.tab.notifications")],
          ["integrity", t("settings.tab.integrity")],
          ["system", t("settings.tab.system")],
        ] as const).map(([key, label]) => (
          <button
            key={key}
            type="button"
            onClick={() => {
              setTab(key);
              // Keep the URL hash in sync so reload/bookmark restores the tab
              // (replaceState avoids polluting history and won't re-fire applyHash).
              try {
                window.history.replaceState(null, "", `#${key}`);
              } catch {
                /* history unavailable — tab state still switches */
              }
            }}
            className={`rounded-lg border px-3 py-1.5 text-sm transition-colors ${
              tab === key
                ? "border-accent bg-accent text-accentContrast"
                : "border-carbon-border text-carbon-textSub hover:text-carbon-text hover:bg-carbon-hover"
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {/* ------------------------------------------------------------------ */}
      {/* SCHEDULES — the single owner of every cadence (migrated from Plans).  */}
      {/* Backup schedules reuse the proven per-domain sections + sync checkbox; */}
      {/* off-site / self-backup / restore-check cadences are edited here too.   */}
      {/* One SaveBar persists them all via the shared baseline-merging save().  */}
      {/* ------------------------------------------------------------------ */}
      {tab === "schedules" && (
        <>
          {/* Backup schedules (schedulesBackup): Containers + sync + VMs + Flash.
              A group heading (Card-title style) labels the three domain cards,
              matching the single-Card off-site / self-backup / checks groups. */}
          <h2 className="text-sm font-semibold text-carbon-textSub uppercase tracking-widest">
            {t("settings.schedulesBackup")}
          </h2>
          <ContainersSection
            settings={settings}
            containers={containers}
            onChange={(v) =>
              setSettings((prev) => (prev ? { ...prev, containersSchedule: v } : prev))
            }
            t={t}
          />
          {/* Sync checkbox — applies the Containers cadence to VMs + Flash too. */}
          <label className="flex items-center gap-2 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={syncSchedules}
              onChange={(e) => setSyncSchedules(e.target.checked)}
              className="h-4 w-4 rounded border-carbon-border bg-carbon-surface2 accent-[var(--accent)]"
            />
            <span className="text-sm text-carbon-text">{t("jobs.syncSchedules")}</span>
          </label>
          <VMsSection
            settings={settings}
            syncSchedules={syncSchedules}
            onChange={(v) =>
              setSettings((prev) => (prev ? { ...prev, vmsSchedule: v } : prev))
            }
            t={t}
          />
          <FlashSection
            settings={settings}
            syncSchedules={syncSchedules}
            onChange={(v) =>
              setSettings((prev) => (prev ? { ...prev, flashSchedule: v } : prev))
            }
            t={t}
          />

          {/* Off-site replication schedules (schedulesOffsite): one cadence per
              domain (+ config). Editors here are the sole owner of these fields. */}
          <Card title={t("settings.schedulesOffsite")}>
            {([
              ["containersOffsiteSchedule", "nav.containers"],
              ["vmsOffsiteSchedule", "nav.vms"],
              ["flashOffsiteSchedule", "nav.flash"],
              ["configOffsiteSchedule", "nav.config"],
            ] as const).map(([key, label]) => (
              <div key={key} className="flex flex-col gap-1">
                <span className="text-xs text-carbon-textSub">{t(label)}</span>
                <input
                  value={settings[key]}
                  spellCheck={false}
                  onChange={(e) =>
                    setSettings((prev) => (prev ? { ...prev, [key]: e.target.value } : prev))
                  }
                  placeholder={t("offsite.schedulePlaceholder")}
                  className="rounded-lg border border-carbon-border bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono focus:outline-none focus:ring-1 focus:ring-accent"
                />
              </div>
            ))}
          </Card>

          {/* Self-backup schedule (schedulesSelfBackup): BombVault's own config. */}
          <Card title={t("settings.schedulesSelfBackup")}>
            <div className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t("nav.config")}</span>
              <input
                value={settings.configSchedule}
                spellCheck={false}
                onChange={(e) =>
                  setSettings((prev) => (prev ? { ...prev, configSchedule: e.target.value } : prev))
                }
                placeholder={t("config.schedulePlaceholder")}
                className="rounded-lg border border-carbon-border bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono focus:outline-none focus:ring-1 focus:ring-accent"
              />
              <p className="text-xs text-carbon-textMuted">{t("config.scheduleHint")}</p>
            </div>
          </Card>

          {/* Restore-check drills (RestoreChecksSection renders its own Card). */}
          <RestoreChecksSection
            settings={settings}
            update={(patch) =>
              setSettings((prev) => (prev ? { ...prev, ...patch } : prev))
            }
            t={t}
          />

          {/* Restore-check schedule (schedulesChecks): the scheduled off-site
              append-only tamper test. Previously had no UI editor at all. */}
          <Card title={t("settings.schedulesChecks")}>
            <div className="rounded-lg bg-carbon-surface2 border border-carbon-border p-4">
              <CadenceBuilder
                label={t("settings.tamperTestSchedule")}
                value={settings.tamperTestSchedule}
                onChange={(v) =>
                  setSettings((prev) => (prev ? { ...prev, tamperTestSchedule: v } : prev))
                }
              />
            </div>
          </Card>

          {/* One Save persists every schedule field via the shared save(). */}
          <SaveBar
            state={schedSaveState}
            error={schedSaveError}
            onSave={() => void save(buildSchedulePatch(), setSchedSaveState, setSchedSaveError)}
            t={t}
          />
        </>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* GENERAL — Domains                                                   */}
      {/* ------------------------------------------------------------------ */}
      {tab === "general" && (
      <Card title={t("settings.domains")}>
        <p className="text-xs text-carbon-textMuted -mt-1">
          Turn each backup domain on or off. Enabling VMs or Flash reveals its
          tab in the sidebar.
        </p>
        <ToggleRow
          label={t("settings.containersEnabled")}
          description="Container backup + restore (always enabled)"
          checked={settings.containersEnabled}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, containersEnabled: v } : prev)
          }
        />
        <ToggleRow
          label={t("settings.vmsEnabled")}
          description="VM backup + restore via libvirt over SSH"
          checked={settings.vmsEnabled}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, vmsEnabled: v } : prev)
          }
        />
        <ToggleRow
          label={t("settings.flashEnabled")}
          description="Unraid USB flash backup (/boot → restic)"
          checked={settings.flashEnabled}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, flashEnabled: v } : prev)
          }
        />
        <ToggleRow
          label={t("settings.configEnabled")}
          description="BombVault's own settings, targets and credentials (self-backup)"
          checked={settings.configEnabled}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, configEnabled: v } : prev)
          }
        />
        <SaveBar
          state={domSaveState}
          error={domSaveError}
          onSave={() =>
            void save(
              {
                containersEnabled: settings.containersEnabled,
                vmsEnabled: settings.vmsEnabled,
                flashEnabled: settings.flashEnabled,
                configEnabled: settings.configEnabled,
              },
              setDomSaveState,
              setDomSaveError
            )
          }
          t={t}
        />
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE — Backup paths                                             */}
      {/* ------------------------------------------------------------------ */}
      {tab === "storage" && (
      <Card title={t("settings.paths")}>
        <p className="text-xs text-carbon-textMuted -mt-1">
          Relative subpaths under the host mount root (
          <span className="font-mono">{hostMountRoot}</span>). Click Browse to
          navigate directories or type a path directly.
        </p>
        <FolderBrowser
          label={t("settings.containersPath")}
          value={settings.containersPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, containersPath: v } : prev)
          }
        />
        <FolderBrowser
          label={t("settings.vmsPath")}
          value={settings.vmsPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, vmsPath: v } : prev)
          }
        />
        <FolderBrowser
          label={t("settings.flashPath")}
          value={settings.flashPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, flashPath: v } : prev)
          }
        />
        <div className="flex flex-col gap-1">
          <FolderBrowser
            label={t("settings.restoreFolder")}
            value={settings.restoreFolder}
            hostMountRoot={hostMountRoot}
            onChange={(v) =>
              setSettings((prev) => prev ? { ...prev, restoreFolder: v } : prev)
            }
          />
          <p className="text-xs text-carbon-textMuted">{t("settings.restoreFolderHint")}</p>
        </div>
        <SaveBar
          state={pathSaveState}
          error={pathSaveError}
          onSave={() =>
            void save(
              {
                containersPath: settings.containersPath,
                vmsPath: settings.vmsPath,
                flashPath: settings.flashPath,
                restoreFolder: settings.restoreFolder,
              },
              setPathSaveState,
              setPathSaveError
            )
          }
          t={t}
        />
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE — Flash zip export (#28) — a plain .zip written after each   */}
      {/* flash backup, for off-server sync. Only relevant when Flash is on.   */}
      {/* ------------------------------------------------------------------ */}
      {tab === "storage" && settings.flashEnabled && (
      <Card title={t("flash.zipExport.title")}>
        <p className="text-xs text-carbon-textMuted -mt-1">{t("flash.zipExport.hint")}</p>
        <ToggleRow
          label={t("flash.zipExport.enable")}
          description={t("flash.zipExport.enableHint")}
          checked={settings.flashZipExportEnabled}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, flashZipExportEnabled: v } : prev)
          }
        />
        {settings.flashZipExportEnabled && (
          <>
            <div className="rounded-lg bg-[#2a2a1c] border border-[#4a4a2a] px-3 py-2.5 text-xs text-[#f1c21b] leading-relaxed">
              {t("flash.zipExport.plaintextWarn")}
            </div>
            <FolderBrowser
              label={t("flash.zipExport.path")}
              value={settings.flashZipExportPath}
              hostMountRoot={hostMountRoot}
              onChange={(v) =>
                setSettings((prev) => prev ? { ...prev, flashZipExportPath: v } : prev)
              }
            />
            <p className="text-xs text-carbon-textMuted -mt-1">{t("flash.zipExport.pathHint")}</p>
            {!settings.flashZipExportPath.trim() && (
              <p className="text-xs text-[#ff8389] -mt-1">{t("flash.zipExport.pathRequired")}</p>
            )}
            <ToggleRow
              label={t("flash.zipExport.keepHistory")}
              description={t("flash.zipExport.keepHistoryHint")}
              // History is "on" whenever we keep more than a single overwritten zip.
              // Turning it on restores the last count the user picked (rememberedKeep,
              // default 7); off collapses back to 0 = a single flash-latest.zip.
              checked={settings.flashZipExportKeep > 0}
              onChange={(v) =>
                setSettings((prev) =>
                  prev
                    ? { ...prev, flashZipExportKeep: v ? rememberedKeep : 0 }
                    : prev
                )
              }
            />
            {settings.flashZipExportKeep > 0 ? (
              <label className="flex flex-col gap-1 max-w-[10rem]">
                <span className="text-xs text-carbon-textSub">{t("flash.zipExport.keepN")}</span>
                <input
                  type="number"
                  min={1}
                  value={settings.flashZipExportKeep}
                  onChange={(e) => {
                    const n = Math.max(1, parseInt(e.target.value, 10) || 1);
                    setRememberedKeep(n);
                    setSettings((prev) => prev ? { ...prev, flashZipExportKeep: n } : prev);
                  }}
                  className="rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm px-3 py-1.5 w-full focus:outline-none focus:border-[#78a9ff]"
                />
                <span className="text-xs text-carbon-textMuted">{t("flash.zipExport.keepNHint")}</span>
              </label>
            ) : (
              <p className="text-xs text-carbon-textMuted">{t("flash.zipExport.latestNote")}</p>
            )}
          </>
        )}
        <SaveBar
          state={flashZipSaveState}
          error={flashZipSaveError}
          disabled={settings.flashZipExportEnabled && !settings.flashZipExportPath.trim()}
          onSave={() =>
            void save(
              {
                flashZipExportEnabled: settings.flashZipExportEnabled,
                flashZipExportPath: settings.flashZipExportPath,
                flashZipExportKeep: settings.flashZipExportKeep,
              },
              setFlashZipSaveState,
              setFlashZipSaveError
            )
          }
          t={t}
        />
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* OFFSITE — Off-site copy (restic copy replication)                  */}
      {/* Default-mode feature (v4): off-site + ransomware protection is a      */}
      {/* first-class flow, not advanced-only. Deep-linked via /settings#offsite */}
      {/* selects this tab (id kept for back-compat).                          */}
      {/* ------------------------------------------------------------------ */}
      {tab === "offsite" && (
      <div id="offsite">
      <h2 className="text-sm font-semibold text-carbon-textSub uppercase tracking-widest">
        {t("offsite.sectionTitle")}
      </h2>
      <Card title={t("settings.offsiteTitle")}>
        <p className="text-xs text-carbon-textMuted -mt-1">{t("settings.offsiteHint")}</p>
        {([
          ["containersOffsite", "nav.containers", "containers"],
          ["vmsOffsite", "nav.vms", "vms"],
          ["flashOffsite", "nav.flash", "flash"],
        ] as const).map(([repoKey, label, domain]) => {
          const wizardOpen = offsiteWizard === domain;
          return (
          <div key={repoKey} className="flex flex-col gap-1 border-b border-carbon-border pb-3 last:border-0">
            <div className="flex items-center justify-between">
              <span className="text-xs text-carbon-textSub">{t(label)}</span>
              <span className="inline-flex items-center gap-2">
                {settings[repoKey] && !wizardOpen && (
                  <>
                    <TestConnectionButton domain={domain} t={t} />
                    <ReplicateNowButton domain={domain} t={t} />
                  </>
                )}
                <button
                  type="button"
                  onClick={() => setOffsiteWizard(wizardOpen ? null : domain)}
                  className="rounded-lg border border-carbon-border bg-carbon-surface2 px-2.5 py-1 text-xs text-carbon-text hover:bg-carbon-hover"
                >
                  {wizardOpen ? t("offsite.wizard.close") : t("offsite.wizard.setup")}
                </button>
              </span>
            </div>
            {wizardOpen ? (
              <OffsiteWizard
                domain={domain}
                settings={settings}
                setSettings={setSettings}
                save={save}
                t={t}
              />
            ) : (
              <input
                value={settings[repoKey]}
                spellCheck={false}
                onChange={(e) =>
                  setSettings((prev) => (prev ? { ...prev, [repoKey]: e.target.value } : prev))
                }
                placeholder="rest:http://host:8000/repo"
                className="rounded-lg border border-carbon-border bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono focus:outline-none focus:ring-1 focus:ring-accent"
              />
            )}
          </div>
          );
        })}
        <SaveBar
          state={offsiteSaveState}
          error={offsiteSaveError}
          onSave={() =>
            // Repo URLs only — the off-site *cadences* are owned by the Schedules
            // tab now, so this Save no longer writes (or clobbers) them.
            void save(
              {
                containersOffsite: settings.containersOffsite,
                vmsOffsite: settings.vmsOffsite,
                flashOffsite: settings.flashOffsite,
              },
              setOffsiteSaveState,
              setOffsiteSaveError
            )
          }
          t={t}
        />
      </Card>
      </div>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* OFFSITE — Retention (default-mode, paired with off-site above).      */}
      {/* ------------------------------------------------------------------ */}
      {tab === "offsite" && (
      <Card title={t("settings.retentionTitle")}>
        <p className="text-xs text-carbon-textMuted -mt-1">
          {t("settings.retentionHint")}
        </p>
        <span className="text-xs font-medium text-carbon-textSub">{t("settings.retentionLocal")}</span>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          {([
            ["retentionKeepLast", "settings.retentionLast"],
            ["retentionKeepDaily", "settings.retentionDaily"],
            ["retentionKeepWeekly", "settings.retentionWeekly"],
            ["retentionKeepMonthly", "settings.retentionMonthly"],
          ] as const).map(([key, label]) => (
            <label key={key} className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t(label)}</span>
              <input
                type="number"
                min={0}
                value={settings[key]}
                onChange={(e) => {
                  const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                  setSettings((prev) => (prev ? { ...prev, [key]: n } : prev));
                }}
                className="rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm px-3 py-1.5 w-full focus:outline-none focus:border-[#78a9ff]"
              />
            </label>
          ))}
        </div>

        <span className="text-xs font-medium text-carbon-textSub mt-2">{t("settings.retentionOffsite")}</span>
        <p className="text-xs text-carbon-textMuted -mt-1">{t("settings.retentionOffsiteHint")}</p>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          {([
            ["offsiteRetentionKeepLast", "settings.retentionLast"],
            ["offsiteRetentionKeepDaily", "settings.retentionDaily"],
            ["offsiteRetentionKeepWeekly", "settings.retentionWeekly"],
            ["offsiteRetentionKeepMonthly", "settings.retentionMonthly"],
          ] as const).map(([key, label]) => (
            <label key={key} className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t(label)}</span>
              <input
                type="number"
                min={0}
                value={settings[key]}
                onChange={(e) => {
                  const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                  setSettings((prev) => (prev ? { ...prev, [key]: n } : prev));
                }}
                className="rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm px-3 py-1.5 w-full focus:outline-none focus:border-[#78a9ff]"
              />
            </label>
          ))}
        </div>
        <SaveBar
          state={retSaveState}
          error={retSaveError}
          onSave={() =>
            void save(
              {
                retentionKeepLast: settings.retentionKeepLast,
                retentionKeepDaily: settings.retentionKeepDaily,
                retentionKeepWeekly: settings.retentionKeepWeekly,
                retentionKeepMonthly: settings.retentionKeepMonthly,
                offsiteRetentionKeepLast: settings.offsiteRetentionKeepLast,
                offsiteRetentionKeepDaily: settings.offsiteRetentionKeepDaily,
                offsiteRetentionKeepWeekly: settings.offsiteRetentionKeepWeekly,
                offsiteRetentionKeepMonthly: settings.offsiteRetentionKeepMonthly,
              },
              setRetSaveState,
              setRetSaveError
            )
          }
          t={t}
        />
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* OFFSITE — Off-site bandwidth                                        */}
      {/* ------------------------------------------------------------------ */}
      {tab === "offsite" && (
      <Advanced>
      <Card title={t("settings.offsiteLimits")}>
        <p className="text-xs text-carbon-textMuted -mt-1">
          {t("settings.limitHint")}
        </p>
        <div className="grid grid-cols-2 gap-3">
          {([
            ["offsiteLimitUpload", "settings.limitUpload"],
            ["offsiteLimitDownload", "settings.limitDownload"],
          ] as const).map(([key, label]) => (
            <label key={key} className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">{t(label)}</span>
              <input
                type="number"
                min={0}
                value={settings[key]}
                onChange={(e) => {
                  const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                  setSettings((prev) => (prev ? { ...prev, [key]: n } : prev));
                }}
                className="rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm px-3 py-1.5 w-full focus:outline-none focus:border-[#78a9ff]"
              />
            </label>
          ))}
        </div>
        <SaveBar
          state={limSaveState}
          error={limSaveError}
          onSave={() =>
            void save(
              {
                offsiteLimitUpload: settings.offsiteLimitUpload,
                offsiteLimitDownload: settings.offsiteLimitDownload,
              },
              setLimSaveState,
              setLimSaveError
            )
          }
          t={t}
        />
      </Card>
      </Advanced>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM — Monitoring (Prometheus)                                   */}
      {/* ------------------------------------------------------------------ */}
      {tab === "system" && (
      <Advanced>
      <Card title={t("settings.metrics")}>
        <p className="text-xs text-carbon-textMuted -mt-1">{t("settings.metricsHint")}</p>
        <ToggleRow
          label={t("settings.metricsEnable")}
          description="GET /metrics"
          checked={settings.metricsEnabled}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, metricsEnabled: v } : prev)
          }
        />
        <label className="flex flex-col gap-1.5">
          <span className="text-xs text-carbon-textSub">{t("settings.metricsToken")}</span>
          <input
            type="text"
            value={settings.metricsToken}
            spellCheck={false}
            autoComplete="off"
            onChange={(e) =>
              setSettings((prev) => prev ? { ...prev, metricsToken: e.target.value } : prev)
            }
            placeholder=""
            className="rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm font-mono px-3 py-1.5 focus:outline-none focus:border-[#78a9ff]"
          />
        </label>
        <SaveBar
          state={metricsSaveState}
          error={metricsSaveError}
          onSave={() =>
            void save(
              {
                metricsEnabled: settings.metricsEnabled,
                metricsToken: settings.metricsToken,
              },
              setMetricsSaveState,
              setMetricsSaveError
            )
          }
          t={t}
        />
      </Card>
      </Advanced>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE — Encryption                                               */}
      {/* ------------------------------------------------------------------ */}
      {tab === "storage" && (
      <Card title={t("settings.encryption")}>
        <ToggleRow
          label={
            settings.encryptionEnabled
              ? t("settings.encryptionOn")
              : t("settings.encryptionOff")
          }
          checked={settings.encryptionEnabled}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, encryptionEnabled: v } : prev)
          }
        />
        <div className="rounded-lg bg-[#2a2a1c] border border-[#4a4a2a] px-3 py-2.5 text-xs text-[#f1c21b] leading-relaxed">
          {t("settings.encryptionWarning")}
        </div>
        {settings.encryptionEnabled && (
          <div className="flex flex-col gap-2 border-t border-carbon-border pt-4">
            <h3 className="text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
              {t("recovery.title")}
            </h3>
            <p className="text-xs text-carbon-textMuted leading-relaxed">
              {t("recovery.why")}
            </p>
            <a
              href={recoveryKitUrl()}
              download="bombvault-recovery-kit.md"
              className="self-start rounded-md bg-carbon-surface3 hover:bg-carbon-border px-3 py-1.5 text-sm text-carbon-text transition-colors"
            >
              {t("recovery.download")}
            </a>
          </div>
        )}
        <SaveBar
          state={encSaveState}
          error={encSaveError}
          onSave={() =>
            void save(
              { encryptionEnabled: settings.encryptionEnabled },
              setEncSaveState,
              setEncSaveError
            )
          }
          t={t}
        />
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM — VM Backup over SSH                                        */}
      {/* Advanced, OR shown whenever VMs are enabled so the SSH setup you    */}
      {/* need to make VM backups work is never hidden behind Advanced.       */}
      {/* ------------------------------------------------------------------ */}
      {tab === "system" && (advanced || settings.vmsEnabled) && <VMSSHCard t={t} />}

      {/* ------------------------------------------------------------------ */}
      {/* OFFSITE — Off-site backends (rclone + cloud credentials)            */}
      {/* ------------------------------------------------------------------ */}
      {tab === "offsite" && <Advanced><RcloneCard t={t} /></Advanced>}

      {tab === "offsite" && <Advanced><CloudCard t={t} /></Advanced>}

      {/* ------------------------------------------------------------------ */}
      {/* NOTIFICATIONS — NotifyCard (renders always; not re-gated).          */}
      {/* ------------------------------------------------------------------ */}
      {tab === "notifications" && <NotifyCard t={t} />}

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM — Spike (host-integration check; KEEP — it is LIVE).         */}
      {/* ------------------------------------------------------------------ */}
      {tab === "system" && (
      <Advanced>
        <Card title={t("spike.title")}>
          <SpikePanel t={t} />
        </Card>
      </Advanced>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* INTEGRITY — Integrity, maintenance & restore drills                 */}
      {/* Default-visible (v4): manual restore drills — including the real     */}
      {/* off-site DR restore — are part of the core ransomware-protection     */}
      {/* flow, alongside the un-gated off-site + retention cards above.       */}
      {/* ------------------------------------------------------------------ */}
      {tab === "integrity" && (
      <IntegrityCard t={t} settings={settings} setSettings={setSettings} save={save} />
      )}

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM — Security                                                  */}
      {/* ------------------------------------------------------------------ */}
      {tab === "system" && (
      <Card title={t("auth.security")}>
        {/* Status badge */}
        <div className="flex items-center gap-2">
          <span
            className={`inline-block h-2 w-2 rounded-full ${authEnabled ? "bg-[#6fdc8c]" : "bg-carbon-textMuted"}`}
          />
          <span className="text-sm text-carbon-text">
            {authEnabled ? t("auth.authOn") : t("auth.authOff")}
          </span>
        </div>

        {/* Password hint */}
        <p className="text-xs text-carbon-textMuted leading-relaxed">
          {t("auth.passwordHint")}
        </p>

        {/* Set / Change password form */}
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-carbon-textSub">
              {authEnabled ? t("auth.changePassword") : t("auth.setPassword")}
            </label>
            <input
              type="password"
              value={pwNew}
              onChange={(e) => setPwNew(e.target.value)}
              autoComplete="new-password"
              placeholder="••••••••"
              className="rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm px-3 py-1.5 focus:outline-none focus:border-[#78a9ff]"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-carbon-textSub">
              {t("auth.confirmPassword")}
            </label>
            <input
              type="password"
              value={pwConfirm}
              onChange={(e) => setPwConfirm(e.target.value)}
              autoComplete="new-password"
              placeholder="••••••••"
              className="rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm px-3 py-1.5 focus:outline-none focus:border-[#78a9ff]"
            />
          </div>

          {/* Save / status row */}
          <div className="flex items-center gap-3 pt-1">
            <button
              onClick={() => void handleSetPassword()}
              disabled={pwSaveState === "saving"}
              className="inline-flex items-center gap-2 rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
            >
              {pwSaveState === "saving" ? (
                <>
                  <span
                    className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin"
                    style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
                  />
                  {t("auth.saving")}
                </>
              ) : (
                t("settings.save")
              )}
            </button>
            {pwSaveState === "saved" && pwSaveMsg && (
              <span className="text-sm text-[#6fdc8c]">{pwSaveMsg}</span>
            )}
            {pwSaveState === "error" && pwSaveMsg && (
              <span className="text-sm text-[#ff8389]">{pwSaveMsg}</span>
            )}
          </div>
        </div>

        {/* Logout button — only shown when currently signed in */}
        {authEnabled && authAuthed && (
          <div className="pt-2 border-t border-carbon-border">
            <button
              onClick={() => void handleLogout()}
              className="rounded-lg bg-carbon-surface2 border border-carbon-border px-4 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover transition-colors"
            >
              {t("auth.logout")}
            </button>
          </div>
        )}
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* GENERAL — Appearance                                               */}
      {/* ------------------------------------------------------------------ */}
      {tab === "general" && (
      <Card title={t("settings.appearance")}>
        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <span className="text-sm text-carbon-text">{t("settings.accentColor")}</span>
            <div className="flex items-center gap-3 flex-wrap">
              {/* Native color picker */}
              <input
                type="color"
                value={accentHex}
                onChange={(e) => {
                  setAccentHex(e.target.value);
                  setAccent(e.target.value);
                }}
                className="h-8 w-14 cursor-pointer rounded border border-carbon-border bg-carbon-surface2 p-0.5"
                title={t("settings.accentColor")}
              />
              {/* Preset swatches */}
              <div className="flex items-center gap-2 flex-wrap">
                <span className="text-xs text-carbon-textMuted">{t("settings.accentPresets")}:</span>
                {ACCENT_PRESETS.map((p) => (
                  <button
                    key={p.hex}
                    title={p.label}
                    onClick={() => {
                      setAccentHex(p.hex);
                      setAccent(p.hex);
                    }}
                    className="w-6 h-6 rounded-full border-2 transition-transform hover:scale-110"
                    style={{
                      backgroundColor: p.hex,
                      borderColor: accentHex.toLowerCase() === p.hex.toLowerCase()
                        ? "var(--carbon-text)"
                        : "var(--carbon-border)",
                    }}
                  />
                ))}
                {/* Reset to default */}
                {accentHex.toLowerCase() !== DEFAULT_ACCENT.toLowerCase() && (
                  <button
                    onClick={() => {
                      setAccentHex(DEFAULT_ACCENT);
                      setAccent(DEFAULT_ACCENT);
                    }}
                    className="text-xs text-carbon-textMuted hover:text-carbon-text transition-colors ml-1"
                  >
                    {t("common.reset")}
                  </button>
                )}
              </div>
            </div>
          </div>
        </div>
      </Card>
      )}

      {/* SYSTEM — Version + report-a-bug (kept out of the sidebar for a clean UI). */}
      {tab === "system" && <AboutFooter />}
    </div>
  );
}
