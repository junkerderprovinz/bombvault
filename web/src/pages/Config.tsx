import { useEffect, useState } from "react";
import {
  backupConfigNow,
  listConfigSnapshots,
  deleteSnapshot,
  getSettings,
  putSettings,
} from "../lib/api";
import type { Snapshot, Settings } from "../lib/api";
import { useT } from "../lib/i18n";
import { ProgressBar } from "../components/ProgressBar";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { useBackupWatch } from "../lib/backupWatch";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { ToggleRow } from "./Settings";

type T = ReturnType<typeof useT>["t"];

// ---------------------------------------------------------------------------
// Backup button — fire-and-watch, mirroring the flash domain (see useBackupWatch:
// the config backup runs detached on the server and the POST returns immediately,
// so we watch the "config" progress + recorded run for the outcome).
// ---------------------------------------------------------------------------

function ConfigBackupButton({
  t,
  onBackedUp,
  externallyBusy = false,
  busyPhase,
}: {
  t: T;
  onBackedUp: () => void;
  /** True when a backup/restore is running elsewhere (any domain). */
  externallyBusy?: boolean;
  busyPhase?: string;
}) {
  const { state, fire, isPending } = useBackupWatch({
    progressKey: "config",
    start: () => backupConfigNow(),
    matchRun: (r) => r.domain === "config",
    onDone: onBackedUp,
  });

  return (
    <div className="flex flex-col gap-1 items-start">
      <button
        onClick={() => void fire()}
        disabled={isPending || externallyBusy}
        className="inline-flex items-center gap-1.5 rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
      >
        {isPending ? (
          <>
            <span
              className="h-3.5 w-3.5 rounded-full border-2 border-t-transparent animate-spin inline-block"
              style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
            />
            {t("config.backingUp")}
          </>
        ) : (
          t("config.backupNow")
        )}
      </button>
      {/* A backup/restore/replication elsewhere blocks a new config backup. */}
      {externallyBusy && !isPending && (
        <span className="text-xs text-carbon-textMuted">
          {t(busyPhraseKey(busyPhase))}
        </span>
      )}
      {state.phase === "success" && (
        <span className="text-xs text-[#6fdc8c]">
          ✓ {t("settings.saved")}
          {state.snapshotId && (
            <span className="font-mono ml-1 text-carbon-textMuted">{state.snapshotId.slice(0, 8)}</span>
          )}
        </span>
      )}
      {state.phase === "error" && (
        <span className="text-xs text-[#ff8389] max-w-[28rem] break-words">{state.message}</span>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Settings card — the config self-backup is configured on its own page (unlike
// flash, whose enable/path/off-site live on the Settings page): one place to say
// "protect BombVault itself". Persists via getSettings/putSettings — the same
// mechanism the rest of the app uses; no new persistence is invented.
// ---------------------------------------------------------------------------

type SaveState = "idle" | "saving" | "saved" | "error";

function labelledInput(
  label: string,
  value: string,
  onChange: (v: string) => void,
  placeholder: string,
  hint?: string
) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs text-carbon-textSub">{label}</span>
      <input
        value={value}
        spellCheck={false}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="rounded-lg border border-carbon-border bg-carbon-surface2 px-3 py-2 text-sm text-carbon-text font-mono focus:outline-none focus:ring-1 focus:ring-accent"
      />
      {hint && <p className="text-xs text-carbon-textMuted">{hint}</p>}
    </div>
  );
}

function ConfigSettingsCard({
  t,
  settings,
  setSettings,
}: {
  t: T;
  settings: Settings;
  setSettings: (updater: (prev: Settings) => Settings) => void;
}) {
  const [saveState, setSaveState] = useState<SaveState>("idle");
  const [saveError, setSaveError] = useState<string | null>(null);

  async function handleSave() {
    setSaveState("saving");
    setSaveError(null);
    try {
      // Re-fetch the latest settings and merge only the fields THIS card owns,
      // then PUT. Since the self-backup + off-site cadences moved to Settings ›
      // Schedules (the sole schedule owner), a full-object PUT of this page's
      // mount-time snapshot could otherwise re-assert a stale configSchedule/
      // configOffsiteSchedule and silently disable a schedule set elsewhere.
      const latest = await getSettings();
      // Do NOT fall back to the stale mount-time snapshot on a failed re-fetch:
      // the backend returns {ok:false} at HTTP 200 (does not throw), and PUTting
      // the old snapshot would re-assert a stale configSchedule/configOffsiteSchedule
      // now owned by Settings › Schedules, silently reverting a schedule set
      // elsewhere. Abort the save instead.
      if (!latest.ok) {
        setSaveState("error");
        setSaveError(latest.error ?? "Could not load current settings");
        return;
      }
      const merged: Settings = {
        ...latest.settings,
        configEnabled: settings.configEnabled,
        configPath: settings.configPath,
        configOffsite: settings.configOffsite,
        configOffsiteImmutable: settings.configOffsiteImmutable,
      };
      const res = await putSettings(merged);
      if (res.ok) {
        setSaveState("saved");
        setTimeout(() => setSaveState("idle"), 3000);
      } else {
        setSaveState("error");
        setSaveError(res.error ?? "Save failed");
      }
    } catch (err) {
      setSaveState("error");
      setSaveError(err instanceof Error ? err.message : "Save failed");
    }
  }

  return (
    <div className="bg-carbon-surface rounded-card border border-carbon-border p-5 flex flex-col gap-4">
      <h2 className="text-sm font-semibold text-carbon-textSub uppercase tracking-widest">
        {t("config.settingsTitle")}
      </h2>
      <p className="text-xs text-carbon-textMuted -mt-1">{t("config.settingsHint")}</p>

      <ToggleRow
        label={t("config.enabled")}
        description={t("config.enabledHint")}
        checked={settings.configEnabled}
        onChange={(v) => setSettings((prev) => ({ ...prev, configEnabled: v }))}
      />

      {labelledInput(
        t("config.path"),
        settings.configPath,
        (v) => setSettings((prev) => ({ ...prev, configPath: v })),
        "user/bombvault/config",
        t("config.pathHint")
      )}

      {/* The self-backup + off-site cadences moved to Settings › Schedules (the
          single schedule owner). Only path / off-site repo / immutable live here. */}
      {labelledInput(
        t("config.offsite"),
        settings.configOffsite,
        (v) => setSettings((prev) => ({ ...prev, configOffsite: v })),
        "rest:http://host:8000/repo",
        t("config.offsiteHint")
      )}

      <ToggleRow
        label={t("config.immutable")}
        description={t("config.immutableHint")}
        checked={settings.configOffsiteImmutable}
        onChange={(v) => setSettings((prev) => ({ ...prev, configOffsiteImmutable: v }))}
      />

      <div className="flex items-center gap-3 pt-1">
        <button
          onClick={() => void handleSave()}
          disabled={saveState === "saving"}
          className="inline-flex items-center gap-2 rounded-lg bg-accent px-4 py-1.5 text-sm font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50"
        >
          {saveState === "saving" ? (
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
        {saveState === "saved" && (
          <span className="text-sm text-[#6fdc8c]">{t("settings.saved")}</span>
        )}
        {saveState === "error" && saveError && (
          <span className="text-sm text-[#ff8389]">{saveError}</span>
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Snapshot row — id + timestamp, with a delete affordance (mirrors Flash's
// FlashSnapshotRow, minus the zip download). Delete targets the currently viewed
// repo via `source`; an off-site append-only repo refuses the delete server-side,
// so the returned error is surfaced in `deleteErr` rather than hidden.
// ---------------------------------------------------------------------------

function ConfigSnapshotRow({
  snap,
  source,
  onDeleted,
  t,
}: {
  snap: Snapshot;
  source: RepoSource;
  onDeleted: () => void;
  t: T;
}) {
  const [deleting, setDeleting] = useState(false);
  const [deleteErr, setDeleteErr] = useState<string | null>(null);

  async function handleDelete() {
    if (!window.confirm(t("snapshots.deleteConfirm"))) return;
    setDeleting(true);
    setDeleteErr(null);
    try {
      const res = await deleteSnapshot("config", snap.id, source);
      if (res.ok) onDeleted();
      else setDeleteErr(res.error ?? "Delete failed");
    } catch (err) {
      setDeleteErr(err instanceof Error ? err.message : "Delete failed");
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div className="flex flex-col gap-1 py-2.5 border-b border-carbon-border last:border-0">
      <div className="flex items-center gap-3 text-sm">
        <span className="font-mono text-carbon-text text-xs w-20 shrink-0">{snap.id.slice(0, 8)}</span>
        <span className="text-carbon-textMuted text-xs flex-1">
          {new Date(snap.time).toLocaleString()}
        </span>
        <button
          onClick={() => void handleDelete()}
          disabled={deleting}
          title={t("snapshots.delete")}
          className="shrink-0 rounded-lg border border-carbon-border px-2 py-1 text-xs text-carbon-textSub hover:bg-[#3a1c1c] hover:text-[#ff8389] transition-colors disabled:opacity-50"
        >
          {deleting ? "…" : t("snapshots.delete")}
        </button>
      </div>
      {deleteErr && <p className="text-xs text-[#ff8389] pl-24 break-words">{deleteErr}</p>}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Config page — BombVault's OWN settings self-backup. Backup + status only; the
// restore flow (which restarts the app to swap the live DB) lives in the Recovery
// tab, so the self-referential restart stays in one place.
// ---------------------------------------------------------------------------

export function Config() {
  const { t } = useT();
  const [settings, setSettings] = useState<Settings | null>(null);
  const [source, setSource] = useState<RepoSource>("local");
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const progressMap = useProgress();
  const progress = progressMap["config"];
  // Any backup/restore/replication in flight (any domain) disables the config
  // backup button + shows a hint, instead of relying on the 409 round-trip.
  const running = anyActive(progressMap);

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok) setSettings(res.settings);
      })
      .catch(() => undefined);
  }, []);

  function load() {
    setError(null);
    return listConfigSnapshots(source)
      .then((res) => {
        if (res.ok) setSnapshots(res.snapshots ?? []);
        else setError(res.error ?? "Failed to load config backups");
      })
      .catch((err: unknown) =>
        setError(err instanceof Error ? err.message : "Failed to load config backups")
      );
  }

  useEffect(() => {
    setLoading(true);
    void load().finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [source]);

  return (
    <div className="flex flex-col gap-6 max-w-3xl">
      {/* Page heading */}
      <div>
        <h1 className="text-2xl font-semibold text-carbon-text">{t("config.title")}</h1>
        <p className="mt-1 text-sm text-carbon-textSub">{t("config.subtitle")}</p>
      </div>

      {/* Settings card */}
      {settings && (
        <ConfigSettingsCard t={t} settings={settings} setSettings={(u) => setSettings((prev) => (prev ? u(prev) : prev))} />
      )}

      {/* Backup card */}
      <div className="relative overflow-hidden bg-carbon-surface rounded-card border border-carbon-border p-5 flex flex-col gap-4">
        <h2 className="text-sm font-semibold text-carbon-textSub uppercase tracking-widest">
          {t("config.backupTitle")}
        </h2>
        <p className="text-xs text-carbon-textMuted -mt-1">{t("config.backupHint")}</p>
        <ConfigBackupButton
          t={t}
          onBackedUp={() => void load()}
          externallyBusy={running.active}
          busyPhase={running.phase}
        />

        {/* Live backup/restore progress, pinned to the card's bottom edge */}
        {progress && (
          <ProgressBar percent={progress.percent} active={progress.active} />
        )}
      </div>

      {/* Snapshots card — list + delete; restoring settings lives in Recovery. */}
      <div className="bg-carbon-surface rounded-card border border-carbon-border p-5 flex flex-col gap-4">
        <h2 className="text-sm font-semibold text-carbon-textSub uppercase tracking-widest">
          {t("config.snapshotsTitle")}
        </h2>
        <div className="rounded-lg bg-[#1c2a3a] border border-[#2a4055] px-3 py-2.5 text-xs text-[#78a9ff] leading-relaxed">
          {t("config.snapshotsHint")}
        </div>

        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-2">
            <span className="text-xs text-carbon-textMuted">{t("source.label")}</span>
            <SourceToggle source={source} onChange={setSource} disabled={loading} />
          </div>
          <p className="text-[11px] text-carbon-textMuted">{t("source.hint")}</p>
        </div>

        {loading && <p className="text-xs text-carbon-textMuted">{t("dashboard.checking")}</p>}
        {error && <p className="text-xs text-[#ff8389]">{error}</p>}
        {!loading && !error && snapshots.length === 0 && (
          <p className="text-xs text-carbon-textMuted">{t("config.none")}</p>
        )}
        {!loading && snapshots.length > 0 && (
          <div className="rounded-lg border border-carbon-border bg-carbon-background px-3 py-1">
            {snapshots.map((snap) => (
              <ConfigSnapshotRow
                key={snap.id}
                snap={snap}
                source={source}
                onDeleted={() => void load()}
                t={t}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
