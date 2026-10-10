import { useEffect, useRef, type PointerEvent } from "react";

const HOLD_MS = 500;

// A finger never rests quite still, and one that travels further is scrolling.
const SLOP_PX = 10;

export interface LongPress {
  handlers: {
    onPointerDown: (e: PointerEvent) => void;
    onPointerMove: (e: PointerEvent) => void;
    onPointerUp: () => void;
    onPointerCancel: () => void;
  };
  /** True once for the tap that ends a long press, so its click can be dropped. */
  took: () => boolean;
  /** Whether the pointer last put down was a finger. */
  byTouch: () => boolean;
}

/** useLongPress calls back when a finger has rested on an element. A mouse
 *  has its right button for that and never triggers it. */
export function useLongPress(onLongPress: () => void): LongPress {
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const origin = useRef({ x: 0, y: 0 });
  const fired = useRef(false);
  const touch = useRef(false);
  const callback = useRef(onLongPress);
  callback.current = onLongPress;

  const cancel = () => clearTimeout(timer.current);
  useEffect(() => cancel, []);

  return {
    handlers: {
      onPointerDown(e) {
        touch.current = e.pointerType === "touch";
        fired.current = false;
        if (!touch.current) return;
        origin.current = { x: e.clientX, y: e.clientY };
        cancel();
        timer.current = setTimeout(() => {
          fired.current = true;
          callback.current();
        }, HOLD_MS);
      },
      onPointerMove(e) {
        if (Math.hypot(e.clientX - origin.current.x, e.clientY - origin.current.y) > SLOP_PX) cancel();
      },
      onPointerUp: cancel,
      onPointerCancel: cancel,
    },
    took() {
      const was = fired.current;
      fired.current = false;
      return was;
    },
    byTouch: () => touch.current,
  };
}
