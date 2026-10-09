import { useState } from "react";
import { createPortal } from "react-dom";

import { restoreZFSReplica } from "../../../lib/api";
import type { ZFSReplica } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { formatTs } from "../../../lib/reltime";
import { useConfirm } from "../../../lib/useConfirm";
import { useToast } from "../../../lib/toast";
import { zfsCodeSentence } from "../../../lib/zfsCodes";
import { Badge } from "../../Badge";
import { Button } from "../../Button";
import { CopyBlock } from "../../CopyBlock";
import { InfoBubble } from "../../InfoBubble";
import { SelectField } from "../../SelectField";
import { cloneCommand, rootMember, takeoverCommand, unixOf } from "./replicaModel";

/** ReplicaSnapshotsSheet lists the snapshots a replica keeps on its target,
 *  hands out the commands to use one there, and brings one back here as a
 *  new dataset beside the item. */
export function ReplicaSnapshotsSheet({
  itemId,
  name,
  targetName,
  replica,
  onClose,
  onRestoring,
}: {
  itemId: string;
  /** The item's root dataset. */
  name: string;
  targetName: string;
  replica: ZFSReplica;
  onClose: () => void;
  /** Called with the dataset the restore creates, once it has started. */
  onRestoring: (dataset: string) => void;
}) {
  const { t } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const [picked, setPicked] = useState(replica.snapshots[0]?.name ?? "");
  const [starting, setStarting] = useState(false);

  const member = rootMember(replica, name);
  const snapshot = replica.snapshots.find((s) => s.name === picked) ?? replica.snapshots[0];
  const heading = t("zfs.replica.sheet.title").replace("{name}", () => name).replace("{target}", () => targetName);

  async function bringBack() {
    const when = formatTs(unixOf(snapshot.created));
    const question = `${t("zfs.replica.restoreQuestion")} ${t("zfs.replica.restoreText")
      .replace("{when}", () => when)
      .replace("{target}", () => targetName)
      .replace("{fresh}", () => `${name}-bombvault-restore-…`)
      .replace("{name}", () => name)}`;
    if (!(await confirm(question, { confirmKey: "zfs.replica.restoreConfirm" }))) return;
    setStarting(true);
    try {
      const res = await restoreZFSReplica(itemId, snapshot.name);
      if (!res.ok) {
        push(res.code ? zfsCodeSentence(t, res.code) : (res.error ?? t("common.restoreFailed")), "fail");
        return;
      }
      const fresh = res.dataset ?? "";
      push(t("zfs.replica.restoring").replace("{fresh}", () => fresh), "success");
      onRestoring(fresh);
      onClose();
    } catch (err) {
      push(err instanceof Error ? err.message : t("common.restoreFailed"), "fail");
    } finally {
      setStarting(false);
    }
  }

  return createPortal(
    <>
      <div
        className="glim-modal-backdrop fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4"
        onClick={starting ? undefined : onClose}
      >
        <div className="relative w-full max-w-2xl">
          <h2 className="flex items-center px-5">
            <Badge tone="heading" size="heading" wrap>
              {heading}
              <InfoBubble tip={t("zfs.replica.sheet.hint")} onAccent />
            </Badge>
          </h2>
          <div
            role="dialog"
            aria-modal="true"
            aria-label={heading}
            onClick={(e) => e.stopPropagation()}
            className="flex max-h-[90vh] w-full flex-col gap-4 overflow-y-auto rounded-card bg-carbon-surface p-5 shadow-2xl"
          >
            <p dir="ltr" className="font-mono text-xs text-carbon-textMuted text-start wrap-anywhere">
              {member.targetPath}
            </p>

            <div className="flex flex-col gap-1.5">
              <span className="flex items-center gap-1.5 text-xs text-carbon-textSub">
                {t("zfs.replica.sheet.snapshot")}
                <InfoBubble tip={t("zfs.replica.sheet.snapshotHint", replica.snapshots.length)} />
              </span>
              <SelectField
                value={snapshot.name}
                onChange={setPicked}
                label={t("zfs.replica.sheet.snapshot")}
                options={replica.snapshots.map((s) => ({ value: s.name, label: formatTs(unixOf(s.created)) }))}
                className="w-72 max-w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-sm text-carbon-text"
              />
            </div>

            <div className="flex flex-col gap-1.5">
              <span className="text-xs font-semibold text-carbon-textSub">{t("zfs.replica.sheet.clone")}</span>
              <p className="text-xs text-carbon-textSub">{t("zfs.replica.sheet.cloneHint")}</p>
              <CopyBlock text={cloneCommand(member.targetPath, name, snapshot.name)} />
            </div>

            <div className="flex flex-col gap-1.5">
              <span className="text-xs font-semibold text-carbon-textSub">{t("zfs.replica.sheet.takeover")}</span>
              <p className="text-xs text-carbon-textSub">{t("zfs.replica.sheet.takeoverHint")}</p>
              <CopyBlock text={takeoverCommand(member.targetPath)} />
            </div>

            <div className="flex flex-wrap items-center justify-end gap-2 pt-1">
              <Button label={t("common.close")} labelKey="common.close" tone="neutral" onClick={onClose} />
              <Button
                label={t("zfs.replica.restore")}
                labelKey="zfs.replica.restore"
                tone="accent"
                onClick={() => void bringBack()}
                disabled={starting}
                busy={starting}
              />
            </div>
          </div>
        </div>
      </div>
      {confirmDialog}
    </>,
    document.body,
  );
}
