import { backupNow } from "../lib/api";
import { useBackupWatch } from "../lib/backupWatch";
import { busyPhraseKey } from "../lib/progress";
import type { useT } from "../lib/i18n";
import { CheckDraw } from "./CheckDraw";

type T = ReturnType<typeof useT>["t"];

interface BackupButtonProps {
  name: string;
  t: T;
  /** Called after a successful backup so the caller can refresh (e.g. last-backup time). */
  onBackedUp?: () => void;
  /** "Something is running" signal (anyActive): disables this backup with a
   *  friendly hint while another op runs — but never for its OWN in-flight
   *  backup (that is isPending, handled below). */
  running?: { active: boolean; phase?: string };
}

export function BackupButton({ name, t, onBackedUp, running }: BackupButtonProps) {
  // Fire-and-watch: the server runs the backup detached and answers immediately,
  // so we watch the "container:<name>" progress + recorded run for the outcome
  // instead of awaiting (which would die if we back up the proxy the UI runs
  // through). See useBackupWatch.
  const { state, fire, isPending } = useBackupWatch({
    progressKey: `container:${name}`,
    start: () => backupNow(name),
    matchRun: (r) => r.domain === "container" && r.target === name,
    onDone: onBackedUp,
  });
  const blockedByOther = !!running?.active && !isPending;

  return (
    <div className="flex flex-col gap-1 items-start">
      <button
        onClick={() => void fire()}
        disabled={isPending || blockedByOther}
        className="inline-flex items-center gap-1.5 rounded-control bg-accent px-3 py-1.5 text-xs font-medium text-accentContrast hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {isPending ? (
          <>
            <span
              className="h-3 w-3 rounded-full border-2 border-t-transparent animate-spin inline-block"
              style={{ borderColor: "var(--accent-contrast)", borderTopColor: "transparent" }}
            />
            {t("common.backingUp")}
          </>
        ) : (
          t("containers.backupNow")
        )}
      </button>

      {/* A backup/restore/replication elsewhere blocks a new backup — say why. */}
      {blockedByOther && (
        <span className="text-xs text-carbon-textMuted">{t(busyPhraseKey(running?.phase))}</span>
      )}

      {state.phase === "success" &&
        (state.snapshotId ? (
          <span className="inline-flex items-center gap-1 text-xs text-statusOk">
            <CheckDraw />
            {t("common.done")}
            <span dir="ltr" className="font-mono ms-1 text-start text-carbon-textMuted">
              {state.snapshotId.slice(0, 8)}
            </span>
          </span>
        ) : (
          // No snapshot id ⇒ a stateless container with no data folders. The
          // definition/template is still captured for recreate, but no restic
          // snapshot was made — say so instead of an opaque "Done".
          <span className="inline-flex items-center gap-1 text-xs text-carbon-textSub max-w-[18rem] wrap-break-word">
            <CheckDraw />
            {t("backup.configOnly")}
          </span>
        ))}

      {state.phase === "error" && (
        <span className="text-xs text-statusFail max-w-[18rem] wrap-break-word">
          {state.message}
        </span>
      )}

      {/* Neutral terminal: the container is gone, so the backup was skipped (not
          failed). Reuses the existing "Not installed (backups only)" wording. */}
      {state.phase === "skipped" && (
        <span className="text-xs text-carbon-textMuted max-w-[18rem] wrap-break-word">
          ↷ {t("containers.notInstalledTitle")}
        </span>
      )}
    </div>
  );
}
