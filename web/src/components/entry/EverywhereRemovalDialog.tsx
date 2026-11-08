import { useEffect } from "react";
import {
  deleteEverywhere,
  explainedError,
  getEverywhereRemoval,
  type EverywhereRemoval,
  type ItemRef,
  type PlaceOfItem,
  type PlaceResult,
} from "../../lib/api";
import { useT } from "../../lib/i18n";
import { placementErrorText } from "../../lib/placementCodes";
import { useToast } from "../../lib/toast";
import { useConfirm } from "../../lib/useConfirm";
import { useLocalName } from "../../lib/useLocalName";
import { useOwnLogin } from "../../lib/useOwnLogin";

function usePlaceName(domain: string) {
  const local = useLocalName(domain);
  return (p: { place: string; label: string }) => p.label || (p.place === "local" ? local : p.place);
}

function Places({ places, domain }: { places: PlaceOfItem[]; domain: string }) {
  const { t } = useT();
  const nameOf = usePlaceName(domain);
  return (
    <ul className="flex list-disc flex-col gap-1 ps-5 text-sm text-carbon-textSub">
      {places.map((p) => {
        const place = nameOf(p);
        const n = String(p.count ?? 0);
        const key = p.unreadable ? "removeEverywhere.unreadable" : p.appendOnly ? "removeEverywhere.appendOnly" : "removeEverywhere.place";
        return (
          <li key={p.place} data-place={p.place}>
            {t(key).replace("{place}", () => place).replace("{n}", n)}
          </li>
        );
      })}
    </ul>
  );
}

/** EverywhereRemovalDialog deletes an item's backups at its home and at every
 *  off-site target in one go. It lists each place with its count, says which
 *  ones keep theirs, and stays locked until the item's name is typed. */
export function EverywhereRemovalDialog({
  item,
  name,
  onDone,
  onClose,
}: {
  item: ItemRef;
  /** What the card calls the item, for the question; the name to type comes with the preview. */
  name: string;
  onDone: () => void;
  onClose: () => void;
}) {
  const { t, lang } = useT();
  const { push } = useToast();
  const { confirm, confirmDialog } = useConfirm();
  const ownLogin = useOwnLogin();
  const nameOf = usePlaceName(item.domain);

  function report(results: PlaceResult[]) {
    const deleted = results.reduce((sum, r) => sum + r.deleted, 0);
    push(t("removeEverywhere.done").replace("{name}", () => name).replace("{n}", String(deleted)), "success");
    for (const r of results) {
      if (r.kept) push(t("removeEverywhere.kept").replace("{place}", () => nameOf(r)), "warn");
      if (r.error) push(t("removeEverywhere.failed").replace("{place}", () => nameOf(r)).replace("{error}", () => r.error ?? ""), "fail");
    }
  }

  useEffect(() => {
    let live = true;
    async function run() {
      let res: Awaited<ReturnType<typeof getEverywhereRemoval>> = await getEverywhereRemoval(item);
      while (live) {
        if (!res.ok || !res.places) {
          push(placementErrorText(t, lang, res, "settings.error"), "fail");
          onClose();
          return;
        }
        const preview: EverywhereRemoval = { name: res.name ?? "", places: res.places };
        if (preview.places.length === 0) {
          push(t("removeEverywhere.none").replace("{name}", () => name));
          onClose();
          return;
        }
        const yes = await confirm(t("removeEverywhere.ask").replace("{name}", () => name), {
          confirmLabel: t("removeEverywhere.go"),
          confirmLabelKey: "removeEverywhere.go",
          confirmTone: "neutral",
          extra: <Places places={preview.places} domain={item.domain} />,
          requireText: preview.name,
          requirePrompt: t("offsiteRemoval.typeName").replace("{name}", () => preview.name),
        });
        if (!live) return;
        if (!yes) {
          onClose();
          return;
        }
        const del = await deleteEverywhere(
          item,
          preview.name,
          preview.places.map((p) => p.place)
        );
        if (!live) return;
        if (del.ok) {
          report(del.results ?? []);
          onDone();
          return;
        }
        if (del.code === "places-changed" && del.preview) {
          res = { ok: true, ...del.preview };
          continue;
        }
        if (ownLogin.handled(del, onClose)) return;
        push(placementErrorText(t, lang, del, "settings.error"), "fail");
        onClose();
        return;
      }
    }
    run().catch((err: unknown) => {
      if (!live) return;
      push(explainedError(err) ?? t("settings.error"), "fail");
      onClose();
    });
    // Runs once per mount, like OffsiteRemovalDialog: it opens, asks and closes itself.
    return () => {
      live = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <>
      {confirmDialog}
      {ownLogin.dialog}
    </>
  );
}
