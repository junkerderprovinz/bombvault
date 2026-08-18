import { useEffect, useRef, useState } from "react";
import { getSettings, putSettings, getAuth, setAuthPassword, logout, logoutAll, getVMSSH, testVMSSH, getRclone, setRclone, getCloud, setCloud, getCloudCredSets, setCloudCredSets, checkDomain, unlockDomain, pruneDomain, replicateOffsite, testOffsite, tamperTest, getStatus, getNotify, setNotify, testNotify, runDrill, getDrills, listContainers, listVMs, setScheduleCadence, setVMScheduleCadence, listFileSets, patchFileSet, downloadRecoveryKit, exportSettings, importSettingsPreview, importSettingsApply, getHealth, generateWidgetToken, disableWidgetToken, generateFleetToken, disableFleetToken, getDashboardPlugin, installDashboardPlugin, removeDashboardPlugin } from "../lib/api";
import type { CloudCredSet, CloudCredSetInfo } from "../lib/api";
import { SourceToggle, isOffsiteSource, type RepoSource } from "../components/SourceToggle";
import { useOffsiteTargets } from "../lib/useOffsiteTargets";
import { FolderBrowser } from "../components/FolderBrowser";
import { OffsiteWizard } from "../components/OffsiteWizard";
import { InfoBubble } from "../components/InfoBubble";
import { OffsiteTargetsSection } from "../components/OffsiteTargetsSection";
import { CadenceBuilder } from "../components/CadenceBuilder";
import { ItemScheduleOverride } from "../components/ItemScheduleOverride";
import type { Settings, NotifyConfig, RestoreDrill, Container, VM, FileSetView, RegistryAuthEntry, ImportSettingsSummary } from "../lib/api";
import { useT, type TranslationKey } from "../lib/i18n";
import { copyText } from "../lib/clipboard";
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
    <div className="bg-carbon-surface rounded-card p-5 flex flex-col gap-4">
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
        className={`relative inline-flex h-5 w-9 shrink-0 mt-0.5 items-center rounded-full transition-colors focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-statusInfoSolid disabled:opacity-50 ${
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
        <span className="text-sm text-statusOk">{t("settings.saved")}</span>
      )}
      {state === "error" && error && (
        <span className="text-sm text-statusFail">{error}</span>
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
    // copyText falls back to execCommand in non-secure contexts (#112).
    if (await copyText(pub)) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }

  async function handleCopyCmd() {
    if (await copyText(authorizeCmd)) {
      setCmdCopied(true);
      setTimeout(() => setCmdCopied(false), 2000);
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
            <code className="flex-1 break-all rounded-sm bg-carbon-surface2 p-2 text-xs text-carbon-text">
              {pub || "—"}
            </code>
            <button
              onClick={handleCopy}
              disabled={!pub}
              className="shrink-0 rounded-sm bg-accent px-3 py-2 text-xs font-medium text-accentContrast disabled:opacity-50"
            >
              {copied ? t("vm.ssh.copied") : t("vm.ssh.copy")}
            </button>
          </div>
        </div>

        {/* One-time setup instructions */}
        <div className="rounded-lg bg-carbon-surface2 p-3 flex flex-col gap-2">
          <span className="text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
            {t("vm.ssh.setupTitle")}
          </span>
          <ol className="list-decimal pl-5 text-xs text-carbon-textSub flex flex-col gap-1">
            <li>{t("vm.ssh.step1")}</li>
            <li>{t("vm.ssh.step2")}</li>
            <li>{t("vm.ssh.step3")}</li>
          </ol>
          <div className="flex items-start gap-2">
            <pre className="flex-1 overflow-x-auto rounded-sm bg-carbon-background p-2 text-[11px] leading-snug text-carbon-text whitespace-pre">{authorizeCmd || "—"}</pre>
            <button
              onClick={handleCopyCmd}
              disabled={!pub}
              className="shrink-0 rounded-sm bg-carbon-surface3 px-3 py-2 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
            >
              {cmdCopied ? t("vm.ssh.copied") : t("vm.ssh.copyCmd")}
            </button>
          </div>
          <a
            href="https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md"
            target="_blank"
            rel="noreferrer"
            className="text-xs text-statusInfo hover:underline"
          >
            {t("vm.ssh.guide")} →
          </a>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={handleTest}
            disabled={testState === "testing"}
            className="rounded-sm bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
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

// ---------------------------------------------------------------------------
// SettingsPortabilityCard — export this instance's configuration to a JSON file,
// or import a previously exported file. Self-contained: it moves only settings +
// off-site destinations (and, opt-in, the decrypted credentials). Backups,
// snapshots and history are never touched. Import always previews first and asks
// for confirmation before it replaces anything.
// ---------------------------------------------------------------------------

// The machine ids the import summary returns for populated setting areas, mapped
// to their translation keys so the preview lists them human-readably. An unknown
// (future) id falls back to its raw value.
const IMPORT_GROUP_KEYS: Record<string, TranslationKey> = {
  domains: "settingsIO.group.domains",
  schedules: "settingsIO.group.schedules",
  retention: "settingsIO.group.retention",
  offsite: "settingsIO.group.offsite",
  drills: "settingsIO.group.drills",
  digest: "settingsIO.group.digest",
  monitoring: "settingsIO.group.monitoring",
  language: "settingsIO.group.language",
  exportEncryption: "settingsIO.group.exportEncryption",
};

function SettingsPortabilityCard({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const [includeCreds, setIncludeCreds] = useState(true);
  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);

  // Import is a two-step flow: pick a file → preview summary + confirm → apply.
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const [importBusy, setImportBusy] = useState<"idle" | "reading" | "applying">("idle");
  const [importError, setImportError] = useState<string | null>(null);
  const [importDone, setImportDone] = useState(false);
  // The parsed preview and the raw file text held for the confirmed apply.
  const [preview, setPreview] = useState<ImportSettingsSummary | null>(null);
  const [pendingText, setPendingText] = useState<string | null>(null);

  function resetImport() {
    setPreview(null);
    setPendingText(null);
    setImportError(null);
    setImportDone(false);
    if (fileInputRef.current) fileInputRef.current.value = "";
  }

  async function handleExport() {
    setExportError(null);
    setExporting(true);
    // Backend-provided error text (if any) is shown verbatim BY DESIGN — the API
    // answers English and is not translated client-side.
    const err = await exportSettings(includeCreds);
    setExportError(err);
    setExporting(false);
  }

  async function handleFilePicked(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    setImportError(null);
    setImportDone(false);
    setPreview(null);
    setImportBusy("reading");
    try {
      const text = await file.text();
      const res = await importSettingsPreview(text);
      if (res.ok && res.summary) {
        setPendingText(text);
        setPreview(res.summary);
      } else {
        setImportError(res.error ?? t("settingsIO.importFailed"));
      }
    } catch (err) {
      setImportError(err instanceof Error ? err.message : String(err));
    } finally {
      setImportBusy("idle");
    }
  }

  async function handleConfirmImport() {
    if (!pendingText) return;
    setImportError(null);
    setImportBusy("applying");
    try {
      const res = await importSettingsApply(pendingText);
      if (res.ok) {
        setImportDone(true);
        setPreview(null);
        setPendingText(null);
        if (fileInputRef.current) fileInputRef.current.value = "";
      } else {
        setImportError(res.error ?? t("settingsIO.importFailed"));
      }
    } catch (err) {
      setImportError(err instanceof Error ? err.message : String(err));
    } finally {
      setImportBusy("idle");
    }
  }

  const busy = importBusy !== "idle" || exporting;

  return (
    <Card title={t("settingsIO.title")}>
      <p className="text-sm text-carbon-textSub leading-relaxed -mt-1">
        {t("settingsIO.desc")}
      </p>

      {/* EXPORT ---------------------------------------------------------- */}
      <div className="flex flex-col gap-3 border-t border-carbon-border pt-4">
        <h3 className="text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
          {t("settingsIO.exportHeading")}
        </h3>
        <label className="flex items-start gap-2.5 cursor-pointer">
          <input
            type="checkbox"
            checked={includeCreds}
            onChange={(e) => setIncludeCreds(e.target.checked)}
            className="mt-0.5 h-4 w-4 shrink-0 accent-accent"
          />
          <span className="text-sm text-carbon-text">{t("settingsIO.includeCreds")}</span>
        </label>
        {includeCreds && (
          <div className="rounded-lg bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
            {t("settingsIO.credsWarning")}
          </div>
        )}
        <button
          type="button"
          onClick={() => void handleExport()}
          disabled={busy}
          className="self-start rounded-md bg-carbon-surface3 hover:bg-carbon-border px-3 py-1.5 text-sm text-carbon-text transition-colors disabled:opacity-50"
        >
          {exporting ? t("settingsIO.exporting") : t("settingsIO.exportButton")}
        </button>
        {exportError && (
          // Backend error text shown verbatim BY DESIGN (English, not translated).
          <span className="text-xs text-statusFail wrap-break-word">✗ {exportError}</span>
        )}
      </div>

      {/* IMPORT ---------------------------------------------------------- */}
      <div className="flex flex-col gap-3 border-t border-carbon-border pt-4">
        <h3 className="text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
          {t("settingsIO.importHeading")}
        </h3>
        <p className="text-xs text-carbon-textMuted leading-relaxed">
          {t("settingsIO.importHint")}
        </p>

        <input
          ref={fileInputRef}
          type="file"
          accept="application/json,.json"
          onChange={(e) => void handleFilePicked(e)}
          className="hidden"
        />
        <button
          type="button"
          onClick={() => fileInputRef.current?.click()}
          disabled={busy}
          className="self-start rounded-md bg-carbon-surface3 hover:bg-carbon-border px-3 py-1.5 text-sm text-carbon-text transition-colors disabled:opacity-50"
        >
          {importBusy === "reading" ? t("settingsIO.reading") : t("settingsIO.chooseFile")}
        </button>

        {/* Preview + confirmation before anything is written. */}
        {preview && (
          <div className="rounded-lg bg-carbon-surface2 p-4 flex flex-col gap-3">
            <span className="text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
              {t("settingsIO.previewTitle")}
            </span>
            <dl className="flex flex-col gap-1.5 text-xs text-carbon-text">
              <div className="flex justify-between gap-3">
                <dt className="text-carbon-textMuted">{t("settingsIO.previewExportedAt")}</dt>
                <dd className="font-mono text-right wrap-break-word">{preview.exportedAt || "-"}</dd>
              </div>
              <div className="flex justify-between gap-3">
                <dt className="text-carbon-textMuted">{t("settingsIO.previewAppVersion")}</dt>
                <dd className="font-mono text-right wrap-break-word">{preview.appVersion || "-"}</dd>
              </div>
              <div className="flex justify-between gap-3">
                <dt className="text-carbon-textMuted">{t("settingsIO.previewOffsiteTargets")}</dt>
                <dd className="font-mono">{preview.offsiteTargets}</dd>
              </div>
              <div className="flex justify-between gap-3">
                <dt className="text-carbon-textMuted">{t("settingsIO.previewCredentials")}</dt>
                <dd className="text-right">
                  {preview.credentials.present
                    ? t("settingsIO.previewCredsIncluded")
                    : t("settingsIO.previewCredsNotIncluded")}
                </dd>
              </div>
              <div className="flex justify-between gap-3">
                <dt className="text-carbon-textMuted shrink-0">{t("settingsIO.previewSettingsAreas")}</dt>
                <dd className="text-right wrap-break-word">
                  {preview.settingsGroups.length > 0
                    ? preview.settingsGroups
                        .map((g) => (IMPORT_GROUP_KEYS[g] ? t(IMPORT_GROUP_KEYS[g]) : g))
                        .join(", ")
                    : t("settingsIO.previewNone")}
                </dd>
              </div>
            </dl>
            <div className="rounded-md bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
              {t("settingsIO.confirmWarning")}
            </div>
            <div className="flex items-center gap-3">
              <button
                type="button"
                onClick={() => void handleConfirmImport()}
                disabled={busy}
                className="rounded-md bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
              >
                {importBusy === "applying" ? t("settingsIO.importing") : t("settingsIO.confirmButton")}
              </button>
              <button
                type="button"
                onClick={resetImport}
                disabled={busy}
                className="rounded-md bg-carbon-surface3 hover:bg-carbon-border px-4 py-1.5 text-sm text-carbon-text transition-colors disabled:opacity-50"
              >
                {t("settingsIO.cancel")}
              </button>
            </div>
          </div>
        )}

        {importDone && (
          <span className="text-xs text-statusOk">✓ {t("settingsIO.importSuccess")}</span>
        )}
        {importError && (
          // Backend error text shown verbatim BY DESIGN (English, not translated).
          <span className="text-xs text-statusFail wrap-break-word">✗ {importError}</span>
        )}
      </div>
    </Card>
  );
}

// The companion dashboard-tile plugin's .plg URL + repo — shown for manual
// install when SSH is missing, and linked for transparency before installing.
// (Install itself uses a hard-coded server-side constant; these are display-only.)
const DASH_PLUGIN_PLG_URL =
  "https://raw.githubusercontent.com/junkerderprovinz/bombvault-widget/main/plugin/bombvaultwidget.plg";
const DASH_PLUGIN_REPO_URL = "https://github.com/junkerderprovinz/bombvault-widget";

type DashPluginStatus =
  | { kind: "loading" }
  | { kind: "noSsh" }
  | { kind: "absent" }
  | { kind: "installed"; version: string }
  | { kind: "error"; message: string; output?: string };

// UnraidTileSection — the "Unraid dashboard tile" block inside the Dashboard
// widget card: one-click install/remove of the companion bombvaultwidget plugin
// over the existing host SSH connection. Without SSH it degrades to manual
// instructions (the copyable .plg URL + a CA hint).
function UnraidTileSection({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const [status, setStatus] = useState<DashPluginStatus>({ kind: "loading" });
  const [busy, setBusy] = useState<"idle" | "install" | "remove">("idle");
  const [installOk, setInstallOk] = useState(false);
  const [urlCopied, setUrlCopied] = useState(false);

  function refresh() {
    getDashboardPlugin()
      .then((r) => {
        if (!r.ok) {
          setStatus({ kind: "error", message: r.error ?? t("settings.error") });
        } else if (!r.sshConfigured) {
          setStatus({ kind: "noSsh" });
        } else if (r.installed) {
          setStatus({ kind: "installed", version: r.version ?? "" });
        } else {
          setStatus({ kind: "absent" });
        }
      })
      .catch((err) => {
        setStatus({
          kind: "error",
          message: err instanceof Error ? err.message : t("settings.error"),
        });
      });
  }

  useEffect(refresh, []); // eslint-disable-line react-hooks/exhaustive-deps -- status check on card mount only

  async function run(op: "install" | "remove") {
    setBusy(op);
    setInstallOk(false);
    try {
      const r = await (op === "install" ? installDashboardPlugin() : removeDashboardPlugin());
      if (r.ok) {
        if (op === "install") setInstallOk(true);
        refresh();
      } else {
        setStatus({
          kind: "error",
          message: r.error ?? t("settings.error"),
          output: r.output,
        });
      }
    } catch (err) {
      setStatus({
        kind: "error",
        message: err instanceof Error ? err.message : t("settings.error"),
      });
    } finally {
      setBusy("idle");
    }
  }

  async function handleCopyUrl() {
    if (await copyText(DASH_PLUGIN_PLG_URL)) {
      setUrlCopied(true);
      setTimeout(() => setUrlCopied(false), 2000);
    }
  }

  return (
    <div className="flex flex-col gap-3 border-t border-carbon-border pt-4">
      <h3 className="text-xs font-semibold text-carbon-textSub uppercase tracking-widest">
        {t("settings.dashTile")}
      </h3>
      <p className="text-xs text-carbon-textMuted">{t("settings.dashTileHint")}</p>

      {status.kind === "loading" && (
        <span className="text-xs text-carbon-textMuted">{t("settings.dashTileChecking")}</span>
      )}

      {status.kind === "noSsh" && (
        <div className="flex flex-col gap-2">
          <p className="text-xs text-carbon-textSub">{t("settings.dashTileNoSsh")}</p>
          <div className="flex items-start gap-2">
            <code className="flex-1 break-all rounded-sm bg-carbon-surface2 p-2 text-xs text-carbon-text">
              {DASH_PLUGIN_PLG_URL}
            </code>
            <button
              type="button"
              onClick={() => void handleCopyUrl()}
              className="shrink-0 rounded-sm bg-accent px-3 py-2 text-xs font-medium text-accentContrast"
            >
              {urlCopied ? t("vm.ssh.copied") : t("vm.ssh.copy")}
            </button>
          </div>
          <p className="text-xs text-carbon-textMuted">{t("settings.dashTileCa")}</p>
        </div>
      )}

      {status.kind === "absent" && (
        <div className="flex flex-col gap-2">
          <span className="text-sm text-carbon-text">{t("settings.dashTileNotInstalled")}</span>
          {/* Transparency BEFORE the call: what Install does, and where the code lives. */}
          <p className="text-xs text-carbon-textMuted">{t("settings.dashTileConfirm")}</p>
          <a
            href={DASH_PLUGIN_REPO_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="text-xs text-statusInfo hover:underline self-start"
          >
            {t("settings.dashTileRepo")} →
          </a>
          <button
            type="button"
            onClick={() => void run("install")}
            disabled={busy !== "idle"}
            className="self-start rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
          >
            {busy === "install" ? t("settings.dashTileInstalling") : t("settings.dashTileInstall")}
          </button>
        </div>
      )}

      {status.kind === "installed" && (
        <div className="flex flex-col gap-2">
          <span className="text-sm text-statusOk">
            ✓{" "}
            {status.version
              ? t("settings.dashTileInstalled").replace("{version}", status.version)
              : t("settings.dashTileInstalledNoV")}
          </span>
          {installOk && (
            <span className="text-xs text-statusOk">{t("settings.dashTileInstallOk")}</span>
          )}
          <p className="text-xs text-carbon-textMuted">{t("settings.dashTileInstalledHint")}</p>
          <button
            type="button"
            onClick={() => void run("remove")}
            disabled={busy !== "idle"}
            className="self-start rounded-sm bg-carbon-surface3 px-3 py-2 text-xs text-statusFail hover:bg-carbon-hover disabled:opacity-50"
          >
            {busy === "remove" ? t("settings.dashTileRemoving") : t("settings.dashTileRemove")}
          </button>
        </div>
      )}

      {status.kind === "error" && (
        <div className="flex flex-col gap-2">
          <span className="text-xs text-statusFail wrap-break-word">✗ {status.message}</span>
          {status.output && (
            <pre className="overflow-x-auto rounded-sm bg-carbon-background p-2 text-[11px] leading-snug text-carbon-text whitespace-pre-wrap">
              {status.output}
            </pre>
          )}
          <button
            type="button"
            onClick={() => {
              setStatus({ kind: "loading" });
              refresh();
            }}
            className="self-start rounded-sm bg-carbon-surface3 px-3 py-2 text-xs text-carbon-text hover:bg-carbon-hover"
          >
            {t("whatsnew.retry")}
          </button>
        </div>
      )}
    </div>
  );
}

// DashboardWidgetCard manages the embeddable activity-log widget (GET /widget):
// generate/rotate/disable its access token, show the copyable widget URL and a
// live iframe preview. The token is a show-once secret — the server stores it
// but never echoes it back (settings GET only reports widgetTokenSet), so the
// URL + preview render only right after generating; after a reload the card
// shows the kept-placeholder until the user regenerates.
function DashboardWidgetCard({
  t,
  tokenSet,
  onTokenSet,
}: {
  t: ReturnType<typeof useT>["t"];
  tokenSet: boolean;
  onTokenSet: (set: boolean) => void;
}) {
  const [token, setToken] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const widgetUrl = token ? `${window.location.origin}/widget?token=${token}` : null;

  async function handleGenerate() {
    setBusy(true);
    setError(null);
    try {
      const r = await generateWidgetToken();
      if (r.ok && r.token) {
        setToken(r.token);
        onTokenSet(true);
      } else {
        setError(r.error ?? t("settings.error"));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("settings.error"));
    } finally {
      setBusy(false);
    }
  }

  async function handleDisable() {
    setBusy(true);
    setError(null);
    try {
      const r = await disableWidgetToken();
      if (r.ok) {
        setToken(null);
        onTokenSet(false);
      } else {
        setError(r.error ?? t("settings.error"));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("settings.error"));
    } finally {
      setBusy(false);
    }
  }

  async function handleCopy() {
    if (!widgetUrl) return;
    if (await copyText(widgetUrl)) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }

  return (
    <Card title={t("settings.widget")}>
      <p className="text-xs text-carbon-textMuted -mt-1">{t("settings.widgetHint")}</p>

      <ul className="list-disc pl-5 text-xs text-carbon-textSub flex flex-col gap-1">
        <li>{t("settings.widgetHow")}</li>
        <li>{t("settings.widgetAccess")}</li>
        <li>{t("settings.widgetEnglish")}</li>
      </ul>

      {tokenSet ? (
        <div className="flex flex-col gap-1.5">
          <span className="text-xs text-carbon-textSub">{t("settings.widgetToken")}</span>
          <div className="flex items-center gap-2">
            {/* Show-once secret: value is only the freshly generated token; a
                stored-but-unknown one renders the cloud.secretSet placeholder. */}
            <input
              type="password"
              readOnly
              value={token ?? ""}
              placeholder={token ? "" : t("cloud.secretSet")}
              className="flex-1 min-w-0 rounded-lg bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
            />
            <button
              type="button"
              onClick={() => void handleGenerate()}
              disabled={busy}
              className="shrink-0 rounded-sm bg-carbon-surface3 px-3 py-2 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
            >
              {t("settings.widgetRegenerate")}
            </button>
            <button
              type="button"
              onClick={() => void handleDisable()}
              disabled={busy}
              className="shrink-0 rounded-sm bg-carbon-surface3 px-3 py-2 text-xs text-statusFail hover:bg-carbon-hover disabled:opacity-50"
            >
              {t("settings.widgetDisable")}
            </button>
          </div>
        </div>
      ) : (
        <button
          type="button"
          onClick={() => void handleGenerate()}
          disabled={busy}
          className="self-start rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
        >
          {t("settings.widgetGenerate")}
        </button>
      )}
      {error && <span className="text-xs text-statusFail wrap-break-word">✗ {error}</span>}

      {tokenSet && !token && (
        <p className="text-xs text-carbon-textMuted">{t("settings.widgetUrlOnce")}</p>
      )}
      {widgetUrl && (
        <>
          <div className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("settings.widgetUrl")}</span>
            <div className="flex items-start gap-2">
              <code className="flex-1 break-all rounded-sm bg-carbon-surface2 p-2 text-xs text-carbon-text">
                {widgetUrl}
              </code>
              <button
                type="button"
                onClick={() => void handleCopy()}
                className="shrink-0 rounded-sm bg-accent px-3 py-2 text-xs font-medium text-accentContrast"
              >
                {copied ? t("vm.ssh.copied") : t("vm.ssh.copy")}
              </button>
            </div>
          </div>
          <div className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("settings.widgetPreview")}</span>
            <iframe
              src={widgetUrl}
              title={t("settings.widgetPreview")}
              className="w-full max-w-[560px] h-[300px] rounded-card bg-carbon-surface2"
            />
          </div>
        </>
      )}

      {/* Companion Unraid dashboard-tile plugin (one-click install over SSH). */}
      <UnraidTileSection t={t} />
    </Card>
  );
}

// FleetSettingsCard manages this instance's own identity for the Fleet view:
// the display name reported to polling peers, and the peer status token (GET
// /api/fleet/status) that authorizes OTHER instances to poll THIS one. The
// token follows the exact same show-once secret contract as the widget token
// (generate/rotate/disable, never echoed back after the fact).
function FleetSettingsCard({
  t,
  settings,
  setSettings,
  save,
  tokenSet,
  onTokenSet,
}: {
  t: ReturnType<typeof useT>["t"];
  settings: Settings;
  setSettings: React.Dispatch<React.SetStateAction<Settings | null>>;
  save: (
    patch: Partial<Settings>,
    setSaveState: (s: SaveState) => void,
    setSaveError: (e: string | null) => void
  ) => Promise<boolean>;
  tokenSet: boolean;
  onTokenSet: (set: boolean) => void;
}) {
  const [nameSaveState, setNameSaveState] = useState<SaveState>("idle");
  const [nameSaveError, setNameSaveError] = useState<string | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  async function handleGenerate() {
    setBusy(true);
    setError(null);
    try {
      const r = await generateFleetToken();
      if (r.ok && r.token) {
        setToken(r.token);
        onTokenSet(true);
      } else {
        setError(r.error ?? t("settings.error"));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("settings.error"));
    } finally {
      setBusy(false);
    }
  }

  async function handleDisable() {
    setBusy(true);
    setError(null);
    try {
      const r = await disableFleetToken();
      if (r.ok) {
        setToken(null);
        onTokenSet(false);
      } else {
        setError(r.error ?? t("settings.error"));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("settings.error"));
    } finally {
      setBusy(false);
    }
  }

  async function handleCopy() {
    if (!token) return;
    if (await copyText(token)) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }

  return (
    <Card title={t("settings.fleet")}>
      <p className="text-xs text-carbon-textMuted -mt-1">{t("settings.fleetHint")}</p>

      <div className="flex flex-col gap-1.5">
        <label className="text-xs text-carbon-textSub">{t("settings.instanceName")}</label>
        <div className="flex items-center gap-2">
          <input
            type="text"
            value={settings.instanceName}
            onChange={(e) => setSettings((prev) => (prev ? { ...prev, instanceName: e.target.value } : prev))}
            spellCheck={false}
            autoComplete="off"
            placeholder="tower"
            className="flex-1 min-w-0 rounded-lg bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
          />
        </div>
        <SaveBar
          state={nameSaveState}
          error={nameSaveError}
          onSave={() => void save({ instanceName: settings.instanceName }, setNameSaveState, setNameSaveError)}
          t={t}
        />
      </div>

      <ul className="list-disc pl-5 text-xs text-carbon-textSub flex flex-col gap-1">
        <li>{t("settings.fleetHow")}</li>
        <li>{t("settings.fleetAccess")}</li>
      </ul>

      {tokenSet ? (
        <div className="flex flex-col gap-1.5">
          <span className="text-xs text-carbon-textSub">{t("settings.fleetToken")}</span>
          <div className="flex items-center gap-2">
            <input
              type="password"
              readOnly
              value={token ?? ""}
              placeholder={token ? "" : t("cloud.secretSet")}
              className="flex-1 min-w-0 rounded-lg bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
            />
            <button
              type="button"
              onClick={() => void handleGenerate()}
              disabled={busy}
              className="shrink-0 rounded-sm bg-carbon-surface3 px-3 py-2 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
            >
              {t("settings.fleetRegenerate")}
            </button>
            <button
              type="button"
              onClick={() => void handleDisable()}
              disabled={busy}
              className="shrink-0 rounded-sm bg-carbon-surface3 px-3 py-2 text-xs text-statusFail hover:bg-carbon-hover disabled:opacity-50"
            >
              {t("settings.fleetDisable")}
            </button>
          </div>
        </div>
      ) : (
        <button
          type="button"
          onClick={() => void handleGenerate()}
          disabled={busy}
          className="self-start rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
        >
          {t("settings.fleetGenerate")}
        </button>
      )}
      {error && <span className="text-xs text-statusFail wrap-break-word">✗ {error}</span>}

      {tokenSet && !token && (
        <p className="text-xs text-carbon-textMuted">{t("settings.fleetTokenOnce")}</p>
      )}
      {token && (
        <div className="flex flex-col gap-1">
          <span className="text-xs text-carbon-textSub">{t("settings.fleetTokenPasteHint")}</span>
          <div className="flex items-start gap-2">
            <code className="flex-1 break-all rounded-sm bg-carbon-surface2 p-2 text-xs text-carbon-text">
              {token}
            </code>
            <button
              type="button"
              onClick={() => void handleCopy()}
              className="shrink-0 rounded-sm bg-accent px-3 py-2 text-xs font-medium text-accentContrast"
            >
              {copied ? t("vm.ssh.copied") : t("vm.ssh.copy")}
            </button>
          </div>
          <p className="text-[11px] text-carbon-textMuted">
            {t("settings.fleetUrlHint").replace("{url}", window.location.origin)}
          </p>
        </div>
      )}
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
        className="rounded-lg bg-carbon-surface2 text-carbon-text text-xs font-mono px-3 py-2 focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
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
        {state === "saved" && <span className="text-sm text-statusOk">{t("settings.saved")}</span>}
        {state === "error" && msg && <span className="text-sm text-statusFail">{msg}</span>}
      </div>
    </Card>
  );
}

// CloudCard stores credentials for off-site restic backends (S3 + restic REST),
// kept encrypted. Secrets are write-only: blank on load, blank-on-save keeps the
// stored value. Field labels are restic's actual env var names (self-documenting).
export function CloudCard({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const [c, setC] = useState({ s3KeyId: "", s3Secret: "", s3Region: "", restUser: "", restPassword: "", s3StorageClass: "" });
  const [secretSet, setSecretSet] = useState(false);
  const [pwSet, setPwSet] = useState(false);
  const [state, setState] = useState<SaveState>("idle");
  const [msg, setMsg] = useState<string | null>(null);

  function refresh() {
    getCloud()
      .then((r) => {
        if (r.ok) {
          setC((p) => ({ ...p, s3KeyId: r.s3KeyId ?? "", s3Region: r.s3Region ?? "", restUser: r.restUser ?? "", s3StorageClass: r.s3StorageClass ?? "" }));
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
    "rounded-lg bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid";
  const fieldCls = "flex flex-col gap-1 text-xs font-mono text-carbon-textSub";

  return (
    <Card title={t("cloud.title")}>
      <p className="text-xs text-carbon-textMuted -mt-1">{t("cloud.hint")}</p>

      <div className="flex flex-col gap-2 rounded-lg bg-carbon-surface2 p-3">
        <span className="text-xs font-semibold text-carbon-textSub">Amazon S3</span>
        <label className={fieldCls}>AWS_ACCESS_KEY_ID
          <input value={c.s3KeyId} onChange={(e) => set("s3KeyId", e.target.value)} spellCheck={false} className={inputCls} /></label>
        <label className={fieldCls}>AWS_SECRET_ACCESS_KEY
          <input type="password" value={c.s3Secret} onChange={(e) => set("s3Secret", e.target.value)} spellCheck={false}
            placeholder={secretSet ? t("cloud.secretSet") : ""} className={inputCls} /></label>
        <label className={fieldCls}>AWS_DEFAULT_REGION
          <input value={c.s3Region} onChange={(e) => set("s3Region", e.target.value)} spellCheck={false} placeholder="us-east-1" className={inputCls} /></label>
        <label className={fieldCls}>{t("cloud.storageClass.label")}
          <select value={c.s3StorageClass} onChange={(e) => set("s3StorageClass", e.target.value)} className={inputCls}>
            <option value="">{t("cloud.storageClass.default")}</option>
            <option value="STANDARD">STANDARD</option>
            <option value="STANDARD_IA">STANDARD_IA</option>
            <option value="ONEZONE_IA">ONEZONE_IA</option>
            <option value="INTELLIGENT_TIERING">INTELLIGENT_TIERING</option>
            <option value="GLACIER_IR">GLACIER_IR</option>
          </select></label>
        <p className="text-xs text-carbon-textMuted normal-case font-sans">{t("cloud.storageClass.hint")}</p>
      </div>

      <div className="flex flex-col gap-2 rounded-lg bg-carbon-surface2 p-3">
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
        {state === "saved" && <span className="text-sm text-statusOk">{t("settings.saved")}</span>}
        {state === "error" && msg && <span className="text-sm text-statusFail">{msg}</span>}
      </div>
    </Card>
  );
}

// toDraft turns a secret-blanked CloudCredSetInfo (from GET) into an editable
// CloudCredSet with blank secret fields — sending it back with those fields
// blank is exactly what makes the backend's keep-prior-if-blank merge (matched
// by id) preserve the real stored secret, so an untouched set's key/password
// survives a save that only edited a DIFFERENT set in the same list.
function toDraft(s: CloudCredSetInfo): CloudCredSet {
  return { id: s.id, name: s.name, s3KeyId: s.s3KeyId, s3Secret: "", s3Region: s.s3Region, restUser: s.restUser, restPassword: "", s3StorageClass: s.s3StorageClass };
}

// CloudCredSetsCard manages ADDITIONAL named credential sets (#141 stage 2):
// lets an off-site target (OffsiteTargetsSection's editor) opt into its OWN
// S3/restic-REST credentials instead of sharing the single set CloudCard
// manages above — e.g. two S3 endpoints (Hetzner + a local Garage/MinIO) that
// need different keys. Same write-only-secret contract as CloudCard; the
// whole list round-trips through setCloudCredSets (replace-all), which is why
// every save resends every set (via toDraft — see its own comment for why
// that is safe for the sets NOT being edited).
export function CloudCredSetsCard({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const [sets, setSets] = useState<CloudCredSetInfo[]>([]);
  const [editing, setEditing] = useState<CloudCredSet | null>(null);
  const [state, setState] = useState<SaveState>("idle");
  const [msg, setMsg] = useState<string | null>(null);
  const [confirmRemove, setConfirmRemove] = useState<string | null>(null);
  const [removingId, setRemovingId] = useState<string | null>(null);

  function refresh() {
    getCloudCredSets()
      .then((r) => { if (r.ok) setSets(r.sets ?? []); })
      .catch(() => undefined);
  }
  useEffect(refresh, []);

  function openNew() {
    setEditing({ id: crypto.randomUUID(), name: "", s3KeyId: "", s3Secret: "", s3Region: "", restUser: "", restPassword: "", s3StorageClass: "" });
    setState("idle");
    setMsg(null);
  }
  function openEdit(s: CloudCredSetInfo) {
    setEditing(toDraft(s));
    setState("idle");
    setMsg(null);
  }
  function closeEditor() {
    setEditing(null);
    setState("idle");
    setMsg(null);
  }
  function setField<K extends keyof CloudCredSet>(k: K, v: CloudCredSet[K]) {
    setEditing((p) => (p ? { ...p, [k]: v } : p));
  }

  async function save() {
    if (!editing) return;
    setState("saving");
    setMsg(null);
    const isNew = !sets.some((s) => s.id === editing.id);
    const rest = sets.filter((s) => s.id !== editing.id).map(toDraft);
    const next = isNew ? [...rest, editing] : [...rest, editing];
    try {
      const r = await setCloudCredSets(next);
      if (r.ok) {
        setState("saved");
        closeEditor();
        refresh();
      } else {
        setState("error");
        setMsg(r.error ?? t("settings.error"));
      }
    } catch (err) {
      setState("error");
      setMsg(err instanceof Error ? err.message : t("settings.error"));
    }
  }

  async function remove(id: string) {
    setRemovingId(id);
    try {
      const next = sets.filter((s) => s.id !== id).map(toDraft);
      const r = await setCloudCredSets(next);
      if (r.ok) {
        refresh();
      } else {
        setMsg(r.error ?? t("settings.error"));
      }
    } catch (err) {
      setMsg(err instanceof Error ? err.message : t("settings.error"));
    } finally {
      setRemovingId(null);
      setConfirmRemove(null);
    }
  }

  const inputCls =
    "rounded-lg bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid";
  const fieldCls = "flex flex-col gap-1 text-xs font-mono text-carbon-textSub";

  return (
    <Card title={t("cloud.credSets.title")}>
      <p className="text-xs text-carbon-textMuted -mt-1">{t("cloud.credSets.hint")}</p>

      {sets.length === 0 && !editing && (
        <span className="text-xs text-carbon-textMuted">{t("cloud.credSets.none")}</span>
      )}

      {sets.map((s) => (
        <div key={s.id} className="flex items-start justify-between gap-3 rounded-lg bg-carbon-surface2 p-3">
          <div className="flex min-w-0 flex-col gap-1">
            <span className="text-sm text-carbon-text truncate">{s.name}</span>
            <span className="text-xs text-carbon-textMuted font-mono break-all">
              {s.s3KeyId || s.restUser || "—"}
            </span>
          </div>
          <div className="flex shrink-0 items-start gap-2">
            <button
              type="button"
              onClick={() => openEdit(s)}
              className="rounded-lg bg-carbon-surface2 px-2.5 py-1 text-xs text-carbon-text hover:bg-carbon-hover"
            >
              {t("offsite.targets.edit")}
            </button>
            {confirmRemove === s.id ? (
              <button
                type="button"
                onClick={() => void remove(s.id)}
                disabled={removingId === s.id}
                className="rounded-lg bg-statusFailBg px-2.5 py-1 text-xs font-medium text-statusFail hover:bg-statusFailBgHover disabled:opacity-50"
              >
                {removingId === s.id ? t("offsite.targets.removing") : t("offsite.targets.confirmRemove")}
              </button>
            ) : (
              <button
                type="button"
                onClick={() => setConfirmRemove(s.id)}
                className="rounded-lg bg-carbon-surface2 px-2.5 py-1 text-xs text-statusFail hover:bg-carbon-hover"
              >
                {t("offsite.targets.remove")}
              </button>
            )}
          </div>
        </div>
      ))}

      {editing ? (
        <div className="flex flex-col gap-3 rounded-lg bg-carbon-surface2 p-3">
          <label className={fieldCls}>{t("cloud.credSets.name")}
            <input value={editing.name} onChange={(e) => setField("name", e.target.value)} className={inputCls} /></label>
          <div className="flex flex-col gap-2 rounded-lg bg-carbon-surface3/40 p-3">
            <span className="text-xs font-semibold text-carbon-textSub">Amazon S3</span>
            <label className={fieldCls}>AWS_ACCESS_KEY_ID
              <input value={editing.s3KeyId} onChange={(e) => setField("s3KeyId", e.target.value)} spellCheck={false} className={inputCls} /></label>
            <label className={fieldCls}>AWS_SECRET_ACCESS_KEY
              <input type="password" value={editing.s3Secret} onChange={(e) => setField("s3Secret", e.target.value)} spellCheck={false}
                placeholder={sets.find((s) => s.id === editing.id)?.s3SecretSet ? t("cloud.secretSet") : ""} className={inputCls} /></label>
            <label className={fieldCls}>AWS_DEFAULT_REGION
              <input value={editing.s3Region} onChange={(e) => setField("s3Region", e.target.value)} spellCheck={false} placeholder="us-east-1" className={inputCls} /></label>
            <label className={fieldCls}>{t("cloud.storageClass.label")}
              <select value={editing.s3StorageClass} onChange={(e) => setField("s3StorageClass", e.target.value)} className={inputCls}>
                <option value="">{t("cloud.storageClass.default")}</option>
                <option value="STANDARD">STANDARD</option>
                <option value="STANDARD_IA">STANDARD_IA</option>
                <option value="ONEZONE_IA">ONEZONE_IA</option>
                <option value="INTELLIGENT_TIERING">INTELLIGENT_TIERING</option>
                <option value="GLACIER_IR">GLACIER_IR</option>
              </select></label>
          </div>
          <div className="flex flex-col gap-2 rounded-lg bg-carbon-surface3/40 p-3">
            <span className="text-xs font-semibold text-carbon-textSub">restic REST server</span>
            <label className={fieldCls}>RESTIC_REST_USERNAME
              <input value={editing.restUser} onChange={(e) => setField("restUser", e.target.value)} spellCheck={false} className={inputCls} /></label>
            <label className={fieldCls}>RESTIC_REST_PASSWORD
              <input type="password" value={editing.restPassword} onChange={(e) => setField("restPassword", e.target.value)} spellCheck={false}
                placeholder={sets.find((s) => s.id === editing.id)?.restPasswordSet ? t("cloud.secretSet") : ""} className={inputCls} /></label>
          </div>
          <div className="flex items-center gap-3">
            <button
              onClick={() => void save()}
              disabled={state === "saving"}
              className="inline-flex items-center gap-2 rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
            >
              {state === "saving" ? t("auth.saving") : t("settings.save")}
            </button>
            <button
              onClick={closeEditor}
              className="rounded-lg bg-carbon-surface3 px-4 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover"
            >
              {t("common.close")}
            </button>
            {state === "error" && msg && <span className="text-sm text-statusFail">{msg}</span>}
          </div>
        </div>
      ) : (
        <button
          type="button"
          onClick={openNew}
          className="self-start rounded-lg bg-carbon-surface2 px-3 py-1.5 text-xs text-carbon-text hover:bg-carbon-hover"
        >
          + {t("cloud.credSets.add")}
        </button>
      )}
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
  appriseUrl: "",
  appriseTags: "",
  scheduledSummary: false,
  notifyOnUpdate: false,
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
    "rounded-lg bg-carbon-surface3 text-carbon-text text-sm font-mono px-3 py-1.5 focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid";
  const selectCls =
    "rounded-lg bg-carbon-surface3 text-carbon-text text-sm px-2.5 py-1.5 focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid";
  // Card-level sibling of selectCls: same styling, but this one sits directly on
  // the Card (bg-carbon-surface), so its fill is surface2 — the panel-level
  // fields above use surface3 because they sit ON a surface2 panel.
  const selectCardCls =
    "rounded-lg bg-carbon-surface2 text-carbon-text text-sm px-2.5 py-1.5 focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid";
  const labelCls = "flex flex-col gap-1 text-xs text-carbon-textSub";

  return (
    <Card title={t("notify.title")}>
      <p className="text-xs text-carbon-textMuted -mt-1">{t("notify.hint")}</p>

      <label className={labelCls}>
        {t("notify.on")}
        <select value={cfg.on} onChange={(e) => set("on", e.target.value)} className={selectCardCls}>
          <option value="never">{t("notify.onNever")}</option>
          <option value="failure">{t("notify.onFailure")}</option>
          <option value="always">{t("notify.onAlways")}</option>
        </select>
      </label>

      {/* #56: one summary per scheduled run instead of one message per container. */}
      <label className="flex items-start gap-2 rounded-lg bg-carbon-surface2 p-3 cursor-pointer">
        <input
          type="checkbox"
          checked={cfg.scheduledSummary}
          onChange={(e) => set("scheduledSummary", e.target.checked)}
          className="mt-0.5"
          style={{ accentColor: "var(--accent)" }}
        />
        <span className="flex flex-col gap-0.5">
          <span className="text-sm text-carbon-text">{t("notify.scheduledSummary")}</span>
          <span className="text-xs text-carbon-textMuted">{t("notify.scheduledSummaryHint")}</span>
        </span>
      </label>

      {/* #56: notify when a container is updated by the post-backup image update. */}
      <label className="flex items-start gap-2 rounded-lg bg-carbon-surface2 p-3 cursor-pointer">
        <input
          type="checkbox"
          checked={cfg.notifyOnUpdate}
          onChange={(e) => set("notifyOnUpdate", e.target.checked)}
          className="mt-0.5"
          style={{ accentColor: "var(--accent)" }}
        />
        <span className="flex flex-col gap-0.5">
          <span className="text-sm text-carbon-text">{t("notify.notifyOnUpdate")}</span>
          <span className="text-xs text-carbon-textMuted">{t("notify.notifyOnUpdateHint")}</span>
        </span>
      </label>

      {/* Unraid native notifications (delivered over the host SSH connection). */}
      <label className="flex items-start gap-2 rounded-lg bg-carbon-surface2 p-3 cursor-pointer">
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
      <div className="flex flex-col gap-2 rounded-lg bg-carbon-surface2 p-3">
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

      {/* Apprise API: posts to a user-run apprise-api server, unlocking Apprise's
          100+ services without bundling Python. Shares the card's Save + Test bar
          like the other channels. */}
      <div className="flex flex-col gap-2 rounded-lg bg-carbon-surface2 p-3">
        <span className="text-xs font-medium text-carbon-textSub">{t("notify.apprise")}</span>
        <label className={labelCls}>
          {t("notify.appriseUrl")}
          <input value={cfg.appriseUrl} onChange={(e) => set("appriseUrl", e.target.value)} spellCheck={false}
            placeholder="http://apprise:8000/notify/bombvault" className={inputCls} />
        </label>
        <label className={labelCls}>
          {t("notify.appriseTags")}
          <input value={cfg.appriseTags} onChange={(e) => set("appriseTags", e.target.value)} spellCheck={false}
            placeholder="backups,homelab" className={inputCls} />
        </label>
        <p className="text-xs text-carbon-textMuted">{t("notify.appriseHint")}</p>
      </div>

      <div className="flex flex-col gap-2 rounded-lg bg-carbon-surface2 p-3">
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
      <div className="flex flex-col gap-2 rounded-lg bg-carbon-surface2 p-3">
        <span className="text-xs font-medium text-carbon-textSub">{t("notify.hcPerDomain")}</span>
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
      <div className="flex flex-col gap-2 rounded-lg bg-carbon-surface2 p-3">
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
          className="rounded-lg bg-carbon-surface2 px-4 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover transition-colors">
          {t("notify.test")}
        </button>
        {state === "saved" && <span className="text-sm text-statusOk">{t("settings.saved")}</span>}
        {tested && <span className="text-sm text-statusOk">{t("notify.tested")}</span>}
        {state === "error" && msg && <span className="text-sm text-statusFail wrap-break-word">{msg}</span>}
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
  domain: "containers" | "vms" | "flash" | "files";
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
        className="rounded-lg bg-carbon-surface2 px-2.5 py-1 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
      >
        {st === "busy" ? t("offsite.replicating") : t("offsite.replicateNow")}
      </button>
      {st === "ok" && <span className="text-xs text-statusOk">{t("offsite.replicateStarted")}</span>}
      {st === "fail" && <span className="text-xs text-statusFail wrap-break-word">{err}</span>}
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
  domain: "containers" | "vms" | "flash" | "files";
  t: ReturnType<typeof useT>["t"];
}) {
  const [st, setSt] = useState<"idle" | "busy" | "ok" | "uninit" | "fail">("idle");
  const [err, setErr] = useState<string | null>(null);
  // This button probes the PRIMARY target only. Once a domain has more than one
  // off-site copy, say so on the label — an unqualified "Test connection" going
  // green while a second destination was broken is exactly what issue #138
  // reported. Each additional target has its own button in OffsiteTargetsSection.
  const multiTarget = useOffsiteTargets(domain).length > 1;
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
        className="rounded-lg bg-carbon-surface2 px-2.5 py-1 text-xs text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
      >
        {multiTarget ? t("offsite.testPrimary") : t("offsite.test")}
      </button>
      {st === "ok" && <span className="text-xs text-statusOk">{t("offsite.testOk")}</span>}
      {st === "uninit" && <span className="text-xs text-statusWarn">{t("offsite.testUninitialized")}</span>}
      {st === "fail" && (
        <span className="text-xs text-statusFail wrap-break-word">{err ?? t("offsite.testFailed")}</span>
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
//     off-site snapshot, then verification + cleanup. All domains except config
//     (config's real recovery path is the in-place staged restart, not a sandbox
//     restore of the settings DB).
// The DR-drill target (which container's/VM's off-site snapshot to restore) binds
// to the shared settings.drDrillTarget / drDrillTargetVm via the parent's
// baseline-merging save().
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
  // Append-only check (#109): the off-site wizard's tamper test, surfaced here
  // under its plainer name because this card is where users look for checks.
  // TamperRes mirrors the wizard's tri-state verdict: not-testable (amber) /
  // protected (green) / delete-accepted (red); lastTamper feeds the idle
  // "append-only protection · Last checked …" caption from /api/status.
  type TamperRes =
    | { kind: "busy" }
    | { kind: "verdict"; testable: boolean; protected: boolean }
    | { kind: "error"; message: string };
  const [tamper, setTamper] = useState<Record<string, TamperRes | undefined>>({});
  const [lastTamper, setLastTamper] = useState<Record<string, { at: number; ok: boolean } | null>>({});
  // Container list feeding the DR-drill target dropdown (kind "dr", containers).
  const [containers, setContainers] = useState<Container[]>([]);
  // VM list feeding the DR-drill target dropdown (kind "dr", VMs).
  const [vms, setVMs] = useState<VM[]>([]);
  // Save state for the drill-target dropdowns (persisted via the parent save()).
  const [tgtState, setTgtState] = useState<SaveState>("idle");
  const [tgtError, setTgtError] = useState<string | null>(null);
  const [tgtVMState, setTgtVMState] = useState<SaveState>("idle");
  const [tgtVMError, setTgtVMError] = useState<string | null>(null);

  type Domain = "containers" | "vms" | "flash" | "files";
  type Action = "verify" | "unlock" | "prune";

  const domains: { key: Domain; label: string }[] = [
    { key: "containers", label: t("settings.containersEnabled") },
    { key: "vms", label: t("settings.vmsEnabled") },
    { key: "flash", label: t("settings.flashEnabled") },
    { key: "files", label: t("settings.filesEnabled") },
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

  // Load the VMs once for the DR-drill target picker, same reasoning as containers.
  useEffect(() => {
    let active = true;
    listVMs()
      .then((r) => {
        if (active && r.ok) setVMs(r.vms ?? []);
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

  // Load each domain's last tamper-test verdict once, so the append-only row's
  // idle caption mirrors the drill row's "last verified" line. The check always
  // probes the OFF-SITE repo, so the source toggle never re-triggers this.
  useEffect(() => {
    let active = true;
    getStatus()
      .then((r) => {
        if (!active || !r.ok || !r.domains) return;
        const m: Record<string, { at: number; ok: boolean } | null> = {};
        for (const d of r.domains) {
          m[d.domain] = d.lastTamperAt > 0 ? { at: d.lastTamperAt, ok: d.lastTamperOK } : null;
        }
        setLastTamper(m);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, []);

  // runTamperFor proves the domain's off-site repo still refuses deletes — the
  // exact tamper-test API behind the wizard's "Test append-only now" (#109: users
  // searched for it here, and "append-only" is the plainer word for it).
  async function runTamperFor(domain: Domain) {
    setTamper((m) => ({ ...m, [domain]: { kind: "busy" } }));
    try {
      const r = await tamperTest(domain);
      if (r.ok) {
        setTamper((m) => ({
          ...m,
          [domain]: { kind: "verdict", testable: !!r.testable, protected: !!r.protected },
        }));
        // A decisive verdict is also the new "last checked" fact; a not-testable
        // repo records no verdict server-side, so leave the caption untouched.
        if (r.testable) {
          setLastTamper((m) => ({ ...m, [domain]: { at: Math.floor(Date.now() / 1000), ok: !!r.protected } }));
        }
        // The verdict + its run row land in /api/status (scorecard tamper state) —
        // broadcast so the dashboard refetches, mirroring runDrillFor above.
        window.dispatchEvent(new Event("bv:settings-changed"));
      } else {
        setTamper((m) => ({ ...m, [domain]: { kind: "error", message: r.error ?? t("offsite.tamperError") } }));
      }
    } catch (err) {
      setTamper((m) => ({
        ...m,
        [domain]: { kind: "error", message: err instanceof Error ? err.message : t("offsite.tamperError") },
      }));
    }
  }

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

  // Append-only check eligibility: only a domain whose off-site repo is set AND
  // flagged immutable gets the button — the same precondition the wizard's manual
  // test has (anything else could only ever surface a backend error).
  const appendOnlyEligible: Record<Domain, boolean> = {
    containers: settings.containersOffsite !== "" && settings.containersOffsiteImmutable,
    vms: settings.vmsOffsite !== "" && settings.vmsOffsiteImmutable,
    flash: settings.flashOffsite !== "" && settings.flashOffsiteImmutable,
    files: settings.filesOffsite !== "" && settings.filesOffsiteImmutable,
  };

  const selectCls =
    "rounded-lg bg-carbon-surface3 text-carbon-text text-sm px-2.5 py-1.5 focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid";

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
        <div className="inline-flex rounded-lg bg-carbon-surface2 overflow-hidden">
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
                  : "text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
              }`}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      {/* DR-drill controls: an explainer + the container/VM target pickers. Each
          target is a shared setting (settings.drDrillTarget / drDrillTargetVm)
          saved via the parent's baseline-merging save(), so it never clobbers
          other cards' edits. Flash and files have no picker (their whole
          snapshot is restored). */}
      {kind === "dr" && (
        <div className="flex flex-col gap-2 rounded-lg bg-carbon-surface2 p-3">
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
          {tgtState === "saved" && <span className="text-xs text-statusOk">{t("settings.saved")}</span>}
          {tgtState === "error" && tgtError && <span className="text-xs text-statusFail">{tgtError}</span>}
          <label className="flex flex-col gap-1 text-xs text-carbon-textSub max-w-xs">
            {t("drill.targetVM")}
            <select
              value={settings.drDrillTargetVm}
              onChange={(e) => {
                const v = e.target.value;
                setSettings((prev) => (prev ? { ...prev, drDrillTargetVm: v } : prev));
                void save({ drDrillTargetVm: v }, setTgtVMState, setTgtVMError);
              }}
              className={selectCls}
            >
              <option value="">{t("drill.targetMostRecent")}</option>
              {vms.map((v) => (
                // value must be the raw libvirt name: pickDRSnapshot (service.go)
                // matches it against the "vm:"+name backup tag, never the
                // display-only friendly name a TrueNAS VM shows here.
                <option key={v.libvirtName} value={v.libvirtName}>{v.name}</option>
              ))}
            </select>
          </label>
          {tgtVMState === "saved" && <span className="text-xs text-statusOk">{t("settings.saved")}</span>}
          {tgtVMState === "error" && tgtVMError && <span className="text-xs text-statusFail">{tgtVMError}</span>}
        </div>
      )}

      <div className="flex flex-col gap-3">
        {domains.map(({ key: domain, label }) => {
          const dKey = `${domain}:drill`;
          const drill = lastDrill[domain];
          const tRes = tamper[domain];
          const tLast = lastTamper[domain];
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
                        className="rounded-lg bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
                      >
                        {state[k] === "busy" ? a.busy : a.label}
                      </button>
                      {state[k] === "ok" && <span className="text-sm text-statusOk">{t("integrity.ok")}</span>}
                    </span>
                  );
                })}
              </div>

              {/* Restore-verification drill: its own row + inline result + last drill.
                  The run button + labels follow the selected drill kind. */}
              <div className="flex items-center gap-2 flex-wrap">
                <span className="w-24 shrink-0" />
                <button
                  onClick={() => void runDrillFor(domain)}
                  disabled={state[dKey] === "busy"}
                  title={kind === "dr" ? t("drill.drNote") : t("verify.hint")}
                  className="rounded-lg bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
                >
                  {state[dKey] === "busy"
                    ? kind === "dr" ? t("drill.runningDR") : t("verify.running")
                    : kind === "dr" ? t("drill.runDR") : t("verify.now")}
                </button>
                {state[dKey] === "ok" && <span className="text-sm text-statusOk">✓ {t("verify.ok")}</span>}
                {state[dKey] === "fail" && (
                  <span className="text-sm text-statusFail wrap-break-word">✗ {msg[dKey] || t("verify.failed")}</span>
                )}
                {/* Last recorded drill for this domain/source (idle state only).
                    Names WHICH check ran (off-site DR vs local integrity) and,
                    on a stored failure, the scrubbed reason. */}
                {state[dKey] !== "busy" && state[dKey] !== "ok" && state[dKey] !== "fail" && (
                  drill ? (
                    <>
                      <span className="text-xs text-carbon-textMuted">
                        {isOffsiteSource(drill.source) && drill.kind === "dr"
                          ? t("drill.checkOffsiteDr")
                          : t("drill.checkLocal")}
                        {" · "}
                        {t("verify.last").replace("{time}", relativeTime(t, drill.at))} {drill.ok ? "✓" : "✗"}
                      </span>
                      {!drill.ok && drill.detail && (
                        <span className="text-xs text-statusFail wrap-break-word" title={drill.detail}>
                          {t("drill.failReasonPrefix")} {drill.detail}
                        </span>
                      )}
                    </>
                  ) : (
                    <span className="text-xs text-carbon-textMuted">{t("verify.never")}</span>
                  )
                )}
              </div>

              {/* Append-only check (#109): the wizard's tamper test, findable in
                  this card and led by the plainer name. Immutable off-site domains
                  only; always probes the OFF-SITE repo (source-independent). The
                  verdict rendering mirrors the wizard, incl. the glyph as its own
                  node so RTL locales place it correctly. */}
              {appendOnlyEligible[domain] && (
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="w-24 shrink-0" />
                  <button
                    onClick={() => void runTamperFor(domain)}
                    disabled={tRes?.kind === "busy"}
                    title={t("integrity.appendOnlyHint")}
                    className="rounded-lg bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover disabled:opacity-50"
                  >
                    {tRes?.kind === "busy" ? t("integrity.checking") : t("integrity.appendOnly")}
                  </button>
                  {tRes?.kind === "verdict" && (
                    <span
                      className={`text-sm wrap-break-word ${
                        !tRes.testable ? "text-statusWarn" : tRes.protected ? "text-statusOk" : "text-statusFail"
                      }`}
                    >
                      {tRes.testable && <span aria-hidden="true">{tRes.protected ? "✓" : "✗"}&nbsp;</span>}
                      {!tRes.testable
                        ? t("offsite.tamperUnverifiable")
                        : tRes.protected
                          ? t("offsite.tamperOk")
                          : t("offsite.tamperFail")}
                    </span>
                  )}
                  {tRes?.kind === "error" && (
                    <span className="text-sm text-statusFail wrap-break-word">{tRes.message}</span>
                  )}
                  {/* Idle caption: the last recorded check, mirroring the drill
                      row's "Last verified …" line. */}
                  {!tRes &&
                    (tLast ? (
                      <span className="text-xs text-carbon-textMuted">
                        {t("integrity.appendOnlyLast").replace("{time}", relativeTime(t, tLast.at))} {tLast.ok ? "✓" : "✗"}
                      </span>
                    ) : (
                      <span className="text-xs text-carbon-textMuted">{t("integrity.appendOnlyNever")}</span>
                    ))}
                </div>
              )}

              {actions.map((a) =>
                state[`${domain}:${a.key}`] === "fail" ? (
                  <span key={a.key} className="text-xs text-statusFail wrap-break-word">
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
    active: "bg-statusOkBg text-statusOk",
    paused: "bg-statusWarnBg text-statusWarn",
    off:    "bg-carbon-surface2 text-carbon-textSub",
  };
  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 rounded-sm text-xs font-medium ${cls[status]}`}
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
  perItem,
  t,
}: {
  settings: Settings;
  containers: Container[];
  onChange: (schedule: string) => void;
  /** #121: when on, each included container exposes a per-item schedule override. */
  perItem: boolean;
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
      <div className="rounded-lg bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.containersSection")}
          value={schedule}
          onChange={onChange}
        />
        <p className="text-xs text-carbon-textMuted mt-2">{t("containers.scheduleHint")}</p>
      </div>

      {/* Member list */}
      {included.length === 0 ? (
        <p className="text-sm text-carbon-textMuted">{t("jobs.noContainersIncluded")}</p>
      ) : (
        <div className="flex flex-col gap-1 divide-y divide-carbon-border">
          {included.map((c) => (
            <div key={c.name} className="flex flex-col gap-2 py-2 text-sm">
              <div className="flex items-center gap-3">
                <div
                  className={`w-2 h-2 rounded-full shrink-0 ${
                    c.state.toLowerCase() === "running"
                      ? "bg-statusOkSolid"
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
              {perItem && (
                <ItemScheduleOverride
                  name={c.name}
                  initial={c.scheduleCadence ?? ""}
                  onSave={(cadence) => setScheduleCadence(c.name, cadence)}
                />
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
  vms,
  perItem,
  t,
}: {
  settings: Settings;
  syncSchedules: boolean;
  onChange: (schedule: string) => void;
  /** Included VMs, for the per-item override list (#121). */
  vms: VM[];
  /** #121: when on, each included VM exposes a per-item schedule override. */
  perItem: boolean;
  t: ReturnType<typeof useT>["t"];
}) {
  const schedule = syncSchedules ? settings.containersSchedule : settings.vmsSchedule;
  const status = scheduleStatus(schedule);
  const included = vms.filter((v) => v.includeInSchedule);

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
      <div className={`rounded-lg bg-carbon-surface2 p-4 ${syncSchedules ? "opacity-50" : ""}`}>
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

      {/* Per-item overrides (#121): an included-VM list with a per-VM cadence,
          shown only when the toggle is on so the section is otherwise unchanged. */}
      {perItem && (
        included.length === 0 ? (
          <p className="text-sm text-carbon-textMuted">{t("jobs.noVMsIncluded")}</p>
        ) : (
          <div className="flex flex-col gap-1 divide-y divide-carbon-border">
            {included.map((v) => (
              <div key={v.libvirtName} className="flex flex-col gap-2 py-2 text-sm">
                <div className="flex items-center gap-3">
                  <div
                    className={`w-2 h-2 rounded-full shrink-0 ${
                      v.state.toLowerCase() === "running" ? "bg-statusOkSolid" : "bg-carbon-surface3"
                    }`}
                  />
                  <span className="font-medium text-carbon-text flex-1 truncate">{v.name}</span>
                </div>
                <ItemScheduleOverride
                  name={v.name}
                  initial={v.scheduleCadence ?? ""}
                  // libvirtName, not name: PATCH /api/vms/{name} resolves the
                  // path segment against the raw name (see vmNameParam),
                  // never the TrueNAS display-only friendly name.
                  onSave={(cadence) => setVMScheduleCadence(v.libvirtName, cadence)}
                />
              </div>
            ))}
          </div>
        )
      )}
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
      <div className={`rounded-lg bg-carbon-surface2 p-4 ${syncSchedules ? "opacity-50" : ""}`}>
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

// Domain section — Files (editable schedule + per-set include list). Mirrors
// VMsSection for the cadence and ContainersSection for the member list, except
// the per-set "include in schedule" toggles PATCH each file set directly (the
// same {enabled} flag the Files tab edits) — they are not part of the SaveBar.
function FilesSection({
  settings,
  fileSets,
  onChange,
  onSetsChanged,
  t,
}: {
  settings: Settings;
  fileSets: FileSetView[];
  onChange: (schedule: string) => void;
  /** A toggle PATCHed a set — reload the list so the rows reflect the server. */
  onSetsChanged: () => void;
  t: ReturnType<typeof useT>["t"];
}) {
  const schedule = settings.filesSchedule;
  const status = scheduleStatus(schedule);
  // Per-set toggle busy/error state, keyed by set id.
  const [busy, setBusy] = useState<Record<string, boolean>>({});
  const [error, setError] = useState<string | null>(null);

  async function toggle(set: FileSetView) {
    setBusy((b) => ({ ...b, [set.id]: true }));
    setError(null);
    try {
      const res = await patchFileSet(set.id, { enabled: !set.enabled });
      if (res.ok) onSetsChanged();
      else setError(res.error ?? t("settings.error"));
    } catch (err) {
      setError(err instanceof Error ? err.message : t("settings.error"));
    } finally {
      setBusy((b) => ({ ...b, [set.id]: false }));
    }
  }

  return (
    <Card title={t("jobs.filesSection")}>
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
      <div className="rounded-lg bg-carbon-surface2 p-4">
        <CadenceBuilder
          label={t("jobs.filesSection")}
          value={schedule}
          onChange={onChange}
        />
        <p className="text-xs text-carbon-textMuted mt-2">{t("jobs.filesIncludeHint")}</p>
      </div>

      {/* Member list — every file set with its live include-in-schedule toggle. */}
      {fileSets.length === 0 ? (
        <p className="text-sm text-carbon-textMuted">{t("jobs.noFileSetsIncluded")}</p>
      ) : (
        <div className="flex flex-col gap-1 divide-y divide-carbon-border">
          {fileSets.map((s) => (
            <div key={s.id} className="flex items-center gap-3 py-2 text-sm">
              <div
                className={`w-2 h-2 rounded-full shrink-0 ${
                  s.enabled ? "bg-statusOkSolid" : "bg-carbon-surface3"
                }`}
              />
              <span className="font-medium text-carbon-text flex-1 min-w-0 truncate">{s.name}</span>
              {s.path && (
                <span className="text-xs font-mono text-carbon-textMuted truncate hidden sm:block max-w-xs">
                  {s.path}
                </span>
              )}
              <button
                role="switch"
                aria-checked={s.enabled}
                aria-label={`${t("files.enabled")}: ${s.name}`}
                disabled={!!busy[s.id]}
                onClick={() => void toggle(s)}
                className={`relative inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-statusInfoSolid disabled:opacity-50 ${
                  s.enabled ? "bg-accent" : "bg-carbon-surface3"
                }`}
              >
                <span
                  className={`inline-block h-3.5 w-3.5 rounded-full bg-carbon-background transition-transform ${
                    s.enabled ? "translate-x-[18px]" : "translate-x-[3px]"
                  }`}
                />
              </button>
            </div>
          ))}
        </div>
      )}
      {error && <p className="text-xs text-statusFail wrap-break-word">{error}</p>}
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
      <div className={`rounded-lg bg-carbon-surface2 p-4 ${settings.drillsEnabled ? "" : "opacity-50"}`}>
        <CadenceBuilder
          label={t("settings.schedule")}
          value={settings.drillsSchedule}
          disabled={!settings.drillsEnabled}
          onChange={(v) => update({ drillsSchedule: v })}
        />
      </div>
      <label className="flex flex-col gap-1 max-w-40">
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
          className="rounded-lg bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
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
  // Recovery-kit download refusal (e.g. the 403 "set a login password" fail-closed
  // answer when auth is off) — surfaced next to the download button.
  const [kitError, setKitError] = useState<string | null>(null);

  const [pathSaveState, setPathSaveState] = useState<SaveState>("idle");
  const [pathSaveError, setPathSaveError] = useState<string | null>(null);
  // Flash zip export (#28) — its own save state, persisted via the shared save().
  const [flashZipSaveState, setFlashZipSaveState] = useState<SaveState>("idle");
  const [flashZipSaveError, setFlashZipSaveError] = useState<string | null>(null);
  // Remembers the last "keep N" the user picked so toggling history OFF (which
  // zeroes flashZipExportKeep) and back ON restores their count instead of the
  // default. Updated whenever the keepN input is set to a value >= 1.
  const [rememberedKeep, setRememberedKeep] = useState(7);
  const [exportEncSaveState, setExportEncSaveState] = useState<SaveState>("idle");
  const [exportEncSaveError, setExportEncSaveError] = useState<string | null>(null);
  const [offsiteSaveState, setOffsiteSaveState] = useState<SaveState>("idle");
  const [offsiteSaveError, setOffsiteSaveError] = useState<string | null>(null);
  // Which domain's guided off-site setup wizard is expanded (null = none).
  const [offsiteWizard, setOffsiteWizard] = useState<"containers" | "vms" | "flash" | "files" | null>(null);

  const [domSaveState, setDomSaveState] = useState<SaveState>("idle");
  const [domSaveError, setDomSaveError] = useState<string | null>(null);

  const [retSaveState, setRetSaveState] = useState<SaveState>("idle");
  const [retSaveError, setRetSaveError] = useState<string | null>(null);

  const [pruneSaveState, setPruneSaveState] = useState<SaveState>("idle");
  const [pruneSaveError, setPruneSaveError] = useState<string | null>(null);
  // #116: reconcile Unraid's cached update status after a post-backup update.
  const [reconcileSaveState, setReconcileSaveState] = useState<SaveState>("idle");
  const [reconcileSaveError, setReconcileSaveError] = useState<string | null>(null);

  // Container-registry credentials (#106) — its own save state, persisted via
  // the shared baseline-merging save().
  const [registrySaveState, setRegistrySaveState] = useState<SaveState>("idle");
  const [registrySaveError, setRegistrySaveError] = useState<string | null>(null);

  const [cacheSaveState, setCacheSaveState] = useState<SaveState>("idle");
  const [cacheSaveError, setCacheSaveError] = useState<string | null>(null);

  const [offRetSaveState, setOffRetSaveState] = useState<SaveState>("idle");
  const [offRetSaveError, setOffRetSaveError] = useState<string | null>(null);

  const [limSaveState, setLimSaveState] = useState<SaveState>("idle");
  const [limSaveError, setLimSaveError] = useState<string | null>(null);

  const [metricsSaveState, setMetricsSaveState] = useState<SaveState>("idle");
  const [metricsSaveError, setMetricsSaveError] = useState<string | null>(null);

  // Weekly digest (notifications tab) — its own save state, persisted via the
  // shared baseline-merging save().
  const [digestSaveState, setDigestSaveState] = useState<SaveState>("idle");
  const [digestSaveError, setDigestSaveError] = useState<string | null>(null);

  // Overdue-backup watchdog (notifications tab) — its own save state, same
  // baseline-merging save() as the digest card above it.
  const [watchdogSaveState, setWatchdogSaveState] = useState<SaveState>("idle");
  const [watchdogSaveError, setWatchdogSaveError] = useState<string | null>(null);

  // Schedules tab (migrated from the retired Plans page). The container list
  // feeds the Containers schedule section's included-members list; syncSchedules
  // applies the Containers cadence to VMs + Flash; schedSave* drives its SaveBar.
  const [containers, setContainers] = useState<Container[]>([]);
  // VMs feed the VMs schedule section's per-item override list (#121).
  const [vms, setVMs] = useState<VM[]>([]);
  // File sets feed the Files schedule section's member list (live enabled toggles).
  const [fileSets, setFileSets] = useState<FileSetView[]>([]);
  const [syncSchedules, setSyncSchedules] = useState(false);
  const [schedSaveState, setSchedSaveState] = useState<SaveState>("idle");
  const [schedSaveError, setSchedSaveError] = useState<string | null>(null);
  // Health-gated ordered restart (#119) — its own save state, same
  // baseline-merging save() as the other cards on this tab.
  const [restartSaveState, setRestartSaveState] = useState<SaveState>("idle");
  const [restartSaveError, setRestartSaveError] = useState<string | null>(null);

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

    // Load the VM list for the Schedules tab's VMs section per-item overrides (#121).
    listVMs()
      .then((r) => {
        if (r.ok) setVMs(r.vms ?? []);
      })
      .catch(() => {
        // Non-fatal: the VMs schedule section shows an empty per-item list.
      });

    // Load the file sets for the Schedules tab's Files section. Non-fatal too.
    loadFileSets();
  }, []);

  // loadFileSets (re)fetches the file-set list — on mount and after a Files
  // section toggle PATCHes a set, so the member rows track the server state.
  function loadFileSets() {
    listFileSets()
      .then((r) => {
        if (r.ok) setFileSets(r.fileSets ?? []);
      })
      .catch(() => {
        // Non-fatal: the Files schedule section shows an empty member list.
      });
  }

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
    // Files cadence — independent of the sync checkbox (it covers VMs + Flash).
    patch.filesSchedule = settings.filesSchedule;
    // Restore-check (drill) schedule.
    patch.drillsEnabled = settings.drillsEnabled;
    patch.offsiteDrillsEnabled = settings.offsiteDrillsEnabled;
    patch.drillsSchedule = settings.drillsSchedule;
    patch.drillsSubsetPct = settings.drillsSubsetPct;
    // Off-site replication cadences (+ config + files) — sole owner is this tab.
    patch.containersOffsiteSchedule = settings.containersOffsiteSchedule;
    patch.vmsOffsiteSchedule = settings.vmsOffsiteSchedule;
    patch.flashOffsiteSchedule = settings.flashOffsiteSchedule;
    patch.configOffsiteSchedule = settings.configOffsiteSchedule;
    patch.filesOffsiteSchedule = settings.filesOffsiteSchedule;
    // Self-backup cadence + scheduled off-site tamper test.
    patch.configSchedule = settings.configSchedule;
    patch.tamperTestSchedule = settings.tamperTestSchedule;
    // Anacron-style catch-up toggle (Missed schedules card on this tab).
    patch.catchUpMissed = settings.catchUpMissed;
    // Per-item schedules opt-in (#121).
    patch.perItemSchedules = settings.perItemSchedules;
    return patch;
  }

  if (loadError) {
    return (
      <div className="max-w-3xl">
        <p className="text-sm text-statusFail">{loadError}</p>
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

  async function handleLogoutAll() {
    // Rotates the server-side session epoch, revoking EVERY outstanding session
    // cookie (all browsers/devices) — not just clearing this one.
    await logoutAll().catch(() => undefined);
    // Reload so the auth gate re-checks and shows the login screen. Reached via
    // globalThis (cf. downloadRecoveryKit in api.ts): runtime-identical to bare
    // window, but immune to the broken DOM lib resolution.
    const g = globalThis as unknown as { location: { reload(): void } };
    g.location.reload();
  }

  // Tamper-test schedule eligibility (#109): mirrors immutableOffsiteDomains in
  // internal/schedule/schedule.go — the scheduler only wires the scheduled
  // tamper-test job when at least one domain's off-site repo is set AND
  // flagged immutable. Without that, the cadence editor below silently never
  // runs (the same per-domain predicate as appendOnlyEligible in IntegrityCard,
  // widened to "any domain including config").
  const tamperScheduleActive =
    (settings.containersOffsite !== "" && settings.containersOffsiteImmutable) ||
    (settings.vmsOffsite !== "" && settings.vmsOffsiteImmutable) ||
    (settings.flashOffsite !== "" && settings.flashOffsiteImmutable) ||
    (settings.configOffsite !== "" && settings.configOffsiteImmutable) ||
    (settings.filesOffsite !== "" && settings.filesOffsiteImmutable);

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
            className={`rounded-lg px-3 py-1.5 text-sm transition-colors ${
              tab === key
                ? "bg-accent text-accentContrast"
                : "text-carbon-textSub hover:text-carbon-text hover:bg-carbon-hover"
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
          {/* Per-item schedules toggle (#121): opt in to per-container/VM overrides.
              Off by default — while off, the member lists below are unchanged. */}
          <label className="flex items-start gap-2 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={settings.perItemSchedules}
              onChange={(e) =>
                setSettings((prev) => (prev ? { ...prev, perItemSchedules: e.target.checked } : prev))
              }
              className="mt-0.5 h-4 w-4 rounded-sm border-carbon-border bg-carbon-surface2 accent-(--accent)"
            />
            <span className="flex flex-col">
              <span className="text-sm text-carbon-text">{t("settings.perItemSchedules")}</span>
              <span className="text-xs text-carbon-textMuted">{t("settings.perItemSchedulesHint")}</span>
            </span>
          </label>
          <ContainersSection
            settings={settings}
            containers={containers}
            onChange={(v) =>
              setSettings((prev) => (prev ? { ...prev, containersSchedule: v } : prev))
            }
            perItem={settings.perItemSchedules}
            t={t}
          />
          {/* Sync checkbox — applies the Containers cadence to VMs + Flash too. */}
          <label className="flex items-center gap-2 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={syncSchedules}
              onChange={(e) => setSyncSchedules(e.target.checked)}
              className="h-4 w-4 rounded-sm border-carbon-border bg-carbon-surface2 accent-(--accent)"
            />
            <span className="text-sm text-carbon-text">{t("jobs.syncSchedules")}</span>
          </label>
          <VMsSection
            settings={settings}
            syncSchedules={syncSchedules}
            onChange={(v) =>
              setSettings((prev) => (prev ? { ...prev, vmsSchedule: v } : prev))
            }
            vms={vms}
            perItem={settings.perItemSchedules}
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
          <FilesSection
            settings={settings}
            fileSets={fileSets}
            onChange={(v) =>
              setSettings((prev) => (prev ? { ...prev, filesSchedule: v } : prev))
            }
            onSetsChanged={loadFileSets}
            t={t}
          />

          {/* Off-site replication schedules (schedulesOffsite): one cadence per
              domain (+ config + files). Editors here are the sole owner of these
              fields. */}
          <Card title={t("settings.schedulesOffsite")}>
            {([
              ["containersOffsiteSchedule", "nav.containers"],
              ["vmsOffsiteSchedule", "nav.vms"],
              ["flashOffsiteSchedule", "nav.flash"],
              ["configOffsiteSchedule", "nav.config"],
              ["filesOffsiteSchedule", "nav.files"],
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
                  className="rounded-lg bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
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
                className="rounded-lg bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
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

          {/* Missed schedules: anacron-style catch-up after start. Backend runs
              the missed domain job ~2 minutes after boot (see internal/schedule
              CatchUpMissed). */}
          <Card title={t("settings.missedSchedulesTitle")}>
            <ToggleRow
              label={t("settings.catchUpMissed")}
              description={t("settings.catchUpMissedHint")}
              checked={settings.catchUpMissed}
              onChange={(v) =>
                setSettings((prev) => (prev ? { ...prev, catchUpMissed: v } : prev))
              }
            />
          </Card>

          {/* Health-gated ordered restart (#119): after a backup that stopped
              other containers ("Stop other containers during backup"), they
              restart in compose depends_on order and each must report
              healthy/running before its dependents start. The wait also holds
              through the post-backup update recreate (see internal/backup
              orchestrator WhileDependentsStopped). */}
          <Card title={t("settings.restartHealthTitle")}>
            <ToggleRow
              label={t("settings.restartHealthWait")}
              description={t("settings.restartHealthWaitHint")}
              checked={settings.restartHealthWait}
              onChange={(v) =>
                setSettings((prev) => (prev ? { ...prev, restartHealthWait: v } : prev))
              }
            />
            {settings.restartHealthWait && (
              <label className="flex flex-col gap-1 sm:w-1/2">
                <span className="text-xs text-carbon-textSub">
                  {t("settings.restartHealthTimeoutLabel")}
                </span>
                <input
                  type="number"
                  min={5}
                  max={3600}
                  value={settings.restartHealthTimeoutSec}
                  onChange={(e) => {
                    const raw = (e.target as unknown as { value: string }).value;
                    // Clamp to the field minimum (5): never let a transient sub-5
                    // value sit in component state. The server clamps to 5..3600.
                    const n = Math.max(5, parseInt(raw, 10) || 0);
                    setSettings((prev) =>
                      prev ? { ...prev, restartHealthTimeoutSec: n } : prev
                    );
                  }}
                  className="rounded-lg bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
                />
                <span className="text-xs text-carbon-textMuted">
                  {t("settings.restartHealthTimeoutHint")}
                </span>
              </label>
            )}
            <SaveBar
              state={restartSaveState}
              error={restartSaveError}
              onSave={() =>
                void save(
                  {
                    restartHealthWait: settings.restartHealthWait,
                    restartHealthTimeoutSec: settings.restartHealthTimeoutSec,
                  },
                  setRestartSaveState,
                  setRestartSaveError
                )
              }
              t={t}
            />
          </Card>

          {/* Restore-check schedule (schedulesChecks): the scheduled off-site
              append-only tamper test. Previously had no UI editor at all. */}
          <Card title={t("settings.schedulesChecks")}>
            <div className="rounded-lg bg-carbon-surface2 p-4">
              <CadenceBuilder
                label={t("settings.tamperTestSchedule")}
                value={settings.tamperTestSchedule}
                onChange={(v) =>
                  setSettings((prev) => (prev ? { ...prev, tamperTestSchedule: v } : prev))
                }
              />
              {/* #109: the scheduler stays inert without a qualifying domain — this
                  is the only place that told manilx why Sun 08:00 never ran. */}
              {!tamperScheduleActive && (
                <div className="mt-3 rounded-lg bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
                  {t("settings.tamperScheduleInactive")}
                </div>
              )}
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
          label={t("settings.filesEnabled")}
          description="Back up arbitrary folders under your mounts (file sets)"
          checked={settings.filesEnabled}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, filesEnabled: v } : prev)
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
        <ToggleRow
          label={t("settings.receiverEnabled")}
          description={t("settings.receiverEnabledHint")}
          checked={settings.receiverEnabled}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, receiverEnabled: v } : prev)
          }
        />
        <ToggleRow
          label={t("settings.fleetEnabled")}
          description={t("settings.fleetEnabledHint")}
          checked={settings.fleetEnabled}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, fleetEnabled: v } : prev)
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
                filesEnabled: settings.filesEnabled,
                configEnabled: settings.configEnabled,
                receiverEnabled: settings.receiverEnabled,
                fleetEnabled: settings.fleetEnabled,
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
        <FolderBrowser
          label={t("settings.filesPath")}
          value={settings.filesPath}
          hostMountRoot={hostMountRoot}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, filesPath: v } : prev)
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
                filesPath: settings.filesPath,
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
      {/* STORAGE — Local snapshot retention (#51 — moved here from Off-site,  */}
      {/* so it sits with the local backup paths it prunes).                   */}
      {/* ------------------------------------------------------------------ */}
      {tab === "storage" && (
      <Card title={t("settings.retentionTitle")}>
        <p className="text-xs text-carbon-textMuted -mt-1 flex items-center gap-1.5">
          {t("settings.retentionHint")}
          <InfoBubble text={t("settings.retentionCombineInfo")} />
        </p>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          {([
            ["retentionKeepLast", "settings.retentionLast", "settings.retentionLastInfo"],
            ["retentionKeepDaily", "settings.retentionDaily", "settings.retentionDailyInfo"],
            ["retentionKeepWeekly", "settings.retentionWeekly", "settings.retentionWeeklyInfo"],
            ["retentionKeepMonthly", "settings.retentionMonthly", "settings.retentionMonthlyInfo"],
          ] as const).map(([key, label, info]) => (
            <label key={key} className="flex flex-col gap-1">
              <span className="flex items-center gap-1 text-xs text-carbon-textSub">
                {t(label)}
                <InfoBubble text={t(info)} />
              </span>
              <input
                type="number"
                min={0}
                value={settings[key]}
                onChange={(e) => {
                  const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                  setSettings((prev) => (prev ? { ...prev, [key]: n } : prev));
                }}
                className="rounded-lg bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
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
      {/* STORAGE — Prune the superseded image after a post-backup container   */}
      {/* update (#56). Opt-in; keeping the old image makes rollback cheap.    */}
      {/* ------------------------------------------------------------------ */}
      {tab === "storage" && (
      <Card title={t("settings.imageCleanupTitle")}>
        <p className="text-xs text-carbon-textMuted -mt-1">{t("settings.imageCleanupHint")}</p>
        <ToggleRow
          label={t("settings.pruneImageAfterUpdate")}
          description={t("settings.pruneImageAfterUpdateHint")}
          checked={settings.pruneImageAfterUpdate}
          onChange={(v) =>
            setSettings((prev) => (prev ? { ...prev, pruneImageAfterUpdate: v } : prev))
          }
        />
        <SaveBar
          state={pruneSaveState}
          error={pruneSaveError}
          onSave={() =>
            void save(
              { pruneImageAfterUpdate: settings.pruneImageAfterUpdate },
              setPruneSaveState,
              setPruneSaveError
            )
          }
          t={t}
        />
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE — Reconcile Unraid's update status (#116). After the         */}
      {/* post-backup update recreates a container, ask Unraid to refresh its  */}
      {/* own cached "update available" status over the host SSH link so the   */}
      {/* Docker tab's stale banner clears. Best-effort and non-fatal.         */}
      {/* ------------------------------------------------------------------ */}
      {tab === "storage" && (
      <Card title={t("settings.reconcileTitle")}>
        <ToggleRow
          label={t("settings.reconcileUnraidStatus")}
          description={t("settings.reconcileUnraidStatusHint")}
          checked={settings.reconcileUnraidUpdateStatus}
          onChange={(v) =>
            setSettings((prev) =>
              prev ? { ...prev, reconcileUnraidUpdateStatus: v } : prev
            )
          }
        />
        <SaveBar
          state={reconcileSaveState}
          error={reconcileSaveError}
          onSave={() =>
            void save(
              { reconcileUnraidUpdateStatus: settings.reconcileUnraidUpdateStatus },
              setReconcileSaveState,
              setReconcileSaveError
            )
          }
          t={t}
        />
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE — Private container registries (#106): credentials the       */}
      {/* post-backup update pull uses for images in private/sponsor-gated     */}
      {/* registries (e.g. a ghcr.io sponsor image). Tokens are write-only:    */}
      {/* GET returns them blank (tokenSet = stored), blank-on-save keeps the  */}
      {/* stored one, and removing a row deletes that registry's credential.   */}
      {/* ------------------------------------------------------------------ */}
      {tab === "storage" && (
      <Card title={t("settings.registriesTitle")}>
        <p className="text-xs text-carbon-textMuted -mt-1">
          {t("settings.registriesHint")}
        </p>
        {settings.registryAuths.length === 0 && (
          <p className="text-sm text-carbon-textMuted">
            {t("settings.registriesEmpty")}
          </p>
        )}
        {settings.registryAuths.map((entry, i) => (
          <div
            key={i}
            className="grid grid-cols-1 sm:grid-cols-[1fr_1fr_1fr_auto] gap-2 items-end"
          >
            <label className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">
                {t("settings.registryHost")}
              </span>
              <input
                type="text"
                value={entry.host}
                placeholder="ghcr.io"
                onChange={(e) => {
                  const host = e.target.value;
                  setSettings((prev) =>
                    prev
                      ? {
                          ...prev,
                          registryAuths: prev.registryAuths.map((a, j) =>
                            j === i ? { ...a, host } : a
                          ),
                        }
                      : prev
                  );
                }}
                className="rounded-lg bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
              />
            </label>
            <label className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">
                {t("settings.registryUser")}
              </span>
              <input
                type="text"
                value={entry.username}
                autoComplete="off"
                onChange={(e) => {
                  const username = e.target.value;
                  setSettings((prev) =>
                    prev
                      ? {
                          ...prev,
                          registryAuths: prev.registryAuths.map((a, j) =>
                            j === i ? { ...a, username } : a
                          ),
                        }
                      : prev
                  );
                }}
                className="rounded-lg bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
              />
            </label>
            <label className="flex flex-col gap-1">
              <span className="text-xs text-carbon-textSub">
                {t("settings.registryToken")}
              </span>
              <input
                type="password"
                value={entry.token}
                autoComplete="new-password"
                placeholder={
                  entry.tokenSet && entry.token === ""
                    ? t("cloud.secretSet")
                    : ""
                }
                onChange={(e) => {
                  const token = e.target.value;
                  setSettings((prev) =>
                    prev
                      ? {
                          ...prev,
                          registryAuths: prev.registryAuths.map((a, j) =>
                            j === i ? { ...a, token } : a
                          ),
                        }
                      : prev
                  );
                }}
                className="rounded-lg bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
              />
            </label>
            <button
              type="button"
              onClick={() =>
                setSettings((prev) =>
                  prev
                    ? {
                        ...prev,
                        registryAuths: prev.registryAuths.filter((_, j) => j !== i),
                      }
                    : prev
                )
              }
              className="rounded-lg bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover transition-colors"
            >
              {t("settings.registryRemove")}
            </button>
          </div>
        ))}
        <div>
          <button
            type="button"
            onClick={() =>
              setSettings((prev) => {
                if (!prev) return prev;
                const blank: RegistryAuthEntry = {
                  host: "",
                  username: "",
                  token: "",
                  tokenSet: false,
                };
                return { ...prev, registryAuths: [...prev.registryAuths, blank] };
              })
            }
            className="rounded-lg bg-carbon-surface2 px-4 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover transition-colors"
          >
            {t("settings.registryAdd")}
          </button>
        </div>
        <SaveBar
          state={registrySaveState}
          error={registrySaveError}
          onSave={() =>
            void save(
              {
                // Drop untouched blank rows; mark a freshly typed token as
                // stored so its input shows the kept-placeholder after saving
                // (mirrors the metricsTokenSet handling).
                registryAuths: settings.registryAuths
                  .filter(
                    (a) =>
                      a.host.trim() !== "" ||
                      a.username.trim() !== "" ||
                      a.token.trim() !== ""
                  )
                  .map((a) => ({
                    ...a,
                    tokenSet: a.tokenSet || a.token.trim() !== "",
                  })),
              },
              setRegistrySaveState,
              setRegistrySaveError
            )
          }
          t={t}
        />
      </Card>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* STORAGE — restic cache size limit. The persistent cache under        */}
      {/* /config (RESTIC_CACHE_DIR) survives restarts and would otherwise     */}
      {/* grow unbounded; LRU per-repo caches are evicted after scheduled runs.*/}
      {/* ------------------------------------------------------------------ */}
      {tab === "storage" && (
      <Advanced>
      <Card title={t("settings.cacheTitle")}>
        <p className="text-xs text-carbon-textMuted -mt-1">
          {t("settings.cacheHint")}
        </p>
        <label className="flex flex-col gap-1 sm:w-1/2">
          <span className="text-xs text-carbon-textSub">{t("settings.cacheLimitLabel")}</span>
          <input
            type="number"
            min={0}
            value={settings.resticCacheMaxMB}
            onChange={(e) => {
              // Structural cast (cf. handleLogoutAll): runtime-identical to
              // e.target.value, but immune to the broken DOM lib resolution.
              const raw = (e.target as unknown as { value: string }).value;
              const n = Math.max(0, parseInt(raw, 10) || 0);
              setSettings((prev) => (prev ? { ...prev, resticCacheMaxMB: n } : prev));
            }}
            className="rounded-lg bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
          />
        </label>
        <SaveBar
          state={cacheSaveState}
          error={cacheSaveError}
          onSave={() =>
            void save(
              { resticCacheMaxMB: settings.resticCacheMaxMB },
              setCacheSaveState,
              setCacheSaveError
            )
          }
          t={t}
        />
      </Card>
      </Advanced>
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
            <div className="rounded-lg bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
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
              <p className="text-xs text-statusFail -mt-1">{t("flash.zipExport.pathRequired")}</p>
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
              <label className="flex flex-col gap-1 max-w-40">
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
                  className="rounded-lg bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
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
      {/* STORAGE: Export encryption (age). Optionally seals the PLAIN export   */}
      {/* artifacts (container/VM tar.gz + xml, flash zip) with age recipients. */}
      {/* Applies across domains, so it is not gated on any single domain.      */}
      {/* ------------------------------------------------------------------ */}
      {tab === "storage" && (
      <Card title={t("export.encrypt.title")}>
        <p className="text-xs text-carbon-textMuted -mt-1">{t("export.encrypt.hint")}</p>
        <ToggleRow
          label={t("export.encrypt.enable")}
          description={t("export.encrypt.enableHint")}
          checked={settings.exportEncryptEnabled}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, exportEncryptEnabled: v } : prev)
          }
        />
        {settings.exportEncryptEnabled && (
          <label className="flex flex-col gap-1">
            <span className="text-xs text-carbon-textSub">{t("export.encrypt.recipients")}</span>
            <textarea
              value={settings.exportAgeRecipients}
              spellCheck={false}
              rows={3}
              onChange={(e) =>
                setSettings((prev) => prev ? { ...prev, exportAgeRecipients: e.target.value } : prev)
              }
              placeholder={t("export.encrypt.recipientsPlaceholder")}
              className="rounded-lg bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
            />
            <span className="text-xs text-carbon-textMuted">{t("export.encrypt.recipientsHint")}</span>
            {!settings.exportAgeRecipients.trim() && (
              <span className="text-xs text-statusFail">{t("export.encrypt.recipientsRequired")}</span>
            )}
          </label>
        )}
        <SaveBar
          state={exportEncSaveState}
          error={exportEncSaveError}
          disabled={settings.exportEncryptEnabled && !settings.exportAgeRecipients.trim()}
          onSave={() =>
            void save(
              {
                exportEncryptEnabled: settings.exportEncryptEnabled,
                exportAgeRecipients: settings.exportAgeRecipients,
              },
              setExportEncSaveState,
              setExportEncSaveError
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
          ["filesOffsite", "nav.files", "files"],
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
                  className="rounded-lg bg-carbon-surface2 px-2.5 py-1 text-xs text-carbon-text hover:bg-carbon-hover"
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
              <>
                <input
                  value={settings[repoKey]}
                  spellCheck={false}
                  onChange={(e) =>
                    setSettings((prev) => (prev ? { ...prev, [repoKey]: e.target.value } : prev))
                  }
                  placeholder="rest:http://host:8000/repo"
                  className="rounded-lg bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
                />
                {/* A mounted share is a perfectly valid off-site target, but the
                    placeholder only ever showed a REST URL — so nothing told the
                    operator a bare relative path works here (issue #138). */}
                <span className="text-xs text-carbon-textMuted">{t("offsite.repoLocalHint")}</span>
              </>
            )}
            {/* Additional off-site targets (multi-off-site): extra copies of this
                domain beyond the primary editor above, managed via the CRUD API. */}
            <OffsiteTargetsSection domain={domain} t={t} />
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
                filesOffsite: settings.filesOffsite,
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
      {/* OFFSITE — Retention (off-site repo only; local retention now lives   */}
      {/* in the Storage tab, #51).                                            */}
      {/* ------------------------------------------------------------------ */}
      {tab === "offsite" && (
      <Card title={t("settings.retentionOffsiteTitle")}>
        <p className="text-xs text-carbon-textMuted -mt-1 flex items-center gap-1.5">
          {t("settings.retentionOffsiteHint")}
          <InfoBubble text={t("settings.retentionCombineInfo")} />
          <InfoBubble text={t("settings.retentionOffsiteImmutableInfo")} />
        </p>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          {([
            ["offsiteRetentionKeepLast", "settings.retentionLast", "settings.retentionLastInfo"],
            ["offsiteRetentionKeepDaily", "settings.retentionDaily", "settings.retentionDailyInfo"],
            ["offsiteRetentionKeepWeekly", "settings.retentionWeekly", "settings.retentionWeeklyInfo"],
            ["offsiteRetentionKeepMonthly", "settings.retentionMonthly", "settings.retentionMonthlyInfo"],
          ] as const).map(([key, label, info]) => (
            <label key={key} className="flex flex-col gap-1">
              <span className="flex items-center gap-1 text-xs text-carbon-textSub">
                {t(label)}
                <InfoBubble text={t(info)} />
              </span>
              <input
                type="number"
                min={0}
                value={settings[key]}
                onChange={(e) => {
                  const n = Math.max(0, parseInt(e.target.value, 10) || 0);
                  setSettings((prev) => (prev ? { ...prev, [key]: n } : prev));
                }}
                className="rounded-lg bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
              />
            </label>
          ))}
        </div>
        <SaveBar
          state={offRetSaveState}
          error={offRetSaveError}
          onSave={() =>
            void save(
              {
                offsiteRetentionKeepLast: settings.offsiteRetentionKeepLast,
                offsiteRetentionKeepDaily: settings.offsiteRetentionKeepDaily,
                offsiteRetentionKeepWeekly: settings.offsiteRetentionKeepWeekly,
                offsiteRetentionKeepMonthly: settings.offsiteRetentionKeepMonthly,
              },
              setOffRetSaveState,
              setOffRetSaveError
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
                className="rounded-lg bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 w-full focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
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
        {/* Write-only secret (the GET never echoes it): blank-on-save keeps the
            stored token, so a stored one shows as the same "saved — leave blank
            to keep" placeholder the cloud-credential secrets use. */}
        <label className="flex flex-col gap-1.5">
          <span className="text-xs text-carbon-textSub">{t("settings.metricsToken")}</span>
          <input
            type="password"
            value={settings.metricsToken}
            spellCheck={false}
            autoComplete="off"
            onChange={(e) =>
              setSettings((prev) => prev ? { ...prev, metricsToken: e.target.value } : prev)
            }
            placeholder={settings.metricsTokenSet && settings.metricsToken === "" ? t("cloud.secretSet") : ""}
            className="rounded-lg bg-carbon-surface2 text-carbon-text text-sm font-mono px-3 py-1.5 focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
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
                // Keep the is-set flag honest locally: saving a non-blank token
                // stores one; a blank save keeps whatever was stored before.
                metricsTokenSet: settings.metricsToken.trim() !== "" || settings.metricsTokenSet,
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
      {/* SYSTEM — Dashboard widget (embeddable activity log). Not behind      */}
      {/* Advanced: it is an end-user feature, unlike the ops-y metrics card.  */}
      {/* ------------------------------------------------------------------ */}
      {tab === "system" && (
        <>
        <DashboardWidgetCard
          t={t}
          tokenSet={settings.widgetTokenSet}
          onTokenSet={(set) => {
            // Keep BOTH the live and the saved baseline in sync: the token is
            // managed by its own endpoints, so a later card save (which merges
            // onto savedSettings) must not carry a stale widgetTokenSet.
            setSettings((prev) => (prev ? { ...prev, widgetTokenSet: set } : prev));
            setSavedSettings((prev) => (prev ? { ...prev, widgetTokenSet: set } : prev));
          }}
        />
        <FleetSettingsCard
          t={t}
          settings={settings}
          setSettings={setSettings}
          save={save}
          tokenSet={settings.fleetTokenSet}
          onTokenSet={(set) => {
            setSettings((prev) => (prev ? { ...prev, fleetTokenSet: set } : prev));
            setSavedSettings((prev) => (prev ? { ...prev, fleetTokenSet: set } : prev));
          }}
        />
        </>
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
        <div className="rounded-lg bg-statusWarnBg px-3 py-2.5 text-xs text-statusWarn leading-relaxed">
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
            <button
              type="button"
              onClick={() => {
                setKitError(null);
                void downloadRecoveryKit().then(setKitError);
              }}
              className="self-start rounded-md bg-carbon-surface3 hover:bg-carbon-border px-3 py-1.5 text-sm text-carbon-text transition-colors"
            >
              {t("recovery.download")}
            </button>
            {kitError && (
              // Backend-provided error text shown verbatim BY DESIGN (e.g. the
              // fail-closed "set a login password" refusal when auth is off) —
              // the API answers English and is not translated client-side.
              <span className="text-xs text-statusFail wrap-break-word">✗ {kitError}</span>
            )}
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
      {/* OFFSITE — Off-site backends (rclone + cloud credentials). Same     */}
      {/* "not advanced-only" rule as the off-site repo-path Card above: a   */}
      {/* user can't actually USE an rclone:/s3:/rest: off-site URL without  */}
      {/* these credentials, so hiding them behind Advanced silently broke   */}
      {/* off-site setup for Simple-mode users (they'd only find these two   */}
      {/* cards by way of the Recovery page, which never gated them either). */}
      {/* ------------------------------------------------------------------ */}
      {tab === "offsite" && <RcloneCard t={t} />}

      {tab === "offsite" && <CloudCard t={t} />}
      {tab === "offsite" && <CloudCredSetsCard t={t} />}

      {/* ------------------------------------------------------------------ */}
      {/* NOTIFICATIONS — NotifyCard (renders always; not re-gated).          */}
      {/* ------------------------------------------------------------------ */}
      {tab === "notifications" && <NotifyCard t={t} />}

      {/* NOTIFICATIONS — Weekly digest: one summary message per week through
          the channels configured above. Schedule input mirrors the drills/
          tamper cadence editors (CadenceBuilder + opacity gate). */}
      {tab === "notifications" && (
        <Card title={t("settings.digestTitle")}>
          <p className="text-xs text-carbon-textMuted -mt-1">
            {t("settings.digestHint")}
          </p>
          <ToggleRow
            label={t("settings.digestToggle")}
            checked={settings.digestEnabled}
            onChange={(v) =>
              setSettings((prev) => (prev ? { ...prev, digestEnabled: v } : prev))
            }
          />
          <div className={`rounded-lg bg-carbon-surface2 p-4 ${settings.digestEnabled ? "" : "opacity-50"}`}>
            <CadenceBuilder
              label={t("settings.schedule")}
              value={settings.digestSchedule}
              disabled={!settings.digestEnabled}
              onChange={(v) =>
                setSettings((prev) => (prev ? { ...prev, digestSchedule: v } : prev))
              }
            />
          </div>
          <SaveBar
            state={digestSaveState}
            error={digestSaveError}
            onSave={() =>
              void save(
                {
                  digestEnabled: settings.digestEnabled,
                  digestSchedule: settings.digestSchedule,
                },
                setDigestSaveState,
                setDigestSaveError
              )
            }
            t={t}
          />
        </Card>
      )}

      {/* NOTIFICATIONS — Overdue-backup watchdog: a fixed daily check (09:00)
          that pushes ONE notification per overdue episode through the channels
          configured above; a new successful backup re-arms it. */}
      {tab === "notifications" && (
        <Card title={t("settings.watchdogTitle")}>
          <p className="text-xs text-carbon-textMuted -mt-1">{t("settings.watchdogHint")}</p>
          <ToggleRow
            label={t("settings.watchdogToggle")}
            checked={settings.watchdogEnabled}
            onChange={(v) =>
              setSettings((prev) => (prev ? { ...prev, watchdogEnabled: v } : prev))
            }
          />
          <SaveBar
            state={watchdogSaveState}
            error={watchdogSaveError}
            onSave={() =>
              void save(
                { watchdogEnabled: settings.watchdogEnabled },
                setWatchdogSaveState,
                setWatchdogSaveError
              )
            }
            t={t}
          />
        </Card>
      )}

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
            className={`inline-block h-2 w-2 rounded-full ${authEnabled ? "bg-statusOkSolid" : "bg-carbon-textMuted"}`}
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
              className="rounded-lg bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
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
              className="rounded-lg bg-carbon-surface2 text-carbon-text text-sm px-3 py-1.5 focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
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
              <span className="text-sm text-statusOk">{pwSaveMsg}</span>
            )}
            {pwSaveState === "error" && pwSaveMsg && (
              <span className="text-sm text-statusFail">{pwSaveMsg}</span>
            )}
          </div>
        </div>

        {/* Logout buttons — only shown when currently signed in. Plain sign-out
            clears THIS browser's cookie; "sign out everywhere" rotates the
            server-side session epoch, revoking every outstanding session. */}
        {authEnabled && authAuthed && (
          <div className="pt-2 border-t border-carbon-border flex items-center gap-3">
            <button
              onClick={() => void handleLogout()}
              className="rounded-lg bg-carbon-surface2 px-4 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover transition-colors"
            >
              {t("auth.logout")}
            </button>
            <button
              onClick={() => void handleLogoutAll()}
              className="rounded-lg bg-carbon-surface2 px-4 py-1.5 text-sm text-carbon-text hover:bg-carbon-hover transition-colors"
            >
              {t("settings.logoutAll")}
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
                className="h-8 w-14 cursor-pointer rounded-sm bg-carbon-surface2 p-0.5 focus:outline-solid focus:outline-2 focus:outline-statusInfoSolid"
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

      {/* ------------------------------------------------------------------ */}
      {/* SYSTEM — Export / import settings                                   */}
      {/* Portable config file: move this instance's settings + off-site      */}
      {/* destinations (and, opt-in, credentials) to another install. Backups, */}
      {/* snapshots and history are never touched.                            */}
      {/* ------------------------------------------------------------------ */}
      {tab === "system" && <SettingsPortabilityCard t={t} />}

      {/* SYSTEM — Version + report-a-bug (kept out of the sidebar for a clean UI). */}
      {tab === "system" && <AboutFooter />}
    </div>
  );
}
