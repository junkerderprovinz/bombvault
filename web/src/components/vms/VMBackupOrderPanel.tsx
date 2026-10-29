import { useEffect, useMemo, useRef, useState } from "react";
import { getVmBackupOrder, setVmBackupOrder } from "../../lib/api";
import type { VM, VmOrder } from "../../lib/api";
import { useReorder } from "../../lib/dragLift";
import { useT } from "../../lib/i18n";
import { useToast } from "../../lib/toast";
import { Button } from "../Button";
import { IconGrip } from "../glyphs";
import { IconTipButton } from "../IconTipButton";

type T = ReturnType<typeof useT>["t"];

/**
 * VMBackupOrderPanel sets the order in which the scheduled run backs up the
 * included VMs. It loads the saved order once and then follows VMs coming and
 * going. Every move is stored at once, and Reset stores an empty list, which
 * falls back to name order.
 */
export function VMBackupOrderPanel({
  vms,
  t,
  heading,
}: {
  vms: VM[];
  t: T;
  /** The heading of the list inside the card that holds it. */
  heading: string;
}) {
  const [savedOrder, setSavedOrder] = useState<VmOrder[] | null>(null);
  // Libvirt names, which the server stores and matches on.
  const [names, setNames] = useState<string[]>([]);
  const [saveState, setSaveState] = useState<"idle" | "saving">("idle");
  const { push } = useToast();
  const [shakeReset, setShakeReset] = useState(0);
  const hydrated = useRef(false);
  const displayByLibvirtName = useMemo(
    () => new Map(vms.map((v) => [v.libvirtName, v.name])),
    [vms]
  );

  useEffect(() => {
    getVmBackupOrder()
      .then((res) => setSavedOrder(res.ok ? res.order ?? [] : []))
      .catch(() => setSavedOrder([]));
  }, []);

  useEffect(() => {
    if (savedOrder === null) return;
    const orderable = vms.filter((v) => v.includeInSchedule).map((v) => v.libvirtName);
    const set = new Set(orderable);
    const byDisplayName = (a: string, b: string) =>
      (displayByLibvirtName.get(a) ?? a).localeCompare(displayByLibvirtName.get(b) ?? b, undefined, {
        sensitivity: "base",
      });
    if (!hydrated.current) {
      hydrated.current = true;
      const ranked = savedOrder
        .filter((o) => set.has(o.vm))
        .sort((a, b) => a.order - b.order)
        .map((o) => o.vm);
      const rest = orderable.filter((n) => !ranked.includes(n)).sort(byDisplayName);
      setNames([...ranked, ...rest]);
      return;
    }
    setNames((prev) => {
      const kept = prev.filter((n) => set.has(n));
      const added = orderable.filter((n) => !kept.includes(n)).sort(byDisplayName);
      const next = [...kept, ...added];
      return next.length === prev.length && next.every((n, i) => n === prev[i])
        ? prev
        : next;
    });
  }, [vms, savedOrder, displayByLibvirtName]);

  function move(index: number, dir: -1 | 1) {
    const to = index + dir;
    if (to < 0 || to >= names.length) return;
    const next = [...names];
    [next[index], next[to]] = [next[to], next[index]];
    setNames(next);
    void persist(next, "move");
  }

  // A row is carried by its grip and lands in its gap; the arrows are the
  // keyboard's way to do the same.
  const list = useRef<HTMLOListElement>(null);
  const drag = useReorder({
    ids: names,
    container: list,
    attr: "data-order-name",
    axis: "y",
    arm: "move",
    enabled: saveState !== "saving",
    onReorder: (next) => {
      setNames(next);
      void persist(next, "move");
    },
  });

  async function persist(order: string[], via: "move" | "reset") {
    setSaveState("saving");
    try {
      const res = await setVmBackupOrder(order);
      if (res.ok) {
        setSavedOrder(order.map((vm, i) => ({ vm, order: i + 1 })));
        push(t("backupOrder.saved"), "success");
      } else {
        push(res.error ?? t("backupOrder.saveError"), "fail");
        if (via === "reset") setShakeReset((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("backupOrder.saveError"), "fail");
      if (via === "reset") setShakeReset((n) => n + 1);
    } finally {
      setSaveState("idle");
    }
  }

  function clearOrder() {
    const sorted = [...names].sort((a, b) =>
      (displayByLibvirtName.get(a) ?? a).localeCompare(displayByLibvirtName.get(b) ?? b, undefined, {
        sensitivity: "base",
      })
    );
    setNames(sorted);
    void persist([], "reset");
  }

  if (savedOrder === null) return null;

  return (
    <section className="flex flex-col gap-3">
      <h3 className="text-sm font-medium text-carbon-text">{heading}</h3>
      {names.length === 0 ? (
          <p className="text-xs text-carbon-textMuted">{t("vmBackupOrder.empty")}</p>
        ) : (
          <>
            {/* The list is the rows' offsetParent, the layout a drag measures
                in. While a row is carried the others wiggle. */}
            <ol ref={list} className={`relative flex flex-col gap-1 ${drag.held !== null ? "glim-drag-armed" : ""}`}>
              {drag.order.map((name, i) => (
                <li
                  key={name}
                  data-order-name={name}
                  className={`flex select-none items-center gap-2 rounded-control bg-carbon-surface2 px-3 py-1.5 ${drag.look(name)}`}
                >
                  {/* The grip is for a pointer; the keyboard uses the arrows. */}
                  <span
                    className="shrink-0 cursor-grab touch-none text-carbon-textSub active:cursor-grabbing"
                    aria-hidden="true"
                    onPointerDown={(e) => drag.press(e, name)}
                  >
                    <IconGrip />
                  </span>
                  <span className="w-6 text-xs text-carbon-textMuted tabular-nums">{i + 1}.</span>
                  <span className="flex-1 min-w-0 truncate text-sm text-carbon-text">
                    {displayByLibvirtName.get(name) ?? name}
                  </span>
                  <IconTipButton
                    tip={t("backupOrder.moveUp")}
                    onClick={() => move(i, -1)}
                    disabled={i === 0 || saveState === "saving"}
                    className="shrink-0 inline-flex items-center rounded-pill p-1 pointer-coarse:p-2 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-30"
                  >
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                      <path fill="currentColor" d="M1.3 8.7 6 3.3 10.7 8.7Z" />
                    </svg>
                  </IconTipButton>
                  <IconTipButton
                    tip={t("backupOrder.moveDown")}
                    onClick={() => move(i, 1)}
                    disabled={i === names.length - 1 || saveState === "saving"}
                    className="shrink-0 inline-flex items-center rounded-pill p-1 pointer-coarse:p-2 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-30"
                  >
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                      <path fill="currentColor" d="M1.3 3.3 6 8.7 10.7 3.3Z" />
                    </svg>
                  </IconTipButton>
                </li>
              ))}
            </ol>
            <div className="flex items-center gap-3 flex-wrap">
              <Button
                key={shakeReset}
                label={t("backupOrder.reset")}
                labelKey="backupOrder.reset"
                tone="subtle"
                onClick={clearOrder}
                disabled={saveState === "saving"}
                className={`inline-flex items-center rounded-pill px-3 py-1.5 text-xs font-medium text-carbon-textSub hover:text-carbon-text transition-colors disabled:opacity-50 ${
                  shakeReset ? "glim-shake" : ""
                }`}
              />
            </div>
          </>
      )}
    </section>
  );
}
