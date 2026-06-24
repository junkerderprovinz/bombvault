import { useEffect, useState, useCallback } from "react";
import { getSettings, putSettings, browse, getAuth, setAuthPassword, logout, getVMSSH, testVMSSH, getRclone, setRclone, getCloud, setCloud, checkDomain, unlockDomain, pruneDomain, replicateOffsite, getNotify, setNotify, testNotify } from "../lib/api";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import type { Settings, NotifyConfig } from "../lib/api";
import { useT } from "../lib/i18n";
import { SpikePanel } from "../components/SpikePanel";
import { getAccent, setAccent, DEFAULT_ACCENT } from "../lib/accent";

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

function ToggleRow({
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
          checked ? "bg-[#6fdc8c]" : "bg-carbon-surface3"
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
}: {
  state: SaveState;
  error: string | null;
  onSave: () => void;
  t: ReturnType<typeof useT>["t"];
}) {
  return (
    <div className="flex items-center gap-3 pt-1">
      <button
        onClick={onSave}
        disabled={state === "saving"}
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
// Folder browser (Feature 3)
// ---------------------------------------------------------------------------

interface FolderBrowserProps {
  label: string;
  value: string;
  hostMountRoot: string;
  onChange: (v: string) => void;
}

function FolderBrowser({ label, value, hostMountRoot, onChange }: FolderBrowserProps) {
  const { t } = useT();
  // browsePath tracks the *current directory being listed* (not the selected value).
  // We initialise it to the current value so opening the browser starts in the right folder.
  const [open, setOpen] = useState(false);
  const [browsePath, setBrowsePath] = useState(value);
  const [dirs, setDirs] = useState<{ name: string; path: string }[]>([]);
  const [browseError, setBrowseError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [manualFallback, setManualFallback] = useState(false);

  const doFetch = useCallback((path: string) => {
    setLoading(true);
    setBrowseError(null);
    browse(path)
      .then((res) => {
        if (!res.ok) {
          setBrowseError(res.error ?? t("folder.couldNotRead"));
          setManualFallback(true);
          return;
        }
        setDirs(res.dirs ?? []);
        setBrowsePath(path);
      })
      .catch((err: unknown) => {
        const msg = err instanceof Error ? err.message : t("folder.browseFailed");
        setBrowseError(msg);
        setManualFallback(true);
      })
      .finally(() => setLoading(false));
  }, [t]);

  function handleOpen() {
    setManualFallback(false);
    setOpen(true);
    doFetch(value);
  }

  function handleClose() {
    setOpen(false);
    setBrowseError(null);
  }

  function handleUp() {
    const parts = browsePath.split("/").filter(Boolean);
    parts.pop();
    doFetch(parts.join("/"));
  }

  function handleSelect() {
    onChange(browsePath);
    setOpen(false);
  }

  const trimmed = value.trim();
  const resolved =
    trimmed && !trimmed.startsWith("/") && !trimmed.includes("..")
      ? `${hostMountRoot}/${trimmed}`
      : "";

  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-xs text-carbon-textSub">{label}</label>

      {/* Current value + browser trigger */}
      <div className="flex items-center gap-2">
        <input
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          spellCheck={false}
          placeholder="user/bombvault/container"
          className="flex-1 rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm font-mono px-3 py-1.5 focus:outline-none focus:border-[#78a9ff]"
        />
        <button
          onClick={handleOpen}
          title={t("folder.browseTitle")}
          className="shrink-0 rounded-lg bg-carbon-surface2 border border-carbon-border px-3 py-1.5 text-xs text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors"
        >
          {t("folder.browse")}
        </button>
      </div>

      {/* Absolute path preview */}
      {resolved && (
        <p className="text-xs text-carbon-textMuted font-mono">→ {resolved}</p>
      )}
      {!resolved && trimmed && (
        <p className="text-xs text-[#ff8389]">
          {t("folder.pathHint")}
        </p>
      )}

      {/* Browser panel */}
      {open && (
        <div className="mt-1 rounded-lg bg-carbon-surface2 border border-carbon-border p-3 flex flex-col gap-2">
          {/* Header: current path + close */}
          <div className="flex items-center justify-between gap-2">
            <span className="text-xs font-mono text-carbon-textSub truncate">
              {hostMountRoot}/{browsePath || ""}
            </span>
            <button
              onClick={handleClose}
              className="text-xs text-carbon-textMuted hover:text-carbon-text shrink-0"
            >
              ✕
            </button>
          </div>

          {/* Error state with manual fallback */}
          {browseError && (
            <p className="text-xs text-[#ff8389]">{browseError}</p>
          )}

          {/* Loading spinner */}
          {loading && (
            <div className="flex items-center gap-2 text-xs text-carbon-textMuted">
              <span className="h-3 w-3 rounded-full border-2 border-[#78a9ff] border-t-transparent animate-spin" />
              {t("folder.loading")}
            </div>
          )}

          {/* Directory listing */}
          {!loading && !manualFallback && (
            <div className="flex flex-col gap-0.5 max-h-48 overflow-y-auto">
              {/* ".." go up — only when not at root */}
              {browsePath !== "" && (
                <button
                  onClick={handleUp}
                  className="text-left text-xs font-mono text-carbon-textSub px-2 py-1 rounded hover:bg-carbon-hover hover:text-carbon-text transition-colors"
                >
                  ..
                </button>
              )}
              {dirs.length === 0 && !browseError && (
                <p className="text-xs text-carbon-textMuted px-2">{t("folder.none")}</p>
              )}
              {dirs.map((d) => (
                <button
                  key={d.path}
                  onClick={() => doFetch(d.path)}
                  className="text-left text-xs font-mono text-carbon-textSub px-2 py-1 rounded hover:bg-carbon-hover hover:text-carbon-text transition-colors"
                >
                  {d.name}/
                </button>
              ))}
            </div>
          )}

          {/* Action buttons */}
          {!manualFallback && (
            <div className="flex items-center gap-2 pt-1 border-t border-carbon-border">
              <button
                onClick={handleSelect}
                className="text-xs rounded-lg bg-carbon-surface3 px-3 py-1 text-carbon-text hover:bg-carbon-hover transition-colors"
              >
                {t("folder.use")}
              </button>
              <span className="text-xs text-carbon-textMuted font-mono truncate">
                {browsePath || "(root)"}
              </span>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Schedule cadence builder (Feature 2)
// ---------------------------------------------------------------------------

type CadenceMode = "off" | "daily" | "weekly" | "everyN";

const WEEKDAYS = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"] as const;

interface CadenceState {
  mode: CadenceMode;
  time: string; // "HH:MM"
  weekdays: string[]; // subset of WEEKDAYS, for weekly
  intervalDays: number; // for everyN
}

const DEFAULT_CADENCE: CadenceState = {
  mode: "off",
  time: "02:00",
  weekdays: ["Mon"],
  intervalDays: 3,
};

/** Build the grammar string from builder state. */
function buildCadenceString(s: CadenceState): string {
  switch (s.mode) {
    case "off":
      return "off";
    case "daily":
      return `daily ${s.time}`;
    case "weekly": {
      const days = WEEKDAYS.filter((d) => s.weekdays.includes(d));
      const daysStr = days.length > 0 ? days.join(",") : "Mon";
      return `weekly ${daysStr} ${s.time}`;
    }
    case "everyN":
      return `everyN ${Math.max(1, s.intervalDays)} ${s.time}`;
  }
}

/** Parse a stored cadence string back into builder state. */
function parseCadenceString(raw: string): CadenceState {
  const s = (raw ?? "").trim();
  if (!s || s === "off") return { ...DEFAULT_CADENCE, mode: "off" };

  const dailyM = /^daily\s+(\d{1,2}:\d{2})$/.exec(s);
  if (dailyM) return { mode: "daily", time: dailyM[1], weekdays: ["Mon"], intervalDays: 3 };

  const weeklyM = /^weekly\s+([\w,]+)\s+(\d{1,2}:\d{2})$/.exec(s);
  if (weeklyM) {
    const days = weeklyM[1]
      .split(",")
      .map((d) => d.trim())
      .map((d) => d.charAt(0).toUpperCase() + d.slice(1).toLowerCase());
    return { mode: "weekly", time: weeklyM[2], weekdays: days, intervalDays: 3 };
  }

  const everyNM = /^everyN\s+(\d+)\s+(\d{1,2}:\d{2})$/.exec(s);
  if (everyNM) {
    return { mode: "everyN", time: everyNM[2], weekdays: ["Mon"], intervalDays: parseInt(everyNM[1], 10) };
  }

  // Unrecognised (e.g. raw cron from old data) — fall back to off
  return { ...DEFAULT_CADENCE, mode: "off" };
}

function CadenceBuilder({
  label,
  value,
  disabled,
  onChange,
}: {
  label: string;
  value: string;
  disabled?: boolean;
  onChange: (v: string) => void;
}) {
  const { t } = useT();
  const [state, setState] = useState<CadenceState>(() => parseCadenceString(value));

  // Re-parse when the stored value changes externally (e.g. after load or sync checkbox)
  useEffect(() => {
    setState(parseCadenceString(value));
  }, [value]);

  function update(patch: Partial<CadenceState>) {
    setState((prev) => {
      const next = { ...prev, ...patch };
      onChange(buildCadenceString(next));
      return next;
    });
  }

  function toggleWeekday(day: string) {
    const current = state.weekdays;
    const next = current.includes(day)
      ? current.filter((d) => d !== day)
      : [...current, day];
    // Always keep at least one weekday selected
    if (next.length === 0) return;
    update({ weekdays: next });
  }

  const inputCls =
    "rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm px-2.5 py-1.5 focus:outline-none focus:border-[#78a9ff] disabled:opacity-50";

  return (
    <div className={`flex flex-col gap-3 ${disabled ? "opacity-50 pointer-events-none" : ""}`}>
      <span className="text-xs text-carbon-textSub font-medium">{label}</span>

      {/* Mode pills */}
      <div className="flex flex-wrap gap-2">
        {(["off", "daily", "weekly", "everyN"] as CadenceMode[]).map((m) => (
          <button
            key={m}
            onClick={() => update({ mode: m })}
            className={`rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
              state.mode === m
                ? "bg-carbon-surface3 text-carbon-text"
                : "bg-carbon-surface2 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text"
            }`}
          >
            {m === "off" ? t("cadence.off") : m === "daily" ? t("cadence.daily") : m === "weekly" ? t("cadence.weekly") : t("cadence.everyN")}
          </button>
        ))}
      </div>

      {/* Time picker — shown for all non-off modes */}
      {state.mode !== "off" && (
        <div className="flex items-center gap-3">
          <label className="text-xs text-carbon-textMuted w-16">{t("cadence.time")}</label>
          <input
            type="time"
            value={state.time}
            onChange={(e) => update({ time: e.target.value })}
            className={inputCls}
          />
        </div>
      )}

      {/* Weekly: weekday checkboxes */}
      {state.mode === "weekly" && (
        <div className="flex items-center gap-2 flex-wrap">
          <label className="text-xs text-carbon-textMuted w-16">{t("cadence.days")}</label>
          <div className="flex flex-wrap gap-1.5">
            {WEEKDAYS.map((d) => (
              <button
                key={d}
                onClick={() => toggleWeekday(d)}
                className={`rounded px-2 py-0.5 text-xs font-medium transition-colors ${
                  state.weekdays.includes(d)
                    ? "bg-[#1c3a2a] text-[#6fdc8c] border border-[#2a5540]"
                    : "bg-carbon-surface2 text-carbon-textSub border border-carbon-border hover:bg-carbon-hover"
                }`}
              >
                {d}
              </button>
            ))}
          </div>
        </div>
      )}

      {/* Every N days: number input */}
      {state.mode === "everyN" && (
        <div className="flex items-center gap-3">
          <label className="text-xs text-carbon-textMuted w-16">{t("cadence.every")}</label>
          <input
            type="number"
            min={1}
            value={state.intervalDays}
            onChange={(e) => {
              const n = parseInt(e.target.value, 10);
              if (!isNaN(n) && n >= 1) update({ intervalDays: n });
            }}
            className={`${inputCls} w-20`}
          />
          <span className="text-xs text-carbon-textMuted">{t("cadence.daysUnit")}</span>
        </div>
      )}

      {/* Preview */}
      {state.mode !== "off" && (
        <p className="text-xs text-carbon-textMuted">
          Value:{" "}
          <span className="font-mono text-carbon-textSub">{buildCadenceString(state)}</span>
        </p>
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
function RcloneCard({ t }: { t: ReturnType<typeof useT>["t"] }) {
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
function CloudCard({ t }: { t: ReturnType<typeof useT>["t"] }) {
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
};

// NotifyCard configures backup notifications (webhook / Matrix / Healthchecks).
// Stored encrypted at rest; the form pre-fills from the saved config and Test
// sends to the CURRENT form values (no save needed).
function NotifyCard({ t }: { t: ReturnType<typeof useT>["t"] }) {
  const [cfg, setCfg] = useState<NotifyConfig>(emptyNotify);
  const [state, setState] = useState<SaveState>("idle");
  const [msg, setMsg] = useState<string | null>(null);
  const [tested, setTested] = useState(false);

  useEffect(() => {
    getNotify()
      .then((r) => {
        if (r.ok && r.notify) setCfg({ ...emptyNotify, ...r.notify });
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
            type="password" className={inputCls} />
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

// IntegrityCard runs per-domain repository maintenance: verify (restic check),
// unlock (clear stale locks), and prune (reclaim space). Self-contained.
function IntegrityCard({ t }: { t: ReturnType<typeof useT>["t"] }) {
  type ActState = "idle" | "busy" | "ok" | "fail";
  const [state, setState] = useState<Record<string, ActState>>({});
  const [msg, setMsg] = useState<Record<string, string>>({});
  const [source, setSource] = useState<RepoSource>("local");

  type Domain = "containers" | "vms" | "flash";
  type Action = "verify" | "unlock" | "prune";

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

  const domains: { key: Domain; label: string }[] = [
    { key: "containers", label: t("settings.containersEnabled") },
    { key: "vms", label: t("settings.vmsEnabled") },
    { key: "flash", label: t("settings.flashEnabled") },
  ];
  const actions: { key: Action; label: string; busy: string }[] = [
    { key: "verify", label: t("integrity.verify"), busy: t("integrity.checking") },
    { key: "unlock", label: t("integrity.unlock"), busy: "…" },
    { key: "prune", label: t("integrity.prune"), busy: "…" },
  ];

  return (
    <Card title={t("integrity.title")}>
      <p className="text-xs text-carbon-textMuted -mt-1">{t("integrity.hint")}</p>
      <div className="flex items-center gap-2">
        <span className="text-xs text-carbon-textMuted">{t("source.label")}</span>
        <SourceToggle
          source={source}
          onChange={setSource}
          disabled={Object.values(state).some((v) => v === "busy")}
        />
      </div>
      <div className="flex flex-col gap-3">
        {domains.map(({ key: domain, label }) => (
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
            {actions.map((a) =>
              state[`${domain}:${a.key}`] === "fail" ? (
                <span key={a.key} className="text-xs text-[#ff8389] break-words">
                  {a.label}: {msg[`${domain}:${a.key}`] || t("integrity.failed")}
                </span>
              ) : null
            )}
          </div>
        ))}
      </div>
    </Card>
  );
}

export function SettingsPage() {
  const { t } = useT();

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
  const [offsiteSaveState, setOffsiteSaveState] = useState<SaveState>("idle");
  const [offsiteSaveError, setOffsiteSaveError] = useState<string | null>(null);

  const [domSaveState, setDomSaveState] = useState<SaveState>("idle");
  const [domSaveError, setDomSaveError] = useState<string | null>(null);

  const [schedSaveState, setSchedSaveState] = useState<SaveState>("idle");
  const [schedSaveError, setSchedSaveError] = useState<string | null>(null);

  const [retSaveState, setRetSaveState] = useState<SaveState>("idle");
  const [retSaveError, setRetSaveError] = useState<string | null>(null);

  // "Use containers schedule for VMs and Flash too" checkbox
  const [syncSchedules, setSyncSchedules] = useState(false);

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok) {
          setSettings(res.settings);
          setSavedSettings(res.settings);
          if (res.hostMountRoot) setHostMountRoot(res.hostMountRoot);
          // Detect if schedules are already in sync
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
  }, []);

  // ---------------------------------------------------------------------------
  // Generic save helper
  // ---------------------------------------------------------------------------

  async function save(
    patch: Partial<Settings>,
    setSaveState: (s: SaveState) => void,
    setSaveError: (e: string | null) => void
  ) {
    const base = savedSettings ?? settings;
    if (!base) return;
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
      } else {
        setSaveError(res.error ?? t("settings.error"));
        setSaveState("error");
      }
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : t("settings.error"));
      setSaveState("error");
    }
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

  // Build the schedule patch (used by the Schedule save button)
  function buildSchedulePatch(): Partial<Settings> {
    const patch: Partial<Settings> = {
      containersSchedule: settings!.containersSchedule,
    };
    if (syncSchedules) {
      patch.vmsSchedule = settings!.containersSchedule;
      patch.flashSchedule = settings!.containersSchedule;
    } else {
      patch.vmsSchedule = settings!.vmsSchedule;
      patch.flashSchedule = settings!.flashSchedule;
    }
    return patch;
  }

  return (
    <div className="flex flex-col gap-6 max-w-3xl">
      {/* Page heading */}
      <div>
        <h1 className="text-2xl font-semibold text-carbon-text">
          {t("settings.title")}
        </h1>
        <p className="mt-1 text-sm text-carbon-textSub">
          BombVault configuration — changes take effect immediately.
        </p>
      </div>

      {/* ------------------------------------------------------------------ */}
      {/* Domains                                                            */}
      {/* ------------------------------------------------------------------ */}
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
        <SaveBar
          state={domSaveState}
          error={domSaveError}
          onSave={() =>
            void save(
              {
                containersEnabled: settings.containersEnabled,
                vmsEnabled: settings.vmsEnabled,
                flashEnabled: settings.flashEnabled,
              },
              setDomSaveState,
              setDomSaveError
            )
          }
          t={t}
        />
      </Card>

      {/* ------------------------------------------------------------------ */}
      {/* Schedule                                                           */}
      {/* ------------------------------------------------------------------ */}
      <Card title={t("settings.schedule")}>
        <p className="text-xs text-carbon-textMuted -mt-1">
          Configure when automatic backups run per domain.
        </p>

        {/* Containers schedule */}
        <div className="rounded-lg bg-carbon-surface2 border border-carbon-border p-4">
          <CadenceBuilder
            label="Containers"
            value={settings.containersSchedule}
            onChange={(v) =>
              setSettings((prev) =>
                prev ? { ...prev, containersSchedule: v } : prev
              )
            }
          />
        </div>

        {/* Sync checkbox */}
        <label className="flex items-center gap-2 cursor-pointer select-none">
          <input
            type="checkbox"
            checked={syncSchedules}
            onChange={(e) => setSyncSchedules(e.target.checked)}
            className="h-4 w-4 rounded border-carbon-border bg-carbon-surface2 accent-[#6fdc8c]"
          />
          <span className="text-sm text-carbon-text">
            Use the Containers schedule for VMs and Flash too
          </span>
        </label>

        {/* VMs schedule */}
        <div className={`rounded-lg bg-carbon-surface2 border border-carbon-border p-4 ${syncSchedules ? "opacity-50" : ""}`}>
          <CadenceBuilder
            label="VMs"
            value={syncSchedules ? settings.containersSchedule : settings.vmsSchedule}
            disabled={syncSchedules}
            onChange={(v) =>
              setSettings((prev) =>
                prev ? { ...prev, vmsSchedule: v } : prev
              )
            }
          />
          {!syncSchedules && (
            <p className="text-xs text-carbon-textMuted mt-2">
              Backs up every VM with “include in schedule” enabled (set it per VM in the VMs tab).
            </p>
          )}
        </div>

        {/* Flash schedule */}
        <div className={`rounded-lg bg-carbon-surface2 border border-carbon-border p-4 ${syncSchedules ? "opacity-50" : ""}`}>
          <CadenceBuilder
            label="Flash (later phase)"
            value={syncSchedules ? settings.containersSchedule : settings.flashSchedule}
            disabled={syncSchedules}
            onChange={(v) =>
              setSettings((prev) =>
                prev ? { ...prev, flashSchedule: v } : prev
              )
            }
          />
          {!syncSchedules && (
            <p className="text-xs text-carbon-textMuted mt-2">
              Note: Flash backup executor is not yet implemented in Phase 1 — schedule is stored but not executed.
            </p>
          )}
        </div>

        <SaveBar
          state={schedSaveState}
          error={schedSaveError}
          onSave={() => void save(buildSchedulePatch(), setSchedSaveState, setSchedSaveError)}
          t={t}
        />
      </Card>

      {/* ------------------------------------------------------------------ */}
      {/* Backup paths                                                       */}
      {/* ------------------------------------------------------------------ */}
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
        <SaveBar
          state={pathSaveState}
          error={pathSaveError}
          onSave={() =>
            void save(
              {
                containersPath: settings.containersPath,
                vmsPath: settings.vmsPath,
                flashPath: settings.flashPath,
              },
              setPathSaveState,
              setPathSaveError
            )
          }
          t={t}
        />
      </Card>

      {/* ------------------------------------------------------------------ */}
      {/* Off-site copy (restic copy replication)                            */}
      {/* ------------------------------------------------------------------ */}
      <Card title={t("settings.offsiteTitle")}>
        <p className="text-xs text-carbon-textMuted -mt-1">{t("settings.offsiteHint")}</p>
        {([
          ["containersOffsite", "containersOffsiteSchedule", "nav.containers", "containers"],
          ["vmsOffsite", "vmsOffsiteSchedule", "nav.vms", "vms"],
          ["flashOffsite", "flashOffsiteSchedule", "nav.flash", "flash"],
        ] as const).map(([repoKey, schedKey, label, domain]) => (
          <div key={repoKey} className="flex flex-col gap-1 border-b border-carbon-border pb-3 last:border-0">
            <div className="flex items-center justify-between">
              <span className="text-xs text-carbon-textSub">{t(label)}</span>
              {settings[repoKey] && <ReplicateNowButton domain={domain} t={t} />}
            </div>
            <input
              value={settings[repoKey]}
              spellCheck={false}
              onChange={(e) =>
                setSettings((prev) => (prev ? { ...prev, [repoKey]: e.target.value } : prev))
              }
              placeholder="rest:http://host:8000/repo"
              className="rounded-lg border border-carbon-border bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono focus:outline-none focus:ring-1 focus:ring-accent"
            />
            <input
              value={settings[schedKey]}
              spellCheck={false}
              onChange={(e) =>
                setSettings((prev) => (prev ? { ...prev, [schedKey]: e.target.value } : prev))
              }
              placeholder={t("offsite.schedulePlaceholder")}
              className="rounded-lg border border-carbon-border bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono focus:outline-none focus:ring-1 focus:ring-accent"
            />
          </div>
        ))}
        <SaveBar
          state={offsiteSaveState}
          error={offsiteSaveError}
          onSave={() =>
            void save(
              {
                containersOffsite: settings.containersOffsite,
                vmsOffsite: settings.vmsOffsite,
                flashOffsite: settings.flashOffsite,
                containersOffsiteSchedule: settings.containersOffsiteSchedule,
                vmsOffsiteSchedule: settings.vmsOffsiteSchedule,
                flashOffsiteSchedule: settings.flashOffsiteSchedule,
              },
              setOffsiteSaveState,
              setOffsiteSaveError
            )
          }
          t={t}
        />
      </Card>

      {/* ------------------------------------------------------------------ */}
      {/* Retention                                                            */}
      {/* ------------------------------------------------------------------ */}
      <Card title={t("settings.retentionTitle")}>
        <p className="text-xs text-carbon-textMuted -mt-1">
          {t("settings.retentionHint")}
        </p>
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

      {/* ------------------------------------------------------------------ */}
      {/* Encryption                                                         */}
      {/* ------------------------------------------------------------------ */}
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

      {/* ------------------------------------------------------------------ */}
      {/* VM Backup over SSH                                                 */}
      {/* ------------------------------------------------------------------ */}
      <VMSSHCard t={t} />

      {/* ------------------------------------------------------------------ */}
      {/* Off-site (rclone)                                                    */}
      {/* ------------------------------------------------------------------ */}
      <RcloneCard t={t} />

      <CloudCard t={t} />

      {/* ------------------------------------------------------------------ */}
      {/* Notifications                                                       */}
      {/* ------------------------------------------------------------------ */}
      <NotifyCard t={t} />

      {/* ------------------------------------------------------------------ */}
      {/* Spike                                                              */}
      {/* ------------------------------------------------------------------ */}
      <Card title={t("spike.title")}>
        <SpikePanel t={t} />
      </Card>

      {/* ------------------------------------------------------------------ */}
      {/* Integrity (restic check)                                            */}
      {/* ------------------------------------------------------------------ */}
      <IntegrityCard t={t} />

      {/* ------------------------------------------------------------------ */}
      {/* Security                                                           */}
      {/* ------------------------------------------------------------------ */}
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

      {/* ------------------------------------------------------------------ */}
      {/* Appearance                                                         */}
      {/* ------------------------------------------------------------------ */}
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
    </div>
  );
}
