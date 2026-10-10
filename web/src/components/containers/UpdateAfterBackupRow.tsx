import { useEffect, useState } from "react";
import { setUpdateAfterBackup } from "../../lib/api";
import { useT, type TranslationKey } from "../../lib/i18n";
import { ToggleRow } from "../../pages/settings/shared";
import { relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";

type T = ReturnType<typeof useT>["t"];

// FoldersEditor lets the user choose which of a container's mapped folders get
// backed up (appdata is the default), plus add custom paths under the host
// mount. Collapsible; loads the mount list lazily on first open.
// UpdateAfterBackupRow toggles the per-container "update after successful backup"
// opt-in (#52): after a backup, BombVault pulls the image and recreates the
// container only when a newer image is available (the fresh backup is the safety
// net). Off by default.
//
// (jdp, live-review): the explanation used to sit as a
// permanently-visible caption under the label — moved into a real "(i)"
// InfoBubble instead, via the SAME shared `ToggleRow` Settings.tsx/Config.tsx/
// Recovery.tsx already use for every other "label + hint bubble + flush-right
// switch" row in this app (its own `hint` prop wires an InfoBubble internally
// — see ToggleRow's own doc), rather than hand-rolling a second, bespoke
// bubble treatment here. This row also grew the post-backup update-check
// RESULT line (moved here from the top-right corner, where it used to sit
// under "Letztes Backup" — see ContainerRow's own comment on that move): it
// is live status data about THIS toggle's own last run, not static
// explanatory text, so it stays visible prose, not a second bubble.
export function UpdateAfterBackupRow({
  name,
  initial,
  lastUpdateCheck,
  lastUpdateResult,
  databaseWarn,
  t,
}: {
  name: string;
  initial: boolean;
  lastUpdateCheck: number;
  lastUpdateResult: string;
  /** What an update means for a recognised database, whose major version it
   *  can move; null for any other container. */
  databaseWarn: TranslationKey | null;
  t: T;
}) {
  const [enabled, setEnabled] = useState(initial);
  const [busy, setBusy] = useState(false);
  const { push } = useToast();
  // GlimStone standing rule (jdp, live review, emphatic, system-wide): a
  // failed toggle toasts AND shakes, same mechanism as ToggleRow's shakeNonce.
  const [shake, setShake] = useState(0);
  useEffect(() => setEnabled(initial), [initial]);

  async function handle(next: boolean) {
    setBusy(true);
    try {
      const res = await setUpdateAfterBackup(name, next);
      if (res.ok) setEnabled(next);
      else {
        push(res.error ?? t("containers.updateSettingFailed"), "fail");
        setShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("containers.updateSettingFailed"), "fail");
      setShake((n) => n + 1);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <ToggleRow
        label={t("update.afterBackup")}
        // The old image is left behind on purpose, and Unraid's Docker page
        // lists it as an "orphan image". A successful update therefore looks
        // like wreckage from a failed one, and that is how it was reported
        // (issue #193, after a first night with this on for every container).
        // Said here rather than only in the docs, because this toggle is where
        // someone decides to switch it on.
        hint={`${t("update.afterBackupHint")} ${t("update.afterBackupOrphans")}${databaseWarn ? ` ${t(databaseWarn)}` : ""}`}
        checked={enabled}
        onChange={(next) => void handle(next)}
        disabled={busy}
        shakeNonce={shake}
      />
      {/* Post-backup update-check signal (G4): only meaningful when the
          opt-in is on and a check has actually completed. An up-to-date
          check records no run, so this line is its only surface. */}
      {enabled && lastUpdateCheck > 0 && (
        <p className="text-xs text-carbon-textMuted text-end">
          {t("containers.updateCheckLabel")}: {relativeTime(t, lastUpdateCheck)}, {updateCheckResultText(t, lastUpdateResult)}
        </p>
      )}
    </div>
  );
}

// updateCheckResultText maps the stored update-check result literal to its
// translated display text; an unknown literal falls back to the raw string.
function updateCheckResultText(t: T, result: string): string {
  switch (result) {
    case "up-to-date":
      return t("containers.updateCheckUpToDate");
    case "updated":
      return t("containers.updateCheckUpdated");
    case "failed":
      return t("containers.updateCheckFailed");
    default:
      return result;
  }
}
