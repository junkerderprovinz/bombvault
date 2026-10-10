import { useEffect, useRef, useState, type CSSProperties } from "react";
import { getBackupOrder, setBackupOrder } from "../../lib/api";
import type { Container, ContainerOrder } from "../../lib/api";
import { IconTipButton } from "../IconTipButton";
import { useT } from "../../lib/i18n";
import { Badge } from "../Badge";
import { Button } from "../Button";
import { useReorder } from "../../lib/dragLift";
import { hueVars } from "../../lib/appearance";
import { useToast } from "../../lib/toast";

type T = ReturnType<typeof useT>["t"];

const BACKUP_ORDER_COLLAPSED_KEY = "bombvault.backupOrderCollapsed";

// BackupOrderPanel arranges the order scheduled and batch backups run in. The
// orderable set is the installed, schedule-included containers without
// BombVault itself. It loads the saved order once, then follows containers
// coming and going without discarding a reorder in progress. Save stores the
// displayed sequence as the explicit order; clearing stores an empty list,
// which returns every container to the most-overdue-first tiebreak.
export function BackupOrderPanel({
  containers,
  t,
  hueIndex,
}: {
  containers: Container[];
  t: T;
  /** Rainbow position of this panel's heading and accent. The caller resolves
   *  it during its own render: a counter called from this component's body
   *  would run after every sibling had already taken its slot. Omit for a
   *  panel that stands alone. */
  hueIndex?: number;
}) {
  const [savedOrder, setSavedOrder] = useState<ContainerOrder[] | null>(null);
  const [names, setNames] = useState<string[]>([]);
  const [saveState, setSaveState] = useState<"idle" | "saving">("idle");
  const { push } = useToast();
  const [shakeSave, setShakeSave] = useState(0);
  const [shakeReset, setShakeReset] = useState(0);
  const hydrated = useRef(false);
  const [collapsed, setCollapsed] = useState(() => {
    try {
      return localStorage.getItem(BACKUP_ORDER_COLLAPSED_KEY) === "1";
    } catch {
      return false;
    }
  });

  useEffect(() => {
    getBackupOrder()
      .then((res) => setSavedOrder(res.ok ? res.order ?? [] : []))
      .catch(() => setSavedOrder([]));
  }, []);

  useEffect(() => {
    if (savedOrder === null) return; // still loading the persisted order
    const orderable = containers
      .filter((c) => c.installed && c.includeInSchedule && !c.self)
      .map((c) => c.name);
    const set = new Set(orderable);
    const byName = (a: string, b: string) =>
      a.localeCompare(b, undefined, { sensitivity: "base" });
    if (!hydrated.current) {
      hydrated.current = true;
      const ranked = savedOrder
        .filter((o) => set.has(o.container))
        .sort((a, b) => a.order - b.order)
        .map((o) => o.container);
      const rest = orderable.filter((n) => !ranked.includes(n)).sort(byName);
      setNames([...ranked, ...rest]);
      return;
    }
    setNames((prev) => {
      const kept = prev.filter((n) => set.has(n));
      const added = orderable.filter((n) => !kept.includes(n)).sort(byName);
      const next = [...kept, ...added];
      return next.length === prev.length && next.every((n, i) => n === prev[i])
        ? prev
        : next;
    });
  }, [containers, savedOrder]);

  function move(index: number, dir: -1 | 1) {
    setNames((prev) => {
      const to = index + dir;
      if (to < 0 || to >= prev.length) return prev;
      const next = [...prev];
      [next[index], next[to]] = [next[to], next[index]];
      return next;
    });
    setSaveState("idle");
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
      setSaveState("idle");
    },
  });

  function toggleCollapsed() {
    setCollapsed((v) => {
      const next = !v;
      try {
        localStorage.setItem(BACKUP_ORDER_COLLAPSED_KEY, next ? "1" : "0");
      } catch {
        /* Without storage (private mode, quota) the choice is not kept. */
      }
      return next;
    });
  }

  async function persist(order: string[], via: "save" | "reset") {
    setSaveState("saving");
    const bumpShake = via === "save" ? setShakeSave : setShakeReset;
    try {
      const res = await setBackupOrder(order);
      if (res.ok) {
        setSavedOrder(order.map((container, i) => ({ container, order: i + 1 })));
        push(t("backupOrder.saved"), "success");
      } else {
        push(res.error ?? t("backupOrder.saveError"), "fail");
        bumpShake((n) => n + 1);
      }
    } catch (err) {
      push(err instanceof Error ? err.message : t("backupOrder.saveError"), "fail");
      bumpShake((n) => n + 1);
    } finally {
      setSaveState("idle");
    }
  }

  function clearOrder() {
    const sorted = [...names].sort((a, b) =>
      a.localeCompare(b, undefined, { sensitivity: "base" })
    );
    setNames(sorted);
    void persist([], "reset");
  }

  if (savedOrder === null) return null;

  return (
    // glim-notch-card only wires the hover reveal of the heading notch;
    // glim-hue is what redefines the accent, so the Save button takes the
    // panel's hue too. mt-4 makes up for the heading badge, which sits half
    // above the box and would otherwise eat into the page's gap above it.
    <div
      className={`relative glim-notch-card bg-carbon-surface rounded-card p-4 mt-4 flex flex-col gap-3${
        hueIndex !== undefined ? " glim-hue" : ""
      }`}
      style={hueIndex !== undefined ? (hueVars(hueIndex) as CSSProperties) : undefined}
    >
      {/* A heading badge is absolutely positioned, so a sibling in the h2
          would render underneath it. The count goes inside the badge. */}
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap hueIndex={hueIndex}>
          {t("backupOrder.title")}
          {names.length > 0 && (
            <span className="ms-1.5 font-normal normal-case tracking-normal tabular-nums opacity-80">
              ({names.length})
            </span>
          )}
        </Badge>
      </h2>
      {/* While collapsed the button holds only the aria-hidden chevron, so
          the aria-label is what names it. */}
      <button
        type="button"
        onClick={toggleCollapsed}
        aria-expanded={!collapsed}
        aria-label={t("backupOrder.title")}
        className="flex w-full items-start gap-2 text-start"
      >
        <svg
          width="14"
          height="14"
          viewBox="0 0 12 12"
          fill="none"
          aria-hidden="true"
          className={`mt-0.5 shrink-0 text-carbon-textSub transition-transform ${collapsed ? "rtl:rotate-180" : "rotate-90"}`}
        >
          <path fill="currentColor" d="M4 1.3 8.5 6 4 10.7Z" />
        </svg>
        {!collapsed && (
          <span className="min-w-0 flex-1 text-xs text-carbon-textMuted">{t("backupOrder.hint")}</span>
        )}
      </button>
      {!collapsed &&
        (names.length === 0 ? (
          <p className="text-xs text-carbon-textMuted">{t("backupOrder.empty")}</p>
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
                    <svg width="10" height="14" viewBox="0 0 10 14" fill="currentColor">
                      <circle cx="3" cy="3" r="1" />
                      <circle cx="7" cy="3" r="1" />
                      <circle cx="3" cy="7" r="1" />
                      <circle cx="7" cy="7" r="1" />
                      <circle cx="3" cy="11" r="1" />
                      <circle cx="7" cy="11" r="1" />
                    </svg>
                  </span>
                  <span className="w-6 text-xs text-carbon-textMuted tabular-nums">
                    {i + 1}.
                  </span>
                  <span className="flex-1 min-w-0 truncate text-sm text-carbon-text">
                    {name}
                  </span>
                  <IconTipButton
                    tip={t("backupOrder.moveUp")}
                    onClick={() => move(i, -1)}
                    disabled={i === 0 || saveState === "saving"}
                    className="shrink-0 inline-flex items-center rounded-pill p-1 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-30"
                  >
                    <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
                      <path fill="currentColor" d="M1.3 8.7 6 3.3 10.7 8.7Z" />
                    </svg>
                  </IconTipButton>
                  <IconTipButton
                    tip={t("backupOrder.moveDown")}
                    onClick={() => move(i, 1)}
                    disabled={i === names.length - 1 || saveState === "saving"}
                    className="shrink-0 inline-flex items-center rounded-pill p-1 text-carbon-textSub hover:bg-carbon-hover hover:text-carbon-text transition-colors disabled:opacity-30"
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
                className={`inline-flex items-center rounded-pill px-3 py-1.5 text-xs font-medium text-carbon-textSub hover:text-carbon-text transition-colors disabled:opacity-50${
                  shakeReset ? " glim-shake" : ""
                }`}
              />
              <Button
                key={shakeSave}
                label={t("backupOrder.save")}
                labelKey="backupOrder.save"
                tone="accent"
                onClick={() => void persist(names, "save")}
                disabled={saveState === "saving"}
                busy={saveState === "saving"}
                className={shakeSave ? "glim-shake" : ""}
              />
            </div>
          </>
        ))}
    </div>
  );
}
