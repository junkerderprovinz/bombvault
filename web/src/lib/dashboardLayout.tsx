// Customizable dashboard layout: card order, visibility and width per browser,
// kept in localStorage. Reordering uses native HTML5 drag-and-drop, with the
// move up/down buttons as the keyboard and touch fallback.

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type DragEvent as ReactDragEvent,
  type ReactNode,
} from "react";
import { IconTipButton } from "../components/IconTipButton";
import type { TranslationKey } from "./i18n";

const KEY = "bombvault.dashboardLayout";

type T = (key: TranslationKey) => string;

// BlockDragHandlers are built by the Dashboard for each block. onDragOver has
// to call preventDefault, or the browser never fires drop.
export interface BlockDragHandlers {
  onDragStart: (e: ReactDragEvent<HTMLElement>) => void;
  onDragOver: (e: ReactDragEvent<HTMLElement>) => void;
  onDrop: (e: ReactDragEvent<HTMLElement>) => void;
  onDragEnd: (e: ReactDragEvent<HTMLElement>) => void;
}

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
  dragHandlers: BlockDragHandlers;
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
  dragHandlers,
  onMoveUp,
  onMoveDown,
  onHide,
  width,
  onToggleWidth,
  t,
  children,
}: CustomizableBlockProps) {
  // dragenter and dragleave also fire for every child element, so a depth
  // counter keeps the drop indicator from flickering.
  const depth = useRef(0);
  const [over, setOver] = useState(false);

  const handleDragEnter = (e: ReactDragEvent<HTMLElement>) => {
    e.preventDefault();
    depth.current += 1;
    setOver(true);
  };
  const handleDragLeave = () => {
    depth.current -= 1;
    if (depth.current <= 0) {
      depth.current = 0;
      setOver(false);
    }
  };
  const handleDrop = (e: ReactDragEvent<HTMLElement>) => {
    depth.current = 0;
    setOver(false);
    dragHandlers.onDrop(e);
  };
  const handleDragEnd = (e: ReactDragEvent<HTMLElement>) => {
    depth.current = 0;
    setOver(false);
    dragHandlers.onDragEnd(e);
  };

  if (!editing) return <>{children}</>;

  return (
    <div
      role="group"
      aria-label={`${label} (${index + 1}/${total})`}
      data-block-id={id}
      draggable
      onDragStart={dragHandlers.onDragStart}
      onDragEnter={handleDragEnter}
      onDragOver={dragHandlers.onDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
      onDragEnd={handleDragEnd}
      className="relative rounded-card"
    >
      {/* Absolute, so the drop indicator never shifts the layout. */}
      {over && (
        <div
          className="pointer-events-none absolute -top-3 start-0 end-0 h-0.5 rounded-control bg-carbon-text"
          aria-hidden="true"
        />
      )}

      <div className="mb-2 flex items-center gap-2 rounded-card bg-carbon-surface2 px-2 py-1.5">
        <span className="shrink-0 cursor-move select-none text-carbon-textMuted" aria-hidden="true">
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
