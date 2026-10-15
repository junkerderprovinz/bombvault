import { useEffect, useState } from "react";
import { followZFSSnapshot, setZFSSnapshot } from "../../lib/api";
import { Button } from "../Button";
import { IconRefresh } from "../glyphs";
import { useT } from "../../lib/i18n";
import { ToggleRow } from "../../pages/settings/shared";
import { useToast } from "../../lib/toast";

type T = ReturnType<typeof useT>["t"];

// ZFSSnapshotRow is a container's "snapshot instead of stop": a container
// whose data lies on ZFS is down only while its snapshot is taken. Until the
// container has a choice of its own it follows the switch on the Schedules
// page, the way the post-backup update does.
export function ZFSSnapshotRow({
  name,
  initial,
  own: initialOwn,
  onFollow,
  t,
}: {
  name: string;
  initial: boolean;
  own: boolean;
  /** Reloads the container after it hands its choice back. */
  onFollow: () => void;
  t: T;
}) {
  const [enabled, setEnabled] = useState(initial);
  const [own, setOwn] = useState(initialOwn);
  const [busy, setBusy] = useState(false);
  const [shake, setShake] = useState(0);
  const { push } = useToast();
  useEffect(() => setEnabled(initial), [initial]);
  useEffect(() => setOwn(initialOwn), [initialOwn]);

  async function save(call: () => Promise<{ ok: boolean; error?: string }>, done: () => void) {
    setBusy(true);
    try {
      const res = await call();
      if (res.ok) {
        done();
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

  return (
    <div className="flex flex-col items-end gap-1">
      <ToggleRow
        label={t("settings.containerSnapshots")}
        hint={t("settings.containerSnapshotsHint")}
        checked={enabled}
        onChange={(next) =>
          void save(
            () => setZFSSnapshot(name, next),
            () => {
              setEnabled(next);
              setOwn(true);
            },
          )
        }
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
            onClick={() =>
              void save(
                () => followZFSSnapshot(name),
                () => {
                  setOwn(false);
                  onFollow();
                },
              )
            }
            disabled={busy}
          />
        )}
      </p>
    </div>
  );
}
