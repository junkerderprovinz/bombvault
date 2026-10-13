import { useRef, useState, type PointerEvent as ReactPointerEvent, type ReactNode, type RefObject } from "react";

import { Badge } from "../../components/Badge";
import { Button } from "../../components/Button";
import {
  CustomizableBlock,
  GRID_COLUMNS,
  packSpans,
  snapHeight,
  snapSpan,
  type CardSpan,
} from "../../lib/dashboardLayout";
import { useReorder } from "../../lib/dragLift";
import type { useT } from "../../lib/i18n";
import { useTipBubble } from "../../lib/useTipBubble";

type T = ReturnType<typeof useT>["t"];

export interface GridCard {
  id: string;
  label: string;
  /** The width somebody gave the card, or its default. */
  span: CardSpan;
  /** The height somebody gave the card. Without one it is as tall as its row. */
  height?: number;
  node: ReactNode;
}

interface Sizing {
  id: string;
  span: CardSpan;
  height: number;
}

// A pointer has to travel this far before a press on the handle is a drag, so
// a double click still reaches it.
const DRAG_THRESHOLD = 3;

/**
 * useResize follows the pointer from a card's handle: the width snaps to the
 * grid's columns and the height to its step. `sizing` is the size under the
 * pointer while it is down, and the card takes it on release.
 */
function useResize(grid: RefObject<HTMLDivElement | null>, onResize: (id: string, span: CardSpan, height: number) => void) {
  const [sizing, setSizing] = useState<Sizing | null>(null);

  function start(e: ReactPointerEvent<HTMLElement>, id: string) {
    const cell = e.currentTarget.closest<HTMLElement>("[data-grid-block]");
    const box = grid.current;
    if (!cell || !box) return;
    e.preventDefault();
    e.stopPropagation();
    const style = getComputedStyle(box);
    // A grid without a gap reports "normal".
    const gap = parseFloat(style.columnGap) || 0;
    const column = (box.getBoundingClientRect().width + gap) / GRID_COLUMNS;
    const rtl = style.direction === "rtl";
    const rect = cell.getBoundingClientRect();
    const from = { x: e.clientX, y: e.clientY };
    let next: Sizing | null = null;

    const move = (m: PointerEvent) => {
      if (!next && Math.hypot(m.clientX - from.x, m.clientY - from.y) <= DRAG_THRESHOLD) return;
      const width = rtl ? rect.right - m.clientX : m.clientX - rect.left;
      next = { id, span: snapSpan(width / column), height: snapHeight(m.clientY - rect.top) };
      setSizing(next);
    };
    const end = (done: PointerEvent) => {
      document.removeEventListener("pointermove", move);
      document.removeEventListener("pointerup", end);
      document.removeEventListener("pointercancel", end);
      setSizing(null);
      if (next && done.type === "pointerup") onResize(next.id, next.span, next.height);
    };
    document.addEventListener("pointermove", move);
    document.addEventListener("pointerup", end);
    document.addEventListener("pointercancel", end);
  }

  return { sizing, start };
}

function ResizeHandle({
  t,
  onPointerDown,
  onDoubleClick,
}: {
  t: T;
  onPointerDown: (e: ReactPointerEvent<HTMLElement>) => void;
  onDoubleClick: () => void;
}) {
  const tip = useTipBubble(t("overview.resizeHint"));
  return (
    <>
      {/* For a pointer only. The keyboard steps the width with the button in
          the card's bar. */}
      <span
        ref={tip.ref}
        aria-hidden="true"
        data-testid="resize-handle"
        onMouseEnter={tip.handlers.onMouseEnter}
        onMouseLeave={tip.handlers.onMouseLeave}
        onPointerDown={onPointerDown}
        onDoubleClick={onDoubleClick}
        className="absolute -bottom-2 -end-2 z-10 h-[18px] w-[18px] cursor-nwse-resize touch-none rounded-full bg-accent shadow-[0_0_0_3px_var(--carbon-background)] rtl:cursor-nesw-resize"
      />
      {tip.bubble}
    </>
  );
}

/**
 * OverviewGrid lays the cards out in six columns. Each row is filled, its
 * cards share its height, and a card with a height of its own scrolls inside.
 * While `editing` every card carries the bar it is moved, widened and hidden
 * by and the handle it is resized with. Below the desktop width the sizes are
 * ignored and the cards stand in one column.
 */
export function OverviewGrid({
  cards,
  editing,
  isDesktop,
  t,
  onReorder,
  onMove,
  onHide,
  onStepWidth,
  onResize,
  onAutoHeight,
}: {
  /** The visible cards in their order. */
  cards: GridCard[];
  editing: boolean;
  isDesktop: boolean;
  t: T;
  onReorder: (ids: string[]) => void;
  /** Puts a card on the far side of the neighbour it is moved past. */
  onMove: (id: string, neighbour: string) => void;
  onHide: (id: string) => void;
  onStepWidth: (id: string, shown: CardSpan) => void;
  onResize: (id: string, span: CardSpan, height: number) => void;
  onAutoHeight: (id: string) => void;
}) {
  const grid = useRef<HTMLDivElement>(null);
  const drag = useReorder({
    ids: cards.map((c) => c.id),
    container: grid,
    attr: "data-grid-block",
    axis: "x",
    arm: "move",
    enabled: editing,
    onReorder,
  });
  const { sizing, start } = useResize(grid, onResize);

  const byId = new Map(cards.map((c) => [c.id, c]));
  const shown = drag.order.map((id) => byId.get(id)).filter((c): c is GridCard => !!c);
  const spans = packSpans(shown.map((c) => (sizing?.id === c.id ? sizing.span : c.span)));

  return (
    <div
      ref={grid}
      data-testid="overview-grid"
      className={`relative grid grid-cols-1 gap-6 md:grid-cols-6 md:gap-10 ${drag.held !== null ? "glim-drag-armed" : ""}`}
    >
      {shown.map((card, i) => {
        const height = sizing?.id === card.id ? sizing.height : card.height;
        const sized = isDesktop && height !== undefined;
        return (
          <div
            key={card.id}
            data-grid-block={card.id}
            data-span={spans[i]}
            data-sized={sized ? "" : undefined}
            style={isDesktop ? { gridColumn: `span ${spans[i]}`, height: sized ? height : undefined } : undefined}
            className={`relative flex min-w-0 flex-col ${drag.look(card.id)}${
              editing ? " rounded-card outline-1 outline-offset-8 outline-dashed outline-carbon-border" : ""
            }`}
          >
            <CustomizableBlock
              id={card.id}
              label={card.label}
              index={i}
              total={shown.length}
              isFirst={i === 0}
              isLast={i === shown.length - 1}
              editing={editing}
              onGripPointerDown={(e) => drag.press(e, card.id)}
              onMoveUp={() => onMove(card.id, shown[i - 1].id)}
              onMoveDown={() => onMove(card.id, shown[i + 1].id)}
              onHide={() => onHide(card.id)}
              span={spans[i]}
              onStepWidth={() => onStepWidth(card.id, spans[i])}
              t={t}
            >
              {card.node}
            </CustomizableBlock>
            {editing && (
              <ResizeHandle t={t} onPointerDown={(e) => start(e, card.id)} onDoubleClick={() => onAutoHeight(card.id)} />
            )}
          </div>
        );
      })}
    </div>
  );
}

/** HiddenTray holds the cards that are switched off, each with the way back. */
export function HiddenTray({ cards, t, onShow }: { cards: { id: string; label: string }[]; t: T; onShow: (id: string) => void }) {
  if (cards.length === 0) return null;
  return (
    <div data-testid="hidden-cards" className="relative flex flex-col gap-3 rounded-card border border-dashed border-carbon-border p-4">
      <h2 className="flex items-center">
        <Badge tone="heading" size="heading" wrap>
          {t("dashboard.hiddenCards")}
        </Badge>
      </h2>
      <div className="flex flex-wrap gap-2">
        {cards.map((card) => (
          <div key={card.id} data-hidden-card={card.id} className="flex items-center gap-2 rounded-pill bg-carbon-surface2 px-2.5 py-1.5">
            <span className="max-w-48 truncate text-xs text-carbon-textSub">{card.label}</span>
            <Button
              label={`${t("dashboard.showCard")} ${card.label}`}
              labelKey="dashboard.showCard"
              tone="neutral"
              onClick={() => onShow(card.id)}
            />
          </div>
        ))}
      </div>
    </div>
  );
}
