import { useEffect, useState, useCallback } from "react";
import { getSettings, putSettings, browse, getAuth, setAuthPassword, logout } from "../lib/api";
import type { Settings } from "../lib/api";
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
            Saving…
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
          setBrowseError(res.error ?? "Could not read directory");
          setManualFallback(true);
          return;
        }
        setDirs(res.dirs ?? []);
        setBrowsePath(path);
      })
      .catch((err: unknown) => {
        const msg = err instanceof Error ? err.message : "Browse failed";
        setBrowseError(msg);
        setManualFallback(true);
      })
      .finally(() => setLoading(false));
  }, []);

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
          placeholder="backups/bombvault/containers"
          className="flex-1 rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm font-mono px-3 py-1.5 focus:outline-none focus:border-[#78a9ff]"
        />
        <button
          onClick={handleOpen}
          title="Browse folders"
          className="shrink-0 rounded-lg bg-carbon-surface2 border border-carbon-border px-3 py-1.5 text-xs text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors"
        >
          Browse…
        </button>
      </div>

      {/* Absolute path preview */}
      {resolved && (
        <p className="text-xs text-carbon-textMuted font-mono">→ {resolved}</p>
      )}
      {!resolved && trimmed && (
        <p className="text-xs text-[#ff8389]">
          Path must be a relative subpath (no leading / or ..)
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
              Loading…
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
                <p className="text-xs text-carbon-textMuted px-2">No subdirectories</p>
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
                Use this folder
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
            {m === "off" ? "Off" : m === "daily" ? "Daily" : m === "weekly" ? "Weekly" : "Every N days"}
          </button>
        ))}
      </div>

      {/* Time picker — shown for all non-off modes */}
      {state.mode !== "off" && (
        <div className="flex items-center gap-3">
          <label className="text-xs text-carbon-textMuted w-16">Time</label>
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
          <label className="text-xs text-carbon-textMuted w-16">Days</label>
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
          <label className="text-xs text-carbon-textMuted w-16">Every</label>
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
          <span className="text-xs text-carbon-textMuted">days</span>
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

export function SettingsPage() {
  const { t } = useT();

  const [settings, setSettings] = useState<Settings | null>(null);
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

  const [domSaveState, setDomSaveState] = useState<SaveState>("idle");
  const [domSaveError, setDomSaveError] = useState<string | null>(null);

  const [schedSaveState, setSchedSaveState] = useState<SaveState>("idle");
  const [schedSaveError, setSchedSaveError] = useState<string | null>(null);

  // "Use containers schedule for VMs and Flash too" checkbox
  const [syncSchedules, setSyncSchedules] = useState(false);

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok) {
          setSettings(res.settings);
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
    if (!settings) return;
    setSaveState("saving");
    setSaveError(null);
    const updated: Settings = { ...settings, ...patch };
    try {
      const res = await putSettings(updated);
      if (res.ok) {
        setSettings(updated);
        setSaveState("saved");
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
      {/* Appearance                                                           */}
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
                    Reset
                  </button>
                )}
              </div>
            </div>
          </div>
        </div>
      </Card>

      {/* ------------------------------------------------------------------ */}
      {/* Encryption                                                           */}
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
      {/* Backup paths                                                         */}
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
      {/* Schedule                                                             */}
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
            label="VMs (later phase)"
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
              Note: VM backup executor is not yet implemented in Phase 1 — schedule is stored but not executed.
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
      {/* Domains                                                              */}
      {/* ------------------------------------------------------------------ */}
      <Card title={t("settings.domains")}>
        <p className="text-xs text-carbon-textMuted -mt-1">
          Enabling a domain reveals its navigation tab in the sidebar.
          Containers is always shown.
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
          description="VM backup via libvirt + qemu-img (coming soon)"
          checked={settings.vmsEnabled}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, vmsEnabled: v } : prev)
          }
        />
        <ToggleRow
          label={t("settings.flashEnabled")}
          description="Unraid flash drive backup (coming soon)"
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
      {/* Spike                                                                */}
      {/* ------------------------------------------------------------------ */}
      <Card title={t("spike.title")}>
        <SpikePanel t={t} />
      </Card>

      {/* ------------------------------------------------------------------ */}
      {/* Security                                                             */}
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
    </div>
  );
}
