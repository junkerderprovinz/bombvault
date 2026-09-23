import { useRef, useState } from "react";
import type { DragEvent, DragEventHandler, HTMLAttributes } from "react";

// useDragReorder reorders a list while a row is dragged over the others, not
// only on drop, so the new position is visible during the drag. E is the row
// element type, so the props spread onto an <li> or a <div> alike:
//
//   const { dragIndex, rowProps } = useDragReorder<HTMLLIElement>(reorder, saving);
//   items.map((it, i) => (
//     <li {...rowProps(i)} className={dragIndex === i ? "opacity-40" : ""}>…</li>
//   ))
//
// reorder(from, to) moves item `from` to index `to` in the caller's state.
export function useDragReorder<E extends HTMLElement = HTMLElement>(
  onReorder: (from: number, to: number) => void,
  disabled = false,
): {
  dragIndex: number | null;
  rowProps: (index: number) => HTMLAttributes<E>;
} {
  // Several dragEnter events can fire between renders, so the current index
  // lives in a ref; state would be stale.
  const dragRef = useRef<number | null>(null);
  const [dragIndex, setDragIndex] = useState<number | null>(null);

  function rowProps(index: number): HTMLAttributes<E> {
    if (disabled) return {};

    const onDragStart: DragEventHandler<E> = (e) => {
      dragRef.current = index;
      setDragIndex(index);
      e.dataTransfer.effectAllowed = "move";
      try {
        e.dataTransfer.setData("text/plain", String(index));
      } catch {
        /* some environments disallow setData; dragRef still carries the index */
      }
    };

    const shiftOver = (e: DragEvent<E>) => {
      e.preventDefault();
      const from = dragRef.current;
      if (from === null || from === index) return;
      onReorder(from, index);
      dragRef.current = index;
      setDragIndex(index);
    };

    const onDragOver: DragEventHandler<E> = (e) => {
      e.preventDefault();
      e.dataTransfer.dropEffect = "move";
    };

    const clear = () => {
      dragRef.current = null;
      setDragIndex(null);
    };

    return {
      draggable: true,
      onDragStart,
      onDragEnter: shiftOver,
      onDragOver,
      onDragEnd: clear,
      onDrop: (e) => {
        e.preventDefault();
        clear();
      },
    };
  }

  return { dragIndex, rowProps };
}
