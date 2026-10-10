// Customizable dashboard layout: card order, visibility and width per browser,
// kept in localStorage. A card is carried by the grip in its control bar
// (lib/dragLift.ts), with the move up/down buttons as the keyboard's way.

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type PointerEventHandler,
  type ReactNode,
} from "react";
import { IconTipButton } from "../components/IconTipButton";
import type { TranslationKey } from "./i18n";

const KEY = "bombvault.dashboardLayout";

type T = (key: TranslationKey) => string;

// CardWidth "half" puts two cards side by side in the Dashboard grid. Cards
// that were never toggled are "full".
export type CardWidth = "full" | "half";

interface LayoutState {
  order: string[];
  hidden: Set<string>;
  widths: Record<string, CardWidth>;
}

interface StoredLayout {
  order: string[];
  hidden: string[];
  widths: Record<string, CardWidth>;
}

// readStored returns null for a missing or corrupt value. Entries of the wrong
// type are dropped, so an unknown width falls back to full.
function readStored(): StoredLayout | null {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return null;
    const parsed: unknown = JSON.parse(raw);
    if (!parsed || typeof parsed !== "object") return null;
    const obj = parsed as { order?: unknown; hidden?: unknown; widths?: unknown };
    const order = Array.isArray(obj.order)
      ? obj.order.filter((x): x is string => typeof x === "string")
      : [];
    const hidden = Array.isArray(obj.hidden)
      ? obj.hidden.filter((x): x is string => typeof x === "string")
      : [];
    const widths: Record<string, CardWidth> = {};
    if (obj.widths && typeof obj.widths === "object" && !Array.isArray(obj.widths)) {
      for (const [id, w] of Object.entries(obj.widths as Record<string, unknown>)) {
        if (w === "full" || w === "half") widths[id] = w;
      }
    }
    return { order, hidden, widths };
  } catch {
    return null;
  }
}

// mergeOrder drops stored ids that are no longer known and places every new
// default id directly behind the card it follows by default, so a card added
// later lands next to the one it belongs with rather than below the fold. A
// predecessor the stored order does not have leaves the new id at the end.
export function mergeOrder(stored: string[], defaultOrder: string[]): string[] {
  const known = new Set(defaultOrder);
  const merged: string[] = [];
  const seen = new Set<string>();
  for (const id of stored) {
    if (known.has(id) && !seen.has(id)) {
      merged.push(id);
      seen.add(id);
    }
  }
  for (const [i, id] of defaultOrder.entries()) {
    if (seen.has(id)) continue;
    const after = i > 0 ? merged.indexOf(defaultOrder[i - 1]) : -1;
    if (after >= 0) merged.splice(after + 1, 0, id);
    else merged.push(id);
    seen.add(id);
  }
  return merged;
}

export const LAYOUT_VERSION = 2;
export const GRID_COLUMNS = 6;
// A card takes a third, a half, two thirds or the whole row.
export const CARD_SPANS = [2, 3, 4, 6] as const;
export type CardSpan = (typeof CARD_SPANS)[number];
export const MIN_CARD_HEIGHT = 140;
export const CARD_HEIGHT_STEP = 20;

/** CardDefault is a card's place in the default layout. */
export interface CardDefault {
  id: string;
  span: CardSpan;
  hidden?: boolean;
}

/**
 * GridLayout is the stored layout of the six-column grid. widths and heights
 * hold only the cards somebody resized: the others take their default span and
 * their natural height.
 */
export interface GridLayout {
  v: typeof LAYOUT_VERSION;
  order: string[];
  hidden: string[];
  widths: Record<string, CardSpan>;
  heights: Record<string, number>;
}

/** LayoutMigration says what became of the cards of an unversioned layout. */
export interface LayoutMigration {
  /** Old card id to the id that replaces it. Several ids may share one. */
  renames: Record<string, string>;
  /** Cards that are gone without a successor. */
  drops: string[];
}

/** snapSpan returns the span nearest to a width in columns, the narrower one on a tie. */
export function snapSpan(columns: number): CardSpan {
  return CARD_SPANS.reduce((best, span) =>
    Math.abs(span - columns) < Math.abs(best - columns) ? span : best
  );
}

/** snapHeight rounds a height in pixels to the step and keeps it above the minimum. */
export function snapHeight(px: number): number {
  return Math.max(MIN_CARD_HEIGHT, Math.round(px / CARD_HEIGHT_STEP) * CARD_HEIGHT_STEP);
}

/** nextSpan steps to the next wider span and from the full row back to a third. */
export function nextSpan(span: CardSpan): CardSpan {
  return CARD_SPANS[(CARD_SPANS.indexOf(span) + 1) % CARD_SPANS.length];
}

/**
 * packSpans widens the last card of every row by the columns the row leaves
 * free, so hiding or moving a card never opens a gap. It takes the spans of
 * the visible cards in order and leaves the stored widths alone.
 */
export function packSpans(spans: CardSpan[]): CardSpan[] {
  const packed: number[] = spans.slice();
  let used = 0;
  spans.forEach((span, i) => {
    if (used + span > GRID_COLUMNS) {
      packed[i - 1] += GRID_COLUMNS - used;
      used = 0;
    }
    used += span;
  });
  if (packed.length > 0) packed[packed.length - 1] += GRID_COLUMNS - used;
  // Whatever a row of allowed spans leaves free widens its last card to
  // another allowed span.
  return packed as CardSpan[];
}

/** cardSpan is the width somebody gave the card, or its default. */
export function cardSpan(layout: GridLayout, card: CardDefault): CardSpan {
  return layout.widths[card.id] ?? card.span;
}

export function defaultLayout(cards: CardDefault[]): GridLayout {
  return {
    v: LAYOUT_VERSION,
    order: cards.map((c) => c.id),
    hidden: cards.filter((c) => c.hidden).map((c) => c.id),
    widths: {},
    heights: {},
  };
}

function strings(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((x): x is string => typeof x === "string") : [];
}

function entries(value: unknown): [string, unknown][] {
  return value && typeof value === "object" && !Array.isArray(value) ? Object.entries(value) : [];
}

function ofKnown<V>(sized: Record<string, V>, known: Set<string>): Record<string, V> {
  return Object.fromEntries(Object.entries(sized).filter(([id]) => known.has(id)));
}

interface Arrangement {
  order: string[];
  hidden: Set<string>;
  widths: Record<string, CardSpan>;
}

// rearrange applies the drops and renames to an unversioned layout. A card the
// layout already has keeps its own place, visibility and width, and whatever is
// renamed to it just goes. Otherwise the new card stands where the first of its
// old cards stood, takes that one's width, and is hidden only if all were.
function rearrange(stored: Arrangement, migration: LayoutMigration): Arrangement {
  const dropped = new Set(migration.drops);
  const kept = stored.order.filter((id) => !dropped.has(id));
  const had = new Set(kept);
  const order: string[] = [];
  const shown = new Set<string>();
  const widths: Record<string, CardSpan> = {};
  for (const id of kept) {
    const to = migration.renames[id] ?? id;
    if (to !== id && had.has(to)) continue;
    if (!order.includes(to)) order.push(to);
    if (!stored.hidden.has(id)) shown.add(to);
    if (widths[to] === undefined && stored.widths[id] !== undefined) widths[to] = stored.widths[id];
  }
  return { order, hidden: new Set(order.filter((id) => !shown.has(id))), widths };
}

/**
 * migrateLayout turns whatever the browser stored into a layout for the given
 * cards. Nothing stored, or nothing readable, gives the default. An
 * unversioned layout keeps its order and hidden set, has its cards dropped and
 * renamed as the migration says, and maps full and half to 6 and 3 columns. A
 * card the stored layout does not know appears as the default has it, behind
 * the card it follows by default, and the default's first card leads.
 */
export function migrateLayout(
  stored: unknown,
  cards: CardDefault[],
  migration: LayoutMigration
): GridLayout {
  if (!stored || typeof stored !== "object") return defaultLayout(cards);
  const raw = stored as {
    v?: unknown;
    order?: unknown;
    hidden?: unknown;
    widths?: unknown;
    heights?: unknown;
  };
  if (raw.v !== undefined && raw.v !== LAYOUT_VERSION) return defaultLayout(cards);

  let arranged: Arrangement = {
    order: strings(raw.order),
    hidden: new Set(strings(raw.hidden)),
    widths: {},
  };
  const heights: Record<string, number> = {};
  if (raw.v === LAYOUT_VERSION) {
    for (const [id, w] of entries(raw.widths)) {
      if (typeof w === "number" && Number.isFinite(w)) arranged.widths[id] = snapSpan(w);
    }
    for (const [id, h] of entries(raw.heights)) {
      if (typeof h === "number" && Number.isFinite(h)) heights[id] = snapHeight(h);
    }
  } else {
    for (const [id, w] of entries(raw.widths)) {
      if (w === "full") arranged.widths[id] = 6;
      else if (w === "half") arranged.widths[id] = 3;
    }
    arranged = rearrange(arranged, migration);
  }

  const ids = cards.map((c) => c.id);
  const known = new Set(ids);
  const had = new Set(arranged.order);
  const hiddenByDefault = new Set(cards.filter((c) => c.hidden).map((c) => c.id));
  // mergeOrder puts a card that nothing precedes at the end. The first card of
  // the default belongs at the top, so it is seeded there.
  const lead = ids.length > 0 && !had.has(ids[0]) ? [ids[0]] : [];
  const order = mergeOrder([...lead, ...arranged.order], ids);
  return {
    v: LAYOUT_VERSION,
    order,
    hidden: order.filter((id) => (had.has(id) ? arranged.hidden.has(id) : hiddenByDefault.has(id))),
    widths: ofKnown(arranged.widths, known),
    heights: ofKnown(heights, known),
  };
}

/**
 * useDashboardLayout keeps the card order, hidden set and widths for this
 * browser. defaultOrder decides which ids are known: unknown stored ids are
 * ignored, and new ones are appended, visible.
 */
export function useDashboardLayout(defaultOrder: string[]) {
  const [state, setState] = useState<LayoutState>(() => {
    const stored = readStored();
    const known = new Set(defaultOrder);
    const order = mergeOrder(stored?.order ?? [], defaultOrder);
    const hidden = new Set((stored?.hidden ?? []).filter((id) => known.has(id)));
    const widths: Record<string, CardWidth> = {};
    for (const [id, w] of Object.entries(stored?.widths ?? {})) {
      if (known.has(id)) widths[id] = w;
    }
    return { order, hidden, widths };
  });

  // Skip the initial render so a user who never customises gets no write.
  const firstRun = useRef(true);
  useEffect(() => {
    if (firstRun.current) {
      firstRun.current = false;
      return;
    }
    try {
      const payload: StoredLayout = {
        order: state.order,
        hidden: Array.from(state.hidden),
        widths: state.widths,
      };
      localStorage.setItem(KEY, JSON.stringify(payload));
    } catch {
      /* storage unavailable: keep the layout in memory for this session */
    }
  }, [state]);

  // move swaps with the adjacent id even when that one is hidden; drag-and-drop
  // covers precise placement.
  const move = useCallback((id: string, dir: -1 | 1) => {
    setState((prev) => {
      const idx = prev.order.indexOf(id);
      if (idx < 0) return prev;
      const swapIdx = idx + dir;
      if (swapIdx < 0 || swapIdx >= prev.order.length) return prev;
      const order = prev.order.slice();
      [order[idx], order[swapIdx]] = [order[swapIdx], order[idx]];
      return { order, hidden: prev.hidden, widths: prev.widths };
    });
  }, []);

  // Dragging down drops after the target, dragging up drops before it.
  const reorder = useCallback((draggedId: string, targetId: string) => {
    setState((prev) => {
      if (draggedId === targetId) return prev;
      const from = prev.order.indexOf(draggedId);
      const to = prev.order.indexOf(targetId);
      if (from < 0 || to < 0) return prev;
      const order = prev.order.slice();
      order.splice(from, 1);
      const targetAt = order.indexOf(targetId);
      const insertAt = from < to ? targetAt + 1 : targetAt;
      order.splice(insertAt, 0, draggedId);
      return { order, hidden: prev.hidden, widths: prev.widths };
    });
  }, []);

  // A drop hands over the visible cards in their new order. The hidden ones
  // keep their places, and the visible ones fill the places visible cards held.
  const setVisibleOrder = useCallback((visible: string[]) => {
    setState((prev) => {
      const moving = new Set(visible);
      const next = visible.slice();
      const order = prev.order.map((id) => (moving.has(id) ? (next.shift() ?? id) : id));
      return { order, hidden: prev.hidden, widths: prev.widths };
    });
  }, []);

  const toggleHidden = useCallback((id: string) => {
    setState((prev) => {
      const hidden = new Set(prev.hidden);
      if (hidden.has(id)) hidden.delete(id);
      else hidden.add(id);
      return { order: prev.order, hidden, widths: prev.widths };
    });
  }, []);

  const toggleWidth = useCallback((id: string) => {
    setState((prev) => {
      const current = prev.widths[id] ?? "full";
      const widths = { ...prev.widths, [id]: current === "full" ? "half" : "full" } as Record<
        string,
        CardWidth
      >;
      return { order: prev.order, hidden: prev.hidden, widths };
    });
  }, []);

  const reset = useCallback(() => {
    setState({ order: defaultOrder.slice(), hidden: new Set<string>(), widths: {} });
  }, [defaultOrder]);

  const getVisibleIds = useCallback(
    () => state.order.filter((id) => !state.hidden.has(id)),
    [state]
  );

  const getWidth = useCallback(
    (id: string): CardWidth => state.widths[id] ?? "full",
    [state]
  );

  return {
    order: state.order,
    hidden: state.hidden,
    getVisibleIds,
    move,
    reorder,
    setVisibleOrder,
    toggleHidden,
    toggleWidth,
    getWidth,
    reset,
  };
}

function GripIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="currentColor" aria-hidden="true" className="block">
      <circle cx="5" cy="3" r="1.15" />
      <circle cx="9" cy="3" r="1.15" />
      <circle cx="5" cy="7" r="1.15" />
      <circle cx="9" cy="7" r="1.15" />
      <circle cx="5" cy="11" r="1.15" />
      <circle cx="9" cy="11" r="1.15" />
    </svg>
  );
}

function ChevronUpIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true" className="block">
      <path d="M3.2 10.7 8 5.3 12.8 10.7Z" />
    </svg>
  );
}

function ChevronDownIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true" className="block">
      <path d="M3.2 5.3 8 10.7 12.8 5.3Z" />
    </svg>
  );
}

// The eye and columns icons cut the pupil and the divider out of a filled
// shape with the surface colour, so they still read as gaps.
function EyeOffIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true" className="block">
      <path d="M2 8s2.4-4 6-4 6 4 6 4-2.4 4-6 4-6-4-6-4Z" />
      <circle cx="8" cy="8" r="1.6" fill="var(--carbon-surface2, transparent)" />
      <rect x="0.5" y="7.2" width="15" height="1.6" rx="0.8" transform="rotate(45 8 8)" />
    </svg>
  );
}

function ColumnsIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true" className="block">
      <rect x="2" y="2.5" width="12" height="11" rx="1.5" />
      <rect x="7.35" y="3.3" width="1.3" height="9.4" rx="0.65" fill="var(--carbon-surface2, transparent)" />
    </svg>
  );
}

const iconBtn =
  "rounded-pill p-1 text-carbon-textSub hover:text-carbon-text hover:bg-carbon-hover " +
  "disabled:opacity-40 disabled:pointer-events-none motion-safe:transition-colors";

export interface CustomizableBlockProps {
  id: string;
  label: string;
  index: number;
  total: number;
  isFirst: boolean;
  isLast: boolean;
  editing: boolean;
  /** Starts carrying the card; the grid cell around it is what lifts. */
  onGripPointerDown: PointerEventHandler<HTMLSpanElement>;
  onMoveUp: () => void;
  onMoveDown: () => void;
  onHide: () => void;
  width: CardWidth;
  onToggleWidth: () => void;
  t: T;
  children: ReactNode;
}

// CustomizableBlock renders its children unchanged outside edit mode. In edit
// mode it adds a control bar and makes the block a drag source and drop
// target. The width toggle does not size anything here: Dashboard reads it back
// through getWidth() for the grid cell.
export function CustomizableBlock({
  id,
  label,
  index,
  total,
  isFirst,
  isLast,
  editing,
  onGripPointerDown,
  onMoveUp,
  onMoveDown,
  onHide,
  width,
  onToggleWidth,
  t,
  children,
}: CustomizableBlockProps) {
  if (!editing) return <>{children}</>;

  return (
    <div role="group" aria-label={`${label} (${index + 1}/${total})`} data-block-id={id} className="rounded-card">
      <div className="mb-2 flex items-center gap-2 rounded-card bg-carbon-surface2 px-2 py-1.5">
        {/* The grip is for a pointer; the keyboard uses the arrows. */}
        <span
          className="shrink-0 cursor-grab touch-none select-none text-carbon-textMuted active:cursor-grabbing"
          aria-hidden="true"
          onPointerDown={onGripPointerDown}
        >
          <GripIcon />
        </span>
        <span className="min-w-0 flex-1 truncate text-xs font-semibold uppercase tracking-wider text-carbon-textSub">
          {label}
        </span>
        <div className="flex shrink-0 items-center gap-1">
          <IconTipButton tip={t("dashboard.moveUp")} onClick={onMoveUp} disabled={isFirst} className={iconBtn}>
            <ChevronUpIcon />
          </IconTipButton>
          <IconTipButton tip={t("dashboard.moveDown")} onClick={onMoveDown} disabled={isLast} className={iconBtn}>
            <ChevronDownIcon />
          </IconTipButton>
          <IconTipButton
            tip={width === "full" ? t("dashboard.makeHalfWidth") : t("dashboard.makeFullWidth")}
            onClick={onToggleWidth}
            className={iconBtn}
          >
            <ColumnsIcon />
          </IconTipButton>
          <IconTipButton tip={t("dashboard.hideCard")} onClick={onHide} className={iconBtn}>
            <EyeOffIcon />
          </IconTipButton>
        </div>
      </div>

      {children}
    </div>
  );
}
