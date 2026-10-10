import { useState } from "react";
import { deleteZFSSafetySnapshot, listZFSSafetySnapshots } from "../../lib/api";
import type { ZFSDatasetView, ZFSSafetySnapshot } from "../../lib/api";
import { humanBytes } from "../../lib/forecast";
import { useT } from "../../lib/i18n";
import { relativeTime } from "../../lib/reltime";
import { useConfirm } from "../../lib/useConfirm";
import { useToast } from "../../lib/toast";
import { Button } from "../Button";
import { IconDisclosure } from "../IconDisclosure";
import { InfoBubble } from "../InfoBubble";
import { IconTrash } from "../Sidebar";
import { failText } from "./failText";

type T = ReturnType<typeof useT>["t"];

const DAY = 24 * 60 * 60;

export function ZFSSafetySection({ item, t, onRefresh }: { item: ZFSDatasetView; t: T; onRefresh: () => void }) {
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [open, setOpen] = useState(false);
  const [snapshots, setSnapshots] = useState<ZFSSafetySnapshot[]>([]);

  const title = t("zfs.safety.title", item.safetyCount);

  function load() {
    listZFSSafetySnapshots(item.id)
      .then((res) => setSnapshots(res.snapshots ?? []))
      .catch(() => undefined);
  }

  function handleOpen() {
    const next = !open;
    setOpen(next);
    if (next) load();
  }

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
    <div className="flex flex-col gap-1">
      <div className="flex items-center gap-1.5">
        <button
          type="button"
          aria-expanded={open}
          onClick={handleOpen}
          className="flex items-center gap-1.5 text-xs text-carbon-textSub hover:text-carbon-text pointer-coarse:min-h-11"
        >
          <IconDisclosure open={open} />
          {title}
        </button>
        <InfoBubble tip={t("zfs.safety.hint")} />
      </div>
      {old && <p className="text-xs text-statusWarn">{t("zfs.safety.old")}</p>}
      {open && (
        <ul aria-label={title} className="flex flex-col gap-1">
          {snapshots.map((snap) => (
            <li key={`${snap.dataset}@${snap.name}`} className="flex items-center gap-2 text-xs">
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
                tone="accent"
                variant="icon"
                onClick={() => void handleDelete(snap)}
                className="ms-auto shrink-0"
              />
            </li>
          ))}
        </ul>
      )}
      {confirmDialog}
    </div>
  );
}
