import { useEffect, useState } from "react";
import { followUpdateAfterBackup, setUpdateAfterBackup } from "../../lib/api";
import { Button } from "../Button";
import { IconRefresh } from "../glyphs";
import { useT, type TranslationKey } from "../../lib/i18n";
import { ToggleRow } from "../../pages/settings/shared";
import { relativeTime } from "../../lib/reltime";
import { useToast } from "../../lib/toast";

type T = ReturnType<typeof useT>["t"];

// UpdateAfterBackupRow is a container's "update after successful backup":
// after a backup, BombVault pulls the image and recreates the container only
// when a newer image exists, with the fresh backup as the safety net. The
// switch shows what happens; until the container has a choice of its own it
// follows the switch for every container on the Schedules page. The result of
// the last check is status and stays visible under the toggle.
export function UpdateAfterBackupRow({
  name,
  initial,
  own: initialOwn,
  onFollow,
  lastUpdateCheck,
  lastUpdateResult,
  databaseWarn,
  t,
}: {
  name: string;
  initial: boolean;
  own: boolean;
  /** Reloads the container after it hands its choice back, since the switch
   *  for every container decides what it then shows. */
  onFollow: () => void;
  lastUpdateCheck: number;
  lastUpdateResult: string;
  /** What an update means for a recognised database, whose major version it
   *  can move; null for any other container. */
  databaseWarn: TranslationKey | null;
  t: T;
}) {
  const [enabled, setEnabled] = useState(initial);
  const [own, setOwn] = useState(initialOwn);
  const [busy, setBusy] = useState(false);
  const { push } = useToast();
  const [shake, setShake] = useState(0);
  useEffect(() => setEnabled(initial), [initial]);
  useEffect(() => setOwn(initialOwn), [initialOwn]);

  async function handle(next: boolean) {
    setBusy(true);
    try {
      const res = await setUpdateAfterBackup(name, next);
      if (res.ok) {
        setEnabled(next);
        setOwn(true);
      } else {
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

  async function follow() {
    setBusy(true);
    try {
      const res = await followUpdateAfterBackup(name);
      if (res.ok) {
        setOwn(false);
        onFollow();
      } else {
        push(res.error ?? t("containers.updateSettingFailed"), "fail");
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("containers.updateSettingFailed"), "fail");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-col items-end gap-1">
      <ToggleRow
        label={t("update.afterBackup")}
        // The old image stays behind and Unraid's Docker page lists it as an
        // "orphan image", which reads as the wreckage of a failed update
        // (issue #193). The hint says so where the toggle gets switched on.
        hint={`${t("update.afterBackupHint")} ${t("update.afterBackupOrphans")}${databaseWarn ? ` ${t(databaseWarn)}` : ""}`}
        checked={enabled}
        onChange={(next) => void handle(next)}
        disabled={busy}
        shakeNonce={shake}
      />
      <p className="flex flex-wrap items-center justify-end gap-x-2 gap-y-1 text-xs text-carbon-textMuted">
        {own ? t("update.ownChoice") : t("update.followsSchedule")}
        {own && (
          <Button
            label={t("update.useDefault")}
            labelKey="update.useDefault"
            glyph={<IconRefresh />}
            tone="neutral"
            onClick={() => void follow()}
            disabled={busy}
          />
        )}
      </p>
      {/* An up-to-date check records no run, so this line is the only place
          its result shows. */}
      {enabled && lastUpdateCheck > 0 && (
        <p className="text-xs text-carbon-textMuted text-end">
          {t("containers.updateCheckLabel")}: {relativeTime(t, lastUpdateCheck)}, {updateCheckResultText(t, lastUpdateResult)}
        </p>
      )}
    </div>
  );
}

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
