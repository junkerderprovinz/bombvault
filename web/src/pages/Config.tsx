import { useEffect, useRef, useState, type CSSProperties } from "react";
import { hueVars } from "../lib/appearance";
import { backupConfigNow, getSettings, putSettings } from "../lib/api";
import type { Settings } from "../lib/api";
import { useT } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";
import { BackupCancelButton } from "../components/BackupCancelButton";
import { ProgressBar } from "../components/ProgressBar";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { useBackupWatch } from "../lib/backupWatch";
import { PlacementFlow } from "../components/placement/PlacementFlow";
import { Timeline } from "../components/timeline/Timeline";
import { ToggleRow } from "./settings/shared";
import { useToast } from "../lib/toast";
import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { InfoBubble } from "../components/InfoBubble";
import { IconBackupNow } from "../components/Sidebar";
import { tLtr } from "../lib/ltrFragments";

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
      <PlacementFlow domain="config" />

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
  const [reloadTick, setReloadTick] = useState(0);
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

  return (
    <div className={PAGE_SHELL}>
      <div>
        <h1 className="text-2xl font-semibold text-carbon-text">{t("config.title")}</h1>
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
              onBackedUp={() => setReloadTick((n) => n + 1)}
              externallyBusy={running.active}
              busyPhase={running.phase}
            />
          </div>

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

        <div className="rounded-card bg-carbon-background px-3 py-1">
          <Timeline
            key={reloadTick}
            domain="config"
            itemKey="config"
            itemName={t("config.snapshotsTitle")}
            open
            renderActions={() => null}
          />
        </div>
      </div>
    </div>
  );
}
