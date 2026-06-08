import { useEffect, useState } from "react";
import { getSettings, putSettings } from "../lib/api";
import type { Settings } from "../lib/api";
import { useT } from "../lib/i18n";
import { SpikePanel } from "../components/SpikePanel";

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
          className={`inline-block h-3.5 w-3.5 rounded-full bg-[#161616] transition-transform ${
            checked ? "translate-x-[18px]" : "translate-x-[3px]"
          }`}
        />
      </button>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Path row with preview
// ---------------------------------------------------------------------------

const HOST_MOUNT_ROOT = "/host/user"; // kept in sync with server default

function PathRow({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
}) {
  const trimmed = value.trim();
  const resolved =
    trimmed && !trimmed.startsWith("/") && !trimmed.includes("..")
      ? `${HOST_MOUNT_ROOT}/${trimmed}`
      : "";

  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-xs text-carbon-textSub">{label}</label>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        spellCheck={false}
        placeholder="backups/bombvault/containers"
        className="rounded-lg bg-carbon-surface2 border border-carbon-border text-carbon-text text-sm font-mono px-3 py-1.5 focus:outline-none focus:border-[#78a9ff] w-full"
      />
      {resolved && (
        <p className="text-xs text-carbon-textMuted font-mono">
          → {resolved}
        </p>
      )}
      {!resolved && trimmed && (
        <p className="text-xs text-[#ff8389]">
          Path must be a relative subpath (no leading / or ..)
        </p>
      )}
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
        className="inline-flex items-center gap-2 rounded-lg bg-carbon-surface3 px-4 py-1.5 text-sm font-medium text-carbon-text hover:bg-carbon-hover transition-colors disabled:opacity-50"
      >
        {state === "saving" ? (
          <>
            <span className="h-3.5 w-3.5 rounded-full border-2 border-[#78a9ff] border-t-transparent animate-spin" />
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
// Settings page
// ---------------------------------------------------------------------------

export function SettingsPage() {
  const { t } = useT();

  const [settings, setSettings] = useState<Settings | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);

  // Per-section save state
  const [encSaveState, setEncSaveState] = useState<SaveState>("idle");
  const [encSaveError, setEncSaveError] = useState<string | null>(null);

  const [pathSaveState, setPathSaveState] = useState<SaveState>("idle");
  const [pathSaveError, setPathSaveError] = useState<string | null>(null);

  const [domSaveState, setDomSaveState] = useState<SaveState>("idle");
  const [domSaveError, setDomSaveError] = useState<string | null>(null);

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok) setSettings(res.settings);
        else setLoadError("Failed to load settings");
      })
      .catch(() => setLoadError("Failed to load settings"));
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
          <span className="font-mono">{HOST_MOUNT_ROOT}</span>). The resolved
          absolute path is shown as a preview.
        </p>
        <PathRow
          label={t("settings.containersPath")}
          value={settings.containersPath}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, containersPath: v } : prev)
          }
        />
        <PathRow
          label={t("settings.vmsPath")}
          value={settings.vmsPath}
          onChange={(v) =>
            setSettings((prev) => prev ? { ...prev, vmsPath: v } : prev)
          }
        />
        <PathRow
          label={t("settings.flashPath")}
          value={settings.flashPath}
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
    </div>
  );
}
