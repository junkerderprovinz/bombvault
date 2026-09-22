import { useEffect, useRef, useState } from "react";
import {
  setItemPlacement,
  type DroppedTarget,
  type ItemRef,
  type PlacementChange,
  type PlacementPatchResponse,
  type PlacementView,
} from "./api";
import { useT } from "./i18n";
import { placementErrorText } from "./placementCodes";
import { placementChanged } from "./placementEvents";
import { useToast } from "./toast";

type T = ReturnType<typeof useT>["t"];

function droppedText(t: T, d: DroppedTarget): string {
  if (d.appendOnly) return t("placement.droppedAppendOnly").replace("{target}", () => d.name);
  if (d.copies === null) return t("placement.droppedKeepsUnknown").replace("{target}", () => d.name);
  return t("placement.droppedKeeps").replace("{target}", () => d.name).replace("{n}", String(d.copies));
}

/** usePlacementSave writes one card's placement: every change shows at once,
 *  one PATCH runs at a time, and only the newest change waits behind it. */
export function usePlacementSave(item: ItemRef, view: PlacementView, onView: (next: PlacementView) => void) {
  const { t, lang } = useT();
  const { push } = useToast();
  const [confirmed, setConfirmed] = useState(view);
  const [optimistic, setOptimistic] = useState<Partial<PlacementView> | null>(null);
  const [saving, setSaving] = useState(false);
  const [shake, setShake] = useState(0);
  const running = useRef(false);
  const waiting = useRef<PlacementChange | null>(null);
  const latest = useRef({ item, onView, t, lang, push });
  latest.current = { item, onView, t, lang, push };
  const alive = useRef(false);

  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);

  // A list read that started before this card's change answers with the state
  // the change has already left behind, so it must not reseed the card.
  useEffect(() => {
    if (!running.current) setConfirmed(view);
  }, [view]);

  async function drain(first: PlacementChange) {
    running.current = true;
    setSaving(true);
    let wrote = false;
    let next: PlacementChange | null = first;
    while (next) {
      const now = latest.current;
      let res: PlacementPatchResponse;
      try {
        res = await setItemPlacement(now.item, next);
      } catch (err) {
        res = { ok: false, error: err instanceof Error ? err.message : undefined };
      }
      // The card can be gone by the time the answer lands, and then there is
      // nothing left to correct, shake or tell the page about.
      if (!alive.current) return;
      if (!res.ok) {
        waiting.current = null;
        setOptimistic(null);
        setShake((n) => n + 1);
        now.push(placementErrorText(now.t, now.lang, res, "settings.error"), "fail");
        break;
      }
      wrote = true;
      const placed = res.placement;
      if (placed) {
        setConfirmed(placed);
        now.onView(placed);
      }
      for (const d of res.dropped ?? []) now.push(droppedText(now.t, d), "warn");
      next = waiting.current;
      waiting.current = null;
      if (!next) setOptimistic(null);
    }
    running.current = false;
    setSaving(false);
    // The list is read again once the write is through, so an answer to a read
    // that started before it cannot keep the last word.
    if (wrote) placementChanged();
  }

  function save(change: PlacementChange, opt: Partial<PlacementView>) {
    setOptimistic((prev) => ({ ...prev, ...opt }));
    if (running.current) {
      waiting.current = change;
      return;
    }
    void drain(change);
  }

  const shown = optimistic ? { ...confirmed, ...optimistic } : confirmed;
  return { shown, saving, shake, save };
}
