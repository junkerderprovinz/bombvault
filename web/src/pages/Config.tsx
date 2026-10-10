import { useCallback, useEffect, useRef, useState, type CSSProperties } from "react";
import { hueVars } from "../lib/appearance";
import { backupConfigNow, getSettings } from "../lib/api";
import type { Settings, TimelineRow } from "../lib/api";
import { useT } from "../lib/i18n";
import { PAGE_SHELL_RESPONSIVE } from "../lib/pageShell";
import { BackupCancelButton } from "../components/BackupCancelButton";
import { ProgressBar } from "../components/ProgressBar";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { useBackupWatch } from "../lib/backupWatch";
import { Timeline } from "../components/timeline/Timeline";
import { useToast } from "../lib/toast";
import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { InfoBubble } from "../components/InfoBubble";
import { IconBackupNow } from "../components/Sidebar";
import { tLtr } from "../lib/ltrFragments";
import { ItemAnomalyBadge } from "../components/ItemAnomalyBadge";
import { PageTitle } from "../components/PageTitle";
import { ItemChecksLine } from "../components/ItemChecksLine";
import { useItemChecks } from "../lib/useItemChecks";
import { ItemAnomalySettings } from "../components/ItemAnomalySettings";
import { useAnomalyItems, useAnomalySummary, useOpenAnomalies } from "../lib/useAnomalies";
import { useRestoreRequest } from "../lib/restoreRequest";
import { ConfigSettingsCard } from "../components/config/ConfigSettingsCard";

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

// Config is the self-backup page for BombVault's own settings: backups and the
// snapshot list. Restoring restarts the app to swap the live database, so that
// stays in the Recovery tab.
export function Config() {
  const { t } = useT();
  const [settings, setSettings] = useState<Settings | null>(null);
  const anomaly = useAnomalyItems().find("config", "config");
  const itemChecks = useItemChecks();
  const anomalyEnabled = useAnomalySummary().summary?.enabled ?? false;
  const { flagged } = useOpenAnomalies();
  const restoreRequest = useRestoreRequest();
  const [reloadTick, setReloadTick] = useState(0);
  const [hasBackup, setHasBackup] = useState(false);
  const onRows = useCallback((rows: TimelineRow[]) => setHasBackup(rows.length > 0), []);
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
    <div className={PAGE_SHELL_RESPONSIVE}>
      <PageTitle>{t("config.title")}</PageTitle>

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
          {/* The page's heading is only read out, so the learning caption sits
              with the backups it counts. */}
          <div className="flex flex-wrap items-center justify-end gap-3">
            <span className="me-auto">
              <ItemAnomalyBadge item={anomaly} enabled={anomalyEnabled} t={t} />
            </span>
            <ConfigBackupButton
              t={t}
              onBackedUp={() => setReloadTick((n) => n + 1)}
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
          <ItemChecksLine checks={itemChecks.find("config", "config")} hasBackup={hasBackup} onChanged={itemChecks.reload} />

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
            onRows={onRows}
            flagged={flagged}
            request={restoreRequest}
          />
        </div>
      </div>
    </div>
  );
}
