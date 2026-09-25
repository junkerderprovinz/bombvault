import { useEffect, useRef, useState, type CSSProperties } from "react";
import { hueVars } from "../lib/appearance";
import {
  backupConfigNow,
  listConfigSnapshots,
  deleteSnapshot,
  getSettings,
  putSettings,
} from "../lib/api";
import type { Snapshot, Settings } from "../lib/api";
import { useT } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";
import { BackupCancelButton } from "../components/BackupCancelButton";
import { ProgressBar } from "../components/ProgressBar";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { useBackupWatch } from "../lib/backupWatch";
import { SourceToggle, type RepoSource } from "../components/SourceToggle";
import { ToggleRow } from "./settings/shared";
import { useConfirm } from "../lib/useConfirm";
import { useToast } from "../lib/toast";
import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { InfoBubble } from "../components/InfoBubble";
import { IconBackupNow, IconTrash } from "../components/Sidebar";
import { tLtr } from "../lib/ltrFragments";
import { ItemAnomalyBadge } from "../components/ItemAnomalyBadge";
import { ItemAnomalySettings } from "../components/ItemAnomalySettings";
import { MissingRestorePoint, restorePointOf } from "../components/restore/MissingRestorePoint";
import { findingSnapshotId } from "../lib/anomalies";
import { useAnomalyItems, useAnomalySummary, useOpenAnomalies } from "../lib/useAnomalies";
import { useRestoreRequest } from "../lib/restoreRequest";

type T = ReturnType<typeof useT>["t"];

// ConfigBackupButton starts a config backup and follows it through
// useBackupWatch, since the server runs it detached and the POST returns at
// once. The outcome is a toast; a failure also shakes the button.
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
  // A backup/restore/replication elsewhere blocks a new config backup.
  const blockedByOther = externallyBusy && !isPending;
  const { push } = useToast();
  // Used as the button's key, so each failure remounts it and replays the shake.
  const [shake, setShake] = useState(0);
  // Toast once per new terminal phase. state.phase starts at "idle", so nothing
  // fires on mount.
  const seenPhase = useRef(state.phase);

  useEffect(() => {
    if (state.phase === seenPhase.current) return;
    seenPhase.current = state.phase;
    if (state.phase === "success") {
      push(
        state.snapshotId ? `${t("settings.saved")} · ${state.snapshotId.slice(0, 8)}` : t("settings.saved"),
        "success"
      );
    } else if (state.phase === "error") {
      push(state.message, "fail");
      setShake((n) => n + 1);
    }
  }, [state, push, t]);

  // The label stays fixed; pending and blocked states only show as a tooltip.
  const stateTip = isPending
    ? t("config.backingUp")
    : blockedByOther
      ? t(busyPhraseKey(busyPhase))
      : undefined;

  return (
    <Button
      key={shake}
      label={t("config.backupNow")}
      labelKey="config.backupNow"
      glyph={<IconBackupNow />}
      tone="accent"
      onClick={() => void fire()}
      disabled={isPending || blockedByOther}
      busy={isPending}
      title={stateTip}
      className={shake ? "glim-shake" : ""}
    />
  );
}

type SaveState = "idle" | "saving";

// ConfigSettingsCard holds the self-backup toggle. Path, schedules and off-site
// settings for self-backup are edited in Settings, not here.
function ConfigSettingsCard({
  t,
  settings,
  setSettings,
  hueIndex,
}: {
  t: T;
  settings: Settings;
  setSettings: (updater: (prev: Settings) => Settings) => void;
  /** Rainbow position for this card's heading notch. */
  hueIndex?: number;
}) {
  const { push } = useToast();
  const [, setSaveState] = useState<SaveState>("idle");

  async function persist(enabled: boolean) {
    setSaveState("saving");
    try {
      // Merge into freshly fetched settings: a PUT of this page's mount-time
      // copy would revert schedule, off-site and path changes made in Settings.
      const latest = await getSettings();
      // A failed fetch comes back as {ok:false}, not a throw. Saving the stale
      // copy instead would cause exactly that revert, so give up.
      if (!latest.ok) {
        setSaveState("idle");
        push(latest.error ?? t("config.loadSettingsFailed"), "fail");
        return;
      }
      const merged: Settings = {
        ...latest.settings,
        configEnabled: enabled,
      };
      const res = await putSettings(merged);
      if (res.ok) {
        setSaveState("idle");
        push(t("settings.saved"), "success");
      } else {
        setSaveState("idle");
        push(res.error ?? t("common.saveFailed"), "fail");
      }
    } catch (err) {
      setSaveState("idle");
      push(err instanceof Error ? err.message : t("common.saveFailed"), "fail");
    }
  }

  return (
    // glim-notch-card reveals the heading notch colour when the pointer or
    // focus is anywhere in the card; glim-hue sets the rainbow accent for
    // everything inside it.
    <div
      className={`relative glim-notch-card bg-carbon-surface rounded-card p-5 flex flex-col gap-4${
        hueIndex !== undefined ? " glim-hue" : ""
      }`}
      style={hueIndex !== undefined ? (hueVars(hueIndex) as CSSProperties) : undefined}
    >
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap hueIndex={hueIndex}>
          {t("config.settingsTitle")}
          <InfoBubble tip={t("config.settingsHint")} onAccent />
        </Badge>
      </h2>

      {/* Saves on change, like every other toggle in the app. */}
      <ToggleRow
        label={t("config.enabled")}
        hint={tLtr(t, "config.enabledHint")}
        checked={settings.configEnabled}
        onChange={(v) => {
          setSettings((prev) => ({ ...prev, configEnabled: v }));
          void persist(v);
        }}
      />

      <p className="text-xs text-carbon-textMuted">{t("config.pathMoved")}</p>

      <p className="text-xs text-carbon-textMuted">{t("config.offsiteMoved")}</p>

    </div>
  );
}

// ConfigSnapshotRow shows one snapshot with a delete button. The delete goes to
// the repo selected by source; an append-only off-site repo refuses it, and the
// error is toasted.
function ConfigSnapshotRow({
  snap,
  source,
  flagged,
  preselected,
  onDeleted,
  t,
}: {
  snap: Snapshot;
  source: RepoSource;
  /** An open data-loss finding was raised on this snapshot. */
  flagged: boolean;
  /** A finding's restore link asked for this snapshot, so the row stands out
   *  from its neighbours. */
  preselected: boolean;
  onDeleted: () => void;
  t: T;
}) {
  const [deleting, setDeleting] = useState(false);
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [shake, setShake] = useState(0);

  async function handleDelete() {
    if (!(await confirm(t("snapshots.deleteConfirm"), { confirmKey: "snapshots.delete" }))) return;
    setDeleting(true);
    try {
      const res = await deleteSnapshot("config", snap.id, source);
      if (res.ok) onDeleted();
      else {
        push(res.error ?? t("common.deleteFailed"), "fail");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.deleteFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setDeleting(false);
    }
  }

  return (
    // py-1.5 keeps the row at 44px around the 32px delete badge, as in
    // RestorePanel's SnapshotRow.
    <div
      className={`flex flex-col gap-1 py-1.5 border-b border-carbon-border last:border-0${
        preselected ? " bg-carbon-surface2 px-2 rounded-control" : ""
      }`}
    >
      <div className="flex items-center gap-3 text-sm">
        <span dir="ltr" className="font-mono text-start text-carbon-text text-xs w-20 shrink-0">{snap.id.slice(0, 8)}</span>
        <span className="text-carbon-textMuted text-xs flex-1">
          {new Date(snap.time).toLocaleString()}
        </span>
        {flagged && (
          <Badge tone="fail" size="small">
            {t("anomaly.snapshotFlagged")}
          </Badge>
        )}
        {/* Accent rather than red: the glyph, the label and the confirm dialog
            already mark the action as destructive. */}
        <Button
          key={shake}
          label={t("snapshots.delete")}
          labelKey="snapshots.delete"
          glyph={<IconTrash />}
          tone="accent"
          onClick={() => void handleDelete()}
          disabled={deleting}
          className={`shrink-0${shake ? " glim-shake" : ""}`}
        />
      </div>
      {confirmDialog}
    </div>
  );
}

// Config is the self-backup page for BombVault's own settings: backups and the
// snapshot list. Restoring restarts the app to swap the live database, so that
// stays in the Recovery tab.
export function Config() {
  const { t } = useT();
  const [settings, setSettings] = useState<Settings | null>(null);
  const anomaly = useAnomalyItems().find("config", "config");
  const anomalyEnabled = useAnomalySummary().summary?.enabled ?? false;
  const { flagged } = useOpenAnomalies();
  const restoreRequest = useRestoreRequest();
  const [source, setSource] = useState<RepoSource>("local");
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const progressMap = useProgress();
  const progress = progressMap["config"];
  // Any backup, restore or replication in flight disables the backup button up
  // front instead of relying on the 409 round-trip.
  const running = anyActive(progressMap);

  useEffect(() => {
    getSettings()
      .then((res) => {
        if (res.ok) setSettings(res.settings);
      })
      .catch(() => undefined);
  }, []);

  const [reloadTick, setReloadTick] = useState(0);
  const reload = () => setReloadTick((n) => n + 1);

  // A switch of source hides the list until the new one is in (the toggle sets
  // `loading`); a reload after a backup or a delete swaps it in place. An
  // answer that arrives after the next switch is dropped, and a failure
  // empties the list, whose rows belong to the source just left. t() only
  // builds the failure message, so a language switch fetches nothing.
  useEffect(() => {
    let current = true;
    const fail = (message: string) => {
      if (!current) return;
      setSnapshots([]);
      setError(message);
    };
    setError(null);
    listConfigSnapshots(source)
      .then((res) => {
        if (!res.ok) return fail(res.error ?? t("config.loadBackupsFailed"));
        if (current) setSnapshots(res.snapshots ?? []);
      })
      .catch((err: unknown) => fail(err instanceof Error ? err.message : t("config.loadBackupsFailed")))
      .finally(() => {
        if (current) setLoading(false);
      });
    return () => {
      current = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [source, reloadTick]);

  return (
    <div className={PAGE_SHELL}>
      <div>
        <div className="flex flex-wrap items-center gap-2">
          <h1 className="text-2xl font-semibold text-carbon-text">{t("config.title")}</h1>
          <ItemAnomalyBadge item={anomaly} enabled={anomalyEnabled} t={t} />
        </div>
        <p className="mt-1 text-sm text-carbon-textSub">{t("config.subtitle")}</p>
      </div>

      {settings && (
        <ConfigSettingsCard t={t} settings={settings} setSettings={(u) => setSettings((prev) => (prev ? u(prev) : prev))} hueIndex={0} />
      )}

      {/* The heading badge pokes out above the card, so it lives on this outer
          div rather than inside the overflow-hidden box that ProgressBar clips
          to. insetStart={5} lines it up with that box's padding. */}
      <div className="relative glim-notch-card glim-hue" style={hueVars(1) as CSSProperties}>
        <h2 className="flex items-center">
          <Badge tone="heading" size="heading" wrap hueIndex={1} insetStart={5}>
            {t("config.backupTitle")}
            <InfoBubble tip={tLtr(t, "config.backupHint")} onAccent />
          </Badge>
        </h2>
        <div className="relative overflow-hidden bg-carbon-surface rounded-card p-5 flex flex-col gap-4">
          <div className="flex justify-end">
            <ConfigBackupButton
              t={t}
              onBackedUp={reload}
              externallyBusy={running.active}
              busyPhase={running.phase}
            />
          </div>
          <ItemAnomalySettings
            item={anomaly}
            enabled={anomalyEnabled}
            globals={
              settings
                ? { sensitivity: settings.anomalySensitivity, notifyMin: settings.anomalyNotifyMin }
                : undefined
            }
            t={t}
          />

          {/* A restore has its own control with its own warning. */}
          {progress && progress.active && progress.phase !== "restore" && (
            <div className="flex justify-end">
              <BackupCancelButton cancelKey={"config"} name={t("nav.config")} t={t} />
            </div>
          )}

          {progress && (
            <ProgressBar percent={progress.percent} active={progress.active} />
          )}
        </div>
      </div>

      <div
        className="relative glim-notch-card glim-hue bg-carbon-surface rounded-card p-5 flex flex-col gap-4"
        style={hueVars(2) as CSSProperties}
      >
        <h2 className="flex items-center">
          <Badge tone="heading" size="heading" wrap hueIndex={2}>
            {t("config.snapshotsTitle")}
            <InfoBubble tip={t("config.snapshotsHint")} onAccent />
          </Badge>
        </h2>

        <div className="flex items-center gap-2">
          <span className="flex items-center gap-1 text-xs text-carbon-textMuted">
            {t("source.label")}
            <InfoBubble tip={t("source.hint")} />
          </span>
          <SourceToggle
            source={source}
            onChange={(next) => {
              // The selector reports a click on the active source too, and
              // no load would follow to clear `loading`.
              if (next === source) return;
              setLoading(true);
              setSource(next);
            }}
            disabled={loading}
            domain="config"
          />
        </div>

        {loading && <p className="text-xs text-carbon-textMuted">{t("dashboard.checking")}</p>}
        {error && <p className="text-xs text-statusFail">{error}</p>}
        {!loading && !error && (
          <MissingRestorePoint
            requested={restoreRequest.snapshot}
            requestedAt={restoreRequest.at}
            points={snapshots.map(restorePointOf)}
            t={t}
          />
        )}
        {!loading && !error && snapshots.length === 0 && (
          <p className="text-xs text-carbon-textMuted">{t("config.none")}</p>
        )}
        {!loading && snapshots.length > 0 && (
          <div className="rounded-card bg-carbon-background px-3 py-1">
            {snapshots.map((snap) => (
              <ConfigSnapshotRow
                key={snap.id}
                snap={snap}
                source={source}
                flagged={flagged.has(findingSnapshotId(snap))}
                preselected={findingSnapshotId(snap) === restoreRequest.snapshot}
                onDeleted={reload}
                t={t}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
