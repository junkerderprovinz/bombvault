import { useEffect } from "react";
import { deleteAtTarget, getOffsiteRemoval, type ItemRef, type RemovalPreview } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { placementErrorText } from "../../lib/placementCodes";
import { formatTs } from "../../lib/reltime";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useHostLabel } from "../../lib/useHostLabel";

function OnlyThere({ preview }: { preview: RemovalPreview }) {
  const { t } = useT();
  const host = useHostLabel();
  return (
    <div className="flex flex-col gap-2 text-sm text-carbon-textSub">
      <p>{t("offsiteRemoval.onlyThere")}</p>
      <ul className="list-disc ps-5">
        {preview.onlyThere.map((s) => (
          <li key={s.id}>{formatTs(Date.parse(s.time) / 1000)}</li>
        ))}
      </ul>
      {preview.homeUnreadable && (
        <p>{t("offsiteRemoval.homeUnreadable").replace("{home}", () => preview.homeLabel || host)}</p>
      )}
    </div>
  );
}

/** OffsiteRemovalDialog deletes one item's copies at one target: a plain count
 *  when nothing there is the only copy, otherwise the snapshots listed with
 *  their time and the confirm button locked until the name the server compares
 *  is typed. */
export function OffsiteRemovalDialog({
  item,
  name,
  target,
  onDone,
  onClose,
}: {
  item: ItemRef;
  /** What the card calls the item, for the question; the name to type comes with the preview. */
  name: string;
  target: { id: string; name: string };
  onDone: (deleted: number) => void;
  onClose: () => void;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();

  useEffect(() => {
    let live = true;
    async function run() {
      let res = await getOffsiteRemoval(item, target.id);
      while (live) {
        if (!res.ok || !res.target) {
          push(placementErrorText(t, lang, res, "settings.error"), "fail");
          onClose();
          return;
        }
        const preview: RemovalPreview = {
          target: res.target,
          name: res.name ?? "",
          count: res.count ?? 0,
          onlyThere: res.onlyThere ?? [],
          homeUnreadable: res.homeUnreadable ?? false,
          homeLabel: res.homeLabel ?? "",
        };
        const only = preview.onlyThere.length > 0;
        const yes = await confirm(
          t("offsiteRemoval.ask")
            .replace("{name}", () => name)
            .replace("{target}", () => target.name)
            .replace("{n}", String(preview.count)),
          {
            confirmLabel: t("offsiteRemoval.delete").replace("{target}", () => target.name),
            confirmLabelKey: "common.delete",
            extra: only ? <OnlyThere preview={preview} /> : undefined,
            requireText: only ? preview.name : undefined,
            requirePrompt: t("offsiteRemoval.typeName").replace("{name}", () => preview.name),
          }
        );
        if (!live) return;
        if (!yes) {
          onClose();
          return;
        }
        const del = await deleteAtTarget(
          item,
          target.id,
          preview.onlyThere.map((s) => s.id),
          only ? preview.name : ""
        );
        if (!live) return;
        if (del.ok) {
          const n = del.deleted ?? 0;
          push(t("offsiteRemoval.done").replace("{target}", () => target.name).replace("{n}", String(n)));
          onDone(n);
          return;
        }
        if (del.code === "removal-grown" && del.preview) {
          res = { ok: true, ...del.preview };
          continue;
        }
        push(placementErrorText(t, lang, del, "settings.error"), "fail");
        onClose();
        return;
      }
    }
    run().catch(() => {
      if (!live) return;
      push(t("settings.error"), "fail");
      onClose();
    });
    // Runs once per mount: the dialog opens, asks, and closes itself. `live`
    // is what keeps StrictMode's dev double-invoke from opening it twice.
    return () => {
      live = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return <>{confirmDialog}</>;
}
