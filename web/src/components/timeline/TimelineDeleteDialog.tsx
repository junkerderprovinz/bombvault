import { useEffect } from "react";
import { deleteTimelineRow, getTimelineDeletePreview, type TimelineDomain, type TimelineRow } from "../../lib/api";
import { useT } from "../../lib/i18n";
import { formatList } from "../../lib/placement";
import { placementErrorText } from "../../lib/placementCodes";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { hostLabelSettled } from "../../lib/useHostLabel";

/** TimelineDeleteDialog deletes one row at one or more places: the server
 *  lists every other place live, so the question can say whether this is the
 *  last copy, which places still hold it, and which could not be checked.
 *  `onDone` fires even on a refusal, since part of the row can already be
 *  gone, and when a place turns out not to hold the backup, since the card's
 *  row is stale then; a cancel or any other empty preview only closes. */
export function TimelineDeleteDialog({
  domain,
  itemKey,
  row,
  places,
  onDone,
  onClose,
}: {
  domain: TimelineDomain;
  itemKey: string;
  row: TimelineRow;
  /** The places to delete at; empty means the whole row. */
  places: string[];
  onDone: () => void;
  onClose: () => void;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();

  useEffect(() => {
    let live = true;
    async function run() {
      // Fetched alongside the preview rather than read from useHostLabel():
      // that hook's value only reaches this component on a re-render, which
      // this effect, run once per opening, never sees.
      const [res, host] = await Promise.all([
        getTimelineDeletePreview(domain, itemKey, row.key, places),
        hostLabelSettled().then((label) => label || t("placement.hostGeneric")),
      ]);
      if (!live) return;
      const names = (list: { label: string }[]) => formatList(lang, list.map((p) => p.label || host));
      const del = res.delete ?? [];
      const others = res.others ?? [];
      if (!res.ok) {
        push(placementErrorText(t, lang, res, "common.deleteFailed"), "fail");
        onClose();
        return;
      }
      if (del.length === 0) {
        // Only the places this delete asked for say why nothing came back; the
        // others are there for the question, which is dropped.
        const asked = places.length === 0 ? others : others.filter((o) => places.includes(o.place));
        const unreadable = asked.filter((o) => o.state === "unreadable");
        const appendOnly = asked.filter((o) => o.state === "append-only");
        if (unreadable.length > 0) push(t("placement.uncheckable").replace("{list}", () => names(unreadable)), "fail");
        else if (appendOnly.length > 0) push(t("timeline.deleteSkipped").replace("{list}", () => names(appendOnly)), "fail");
        else push(t("placementCode.snapshotMissing"), "fail");
        // A place that no longer holds the backup leaves the card showing a
        // row that is gone, so the caller reloads instead of only closing.
        if (asked.some((o) => o.state === "missing")) onDone();
        else onClose();
        return;
      }
      // Append-only places keep their copy no matter what is asked, so they
      // count as "still held" as well as being called out as left out.
      const kept = others.filter((o) => o.state === "holds" || o.state === "append-only");
      const unread = others.filter((o) => o.state === "unreadable");
      const leftOut = others.filter((o) => o.state === "append-only");
      const lines: string[] = [];
      if (kept.length === 0 && unread.length === 0) lines.push(t("timeline.deleteLast"));
      if (kept.length > 0) lines.push(t("timeline.deleteHeldBy").replace("{list}", () => names(kept)));
      if (unread.length > 0) lines.push(t("placement.uncheckable").replace("{list}", () => names(unread)));
      if (leftOut.length > 0) lines.push(t("timeline.deleteSkipped").replace("{list}", () => names(leftOut)));
      const everywhere = places.length === 0;
      const yes = await confirm(t("timeline.deleteAsk").replace("{list}", () => names(del)), {
        confirmLabel: everywhere ? t("timeline.deleteRow") : t("common.delete"),
        confirmLabelKey: everywhere ? "timeline.deleteRow" : "common.delete",
        extra: (
          <div className="flex flex-col gap-1 text-sm text-carbon-textSub">
            {lines.map((line) => (
              <p key={line}>{line}</p>
            ))}
          </div>
        ),
      });
      if (!live) return;
      if (!yes) {
        onClose();
        return;
      }
      const out = await deleteTimelineRow(domain, itemKey, row.key, del);
      if (!live) return;
      const skipped = out.skipped ?? [];
      const done = out.deleted ?? [];
      if (!out.ok) {
        push(placementErrorText(t, lang, out, "common.deleteFailed"), "fail");
        // A refusal can come after places were emptied, and those copies are
        // gone for good, so the failure never stands on its own.
        if (done.length > 0) push(t("timeline.deletePartial").replace("{list}", () => names(done)));
      } else if (skipped.length > 0) push(t("timeline.deleteSkipped").replace("{list}", () => names(skipped)));
      onDone();
    }
    run().catch(() => {
      if (!live) return;
      push(t("common.deleteFailed"), "fail");
      onClose();
    });
    // One run per opening of the dialog; a `places` dependency would remount
    // it mid-flow and reopen a confirmation the operator already answered.
    return () => {
      live = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return <>{confirmDialog}</>;
}
