// ---------------------------------------------------------------------------
// Customizable dashboard layout (#46) — per-browser card order + visibility.
//
// The user can reorder the dashboard cards, hide the ones they don't want, and
// (per-card width toggle) size a card full or half width so two half cards sit
// side by side. The preference is persisted in localStorage (like the
// Simple/Advanced toggle in advanced.tsx). This module owns:
//   • useDashboardLayout() — the persisted { order, hidden, widths } state +
//                            mutators.
//   • CustomizableBlock    — the per-card wrapper that, in edit mode, adds a
//                            Carbon-styled control bar (drag handle, move
//                            up/down, width toggle, hide) and wires native
//                            HTML5 drag-and-drop.
//
// No new npm dependency: reordering uses native HTML5 drag-and-drop, with the
// move up/down buttons as the accessible + touch fallback.
// ---------------------------------------------------------------------------

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

// Native-DnD handlers the Dashboard builds per block and hands to the wrapper.
export interface BlockDragHandlers {
  onDragStart: (e: ReactDragEvent<HTMLElement>) => void;
  onDragOver: (e: ReactDragEvent<HTMLElement>) => void;
  onDrop: (e: ReactDragEvent<HTMLElement>) => void;
  onDragEnd: (e: ReactDragEvent<HTMLElement>) => void;
}

// CardWidth — per-card width toggle (feature request B): "half" cards flow
// side-by-side (two per row) in the Dashboard's responsive grid; "full" (the
// default for every card, incl. ones never toggled) spans the full row.
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

// readStored parses the persisted value defensively: a missing, corrupt or
// wrong-shaped value yields null (→ defaults). Only string entries survive;
// widths entries that aren't exactly "full"/"half" are dropped (→ default full).
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

// mergeOrder normalises a stored order against the known default order:
//   • stale / unknown ids (not in defaultOrder) are dropped, and
//   • newly-added block ids (present in defaultOrder but not yet stored) are
//     appended at the end so a future card shows up, visible, without a reset.
function mergeOrder(stored: string[], defaultOrder: string[]): string[] {
  const known = new Set(defaultOrder);
  const merged: string[] = [];
  const seen = new Set<string>();
  for (const id of stored) {
    if (known.has(id) && !seen.has(id)) {
      merged.push(id);
      seen.add(id);
    }
  }
  for (const id of defaultOrder) {
    if (!seen.has(id)) {
      merged.push(id);
      seen.add(id);
    }
  }
  return merged;
}

/**
 * useDashboardLayout — persisted per-browser card order + hidden set.
 *
 * @param defaultOrder the canonical block-id order (source of truth for which
 *        ids are "known"). Unknown ids in storage are ignored; new default ids
 *        are appended at the end, visible.
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

  // Persist on change. Skip the very first run so users who never customise
  // don't get a redundant write; every real mutation below persists.
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
      /* storage unavailable — keep the layout in-memory for this session */
    }
  }, [state]);

  // move — simple index swap with the immediately-adjacent id in `order`
  // (dir -1 = up, +1 = down). Skipping past hidden neighbours is intentionally
  // not done; native drag-and-drop covers precise placement.
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

  // reorder — move `draggedId` next to `targetId` (used by native DnD). Dragging
  // downward drops after the target, upward drops before it, which feels natural.
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

  // toggleWidth — flips a single card between full (default) and half width.
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

  // getVisibleIds — the layout-visible (not hidden) ids, in order. Callers that
  // also gate on Advanced mode filter this further against their block list.
  const getVisibleIds = useCallback(
    () => state.order.filter((id) => !state.hidden.has(id)),
    [state]
  );

  // getWidth — the persisted width for a card id, defaulting to "full" for
  // ids never toggled (matches the pre-existing single full-width column).
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

// ---------------------------------------------------------------------------
// Inline icons — stroke/fill currentColor, matching the existing SVG idiom.
// ---------------------------------------------------------------------------

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

// FILLED (design-language.md "Icon glyphs" — GlimStone follow-up round, full
// icon-fill sweep): all four glyphs below were the last stroke-only icons in
// the dashboard's per-card control bar. Chevrons are redrawn as filled
// triangles (rule 219 — a chevron is a line glyph, needs real geometry, not
// a thicker stroke); the eye and the columns split use the same
// filled-shape-with-a-surface-colour-cutout technique this app already uses
// elsewhere for a slider knob/switch dot (rule 220's thin-structural-line
// case), so the pupil/divider still reads as a gap rather than vanishing
// into a solid blob.
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
  "rounded-control p-1 text-carbon-textSub hover:text-carbon-text hover:bg-carbon-hover " +
  "disabled:opacity-40 disabled:pointer-events-none motion-safe:transition-colors";

// ---------------------------------------------------------------------------
// CustomizableBlock — wraps a single dashboard block.
//   editing=false → renders children plainly, no wrapper chrome (view parity).
//   editing=true  → adds a control bar (drag handle, label, move up/down,
//                   width toggle, hide) and makes the block a native drag
//                   source + drop target. The width toggle itself doesn't
//                   affect layout here — Dashboard.tsx reads `width` back via
//                   getWidth() to size the grid wrapper around this block.
// ---------------------------------------------------------------------------

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
  // Drop-target highlight. A depth counter tracks dragenter/dragleave across the
  // block's own children so the indicator doesn't flicker as the pointer crosses
  // inner elements.
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
  const handleDragOver = (e: ReactDragEvent<HTMLElement>) => {
    // preventDefault (in the parent handler) is required for onDrop to fire.
    dragHandlers.onDragOver(e);
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

  // View mode: no chrome — the card renders exactly as it did before #46.
  if (!editing) return <>{children}</>;

  return (
    <div
      role="group"
      aria-label={`${label} (${index + 1}/${total})`}
      data-block-id={id}
      draggable
      onDragStart={dragHandlers.onDragStart}
      onDragEnter={handleDragEnter}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
      onDragEnd={handleDragEnd}
      className="relative rounded-card"
    >
      {/* Drop-target indicator — absolute so it never shifts layout. */}
      {over && (
        <div
          className="pointer-events-none absolute -top-3 start-0 end-0 h-0.5 rounded-control bg-carbon-text"
          aria-hidden="true"
        />
      )}

      {/* Control bar */}
      <div className="mb-2 flex items-center gap-2 rounded-card bg-carbon-surface2 px-2 py-1.5">
        <span className="shrink-0 cursor-move select-none text-carbon-textMuted" aria-hidden="true">
          <GripIcon />
        </span>
        <span className="min-w-0 flex-1 truncate text-xs font-semibold uppercase tracking-wider text-carbon-textSub">
          {label}
        </span>
        {/* All four are IconTipButton, not plain <button> + `title` (whole-app
            sweep). Each carried an `aria-label` AND a duplicate native
            `title` of the same string: a silent accessible name plus the
            browser's own unstyled OS balloon, on a bar of four glyphs whose
            meaning is not guessable. That pairing is exactly what
            IconTipButton.tsx's header says the file exists to replace, and
            it is what Dashboard's own customize pencil (the trigger that
            reveals THIS bar) was converted away from one commit earlier —
            these four sat one component away from it and were missed.
              Same tip strings, same handlers, same `iconBtn` chrome; the
            bubble is now the real .glim-bubble, appears on keyboard focus
            as well as hover, and closes on scroll/Escape like every other
            tooltip in the app. IconTipButton sets `aria-label` from `tip`
            itself, so the accessible name is unchanged. */}
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
