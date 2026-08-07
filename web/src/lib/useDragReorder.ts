import { useRef, useState } from "react";
import type { DragEvent, DragEventHandler, HTMLAttributes } from "react";

// useDragReorder — live drag-to-reorder for a list rendered as rows. As a dragged
// row passes over another, the list reorders IMMEDIATELY (the other rows shift
// live) instead of only settling on drop, so you see the new position while
// dragging. Shared by the container and VM backup-order lists.
//
// Generic over the row element type so the returned props spread cleanly onto an
// <li> (containers) or a <div> (VMs) without a type mismatch.
//
// Wire it up:
//   const { dragIndex, rowProps } = useDragReorder<HTMLLIElement>(reorder, saving);
//   items.map((it, i) => (
//     <li {...rowProps(i)} className={dragIndex === i ? "opacity-40" : ""}>…</li>
//   ))
// where reorder(from, to) moves item `from` to index `to` in your list state.
export function useDragReorder<E extends HTMLElement = HTMLElement>(
  onReorder: (from: number, to: number) => void,
  disabled = false,
): {
  dragIndex: number | null;
  rowProps: (index: number) => HTMLAttributes<E>;
} {
  // A ref holds the authoritative current index of the dragged row: several
  // dragEnter events can fire between renders, so reading state would go stale.
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
      onReorder(from, index); // move the dragged row here now → the others shift live
      dragRef.current = index; // the dragged row now lives at `index`
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
