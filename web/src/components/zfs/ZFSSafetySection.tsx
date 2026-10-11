import { useEffect, useState } from "react";
import { deleteZFSSafetySnapshot, listZFSSafetySnapshots } from "../../lib/api";
import type { ZFSDatasetView, ZFSSafetySnapshot } from "../../lib/api";
import { humanBytes } from "../../lib/forecast";
import { useT } from "../../lib/i18n";
import { relativeTime } from "../../lib/reltime";
import { useConfirm } from "../../lib/useConfirm";
import { useToast } from "../../lib/toast";
import { Button } from "../Button";
import { Card } from "../../pages/settings/shared";
import { Note } from "../entry/Note";
import { IconTrash } from "../Sidebar";
import { failText } from "./failText";

type T = ReturnType<typeof useT>["t"];

const DAY = 24 * 60 * 60;

/** ZFSSafetySection is the card of the safety snapshots a restore left in the
 *  item's datasets, each with the way to delete it. */
export function ZFSSafetySection({
  item,
  t,
  onRefresh,
  hueIndex,
}: {
  item: ZFSDatasetView;
  t: T;
  onRefresh: () => void;
  hueIndex?: number;
}) {
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [snapshots, setSnapshots] = useState<ZFSSafetySnapshot[]>([]);

  const title = t("zfs.safety.title", item.safetyCount);

  function load() {
    listZFSSafetySnapshots(item.id)
      .then((res) => setSnapshots(res.snapshots ?? []))
      .catch(() => undefined);
  }

  useEffect(load, [item.id, item.safetyCount]);

  async function handleDelete(snap: ZFSSafetySnapshot) {
    const question = t("zfs.safety.deleteConfirm")
      .replace("{name}", snap.name)
      .replace("{dataset}", snap.dataset);
    if (!(await confirm(question, { confirmKey: "common.delete" }))) return;
    try {
      const res = await deleteZFSSafetySnapshot(item.id, snap.dataset, snap.name);
      if (res.ok) {
        load();
        onRefresh();
      } else {
        push(res.error ?? t("common.deleteFailed"), "fail");
      }
    } catch (err) {
      push(failText(t, err), "fail");
    }
  }

  const old = item.safetyOldestAt > 0 && Date.now() / 1000 - item.safetyOldestAt > 30 * DAY;

  return (
    <Card title={title} hint={t("zfs.safety.hint")} hueIndex={hueIndex}>
      {old && <Note tone="warn">{t("zfs.safety.old")}</Note>}
      <ul aria-label={title} className="flex flex-col divide-y divide-carbon-border/60">
          {snapshots.map((snap) => (
            <li key={`${snap.dataset}@${snap.name}`} className="flex items-center gap-2 py-2 text-xs first:pt-0 last:pb-0">
              <span dir="ltr" className="font-mono text-carbon-textSub text-start truncate max-md:whitespace-normal max-md:wrap-anywhere">
                {t("zfs.safety.row")
                  .replace("{dataset}", `${snap.dataset}@${snap.name}`)
                  .replace("{age}", relativeTime(t, snap.createdAt))
                  .replace("{size}", humanBytes(snap.usedBytes))}
              </span>
              <Button
                label={t("common.delete")}
                labelKey="common.delete"
                glyph={<IconTrash />}
                variant="icon"
                glyphOnly
                onClick={() => void handleDelete(snap)}
                className="ms-auto shrink-0"
              />
            </li>
          ))}
      </ul>
      {confirmDialog}
    </Card>
  );
}
