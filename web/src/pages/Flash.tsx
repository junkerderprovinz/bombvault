import { useEffect, useRef, useState, type CSSProperties } from "react";
import { hueVars } from "../lib/appearance";
import { backupFlashNow, flashDownloadURL } from "../lib/api";
import { useT } from "../lib/i18n";
import { PAGE_SHELL } from "../lib/pageShell";
import { BackupCancelButton } from "../components/BackupCancelButton";
import { ProgressBar } from "../components/ProgressBar";
import { useProgress, anyActive, busyPhraseKey } from "../lib/progress";
import { useBackupWatch } from "../lib/backupWatch";
import { OffsiteIndicator } from "../components/OffsiteIndicator";
import { PlacementFlow } from "../components/placement/PlacementFlow";
import { Timeline, type TimelinePick } from "../components/timeline/Timeline";
import { Badge } from "../components/Badge";
import { Button } from "../components/Button";
import { useToast } from "../lib/toast";
import { FlashZipExportCard } from "./settings/FlashZipExportCard";
import { InfoBubble } from "../components/InfoBubble";
import { IconBackupNow, IconDownload } from "../components/Sidebar";
import { tLtr } from "../lib/ltrFragments";

type T = ReturnType<typeof useT>["t"];

// FlashBackupButton is a square icon badge, like Containers.tsx's
// BackupButton. A glyph leaves no room for inline results, so success and
// failure arrive as toasts and a failure also shakes the button.
function FlashBackupButton({
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
  // The backup runs detached on the server, so the outcome comes from the
  // "flash" progress and the recorded run rather than from the POST.
  const { state, fire, isPending } = useBackupWatch({
    progressKey: "flash",
    start: () => backupFlashNow(),
    matchRun: (r) => r.domain === "flash",
    onDone: onBackedUp,
  });
  // A backup/restore/replication elsewhere blocks a new flash backup.
  const blockedByOther = externallyBusy && !isPending;
  const { push } = useToast();
  const [shake, setShake] = useState(0);
  // The last phase already reported, so each transition toasts once. The
  // phase starts at "idle", so nothing fires on mount.
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

  // The label stays fixed; busy states only show in the tooltip.
  const stateTip = isPending
    ? t("flash.backingUp")
    : blockedByOther
      ? t(busyPhraseKey(busyPhase))
      : undefined;

  return (
    <Button
      key={shake}
      label={t("flash.backupNow")}
      labelKey="flash.backupNow"
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

// How long the download button spins after a click. The server sends nothing
// until it has dumped and recompressed the whole snapshot (dumpFlashZipCompat),
// and the browser has no event to wait for, so this is a fixed guess.
const DOWNLOAD_PREPARING_MS = 20_000;

function FlashDownload({ pick, t }: { pick: TimelinePick; t: T }) {
  const [preparing, setPreparing] = useState(false);

  // A native <a download> leaves progress to the browser's download manager,
  // which survives this row unmounting on a tab switch. It gives up the JSON
  // error fetch() could show before the stream starts; a flash zip is large
  // and rarely fails.
  function handleDownload() {
    setPreparing(true);
    setTimeout(() => setPreparing(false), DOWNLOAD_PREPARING_MS);
    const a = document.createElement("a");
    a.href = flashDownloadURL(pick.snapshotId, pick.source);
    a.download = `flash-${pick.snapshotId.slice(0, 8)}.zip`;
    document.body.appendChild(a);
    a.click();
    a.remove();
  }

  return (
    <Button
      label={t("flash.download")}
      labelKey="flash.download"
      glyph={<IconDownload />}
      tone="accent"
      onClick={handleDownload}
      disabled={preparing}
      busy={preparing}
      className="shrink-0"
    />
  );
}

export function Flash() {
  const { t } = useT();
  const progressMap = useProgress();
  const progress = progressMap["flash"];
  // Any backup, restore or replication in flight disables the backup button
  // up front instead of waiting for the server's 409.
  const running = anyActive(progressMap);
  const [reloadTick, setReloadTick] = useState(0);
  const reload = () => setReloadTick((n) => n + 1);

  return (
    // The OffsiteIndicator sits inside the heading div, so the shell gap alone
    // spaces the heading and the cards.
    <div className={PAGE_SHELL}>
      <div>
        <h1 className="text-2xl font-semibold text-carbon-text">{t("flash.title")}</h1>
        <p className="mt-1 text-sm text-carbon-textSub">{tLtr(t, "flash.subtitle")}</p>
        <div className="mt-2 flex flex-col gap-1">
          <OffsiteIndicator domain="flash" />
          <PlacementFlow domain="flash" />
        </div>
      </div>

      {/* The outer div holds the heading badge, so it carries the notch hover
          zone and the hue, and stays unpadded so its top edge matches the
          inner box for the badge's top-0. insetStart={5} lines the badge up
          with the inner p-5. The inner box is what ProgressBar clips to, as
          in Config.tsx's backup card. */}
      <div className="relative glim-notch-card glim-hue" style={hueVars(0) as CSSProperties}>
        <h2 className="flex items-center">
          <Badge tone="heading" size="heading" wrap hueIndex={0} insetStart={5}>
            {t("flash.backupTitle")}
            <InfoBubble tip={tLtr(t, "flash.backupHint")} onAccent />
          </Badge>
        </h2>
        <div className="relative overflow-hidden bg-carbon-surface rounded-card p-5 flex flex-col gap-4">
          <div className="flex justify-end">
            <FlashBackupButton
              t={t}
              onBackedUp={reload}
              externallyBusy={running.active}
              busyPhase={running.phase}
            />
          </div>

          {/* As on the Folders page: a restore has its own control with its
              own warning. */}
          {progress && progress.active && progress.phase !== "restore" && (
            <div className="flex justify-end">
              <BackupCancelButton cancelKey={"flash"} name={t("nav.flash")} t={t} />
            </div>
          )}

          {/* Pinned to the card's bottom edge. */}
          {progress && (
            <ProgressBar percent={progress.percent} active={progress.active} />
          )}
        </div>
      </div>

      {/* Restore card. `glim-notch-card`: see Settings.tsx's Card() for the
          reasoning. `.glim-hue`: same hueIndex={1} the Badge already uses;
          FlashDownload's badge inherits it via the ordinary custom-property
          cascade, so it passes no hueIndex of its own. */}
      <div
        className="relative glim-notch-card glim-hue bg-carbon-surface rounded-card p-5 flex flex-col gap-4"
        style={hueVars(1) as CSSProperties}
      >
        <h2 className="flex items-center">
          <Badge tone="heading" size="heading" wrap hueIndex={1}>
            {t("snapshots.title")}
            <InfoBubble tip={tLtr(t, "flash.restoreNote")} onAccent />
          </Badge>
        </h2>

        <div className="rounded-card bg-carbon-background px-3 py-1">
          <Timeline
            key={reloadTick}
            domain="flash"
            itemKey="flash"
            itemName={t("flash.title")}
            open
            renderActions={(pick) => <FlashDownload pick={pick} t={t} />}
          />
        </div>
      </div>

      {/* This page numbers its notches by hand: backup 0, restore 1, this 2. */}
      <FlashZipExportCard t={t} hueIndex={2} />
    </div>
  );
}
